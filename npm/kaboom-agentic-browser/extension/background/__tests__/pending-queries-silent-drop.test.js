// pending-queries-silent-drop.test.js — Tests that handlePendingQuery always
// sends a result back to the daemon, even when no tab is available.
//
// Bug: Several code paths returned silently without calling sendResult or
// sendAsyncResult, leaving queries in "pending" state forever.
//
// Run: node --test extension/background/__tests__/pending-queries-silent-drop.test.js

import { test, describe, mock, beforeEach } from 'node:test'
import assert from 'node:assert'

// ============================================
// Mock Chrome APIs (must be set before module import)
// ============================================

const mockTabsGet = mock.fn()
const mockTabsQuery = mock.fn()
const mockStorageLocalGet = mock.fn()
const mockSendMessage = mock.fn(() => Promise.resolve())

globalThis.chrome = {
  runtime: {
    getURL: () => '',
    sendMessage: mockSendMessage,
    onMessage: { addListener: () => {} },
    onInstalled: { addListener: () => {} },
    getManifest: () => ({ version: '1.0.0' })
  },
  storage: {
    local: {
      get: mockStorageLocalGet,
      set: (_d, cb) => cb && cb(),
      remove: (_k, cb) => {
        if (cb) cb()
        return Promise.resolve()
      }
    },
    onChanged: { addListener: () => {} }
  },
  tabs: {
    get: mockTabsGet,
    query: mockTabsQuery,
    onRemoved: { addListener: () => {} },
    onUpdated: { addListener: () => {} },
    sendMessage: mockSendMessage
  },
  action: {
    setBadgeText: () => {},
    setBadgeBackgroundColor: () => {}
  },
  scripting: { executeScript: () => Promise.resolve([]) },
  alarms: { create: () => {}, onAlarm: { addListener: () => {} } },
  commands: { onCommand: { addListener: () => {} } }
}

globalThis.fetch = mock.fn(() => Promise.resolve({ ok: true, json: () => Promise.resolve({}) }))

// ============================================
// Import after mocks are set up
// ============================================

const { handlePendingQuery } = await import('../pending-queries.js')
const { markInitComplete } = await import('../state.js')

// ============================================
// Mock SyncClient
// ============================================

function createMockSyncClient() {
  const results = []
  return {
    queueCommandResult(result) {
      results.push(result)
    },
    flush() {},
    getResults() {
      return results
    }
  }
}

// ============================================
// Tests
// ============================================

describe('handlePendingQuery — silent drop prevention', () => {
  beforeEach(() => {
    markInitComplete()
    mockTabsGet.mock.resetCalls()
    mockTabsQuery.mock.resetCalls()
    mockStorageLocalGet.mock.resetCalls()
    mockSendMessage.mock.resetCalls()
  })

  test('sends error result when no tracked tab and no active tab (sync query)', async () => {
    // No tracked tab
    mockStorageLocalGet.mock.mockImplementation(() => Promise.resolve({}))
    // No active tab
    mockTabsQuery.mock.mockImplementation(() => Promise.resolve([]))

    const syncClient = createMockSyncClient()
    const query = { id: 'q-1', type: 'dom', params: '{}' }

    await handlePendingQuery(query, syncClient)

    const results = syncClient.getResults()
    assert.ok(
      results.length > 0,
      'Expected syncClient to receive an error result, but no result was sent (query silently dropped)'
    )
    assert.strictEqual(results[0].id, 'q-1')
    // Should indicate an error, not success
    const result = results[0]
    const hasError =
      result.status === 'error' || (result.result && typeof result.result === 'object' && 'error' in result.result)
    assert.ok(hasError, `Expected error result, got: ${JSON.stringify(result)}`)
  })

  test('sends error result when no tracked tab and no active tab (async query with correlation_id)', async () => {
    // No tracked tab
    mockStorageLocalGet.mock.mockImplementation(() => Promise.resolve({}))
    // No active tab
    mockTabsQuery.mock.mockImplementation(() => Promise.resolve([]))

    const syncClient = createMockSyncClient()
    const query = { id: 'q-2', type: 'upload', params: '{}', correlation_id: 'corr-2' }

    await handlePendingQuery(query, syncClient)

    const results = syncClient.getResults()
    assert.ok(
      results.length > 0,
      'Expected syncClient to receive an error result for async query, but no result was sent (query silently dropped)'
    )
    assert.strictEqual(results[0].id, 'q-2')
    assert.ok(
      results[0].status === 'error' || results[0].error,
      `Expected error status or error field, got: ${JSON.stringify(results[0])}`
    )
  })

  test('sends error result when tracked tab is gone and no active fallback tab', async () => {
    // Tracked tab exists in storage
    mockStorageLocalGet.mock.mockImplementation(() => Promise.resolve({ trackedTabId: 999 }))
    // tabs.get(999) always fails (tab gone)
    mockTabsGet.mock.mockImplementation(() => Promise.reject(new Error('No tab with id 999')))
    // No active tab fallback
    mockTabsQuery.mock.mockImplementation(() => Promise.resolve([]))

    const syncClient = createMockSyncClient()
    const query = { id: 'q-3', type: 'dom', params: '{}' }

    await handlePendingQuery(query, syncClient)

    const results = syncClient.getResults()
    assert.ok(
      results.length > 0,
      'Expected syncClient to receive an error result when tracked tab gone and no fallback, but no result was sent'
    )
    assert.strictEqual(results[0].id, 'q-3')
  })

  test('sends error result when outer catch fires due to unexpected error', async () => {
    // Make getTrackedTabInfo throw to trigger outer catch
    mockStorageLocalGet.mock.mockImplementation(() => {
      throw new Error('storage unavailable')
    })

    const syncClient = createMockSyncClient()
    const query = { id: 'q-4', type: 'dom', params: '{}' }

    await handlePendingQuery(query, syncClient)

    const results = syncClient.getResults()
    assert.ok(
      results.length > 0,
      'Expected syncClient to receive an error result from outer catch, but no result was sent (error swallowed)'
    )
    assert.strictEqual(results[0].id, 'q-4')
  })
})
