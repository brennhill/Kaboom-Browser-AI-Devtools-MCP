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
    clearExtensionLogs: mock.fn(),
    debugLog: mock.fn(),
    ...overrides
  }
}

/** Build a valid /sync response body. */
export function makeSyncResponse(overrides = {}) {
  return {
    ack: true,
    commands: [],
    next_poll_ms: 1000,
    server_time: new Date().toISOString(),
    ...overrides
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
