// check-reference-schema-sync.test.mjs — Both directions of the reference-doc/schema contract.
//
// Why: the gate historically checked schema -> doc only (every shipped mode has a section).
// A doc naming a mode the schema never exposed passed silently, and an agent reading the
// reference called it and got an invalid-mode error (kaboom-n3si: analyze({what:"history"})).
// These tests pin the doc -> schema direction as well.

import { test } from 'node:test'
import assert from 'node:assert/strict'
import { promises as fs } from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

import {
  TOOL_SPECS,
  collectViolations,
  collectSiteDocViolations,
  findUnknownModes,
  extractToolModes
} from './check-reference-schema-sync.mjs'

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../..')
const GOLDEN = 'cmd/browser-agent/testdata/mcp-tools-list.golden.json'

test('findUnknownModes flags a documented mode the schema does not expose', () => {
  const doc = ['### `history`', '', '```js', 'analyze({what: "history"})', '```'].join('\n')
  assert.deepEqual(findUnknownModes('analyze', doc, ['dom', 'performance']), ['history'])
})

test('findUnknownModes accepts a documented mode the schema exposes', () => {
  const doc = ['### `dom`', '', '```js', 'analyze({what: "dom", selector: "#a"})', '```'].join('\n')
  assert.deepEqual(findUnknownModes('analyze', doc, ['dom', 'performance']), [])
})

test('findUnknownModes ignores calls addressed to a different tool', () => {
  const doc = '```js\nobserve({what: "history"})\n```'
  assert.deepEqual(findUnknownModes('analyze', doc, ['dom']), [])
})

test('findUnknownModes reports each unknown mode once, in document order', () => {
  const doc = 'analyze({what: "zzz"})\nanalyze({what: "aaa"})\nanalyze({what: "zzz"})'
  assert.deepEqual(findUnknownModes('analyze', doc, ['dom']), ['zzz', 'aaa'])
})

test('every mode named by a published site page exists in that tool schema', async () => {
  const unknown = (await collectSiteDocViolations()).map(
    (v) => `${v.docPath} -> ${v.tool}: ${v.unknownModes.join(', ')}`
  )
  assert.deepEqual(unknown, [], `site docs name modes the schema does not expose:\n${unknown.join('\n')}`)
})

test('the shipped reference docs still cover every mode and required heading', async () => {
  assert.deepEqual(await collectViolations(), [])
})

test('extracted schema modes match the shipped tools/list golden', async () => {
  const golden = JSON.parse(await fs.readFile(path.join(repoRoot, GOLDEN), 'utf8'))
  const goldenModes = new Map(golden.map((tool) => [tool.name, tool.inputSchema.properties.what.enum]))

  for (const spec of TOOL_SPECS) {
    const extracted = await extractToolModes(spec)
    assert.deepEqual(
      [...extracted].sort(),
      [...goldenModes.get(spec.tool)].sort(),
      `${spec.tool}: ${spec.schemaPath} and ${GOLDEN} disagree on the mode list`
    )
  }
})
