// dom-dispatch-structured.test.js — Regression for get_text structured passthrough (#390).

import { beforeEach, describe, mock, test } from 'node:test'
import assert from 'node:assert'

const executeScriptMock = mock.fn(async () => [{ frameId: 0, result: { success: true, action: 'get_text', selector: '.accordion', value: 'ok' } }])

if (typeof globalThis.chrome === 'undefined') {
  globalThis.chrome = {}
}
globalThis.chrome.scripting = { executeScript: executeScriptMock }
globalThis.chrome.tabs = {
  get: async () => ({ url: 'https://example.com', title: 'Example' })
}
globalThis.chrome.debugger = {
  sendCommand: async () => ({}),
  attach: async () => {},
  detach: async () => {}
}

const { executeDOMAction } = await import('../dom-dispatch.js')

describe('executeDOMAction structured passthrough', () => {
  beforeEach(() => {
    executeScriptMock.mock.resetCalls()
  })

  test('forwards structured=true to domPrimitive options for get_text', async () => {
    let asyncResult = null
    const sendAsyncResult = (_syncClient, _id, _corr, status, result, error) => {
      asyncResult = { status, result, error }
    }

    const query = {
      id: 'q-structured-1',
      correlation_id: 'dom_get_text_structured_1',
      params: JSON.stringify({
        action: 'get_text',
        selector: '.accordion',
        structured: true
      })
    }

    await executeDOMAction(query, 42, {}, sendAsyncResult, () => {})

    assert.strictEqual(executeScriptMock.mock.calls.length, 1, 'expected one executeScript call')
    const call = executeScriptMock.mock.calls[0]
    const injectedArgs = call.arguments[0].args
    assert.strictEqual(injectedArgs[0], 'get_text')
    assert.strictEqual(injectedArgs[1], '.accordion')
    assert.strictEqual(injectedArgs[2].structured, true, 'structured flag should be forwarded to domPrimitive')

    assert.ok(asyncResult, 'sendAsyncResult should be called')
    assert.strictEqual(asyncResult.status, 'complete')
    assert.strictEqual(asyncResult.error, undefined)
  })
})

