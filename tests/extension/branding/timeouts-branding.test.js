// @ts-nocheck
import { beforeEach, describe, test } from 'node:test'
import assert from 'node:assert'

describe('timeout branding contracts', () => {
  beforeEach(() => {
    delete globalThis.KABOOM_TEST_TIMEOUT_SCALE
    delete globalThis.GASOLINE_TEST_TIMEOUT_SCALE
  })

  test('scaleTimeout reads Kaboom test timeout scale from global state', async () => {
    globalThis.KABOOM_TEST_TIMEOUT_SCALE = 0.25

    const { scaleTimeout } = await import('../../../extension/lib/timeouts.js')

    assert.strictEqual(scaleTimeout(200), 50)
  })

  test('scaleTimeout ignores legacy Kaboom timeout scale globals', async () => {
    globalThis.GASOLINE_TEST_TIMEOUT_SCALE = 0.25

    const { scaleTimeout } = await import('../../../extension/lib/timeouts.js')

    assert.strictEqual(scaleTimeout(200), 200)
  })
})
