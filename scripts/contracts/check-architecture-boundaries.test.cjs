// check-architecture-boundaries.test.cjs — Regression tests for architecture policy enforcement.
'use strict'

const assert = require('node:assert/strict')
const { mkdirSync, mkdtempSync, writeFileSync } = require('node:fs')
const { tmpdir } = require('node:os')
const { dirname, join } = require('node:path')
const { spawnSync } = require('node:child_process')
const { test } = require('node:test')

function fixture(files, overrides = {}) {
  const root = mkdtempSync(join(tmpdir(), 'kaboom-architecture-'))
  const config = {
    max_exports_per_file: 2,
    export_exceptions: {},
    forbidden_imports: { content: ['background'], background: ['content'] },
    ...overrides
  }
  writeFileSync(join(root, '.architecture-boundaries.json'), JSON.stringify(config))
  for (const [relative, source] of Object.entries(files)) {
    const destination = join(root, relative)
    mkdirSync(dirname(destination), { recursive: true })
    writeFileSync(destination, source)
  }
  return root
}

function check(root) {
  return spawnSync('node', ['scripts/contracts/check-architecture-boundaries.cjs', root], {
    cwd: process.cwd(),
    encoding: 'utf8'
  })
}

test('rejects forbidden feature dependency directions', () => {
  const root = fixture({
    'src/content/view.ts': "import { state } from '../background/state.js'\nexport const view = state\n",
    'src/background/state.ts': 'export const state = 1\n'
  })
  const result = check(root)
  assert.equal(result.status, 1)
  assert.match(result.stderr, /content must not import background/)
})

test('rejects oversized public surfaces unless a documented exception budgets them', () => {
  const source = 'export const one = 1\nexport const two = 2\nexport const three = 3\n'
  const rejected = check(fixture({ 'src/lib/public.ts': source }))
  assert.equal(rejected.status, 1)
  assert.match(rejected.stderr, /3 exports exceeds public-surface budget 2/)

  const accepted = check(
    fixture(
      { 'src/lib/public.ts': source },
      { export_exceptions: { 'src/lib/public.ts': { max: 3, reason: 'Canonical fixture contract.' } } }
    )
  )
  assert.equal(accepted.status, 0)
})
