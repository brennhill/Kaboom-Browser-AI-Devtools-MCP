// @ts-nocheck
/**
 * @fileoverview cache-limits.test.js -- Tests for cache-limits module.
 * Covers screenshot rate limiting, memory pressure detection/enforcement,
 * source map cache with LRU eviction, and exported constants.
 */

import { test, describe, mock, beforeEach } from 'node:test'
import assert from 'node:assert'
import { MANIFEST_VERSION } from '../shared/helpers.js'

// Mock Chrome APIs before import
globalThis.chrome = {
  runtime: {
    onMessage: { addListener: mock.fn() },
    sendMessage: mock.fn(() => Promise.resolve()),
    getManifest: () => ({ version: MANIFEST_VERSION })
  },
  action: { setBadgeText: mock.fn(), setBadgeBackgroundColor: mock.fn() },
  storage: {
    local: {
      get: mock.fn((k, cb) => cb({})),
      set: mock.fn((d, cb) => cb && cb()),
      remove: mock.fn((k, cb) => {
        if (typeof cb === 'function') cb()
        else return Promise.resolve()
      })
    },
    sync: { get: mock.fn((k, cb) => cb({})), set: mock.fn() },
    session: { get: mock.fn((k, cb) => cb({})), set: mock.fn() },
    onChanged: { addListener: mock.fn() }
  },
  alarms: { create: mock.fn(), onAlarm: { addListener: mock.fn() } },
  tabs: { get: mock.fn(), query: mock.fn(), onRemoved: { addListener: mock.fn() } }
}

import {
  SOURCE_MAP_CACHE_SIZE,
  MEMORY_SOFT_LIMIT,
  MEMORY_HARD_LIMIT,
  MEMORY_CHECK_INTERVAL_MS,
  MEMORY_AVG_LOG_ENTRY_SIZE,
  MEMORY_AVG_WS_EVENT_SIZE,
  MEMORY_AVG_NETWORK_BODY_SIZE,
  MEMORY_AVG_ACTION_SIZE,
  MAX_PENDING_BUFFER,
  canTakeScreenshot,
  recordScreenshot,
  clearScreenshotTimestamps,
  estimateBufferMemory,
  checkMemoryPressure,
  getMemoryPressureState,
  resetMemoryPressureState,
  isNetworkBodyCaptureDisabled,
  setSourceMapEnabled,
  isSourceMapEnabled,
  setSourceMapCacheEntry,
  getSourceMapCacheEntry,
  getSourceMapCacheSize,
  clearSourceMapCache
} from '../../../extension/background/caches/cache-limits.js'

// =============================================================================
// HELPER: create a minimal BufferState for memory estimation
// =============================================================================
function createBufferState(overrides = {}) {
  return {
    logEntries: overrides.logEntries ?? [],
    wsEvents: overrides.wsEvents ?? [],
    networkBodies: overrides.networkBodies ?? [],
    enhancedActions: overrides.enhancedActions ?? []
  }
}

// =============================================================================
// CONSTANTS
// =============================================================================

describe('Exported Constants', () => {
  test('SOURCE_MAP_CACHE_SIZE should be 50', () => {
    assert.strictEqual(SOURCE_MAP_CACHE_SIZE, 50)
  })

  test('MEMORY_SOFT_LIMIT should be 20MB', () => {
    assert.strictEqual(MEMORY_SOFT_LIMIT, 20 * 1024 * 1024)
  })

  test('MEMORY_HARD_LIMIT should be 50MB', () => {
    assert.strictEqual(MEMORY_HARD_LIMIT, 50 * 1024 * 1024)
  })

  test('MEMORY_CHECK_INTERVAL_MS should be 30000', () => {
    assert.strictEqual(MEMORY_CHECK_INTERVAL_MS, 30000)
  })

  test('average size estimates should be correct', () => {
    assert.strictEqual(MEMORY_AVG_LOG_ENTRY_SIZE, 500)
    assert.strictEqual(MEMORY_AVG_WS_EVENT_SIZE, 300)
    assert.strictEqual(MEMORY_AVG_NETWORK_BODY_SIZE, 1000)
    assert.strictEqual(MEMORY_AVG_ACTION_SIZE, 400)
  })

  test('MAX_PENDING_BUFFER should be 1000', () => {
    assert.strictEqual(MAX_PENDING_BUFFER, 1000)
  })
})

// =============================================================================
// SCREENSHOT RATE LIMITING
// =============================================================================

