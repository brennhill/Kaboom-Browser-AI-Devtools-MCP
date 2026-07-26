// @ts-nocheck
/**
 * @fileoverview sync-client.test.js — Tests for the unified sync client.
 * Covers construction, connection state transitions, retry logic, command dispatch,
 * command result queuing, version mismatch detection, flush behavior, capture
 * overrides, clean shutdown, and error recovery from network/timeout/malformed responses.
 */

import { test, describe, mock, beforeEach, afterEach } from 'node:test'
import assert from 'node:assert'
import { MANIFEST_VERSION } from './helpers.js'

// Mock Chrome APIs before importing module
globalThis.chrome = {
  runtime: {
    onMessage: { addListener: mock.fn() },
    sendMessage: mock.fn(() => Promise.resolve()),
    getManifest: () => ({ version: MANIFEST_VERSION })
  },
  action: { setBadgeText: mock.fn(), setBadgeBackgroundColor: mock.fn() },
  storage: {
    local: { get: mock.fn((k, cb) => cb({})), set: mock.fn(), remove: mock.fn((k, cb) => cb && cb()) },
    sync: { get: mock.fn((k, cb) => cb({})), set: mock.fn() },
    session: { get: mock.fn((k, cb) => cb({})), set: mock.fn() },
    onChanged: { addListener: mock.fn() }
  },
  tabs: { get: mock.fn(), query: mock.fn(), onRemoved: { addListener: mock.fn() } }
}

import { SyncClient, createSyncClient } from '../../extension/background/sync/sync-client.js'

// =============================================================================
// HELPERS
// =============================================================================

/** Build a minimal callbacks object. Every function is a mock. */
function createMockCallbacks(overrides = {}) {
  return {
    onCommand: mock.fn(() => Promise.resolve()),
    onConnectionChange: mock.fn(),
    onCaptureOverrides: mock.fn(),
    onVersionMismatch: mock.fn(),
    getSettings: mock.fn(() =>
      Promise.resolve({
        pilot_enabled: false,
        tracking_enabled: false,
        tracked_tab_id: 0,
        tracked_tab_url: '',
        tracked_tab_title: '',
        capture_logs: true,
        capture_network: true,
        capture_websocket: false,
        capture_actions: true
      })
    ),
    getExtensionLogs: mock.fn(() => []),
    clearExtensionLogs: mock.fn(),
    debugLog: mock.fn(),
    ...overrides
  }
}

/** Build a valid /sync response body. */
function makeSyncResponse(overrides = {}) {
  return {
    ack: true,
    commands: [],
    next_poll_ms: 1000,
    server_time: new Date().toISOString(),
    ...overrides
  }
}

/** Install a mock fetch that returns a successful /sync response. */
function installFetchMock(responseBody = makeSyncResponse(), options = {}) {
  const mockFetch = mock.fn(() =>
    Promise.resolve({
      ok: options.ok !== undefined ? options.ok : true,
      status: options.status || 200,
      statusText: options.statusText || 'OK',
      json: () => Promise.resolve(responseBody)
    })
  )
  globalThis.fetch = mockFetch
  return mockFetch
}

/** Wait for an async tick + small delay for setTimeout(0) to fire. */
function tick(ms = 20) {
  return new Promise((r) => setTimeout(r, ms))
}

// =============================================================================
// TESTS
// =============================================================================

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
    // Fail twice, then succeed. Each failure retries after BASE_POLL_MS (1000ms).
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

    // Wait for: immediate first call (fail ~0ms), retry at ~1000ms (fail), retry at ~2000ms (success)
    await tick(2200)

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

