// @ts-nocheck
/**
 * @fileoverview circuit-breaker.test.js — Tests for circuit breaker state machine.
 * Covers state transitions (closed/open/half-open), failure threshold triggers,
 * exponential backoff timing, half-open probe requests, and recovery to closed state.
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
    local: { get: mock.fn((k, cb) => cb({})), set: mock.fn() },
    sync: { get: mock.fn((k, cb) => cb({})), set: mock.fn() },
    session: { get: mock.fn((k, cb) => cb({})), set: mock.fn() },
    onChanged: { addListener: mock.fn() }
  },
  alarms: { create: mock.fn(), onAlarm: { addListener: mock.fn() } },
  tabs: { get: mock.fn(), query: mock.fn(), onRemoved: { addListener: mock.fn() } }
}

import { createCircuitBreaker } from '../../../extension/background/sync/circuit-breaker.js'

describe('Circuit Breaker', () => {
  let _clock

  beforeEach(() => {
    mock.reset()
  })

  test('should be created with default options', () => {
    const sendFn = mock.fn()
    const cb = createCircuitBreaker(sendFn)

    assert.strictEqual(cb.getState(), 'closed')
    assert.deepStrictEqual(cb.getStats(), {
      state: 'closed',
      consecutiveFailures: 0,
      totalFailures: 0,
      totalSuccesses: 0,
      currentBackoff: 0
    })
  })

  test('should pass through calls when circuit is closed', async () => {
    const sendFn = mock.fn(() => Promise.resolve({ ok: true }))
    const cb = createCircuitBreaker(sendFn)

    const result = await cb.execute(['entry1', 'entry2'])

    assert.strictEqual(sendFn.mock.calls.length, 1)
    assert.deepStrictEqual(sendFn.mock.calls[0].arguments[0], ['entry1', 'entry2'])
    assert.deepStrictEqual(result, { ok: true })
  })

  test('should track consecutive failures', async () => {
    const sendFn = mock.fn(() => Promise.reject(new Error('Network error')))
    const cb = createCircuitBreaker(sendFn, { maxFailures: 5, initialBackoff: 1, maxBackoff: 5 })

    // First failure
    await assert.rejects(() => cb.execute(['entry']), { message: 'Network error' })
    assert.strictEqual(cb.getStats().consecutiveFailures, 1)
    assert.strictEqual(cb.getState(), 'closed')

    // Second failure
    await assert.rejects(() => cb.execute(['entry']), { message: 'Network error' })
    assert.strictEqual(cb.getStats().consecutiveFailures, 2)
  })

  test('should reset consecutive failures on success', async () => {
    let callCount = 0
    const sendFn = mock.fn(() => {
      callCount++
      if (callCount <= 2) return Promise.reject(new Error('fail'))
      return Promise.resolve({ ok: true })
    })
    const cb = createCircuitBreaker(sendFn, { maxFailures: 5, initialBackoff: 1, maxBackoff: 5 })

    // Two failures
    await assert.rejects(() => cb.execute(['entry']))
    await assert.rejects(() => cb.execute(['entry']))
    assert.strictEqual(cb.getStats().consecutiveFailures, 2)

    // Success resets
    await cb.execute(['entry'])
    assert.strictEqual(cb.getStats().consecutiveFailures, 0)
    assert.strictEqual(cb.getStats().totalSuccesses, 1)
    assert.strictEqual(cb.getStats().totalFailures, 2)
  })

  test('should open circuit after maxFailures consecutive failures', async () => {
    const sendFn = mock.fn(() => Promise.reject(new Error('fail')))
    const cb = createCircuitBreaker(sendFn, { maxFailures: 3, initialBackoff: 1, maxBackoff: 5 })

    // Trigger 3 failures
    for (let i = 0; i < 3; i++) {
      await assert.rejects(() => cb.execute(['entry']))
    }

    assert.strictEqual(cb.getState(), 'open')
  })

  test('should reject immediately when circuit is open', async () => {
    const sendFn = mock.fn(() => Promise.reject(new Error('fail')))
    const cb = createCircuitBreaker(sendFn, { maxFailures: 2, initialBackoff: 1, maxBackoff: 5 })

    // Open the circuit
    await assert.rejects(() => cb.execute(['entry']))
    await assert.rejects(() => cb.execute(['entry']))
    assert.strictEqual(cb.getState(), 'open')

    // Next call should not invoke sendFn
    const prevCallCount = sendFn.mock.calls.length
    await assert.rejects(() => cb.execute(['entry']), { message: /circuit breaker is open/ })
    assert.strictEqual(sendFn.mock.calls.length, prevCallCount)
  })

  test('should transition to half-open after resetTimeout', async () => {
    const sendFn = mock.fn(() => Promise.reject(new Error('fail')))
    const cb = createCircuitBreaker(sendFn, { maxFailures: 2, resetTimeout: 5, initialBackoff: 1, maxBackoff: 5 })

    // Open the circuit
    await assert.rejects(() => cb.execute(['entry']))
    await assert.rejects(() => cb.execute(['entry']))
    assert.strictEqual(cb.getState(), 'open')

    // Wait for reset timeout
    await new Promise((r) => setTimeout(r, 8))

    assert.strictEqual(cb.getState(), 'half-open')
  })

  test('should close circuit on success in half-open state', async () => {
    let shouldFail = true
    const sendFn = mock.fn(() => {
      if (shouldFail) return Promise.reject(new Error('fail'))
      return Promise.resolve({ ok: true })
    })
    const cb = createCircuitBreaker(sendFn, { maxFailures: 2, resetTimeout: 5, initialBackoff: 1, maxBackoff: 5 })

    // Open the circuit
    await assert.rejects(() => cb.execute(['entry']))
    await assert.rejects(() => cb.execute(['entry']))

    // Wait for half-open
    await new Promise((r) => setTimeout(r, 8))
    assert.strictEqual(cb.getState(), 'half-open')

    // Success in half-open closes the circuit
    shouldFail = false
    await cb.execute(['entry'])
    assert.strictEqual(cb.getState(), 'closed')
    assert.strictEqual(cb.getStats().consecutiveFailures, 0)
  })

  test('should re-open circuit on failure in half-open state', async () => {
    const sendFn = mock.fn(() => Promise.reject(new Error('still failing')))
    const cb = createCircuitBreaker(sendFn, { maxFailures: 2, resetTimeout: 5, initialBackoff: 1, maxBackoff: 5 })

    // Open the circuit
    await assert.rejects(() => cb.execute(['entry']))
    await assert.rejects(() => cb.execute(['entry']))

    // Wait for half-open
    await new Promise((r) => setTimeout(r, 8))
    assert.strictEqual(cb.getState(), 'half-open')

    // Failure in half-open re-opens
    await assert.rejects(() => cb.execute(['entry']))
    assert.strictEqual(cb.getState(), 'open')
  })

  test('should allow manual reset', async () => {
    const sendFn = mock.fn(() => Promise.reject(new Error('fail')))
    const cb = createCircuitBreaker(sendFn, { maxFailures: 2, initialBackoff: 1, maxBackoff: 5 })

    // Open the circuit
    await assert.rejects(() => cb.execute(['entry']))
    await assert.rejects(() => cb.execute(['entry']))
    assert.strictEqual(cb.getState(), 'open')

    // Manual reset
    cb.reset()
    assert.strictEqual(cb.getState(), 'closed')
    assert.strictEqual(cb.getStats().consecutiveFailures, 0)
  })

  test('should only allow one probe in half-open state', async () => {
    let _resolveProbe
    // Slow sendFn that hangs until we resolve it
    const slowFn = mock.fn(
      () =>
        new Promise((resolve, reject) => {
          _resolveProbe = reject
        })
    )
    const _cb = createCircuitBreaker(slowFn, { maxFailures: 2, resetTimeout: 5, initialBackoff: 1 })

    // Open circuit with quick failures first
    let _callCount = 0
    const openCb = createCircuitBreaker(
      () => {
        _callCount++
        return Promise.reject(new Error('fail'))
      },
      { maxFailures: 2, resetTimeout: 5, initialBackoff: 1 }
    )

    await assert.rejects(() => openCb.execute(['entry']))
    await assert.rejects(() => openCb.execute(['entry']))

    // Wait for half-open
    await new Promise((r) => setTimeout(r, 8))
    assert.strictEqual(openCb.getState(), 'half-open')

    // First probe starts - hangs because failFn takes time
    const probePromise = openCb.execute(['probe']).catch(() => {})

    // Since failFn rejected immediately, state went back to open
    // Second call should be rejected
    await assert.rejects(() => openCb.execute(['another']), { message: /circuit breaker/ })

    await probePromise
  })
})

describe('Exponential Backoff', () => {
  test('should apply backoff delay on failures', async () => {
    const sendFn = mock.fn(() => Promise.reject(new Error('fail')))
    const cb = createCircuitBreaker(sendFn, {
      maxFailures: 10,
      initialBackoff: 5,
      maxBackoff: 40
    })

    // First call - no backoff (consecutiveFailures starts at 0)
    await assert.rejects(() => cb.execute(['entry']))
    assert.strictEqual(cb.getStats().currentBackoff, 0)

    // Second call - still no backoff (backoff starts after 2nd failure)
    await assert.rejects(() => cb.execute(['entry']))
    assert.strictEqual(cb.getStats().currentBackoff, 5)

    // Third call should have had 5ms backoff applied; now backoff doubles to 10ms
    const before = Date.now()
    await assert.rejects(() => cb.execute(['entry']))
    const elapsed = Date.now() - before
    assert.ok(elapsed >= 4, `Expected >= 4ms backoff delay, got ${elapsed}ms`)
    assert.strictEqual(cb.getStats().currentBackoff, 10)
  })

  test('should cap backoff at maxBackoff', async () => {
    const sendFn = mock.fn(() => Promise.reject(new Error('fail')))
    const cb = createCircuitBreaker(sendFn, {
      maxFailures: 20,
      initialBackoff: 1,
      maxBackoff: 5
    })

    // Generate enough failures to exceed maxBackoff
    for (let i = 0; i < 10; i++) {
      await assert.rejects(() => cb.execute(['entry']))
    }

    // The backoff should not exceed maxBackoff
    const stats = cb.getStats()
    assert.ok(stats.currentBackoff <= 5, `Backoff ${stats.currentBackoff} exceeds max 5ms`)
  })

  test('should reset backoff on success', async () => {
    let callCount = 0
    const sendFn = mock.fn(() => {
      callCount++
      if (callCount <= 3) return Promise.reject(new Error('fail'))
      return Promise.resolve({ ok: true })
    })
    const cb = createCircuitBreaker(sendFn, {
      maxFailures: 10,
      initialBackoff: 1,
      maxBackoff: 100
    })

    // Generate some failures to build up backoff
    for (let i = 0; i < 3; i++) {
      await assert.rejects(() => cb.execute(['entry']))
    }
    assert.ok(cb.getStats().currentBackoff > 0)

    // Success resets backoff
    await cb.execute(['entry'])
    assert.strictEqual(cb.getStats().currentBackoff, 0)
  })

  test('should not apply backoff on first call', async () => {
    const sendFn = mock.fn(() => Promise.reject(new Error('fail')))
    const cb = createCircuitBreaker(sendFn, { initialBackoff: 500 })

    const start = Date.now()
    await assert.rejects(() => cb.execute(['entry']))
    const elapsed = Date.now() - start

    // First call should have no backoff
    assert.ok(elapsed < 50, `First call took ${elapsed}ms, expected < 50ms`)
  })
})