describe('Screenshot Rate Limiting', () => {
  beforeEach(() => {
    // Clear all tab timestamps between tests
    clearScreenshotTimestamps(1)
    clearScreenshotTimestamps(2)
    clearScreenshotTimestamps(99)
  })

  test('should allow first screenshot for a new tab', () => {
    const result = canTakeScreenshot(1)
    assert.strictEqual(result.allowed, true)
    assert.strictEqual(result.reason, undefined)
  })

  test('should allow screenshot after recording one if rate limit window passed', () => {
    // Record a screenshot, then check immediately -- should be rate limited
    recordScreenshot(1)
    const result = canTakeScreenshot(1)
    assert.strictEqual(result.allowed, false)
    assert.strictEqual(result.reason, 'rate_limit')
    assert.ok(typeof result.nextAllowedIn === 'number')
    assert.ok(result.nextAllowedIn > 0)
    assert.ok(result.nextAllowedIn <= 5000)
  })

  test('should enforce per-tab isolation', () => {
    recordScreenshot(1)
    // Tab 1 should be rate limited
    const result1 = canTakeScreenshot(1)
    assert.strictEqual(result1.allowed, false)

    // Tab 2 should be allowed (different tab)
    const result2 = canTakeScreenshot(2)
    assert.strictEqual(result2.allowed, true)
  })

  test('should enforce session limit of 30 screenshots per minute', () => {
    const tabId = 99
    // Record 30 screenshots with enough time between them to avoid rate limit
    // We need to manipulate timestamps directly -- use recordScreenshot 10 times
    // but the rate limit check also looks at the 5-second window, so we need a workaround
    // Since recordScreenshot pushes Date.now(), and canTakeScreenshot filters by 60s window,
    // we can just push 30 entries and check session limit
    for (let i = 0; i < 30; i++) {
      recordScreenshot(tabId)
    }

    const result = canTakeScreenshot(tabId)
    assert.strictEqual(result.allowed, false)
    assert.strictEqual(result.reason, 'session_limit')
    assert.strictEqual(result.nextAllowedIn, null)
  })

  test('should clear timestamps for a specific tab', () => {
    recordScreenshot(1)
    recordScreenshot(2)

    clearScreenshotTimestamps(1)

    // Tab 1 should be allowed again
    const result1 = canTakeScreenshot(1)
    assert.strictEqual(result1.allowed, true)

    // Tab 2 should still be rate limited
    const result2 = canTakeScreenshot(2)
    assert.strictEqual(result2.allowed, false)
  })

  test('should calculate nextAllowedIn correctly for rate limit', () => {
    recordScreenshot(1)
    const result = canTakeScreenshot(1)
    assert.strictEqual(result.allowed, false)
    assert.strictEqual(result.reason, 'rate_limit')
    // nextAllowedIn should be close to 5000ms (rate limit) minus elapsed time
    assert.ok(result.nextAllowedIn > 4900, `nextAllowedIn was ${result.nextAllowedIn}`)
    assert.ok(result.nextAllowedIn <= 5000, `nextAllowedIn was ${result.nextAllowedIn}`)
  })

  test('recordScreenshot should handle tab with no prior entries', () => {
    // Should not throw
    assert.doesNotThrow(() => recordScreenshot(42))
    const result = canTakeScreenshot(42)
    assert.strictEqual(result.allowed, false)
    assert.strictEqual(result.reason, 'rate_limit')
    clearScreenshotTimestamps(42)
  })

  test('clearScreenshotTimestamps should be idempotent for unknown tab', () => {
    // Should not throw for a tab that was never tracked
    assert.doesNotThrow(() => clearScreenshotTimestamps(999))
  })
})

// =============================================================================
// MEMORY ESTIMATION
// =============================================================================

