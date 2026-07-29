// @ts-nocheck
/**
 * @fileoverview Sync-client polling, flush, overrides, recovery, reset, and logging tests.
 */

import { test, describe, mock, beforeEach, afterEach } from 'node:test'
import assert from 'node:assert'
import {
  SyncClient,
  createMockCallbacks,
  installFetchMock,
  makeSyncResponse,
  tick,
} from './sync-client-fixture.js'

describe('SyncClient — Polling loop behavior', () => {
  let client
  let callbacks

  beforeEach(() => {
    mock.reset()
    callbacks = createMockCallbacks()
  })

  afterEach(() => {
    client.stop()
  })

  test('should use next_poll_ms from server response for next sync', async () => {
    // Server says poll again in 50ms
    const mockFetch = installFetchMock(makeSyncResponse({ next_poll_ms: 50 }))

    client = new SyncClient('http://localhost:7777', 'sess-1', callbacks)
    client.start()

    // Wait enough for initial sync + one retry at ~50ms
    await tick(150)

    assert.ok(mockFetch.mock.calls.length >= 2, `Expected >=2 calls, got ${mockFetch.mock.calls.length}`)
  })

  test('should default to BASE_POLL_MS (1000) when next_poll_ms is 0', async () => {
    const mockFetch = installFetchMock(makeSyncResponse({ next_poll_ms: 0 }))

    client = new SyncClient('http://localhost:7777', 'sess-1', callbacks)
    client.start()

    await tick(50) // first sync
    const afterFirst = mockFetch.mock.calls.length

    await tick(500) // should NOT have fired another sync yet (1000ms delay)
    assert.strictEqual(mockFetch.mock.calls.length, afterFirst)
  })

  test('should not schedule sync after stop', async () => {
    installFetchMock(makeSyncResponse({ next_poll_ms: 10 }))
    client = new SyncClient('http://localhost:7777', 'sess-1', callbacks)

    client.start()
    await tick(30)
    client.stop()

    const countAfterStop = globalThis.fetch.mock.calls.length
    await tick(100)

    assert.strictEqual(globalThis.fetch.mock.calls.length, countAfterStop)
  })
})

describe('SyncClient — Flush behavior', () => {
  let client
  let callbacks

  beforeEach(() => {
    mock.reset()
    callbacks = createMockCallbacks()
  })

  afterEach(() => {
    client.stop()
  })

  test('flush should trigger immediate sync', async () => {
    const mockFetch = installFetchMock(makeSyncResponse({ next_poll_ms: 60000 }))

    client = new SyncClient('http://localhost:7777', 'sess-1', callbacks)
    client.start()
    await tick(50) // first sync

    const callsBefore = mockFetch.mock.calls.length
    client.flush()
    await tick(50)

    assert.ok(mockFetch.mock.calls.length > callsBefore, 'Flush should have triggered another sync')
  })

  test('flush should be a no-op when client is not running', async () => {
    const mockFetch = installFetchMock()
    client = new SyncClient('http://localhost:7777', 'sess-1', callbacks)

    client.flush()
    await tick(50)

    assert.strictEqual(mockFetch.mock.calls.length, 0)
  })
})

describe('SyncClient — Capture overrides', () => {
  let client
  let callbacks

  beforeEach(() => {
    mock.reset()
    callbacks = createMockCallbacks()
  })

  afterEach(() => {
    client.stop()
  })

  test('should forward capture_overrides to callback', async () => {
    const overrides = { capture_logs: 'true', capture_network: 'false' }
    installFetchMock(makeSyncResponse({ capture_overrides: overrides, next_poll_ms: 60000 }))

    client = new SyncClient('http://localhost:7777', 'sess-1', callbacks)
    client.start()
    await tick(50)

    assert.strictEqual(callbacks.onCaptureOverrides.mock.calls.length, 1)
    assert.deepStrictEqual(callbacks.onCaptureOverrides.mock.calls[0].arguments[0], overrides)
  })

  test('should not call onCaptureOverrides when absent from response', async () => {
    installFetchMock(makeSyncResponse({ next_poll_ms: 60000 }))

    client = new SyncClient('http://localhost:7777', 'sess-1', callbacks)
    client.start()
    await tick(50)

    assert.strictEqual(callbacks.onCaptureOverrides.mock.calls.length, 0)
  })
})

describe('SyncClient — Extension log acknowledgement', () => {
  let client
  let callbacks

  beforeEach(() => {
    mock.reset()
    callbacks = createMockCallbacks()
  })

  afterEach(() => {
    client.stop()
  })

  test('should acknowledge only extension logs included in a successful sync', async () => {
    callbacks.getExtensionLogs = mock.fn(() => [{ timestamp: 'now', level: 'info', message: 'x', source: 'bg', category: 'sync' }])
    installFetchMock(makeSyncResponse({ next_poll_ms: 60000 }))

    client = new SyncClient('http://localhost:7777', 'sess-1', callbacks)
    client.start()
    await tick(50)

    assert.strictEqual(callbacks.acknowledgeExtensionLogs.mock.calls.length, 1)
    assert.strictEqual(callbacks.acknowledgeExtensionLogs.mock.calls[0].arguments[0], 1)
  })

  test('should NOT clear extension logs when none were sent', async () => {
    callbacks.getExtensionLogs = mock.fn(() => [])
    installFetchMock(makeSyncResponse({ next_poll_ms: 60000 }))

    client = new SyncClient('http://localhost:7777', 'sess-1', callbacks)
    client.start()
    await tick(50)

    assert.strictEqual(callbacks.acknowledgeExtensionLogs.mock.calls.length, 0)
  })
})

