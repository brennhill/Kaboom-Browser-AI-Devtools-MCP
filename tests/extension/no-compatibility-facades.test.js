// no-compatibility-facades.test.js — Prevents deleted extension compatibility barrels from returning.

import assert from 'node:assert/strict'
import { existsSync, readFileSync } from 'node:fs'
import test from 'node:test'

test('obsolete extension type compatibility barrel is absent', () => {
  assert.equal(
    existsSync('src/types/messages.ts'),
    false,
    'src/types/messages.ts is a compatibility facade; import focused type modules directly'
  )
})

test('pending query dispatcher does not re-export APIs owned by command modules', () => {
  const source = readFileSync('src/background/pending-queries.ts', 'utf8')
  assert.doesNotMatch(source, /export\s+(?:type\s+)?\{/, 'dispatcher must not re-export command helper APIs')
})