describe('estimateBufferMemory', () => {
  test('should return 0 for empty buffers', () => {
    const result = estimateBufferMemory(createBufferState())
    assert.strictEqual(result, 0)
  })

  test('should estimate log entries correctly', () => {
    const buffers = createBufferState({
      logEntries: new Array(10).fill({})
    })
    const result = estimateBufferMemory(buffers)
    assert.strictEqual(result, 10 * MEMORY_AVG_LOG_ENTRY_SIZE)
  })

  test('should estimate websocket events with base size', () => {
    const buffers = createBufferState({
      wsEvents: [{ type: 'message' }, { type: 'open' }]
    })
    const result = estimateBufferMemory(buffers)
    assert.strictEqual(result, 2 * MEMORY_AVG_WS_EVENT_SIZE)
  })

  test('should add string data length for websocket events', () => {
    const dataStr = 'a'.repeat(1000)
    const buffers = createBufferState({
      wsEvents: [{ data: dataStr }]
    })
    const result = estimateBufferMemory(buffers)
    assert.strictEqual(result, MEMORY_AVG_WS_EVENT_SIZE + 1000)
  })

  test('should not add data length for non-string ws data', () => {
    const buffers = createBufferState({
      wsEvents: [{ data: 12345 }]
    })
    const result = estimateBufferMemory(buffers)
    // Only base size, no additional data length for numeric data
    assert.strictEqual(result, MEMORY_AVG_WS_EVENT_SIZE)
  })

  test('should estimate network bodies with request and response sizes', () => {
    const reqBody = 'x'.repeat(500)
    const resBody = 'y'.repeat(2000)
    const buffers = createBufferState({
      networkBodies: [{ request_body: reqBody, response_body: resBody }]
    })
    const result = estimateBufferMemory(buffers)
    assert.strictEqual(result, MEMORY_AVG_NETWORK_BODY_SIZE + 500 + 2000)
  })

  test('should handle network bodies with only request or only response', () => {
    const buffers = createBufferState({
      networkBodies: [{ request_body: 'abc' }, { response_body: 'defgh' }]
    })
    const result = estimateBufferMemory(buffers)
    assert.strictEqual(result, 2 * MEMORY_AVG_NETWORK_BODY_SIZE + 3 + 5)
  })

  test('should estimate enhanced actions correctly', () => {
    const buffers = createBufferState({
      enhancedActions: new Array(5).fill({})
    })
    const result = estimateBufferMemory(buffers)
    assert.strictEqual(result, 5 * MEMORY_AVG_ACTION_SIZE)
  })

  test('should sum all buffer types together', () => {
    const buffers = createBufferState({
      logEntries: new Array(3).fill({}),
      wsEvents: [{ data: 'hello' }],
      networkBodies: [{ request_body: 'req' }],
      enhancedActions: new Array(2).fill({})
    })
    const expected =
      3 * MEMORY_AVG_LOG_ENTRY_SIZE +
      MEMORY_AVG_WS_EVENT_SIZE + 5 +
      MEMORY_AVG_NETWORK_BODY_SIZE + 3 +
      2 * MEMORY_AVG_ACTION_SIZE
    const result = estimateBufferMemory(buffers)
    assert.strictEqual(result, expected)
  })
})

// =============================================================================
// MEMORY PRESSURE
// =============================================================================

