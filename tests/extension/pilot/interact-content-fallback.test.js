// @ts-nocheck
/**
 * @fileoverview interact-content-fallback.test.js — Guards content extractor fallback
 * behavior when content scripts are unreachable.
 */

import { beforeEach, describe, test, mock } from 'node:test'
import assert from 'node:assert'

const registered = new Map()
const mockRegisterCommand = mock.fn((name, handler) => {
  registered.set(name, handler)
})

mock.module('../../../extension/background/commands/registry.js', {
  namedExports: {
    registerCommand: mockRegisterCommand
  }
})

globalThis.chrome = {
  tabs: {
    sendMessage: mock.fn()
  },
  scripting: {
    executeScript: mock.fn()
  }
}

await import('../../../extension/background/commands/interact-content.js')

describe('interact content extraction fallback', () => {
  beforeEach(() => {
    mock.reset()
  })

  test('get_readable falls back to executeScript when content script is unreachable', async () => {
    const handler = registered.get('get_readable')
    assert.ok(handler, 'get_readable handler should be registered')

    globalThis.chrome.tabs.sendMessage.mock.mockImplementationOnce(async () => {
      throw new Error('Could not establish connection. Receiving end does not exist.')
    })
    globalThis.chrome.scripting.executeScript.mock.mockImplementationOnce(async () => [
      { result: { title: 'Example', content: 'Readable body', fallback: true } }
    ])

    const sendResult = mock.fn()
    await handler({
      tabId: 77,
      query: { id: 'q1', params: { include_meta: true } },
      sendResult
    })

    assert.strictEqual(globalThis.chrome.tabs.sendMessage.mock.calls.length, 1)
    assert.strictEqual(globalThis.chrome.scripting.executeScript.mock.calls.length, 1)
    assert.strictEqual(sendResult.mock.calls.length, 1)
    const payload = sendResult.mock.calls[0].arguments[0]
    assert.strictEqual(payload.title, 'Example')
    assert.strictEqual(payload.content, 'Readable body')
    assert.strictEqual(payload.fallback, true)
    // The fallback is a self-contained injected function with no provenance observer behind it.
    // An absent provenance block would read as a clean first-party page (kaboom-x0li.3).
    assert.strictEqual(payload.provenance.attribution_available, false)
    assert.deepStrictEqual(payload.provenance.regions, [])
    assert.ok(payload.provenance.notes.some((note) => note.includes('content_script_not_loaded')))
  })

  test('get_readable returns structured guidance when fallback injection also fails', async () => {
    const handler = registered.get('get_readable')
    assert.ok(handler, 'get_readable handler should be registered')

    globalThis.chrome.tabs.sendMessage.mock.mockImplementationOnce(async () => {
      throw new Error('Receiving end does not exist.')
    })
    globalThis.chrome.scripting.executeScript.mock.mockImplementationOnce(async () => {
      throw new Error('Cannot access page')
    })

    const sendResult = mock.fn()
    await handler({
      tabId: 77,
      query: { id: 'q2', params: {} },
      sendResult
    })

    assert.strictEqual(sendResult.mock.calls.length, 1)
    const payload = sendResult.mock.calls[0].arguments[0]
    assert.strictEqual(payload.error, 'get_readable_failed')
    assert.ok(String(payload.message).includes('fallback injection failed'))
    assert.ok(String(payload.message).includes('Refresh the page first'))
  })
})
