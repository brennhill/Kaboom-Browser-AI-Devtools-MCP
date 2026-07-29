// @ts-nocheck
/**
 * @fileoverview Sync-client construction, lifecycle, connection, retry, and request tests.
 */

import { test, describe, mock, beforeEach, afterEach } from 'node:test'
import assert from 'node:assert'
import {
  SyncClient,
  createMockCallbacks,
  createSyncClient,
  installFetchMock,
  makeSyncResponse,
  tick,
} from './sync-client-fixture.js'

describe('SyncClient — Construction and Initialization', () => {
  beforeEach(() => mock.reset())

  test('should construct with required parameters', () => {
    const cb = createMockCallbacks()
    const client = new SyncClient('http://localhost:7777', 'sess-1', cb)

    assert.ok(client instanceof SyncClient)
    assert.strictEqual(client.isConnected(), false)
  })

  test('should construct with extensionVersion parameter', () => {
    const cb = createMockCallbacks()
    const client = new SyncClient('http://localhost:7777', 'sess-1', cb, '6.0.3')

    const state = client.getState()
    assert.strictEqual(state.connected, false)
    assert.strictEqual(state.consecutiveFailures, 0)
    assert.strictEqual(state.lastSyncAt, 0)
    assert.strictEqual(state.lastCommandAck, null)
  })

  test('createSyncClient factory returns SyncClient instance', () => {
    const cb = createMockCallbacks()
    const client = createSyncClient('http://localhost:7777', 'sess-1', cb, '6.0.3')

    assert.ok(client instanceof SyncClient)
  })

  test('getState returns an immutable copy', () => {
    const cb = createMockCallbacks()
    const client = new SyncClient('http://localhost:7777', 'sess-1', cb)

    const state1 = client.getState()
    state1.connected = true
    state1.consecutiveFailures = 999

    const state2 = client.getState()
    assert.strictEqual(state2.connected, false)
    assert.strictEqual(state2.consecutiveFailures, 0)
  })
})

describe('SyncClient — Start / Stop lifecycle', () => {
  let client
  let callbacks

  beforeEach(() => {
    mock.reset()
    callbacks = createMockCallbacks()
    installFetchMock()
    client = new SyncClient('http://localhost:7777', 'sess-1', callbacks)
  })

  afterEach(() => {
    client.stop()
  })

  test('start should begin the sync loop', async () => {
    client.start()
    await tick(50)

    // Fetch should have been called at least once (immediate sync on start)
    assert.ok(globalThis.fetch.mock.calls.length >= 1, 'fetch should have been called after start')
  })

  test('start should be idempotent — calling twice does not double-poll', async () => {
    client.start()
    client.start() // should no-op

    await tick(50)

    // Should not have extra syncs — one start schedules one initial sync
    const callCount = globalThis.fetch.mock.calls.length
    assert.ok(callCount >= 1 && callCount <= 3, `Expected 1-3 fetch calls, got ${callCount}`)
  })

  test('stop should halt the sync loop', async () => {
    client.start()
    await tick(30)

    const callsBeforeStop = globalThis.fetch.mock.calls.length
    client.stop()

    await tick(100)
    const callsAfterStop = globalThis.fetch.mock.calls.length

    // No more fetch calls after stop
    assert.strictEqual(callsAfterStop, callsBeforeStop)
  })

  test('stop should clear the interval', () => {
    client.start()
    client.stop()

    // Calling stop should log 'Stopped sync client'
    const debugCalls = callbacks.debugLog.mock.calls
    const stopLog = debugCalls.find((c) => c.arguments[1] === 'Stopped sync client')
    assert.ok(stopLog, 'Should log stop message')
  })
})

