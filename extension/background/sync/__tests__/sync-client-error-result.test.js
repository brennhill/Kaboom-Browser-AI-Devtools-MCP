// sync-client-error-result.test.js — Tests that SyncClient queues error results
// when the onCommand callback throws.
//
// Bug: The catch block in doSync's command dispatch loop logged the error but
// never called queueCommandResult, leaving the query pending forever.
//
// Run: node --test extension/background/__tests__/sync-client-error-result.test.js

import { test, describe, mock, beforeEach } from 'node:test'
import assert from 'node:assert'

// ============================================
// Mock Chrome APIs
// ============================================

if (typeof globalThis.chrome === 'undefined') {
  globalThis.chrome = {
    runtime: {
      getURL: () => '',
      sendMessage: () => Promise.resolve(),
      onMessage: { addListener: () => {} },
      onInstalled: { addListener: () => {} },
      getManifest: () => ({ version: '1.0.0' })
    },
    storage: {
      local: {
        get: (_k, cb) => cb && cb({}),
        set: (_d, cb) => cb && cb()
      },
      onChanged: { addListener: () => {} }
    },
    tabs: {
      get: () => Promise.resolve({}),
      query: () => Promise.resolve([]),
      onRemoved: { addListener: () => {} },
      onUpdated: { addListener: () => {} }
    },
    action: { setBadgeText: () => {}, setBadgeBackgroundColor: () => {} },
    alarms: { create: () => {}, onAlarm: { addListener: () => {} } },
    commands: { onCommand: { addListener: () => {} } }
  }
}

// Mock fetch
const mockFetch = mock.fn()
globalThis.fetch = mockFetch

// ============================================
// Import after mocks
// ============================================

const { SyncClient } = await import('../sync-client.js')

// ============================================
// Helpers
// ============================================

const defaultSettings = {
  pilot_enabled: false,
  tracking_enabled: false,
  tracked_tab_id: 0,
  tracked_tab_url: '',
  tracked_tab_title: '',
  capture_logs: true,
  capture_network: true,
  capture_websocket: true,
  capture_actions: true
}

// ============================================
// Tests
// ============================================

describe('SyncClient — error result on command dispatch failure', () => {
  beforeEach(() => {
    mockFetch.mock.resetCalls()
  })

  test('queues error result when onCommand throws', async () => {
    const handlerError = new Error('handler exploded')
    const queuedResults = []

    const client = new SyncClient('http://localhost:7890', 'test-session', {
      onCommand: async () => {
        throw handlerError
      },
      onConnectionChange: () => {},
      getSettings: async () => defaultSettings,
      getExtensionLogs: () => [],
      clearExtensionLogs: () => {}
    })

    // Spy on queueCommandResult to capture calls
    client.queueCommandResult = (result) => {
      queuedResults.push(result)
    }

    // Mock fetch to return a sync response with one command
    mockFetch.mock.mockImplementation(() =>
      Promise.resolve({
        ok: true,
        json: () =>
          Promise.resolve({
            ack: true,
            commands: [{ id: 'cmd-1', type: 'upload', params: '{}', correlation_id: 'corr-1' }],
            next_poll_ms: 5000,
            server_time: new Date().toISOString()
          })
      })
    )

    // Start client — triggers immediate sync
    client.start()

    // Wait for the async sync cycle to complete
    await new Promise((r) => setTimeout(r, 200))

    client.stop()

    // The onCommand threw — the catch block should have queued an error result
    assert.ok(
      queuedResults.length > 0,
      'Expected SyncClient to queue an error result when onCommand throws, but no result was queued (error swallowed)'
    )
    assert.strictEqual(queuedResults[0].id, 'cmd-1', 'Error result should reference the failed command ID')
    assert.strictEqual(queuedResults[0].status, 'error', 'Error result status should be "error"')
    assert.ok(
      queuedResults[0].error && queuedResults[0].error.includes('handler exploded'),
      `Error message should contain the thrown error, got: ${queuedResults[0].error}`
    )
  })

  test('queues error results for all commands when onCommand throws', async () => {
    const queuedResults = []

    const client = new SyncClient('http://localhost:7890', 'test-session', {
      onCommand: async () => {
        throw new Error('boom')
      },
      onConnectionChange: () => {},
      getSettings: async () => defaultSettings,
      getExtensionLogs: () => [],
      clearExtensionLogs: () => {}
    })

    client.queueCommandResult = (result) => {
      queuedResults.push(result)
    }

    // Two commands in one sync response
    mockFetch.mock.mockImplementation(() =>
      Promise.resolve({
        ok: true,
        json: () =>
          Promise.resolve({
            ack: true,
            commands: [
              { id: 'cmd-A', type: 'dom', params: '{}' },
              { id: 'cmd-B', type: 'dom', params: '{}' }
            ],
            next_poll_ms: 5000,
            server_time: new Date().toISOString()
          })
      })
    )

    client.start()
    await new Promise((r) => setTimeout(r, 200))
    client.stop()

    // Both commands should get error results
    const cmdIds = queuedResults.map((r) => r.id)
    assert.ok(
      cmdIds.includes('cmd-A'),
      `Expected error result for cmd-A, got results for: ${cmdIds.join(', ') || '(none)'}`
    )
    assert.ok(
      cmdIds.includes('cmd-B'),
      `Expected error result for cmd-B, got results for: ${cmdIds.join(', ') || '(none)'}`
    )
  })
})