describe('SyncClient — Command dispatch', () => {
  let client
  let callbacks

  beforeEach(() => {
    mock.reset()
    callbacks = createMockCallbacks()
  })

  afterEach(() => {
    client.stop()
  })

  test('should dispatch commands received from server', async () => {
    const commands = [
      { id: 'cmd-1', type: 'dom_query', params: { selector: '#app' } },
      { id: 'cmd-2', type: 'screenshot', params: {} }
    ]
    installFetchMock(makeSyncResponse({ commands, next_poll_ms: 60000 }))

    client = new SyncClient('http://localhost:7777', 'sess-1', callbacks)
    client.start()
    await tick(50)

    assert.strictEqual(callbacks.onCommand.mock.calls.length, 2)
    assert.deepStrictEqual(callbacks.onCommand.mock.calls[0].arguments[0], commands[0])
    assert.deepStrictEqual(callbacks.onCommand.mock.calls[1].arguments[0], commands[1])
  })

  test('should set lastCommandAck to the last successfully dispatched command id', async () => {
    const commands = [
      { id: 'cmd-1', type: 'dom_query', params: {} },
      { id: 'cmd-2', type: 'screenshot', params: {} }
    ]
    installFetchMock(makeSyncResponse({ commands, next_poll_ms: 60000 }))

    client = new SyncClient('http://localhost:7777', 'sess-1', callbacks)
    client.start()
    await tick(50) // wait for async dispatch to complete (ack set in finally block)

    assert.strictEqual(client.getState().lastCommandAck, 'cmd-2')
  })

  test('should continue dispatching if one command handler throws', async () => {
    let callNum = 0
    callbacks.onCommand = mock.fn(() => {
      callNum++
      if (callNum === 1) throw new Error('handler crash')
      return Promise.resolve()
    })

    const commands = [
      { id: 'cmd-1', type: 'will_fail', params: {} },
      { id: 'cmd-2', type: 'will_succeed', params: {} }
    ]
    installFetchMock(makeSyncResponse({ commands, next_poll_ms: 60000 }))

    client = new SyncClient('http://localhost:7777', 'sess-1', callbacks)
    client.start()
    await tick(50) // wait for async dispatch to complete (ack set in finally block)

    // Both commands attempted
    assert.strictEqual(callbacks.onCommand.mock.calls.length, 2)
    // lastCommandAck should be the second command — ack advances through failed dispatches too
    assert.strictEqual(client.getState().lastCommandAck, 'cmd-2')
  })

  test('should not ack commands before dispatch completes', async () => {
    let resolveSlowHandler
    callbacks.onCommand = mock.fn(
      () =>
        new Promise((resolve) => {
          resolveSlowHandler = resolve
        })
    )

    const commands = [{ id: 'cmd-slow', type: 'browser_action', params: {} }]
    installFetchMock(makeSyncResponse({ commands, next_poll_ms: 60000 }))

    client = new SyncClient('http://localhost:7777', 'sess-1', callbacks)
    client.start()
    await tick(30) // sync fires, command dispatched but handler not resolved yet

    // Ack should NOT be set while dispatch is still in progress
    assert.strictEqual(client.getState().lastCommandAck, null, 'should not ack before dispatch completes')

    // Now resolve the handler
    resolveSlowHandler()
    await tick(30)

    // Ack should now be set
    assert.strictEqual(client.getState().lastCommandAck, 'cmd-slow', 'should ack after dispatch completes')
  })

  test('should report running commands in in_progress heartbeat payload', async () => {
    let callCount = 0
    globalThis.fetch = mock.fn(() => {
      callCount++
      if (callCount === 1) {
        return Promise.resolve({
          ok: true,
          json: () =>
            Promise.resolve(
              makeSyncResponse({
                commands: [{ id: 'cmd-running', type: 'browser_action', correlation_id: 'corr-running', params: {} }],
                next_poll_ms: 10
              })
            )
        })
      }
      return Promise.resolve({
        ok: true,
        json: () => Promise.resolve(makeSyncResponse({ commands: [], next_poll_ms: 60000 }))
      })
    })

    callbacks.onCommand = mock.fn(
      () =>
        new Promise((resolve) => {
          setTimeout(resolve, 120)
        })
    )

    client = new SyncClient('http://localhost:7777', 'sess-1', callbacks)
    client.start()
    await tick(70)

    assert.ok(globalThis.fetch.mock.calls.length >= 2, 'expected at least two sync cycles')
    const secondBody = JSON.parse(globalThis.fetch.mock.calls[1].arguments[1].body)
    assert.ok(Array.isArray(secondBody.in_progress))
    const active = secondBody.in_progress.find((cmd) => cmd.id === 'cmd-running')
    assert.ok(active, `expected cmd-running in in_progress, got ${JSON.stringify(secondBody.in_progress)}`)
    assert.strictEqual(active.correlation_id, 'corr-running')
    assert.strictEqual(active.type, 'browser_action')
    assert.ok(active.status === 'running' || active.status === 'pending')
  })

  test('should not dispatch commands when response has empty commands array', async () => {
    installFetchMock(makeSyncResponse({ commands: [], next_poll_ms: 60000 }))

    client = new SyncClient('http://localhost:7777', 'sess-1', callbacks)
    client.start()
    await tick(50)

    assert.strictEqual(callbacks.onCommand.mock.calls.length, 0)
  })

  test('should not redispatch a command ID that was already acknowledged', async () => {
    let pollCount = 0
    globalThis.fetch = mock.fn(() => {
      pollCount++
      if (pollCount === 1) {
        return Promise.resolve({
          ok: true,
          json: () =>
            Promise.resolve(
              makeSyncResponse({
                commands: [{ id: 'cmd-dup', type: 'dom_query', params: {} }],
                next_poll_ms: 10
              })
            )
        })
      }
      return Promise.resolve({
        ok: true,
        json: () =>
          Promise.resolve(
            makeSyncResponse({
              commands: [{ id: 'cmd-dup', type: 'dom_query', params: {} }],
              next_poll_ms: 60000
            })
          )
      })
    })

    client = new SyncClient('http://localhost:7777', 'sess-1', callbacks)
    client.start()
    await tick(120)

    assert.strictEqual(callbacks.onCommand.mock.calls.length, 1)
  })

  test('should keep syncing while async command handlers are still running', async () => {
    let fetchCalls = 0
    globalThis.fetch = mock.fn(() => {
      fetchCalls++
      if (fetchCalls === 1) {
        return Promise.resolve({
          ok: true,
          json: () =>
            Promise.resolve(
              makeSyncResponse({
                commands: [{ id: 'cmd-slow', type: 'browser_action', params: {} }],
                next_poll_ms: 10
              })
            )
        })
      }
      return Promise.resolve({
        ok: true,
        json: () => Promise.resolve(makeSyncResponse({ commands: [], next_poll_ms: 10 }))
      })
    })

    callbacks.onCommand = mock.fn(
      () =>
        new Promise((resolve) => {
          setTimeout(resolve, 80)
        })
    )

    client = new SyncClient('http://localhost:7777', 'sess-1', callbacks)
    client.start()

    await tick(40)
    assert.ok(fetchCalls >= 2, `expected heartbeat sync to continue while command is running, got ${fetchCalls}`)

    await tick(120)
    assert.strictEqual(callbacks.onCommand.mock.calls.length, 1)
    assert.ok(fetchCalls >= 2, `expected at least 2 sync cycles, got ${fetchCalls}`)
  })

  test('should timeout hanging command handlers and continue syncing', async () => {
    let fetchCalls = 0
    globalThis.fetch = mock.fn(() => {
      fetchCalls++
      if (fetchCalls === 1) {
        return Promise.resolve({
          ok: true,
          json: () =>
            Promise.resolve(
              makeSyncResponse({
                commands: [{ id: 'cmd-hang', type: 'browser_action', params: {} }],
                next_poll_ms: 10
              })
            )
        })
      }
      return Promise.resolve({
        ok: true,
        json: () => Promise.resolve(makeSyncResponse({ commands: [], next_poll_ms: 10 }))
      })
    })

    callbacks.onCommand = mock.fn(async () => {
      await new Promise(() => {})
    })
    callbacks.commandTimeoutMs = 50

    client = new SyncClient('http://localhost:7777', 'sess-1', callbacks)
    const queued = []
    const originalQueue = client.queueCommandResult.bind(client)
    client.queueCommandResult = (result) => {
      queued.push(result)
      originalQueue(result)
    }
    client.start()

    for (let i = 0; i < 40 && queued.length === 0; i++) {
      await tick(10)
    }

    assert.ok(queued.length > 0, 'expected a timeout error result to be queued')
    assert.strictEqual(queued[0].id, 'cmd-hang')
    assert.strictEqual(queued[0].status, 'error')
    assert.ok(String(queued[0].error).includes('timed out'))
    for (let i = 0; i < 20 && fetchCalls < 2; i++) {
      await tick(10)
    }
    assert.ok(fetchCalls >= 2, `expected sync loop to continue after timeout, got ${fetchCalls} fetch call(s)`)
  })

  test('should not let long command timeout block heartbeat polling', async () => {
    let fetchCalls = 0
    globalThis.fetch = mock.fn(() => {
      fetchCalls++
      if (fetchCalls === 1) {
        return Promise.resolve({
          ok: true,
          json: () =>
            Promise.resolve(
              makeSyncResponse({
                commands: [{ id: 'cmd-long', type: 'browser_action', params: {} }],
                next_poll_ms: 10
              })
            )
        })
      }
      return Promise.resolve({
        ok: true,
        json: () => Promise.resolve(makeSyncResponse({ commands: [], next_poll_ms: 10 }))
      })
    })

    callbacks.onCommand = mock.fn(async () => {
      await new Promise(() => {})
    })
    callbacks.commandTimeoutMs = 10_000

    client = new SyncClient('http://localhost:7777', 'sess-1', callbacks)
    client.start()

    await tick(60)
    assert.ok(fetchCalls >= 2, `expected follow-up heartbeat polls while command is still pending, got ${fetchCalls}`)
  })
})

