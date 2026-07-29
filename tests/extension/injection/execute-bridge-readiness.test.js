// @ts-nocheck
/**
 * @fileoverview execute-bridge-readiness.test.js — execute_js readiness gate behavior.
 */

import { beforeEach, afterEach, describe, test, mock } from 'node:test'
import assert from 'node:assert'

const mockEnsureInjectBridgeReady = mock.fn(async () => false)
const mockIsInjectScriptLoaded = mock.fn(() => false)
const mockGetPageNonce = mock.fn(() => 'test-nonce')

mock.module('../../../extension/content/script-injection.js', {
  namedExports: {
    ensureInjectBridgeReady: mockEnsureInjectBridgeReady,
    isInjectScriptLoaded: mockIsInjectScriptLoaded,
    getPageNonce: mockGetPageNonce
  }
})

const { handleExecuteJs } = await import('../../../extension/content/message-handlers.js')

describe('handleExecuteJs bridge readiness', () => {
  let originalSetTimeout

  beforeEach(() => {
    mock.reset()
    globalThis.window = {
      location: { origin: 'http://localhost:3000' },
      postMessage: mock.fn(),
      addEventListener: mock.fn(),
      removeEventListener: mock.fn()
    }

    originalSetTimeout = globalThis.setTimeout
    globalThis.setTimeout = mock.fn(() => 1)
  })

  afterEach(() => {
    globalThis.setTimeout = originalSetTimeout
  })

  test('returns inject_not_loaded when bridge is not ready and inject is absent', async () => {
    mockEnsureInjectBridgeReady.mock.mockImplementation(async () => false)
    mockIsInjectScriptLoaded.mock.mockImplementation(() => false)

    const sendResponse = mock.fn()
    const keepOpen = handleExecuteJs({ script: '1+1', timeout_ms: 1000 }, sendResponse)

    assert.strictEqual(keepOpen, true)
    await Promise.resolve()

    assert.strictEqual(sendResponse.mock.calls.length, 1)
    const payload = sendResponse.mock.calls[0].arguments[0]
    assert.strictEqual(payload.success, false)
    assert.strictEqual(payload.error, 'inject_not_loaded')
    assert.strictEqual(globalThis.window.postMessage.mock.calls.length, 0)
  })

  test('returns inject_not_responding when bridge is not ready but inject is loaded', async () => {
    mockEnsureInjectBridgeReady.mock.mockImplementation(async () => false)
    mockIsInjectScriptLoaded.mock.mockImplementation(() => true)

    const sendResponse = mock.fn()
    handleExecuteJs({ script: '1+1', timeout_ms: 1200 }, sendResponse)
    await Promise.resolve()

    assert.strictEqual(sendResponse.mock.calls.length, 1)
    const payload = sendResponse.mock.calls[0].arguments[0]
    assert.strictEqual(payload.success, false)
    assert.strictEqual(payload.error, 'inject_not_responding')
    assert.strictEqual(globalThis.window.postMessage.mock.calls.length, 0)
  })

  test('dispatches kaboom_execute_js only after bridge readiness succeeds', async () => {
    mockEnsureInjectBridgeReady.mock.mockImplementation(async () => true)
    mockIsInjectScriptLoaded.mock.mockImplementation(() => true)

    const sendResponse = mock.fn()
    handleExecuteJs({ script: '1+1', timeout_ms: 900 }, sendResponse)
    await Promise.resolve()

    assert.strictEqual(sendResponse.mock.calls.length, 0, 'should not respond before inject result/timeout')
    assert.strictEqual(globalThis.window.postMessage.mock.calls.length, 1)
    const [posted, origin] = globalThis.window.postMessage.mock.calls[0].arguments
    assert.strictEqual(posted.type, 'kaboom_execute_js')
    assert.strictEqual(posted.script, '1+1')
    assert.strictEqual(posted.timeoutMs, 900)
    assert.ok(typeof posted.requestId === 'number')
    assert.strictEqual(posted._nonce, 'test-nonce')
    assert.strictEqual(origin, 'http://localhost:3000')
  })
})
