// @ts-nocheck
/**
 * @fileoverview rate-limit.test.js — Tests for rate limiting and circuit breaker.
 * Verifies request throttling, exponential backoff on repeated failures,
 * circuit breaker state transitions (closed → open → half-open), and
 * integration with the batching system in background.js.
 * Tests validate spec scenarios 13-23 from docs/ai-first/tech-spec-rate-limiting.md
 */

import { test, describe, mock, beforeEach } from 'node:test'
import assert from 'node:assert'
import { MANIFEST_VERSION } from '../shared/helpers.js'

// Mock Chrome APIs
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
      set: mock.fn(),
      remove: mock.fn((keys, callback) => {
        if (typeof callback === 'function') callback()
        else return Promise.resolve()
      })
    },
    sync: {
      get: mock.fn((k, cb) => cb({})),
      set: mock.fn(),
      remove: mock.fn((keys, callback) => {
        if (typeof callback === 'function') callback()
        else return Promise.resolve()
      })
    },
    session: {
      get: mock.fn((k, cb) => cb({})),
      set: mock.fn(),
      remove: mock.fn((keys, callback) => {
        if (typeof callback === 'function') callback()
        else return Promise.resolve()
      })
    },
    onChanged: { addListener: mock.fn() }
  },
  alarms: { create: mock.fn(), onAlarm: { addListener: mock.fn() } },
  tabs: {
    get: mock.fn(),
    query: mock.fn(),
    onRemoved: { addListener: mock.fn() }
  }
}

// Mock fetch globally
globalThis.fetch = mock.fn()

import { createCircuitBreaker } from '../../../extension/background/sync/circuit-breaker.js'
import { createBatcherWithCircuitBreaker, RATE_LIMIT_CONFIG } from '../../../extension/background/sync/batchers.js'