describe('SyncClient — Connection state transitions', () => {
  let client
  let callbacks

  beforeEach(() => {
    mock.reset()
    mock.method(Math, 'random', () => 0.5)
    callbacks = createMockCallbacks()
  })

  afterEach(() => {
    client.stop()
  })

  test('should transition from disconnected to connected on first successful sync', async () => {
    installFetchMock()
    client = new SyncClient('http://localhost:7777', 'sess-1', callbacks)

    assert.strictEqual(client.isConnected(), false)

    client.start()
    await tick(50)

    assert.strictEqual(client.isConnected(), true)
    assert.strictEqual(callbacks.onConnectionChange.mock.calls.length, 1)
    assert.strictEqual(callbacks.onConnectionChange.mock.calls[0].arguments[0], true)
  })

  test('should transition from connected to disconnected after 2 consecutive failures', async () => {
    // First call succeeds, subsequent calls fail
    let callCount = 0
    globalThis.fetch = mock.fn(() => {
      callCount++
      if (callCount === 1) {
        return Promise.resolve({
          ok: true,
          json: () => Promise.resolve(makeSyncResponse({ next_poll_ms: 10 }))
        })
      }
      return Promise.reject(new Error('Network failure'))
    })

    client = new SyncClient('http://localhost:7777', 'sess-1', callbacks)
    client.start()

    // Wait for first success + two failures (need 2 consecutive failures for disconnect).
    // First sync fires at ~0ms (success), retry at ~10ms (fail #1), retry at ~1010ms (fail #2).
    await tick(1200)

    assert.strictEqual(client.isConnected(), false)

    // onConnectionChange should have been called twice: true then false
    const changeCalls = callbacks.onConnectionChange.mock.calls
    assert.ok(changeCalls.length >= 2, `Expected >=2 change calls, got ${changeCalls.length}`)
    assert.strictEqual(changeCalls[0].arguments[0], true)
    assert.strictEqual(changeCalls[1].arguments[0], false)
  })

  test('should not emit duplicate connection-change events', async () => {
    // Multiple failures in a row
    globalThis.fetch = mock.fn(() => Promise.reject(new Error('down')))

    client = new SyncClient('http://localhost:7777', 'sess-1', callbacks)
    client.start()

    await tick(200)

    // onConnectionChange(false) should only be called once despite multiple failures
    // (starts disconnected, so zero calls because it was never connected)
    assert.strictEqual(callbacks.onConnectionChange.mock.calls.length, 0)
  })

  test('should NOT disconnect after a single failure (requires 2 consecutive)', async () => {
    // First call succeeds, second fails, third succeeds
    let callCount = 0
    globalThis.fetch = mock.fn(() => {
      callCount++
      if (callCount === 1) {
        return Promise.resolve({
          ok: true,
          json: () => Promise.resolve(makeSyncResponse({ next_poll_ms: 10 }))
        })
      }
      if (callCount === 2) {
        return Promise.reject(new Error('transient failure'))
      }
      return Promise.resolve({
        ok: true,
        json: () => Promise.resolve(makeSyncResponse({ next_poll_ms: 60000 }))
      })
    })

    client = new SyncClient('http://localhost:7777', 'sess-1', callbacks)
    client.start()

    // Wait for first success + one failure + recovery
    await tick(1200)

    assert.strictEqual(client.isConnected(), true)

    // onConnectionChange should only have been called once (connected), never disconnected
    const changeCalls = callbacks.onConnectionChange.mock.calls
    const disconnectCalls = changeCalls.filter((c) => c.arguments[0] === false)
    assert.strictEqual(disconnectCalls.length, 0, 'Should not have disconnected on single failure')
  })

  test('should reset consecutiveFailures on success', async () => {
    // Fail twice, then succeed. Exponential retry waits 1s, then 2s.
    let callCount = 0
    globalThis.fetch = mock.fn(() => {
      callCount++
      if (callCount <= 2) {
        return Promise.reject(new Error('fail'))
      }
      return Promise.resolve({
        ok: true,
        json: () => Promise.resolve(makeSyncResponse({ next_poll_ms: 60000 }))
      })
    })

    client = new SyncClient('http://localhost:7777', 'sess-1', callbacks)
    client.start()

    // Wait for: immediate first call, retry at ~1000ms, recovery at ~3000ms.
    await tick(3200)

    const state = client.getState()
    assert.strictEqual(state.connected, true)
    assert.strictEqual(state.consecutiveFailures, 0)
  })
})

describe('SyncClient — Retry on failure', () => {
  let client
  let callbacks

  beforeEach(() => {
    mock.reset()
    mock.method(Math, 'random', () => 0.5)
    callbacks = createMockCallbacks()
  })

  afterEach(() => {
    client.stop()
  })

  test('should retry after BASE_POLL_MS (1000ms) on failure', async () => {
    globalThis.fetch = mock.fn(() => Promise.reject(new Error('network error')))
    client = new SyncClient('http://localhost:7777', 'sess-1', callbacks)

    client.start()
    await tick(50)

    // First immediate sync fires
    assert.strictEqual(globalThis.fetch.mock.calls.length, 1)

    // After ~1000ms, retry should fire
    await tick(1050)
    assert.ok(globalThis.fetch.mock.calls.length >= 2, 'Should have retried after ~1s')
  })

  test('should increment consecutiveFailures on each failure', async () => {
    globalThis.fetch = mock.fn(() => Promise.reject(new Error('fail')))
    client = new SyncClient('http://localhost:7777', 'sess-1', callbacks)

    client.start()
    await tick(50)

    assert.strictEqual(client.getState().consecutiveFailures, 1)

    // Wait for retry
    await tick(1050)
    assert.ok(client.getState().consecutiveFailures >= 2)
  })

  test('should handle HTTP non-OK status as failure', async () => {
    globalThis.fetch = mock.fn(() =>
      Promise.resolve({
        ok: false,
        status: 503,
        statusText: 'Service Unavailable'
      })
    )

    client = new SyncClient('http://localhost:7777', 'sess-1', callbacks)
    client.start()
    await tick(50)

    assert.strictEqual(client.isConnected(), false)
    assert.strictEqual(client.getState().consecutiveFailures, 1)
  })
})

