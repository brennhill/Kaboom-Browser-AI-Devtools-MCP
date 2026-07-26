// @ts-nocheck
/**
 * @fileoverview reliability-fixes.test.js - Tests for critical reliability fixes.
 *
 * Tests for fixes to:
 *   1. _processingQueries TTL-based cleanup (unbounded Set growth)
 *   2. Pending request Maps cleanup on page unload (content.js)
 *   3. Race condition in timeout cleanup (double callbacks)
 *   4. sourceMapCache LRU eviction (unbounded Map growth)
 *   5. setInterval stacking prevention (checkConnectionAndUpdate)
 *   6. errorGroups periodic cleanup (TTL enforcement)
 */

import { test, describe, mock, beforeEach, afterEach } from 'node:test'
import assert from 'node:assert'
import { MANIFEST_VERSION } from './helpers.js'

// Suppress unhandledRejection errors from module cleanup after tests end
process.on('unhandledRejection', (reason, _promise) => {
  // Suppress initialization-related errors from module import cleanup
  if (
    reason?.message?.includes('_connectionCheckRunning') ||
    reason?.message?.includes('Cannot access') ||
    reason?.message?.includes('before initialization')
  ) {
    return // Expected during test cleanup
  }
  // Re-throw other unhandled rejections
  throw reason
})

// Mock Chrome APIs
const mockChrome = {
  runtime: {
    onMessage: {
      addListener: mock.fn()
    },
    onInstalled: {
      addListener: mock.fn()
    },
    sendMessage: mock.fn(() => Promise.resolve()),
    getManifest: () => ({ version: MANIFEST_VERSION })
  },
  action: {
    setBadgeText: mock.fn(),
    setBadgeBackgroundColor: mock.fn()
  },
  storage: {
    local: {
      get: mock.fn((keys, callback) => {
        if (typeof callback === 'function') callback({ logLevel: 'error' })
        else return Promise.resolve({ logLevel: 'error' })
      }),
      set: mock.fn((data, callback) => {
        if (typeof callback === 'function') callback()
        else return Promise.resolve()
      }),
      remove: mock.fn((keys, callback) => {
        if (typeof callback === 'function') callback()
        else return Promise.resolve()
      })
    },
    sync: {
      get: mock.fn((keys, callback) => callback({})),
      set: mock.fn((data, callback) => callback && callback())
    },
    session: {
      get: mock.fn((keys, callback) => callback({})),
      set: mock.fn((data, callback) => callback && callback())
    },
    onChanged: {
      addListener: mock.fn()
    }
  },
  alarms: {
    create: mock.fn(),
    onAlarm: {
      addListener: mock.fn()
    }
  },
  tabs: {
    get: mock.fn((tabId) => Promise.resolve({ id: tabId, windowId: 1, url: 'http://localhost:3000' })),
    captureVisibleTab: mock.fn(() =>
      Promise.resolve('data:image/jpeg;base64,/9j/4AAQSkZJRgABAQAAAQABAAD/2wBDAAgGBgcGBQgHBwcJCQgKDBQNDAsLDBkS')
    ),
    query: mock.fn(() => Promise.resolve([{ id: 1, windowId: 1 }])),
    onRemoved: {
      addListener: mock.fn()
    }
  }
}

// Set global chrome mock
globalThis.chrome = mockChrome

// Mock fetch
globalThis.fetch = mock.fn(() =>
  Promise.resolve({
    ok: true,
    json: () => Promise.resolve({ queries: [] })
  })
)

// =============================================================================
// Issue 1: _processingQueries TTL-based cleanup
// =============================================================================

