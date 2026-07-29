// @ts-nocheck
import { afterEach, beforeEach, describe, mock, test } from 'node:test'
import assert from 'node:assert'

describe('withTimeoutAndCleanup', () => {
  let originalSetTimeout
  let originalClearTimeout

  beforeEach(() => {
    originalSetTimeout = globalThis.setTimeout
    originalClearTimeout = globalThis.clearTimeout
  })

  afterEach(() => {
    globalThis.setTimeout = originalSetTimeout
    globalThis.clearTimeout = originalClearTimeout
  })

  test('clears the timeout when the wrapped promise resolves first', async () => {
    let timeoutId = 0
    globalThis.setTimeout = mock.fn(() => ({ id: ++timeoutId }))
    globalThis.clearTimeout = mock.fn()

    const { withTimeoutAndCleanup } = await import(`../../../extension/lib/timeout-utils.js?t=${Date.now()}_${Math.random()}`)

    const result = await withTimeoutAndCleanup(Promise.resolve('ok'), 5000)

    assert.strictEqual(result, 'ok')
    assert.strictEqual(globalThis.setTimeout.mock.calls.length, 1)
    assert.strictEqual(globalThis.clearTimeout.mock.calls.length, 1)
    assert.deepStrictEqual(globalThis.clearTimeout.mock.calls[0].arguments[0], { id: 1 })
  })
})