describe('SyncClient — Command result queuing', () => {
  let client
  let callbacks

  beforeEach(() => {
    mock.reset()
    callbacks = createMockCallbacks()
  })

  afterEach(() => {
    client.stop()
  })

  test('should include pending results in request body', async () => {
    const mockFetch = installFetchMock(makeSyncResponse({ next_poll_ms: 60000 }))

    client = new SyncClient('http://localhost:7777', 'sess-1', callbacks)
    client.start()
    await tick(50) // first sync completes

    // Queue a result and it will flush immediately
    client.queueCommandResult({
      id: 'cmd-1',
      correlation_id: 'corr-1',
      status: 'complete',
      result: { html: '<div>test</div>' }
    })
    await tick(50)

    // Find the fetch call that included command_results
    const callsWithResults = mockFetch.mock.calls.filter((c) => {
      const body = JSON.parse(c.arguments[1].body)
      return body.command_results && body.command_results.length > 0
    })

    assert.ok(callsWithResults.length >= 1, 'Should have sent command_results')
    const body = JSON.parse(callsWithResults[0].arguments[1].body)
    assert.strictEqual(body.command_results[0].id, 'cmd-1')
    assert.strictEqual(body.command_results[0].status, 'complete')
  })

  test('should cap pending results queue at 200', () => {
    installFetchMock(makeSyncResponse({ next_poll_ms: 60000 }))
    // Don't start — we just want to test queuing without running syncs
    client = new SyncClient('http://localhost:7777', 'sess-1', callbacks)

    // Queue 250 results
    for (let i = 0; i < 250; i++) {
      // Manually push to avoid flush triggering (since client is not started, flush is a no-op)
      client.queueCommandResult({ id: `cmd-${i}`, status: 'complete' })
    }

    // The internal queue should be capped at 200
    // We can verify by checking that the oldest entries were dropped
    // (We test this indirectly — the code splices to keep last 200)
    // Start a sync to verify the body
    const mockFetch = installFetchMock(makeSyncResponse({ next_poll_ms: 60000 }))
    client.start()

    // Wait for sync
    return tick(50).then(() => {
      client.stop()
      const body = JSON.parse(mockFetch.mock.calls[0].arguments[1].body)
      assert.ok(body.command_results.length <= 200, `Expected <=200 results, got ${body.command_results.length}`)
    })
  })

  test('should clear pending results after successful sync', async () => {
    const mockFetch = installFetchMock(makeSyncResponse({ next_poll_ms: 60000 }))

    client = new SyncClient('http://localhost:7777', 'sess-1', callbacks)

    // Queue result before starting
    client.queueCommandResult({ id: 'cmd-1', status: 'complete' })
    client.start()
    await tick(50)

    // First sync includes results
    const firstBody = JSON.parse(mockFetch.mock.calls[0].arguments[1].body)
    assert.ok(firstBody.command_results)
    assert.strictEqual(firstBody.command_results.length, 1)

    // Force another sync via flush
    client.flush()
    await tick(50)

    // Second sync should not have command_results (they were cleared)
    const lastCall = mockFetch.mock.calls[mockFetch.mock.calls.length - 1]
    const lastBody = JSON.parse(lastCall.arguments[1].body)
    assert.strictEqual(lastBody.command_results, undefined)

    client.stop()
  })
})