describe('Rate Limit: Batcher Circuit Breaker Wiring', () => {
  beforeEach(() => {
    mock.reset()
  })

  test('serializes flushes and sends arrivals during an in-flight request once afterward', async () => {
    let releaseFirst
    const first = new Promise((resolve) => {
      releaseFirst = resolve
    })
    const sent = []
    const sendFn = mock.fn(async (entries) => {
      sent.push(entries)
      if (sent.length === 1) await first
    })
    const { batcher } = createBatcherWithCircuitBreaker(sendFn, {
      debounceMs: 60000,
      maxBatchSize: 1,
      retryBudget: 1
    })

    batcher.add({ id: 1 })
    batcher.add({ id: 2 })
    assert.strictEqual(sendFn.mock.calls.length, 1)
    releaseFirst()
    await batcher.flush()

    assert.deepStrictEqual(sent, [[{ id: 1 }], [{ id: 2 }]])
  })

  // Spec scenario 13: Single 429 -> backoff 100ms before next attempt
  test('13: Single 429 triggers 100ms backoff before next attempt', async () => {
    const sendFn = mock.fn(() => {
      return Promise.reject(new Error('Server error: 429 Too Many Requests'))
    })

    const { batcher, circuitBreaker } = createBatcherWithCircuitBreaker(sendFn, {
      debounceMs: 1,
      maxBatchSize: 50,
      retryBudget: 1 // No retries - test backoff progression across batches
    })

    // First batch - will fail with 429
    batcher.add({ type: 'log', message: 'test1' })
    await batcher.flush()

    // The circuit breaker should now have backoff of 100ms
    const stats = circuitBreaker.getStats()
    assert.strictEqual(stats.consecutiveFailures, 1)
    assert.strictEqual(stats.currentBackoff, RATE_LIMIT_CONFIG.backoffSchedule[0])
  })

  // Spec scenario 14: Second consecutive 429 -> backoff 500ms
  test('14: Second consecutive 429 triggers 500ms backoff', async () => {
    const sendFn = mock.fn(() => {
      return Promise.reject(new Error('Server error: 429 Too Many Requests'))
    })

    const { batcher, circuitBreaker } = createBatcherWithCircuitBreaker(sendFn, {
      debounceMs: 1,
      maxBatchSize: 50,
      retryBudget: 1 // No retries - test backoff progression across batches
    })

    // First failure
    batcher.add({ type: 'log', message: 'test1' })
    await batcher.flush()

    // Second failure
    batcher.add({ type: 'log', message: 'test2' })
    await batcher.flush()

    const stats = circuitBreaker.getStats()
    assert.strictEqual(stats.consecutiveFailures, 2)
    assert.strictEqual(stats.currentBackoff, RATE_LIMIT_CONFIG.backoffSchedule[1])
  })

  // Spec scenario 15: Third consecutive 429 -> backoff 2000ms
  test('15: Third consecutive 429 triggers 2000ms backoff', async () => {
    const sendFn = mock.fn(() => {
      return Promise.reject(new Error('Server error: 429 Too Many Requests'))
    })

    const { batcher, circuitBreaker } = createBatcherWithCircuitBreaker(sendFn, {
      debounceMs: 1,
      maxBatchSize: 50,
      retryBudget: 1 // No retries - test backoff progression across batches
    })

    // Three consecutive failures
    for (let i = 0; i < 3; i++) {
      batcher.add({ type: 'log', message: `test${i}` })
      await batcher.flush()
    }

    const stats = circuitBreaker.getStats()
    assert.strictEqual(stats.consecutiveFailures, 3)
    assert.strictEqual(stats.currentBackoff, RATE_LIMIT_CONFIG.backoffSchedule[2])
  })

  // Spec scenario 16: Fifth consecutive failure -> circuit opens, 30-second pause
  test('16: Fifth consecutive failure opens circuit for 30-second pause', async () => {
    const sendFn = mock.fn(() => {
      return Promise.reject(new Error('Server error: 429 Too Many Requests'))
    })

    const { batcher, circuitBreaker } = createBatcherWithCircuitBreaker(sendFn, {
      debounceMs: 1,
      maxBatchSize: 50,
      retryBudget: 1 // No retries - test circuit opening on 5th failure
    })

    // Five consecutive failures
    for (let i = 0; i < 5; i++) {
      batcher.add({ type: 'log', message: `test${i}` })
      await batcher.flush()
    }

    assert.strictEqual(circuitBreaker.getState(), 'open')
  })

  // Spec scenario 17: Successful POST after failure -> backoff resets to 0
  test('17: Successful POST after failure resets backoff to 0', async () => {
    let callCount = 0
    const sendFn = mock.fn(() => {
      callCount++
      if (callCount <= 2) {
        return Promise.reject(new Error('Server error: 429 Too Many Requests'))
      }
      return Promise.resolve()
    })

    const { batcher, circuitBreaker } = createBatcherWithCircuitBreaker(sendFn, {
      debounceMs: 1,
      maxBatchSize: 50,
      retryBudget: 1 // No retries - test backoff reset on success
    })

    // Two failures
    batcher.add({ type: 'log', message: 'test1' })
    await batcher.flush()
    batcher.add({ type: 'log', message: 'test2' })
    await batcher.flush()
    assert.ok(circuitBreaker.getStats().consecutiveFailures > 0)

    // Success resets
    batcher.add({ type: 'log', message: 'test3' })
    await batcher.flush()
    assert.strictEqual(circuitBreaker.getStats().consecutiveFailures, 0)
    assert.strictEqual(circuitBreaker.getStats().currentBackoff, 0)
  })

  // Spec scenario 18: Circuit open -> no POSTs for 30 seconds
  test('18: Circuit open prevents all POSTs', async () => {
    const sendFn = mock.fn(() => {
      return Promise.reject(new Error('Server error: 429'))
    })

    const { batcher, circuitBreaker } = createBatcherWithCircuitBreaker(sendFn, {
      debounceMs: 1,
      maxBatchSize: 50,
      retryBudget: 1
    })

    // Open the circuit (5 failures)
    for (let i = 0; i < 5; i++) {
      batcher.add({ type: 'log', message: `test${i}` })
      await batcher.flush()
    }
    assert.strictEqual(circuitBreaker.getState(), 'open')

    const callCountAtOpen = sendFn.mock.calls.length

    // Try to flush again - should not call sendFn
    batcher.add({ type: 'log', message: 'blocked' })
    await batcher.flush()

    assert.strictEqual(sendFn.mock.calls.length, callCountAtOpen)
  })

  // Spec scenario 19: After 30 seconds -> single probe sent
  test('19: After resetTimeout, a single probe request is sent', async () => {
    let shouldFail = true
    const sendFn = mock.fn(() => {
      if (shouldFail) return Promise.reject(new Error('Server error: 429'))
      return Promise.resolve()
    })

    const { batcher, circuitBreaker } = createBatcherWithCircuitBreaker(sendFn, {
      debounceMs: 1,
      maxBatchSize: 50,
      retryBudget: 1,
      resetTimeout: 50 // Short timeout for testing
    })

    // Open the circuit
    for (let i = 0; i < 5; i++) {
      batcher.add({ type: 'log', message: `test${i}` })
      await batcher.flush()
    }
    assert.strictEqual(circuitBreaker.getState(), 'open')

    // Wait for reset timeout
    await new Promise((r) => setTimeout(r, 150))
    assert.strictEqual(circuitBreaker.getState(), 'half-open')

    // Next flush should send a probe
    shouldFail = false
    const callCountBefore = sendFn.mock.calls.length
    batcher.add({ type: 'log', message: 'probe' })
    await batcher.flush()

    assert.strictEqual(sendFn.mock.calls.length, callCountBefore + 1)
  })

  // Spec scenario 20: Probe succeeds -> circuit closes, buffer drains
  test('20: Probe succeeds closes circuit and drains buffer', async () => {
    let shouldFail = true
    const sendFn = mock.fn(() => {
      if (shouldFail) return Promise.reject(new Error('Server error: 429'))
      return Promise.resolve()
    })

    const { batcher, circuitBreaker } = createBatcherWithCircuitBreaker(sendFn, {
      debounceMs: 1,
      maxBatchSize: 50,
      retryBudget: 1,
      resetTimeout: 50
    })

    // Open the circuit
    for (let i = 0; i < 5; i++) {
      batcher.add({ type: 'log', message: `fail${i}` })
      await batcher.flush()
    }
    assert.strictEqual(circuitBreaker.getState(), 'open')

    // Wait for half-open
    await new Promise((r) => setTimeout(r, 150))

    // Probe succeeds
    shouldFail = false
    batcher.add({ type: 'log', message: 'probe-success' })
    await batcher.flush()

    assert.strictEqual(circuitBreaker.getState(), 'closed')
    assert.strictEqual(circuitBreaker.getStats().consecutiveFailures, 0)
  })

  // Spec scenario 21: Probe fails -> circuit re-opens for another 30 seconds
  test('21: Probe fails re-opens circuit', async () => {
    const sendFn = mock.fn(() => Promise.reject(new Error('Server error: 429')))

    const { batcher, circuitBreaker } = createBatcherWithCircuitBreaker(sendFn, {
      debounceMs: 1,
      maxBatchSize: 50,
      retryBudget: 1,
      resetTimeout: 50
    })

    // Open the circuit
    for (let i = 0; i < 5; i++) {
      batcher.add({ type: 'log', message: `fail${i}` })
      await batcher.flush()
    }
    assert.strictEqual(circuitBreaker.getState(), 'open')

    // Wait for half-open
    await new Promise((r) => setTimeout(r, 150))
    assert.strictEqual(circuitBreaker.getState(), 'half-open')

    // Probe fails - circuit re-opens
    batcher.add({ type: 'log', message: 'probe-fail' })
    await batcher.flush()
    assert.strictEqual(circuitBreaker.getState(), 'open')
  })

  // Spec scenario 22: During backoff, data still captured to local buffer
  test('22: During backoff, data continues to buffer locally', async () => {
    const sendFn = mock.fn(() => Promise.reject(new Error('Server error: 429')))

    const { batcher, circuitBreaker } = createBatcherWithCircuitBreaker(sendFn, {
      debounceMs: 1,
      maxBatchSize: 50,
      retryBudget: 1
    })

    // Open circuit
    for (let i = 0; i < 5; i++) {
      batcher.add({ type: 'log', message: `fail${i}` })
      await batcher.flush()
    }
    assert.strictEqual(circuitBreaker.getState(), 'open')

    // Adding data should not throw - it buffers locally
    assert.doesNotThrow(() => {
      batcher.add({ type: 'log', message: 'buffered1' })
      batcher.add({ type: 'log', message: 'buffered2' })
      batcher.add({ type: 'log', message: 'buffered3' })
    })

    // Verify data is in the batcher's pending buffer
    const pending = batcher.getPending()
    assert.ok(pending.length >= 3, `Expected at least 3 pending items, got ${pending.length}`)
  })

  // Spec scenario 23: Retry budget of 3 per batch -> after 3 failures, batch is retained
  test('23: Retry budget of 3 - batch retained after 3 attempts', async () => {
    const sendFn = mock.fn(() => Promise.reject(new Error('Server error: 429')))

    const { batcher } = createBatcherWithCircuitBreaker(sendFn, {
      debounceMs: 1,
      maxBatchSize: 50,
      retryBudget: 3,
      maxFailures: 10 // High so circuit doesn't open during this test
    })

    // Add a batch and flush - should retry up to 3 times, then retain it for recovery.
    batcher.add({ type: 'log', message: 'retry-test' })
    await batcher.flush()

    // The sendFn should have been called exactly 3 times (initial + 2 retries = 3 total)
    assert.strictEqual(sendFn.mock.calls.length, 3)

    // Retry exhaustion must not lose telemetry.
    const pending = batcher.getPending()
    assert.deepStrictEqual(pending, [{ type: 'log', message: 'retry-test' }])
  })
})