describe('SyncClient — Error recovery', () => {
  let client
  let callbacks

  beforeEach(() => {
    mock.reset()
    callbacks = createMockCallbacks()
  })

  afterEach(() => {
    client.stop()
  })

  test('should recover from network error and reconnect', async () => {
    let callCount = 0
    globalThis.fetch = mock.fn(() => {
      callCount++
      if (callCount <= 2) {
        return Promise.reject(new Error('ECONNREFUSED'))
      }
      return Promise.resolve({
        ok: true,
        json: () => Promise.resolve(makeSyncResponse({ next_poll_ms: 60000 }))
      })
    })

    client = new SyncClient('http://localhost:7777', 'sess-1', callbacks)
    client.start()

    // Wait for failures + recovery
    await tick(4000)

    assert.strictEqual(client.isConnected(), true)
    assert.strictEqual(client.getState().consecutiveFailures, 0)
  })

  test('should handle AbortController timeout gracefully', async () => {
    // Simulate fetch that hangs beyond the 8s timeout
    globalThis.fetch = mock.fn((_url, opts) => {
      return new Promise((_resolve, reject) => {
        if (opts.signal) {
          opts.signal.addEventListener('abort', () => {
            reject(new DOMException('The operation was aborted.', 'AbortError'))
          })
        }
      })
    })

    client = new SyncClient('http://localhost:7777', 'sess-1', callbacks)
    client.start()

    // Wait for the 8s timeout + a bit
    await tick(8500)

    assert.strictEqual(client.isConnected(), false)
    assert.ok(client.getState().consecutiveFailures >= 1)
  })

  test('should handle malformed JSON response gracefully', async () => {
    globalThis.fetch = mock.fn(() =>
      Promise.resolve({
        ok: true,
        json: () => Promise.reject(new SyntaxError('Unexpected token'))
      })
    )

    client = new SyncClient('http://localhost:7777', 'sess-1', callbacks)
    client.start()
    await tick(50)

    assert.strictEqual(client.isConnected(), false)
    assert.strictEqual(client.getState().consecutiveFailures, 1)
  })
})

describe('SyncClient — resetConnection', () => {
  let client
  let callbacks

  beforeEach(() => {
    mock.reset()
    callbacks = createMockCallbacks()
  })

  afterEach(() => {
    client.stop()
  })

  test('should reset consecutiveFailures to zero', async () => {
    globalThis.fetch = mock.fn(() => Promise.reject(new Error('fail')))

    client = new SyncClient('http://localhost:7777', 'sess-1', callbacks)
    client.start()
    await tick(50)

    assert.ok(client.getState().consecutiveFailures >= 1)

    client.resetConnection()
    assert.strictEqual(client.getState().consecutiveFailures, 0)
  })

  test('should trigger immediate re-sync when running', async () => {
    const mockFetch = installFetchMock(makeSyncResponse({ next_poll_ms: 60000 }))
    client = new SyncClient('http://localhost:7777', 'sess-1', callbacks)
    client.start()
    await tick(50)

    const callsBefore = mockFetch.mock.calls.length
    client.resetConnection()
    await tick(50)

    assert.ok(mockFetch.mock.calls.length > callsBefore, 'resetConnection should trigger another sync')
  })
})

describe('SyncClient — setServerUrl', () => {
  test('should update the server URL for subsequent requests', async () => {
    const callbacks = createMockCallbacks()
    const mockFetch = installFetchMock(makeSyncResponse({ next_poll_ms: 60000 }))

    const client = new SyncClient('http://localhost:7777', 'sess-1', callbacks)
    client.setServerUrl('http://localhost:9999')
    client.start()
    await tick(50)

    const url = mockFetch.mock.calls[0].arguments[0]
    assert.strictEqual(url, 'http://localhost:9999/sync')

    client.stop()
  })
})

describe('SyncClient — Debug logging', () => {
  test('should use debugLog callback when provided', async () => {
    const callbacks = createMockCallbacks()
    installFetchMock(makeSyncResponse({ next_poll_ms: 60000 }))

    const client = new SyncClient('http://localhost:7777', 'sess-1', callbacks)
    client.start()
    await tick(50)

    const syncLogs = callbacks.debugLog.mock.calls.filter((c) => c.arguments[0] === 'sync')
    assert.ok(syncLogs.length > 0, 'Should have logged with category "sync"')

    client.stop()
  })

  test('should fall back to console.log when debugLog is not provided', async () => {
    const callbacks = createMockCallbacks()
    delete callbacks.debugLog

    installFetchMock(makeSyncResponse({ next_poll_ms: 60000 }))

    // Mock console.log to capture
    const origLog = console.log
    const logCalls = []
    console.log = (...args) => logCalls.push(args)

    const client = new SyncClient('http://localhost:7777', 'sess-1', callbacks)
    client.start()
    await tick(50)

    console.log = origLog

    const syncLogs = logCalls.filter((args) => typeof args[0] === 'string' && args[0].includes('[SyncClient]'))
    assert.ok(syncLogs.length > 0, 'Should have logged via console.log')

    client.stop()
  })
})