describe('SyncClient — Version mismatch handling', () => {
  let client
  let callbacks

  beforeEach(() => {
    mock.reset()
    callbacks = createMockCallbacks()
  })

  afterEach(() => {
    client.stop()
  })

  test('should call onVersionMismatch when major.minor differs', async () => {
    installFetchMock(
      makeSyncResponse({
        server_version: '7.1.0',
        next_poll_ms: 60000
      })
    )

    client = new SyncClient('http://localhost:7777', 'sess-1', callbacks, '6.0.3')
    client.start()
    await tick(50)

    assert.strictEqual(callbacks.onVersionMismatch.mock.calls.length, 1)
    assert.strictEqual(callbacks.onVersionMismatch.mock.calls[0].arguments[0], '6.0.3')
    assert.strictEqual(callbacks.onVersionMismatch.mock.calls[0].arguments[1], '7.1.0')
  })

  test('should NOT call onVersionMismatch when major.minor matches', async () => {
    installFetchMock(
      makeSyncResponse({
        server_version: '6.0.9',
        next_poll_ms: 60000
      })
    )

    client = new SyncClient('http://localhost:7777', 'sess-1', callbacks, '6.0.3')
    client.start()
    await tick(50)

    assert.strictEqual(callbacks.onVersionMismatch.mock.calls.length, 0)
  })

  test('should NOT call onVersionMismatch when server_version is absent', async () => {
    installFetchMock(makeSyncResponse({ next_poll_ms: 60000 }))

    client = new SyncClient('http://localhost:7777', 'sess-1', callbacks, '6.0.3')
    client.start()
    await tick(50)

    assert.strictEqual(callbacks.onVersionMismatch.mock.calls.length, 0)
  })

  test('should NOT call onVersionMismatch when extension version is empty', async () => {
    installFetchMock(
      makeSyncResponse({
        server_version: '7.0.0',
        next_poll_ms: 60000
      })
    )

    client = new SyncClient('http://localhost:7777', 'sess-1', callbacks, '')
    client.start()
    await tick(50)

    assert.strictEqual(callbacks.onVersionMismatch.mock.calls.length, 0)
  })

  test('should NOT crash when onVersionMismatch callback is not provided', async () => {
    delete callbacks.onVersionMismatch
    installFetchMock(
      makeSyncResponse({
        server_version: '7.0.0',
        next_poll_ms: 60000
      })
    )

    client = new SyncClient('http://localhost:7777', 'sess-1', callbacks, '6.0.3')
    client.start()
    await tick(50)

    // Should not throw — just skip version check
    assert.strictEqual(client.isConnected(), true)
  })
})

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

describe('SyncClient — Extension logs clearing', () => {
  let client
  let callbacks

  beforeEach(() => {
    mock.reset()
    callbacks = createMockCallbacks()
  })

  afterEach(() => {
    client.stop()
  })

  test('should clear extension logs after successful sync with logs', async () => {
    callbacks.getExtensionLogs = mock.fn(() => [{ timestamp: 'now', level: 'info', message: 'x', source: 'bg', category: 'sync' }])
    installFetchMock(makeSyncResponse({ next_poll_ms: 60000 }))

    client = new SyncClient('http://localhost:7777', 'sess-1', callbacks)
    client.start()
    await tick(50)

    assert.strictEqual(callbacks.clearExtensionLogs.mock.calls.length, 1)
  })

  test('should NOT clear extension logs when none were sent', async () => {
    callbacks.getExtensionLogs = mock.fn(() => [])
    installFetchMock(makeSyncResponse({ next_poll_ms: 60000 }))

    client = new SyncClient('http://localhost:7777', 'sess-1', callbacks)
    client.start()
    await tick(50)

    assert.strictEqual(callbacks.clearExtensionLogs.mock.calls.length, 0)
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
    await tick(2200)

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
