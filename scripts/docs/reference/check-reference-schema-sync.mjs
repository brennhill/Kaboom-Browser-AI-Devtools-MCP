#!/usr/bin/env node

import { promises as fs } from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../..')

export const TOOL_SPECS = [
  {
    tool: 'observe',
    schemaPath: 'internal/schema/observe.go',
    docPath: 'gokaboom.dev/src/content/docs/reference/observe.md',
    enumType: 'what'
  },
  {
    tool: 'analyze',
    schemaPath: 'internal/schema/analyze.go',
    docPath: 'gokaboom.dev/src/content/docs/reference/analyze.md',
    enumType: 'what'
  },
  {
    tool: 'configure',
    schemaPath: 'internal/schema/configure/properties_core.go',
    docPath: 'gokaboom.dev/src/content/docs/reference/configure.md',
    enumType: 'what'
  },
  {
    tool: 'generate',
    schemaPath: 'internal/schema/generate.go',
    docPath: 'gokaboom.dev/src/content/docs/reference/generate.md',
    enumType: 'what'
  },
  {
    tool: 'interact',
    schemaPath: 'internal/schema/interact/actions.go',
    docPath: 'gokaboom.dev/src/content/docs/reference/interact.md',
    enumType: 'interactSpecs',
    specsVar: 'actionSpecs'
  }
]

function escapeRegExp(value) {
  return value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
}

function dedupe(values) {
  return [...new Set(values)]
}

function extractQuotedStrings(source) {
  return [...source.matchAll(/"([^"]+)"/g)].map((match) => match[1])
}