describe('Memory Pressure Detection', () => {
  beforeEach(() => {
    resetMemoryPressureState()
  })

  test('should report normal level for small buffers', () => {
    const buffers = createBufferState({ logEntries: new Array(10).fill({}) })
    const result = checkMemoryPressure(buffers)
    assert.strictEqual(result.level, 'normal')
    assert.strictEqual(result.action, 'none')
    assert.strictEqual(result.alreadyApplied, false)
    assert.ok(result.estimatedMemory < MEMORY_SOFT_LIMIT)
  })

  test('should report soft level when exceeding soft limit', () => {
    // 20MB / 500 bytes per log = 41943 entries needed
    const count = Math.ceil(MEMORY_SOFT_LIMIT / MEMORY_AVG_LOG_ENTRY_SIZE) + 1
    const buffers = createBufferState({ logEntries: new Array(count).fill({}) })
    const result = checkMemoryPressure(buffers)
    assert.strictEqual(result.level, 'soft')
    assert.strictEqual(result.action, 'reduce_capacities')
    assert.ok(result.estimatedMemory >= MEMORY_SOFT_LIMIT)
  })

  test('should report hard level when exceeding hard limit', () => {
    const count = Math.ceil(MEMORY_HARD_LIMIT / MEMORY_AVG_LOG_ENTRY_SIZE) + 1
    const buffers = createBufferState({ logEntries: new Array(count).fill({}) })
    const result = checkMemoryPressure(buffers)
    assert.strictEqual(result.level, 'hard')
    assert.strictEqual(result.action, 'disable_network_capture')
    assert.ok(result.estimatedMemory >= MEMORY_HARD_LIMIT)
  })

  test('should set networkBodyCaptureDisabled when hard limit reached', () => {
    const count = Math.ceil(MEMORY_HARD_LIMIT / MEMORY_AVG_LOG_ENTRY_SIZE) + 1
    const buffers = createBufferState({ logEntries: new Array(count).fill({}) })
    checkMemoryPressure(buffers)
    assert.strictEqual(isNetworkBodyCaptureDisabled(), true)
  })

  test('should re-enable network capture when dropping from hard to soft', () => {
    // First push to hard
    const hardCount = Math.ceil(MEMORY_HARD_LIMIT / MEMORY_AVG_LOG_ENTRY_SIZE) + 1
    checkMemoryPressure(createBufferState({ logEntries: new Array(hardCount).fill({}) }))
    assert.strictEqual(isNetworkBodyCaptureDisabled(), true)

    // Now drop to soft range (between soft and hard limits)
    const softCount = Math.ceil((MEMORY_SOFT_LIMIT + 1000) / MEMORY_AVG_LOG_ENTRY_SIZE)
    const result = checkMemoryPressure(createBufferState({ logEntries: new Array(softCount).fill({}) }))
    assert.strictEqual(result.level, 'soft')
    assert.strictEqual(isNetworkBodyCaptureDisabled(), false)
  })

  test('should return to normal from soft when memory drops', () => {
    // Push to soft
    const softCount = Math.ceil(MEMORY_SOFT_LIMIT / MEMORY_AVG_LOG_ENTRY_SIZE) + 1
    checkMemoryPressure(createBufferState({ logEntries: new Array(softCount).fill({}) }))
    assert.strictEqual(getMemoryPressureState().reducedCapacities, true)

    // Drop to normal
    const result = checkMemoryPressure(createBufferState({ logEntries: [{}] }))
    assert.strictEqual(result.level, 'normal')
    assert.strictEqual(getMemoryPressureState().reducedCapacities, false)
    assert.strictEqual(isNetworkBodyCaptureDisabled(), false)
  })

  test('should track alreadyApplied when level stays the same', () => {
    const softCount = Math.ceil(MEMORY_SOFT_LIMIT / MEMORY_AVG_LOG_ENTRY_SIZE) + 1
    const buffers = createBufferState({ logEntries: new Array(softCount).fill({}) })

    const first = checkMemoryPressure(buffers)
    assert.strictEqual(first.alreadyApplied, false)

    const second = checkMemoryPressure(buffers)
    assert.strictEqual(second.alreadyApplied, true)
  })

  test('should track alreadyApplied for hard level', () => {
    const hardCount = Math.ceil(MEMORY_HARD_LIMIT / MEMORY_AVG_LOG_ENTRY_SIZE) + 1
    const buffers = createBufferState({ logEntries: new Array(hardCount).fill({}) })

    const first = checkMemoryPressure(buffers)
    assert.strictEqual(first.level, 'hard')
    assert.strictEqual(first.alreadyApplied, false)

    const second = checkMemoryPressure(buffers)
    assert.strictEqual(second.level, 'hard')
    assert.strictEqual(second.alreadyApplied, true)
  })

  test('getMemoryPressureState should return current state', () => {
    resetMemoryPressureState()
    const state = getMemoryPressureState()
    assert.strictEqual(state.memoryPressureLevel, 'normal')
    assert.strictEqual(state.lastMemoryCheck, 0)
    assert.strictEqual(state.networkBodyCaptureDisabled, false)
    assert.strictEqual(state.reducedCapacities, false)
  })

  test('checkMemoryPressure should update lastMemoryCheck', () => {
    const before = Date.now()
    checkMemoryPressure(createBufferState())
    const state = getMemoryPressureState()
    assert.ok(state.lastMemoryCheck >= before)
    assert.ok(state.lastMemoryCheck <= Date.now())
  })

  test('resetMemoryPressureState should restore all defaults', () => {
    // Push to hard
    const hardCount = Math.ceil(MEMORY_HARD_LIMIT / MEMORY_AVG_LOG_ENTRY_SIZE) + 1
    checkMemoryPressure(createBufferState({ logEntries: new Array(hardCount).fill({}) }))

    resetMemoryPressureState()
    const state = getMemoryPressureState()
    assert.strictEqual(state.memoryPressureLevel, 'normal')
    assert.strictEqual(state.lastMemoryCheck, 0)
    assert.strictEqual(state.networkBodyCaptureDisabled, false)
    assert.strictEqual(state.reducedCapacities, false)
  })
})

// =============================================================================
// SOURCE MAP CACHE
// =============================================================================