describe('Rate Limit: Configuration', () => {
  test('RATE_LIMIT_CONFIG has correct spec values', () => {
    assert.strictEqual(RATE_LIMIT_CONFIG.maxFailures, 5)
    assert.strictEqual(RATE_LIMIT_CONFIG.resetTimeout, 30000)
    assert.deepStrictEqual(RATE_LIMIT_CONFIG.backoffSchedule, [100, 500, 2000])
    assert.strictEqual(RATE_LIMIT_CONFIG.retryBudget, 3)
  })
})

describe('Rate Limit: Shared Circuit Breaker', () => {
  test('Multiple batchers share the same circuit breaker instance', async () => {
    const sendFn1 = mock.fn(() => Promise.reject(new Error('Server error: 429')))
    const sendFn2 = mock.fn(() => Promise.reject(new Error('Server error: 429')))

    // Create a shared circuit breaker
    const sharedCB = createCircuitBreaker(() => Promise.reject(new Error('fail')), {
      maxFailures: 5,
      resetTimeout: 30000
    })

    const batcher1 = createBatcherWithCircuitBreaker(sendFn1, {
      debounceMs: 1,
      maxBatchSize: 50,
      retryBudget: 1,
      sharedCircuitBreaker: sharedCB
    })

    const batcher2 = createBatcherWithCircuitBreaker(sendFn2, {
      debounceMs: 1,
      maxBatchSize: 50,
      retryBudget: 1,
      sharedCircuitBreaker: sharedCB
    })

    // Failures from batcher1 affect batcher2's circuit state
    for (let i = 0; i < 3; i++) {
      batcher1.batcher.add({ type: 'log', message: `fail${i}` })
      await batcher1.batcher.flush()
    }

    // Shared circuit breaker should show 3 failures
    assert.strictEqual(sharedCB.getStats().consecutiveFailures, 3)

    // Two more failures from batcher2 should open the circuit
    for (let i = 0; i < 2; i++) {
      batcher2.batcher.add({ type: 'ws', message: `fail${i}` })
      await batcher2.batcher.flush()
    }

    assert.strictEqual(sharedCB.getState(), 'open')

    // Both batchers are now blocked
    const callCount1 = sendFn1.mock.calls.length
    const callCount2 = sendFn2.mock.calls.length

    batcher1.batcher.add({ type: 'log', message: 'blocked' })
    await batcher1.batcher.flush()
    batcher2.batcher.add({ type: 'ws', message: 'blocked' })
    await batcher2.batcher.flush()

    assert.strictEqual(sendFn1.mock.calls.length, callCount1)
    assert.strictEqual(sendFn2.mock.calls.length, callCount2)
  })
})

