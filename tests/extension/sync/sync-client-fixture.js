// @ts-nocheck
/**
 * @fileoverview Canonical Chrome, callback, response, and timing fixtures for sync-client tests.
 */

import { mock } from 'node:test'
import { MANIFEST_VERSION } from '../shared/helpers.js'

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

export {
  SyncClient,
  createSyncClient,
} from '../../../extension/background/sync/sync-client.js'

// =============================================================================
// HELPERS
// =============================================================================

/** Build a minimal callbacks object. Every function is a mock. */
export function createMockCallbacks(overrides = {}) {
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
    acknowledgeExtensionLogs: mock.fn(),
    debugLog: mock.fn(),
    ...overrides
  }
}

/** Build a valid /sync response body. */
export function makeSyncResponse(overrides = {}) {
  const connectionGeneration = overrides.connection_generation || 1
  const commands = (overrides.commands || []).map((command) => ({
    connection_generation: connectionGeneration,
    ...command
  }))
  return {
    ack: true,
    connection_generation: connectionGeneration,
    next_poll_ms: 1000,
    server_time: new Date().toISOString(),
    ...overrides,
    commands
  }
}

/** Install a mock fetch that returns a successful /sync response. */
export function installFetchMock(responseBody = makeSyncResponse(), options = {}) {
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
export function tick(ms = 20) {
  return new Promise((r) => setTimeout(r, ms))
}

/** Deterministic clock, scheduler, and transport for sync lifecycle contracts. */
export function createManualSyncRuntime(startMs = 1_700_000_000_000) {
  let now = startMs
  let nextTimerId = 1
  const timers = new Map()

  const settle = async () => {
    for (let i = 0; i < 12; i++) await Promise.resolve()
  }

  const runtime = {
    now: () => now,
    random: () => 0.5,
    setTimer(callback, delayMs) {
      const id = nextTimerId++
      timers.set(id, { callback, dueAt: now + Math.max(0, delayMs) })
      return id
    },
    clearTimer(id) {
      timers.delete(id)
    },
    request(url, init) {
      return globalThis.fetch(url, init)
    }
  }

  const runNext = async () => {
    const next = [...timers.entries()].sort((left, right) => {
      const dueDelta = left[1].dueAt - right[1].dueAt
      return dueDelta || left[0] - right[0]
    })[0]
    if (!next) return false
    const [id, timer] = next
    timers.delete(id)
    now = timer.dueAt
    timer.callback()
    await settle()
    return true
  }

  return {
    runtime,
    runNext,
    settle,
    pendingTimers: () => timers.size,
    now: () => now
  }
}
