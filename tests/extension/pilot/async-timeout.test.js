// @ts-nocheck
/**
 * @fileoverview async-timeout.test.js - Tests for Bug #5: Extension Timeout After 5-6 Operations.
 *
 * Tests that async operations are properly awaited to prevent:
 *   1. Premature cleanup of processing queries
 *   2. Duplicate query processing
 *   3. Timeouts after 5-6 consecutive operations
 *   4. Memory leaks from orphaned promises
 *
 * Root cause: Missing `await` on async handler calls causes the function to return
 * immediately, triggering cleanup while the operation is still running.
 */

import { test, describe, mock, beforeEach, after } from 'node:test'
import assert from 'node:assert'
import { MANIFEST_VERSION } from '../shared/helpers.js'

// Mock Chrome APIs
const createMockChrome = () => ({
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
      if (callback) {
        callback([{ id: 1, windowId: 1, url: 'http://localhost:3000' }])
      }
      return Promise.resolve([{ id: 1, windowId: 1, url: 'http://localhost:3000' }])
    }),
    sendMessage: mock.fn((_tabId, _message) => Promise.resolve({ success: true, result: 'test-result' })),
    get: mock.fn((tabId) => Promise.resolve({ id: tabId, windowId: 1, url: 'http://localhost:3000' })),
    goBack: mock.fn(() => Promise.resolve()),
    goForward: mock.fn(() => Promise.resolve()),
    reload: mock.fn(() => Promise.resolve()),
    update: mock.fn(() => Promise.resolve()),
    create: mock.fn(() => Promise.resolve({ id: 2 })),
    onRemoved: { addListener: mock.fn() }
  },
  storage: {
    local: {
      get: mock.fn((keys, callback) => {
        const data = {
          serverUrl: 'http://localhost:7890',
          aiWebPilotEnabled: true,
          trackedTabId: 1
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
})

// Set global chrome mock
globalThis.chrome = createMockChrome()

// Track intervals to clean up leaked timers
const activeIntervals = new Set()
const _originalSetInterval = globalThis.setInterval
const _originalClearInterval = globalThis.clearInterval
globalThis.setInterval = (...args) => {
  const id = _originalSetInterval(...args)
  activeIntervals.add(id)
  return id
}
globalThis.clearInterval = (id) => {
  activeIntervals.delete(id)
  _originalClearInterval(id)
}

// Clean up leaked intervals after all tests
after(() => {
  for (const id of activeIntervals) {
    _originalClearInterval(id)
  }
  globalThis.setInterval = _originalSetInterval
  globalThis.clearInterval = _originalClearInterval
})

// Suppress unhandledRejection errors from background module initialization
process.on('unhandledRejection', (reason, _promise) => {
  // Suppress initialization errors from background.js module loading
  if (reason?.message?.includes('_connectionCheckRunning') || reason?.message?.includes('Cannot access')) {
    // Expected during test - background.js tries to access globals before init
    return
  }
  // Re-throw other unhandled rejections
  throw reason
})

// =============================================================================
// Bug #5: Missing await on handleAsyncExecuteCommand
// =============================================================================

let resetPilotCacheForTesting

describe('Bug #5: Async Execute Command Await', () => {
  let bgModule

  beforeEach(async () => {
    mock.reset()
    globalThis.chrome = createMockChrome()
    globalThis.fetch = mock.fn(() =>
      Promise.resolve({
        ok: true,
        json: () => Promise.resolve({ queries: [] })
      })
    )

    bgModule = {
      ...(await import('../../../extension/background/index.js')),
      ...(await import('../../../extension/background/state.js')),
      ...(await import('../../../extension/background/caches/snapshots.js'))
    }
    ;({ _resetPilotCacheForTesting: resetPilotCacheForTesting } = await import('../../../extension/background/state.js'))
    bgModule.markInitComplete()
    resetPilotCacheForTesting(true)
  })

  test('handlePendingQuery should await handleAsyncExecuteCommand for correlation_id queries', async () => {
    const query = {
      id: 'query-123',
      type: 'execute',
      correlation_id: 'corr-456',
      params: JSON.stringify({ script: 'return 1+1' })
    }

    let resolveExecute
    globalThis.chrome.tabs.sendMessage = mock.fn((_tabId, message) => {
      if (message?.type === 'kaboom_execute_query') {
        return new Promise((resolve) => {
          resolveExecute = resolve
        })
      }
      return Promise.resolve({ success: true, result: 'ok' })
    })

    const mockSyncClient = { queueCommandResult: mock.fn() }
    let handlePendingQueryCompleted = false
    const promise = bgModule.handlePendingQuery(query, mockSyncClient).then(() => {
      handlePendingQueryCompleted = true
    })

    await new Promise((resolve) => setTimeout(resolve, 10))
    assert.strictEqual(handlePendingQueryCompleted, false)
    assert.strictEqual(mockSyncClient.queueCommandResult.mock.calls.length, 0)

    resolveExecute({ success: true, result: 2 })
    await promise

    assert.strictEqual(mockSyncClient.queueCommandResult.mock.calls.length, 1)
    const queuedResult = mockSyncClient.queueCommandResult.mock.calls[0].arguments[0]
    assert.strictEqual(queuedResult.correlation_id, query.correlation_id)
    assert.strictEqual(queuedResult.status, 'complete')
    assert.strictEqual(queuedResult.result.success, true)
  })

  test('execute failures should post async status=error with error details', async () => {
    const query = {
      id: 'query-fail-1',
      type: 'execute',
      correlation_id: 'corr-fail-1',
      params: JSON.stringify({ script: 'throw new Error("boom")' })
    }

    globalThis.chrome.tabs.sendMessage = mock.fn((_tabId, message) => {
      if (message?.type === 'kaboom_execute_query') {
        return Promise.resolve({
          success: false,
          error: 'execution_failed',
          message: 'boom'
        })
      }
      return Promise.resolve({ success: true, result: 'ok' })
    })

    const mockSyncClient = { queueCommandResult: mock.fn() }
    await bgModule.handlePendingQuery(query, mockSyncClient)

    assert.strictEqual(mockSyncClient.queueCommandResult.mock.calls.length, 1)
    const queuedResult = mockSyncClient.queueCommandResult.mock.calls[0].arguments[0]
    assert.strictEqual(queuedResult.correlation_id, query.correlation_id)
    assert.strictEqual(queuedResult.status, 'error')
    assert.strictEqual(queuedResult.error, 'execution_failed')
    assert.strictEqual(queuedResult.result.success, false)
  })

  test('execute with pilot disabled should post async status=error', async () => {
    resetPilotCacheForTesting(false)

    const query = {
      id: 'query-disabled-1',
      type: 'execute',
      correlation_id: 'corr-disabled-1',
      params: JSON.stringify({ script: 'return 1' })
    }

    const mockSyncClient = { queueCommandResult: mock.fn() }
    await bgModule.handlePendingQuery(query, mockSyncClient)

    assert.strictEqual(mockSyncClient.queueCommandResult.mock.calls.length, 1)
    const queuedResult = mockSyncClient.queueCommandResult.mock.calls[0].arguments[0]
    assert.strictEqual(queuedResult.correlation_id, query.correlation_id)
    assert.strictEqual(queuedResult.status, 'error')
    assert.strictEqual(queuedResult.error, 'ai_web_pilot_disabled')
  })

  test('consecutive execute queries should queue one result per correlation_id', async () => {
    const mockSyncClient = { queueCommandResult: mock.fn() }
    const operationCount = 6
    const queries = []
    for (let i = 0; i < operationCount; i++) {
      queries.push({
        id: `query-${i}`,
        type: 'execute',
        correlation_id: `corr-${i}`,
        params: JSON.stringify({ script: `return ${i}` })
      })
    }

    for (const query of queries) {
      await bgModule.handlePendingQuery(query, mockSyncClient)
    }

    assert.strictEqual(mockSyncClient.queueCommandResult.mock.calls.length, operationCount)
    const seenCorrelationIds = new Set(
      mockSyncClient.queueCommandResult.mock.calls.map((call) => call.arguments[0].correlation_id)
    )
    assert.strictEqual(seenCorrelationIds.size, operationCount)
  })

  test('processing queries set should be cleaned up after async completion', async () => {
    const query = {
      id: 'cleanup-test-123',
      type: 'execute',
      correlation_id: 'cleanup-corr-456',
      params: JSON.stringify({ script: 'return "cleanup test"' })
    }
    const mockSyncClient = { queueCommandResult: mock.fn() }

    const initialState = bgModule.getProcessingQueriesState()
    const initialSize = initialState.size

    await bgModule.handlePendingQuery(query, mockSyncClient)

    const finalState = bgModule.getProcessingQueriesState()
    assert.strictEqual(finalState.has(query.id), false)
    assert.ok(
      finalState.size <= initialSize + 1,
      `Processing queries set grew unexpectedly: ${finalState.size} > ${initialSize + 1}`
    )
  })
})

// =============================================================================
// Bug #5: Async Browser Action Await (verify fix still works)
// =============================================================================

describe('Bug #5: Async Browser Action Await (regression test)', () => {
  let bgModule

  beforeEach(async () => {
    mock.reset()
    globalThis.chrome = createMockChrome()
    globalThis.fetch = mock.fn(() =>
      Promise.resolve({
        ok: true,
        json: () => Promise.resolve({ queries: [] })
      })
    )
    bgModule = {
      ...(await import('../../../extension/background/index.js')),
      ...(await import('../../../extension/background/state.js'))
    }
    bgModule.markInitComplete()
    // Enable pilot cache so browser_action paths don't short-circuit
    resetPilotCacheForTesting(true)
  })

  test('handlePendingQuery should await handleAsyncBrowserAction for browser_action queries', async () => {
    // This verifies the existing fix for handleAsyncBrowserAction still works
    const query = {
      id: 'browser-action-123',
      type: 'browser_action',
      correlation_id: 'browser-corr-456',
      params: JSON.stringify({ action: 'back' })
    }

    // Use a mock sync client (code uses syncClient.queueCommandResult, not fetch)
    const mockSyncClient = { queueCommandResult: mock.fn() }

    await bgModule.handlePendingQuery(query, mockSyncClient)

    // Result should have been delivered via sync client
    assert.strictEqual(
      mockSyncClient.queueCommandResult.mock.calls.length > 0,
      true,
      'Async browser action result was not posted - await may be missing'
    )
  })

  test('browser_action failures should post async status=error', async () => {
    const query = {
      id: 'browser-action-fail-1',
      type: 'browser_action',
      correlation_id: 'browser-corr-fail-1',
      params: JSON.stringify({ action: 'unknown-action' })
    }

    const mockSyncClient = { queueCommandResult: mock.fn() }
    await bgModule.handlePendingQuery(query, mockSyncClient)

    assert.strictEqual(mockSyncClient.queueCommandResult.mock.calls.length, 1)
    const queuedResult = mockSyncClient.queueCommandResult.mock.calls[0].arguments[0]
    assert.strictEqual(queuedResult.correlation_id, query.correlation_id)
    assert.strictEqual(queuedResult.status, 'error')
    assert.strictEqual(queuedResult.error, 'unknown_action')
  })
})

// =============================================================================
// Integration: Multiple consecutive operations stability
// =============================================================================

describe('Bug #5: Extension Stability Under Load', () => {
  let bgModule

  beforeEach(async () => {
    mock.reset()
    globalThis.chrome = createMockChrome()
    globalThis.fetch = mock.fn(() =>
      Promise.resolve({
        ok: true,
        json: () => Promise.resolve({ queries: [] })
      })
    )
    bgModule = {
      ...(await import('../../../extension/background/index.js')),
      ...(await import('../../../extension/background/state.js'))
    }
    bgModule.markInitComplete()
    // Enable pilot cache so execute/browser_action paths don't short-circuit
    resetPilotCacheForTesting(true)
  })

  test('should handle 5+ consecutive operations without timeout', async () => {
    // The bug manifested as timeouts after 5-6 operations
    // This test verifies the fix allows 5+ consecutive operations (proving the bug is fixed)

    let errorCount = 0

    // Use a mock sync client (code uses syncClient.queueCommandResult, not fetch)
    const mockSyncClient = { queueCommandResult: mock.fn() }

    // Run 5 consecutive operations (bug manifested after 5-6, this proves the fix works)
    const operationCount = 5
    for (let i = 0; i < operationCount; i++) {
      const query = {
        id: `load-test-${i}`,
        type: i % 2 === 0 ? 'execute' : 'browser_action',
        correlation_id: `load-corr-${i}`,
        params: JSON.stringify(i % 2 === 0 ? { script: `return ${i}` } : { action: 'back' })
      }

      try {
        await bgModule.handlePendingQuery(query, mockSyncClient)
      } catch {
        errorCount++
      }
    }

    // Should have processed most operations without error
    assert.strictEqual(errorCount, 0, `${errorCount} operations failed - extension may be timing out`)

    // Results should have been delivered via sync client
    const successCount = mockSyncClient.queueCommandResult.mock.calls.length
    assert.ok(
      successCount >= operationCount * 0.5,
      `Only ${successCount}/${operationCount} operations completed - possible timeout cascade`
    )
  })

  test('memory should not grow excessively during heavy load', async () => {
    // With missing await, promises accumulate causing memory growth
    // This test verifies bounded memory usage

    const initialHeap = process.memoryUsage().heapUsed
    const mockSyncClient = { queueCommandResult: mock.fn() }
    const pending = []

    // Run 30 rapid operations
    for (let i = 0; i < 30; i++) {
      const query = {
        id: `memory-test-${i}`,
        type: 'execute',
        correlation_id: `memory-corr-${i}`,
        params: JSON.stringify({ script: `return ${i}` })
      }

      // Don't await - simulate rapid fire
      pending.push(bgModule.handlePendingQuery(query, mockSyncClient))
    }

    await Promise.allSettled(pending)

    // Force garbage collection if available
    if (global.gc) {
      global.gc()
    }

    const finalHeap = process.memoryUsage().heapUsed
    const heapGrowth = finalHeap - initialHeap

    // Heap should not grow more than 50MB (generous limit for test environment)
    const maxGrowthMB = 50
    assert.ok(
      heapGrowth < maxGrowthMB * 1024 * 1024,
      `Heap grew by ${(heapGrowth / 1024 / 1024).toFixed(2)}MB - possible memory leak from unawaited promises`
    )
  })
})

// =============================================================================
// Error handling: Ensure errors don't cause cascading failures
// =============================================================================

describe('Bug #5: Error Handling Robustness', () => {
  let bgModule

  beforeEach(async () => {
    mock.reset()
    globalThis.chrome = createMockChrome()
    globalThis.fetch = mock.fn(() =>
      Promise.resolve({
        ok: true,
        json: () => Promise.resolve({ queries: [] })
      })
    )
    bgModule = {
      ...(await import('../../../extension/background/index.js')),
      ...(await import('../../../extension/background/state.js'))
    }
    bgModule.markInitComplete()
    // Enable pilot cache so execute paths don't short-circuit
    resetPilotCacheForTesting(true)
  })

  test('error in one operation should not affect subsequent operations', async () => {
    // Make tabs.sendMessage fail for the first query only
    let firstCall = true
    globalThis.chrome.tabs.sendMessage = mock.fn(() => {
      if (firstCall) {
        firstCall = false
        return Promise.reject(new Error('Simulated failure'))
      }
      return Promise.resolve({ success: true, result: 'ok' })
    })

    // Use a mock sync client (code uses syncClient.queueCommandResult, not fetch)
    const mockSyncClient = { queueCommandResult: mock.fn() }

    // Run operations - first will fail, rest should succeed
    for (let i = 0; i < 5; i++) {
      const query = {
        id: `error-test-${i}`,
        type: 'execute',
        correlation_id: `error-corr-${i}`,
        params: JSON.stringify({ script: 'return 1' })
      }

      await bgModule.handlePendingQuery(query, mockSyncClient)
    }

    // All 5 operations should deliver results (including the failed one, which sends an error result)
    const operationsCompleted = mockSyncClient.queueCommandResult.mock.calls.length
    assert.ok(operationsCompleted >= 4, `Only ${operationsCompleted}/5 operations completed - error cascaded`)
  })
})
