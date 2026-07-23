// sync-client-upload-timeout.test.js — Prevent sync-loop deadlock on hanging uploads.
//
// Bug: if onCommand(upload) never resolves, doSync() blocks forever and no later
// sync cycles run, so all subsequent command_result polls stay pending.
//
// Run: node --test extension/background/__tests__/sync-client-upload-timeout.test.js

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

async function waitFor(predicate, timeoutMs = 700) {
  const start = Date.now()
  while (!predicate()) {
    if (Date.now() - start > timeoutMs) throw new Error('waitFor timeout')
    await new Promise((r) => setTimeout(r, 10))
  }
}

describe('SyncClient upload hang timeout', () => {
  beforeEach(() => {
    mockFetch.mock.resetCalls()
  })

  test('times out hanging upload command and continues syncing', async () => {
    let syncCount = 0

    mockFetch.mock.mockImplementation(async () => {
      syncCount++
      if (syncCount === 1) {
        return makeSyncResponse([{ id: 'q-1', type: 'upload', params: '{}', correlation_id: 'corr-hang' }])
      }
      return makeSyncResponse([])
    })

    const queued = []
    const client = new SyncClient('http://localhost:7890', 'test-session', {
      onCommand: async () => {
        // Simulate hung upload handler (never resolves/rejects)
        await new Promise(() => {})
      },
      onConnectionChange: () => {},
      getSettings: async () => defaultSettings,
      getExtensionLogs: () => [],
      clearExtensionLogs: () => {},
      uploadCommandTimeoutMs: 50
    })

    // Capture queued results while preserving normal queue behavior
    const originalQueue = client.queueCommandResult.bind(client)
    client.queueCommandResult = (result) => {
      queued.push(result)
      originalQueue(result)
    }

    try {
      client.start()
      await waitFor(() => queued.length > 0)
      await waitFor(() => syncCount >= 2)
    } finally {
      client.stop()
    }

    assert.ok(
      queued[0]?.status === 'error' && typeof queued[0]?.error === 'string',
      `expected timeout error result, got: ${JSON.stringify(queued[0])}`
    )
    assert.ok(queued[0].error.includes('timed out'), `expected timeout wording, got: ${queued[0].error}`)
  })
})
