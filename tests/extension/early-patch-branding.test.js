// @ts-nocheck
import { test, describe } from 'node:test'
import assert from 'node:assert'
import { readFileSync } from 'node:fs'

const EARLY_PATCH_FILES = [
  'src/early-patch.ts',
  'src/lib/net/network.ts',
  'src/lib/net/websocket.ts',
  'src/inject/observers.ts',
  'src/types/global.d.ts'
]

describe('Early-patch branding contracts', () => {
  test('early-patch stash names use Kaboom globals', () => {
    for (const relativePath of EARLY_PATCH_FILES) {
      const contents = readFileSync(relativePath, 'utf8')

      assert.doesNotMatch(
        contents,
        /__GASOLINE_(ORIGINAL|EARLY)_[A-Z_]+__/,
        `${relativePath} still references legacy __GASOLINE_* stash names`
      )
      assert.doesNotMatch(
        contents,
        /__gasolineEarly(?:Method|Url)/,
        `${relativePath} still references legacy __gasolineEarly* XHR stash names`
      )
    }

    const earlyPatch = readFileSync('src/early-patch.ts', 'utf8')
    const globals = readFileSync('src/types/global.d.ts', 'utf8')

    assert.match(earlyPatch, /__KABOOM_ORIGINAL_WS__/)
    assert.match(earlyPatch, /__KABOOM_EARLY_WS__/)
    assert.match(earlyPatch, /__KABOOM_ORIGINAL_FETCH__/)
    assert.match(earlyPatch, /__KABOOM_ORIGINAL_XHR_OPEN__/)
    assert.match(earlyPatch, /__KABOOM_ORIGINAL_XHR_SEND__/)
    assert.match(earlyPatch, /__KABOOM_EARLY_BODIES__/)
    assert.match(earlyPatch, /__kaboomEarlyMethod/)
    assert.match(earlyPatch, /__kaboomEarlyUrl/)

    assert.match(globals, /__KABOOM_ORIGINAL_WS__/)
    assert.match(globals, /__KABOOM_EARLY_WS__/)
    assert.match(globals, /__KABOOM_ORIGINAL_FETCH__/)
    assert.match(globals, /__KABOOM_ORIGINAL_XHR_OPEN__/)
    assert.match(globals, /__KABOOM_ORIGINAL_XHR_SEND__/)
    assert.match(globals, /__KABOOM_EARLY_BODIES__/)
  })
})
