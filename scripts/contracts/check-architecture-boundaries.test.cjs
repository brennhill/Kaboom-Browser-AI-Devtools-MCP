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
    forbidden_source_files: [],
    forbidden_import_suffixes: [],
    forbid_reexports: false,
    enforce_zero_cycles: true,
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

test('rejects circular source dependencies and reports every module in the cycle', () => {
  const result = check(
    fixture({
      'src/background/index.ts': "import { sync } from './sync/sync-manager.js'\nexport const hub = sync\n",
      'src/background/sync/sync-manager.ts':
        "import { execute } from '../exec/browser-actions.js'\nexport const sync = execute\n",
      'src/background/exec/browser-actions.ts': "import { hub } from '../index.js'\nexport const execute = hub\n"
    })
  )

  assert.equal(result.status, 1)
  assert.match(result.stderr, /circular dependency/)
  assert.match(result.stderr, /src\/background\/index\.ts/)
  assert.match(result.stderr, /src\/background\/sync\/sync-manager\.ts/)
  assert.match(result.stderr, /src\/background\/exec\/browser-actions\.ts/)
})

test('rejects aggregate files and imports that bypass canonical owners', () => {
  const root = fixture(
    {
      'src/types/index.ts': 'export interface HiddenContract { value: string }\n',
      'src/content/view.ts': "import type { HiddenContract } from '../types/index.js'\nexport const view = 1\n"
    },
    {
      forbidden_source_files: ['src/types/index.ts'],
      forbidden_import_suffixes: ['types/index.js']
    }
  )
  const result = check(root)
  assert.equal(result.status, 1)
  assert.match(result.stderr, /prohibited aggregate or compatibility surface/)
  assert.match(result.stderr, /import must target the canonical owner/)
})

test('rejects every internal re-export when strict ownership is enabled', () => {
  const result = check(
    fixture(
      {
        'src/lib/owner.ts': 'export const canonical = 1\n',
        'src/lib/facade.ts': "export { canonical } from './owner.js'\n"
      },
      { forbid_reexports: true }
    )
  )
  assert.equal(result.status, 1)
  assert.match(result.stderr, /internal re-export is prohibited/)
})