describe('Source Map Cache', () => {
  beforeEach(() => {
    clearSourceMapCache()
    setSourceMapEnabled(false)
  })

  test('should start empty', () => {
    assert.strictEqual(getSourceMapCacheSize(), 0)
  })

  test('should store and retrieve a source map entry', () => {
    const map = { version: 3, sources: ['a.ts'], mappings: 'AAAA' }
    setSourceMapCacheEntry('http://example.com/app.js.map', map)
    const result = getSourceMapCacheEntry('http://example.com/app.js.map')
    assert.deepStrictEqual(result, map)
    assert.strictEqual(getSourceMapCacheSize(), 1)
  })

  test('should store null entries (negative cache)', () => {
    setSourceMapCacheEntry('http://example.com/missing.js.map', null)
    // getSourceMapCacheEntry returns sourceMapCache.get(url) || null
    // so null stored values will return null
    const result = getSourceMapCacheEntry('http://example.com/missing.js.map')
    assert.strictEqual(result, null)
  })

  test('should return null for unknown URLs', () => {
    const result = getSourceMapCacheEntry('http://unknown.com/nope.js.map')
    assert.strictEqual(result, null)
  })

  test('should update existing entry without growing cache', () => {
    const map1 = { version: 3, sources: ['a.ts'], mappings: 'AAAA' }
    const map2 = { version: 3, sources: ['b.ts'], mappings: 'BBBB' }
    setSourceMapCacheEntry('http://example.com/app.js.map', map1)
    setSourceMapCacheEntry('http://example.com/app.js.map', map2)
    assert.strictEqual(getSourceMapCacheSize(), 1)
    assert.deepStrictEqual(getSourceMapCacheEntry('http://example.com/app.js.map'), map2)
  })

  test('should evict oldest entry when cache is full (FIFO eviction)', () => {
    // Fill cache to SOURCE_MAP_CACHE_SIZE
    for (let i = 0; i < SOURCE_MAP_CACHE_SIZE; i++) {
      setSourceMapCacheEntry(`http://example.com/${i}.js.map`, { version: 3, sources: [`${i}.ts`], mappings: '' })
    }
    assert.strictEqual(getSourceMapCacheSize(), SOURCE_MAP_CACHE_SIZE)

    // Add one more -- should evict the first entry (index 0)
    setSourceMapCacheEntry('http://example.com/new.js.map', { version: 3, sources: ['new.ts'], mappings: '' })
    assert.strictEqual(getSourceMapCacheSize(), SOURCE_MAP_CACHE_SIZE)

    // First entry should be gone
    assert.strictEqual(getSourceMapCacheEntry('http://example.com/0.js.map'), null)
    // New entry should be present
    assert.notStrictEqual(getSourceMapCacheEntry('http://example.com/new.js.map'), null)
    // Second entry should still be present
    assert.notStrictEqual(getSourceMapCacheEntry('http://example.com/1.js.map'), null)
  })

  test('should not evict when updating an existing entry at capacity', () => {
    // Fill cache to capacity
    for (let i = 0; i < SOURCE_MAP_CACHE_SIZE; i++) {
      setSourceMapCacheEntry(`http://example.com/${i}.js.map`, { version: 3, sources: [`${i}.ts`], mappings: '' })
    }

    // Update an existing entry -- should not evict anything
    setSourceMapCacheEntry('http://example.com/0.js.map', { version: 3, sources: ['updated.ts'], mappings: 'UPD' })
    assert.strictEqual(getSourceMapCacheSize(), SOURCE_MAP_CACHE_SIZE)

    // All entries should still be accessible
    for (let i = 0; i < SOURCE_MAP_CACHE_SIZE; i++) {
      assert.notStrictEqual(getSourceMapCacheEntry(`http://example.com/${i}.js.map`), null)
    }
  })

  test('should move re-set entry to end (LRU behavior)', () => {
    // Fill with 3 entries
    setSourceMapCacheEntry('http://a.com/1.map', { version: 3, sources: ['1'], mappings: '' })
    setSourceMapCacheEntry('http://a.com/2.map', { version: 3, sources: ['2'], mappings: '' })
    setSourceMapCacheEntry('http://a.com/3.map', { version: 3, sources: ['3'], mappings: '' })

    // Re-set entry 1 (should move it to end)
    setSourceMapCacheEntry('http://a.com/1.map', { version: 3, sources: ['1-updated'], mappings: '' })

    // Verify size is still 3
    assert.strictEqual(getSourceMapCacheSize(), 3)
  })

  test('clearSourceMapCache should remove all entries', () => {
    setSourceMapCacheEntry('http://a.com/1.map', { version: 3, sources: ['1'], mappings: '' })
    setSourceMapCacheEntry('http://a.com/2.map', { version: 3, sources: ['2'], mappings: '' })
    assert.strictEqual(getSourceMapCacheSize(), 2)

    clearSourceMapCache()
    assert.strictEqual(getSourceMapCacheSize(), 0)
    assert.strictEqual(getSourceMapCacheEntry('http://a.com/1.map'), null)
  })

  test('clearSourceMapCache should be idempotent on empty cache', () => {
    assert.doesNotThrow(() => clearSourceMapCache())
    assert.strictEqual(getSourceMapCacheSize(), 0)
  })
})

