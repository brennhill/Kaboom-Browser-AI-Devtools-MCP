// @ts-nocheck
/**
 * @fileoverview interact-subtitle.test.js — Guards the subtitle command's
 * fail-loud contract (F9): it must report the real send result, not a phantom
 * success when the content script is unreachable.
 */

import { beforeEach, describe, test, mock } from 'node:test'
import assert from 'node:assert'

const registered = new Map()
mock.module('../../../extension/background/commands/registry.js', {
  namedExports: {
    registerCommand: mock.fn((name, handler) => {
      registered.set(name, handler)
    }),
    // Other modules transitively import `dispatch`; provide a stub so mocking the
    // whole registry module does not strip exports they depend on at import time.
    dispatch: mock.fn(async () => {})
  }
})

function makeStorageArea() {
  return {
    get: mock.fn((_k, cb) => (cb ? cb({}) : Promise.resolve({}))),
    set: mock.fn((_d, cb) => (cb ? cb() : Promise.resolve())),
    remove: mock.fn((_k, cb) => (cb ? cb() : Promise.resolve()))
  }
}

globalThis.chrome = {
  tabs: { sendMessage: mock.fn(async () => undefined) },
  runtime: { id: 'test-ext', lastError: null },
  storage: {
    local: makeStorageArea(),
    session: makeStorageArea(),
    onChanged: { addListener: mock.fn() }
  }
}

await import('../../../extension/background/commands/interact.js')

describe('interact subtitle command (F9 fail-loud)', () => {
  beforeEach(() => {
    globalThis.chrome.tabs.sendMessage = mock.fn(async () => undefined)
  })

  test('reports success when the content script accepts the subtitle', async () => {
    const handler = registered.get('subtitle')
    assert.ok(handler, 'subtitle handler should be registered')
    const sendResult = mock.fn()
    await handler({ tabId: 5, params: { text: 'hello' }, sendResult })
    assert.strictEqual(sendResult.mock.calls.length, 1)
    assert.deepStrictEqual(sendResult.mock.calls[0].arguments[0], { success: true, subtitle: 'hello' })
  })

  test('reports failure (not phantom success) when the content script is unreachable', async () => {
    const handler = registered.get('subtitle')
    globalThis.chrome.tabs.sendMessage = mock.fn(async () => {
      throw new Error('Could not establish connection. Receiving end does not exist.')
    })
    const sendResult = mock.fn()
    await handler({ tabId: 5, params: { text: 'hi' }, sendResult })
    assert.strictEqual(sendResult.mock.calls.length, 1)
    const payload = sendResult.mock.calls[0].arguments[0]
    assert.strictEqual(payload.success, false)
    assert.strictEqual(payload.subtitle, 'hi')
    assert.ok(String(payload.error).length > 0, 'should carry a real error string')
  })

  test('labels a cleared subtitle when text is empty', async () => {
    const handler = registered.get('subtitle')
    const sendResult = mock.fn()
    await handler({ tabId: 5, params: {}, sendResult })
    assert.deepStrictEqual(sendResult.mock.calls[0].arguments[0], { success: true, subtitle: 'cleared' })
  })
})
