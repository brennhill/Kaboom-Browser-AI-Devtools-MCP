#!/usr/bin/env node

import { promises as fs } from 'node:fs'
import path from 'node:path'

const repoRoot = process.cwd()

const INTERACT_ALIAS_ACTIONS = new Set(['state_save', 'state_load', 'state_list', 'state_delete'])

const TOOL_SPECS = [
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
    schemaPath: 'internal/schema/configure_properties_core.go',
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
    schemaPath: 'internal/schema/interact_actions.go',
    docPath: 'gokaboom.dev/src/content/docs/reference/interact.md',
    enumType: 'interactSpecs',
    specsVar: 'interactActionSpecs',
    ignore: INTERACT_ALIAS_ACTIONS
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
  const pattern = new RegExp(`var\\s+${escapeRegExp(varName)}\\s*=\\s*\\[\\]InteractActionSpec\\{([\\s\\S]*?)\\n\\}`, 'm')
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

async function readFile(relativePath) {
  return fs.readFile(path.join(repoRoot, relativePath), 'utf8')
}

async function main() {
  const violations = []

  for (const spec of TOOL_SPECS) {
    const schemaSource = await readFile(spec.schemaPath)
    const docSource = await readFile(spec.docPath)

    const extractedModes =
      spec.enumType === 'what'
        ? extractWhatEnum(schemaSource)
        : spec.enumType === 'interactSpecs'
          ? extractInteractSpecs(schemaSource, spec.specsVar)
          : extractArrayVar(schemaSource, spec.arrayVar)

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

  if (violations.length === 0) {
    console.log('Reference schema sync: all reference docs cover current tool modes and required sections.')
    return
  }

  console.error('Reference schema sync violations found:\n')
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

main().catch((error) => {
  console.error('Failed to run reference schema sync check:', error)
  process.exit(1)
})