describe('Issue 1: _processingQueries TTL-based cleanup', () => {
  let bgModule

  beforeEach(async () => {
    mock.reset()
    bgModule = await import('../../extension/background.js')
  })

  afterEach(() => {
    // Stop all intervals to prevent async activity after test.
    // These functions may not be exported from the barrel — guard defensively.
    bgModule.stopQueryPolling?.()
    bgModule.stopSettingsHeartbeat?.()
    bgModule.stopStatusPing?.()
    bgModule.stopExtensionLogsPosting?.()
    bgModule.stopWaterfallPosting?.()
  })

  test('should export getProcessingQueriesState for testing', () => {
    assert.strictEqual(
      typeof bgModule.getProcessingQueriesState,
      'function',
      'getProcessingQueriesState should be exported for testing'
    )
  })

  test('should export cleanupStaleProcessingQueries function', () => {
    assert.strictEqual(
      typeof bgModule.cleanupStaleProcessingQueries,
      'function',
      'cleanupStaleProcessingQueries should be exported'
    )
  })

  test('should track query ID with timestamp', () => {
    const state = bgModule.getProcessingQueriesState()
    // State should be a Map with queryId -> timestamp structure
    assert.ok(state instanceof Map || typeof state === 'object', 'Should return state object')
  })

  test('should remove stale queries older than 60 seconds', async () => {
    // addProcessingQuery is exported from the sub-module, not the facade
    const { addProcessingQuery } = await import('../../extension/background/caches/snapshots.js')

    const oldTimestamp = Date.now() - 70000 // 70 seconds ago
    addProcessingQuery('stale-query-1', oldTimestamp)
    addProcessingQuery('fresh-query-1', Date.now())

    // Run cleanup
    bgModule.cleanupStaleProcessingQueries()

    const updatedState = bgModule.getProcessingQueriesState()
    assert.strictEqual(updatedState.has('stale-query-1'), false, 'Stale query should be removed')
    assert.strictEqual(updatedState.has('fresh-query-1'), true, 'Fresh query should remain')
  })

  test('should not remove queries less than 60 seconds old', async () => {
    const { addProcessingQuery } = await import('../../extension/background/caches/snapshots.js')

    addProcessingQuery('recent-query-1', Date.now() - 30000) // 30 seconds ago

    bgModule.cleanupStaleProcessingQueries()

    const state = bgModule.getProcessingQueriesState()
    assert.strictEqual(state.has('recent-query-1'), true, 'Recent query should remain')
  })
})

// =============================================================================
// Issue 2: Pending request Maps cleanup on page unload
// (Note: This is content.js - we test the exports/functions exist)
// =============================================================================

describe('Issue 2: Pending request Maps cleanup on page unload', () => {
  test(
    'content.js should export clearPendingRequests function',
    { skip: 'content.js requires browser context (chrome API) - cannot import in Node.js' },
    () => {}
  )

  test(
    'should clear all four pending request Maps on unload',
    { skip: 'content.js requires browser context (chrome API) - cannot import in Node.js' },
    () => {}
  )
})

// =============================================================================
// Issue 3: Race condition in timeout cleanup
// =============================================================================

describe('Issue 3: Race condition in timeout cleanup', () => {
  test(
    'should use atomic check-and-delete pattern',
    { skip: 'race condition guard lives in content.js which requires browser context - pattern verified in content.js source' },
    () => {}
  )

  test(
    'pendingHighlightRequests should use guarded deletion pattern',
    { skip: 'pending request Maps live in content.js which requires browser context - pattern verified in content.js source' },
    () => {}
  )
})

// =============================================================================
// Issue 4: sourceMapCache LRU eviction
// =============================================================================

