// sync-client-dedupe.test.js — Tests for SyncClient command dedupe behavior.
//
// Regression: command IDs reset on daemon restart (q-1, q-2, ...). If dedupe is
// keyed only by command ID, new commands after restart are silently skipped and
// observe(command_result) polls pending forever.
//
// Run: node --test extension/background/__tests__/sync-client-dedupe.test.js

import { beforeEach, describe, mock, test } from 'node:test'
import assert from 'node:assert'

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

const mockFetch = mock.fn()
globalThis.fetch = mockFetch

const { SyncClient } = await import('../sync-client.js')

const defaultSettings = {
  pilot_enabled: true,
  tracking_enabled: true,
  tracked_tab_id: 1,
  tracked_tab_url: 'http://127.0.0.1:8090/upload',
  tracked_tab_title: 'Upload',
  capture_logs: true,
  capture_network: true,
  capture_websocket: true,
  capture_actions: true
}

function createSyncResponse(commands) {
  return {
    ok: true,
    async json() {
      return {
        ack: true,
        commands,
        next_poll_ms: 5000,
        server_time: new Date().toISOString()
      }
    }
  }
}

async function waitFor(predicate, timeoutMs = 600) {
  const start = Date.now()
  while (!predicate()) {
    if (Date.now() - start > timeoutMs) {
      throw new Error('waitFor timeout')
    }
    await new Promise((r) => setTimeout(r, 10))
  }
}

describe('SyncClient command dedupe', () => {
  beforeEach(() => {
    mockFetch.mock.resetCalls()
  })

  test('does not skip new command when ID is reused with different correlation_id', async () => {
    const seen = []
    let syncCount = 0

    mockFetch.mock.mockImplementation(async () => {
      syncCount++
      if (syncCount === 1) {
        return createSyncResponse([{ id: 'q-1', type: 'browser_action', params: '{}', correlation_id: 'corr-old' }])
      }
      if (syncCount === 2) {
        // Simulates daemon restart: ID counter reset, new correlation_id.
        return createSyncResponse([{ id: 'q-1', type: 'browser_action', params: '{}', correlation_id: 'corr-new' }])
      }
      return createSyncResponse([])
    })

    const client = new SyncClient('http://localhost:7890', 'test-session', {
      onCommand: async (command) => {
        seen.push(command.correlation_id || '')
      },
      onConnectionChange: () => {},
      getSettings: async () => defaultSettings,
      getExtensionLogs: () => [],
      clearExtensionLogs: () => {}
    })

    try {
      client.start()
      await waitFor(() => seen.length === 1)
      client.flush()
      await waitFor(() => syncCount >= 2)
      await waitFor(() => seen.length === 2)
    } finally {
      client.stop()
    }

    assert.deepStrictEqual(
      seen,
      ['corr-old', 'corr-new'],
      `expected both commands to execute, got: ${JSON.stringify(seen)}`
    )
  })

  test('skips exact duplicate command with same id and correlation_id', async () => {
    const seen = []
    let syncCount = 0

    mockFetch.mock.mockImplementation(async () => {
      syncCount++
      if (syncCount <= 2) {
        return createSyncResponse([{ id: 'q-9', type: 'browser_action', params: '{}', correlation_id: 'corr-same' }])
      }
      return createSyncResponse([])
    })

    const client = new SyncClient('http://localhost:7890', 'test-session', {
      onCommand: async (command) => {
        seen.push(command.correlation_id || '')
      },
      onConnectionChange: () => {},
      getSettings: async () => defaultSettings,
      getExtensionLogs: () => [],
      clearExtensionLogs: () => {}
    })

    try {
      client.start()
      await waitFor(() => seen.length === 1)
      client.flush()
      await waitFor(() => syncCount >= 2)
      await new Promise((r) => setTimeout(r, 50))
    } finally {
      client.stop()
    }

    assert.strictEqual(seen.length, 1, `expected duplicate to be skipped, got ${seen.length} executions`)
    assert.strictEqual(seen[0], 'corr-same')
  })
})