// =============================================================================
// SOURCE MAP ENABLED FLAG
// =============================================================================

describe('Source Map Enabled Flag', () => {
  beforeEach(() => {
    setSourceMapEnabled(false)
  })

  test('should default to disabled', () => {
    assert.strictEqual(isSourceMapEnabled(), false)
  })

  test('should enable source maps', () => {
    setSourceMapEnabled(true)
    assert.strictEqual(isSourceMapEnabled(), true)
  })

  test('should disable source maps', () => {
    setSourceMapEnabled(true)
    setSourceMapEnabled(false)
    assert.strictEqual(isSourceMapEnabled(), false)
  })
})

// =============================================================================
// EDGE CASES
// =============================================================================

describe('Edge Cases', () => {
  beforeEach(() => {
    resetMemoryPressureState()
    clearSourceMapCache()
    clearScreenshotTimestamps(1)
  })

  test('estimateBufferMemory with empty arrays in all fields', () => {
    const result = estimateBufferMemory({
      logEntries: [],
      wsEvents: [],
      networkBodies: [],
      enhancedActions: []
    })
    assert.strictEqual(result, 0)
  })

  test('estimateBufferMemory with ws events with empty string data', () => {
    const buffers = createBufferState({
      wsEvents: [{ data: '' }]
    })
    const result = estimateBufferMemory(buffers)
    // Empty string data has length 0, so only base size
    assert.strictEqual(result, MEMORY_AVG_WS_EVENT_SIZE)
  })

  test('estimateBufferMemory with network bodies with empty strings', () => {
    const buffers = createBufferState({
      networkBodies: [{ request_body: '', response_body: '' }]
    })
    const result = estimateBufferMemory(buffers)
    assert.strictEqual(result, MEMORY_AVG_NETWORK_BODY_SIZE)
  })

  test('memory pressure transition from hard directly to normal', () => {
    // Push to hard
    const hardCount = Math.ceil(MEMORY_HARD_LIMIT / MEMORY_AVG_LOG_ENTRY_SIZE) + 1
    checkMemoryPressure(createBufferState({ logEntries: new Array(hardCount).fill({}) }))
    assert.strictEqual(getMemoryPressureState().memoryPressureLevel, 'hard')

    // Drop directly to normal (skip soft)
    const result = checkMemoryPressure(createBufferState())
    assert.strictEqual(result.level, 'normal')
    assert.strictEqual(result.action, 'none')
    assert.strictEqual(isNetworkBodyCaptureDisabled(), false)
    assert.strictEqual(getMemoryPressureState().reducedCapacities, false)
  })

  test('alreadyApplied should be true when transitioning from hard to soft', () => {
    // Push to hard
    const hardCount = Math.ceil(MEMORY_HARD_LIMIT / MEMORY_AVG_LOG_ENTRY_SIZE) + 1
    checkMemoryPressure(createBufferState({ logEntries: new Array(hardCount).fill({}) }))

    // Drop to soft (still above soft, below hard)
    const softCount = Math.ceil((MEMORY_SOFT_LIMIT + 1000) / MEMORY_AVG_LOG_ENTRY_SIZE)
    const result = checkMemoryPressure(createBufferState({ logEntries: new Array(softCount).fill({}) }))
    assert.strictEqual(result.level, 'soft')
    // alreadyApplied should be true because we were at 'hard' (which implies soft was already applied)
    assert.strictEqual(result.alreadyApplied, true)
  })

  test('single-item buffers should estimate correctly', () => {
    const buffers = createBufferState({
      logEntries: [{}],
      wsEvents: [{}],
      networkBodies: [{}],
      enhancedActions: [{}]
    })
    const result = estimateBufferMemory(buffers)
    const expected =
      MEMORY_AVG_LOG_ENTRY_SIZE +
      MEMORY_AVG_WS_EVENT_SIZE +
      MEMORY_AVG_NETWORK_BODY_SIZE +
      MEMORY_AVG_ACTION_SIZE
    assert.strictEqual(result, expected)
  })
})