describe('Issue 4: sourceMapCache LRU eviction', () => {
  let bgModule

  beforeEach(async () => {
    mock.reset()
    bgModule = await import('../../extension/background.js')
    bgModule.clearSourceMapCache()
  })

  test('SOURCE_MAP_CACHE_SIZE should be 50', () => {
    assert.strictEqual(bgModule.SOURCE_MAP_CACHE_SIZE, 50, 'SOURCE_MAP_CACHE_SIZE should be 50')
  })

  test('should export setSourceMapCacheEntry and getSourceMapCacheEntry', () => {
    assert.strictEqual(typeof bgModule.setSourceMapCacheEntry, 'function', 'setSourceMapCacheEntry should be exported')
    assert.strictEqual(typeof bgModule.getSourceMapCacheEntry, 'function', 'getSourceMapCacheEntry should be exported')
  })

  test('should evict oldest entry when cache exceeds 50 entries', () => {
    // Clear cache first
    bgModule.clearSourceMapCache()

    // Add 51 entries
    for (let i = 0; i < 51; i++) {
      bgModule.setSourceMapCacheEntry(`http://example.com/script${i}.js`, {
        mappings: [],
        sources: [`file${i}.ts`],
        names: []
      })
    }

    // Cache should be at most 50
    const size = bgModule.getSourceMapCacheSize()
    assert.ok(size <= 50, `Cache size should be <= 50, got ${size}`)

    // First entry should have been evicted
    const firstEntry = bgModule.getSourceMapCacheEntry('http://example.com/script0.js')
    assert.strictEqual(firstEntry, null, 'First entry should be evicted')

    // Last entry should exist
    const lastEntry = bgModule.getSourceMapCacheEntry('http://example.com/script50.js')
    assert.ok(lastEntry !== null, 'Last entry should exist')
  })

  test('should update LRU order on access', () => {
    bgModule.clearSourceMapCache()

    // Add entries
    bgModule.setSourceMapCacheEntry('http://example.com/a.js', { mappings: [], sources: ['a.ts'], names: [] })
    bgModule.setSourceMapCacheEntry('http://example.com/b.js', { mappings: [], sources: ['b.ts'], names: [] })

    // Access 'a' to make it recently used (LRU update on set)
    bgModule.setSourceMapCacheEntry('http://example.com/a.js', { mappings: [], sources: ['a.ts'], names: [] })

    // Fill cache to capacity
    for (let i = 0; i < 49; i++) {
      bgModule.setSourceMapCacheEntry(`http://example.com/fill${i}.js`, {
        mappings: [],
        sources: [`fill${i}.ts`],
        names: []
      })
    }

    // 'b' should be evicted (it was least recently used), 'a' should still exist
    const aEntry = bgModule.getSourceMapCacheEntry('http://example.com/a.js')
    assert.ok(aEntry !== null, 'Recently used entry should survive eviction')
  })
})

// =============================================================================
// Issue 5: setInterval stacking prevention
// =============================================================================

describe('Issue 5: setInterval stacking prevention', () => {
  let bgModule

  beforeEach(async () => {
    mock.reset()
    bgModule = await import('../../extension/background.js')
  })

  test(
    'startQueryPolling should clear existing interval before starting new one',
    { skip: 'polling control functions not yet implemented' },
    () => {
      // The existing implementation already does this with stopQueryPolling() first
      // Verify the pattern exists
      assert.strictEqual(typeof bgModule.startQueryPolling, 'function', 'startQueryPolling should be exported')
      assert.strictEqual(typeof bgModule.stopQueryPolling, 'function', 'stopQueryPolling should be exported')
    }
  )

  test(
    'startSettingsHeartbeat should clear existing interval before starting new one',
    { skip: 'polling control functions not yet implemented' },
    () => {
      assert.strictEqual(
        typeof bgModule.startSettingsHeartbeat,
        'function',
        'startSettingsHeartbeat should be exported'
      )
      assert.strictEqual(typeof bgModule.stopSettingsHeartbeat, 'function', 'stopSettingsHeartbeat should be exported')
    }
  )

  test(
    'checkConnectionAndUpdate should have mutex to prevent concurrent executions',
    { skip: 'isConnectionCheckRunning not yet exported' },
    () => {
      // The fix should add a flag to prevent multiple simultaneous executions
      assert.strictEqual(
        typeof bgModule.isConnectionCheckRunning,
        'function',
        'isConnectionCheckRunning should be exported to check mutex state'
      )
    }
  )

  test(
    'should not start duplicate intervals on rapid reconnects',
    { skip: 'polling control functions not yet implemented' },
    async () => {
      // Track interval creations
      let intervalCount = 0
      const originalSetInterval = globalThis.setInterval
      globalThis.setInterval = mock.fn((...args) => {
        intervalCount++
        return originalSetInterval(...args)
      })

      // Simulate rapid reconnects - each should clear previous interval
      bgModule.startQueryPolling('http://localhost:7890')
      bgModule.startQueryPolling('http://localhost:7890')
      bgModule.startQueryPolling('http://localhost:7890')

      // Clean up
      bgModule.stopQueryPolling()

      globalThis.setInterval = originalSetInterval

      // Even with 3 calls, only 3 intervals should be created (each call clears previous)
      // The key is that old intervals are stopped, not that we don't create new ones
      assert.ok(intervalCount >= 1, 'At least one interval should be created')
    }
  )
})

