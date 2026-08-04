/**
 * Purpose: Implements debounced batching with circuit breaker integration for rate-limited server requests.
 * Why: Prevents connection storms and provides backoff when the MCP server is unavailable.
 * Docs: docs/features/feature/backend-log-streaming/index.md
 */

/**
 * @fileoverview Batchers - Batcher creation and circuit breaker integration for
 * debounced batching of server requests.
 */

import type { CircuitBreakerState, CircuitBreakerStats } from '../../types/runtime/state.js'
import type { TimeoutId } from '../../types/utils.js'
import { createCircuitBreaker, type CircuitBreaker } from './circuit-breaker.js'
import { MAX_PENDING_BUFFER } from '../caches/cache-limits.js'

const DEFAULT_DEBOUNCE_MS = 100
const DEFAULT_MAX_BATCH_SIZE = 50

/** Rate limit configuration */
export const RATE_LIMIT_CONFIG = {
  maxFailures: 5,
  resetTimeout: 30000,
  backoffSchedule: [100, 500, 2000] as readonly number[],
  retryBudget: 3
}

/** Batcher instance */
export interface Batcher<T> {
  add: (entry: T) => void
  flush: () => Promise<void> | void
  clear: () => void
  getPending?: () => T[]
}

/** Batcher with circuit breaker result */
export interface BatcherWithCircuitBreaker<T> {
  batcher: Batcher<T>
  circuitBreaker: {
    getState: () => CircuitBreakerState
    getStats: () => CircuitBreakerStats
    reset: () => void
  }
  getConnectionStatus: () => { connected: boolean }
  getPressureStats: () => BatcherPressureStats
}

export interface BatcherPressureStats {
  accepted: number
  delivered: number
  dropped: number
  pending: number
  capacity: number
  saturated: boolean
}

export interface BatcherPressureEvent {
  reason: 'capacity' | 'requeue_overflow' | 'clear'
  dropped: number
  pending: number
  capacity: number
  total_dropped: number
}

export interface BatcherRuntime {
  now: () => number
  setTimeout: (callback: () => void, delay: number) => TimeoutId
  clearTimeout: (id: TimeoutId) => void
}

/** Batcher configuration options */
export interface BatcherConfig {
  debounceMs?: number
  maxBatchSize?: number
  retryBudget?: number
  maxFailures?: number
  resetTimeout?: number
  sharedCircuitBreaker?: CircuitBreaker
  maxPendingEntries?: number
  onPressure?: (event: BatcherPressureEvent) => void
  onPressureRecovered?: () => void
  runtime?: BatcherRuntime
}

/**
 * Creates a batcher wired with circuit breaker logic for rate limiting.
 */
