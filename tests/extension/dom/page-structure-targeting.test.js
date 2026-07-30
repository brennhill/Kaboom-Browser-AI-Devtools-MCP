// @ts-nocheck
/**
 * @fileoverview page-structure-targeting.test.js — Regression guard for analyze(page_structure)
 * target-tab resolution (must not execute against tab_id 0).
 */

import { beforeEach, describe, mock, test } from 'node:test'
import assert from 'node:assert'
import { MANIFEST_VERSION } from '../shared/helpers.js'

function createMockChrome(trackedTabId = 1830196419) {
  return {
    runtime: {
      onMessage: { addListener: mock.fn() },
      onInstalled: { addListener: mock.fn() },
      sendMessage: mock.fn(() => Promise.resolve()),
      getManifest: () => ({ version: MANIFEST_VERSION })
    },
    action: {
      setBadgeText: mock.fn(),
      setBadgeBackgroundColor: mock.fn()
    },
    tabs: {
      query: mock.fn((query, callback) => {
        const result = query?.active
          ? [{ id: trackedTabId, windowId: 1, url: `https://tracked/${trackedTabId}` }]
          : [{ id: trackedTabId, windowId: 1, url: `https://tracked/${trackedTabId}` }]
        if (callback) callback(result)
        return Promise.resolve(result)
      }),
      sendMessage: mock.fn(() => Promise.resolve({ success: true })),
      get: mock.fn((tabId) => Promise.resolve({ id: tabId, windowId: 1, url: `https://tracked/${tabId}`, status: 'complete' })),
      onRemoved: { addListener: mock.fn() }
    },
    scripting: {
      executeScript: mock.fn(() =>
        Promise.resolve([
          {
            result: {
              frameworks: [],
              routing: { type: 'unknown', evidence: '' },
              scroll_containers: [],
              modals: [],
              shadow_roots: 0,
              meta: { viewport: '', charset: '', og_title: '', description: '' }
            }
          }
        ])
      )
    },
    storage: {
      local: {
        get: mock.fn((keys, callback) => {
          const data = {
            serverUrl: 'http://localhost:7890',
            aiWebPilotEnabled: true,
            trackedTabId,
            trackedTabUrl: `https://tracked/${trackedTabId}`,
            trackedTabTitle: 'Tracked Tab'
          }
          if (callback) callback(data)
          return Promise.resolve(data)
        }),
        set: mock.fn((data, callback) => {
          if (callback) callback()
          return Promise.resolve()
        }),
        remove: mock.fn((keys, callback) => {
          if (callback) callback()
          return Promise.resolve()
        })
      },
      sync: {
        get: mock.fn((keys, callback) => {
          if (callback) callback({})
          return Promise.resolve({})
        }),
        set: mock.fn((data, callback) => {
          if (callback) callback()
          return Promise.resolve()
        })
      },
      session: {
        get: mock.fn((keys, callback) => {
          if (callback) callback({})
          return Promise.resolve({})
        }),
        set: mock.fn((data, callback) => {
          if (callback) callback()
          return Promise.resolve()
        })
      },
      onChanged: { addListener: mock.fn() }
    },
    alarms: {
      create: mock.fn(),
      onAlarm: { addListener: mock.fn() }
    }
  }
}

describe('page_structure target resolution', () => {
  let bgModule
  let resetPilotCacheForTesting
  const trackedTabId = 1830196419

  beforeEach(async () => {
    mock.reset()
    globalThis.chrome = createMockChrome(trackedTabId)
    globalThis.fetch = mock.fn(() =>
      Promise.resolve({
        ok: true,
        json: () => Promise.resolve({ queries: [] })
      })
    )

    bgModule = {
      ...(await import('../../../extension/background/pending-queries.js')),
      ...(await import('../../../extension/background/runtime-state/pilot-state.js')),
      ...(await import('../../../extension/background/runtime-state/startup-state.js'))
    }
    ;({ resetPilotCacheForTesting } = await import('../../../extension/background/runtime-state/pilot-state.js'))
    bgModule.markInitComplete()
    resetPilotCacheForTesting(true)
  })

  test('analyze page_structure runs against tracked tab, not tab 0', async () => {
    const mockSyncClient = { queueCommandResult: mock.fn() }

    await bgModule.handlePendingQuery(
      {
        id: 'q-page-structure',
        type: 'page_structure',
        correlation_id: 'corr-page-structure',
        params: JSON.stringify({})
      },
      mockSyncClient
    )

    assert.strictEqual(globalThis.chrome.scripting.executeScript.mock.calls.length >= 1, true)
    const firstCall = globalThis.chrome.scripting.executeScript.mock.calls[0].arguments[0]
    assert.strictEqual(firstCall.target.tabId, trackedTabId)
    assert.notStrictEqual(firstCall.target.tabId, 0)

    assert.strictEqual(mockSyncClient.queueCommandResult.mock.calls.length, 1)
    const queued = mockSyncClient.queueCommandResult.mock.calls[0].arguments[0]
    assert.strictEqual(queued.status, 'complete')
    assert.strictEqual(queued.result.resolved_tab_id, trackedTabId)
    assert.strictEqual(queued.result.target_context.source, 'tracked_tab')
  })
})