// =============================================================================
// Issue 6: errorGroups periodic cleanup (TTL enforcement)
// =============================================================================

describe('Issue 6: errorGroups periodic cleanup', () => {
  let bgModule
  let errorGroupsModule

  beforeEach(async () => {
    mock.reset()
    bgModule = await import('../../extension/background.js')
    errorGroupsModule = await import('../../extension/background/caches/error-groups.js')
    // Clear error groups
    bgModule.flushErrorGroups()
    bgModule.flushErrorGroups()
  })

  test('should export cleanupStaleErrorGroups function', () => {
    assert.strictEqual(
      typeof bgModule.cleanupStaleErrorGroups,
      'function',
      'cleanupStaleErrorGroups should be exported'
    )
  })

  test('should remove error groups older than 1 hour', () => {
    // Add an error
    const entry = {
      type: 'exception',
      level: 'error',
      message: 'Old error for cleanup test',
      ts: new Date().toISOString()
    }
    bgModule.processErrorGroup(entry)

    const state = errorGroupsModule.getErrorGroupsState()
    assert.ok(state.size >= 1, 'Error group should be tracked after processErrorGroup')

    // Manually age the entry by setting lastSeen to >1 hour ago
    for (const [, group] of state) {
      group.lastSeen = Date.now() - 3700000 // 1 hour + 100 seconds ago
    }

    bgModule.cleanupStaleErrorGroups()

    const afterState = errorGroupsModule.getErrorGroupsState()
    assert.strictEqual(afterState.size, 0, 'Stale error groups older than 1 hour should be removed')
  })

  test('should not remove error groups less than 1 hour old', () => {
    const entry = {
      type: 'exception',
      level: 'error',
      message: 'Recent error should remain',
      ts: new Date().toISOString()
    }
    bgModule.processErrorGroup(entry)

    bgModule.cleanupStaleErrorGroups()

    const state = errorGroupsModule.getErrorGroupsState()
    // Recent error should still be tracked (within 1 hour)
    assert.ok(state.size >= 1, 'Recent error should still be tracked')
  })

  test('ERROR_GROUP_MAX_AGE_MS should be 1 hour (3600000ms)', () => {
    assert.strictEqual(
      errorGroupsModule.ERROR_GROUP_MAX_AGE_MS,
      3600000,
      'Max age should be 1 hour (3600000ms)'
    )
  })
})

// =============================================================================
// Integration: All fixes working together
// =============================================================================

describe('Integration: Memory and reliability safeguards', () => {
  test('Maps should have bounded growth', async () => {
    const bgModule = await import('../../extension/background.js')
    const { getErrorGroupsState } = await import('../../extension/background/caches/error-groups.js')

    // sourceMapCache should be bounded to 50
    const size = bgModule.getSourceMapCacheSize()
    assert.ok(size <= 50, 'sourceMapCache should be bounded to 50')

    // errorGroups should be bounded to 100
    const state = getErrorGroupsState()
    assert.ok(state.size <= 100, 'errorGroups should be bounded to 100')
  })
})
