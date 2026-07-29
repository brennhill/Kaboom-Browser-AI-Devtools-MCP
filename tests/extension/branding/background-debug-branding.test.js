// @ts-nocheck
import { test, describe } from 'node:test'
import assert from 'node:assert'
import { readFileSync } from 'node:fs'

describe('Background debug branding contracts', () => {
  test('background debug globals use Kaboom-scoped names', () => {
    const indexFile = readFileSync('src/background/index.ts', 'utf8')
    const helpersFile = readFileSync('src/background/commands/helpers.ts', 'utf8')

    assert.doesNotMatch(indexFile, /__GASOLINE_DEBUG_LOG__/, 'background/index.ts still uses legacy debug global')
    assert.match(indexFile, /__KABOOM_DEBUG_LOG__/, 'background/index.ts should expose __KABOOM_DEBUG_LOG__')

    assert.doesNotMatch(helpersFile, /__GASOLINE_DEBUG_LOG__/, 'helpers.ts still uses legacy debug logger global')
    assert.doesNotMatch(helpersFile, /__GASOLINE_REGISTRY_DEBUG__/, 'helpers.ts still uses legacy registry debug flag')
    assert.match(helpersFile, /__KABOOM_DEBUG_LOG__/, 'helpers.ts should use __KABOOM_DEBUG_LOG__')
    assert.match(helpersFile, /__KABOOM_REGISTRY_DEBUG__/, 'helpers.ts should use __KABOOM_REGISTRY_DEBUG__')
  })
})
