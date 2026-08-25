// check-ts-strictness.test.mjs — Pins the TS strictness ratchet counting rules.
import test from 'node:test'
import assert from 'node:assert/strict'
import { mkdtempSync, mkdirSync, writeFileSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { countStrictness } from './check-ts-strictness.mjs'

function rootWith(files) {
  const root = mkdtempSync(join(tmpdir(), 'ts-strict-'))
  for (const [rel, body] of Object.entries(files)) {
    const path = join(root, rel)
    mkdirSync(join(path, '..'), { recursive: true })
    writeFileSync(path, body)
  }
  return root
}

test('counts explicit any annotations but not any-typed comments or identifiers', () => {
  const root = rootWith({
    'a.ts': `
export const one: any = 1
export const two: any[] = []
export const three = x as any
export const four: Record<string, any> = {}
const obj = { any: 1, anyly: 2 }
obj.any = 3
export const word = 'anywhere no annotation'
`
  })
  const counts = countStrictness(root)
  assert.equal(counts.explicit_any, 4, JSON.stringify(counts))
  assert.equal(counts.ts_nocheck, 0)
  rmSync(root, { recursive: true, force: true })
})

test('counts @ts-nocheck directives in authored files only', () => {
  const root = rootWith({
    'authored.ts': '// @ts-nocheck\nexport const x = 1\n',
    'generated/nocheck.ts': '// @ts-nocheck\nexport const y = 1\n',
    'types/wire/wire-nocheck.ts': '// @ts-nocheck\nexport const z = 1\n',
    'dom-primitives-form.ts': '// @ts-nocheck\nexport const w = 1\n'
  })
  const counts = countStrictness(root)
  assert.equal(counts.ts_nocheck, 1, JSON.stringify(counts))
  rmSync(root, { recursive: true, force: true })
})

test('generated files and non-TS files are excluded from any counting', () => {
  const root = rootWith({
    'generated/gen.ts': 'export const a: any = 1\n',
    'types/wire/wire-x.ts': 'export const b: any = 1\n',
    'dom-primitives-pointer.ts': 'export const c: any = 1\n',
    'plain.js': 'export const d = 1\n',
    'real.ts': 'export const e: any = 1\n'
  })
  const counts = countStrictness(root)
  assert.equal(counts.explicit_any, 1, JSON.stringify(counts))
  rmSync(root, { recursive: true, force: true })
})