describe('SyncClient — Request building', () => {
  let client
  let callbacks
  let mockFetch

  beforeEach(() => {
    mock.reset()
    callbacks = createMockCallbacks()
    mockFetch = installFetchMock(makeSyncResponse({ next_poll_ms: 60000 }))
    client = new SyncClient('http://localhost:7777', 'sess-1', callbacks, '6.0.3')
  })

  afterEach(() => {
    client.stop()
  })

  test('should send ext_session_id and extension_version in request body', async () => {
    client.start()
    await tick(50)

    assert.ok(mockFetch.mock.calls.length >= 1)
    const body = JSON.parse(mockFetch.mock.calls[0].arguments[1].body)
    assert.strictEqual(body.ext_session_id, 'sess-1')
    assert.strictEqual(body.extension_version, '6.0.3')
  })

  test('should include settings from callback', async () => {
    client.start()
    await tick(50)

    const body = JSON.parse(mockFetch.mock.calls[0].arguments[1].body)
    assert.ok(body.settings)
    assert.strictEqual(body.settings.capture_logs, true)
    assert.strictEqual(body.settings.pilot_enabled, false)
  })

  test('should include in_progress heartbeat field even when empty', async () => {
    client.start()
    await tick(50)

    const body = JSON.parse(mockFetch.mock.calls[0].arguments[1].body)
    assert.ok(Array.isArray(body.in_progress))
    assert.strictEqual(body.in_progress.length, 0)
  })

  test('should include extension_logs when present', async () => {
    const logs = [
      { timestamp: '2024-01-01T00:00:00Z', level: 'info', message: 'test', source: 'bg', category: 'sync' }
    ]
    callbacks.getExtensionLogs = mock.fn(() => logs)

    client.start()
    await tick(50)

    const body = JSON.parse(mockFetch.mock.calls[0].arguments[1].body)
    assert.ok(body.extension_logs)
    assert.strictEqual(body.extension_logs.length, 1)
    assert.strictEqual(body.extension_logs[0].message, 'test')
  })

  test('should omit extension_logs when empty', async () => {
    callbacks.getExtensionLogs = mock.fn(() => [])

    client.start()
    await tick(50)

    const body = JSON.parse(mockFetch.mock.calls[0].arguments[1].body)
    assert.strictEqual(body.extension_logs, undefined)
  })

  test('should include last_command_ack when set', async () => {
    // First sync to get connected, with a command that sets lastCommandAck after dispatch
    const response = makeSyncResponse({
      commands: [{ id: 'cmd-1', type: 'dom_query', params: {} }],
      next_poll_ms: 10
    })
    mockFetch = installFetchMock(response)

    client.start()
    await tick(50) // wait for dispatch to complete (ack set in finally block)

    // Second sync should include last_command_ack
    await tick(50)
    const secondCallBody = JSON.parse(mockFetch.mock.calls[1].arguments[1].body)
    assert.strictEqual(secondCallBody.last_command_ack, 'cmd-1')
  })

  test('should set correct headers', async () => {
    client.start()
    await tick(50)

    const opts = mockFetch.mock.calls[0].arguments[1]
    assert.strictEqual(opts.headers['Content-Type'], 'application/json')
    assert.strictEqual(opts.headers['X-Kaboom-Client'], 'kaboom-extension/6.0.3')
    assert.strictEqual(opts.headers['X-Kaboom-Extension-Version'], '6.0.3')
  })

  test('should POST to /sync endpoint', async () => {
    client.start()
    await tick(50)

    const [url, opts] = mockFetch.mock.calls[0].arguments
    assert.strictEqual(url, 'http://localhost:7777/sync')
    assert.strictEqual(opts.method, 'POST')
  })
})
