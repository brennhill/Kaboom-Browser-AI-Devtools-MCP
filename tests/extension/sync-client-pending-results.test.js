// sync-client-pending-results.test.js — Prevent dropping results queued during an in-flight /sync.
//
// Bug: SyncClient cleared pendingResults = [] after every successful /sync.
// If onCommand queued a result during that same sync cycle, the result was
// silently dropped and never sent, leaving command_result polls pending.
//
// Run: node --experimental-test-module-mocks --test tests/extension/sync-client-pending-results.test.js

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

const { SyncClient } = await import('../../extension/background/sync/sync-client.js')

const defaultSettings = {
  pilot_enabled: true,
  tracking_enabled: true,
  tracked_tab_id: 1,
  tracked_tab_url: 'https://example.com',
  tracked_tab_title: 'Example Domain',
  capture_logs: true,
  capture_network: true,
  capture_websocket: true,
  capture_actions: true
}

function makeSyncResponse(commands, nextPollMs = 5) {
  return {
    ok: true,
    async json() {
      return {
        ack: true,
        commands,
        next_poll_ms: nextPollMs,
        server_time: new Date().toISOString()
      }
    }
  }
}

async function waitFor(predicate, timeoutMs = 800) {
  const start = Date.now()
  while (!predicate()) {
    if (Date.now() - start > timeoutMs) throw new Error('waitFor timeout')
    await new Promise((r) => setTimeout(r, 10))
  }
}

describe('SyncClient pendingResults retention', () => {
  beforeEach(() => {
    mockFetch.mock.resetCalls()
  })

  test('does not drop command results queued during the same sync cycle', async () => {
    const requests = []
    let syncCount = 0

    mockFetch.mock.mockImplementation(async (_url, init) => {
      const payload = init?.body ? JSON.parse(init.body) : {}
      requests.push(payload)
      syncCount++
      if (syncCount === 1) {
        return makeSyncResponse([{ id: 'q-1', type: 'browser_action', params: '{}', correlation_id: 'corr-1' }], 5)
      }
      return makeSyncResponse([], 50)
    })

    /** @type {SyncClient|null} */
    let client = null
    client = new SyncClient('http://localhost:7890', 'test-session', {
      onCommand: async (command) => {
        // Queue completion immediately while doSync() is still in its success path.
        client.queueCommandResult({
          id: command.id,
          correlation_id: command.correlation_id,
          status: 'complete',
          result: { success: true }
        })
      },
      onConnectionChange: () => {},
      getSettings: async () => defaultSettings,
      getExtensionLogs: () => [],
      clearExtensionLogs: () => {}
    })

    try {
      client.start()
      await waitFor(() => requests.length >= 2)
    } finally {
      client.stop()
    }

    assert.ok(!requests[0].command_results || requests[0].command_results.length === 0, 'first sync should not send command_results yet')
    assert.ok(Array.isArray(requests[1].command_results), 'second sync should include queued command_results')
    assert.strictEqual(requests[1].command_results.length, 1, `expected exactly one queued result, got ${JSON.stringify(requests[1].command_results)}`)
    assert.strictEqual(requests[1].command_results[0].id, 'q-1')
    assert.strictEqual(requests[1].command_results[0].status, 'complete')
  })
})