function extractWhatEnum(schemaSource) {
  const match = schemaSource.match(/"what"\s*:\s*map\[string\]any\{[\s\S]*?"enum"\s*:\s*\[\]string\{([\s\S]*?)\}/m)
  if (!match) {
    throw new Error('Could not find "what" enum in schema source')
  }
  return dedupe(extractQuotedStrings(match[1]))
}

function extractArrayVar(schemaSource, varName) {
  const pattern = new RegExp(`var\\s+${escapeRegExp(varName)}\\s*=\\s*\\[\\]string\\{([\\s\\S]*?)\\n\\}`, 'm')
  const match = schemaSource.match(pattern)
  if (!match) {
    throw new Error(`Could not find string array variable ${varName}`)
  }
  return dedupe(extractQuotedStrings(match[1]))
}

function extractInteractSpecs(schemaSource, varName) {
  const pattern = new RegExp(`var\\s+${escapeRegExp(varName)}\\s*=\\s*\\[\\]ActionSpec\\{([\\s\\S]*?)\\n\\}`, 'm')
  const match = schemaSource.match(pattern)
  if (!match) {
    throw new Error(`Could not find interact specs variable ${varName}`)
  }
  const names = [...match[1].matchAll(/Name:\s*"([^"]+)"/g)].map((item) => item[1])
  return dedupe(names)
}

function extractHeadingLines(markdown) {
  return markdown
    .split(/\r?\n/)
    .filter((line) => /^##+\s+/.test(line))
    .map((line) => line.toLowerCase())
}

function hasHeading(markdown, headingText) {
  const escaped = escapeRegExp(headingText)
  return new RegExp(`^##\\s+${escaped}\\b`, 'm').test(markdown)
}

function findDocumentedModes(headingLines, expectedModes) {
  const documented = new Set()

  for (const line of headingLines) {
    for (const mode of expectedModes) {
      const re = new RegExp(`(^|[^a-z0-9_])${escapeRegExp(mode)}([^a-z0-9_]|$)`)
      if (re.test(line)) {
        documented.add(mode)
      }
    }
  }

  return documented
}

/**
 * Modes a reference doc invites the reader to call, in `tool({what: "mode"})` form, that the
 * schema never exposed. The doc is the primary surface an LLM reads to decide what it can do,
 * so a mode named here and absent from the enum costs a call and a retry every time
 * (kaboom-n3si: analyze.md documented `history`, which is an observe mode).
 *
 * Only calls addressed to this tool count — a doc may legitimately show a sibling tool's call.
 */
export function findUnknownModes(tool, docSource, knownModes) {
  const known = new Set(knownModes)
  const pattern = new RegExp(`\\b${escapeRegExp(tool)}\\(\\s*\\{\\s*what\\s*:\\s*"([a-z0-9_]+)"`, 'g')
  const referenced = [...docSource.matchAll(pattern)].map((match) => match[1])
  return dedupe(referenced.filter((mode) => !known.has(mode)))
}

async function readFile(relativePath) {
  return fs.readFile(path.join(repoRoot, relativePath), 'utf8')
}

/** Every published site page, not just the five reference pages. An article names modes too. */
const SITE_DOCS_ROOT = 'gokaboom.dev/src/content/docs'

async function markdownPages(relativeDir) {
  const entries = await fs.readdir(path.join(repoRoot, relativeDir), { withFileTypes: true })
  const pages = []
  for (const entry of entries.sort((a, b) => a.name.localeCompare(b.name))) {
    const relPath = path.posix.join(relativeDir, entry.name)
    if (entry.isDirectory()) {
      pages.push(...(await markdownPages(relPath)))
    } else if (entry.name.endsWith('.md') || entry.name.endsWith('.mdx')) {
      pages.push(relPath)
    }
  }
  return pages
}

/** The mode/action list one tool's Go schema declares. */
export async function extractToolModes(spec) {
  const schemaSource = await readFile(spec.schemaPath)
  return spec.enumType === 'what'
    ? extractWhatEnum(schemaSource)
    : spec.enumType === 'interactSpecs'
      ? extractInteractSpecs(schemaSource, spec.specsVar)
      : extractArrayVar(schemaSource, spec.arrayVar)
}

/** schema -> doc: every shipped mode has a section, and the page keeps its required headings. */
export async function collectViolations() {
  const violations = []

  for (const spec of TOOL_SPECS) {
    const docSource = await readFile(spec.docPath)
    const extractedModes = await extractToolModes(spec)

    const expectedModes = spec.ignore
      ? extractedModes.filter((mode) => !spec.ignore.has(mode))
      : extractedModes

    const headings = extractHeadingLines(docSource)
    const documented = findDocumentedModes(headings, expectedModes)
    const missingModes = expectedModes.filter((mode) => !documented.has(mode))

    const sectionViolations = []
    if (!hasHeading(docSource, 'Quick Reference')) {
      sectionViolations.push('Missing required heading: ## Quick Reference')
    }
    if (!hasHeading(docSource, 'Common Parameters')) {
      sectionViolations.push('Missing required heading: ## Common Parameters')
    }

    if (missingModes.length > 0 || sectionViolations.length > 0) {
      violations.push({
        tool: spec.tool,
        docPath: spec.docPath,
        missingModes,
        sectionViolations
      })
    }
  }

  return violations
}

/**
 * Modes named by any published site page that no tool exposes. An article is read by the same
 * agent the reference is, so a stale mode there costs the same failed call and retry.
 */
export async function collectSiteDocViolations() {
  const modesByTool = new Map()
  for (const spec of TOOL_SPECS) {
    modesByTool.set(spec.tool, await extractToolModes(spec))
  }

  const violations = []
  for (const docPath of await markdownPages(SITE_DOCS_ROOT)) {
    const docSource = await readFile(docPath)
    for (const [tool, modes] of modesByTool) {
      const unknownModes = findUnknownModes(tool, docSource, modes)
      if (unknownModes.length > 0) {
        violations.push({ tool, docPath, unknownModes })
      }
    }
  }
  return violations
}

async function main() {
  const violations = await collectViolations()
  const siteViolations = await collectSiteDocViolations()

  if (violations.length === 0 && siteViolations.length === 0) {
    console.log('Reference schema sync: all reference docs cover current tool modes and required sections.')
    return
  }

  console.error('Reference schema sync violations found:\n')
  for (const violation of siteViolations) {
    console.error(`- ${violation.tool} (${violation.docPath})`)
    console.error(`  - Documented modes absent from the schema enum: ${violation.unknownModes.join(', ')}`)
  }
  for (const violation of violations) {
    console.error(`- ${violation.tool} (${violation.docPath})`)
    for (const issue of violation.sectionViolations) {
      console.error(`  - ${issue}`)
    }
    if (violation.missingModes.length > 0) {
      console.error(`  - Missing mode/action sections: ${violation.missingModes.join(', ')}`)
    }
  }

  process.exit(1)
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  main().catch((error) => {
    console.error('Failed to run reference schema sync check:', error)
    process.exit(1)
  })
}