describe('Rate Limit: Connection Status Updates', () => {
  test('Successful send updates connectionStatus.connected to true', async () => {
    const sendFn = mock.fn(() => Promise.resolve({ entries: 5 }))

    const { batcher, getConnectionStatus } = createBatcherWithCircuitBreaker(sendFn, {
      debounceMs: 1,
      maxBatchSize: 50
    })

    batcher.add({ type: 'log', message: 'test' })
    await batcher.flush()

    assert.strictEqual(getConnectionStatus().connected, true)
  })

  test('Failed send updates connectionStatus.connected to false', async () => {
    const sendFn = mock.fn(() => Promise.reject(new Error('Server error: 429')))

    const { batcher, getConnectionStatus } = createBatcherWithCircuitBreaker(sendFn, {
      debounceMs: 1,
      maxBatchSize: 50
    })

    batcher.add({ type: 'log', message: 'test' })
    await batcher.flush()

    assert.strictEqual(getConnectionStatus().connected, false)
  })
})

describe('Rate Limit: Network Errors (non-429)', () => {
  test('TypeError (network failure) triggers same backoff as 429', async () => {
    const sendFn = mock.fn(() => {
      return Promise.reject(new TypeError('Failed to fetch'))
    })

    const { batcher, circuitBreaker } = createBatcherWithCircuitBreaker(sendFn, {
      debounceMs: 1,
      maxBatchSize: 50,
      retryBudget: 1
    })

    batcher.add({ type: 'log', message: 'test1' })
    await batcher.flush()

    const stats = circuitBreaker.getStats()
    assert.strictEqual(stats.consecutiveFailures, 1)
    assert.strictEqual(stats.currentBackoff, RATE_LIMIT_CONFIG.backoffSchedule[0])
  })

  test('Connection refused error triggers backoff progression', async () => {
    const sendFn = mock.fn(() => {
      return Promise.reject(new Error('net::ERR_CONNECTION_REFUSED'))
    })

    const { batcher, circuitBreaker } = createBatcherWithCircuitBreaker(sendFn, {
      debounceMs: 1,
      maxBatchSize: 50,
      retryBudget: 1,
      maxFailures: 10
    })

    for (let i = 0; i < 3; i++) {
      batcher.add({ type: 'log', message: `fail${i}` })
      await batcher.flush()
    }

    const stats = circuitBreaker.getStats()
    assert.strictEqual(stats.consecutiveFailures, 3)
    assert.strictEqual(stats.currentBackoff, RATE_LIMIT_CONFIG.backoffSchedule[2])
  })

  test('5 consecutive network errors open circuit (same as 429s)', async () => {
    const sendFn = mock.fn(() => {
      return Promise.reject(new TypeError('Failed to fetch'))
    })

    const { batcher, circuitBreaker } = createBatcherWithCircuitBreaker(sendFn, {
      debounceMs: 1,
      maxBatchSize: 50,
      retryBudget: 1
    })

    for (let i = 0; i < 5; i++) {
      batcher.add({ type: 'log', message: `fail${i}` })
      await batcher.flush()
    }

    assert.strictEqual(circuitBreaker.getState(), 'open')
  })
})

