// no-compatibility-facades.test.js — Prevents deleted extension compatibility barrels from returning.

import assert from 'node:assert/strict'
import { existsSync } from 'node:fs'
import test from 'node:test'

test('obsolete extension type compatibility barrel is absent', () => {
  assert.equal(
    existsSync('src/types/messages.ts'),
    false,
    'src/types/messages.ts is a compatibility facade; import focused type modules directly'
  )
})
