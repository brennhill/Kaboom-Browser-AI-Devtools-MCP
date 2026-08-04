// @ts-nocheck
import { test, describe, mock, beforeEach } from 'node:test'
import assert from 'node:assert'
import { createMockChrome } from '../shared/helpers.js'

import { initTabTracking } from '../../../extension/content/tab-tracking.js'
import { initWindowMessageListener } from '../../../extension/content/window-message-listener.js'
import {
  initRequestTracking,
  cleanupRequestTracking,
  registerDomRequest,
  resolveDomRequest
} from '../../../extension/content/request-tracking.js'
import { MESSAGE_MAP, safeSendMessage } from '../../../extension/content/message-forwarding.js'
import { getPageNonce } from '../../../extension/content/script-injection.js'

describe('Content Window Message Bridge', () => {
  let messageHandler
  let runtimeSendMessage

  beforeEach(() => {
    messageHandler = undefined

    runtimeSendMessage = mock.fn((msg) => {
      if (msg?.type === 'get_tab_id') return Promise.resolve({ tabId: 42 })
      return Promise.resolve()
    })

    globalThis.chrome = createMockChrome()
    globalThis.chrome.runtime.sendMessage = runtimeSendMessage
    globalThis.chrome.storage.local.get = mock.fn(() => Promise.resolve({ trackedTabId: 42 }))

    globalThis.window = {
      location: { origin: 'http://localhost:3000' },
      addEventListener: mock.fn((type, handler) => {
        if (type === 'message') messageHandler = handler
      }),
      removeEventListener: mock.fn(),
      postMessage: mock.fn()
    }

    globalThis.document = {
      addEventListener: mock.fn(),
      removeEventListener: mock.fn(),
      readyState: 'complete',
      head: { appendChild: mock.fn() },
      documentElement: { appendChild: mock.fn() },
      createElement: mock.fn(() => ({ remove: mock.fn() })),
      querySelector: mock.fn(() => null),
      querySelectorAll: mock.fn(() => [])
    }
  })

  test('MESSAGE_MAP contains expected forwarding contracts', () => {
    assert.strictEqual(MESSAGE_MAP.kaboom_log, 'log')
    assert.strictEqual(MESSAGE_MAP.kaboom_ws, 'ws_event')
    assert.strictEqual(MESSAGE_MAP.kaboom_network_body, 'network_body')
    assert.strictEqual(MESSAGE_MAP.kaboom_enhanced_action, 'enhanced_action')
    assert.strictEqual(MESSAGE_MAP.kaboom_performance_snapshot, 'performance_snapshot')
  })

  test('request tracking owns cancellable per-request expiry without a shared interval', () => {
	const originalSetInterval = globalThis.setInterval
	const originalSetTimeout = globalThis.setTimeout
	const originalClearTimeout = globalThis.clearTimeout
	const setIntervalCalls = []
	const timeoutCallbacks = new Map()
	const cleared = []
	globalThis.setInterval = (...args) => {
		setIntervalCalls.push(args)
		return 99
	}
	globalThis.setTimeout = (callback) => {
		timeoutCallbacks.set(42, callback)
		return 42
	}
	globalThis.clearTimeout = (id) => cleared.push(id)
	try {
		initRequestTracking()
		let resolved
		const requestId = registerDomRequest((result) => { resolved = result }, 1000, () => assert.fail('resolved request timed out'))
      resolveDomRequest(requestId, { matches: [] })
      assert.deepStrictEqual(resolved, { matches: [] })
      assert.deepStrictEqual(cleared, [42])
      assert.strictEqual(setIntervalCalls.length, 0)

      let cancelled = false
      registerDomRequest(
        () => {},
        1000,
        () => {
          cancelled = true
        }
      )
      cleanupRequestTracking()
      assert.strictEqual(cancelled, true)
      assert.deepStrictEqual(cleared, [42, 42])
	} finally {
		cleanupRequestTracking()
		globalThis.setInterval = originalSetInterval
		globalThis.setTimeout = originalSetTimeout
		globalThis.clearTimeout = originalClearTimeout
	}
  })

  test('forwards GASOLINE_NETWORK_BODY from tracked tab through runtime.sendMessage', async () => {
    await initTabTracking()
    initWindowMessageListener()

    assert.ok(messageHandler, 'message listener should be installed')

    const payload = { method: 'GET', url: 'https://api.example.com/users', status: 200 }
    messageHandler({
      source: globalThis.window,
      origin: globalThis.window.location.origin,
      data: { type: 'kaboom_network_body', _nonce: getPageNonce(), payload }
    })

    const forwarded = runtimeSendMessage.mock.calls
      .map((c) => c.arguments[0])
      .find((msg) => msg?.type === 'network_body')

    assert.ok(forwarded, 'expected forwarded network_body message')
    assert.deepStrictEqual(forwarded.payload, payload)
    assert.strictEqual(forwarded.tabId, 42)
  })

  test('rejects forged telemetry with missing or incorrect page nonce', async () => {
    await initTabTracking()
    initWindowMessageListener()

    for (const nonce of [undefined, 'wrong-page-nonce']) {
      messageHandler({
        source: globalThis.window,
        origin: globalThis.window.location.origin,
        data: { type: 'kaboom_log', ...(nonce ? { _nonce: nonce } : {}), payload: { message: 'forged' } }
      })
    }

    const forwarded = runtimeSendMessage.mock.calls.filter((call) => call.arguments[0]?.type === 'log')
    assert.strictEqual(forwarded.length, 0)
  })

  test('rejects oversized and deeply nested telemetry with bounded redacted diagnostics', async () => {
    await initTabTracking()
    initWindowMessageListener()
    const secret = 'private-page-value'
    const deep = { value: secret }
    let cursor = deep
    for (let depth = 0; depth < 20; depth++) {
      cursor.next = { value: depth }
      cursor = cursor.next
    }

    for (const payload of [
      { ts: '2026-08-04T00:00:00Z', level: 'error', message: secret.repeat(300_000) },
      { ts: '2026-08-04T00:00:00Z', level: 'error', details: deep }
    ]) {
      messageHandler({
        source: globalThis.window,
        origin: globalThis.window.location.origin,
        data: { type: 'kaboom_log', _nonce: getPageNonce(), payload }
      })
    }

    const forwarded = runtimeSendMessage.mock.calls.map((call) => call.arguments[0])
    assert.strictEqual(forwarded.filter((message) => message?.type === 'log').length, 0)
    const diagnostics = forwarded.filter((message) => message?.type === 'capture_diagnostic')
    assert.strictEqual(diagnostics.length, 2)
    assert.ok(diagnostics.every((message) => !JSON.stringify(message).includes(secret)))
  })

  test('drops captured events when tab is not tracked', async () => {
    globalThis.chrome.storage.local.get = mock.fn(() => Promise.resolve({ trackedTabId: 999 }))

    await initTabTracking()
    initWindowMessageListener()

    messageHandler({
      source: globalThis.window,
      origin: globalThis.window.location.origin,
      data: { type: 'kaboom_log', payload: { level: 'error', message: 'boom' } }
    })

    const forwardedCount = runtimeSendMessage.mock.calls
      .map((c) => c.arguments[0])
      .filter((msg) => msg?.type === 'log').length

    assert.strictEqual(forwardedCount, 0)
  })

  test('resolves real pending DOM request on kaboom_dom_query_RESPONSE', async () => {
    await initTabTracking()
    initWindowMessageListener()

    const expected = { matchCount: 1, matches: [{ tag: 'button' }] }
    let resolved
    const requestId = registerDomRequest((result) => {
      resolved = result
    })

    messageHandler({
      source: globalThis.window,
      origin: globalThis.window.location.origin,
      data: {
        type: 'kaboom_dom_query_response',
        requestId,
        _nonce: getPageNonce(),
        result: expected
      }
    })

    assert.deepStrictEqual(resolved, expected)
  })

  test('extension reload warning uses Kaboom copy and suppresses repeat sends after invalidation', () => {
    const warn = mock.method(console, 'warn', () => {})
    globalThis.chrome.runtime.sendMessage = mock.fn(() => {
      throw new Error('Extension context invalidated')
    })

    safeSendMessage({ type: 'log', payload: { level: 'warn', message: 'first' } })
    safeSendMessage({ type: 'log', payload: { level: 'warn', message: 'second' } })

    assert.strictEqual(globalThis.chrome.runtime.sendMessage.mock.calls.length, 1)
    assert.strictEqual(warn.mock.calls.length, 1)
    const message = warn.mock.calls[0].arguments[0]
    assert.match(message, /KaBOOM! extension was reloaded/)
    assert.match(message, /A page refresh will reconnect capture automatically/)
    assert.doesNotMatch(message, /Gasoline extension was reloaded|STRUM extension was reloaded/)
  })
})