export function createBatcherWithCircuitBreaker<T>(
  sendFn: (entries: T[]) => Promise<unknown>,
  options: BatcherConfig = {}
): BatcherWithCircuitBreaker<T> {
  const debounceMs = options.debounceMs ?? DEFAULT_DEBOUNCE_MS
  const maxBatchSize = options.maxBatchSize ?? DEFAULT_MAX_BATCH_SIZE
  const retryBudget = options.retryBudget ?? RATE_LIMIT_CONFIG.retryBudget
  const maxFailures = options.maxFailures ?? RATE_LIMIT_CONFIG.maxFailures
  const resetTimeout = options.resetTimeout ?? RATE_LIMIT_CONFIG.resetTimeout
  const capacity = options.maxPendingEntries ?? MAX_PENDING_BUFFER
  const runtime: BatcherRuntime = options.runtime ?? {
    now: Date.now,
    setTimeout: (callback, delay) => {
      const id = setTimeout(callback, delay)
      const nodeTimer = id as unknown as { unref?: () => void }
      nodeTimer.unref?.()
      return id
    },
    clearTimeout: (id) => clearTimeout(id)
  }
  const backoffSchedule = RATE_LIMIT_CONFIG.backoffSchedule

  const localConnectionStatus = { connected: true }
  const isSharedCB = !!options.sharedCircuitBreaker

  const cb =
    options.sharedCircuitBreaker ||
    createCircuitBreaker(sendFn as (args: unknown) => Promise<unknown>, {
      maxFailures,
      resetTimeout,
      initialBackoff: 0,
      maxBackoff: 0,
      now: runtime.now
    })

  function getScheduledBackoff(failures: number): number {
    if (failures <= 0) return 0
    const idx = Math.min(failures - 1, backoffSchedule.length - 1)
    return backoffSchedule[idx] as number
  }

  const wrappedCircuitBreaker = {
    getState: () => cb.getState(),
    getStats: () => {
      const stats = cb.getStats()
      return {
        ...stats,
        currentBackoff: getScheduledBackoff(stats.consecutiveFailures)
      }
    },
    reset: () => cb.reset()
  }

  async function attemptSend(entries: T[]): Promise<unknown> {
    if (!isSharedCB) {
      return await cb.execute<unknown>(entries)
    }

    const state = cb.getState()
    if (state === 'open') {
      const stats = cb.getStats()
      throw new Error(
        `Cannot send batch: circuit breaker is open after ${stats.consecutiveFailures} consecutive failures. Will retry automatically.`
      )
    }

    try {
      const result = await sendFn(entries)
      cb.reset()
      return result
    } catch (err) {
      cb.recordFailure()
      throw err
    }
  }

  let pending: T[] = []
  let timeoutId: TimeoutId | null = null
  let recoveryTimeoutId: TimeoutId | null = null
  let flushPromise: Promise<void> | null = null
  let accepted = 0
  let delivered = 0
  let dropped = 0
  let pressureActive = false

  function reportDrop(reason: BatcherPressureEvent['reason'], count: number): void {
    if (count <= 0) return
    dropped += count
    pressureActive = true
    options.onPressure?.({ reason, dropped: count, pending: pending.length, capacity, total_dropped: dropped })
  }

  function resolvePressureAfterDelivery(): void {
    if (!pressureActive || pending.length >= capacity) return
    pressureActive = false
    options.onPressureRecovered?.()
  }

  function scheduleRecovery(): void {
    if (recoveryTimeoutId !== null || pending.length === 0) return
    recoveryTimeoutId = runtime.setTimeout(() => {
      recoveryTimeoutId = null
      void flushWithCircuitBreaker()
    }, resetTimeout)
  }

  function requeueEntries(entries: T[]): void {
    const combined = entries.concat(pending)
    pending = combined.slice(0, capacity)
    reportDrop('requeue_overflow', combined.length - pending.length)
    if (cb.getState() === 'open') scheduleRecovery()
  }

  async function retryWithBackoff(entries: T[]): Promise<boolean> {
    let retriesLeft = retryBudget - 1
    while (retriesLeft > 0) {
      retriesLeft--

      const stats = cb.getStats()
      const backoff = getScheduledBackoff(stats.consecutiveFailures)
      if (backoff > 0) {
        await new Promise<void>((r) => {
          runtime.setTimeout(r, backoff)
        })
      }

      try {
        await attemptSend(entries)
        delivered += entries.length
        localConnectionStatus.connected = true
        resolvePressureAfterDelivery()
        return true
      } catch {
        localConnectionStatus.connected = false
        if (cb.getState() === 'open') {
          requeueEntries(entries)
          return false
        }
      }
    }
    requeueEntries(entries)
    return false
  }

  async function runFlushChain(): Promise<void> {
    while (pending.length > 0) {
      const shouldContinue = await flushPendingOnce()
      if (!shouldContinue) return
    }
  }

  async function flushPendingOnce(): Promise<boolean> {
    if (pending.length === 0) return false

    const entries = pending
    pending = []

    if (timeoutId) {
      runtime.clearTimeout(timeoutId)
      timeoutId = null
    }
    if (cb.getState() === 'open') {
      requeueEntries(entries)
      return false
    }

    try {
      await attemptSend(entries)
      delivered += entries.length
      localConnectionStatus.connected = true
      resolvePressureAfterDelivery()
      return true
    } catch {
      localConnectionStatus.connected = false
      if (cb.getState() === 'open') {
        requeueEntries(entries)
        return false
      }
      return await retryWithBackoff(entries)
    }
  }

  function flushWithCircuitBreaker(): Promise<void> {
    if (flushPromise) return flushPromise
    flushPromise = runFlushChain().finally(() => {
      flushPromise = null
    })
    return flushPromise
  }

  const scheduleFlush = (): void => {
    if (timeoutId) return
    timeoutId = runtime.setTimeout(() => {
      timeoutId = null
      flushWithCircuitBreaker()
    }, debounceMs)
  }

  const batcher: Batcher<T> = {
    add(entry: T): void {
      accepted++
      if (pending.length >= capacity) {
        reportDrop('capacity', 1)
        return
      }
      pending.push(entry)
      if (pending.length >= maxBatchSize) {
        flushWithCircuitBreaker()
      } else {
        scheduleFlush()
      }
    },

    async flush(): Promise<void> {
      await flushWithCircuitBreaker()
    },

    clear(): void {
      reportDrop('clear', pending.length)
      pending = []
      if (timeoutId) {
        runtime.clearTimeout(timeoutId)
        timeoutId = null
      }
      if (recoveryTimeoutId !== null) {
        runtime.clearTimeout(recoveryTimeoutId)
        recoveryTimeoutId = null
      }
    },

    getPending(): T[] {
      return [...pending]
    }
  }

  return {
    batcher,
    circuitBreaker: wrappedCircuitBreaker,
    getConnectionStatus: () => ({ ...localConnectionStatus }),
    getPressureStats: () => ({
      accepted,
      delivered,
      dropped,
      pending: pending.length,
      capacity,
      saturated: dropped > 0
    })
  }
}