describe('Rate Limit: Backoff Schedule Mapping', () => {
  test('Backoff values follow schedule: 100, 500, 2000 then cap at 2000', async () => {
    const sendFn = mock.fn(() => Promise.reject(new Error('Server error: 429')))

    const { batcher, circuitBreaker } = createBatcherWithCircuitBreaker(sendFn, {
      debounceMs: 1,
      maxBatchSize: 50,
      retryBudget: 1, // No retries - test backoff progression
      maxFailures: 10 // Prevent circuit from opening
    })

    // After 1st failure: backoff should be 100ms
    batcher.add({ msg: '1' })
    await batcher.flush()
    assert.strictEqual(circuitBreaker.getStats().currentBackoff, 100)

    // After 2nd failure: backoff should be 500ms
    batcher.add({ msg: '2' })
    await batcher.flush()
    assert.strictEqual(circuitBreaker.getStats().currentBackoff, 500)

    // After 3rd failure: backoff should be 2000ms
    batcher.add({ msg: '3' })
    await batcher.flush()
    assert.strictEqual(circuitBreaker.getStats().currentBackoff, 2000)

    // After 4th failure: backoff should stay capped at 2000ms
    batcher.add({ msg: '4' })
    await batcher.flush()
    assert.strictEqual(circuitBreaker.getStats().currentBackoff, 2000)
  })
})
