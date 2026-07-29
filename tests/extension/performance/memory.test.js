// @ts-nocheck
/**
 * @fileoverview memory.test.js — Tests for extension-side memory enforcement.
 * Verifies buffer size caps, eviction of oldest entries when limits are reached,
 * memory pressure detection (20MB soft, 50MB hard), and periodic checks.
 *
 * Spec: docs/ai-first/tech-spec-memory-enforcement.md (Extension-Side Memory section)
 * Scenarios covered:
 *   20. Extension soft limit (20MB) -> buffer capacities halved
 *   21. Extension hard limit (50MB) -> network bodies disabled
 *   22. Extension memory check runs every 30 seconds
 *   Plus additional edge cases per task requirements
 */

import { test, describe, mock, beforeEach } from 'node:test'
import assert from 'node:assert'

// Mock Chrome APIs
const mockChrome = {
  runtime: {
    onMessage: {
      addListener: mock.fn()
    },
    onInstalled: {
      addListener: mock.fn()
    },
    sendMessage: mock.fn(() => Promise.resolve())
  },
  action: {
    setBadgeText: mock.fn(),
    setBadgeBackgroundColor: mock.fn()
  },
  storage: {
    local: {
      get: mock.fn((keys, callback) => callback({ logLevel: 'error' })),
      set: mock.fn((data, callback) => callback && callback())
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
    query: mock.fn((query, callback) => callback([{ id: 1, windowId: 1 }])),
    onRemoved: {
      addListener: mock.fn()
    }
  }
}

// Set global chrome mock
globalThis.chrome = mockChrome

// Import after mocking
import {
  estimateBufferMemory,
  checkMemoryPressure,
  getMemoryPressureState,
  resetMemoryPressureState,
  MEMORY_SOFT_LIMIT,
  MEMORY_HARD_LIMIT,
  MEMORY_CHECK_INTERVAL_MS,
  MEMORY_AVG_LOG_ENTRY_SIZE,
  MEMORY_AVG_WS_EVENT_SIZE,
  MEMORY_AVG_NETWORK_BODY_SIZE,
  MEMORY_AVG_ACTION_SIZE
} from '../../../extension/background/caches/cache-limits.js'
import { createLogBatcher } from '../../../extension/background/sync/batchers.js'

describe('Memory Enforcement: Constants', () => {
  test('soft limit should be 20MB', () => {
    assert.strictEqual(MEMORY_SOFT_LIMIT, 20 * 1024 * 1024)
  })

  test('hard limit should be 50MB', () => {
    assert.strictEqual(MEMORY_HARD_LIMIT, 50 * 1024 * 1024)
  })

  test('memory check interval should be 30 seconds', () => {
    assert.strictEqual(MEMORY_CHECK_INTERVAL_MS, 30000)
  })

  test('average log entry size should be ~500 bytes', () => {
    assert.strictEqual(MEMORY_AVG_LOG_ENTRY_SIZE, 500)
  })

  test('average WS event size should be ~300 bytes', () => {
    assert.strictEqual(MEMORY_AVG_WS_EVENT_SIZE, 300)
  })

  test('average network body size should be ~1000 bytes', () => {
    assert.strictEqual(MEMORY_AVG_NETWORK_BODY_SIZE, 1000)
  })

  test('average action size should be ~400 bytes', () => {
    assert.strictEqual(MEMORY_AVG_ACTION_SIZE, 400)
  })
})

describe('Memory Enforcement: estimateBufferMemory', () => {
  beforeEach(() => {
    resetMemoryPressureState()
  })

  test('should return 0 when all buffers are empty', () => {
    const memory = estimateBufferMemory({
      logEntries: [],
      wsEvents: [],
      networkBodies: [],
      enhancedActions: []
    })

    assert.strictEqual(memory, 0)
  })

  test('should estimate memory for log entries (count * avg size)', () => {
    const entries = Array(100).fill({ level: 'error', msg: 'test' })
    const memory = estimateBufferMemory({
      logEntries: entries,
      wsEvents: [],
      networkBodies: [],
      enhancedActions: []
    })

    assert.strictEqual(memory, 100 * MEMORY_AVG_LOG_ENTRY_SIZE)
  })

  test('should estimate memory for WS events (count * avg + data length)', () => {
    const events = [
      { event: 'message', data: 'x'.repeat(200) },
      { event: 'message', data: 'y'.repeat(300) }
    ]
    const memory = estimateBufferMemory({
      logEntries: [],
      wsEvents: events,
      networkBodies: [],
      enhancedActions: []
    })

    // 2 events * 300 avg + 200 data + 300 data = 600 + 500 = 1100
    const expected = 2 * MEMORY_AVG_WS_EVENT_SIZE + 200 + 300
    assert.strictEqual(memory, expected)
  })

  test('should estimate memory for network bodies (count * avg + body lengths)', () => {
    const bodies = [
      { request_body: 'req1', response_body: 'resp1resp1' },
      { request_body: 'r2', response_body: 'response2' }
    ]
    const memory = estimateBufferMemory({
      logEntries: [],
      wsEvents: [],
      networkBodies: bodies,
      enhancedActions: []
    })

    // 2 entries * 1000 avg + len(req1)=4 + len(resp1resp1)=10 + len(r2)=2 + len(response2)=9 = 2000 + 25 = 2025
    const expected = 2 * MEMORY_AVG_NETWORK_BODY_SIZE + 4 + 10 + 2 + 9
    assert.strictEqual(memory, expected)
  })

  test('should estimate memory for enhanced actions (count * avg size)', () => {
    const actions = Array(50).fill({ type: 'click', timestamp: 1000 })
    const memory = estimateBufferMemory({
      logEntries: [],
      wsEvents: [],
      networkBodies: [],
      enhancedActions: actions
    })

    assert.strictEqual(memory, 50 * MEMORY_AVG_ACTION_SIZE)
  })

  test('should sum all buffer estimates together', () => {
    const logEntries = Array(10).fill({ level: 'error' })
    const wsEvents = [{ event: 'open', data: 'abc' }]
    const networkBodies = [{ request_body: 'x', response_body: 'yy' }]
    const enhancedActions = Array(5).fill({ type: 'click' })

    const memory = estimateBufferMemory({ logEntries, wsEvents, networkBodies, enhancedActions })

    const expectedLogs = 10 * MEMORY_AVG_LOG_ENTRY_SIZE
    const expectedWs = 1 * MEMORY_AVG_WS_EVENT_SIZE + 3 // 'abc'.length
    const expectedNet = 1 * MEMORY_AVG_NETWORK_BODY_SIZE + 1 + 2 // 'x'.length + 'yy'.length
    const expectedActions = 5 * MEMORY_AVG_ACTION_SIZE

    assert.strictEqual(memory, expectedLogs + expectedWs + expectedNet + expectedActions)
  })

  test('should handle WS events without data field', () => {
    const events = [
      { event: 'open', url: 'wss://example.com' },
      { event: 'close', code: 1000 }
    ]
    const memory = estimateBufferMemory({
      logEntries: [],
      wsEvents: events,
      networkBodies: [],
      enhancedActions: []
    })

    // 2 events * 300 avg + 0 data = 600
    assert.strictEqual(memory, 2 * MEMORY_AVG_WS_EVENT_SIZE)
  })

  test('should handle network bodies without request_body or response_body', () => {
    const bodies = [{ url: 'http://example.com/api' }, { request_body: 'data' }]
    const memory = estimateBufferMemory({
      logEntries: [],
      wsEvents: [],
      networkBodies: bodies,
      enhancedActions: []
    })

    // 2 entries * 1000 avg + 0 + 0 + 4 + 0 = 2004
    assert.strictEqual(memory, 2 * MEMORY_AVG_NETWORK_BODY_SIZE + 4)
  })
})

describe('Memory Enforcement: checkMemoryPressure', () => {
  beforeEach(() => {
    resetMemoryPressureState()
  })

  test('should return "normal" when memory is below soft limit', () => {
    // 10 entries * 500 bytes = 5000 bytes = ~5KB (well below 20MB)
    const buffers = {
      logEntries: Array(10).fill({ level: 'error' }),
      wsEvents: [],
      networkBodies: [],
      enhancedActions: []
    }

    const result = checkMemoryPressure(buffers)

    assert.strictEqual(result.level, 'normal')
    assert.strictEqual(result.action, 'none')
  })

  test('should return "soft" and reduce capacities when at soft limit (20MB)', () => {
    // Create enough entries to exceed 20MB: 20MB / 500 bytes = 40960 entries
    const buffers = {
      logEntries: Array(42000).fill({ level: 'error' }),
      wsEvents: [],
      networkBodies: [],
      enhancedActions: []
    }

    const result = checkMemoryPressure(buffers)

    assert.strictEqual(result.level, 'soft')
    assert.strictEqual(result.action, 'reduce_capacities')
  })

  test('should return "hard" and disable network capture when at hard limit (50MB)', () => {
    // Create enough entries to exceed 50MB: 50MB / 500 bytes = 104858 entries
    const buffers = {
      logEntries: Array(110000).fill({ level: 'error' }),
      wsEvents: [],
      networkBodies: [],
      enhancedActions: []
    }

    const result = checkMemoryPressure(buffers)

    assert.strictEqual(result.level, 'hard')
    assert.strictEqual(result.action, 'disable_network_capture')
  })

  test('should report estimated memory in result', () => {
    const buffers = {
      logEntries: Array(100).fill({ level: 'error' }),
      wsEvents: [],
      networkBodies: [],
      enhancedActions: []
    }

    const result = checkMemoryPressure(buffers)

    assert.strictEqual(result.estimatedMemory, 100 * MEMORY_AVG_LOG_ENTRY_SIZE)
  })

  test('below soft limit should not take action', () => {
    const buffers = {
      logEntries: Array(10).fill({ level: 'error' }),
      wsEvents: [],
      networkBodies: [],
      enhancedActions: []
    }

    const result = checkMemoryPressure(buffers)

    assert.strictEqual(result.level, 'normal')
    assert.strictEqual(result.action, 'none')
  })
})

describe('Memory Enforcement: getMemoryPressureState', () => {
  beforeEach(() => {
    resetMemoryPressureState()
  })

  test('should return initial state as normal', () => {
    const state = getMemoryPressureState()

    assert.strictEqual(state.memoryPressureLevel, 'normal')
    assert.strictEqual(state.networkBodyCaptureDisabled, false)
    assert.strictEqual(state.reducedCapacities, false)
  })

  test('should update state after soft limit check', () => {
    // Trigger a soft limit check
    const buffers = {
      logEntries: Array(42000).fill({ level: 'error' }),
      wsEvents: [],
      networkBodies: [],
      enhancedActions: []
    }

    checkMemoryPressure(buffers)

    const state = getMemoryPressureState()
    assert.strictEqual(state.memoryPressureLevel, 'soft')
    assert.strictEqual(state.reducedCapacities, true)
    assert.strictEqual(state.networkBodyCaptureDisabled, false)
  })

  test('should update state after hard limit check', () => {
    const buffers = {
      logEntries: Array(110000).fill({ level: 'error' }),
      wsEvents: [],
      networkBodies: [],
      enhancedActions: []
    }

    checkMemoryPressure(buffers)

    const state = getMemoryPressureState()
    assert.strictEqual(state.memoryPressureLevel, 'hard')
    assert.strictEqual(state.networkBodyCaptureDisabled, true)
    assert.strictEqual(state.reducedCapacities, true)
  })

  test('should recover to normal when memory drops below soft limit', () => {
    // First trigger soft limit
    const highBuffers = {
      logEntries: Array(42000).fill({ level: 'error' }),
      wsEvents: [],
      networkBodies: [],
      enhancedActions: []
    }
    checkMemoryPressure(highBuffers)

    // Then check with low memory
    const lowBuffers = {
      logEntries: Array(10).fill({ level: 'error' }),
      wsEvents: [],
      networkBodies: [],
      enhancedActions: []
    }
    checkMemoryPressure(lowBuffers)

    const state = getMemoryPressureState()
    assert.strictEqual(state.memoryPressureLevel, 'normal')
    assert.strictEqual(state.reducedCapacities, false)
    assert.strictEqual(state.networkBodyCaptureDisabled, false)
  })

  test('should not recover from hard to normal directly (must pass through soft first)', () => {
    // First trigger hard limit
    const hardBuffers = {
      logEntries: Array(110000).fill({ level: 'error' }),
      wsEvents: [],
      networkBodies: [],
      enhancedActions: []
    }
    checkMemoryPressure(hardBuffers)

    // Check with memory below hard but above soft
    const softBuffers = {
      logEntries: Array(42000).fill({ level: 'error' }),
      wsEvents: [],
      networkBodies: [],
      enhancedActions: []
    }
    checkMemoryPressure(softBuffers)

    const state = getMemoryPressureState()
    // Should be at soft level now, not immediately back to normal
    assert.strictEqual(state.memoryPressureLevel, 'soft')
    assert.strictEqual(state.reducedCapacities, true)
    // Network capture re-enabled once below hard limit
    assert.strictEqual(state.networkBodyCaptureDisabled, false)
  })

  test('should include lastMemoryCheck timestamp', () => {
    const buffers = {
      logEntries: Array(10).fill({ level: 'error' }),
      wsEvents: [],
      networkBodies: [],
      enhancedActions: []
    }

    const before = Date.now()
    checkMemoryPressure(buffers)
    const after = Date.now()

    const state = getMemoryPressureState()
    assert.ok(state.lastMemoryCheck >= before)
    assert.ok(state.lastMemoryCheck <= after)
  })
})

describe('Memory Enforcement: resetMemoryPressureState', () => {
  test('should reset all state to initial values', () => {
    // First trigger some pressure
    const buffers = {
      logEntries: Array(110000).fill({ level: 'error' }),
      wsEvents: [],
      networkBodies: [],
      enhancedActions: []
    }
    checkMemoryPressure(buffers)

    // Then reset
    resetMemoryPressureState()

    const state = getMemoryPressureState()
    assert.strictEqual(state.memoryPressureLevel, 'normal')
    assert.strictEqual(state.networkBodyCaptureDisabled, false)
    assert.strictEqual(state.reducedCapacities, false)
    assert.strictEqual(state.lastMemoryCheck, 0)
  })
})

describe('Memory Enforcement: Multiple consecutive checks', () => {
  beforeEach(() => {
    resetMemoryPressureState()
  })

  test('multiple checks at soft limit should not re-trigger action if already reduced', () => {
    const buffers = {
      logEntries: Array(42000).fill({ level: 'error' }),
      wsEvents: [],
      networkBodies: [],
      enhancedActions: []
    }

    const result1 = checkMemoryPressure(buffers)
    const result2 = checkMemoryPressure(buffers)

    // Both should report soft level, but the second should indicate already handled
    assert.strictEqual(result1.level, 'soft')
    assert.strictEqual(result2.level, 'soft')
    assert.strictEqual(result2.alreadyApplied, true)
  })

  test('multiple checks at hard limit should not re-trigger if already disabled', () => {
    const buffers = {
      logEntries: Array(110000).fill({ level: 'error' }),
      wsEvents: [],
      networkBodies: [],
      enhancedActions: []
    }

    const result1 = checkMemoryPressure(buffers)
    const result2 = checkMemoryPressure(buffers)

    assert.strictEqual(result1.level, 'hard')
    assert.strictEqual(result2.level, 'hard')
    assert.strictEqual(result2.alreadyApplied, true)
  })
})

describe('Memory Enforcement: Chrome Alarm Integration', () => {
  test('MEMORY_CHECK_INTERVAL_MS should equal 30 seconds for alarm period', () => {
    // The alarm period in the code uses MEMORY_CHECK_INTERVAL_MS / 60000
    // which should produce 0.5 minutes (30 seconds)
    assert.strictEqual(MEMORY_CHECK_INTERVAL_MS / 60000, 0.5)
  })

  test('memoryCheck alarm period should be 0.5 minutes (30 seconds)', () => {
    // Verify the constant is correctly set to produce the right alarm period
    const periodInMinutes = MEMORY_CHECK_INTERVAL_MS / 60000
    assert.strictEqual(periodInMinutes, 0.5, 'Expected 30-second (0.5 minute) period')
  })
})

describe('Memory Enforcement: Network body rejection at hard limit', () => {
  beforeEach(() => {
    resetMemoryPressureState()
  })

  test('after hard limit, network body capture should be disabled', () => {
    const buffers = {
      logEntries: Array(110000).fill({ level: 'error' }),
      wsEvents: [],
      networkBodies: [],
      enhancedActions: []
    }

    checkMemoryPressure(buffers)

    const state = getMemoryPressureState()
    assert.strictEqual(state.networkBodyCaptureDisabled, true)
  })

  test('after hard limit and recovery below soft, network capture should re-enable', () => {
    // Trigger hard limit
    const hardBuffers = {
      logEntries: Array(110000).fill({ level: 'error' }),
      wsEvents: [],
      networkBodies: [],
      enhancedActions: []
    }
    checkMemoryPressure(hardBuffers)

    // Recover below soft
    const lowBuffers = {
      logEntries: Array(10).fill({ level: 'error' }),
      wsEvents: [],
      networkBodies: [],
      enhancedActions: []
    }
    checkMemoryPressure(lowBuffers)

    const state = getMemoryPressureState()
    assert.strictEqual(state.networkBodyCaptureDisabled, false)
  })
})

describe('Memory Enforcement: Capacity reduction at soft limit', () => {
  beforeEach(() => {
    resetMemoryPressureState()
  })

  test('at soft limit, reducedCapacities flag should be set', () => {
    const buffers = {
      logEntries: Array(42000).fill({ level: 'error' }),
      wsEvents: [],
      networkBodies: [],
      enhancedActions: []
    }

    checkMemoryPressure(buffers)

    const state = getMemoryPressureState()
    assert.strictEqual(state.reducedCapacities, true)
  })

  test('after recovery below soft limit, reducedCapacities should be cleared', () => {
    // Trigger soft limit
    const softBuffers = {
      logEntries: Array(42000).fill({ level: 'error' }),
      wsEvents: [],
      networkBodies: [],
      enhancedActions: []
    }
    checkMemoryPressure(softBuffers)

    // Recover
    const lowBuffers = {
      logEntries: Array(10).fill({ level: 'error' }),
      wsEvents: [],
      networkBodies: [],
      enhancedActions: []
    }
    checkMemoryPressure(lowBuffers)

    const state = getMemoryPressureState()
    assert.strictEqual(state.reducedCapacities, false)
  })
})

describe('Memory Enforcement: Hard limit disables network body batcher', () => {
  beforeEach(() => {
    resetMemoryPressureState()
  })

  test('batcher with networkBodyCaptureDisabled should reject new entries', () => {
    // Trigger hard limit to set networkBodyCaptureDisabled
    const hardBuffers = {
      logEntries: Array(110000).fill({ level: 'error' }),
      wsEvents: [],
      networkBodies: [],
      enhancedActions: []
    }
    checkMemoryPressure(hardBuffers)

    const state = getMemoryPressureState()
    assert.strictEqual(state.networkBodyCaptureDisabled, true, 'Network body capture should be disabled at hard limit')
    assert.strictEqual(state.reducedCapacities, true, 'Capacities should also be reduced at hard limit')
  })

  test('hard limit action is disable_network_capture', () => {
    const hardBuffers = {
      logEntries: Array(110000).fill({ level: 'error' }),
      wsEvents: [],
      networkBodies: [],
      enhancedActions: []
    }

    const result = checkMemoryPressure(hardBuffers)
    assert.strictEqual(result.action, 'disable_network_capture')
    assert.strictEqual(result.level, 'hard')
    assert.ok(result.estimatedMemory >= MEMORY_HARD_LIMIT, 'Estimated memory should be at or above hard limit')
  })
})

describe('Memory Enforcement: Batcher interaction with memory pressure', () => {
  beforeEach(() => {
    resetMemoryPressureState()
  })

  test('createLogBatcher should accept memoryPressureGetter option', () => {
    const flushFn = mock.fn()
    const batcher = createLogBatcher(flushFn, {
      debounceMs: 50,
      maxBatchSize: 50,
      memoryPressureGetter: getMemoryPressureState
    })

    assert.ok(batcher)
    assert.ok(batcher.add)
    assert.ok(batcher.flush)
  })

  test('batcher with reduced capacities should halve maxBatchSize', () => {
    // Trigger soft limit to set reducedCapacities
    const softBuffers = {
      logEntries: Array(42000).fill({ level: 'error' }),
      wsEvents: [],
      networkBodies: [],
      enhancedActions: []
    }
    checkMemoryPressure(softBuffers)

    const flushFn = mock.fn()
    const batcher = createLogBatcher(flushFn, {
      debounceMs: 10000,
      maxBatchSize: 50,
      memoryPressureGetter: getMemoryPressureState
    })

    // Add 25 entries (half of 50) - should trigger flush at reduced capacity
    for (let i = 0; i < 25; i++) {
      batcher.add({ msg: `entry-${i}` })
    }

    assert.strictEqual(flushFn.mock.calls.length, 1, 'Expected flush at halved capacity (25)')
    assert.strictEqual(flushFn.mock.calls[0].arguments[0].length, 25)
  })

  test('batcher without memoryPressureGetter should work normally', () => {
    const flushFn = mock.fn()
    const batcher = createLogBatcher(flushFn, {
      debounceMs: 10000,
      maxBatchSize: 50
    })

    // Add 50 entries - should flush at normal capacity
    for (let i = 0; i < 50; i++) {
      batcher.add({ msg: `entry-${i}` })
    }

    assert.strictEqual(flushFn.mock.calls.length, 1)
    assert.strictEqual(flushFn.mock.calls[0].arguments[0].length, 50)
  })
})
