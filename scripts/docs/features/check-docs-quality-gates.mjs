#!/usr/bin/env node

import { spawnSync } from 'node:child_process'
import { promises as fs } from 'node:fs'
import path from 'node:path'

const phase = Number(process.argv[2] ?? 1)
if (!Number.isInteger(phase) || phase < 1 || phase > 3) {
  console.error('Usage: node scripts/docs/features/check-docs-quality-gates.mjs <phase:1|2|3>')
  process.exit(1)
}

function runLintIntegrity() {
  const proc = spawnSync('python3', ['scripts/docs/lint-documentation.py', 'docs/features', 'docs/architecture'], {
    cwd: process.cwd(),
    encoding: 'utf8'
  })

  const output = `${proc.stdout ?? ''}${proc.stderr ?? ''}`
  const issues = output
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter((line) => line.startsWith('⚠️') || line.startsWith('❌'))

  return { code: proc.status ?? 1, output, issues }
}

const INTEGRITY_PATTERNS = [
  /missing YAML frontmatter/i,
  /frontmatter parse error/i,
  /missing review date field/i,
  /broken link/i,
  /unresolved/i,
  /code file not found/i
]

function classifyIssues(issues) {
  const integrity = []
  const stale = []
  const other = []

  for (const issue of issues) {
    if (/review date is stale/i.test(issue)) {
      stale.push(issue)
      continue
    }
    if (INTEGRITY_PATTERNS.some((pattern) => pattern.test(issue)) || issue.startsWith('❌')) {
      integrity.push(issue)
      continue
    }
    other.push(issue)
  }

  return { integrity, stale, other }
}

function dedupe(values) {
  return [...new Set(values)]
}

function extractQuotedStrings(source) {
  return [...source.matchAll(/"([^"]+)"/g)].map((match) => match[1])
}

function extractWhatEnum(schemaSource) {
  const match = schemaSource.match(/"what"\s*:\s*map\[string\]any\{[\s\S]*?"enum"\s*:\s*\[\]string\{([\s\S]*?)\}/m)
  if (!match) throw new Error('Could not find what enum in schema source')
  return dedupe(extractQuotedStrings(match[1]))
}

function extractInteractActions(schemaSource) {
  const match = schemaSource.match(/var\s+actionSpecs\s*=\s*\[\]ActionSpec\{([\s\S]*?)\n\}/m)
  if (!match) throw new Error('Could not find interact specs variable')
  return dedupe(
    [...match[1].matchAll(/Name:\s*"([^"]+)"/g)]
      .map((item) => item[1])
  )
}

function escapeRegExp(value) {
  return value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
}

function sectionForMode(source, mode) {
  const headingRegex = new RegExp(`^###\\s+\`${escapeRegExp(mode)}\`\\s*$`, 'm')
  const startMatch = source.match(headingRegex)
  if (!startMatch || startMatch.index === undefined) return null

  const start = startMatch.index
  const remaining = source.slice(start + startMatch[0].length)
  const nextHeadingOffset = remaining.search(/\n###\s+`/)
  const end = nextHeadingOffset === -1 ? source.length : start + startMatch[0].length + nextHeadingOffset
  return source.slice(start, end)
}

async function validateReferenceExamples() {
  const root = process.cwd()
  const specs = [
    {
      tool: 'observe',
      modes: extractWhatEnum(await fs.readFile(path.join(root, 'internal/schema/observe.go'), 'utf8')),
      doc: path.join(root, 'gokaboom.dev/src/content/docs/reference/examples/observe-examples.md')
    },
    {
      tool: 'analyze',
      modes: extractWhatEnum(await fs.readFile(path.join(root, 'internal/schema/analyze.go'), 'utf8')),
      doc: path.join(root, 'gokaboom.dev/src/content/docs/reference/examples/analyze-examples.md')
    },
    {
      tool: 'configure',
      modes: extractWhatEnum(await fs.readFile(path.join(root, 'internal/schema/configure/properties_core.go'), 'utf8')),
      doc: path.join(root, 'gokaboom.dev/src/content/docs/reference/examples/configure-examples.md')
    },
    {
      tool: 'generate',
      modes: extractWhatEnum(await fs.readFile(path.join(root, 'internal/schema/generate.go'), 'utf8')),
      doc: path.join(root, 'gokaboom.dev/src/content/docs/reference/examples/generate-examples.md')
    },
    {
      tool: 'interact',
      modes: extractInteractActions(await fs.readFile(path.join(root, 'internal/schema/interact/actions.go'), 'utf8')),
      doc: path.join(root, 'gokaboom.dev/src/content/docs/reference/examples/interact-examples.md')
    }
  ]

  const failures = []

  for (const spec of specs) {
    const source = await fs.readFile(spec.doc, 'utf8')
    for (const mode of spec.modes) {
      const section = sectionForMode(source, mode)
      if (!section) {
        failures.push(`${spec.tool}/${mode}: missing section`)
        continue
      }
      if (!/####\s+Minimal call/.test(section)) failures.push(`${spec.tool}/${mode}: missing Minimal call`)
      if (!/####\s+Expected response shape/.test(section)) failures.push(`${spec.tool}/${mode}: missing Expected response shape`)
      if (!/####\s+Failure example and fix/.test(section)) failures.push(`${spec.tool}/${mode}: missing Failure example and fix`)
    }
  }

  return failures
}

async function main() {
  const lint = runLintIntegrity()
  const { integrity, stale, other } = classifyIssues(lint.issues)
  let failed = false

  if (integrity.length > 0) {
    failed = true
    console.error(`Phase ${phase} gate: integrity violations (${integrity.length})`)
    integrity.slice(0, 20).forEach((line) => console.error(`  - ${line}`))
  }

  if (phase >= 2 && stale.length > 0) {
    failed = true
    console.error(`Phase ${phase} gate: stale review-date violations (${stale.length})`)
    stale.slice(0, 20).forEach((line) => console.error(`  - ${line}`))
  }

  if (phase >= 3) {
    const exampleFailures = await validateReferenceExamples()
    if (exampleFailures.length > 0) {
      failed = true
      console.error(`Phase 3 gate: reference example violations (${exampleFailures.length})`)
      exampleFailures.slice(0, 30).forEach((line) => console.error(`  - ${line}`))
    }
  }

  if (!failed) {
    console.log(`Docs quality gate phase ${phase}: PASS`)
    if (other.length > 0) {
      console.log(`Additional non-blocking warnings: ${other.length}`)
    }
    return
  }

  process.exit(1)
}

await main()
