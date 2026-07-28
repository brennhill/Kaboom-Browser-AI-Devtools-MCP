/**
 * Purpose: Implements debounced batching with circuit breaker integration for rate-limited server requests.
 * Why: Prevents connection storms and provides backoff when the MCP server is unavailable.
 * Docs: docs/features/feature/backend-log-streaming/index.md
 */

/**
 * @fileoverview Batchers - Batcher creation and circuit breaker integration for
 * debounced batching of server requests.
 */

import type { MemoryPressureState, TimeoutId, CircuitBreakerState, CircuitBreakerStats } from '../../types/index.js'
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
    getState: () => import('./circuit-breaker.js').CircuitBreakerState
    getStats: () => import('../../types/index.js').CircuitBreakerStats
    reset: () => void
  }
  getConnectionStatus: () => { connected: boolean }
}

/** Batcher configuration options */
export interface BatcherConfig {
  debounceMs?: number
  maxBatchSize?: number
  retryBudget?: number
  maxFailures?: number
  resetTimeout?: number
  sharedCircuitBreaker?: CircuitBreaker
}

/** Log batcher options */
export interface LogBatcherOptions {
  debounceMs?: number
  maxBatchSize?: number
  memoryPressureGetter?: () => MemoryPressureState
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
  const backoffSchedule = RATE_LIMIT_CONFIG.backoffSchedule

  const localConnectionStatus = { connected: true }
  const isSharedCB = !!options.sharedCircuitBreaker

  const cb =
    options.sharedCircuitBreaker ||
    createCircuitBreaker(sendFn as (args: unknown) => Promise<unknown>, {
      maxFailures,
      resetTimeout,
      initialBackoff: 0,
      maxBackoff: 0
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

  function requeueEntries(entries: T[]): void {
    pending = entries.concat(pending).slice(0, MAX_PENDING_BUFFER)
  }

  async function retryWithBackoff(entries: T[]): Promise<void> {
    let retriesLeft = retryBudget - 1
    while (retriesLeft > 0) {
      retriesLeft--

      const stats = cb.getStats()
      const backoff = getScheduledBackoff(stats.consecutiveFailures)
      if (backoff > 0) {
        await new Promise<void>((r) => {
          setTimeout(r, backoff)
        })
      }

      try {
        await attemptSend(entries)
        localConnectionStatus.connected = true
        return
      } catch {
        localConnectionStatus.connected = false
        if (cb.getState() === 'open') {
          requeueEntries(entries)
          return
        }
      }
    }
  }

  async function flushWithCircuitBreaker(): Promise<void> {
    if (pending.length === 0) return

    const entries = pending
    pending = []

    if (timeoutId) {
      clearTimeout(timeoutId)
      timeoutId = null
    }
    if (cb.getState() === 'open') {
      requeueEntries(entries)
      return
    }

    try {
      await attemptSend(entries)
      localConnectionStatus.connected = true
    } catch {
      localConnectionStatus.connected = false
      if (cb.getState() === 'open') {
        requeueEntries(entries)
        return
      }
      await retryWithBackoff(entries)
    }
  }

  const scheduleFlush = (): void => {
    if (timeoutId) return
    timeoutId = setTimeout(() => {
      timeoutId = null
      flushWithCircuitBreaker()
    }, debounceMs)
  }

  const batcher: Batcher<T> = {
    add(entry: T): void {
      if (pending.length >= MAX_PENDING_BUFFER) return
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
      pending = []
      if (timeoutId) {
        clearTimeout(timeoutId)
        timeoutId = null
      }
    },

    getPending(): T[] {
      return [...pending]
    }
  }

  return {
    batcher,
    circuitBreaker: wrappedCircuitBreaker,
    getConnectionStatus: () => ({ ...localConnectionStatus })
  }
}

/**
 * Create a simple log batcher without circuit breaker
 */
export function createLogBatcher<T>(flushFn: (entries: T[]) => void, options: LogBatcherOptions = {}): Batcher<T> {
  const debounceMs = options.debounceMs ?? DEFAULT_DEBOUNCE_MS
  const maxBatchSize = options.maxBatchSize ?? DEFAULT_MAX_BATCH_SIZE
  const memoryPressureGetter = options.memoryPressureGetter ?? null

  let pending: T[] = []
  let timeoutId: TimeoutId | null = null

  const getEffectiveMaxBatchSize = (): number => {
    if (memoryPressureGetter) {
      const state = memoryPressureGetter()
      if (state.reducedCapacities) {
        return Math.floor(maxBatchSize / 2)
      }
    }
    return maxBatchSize
  }

  const flush = (): void => {
    if (pending.length === 0) return

    const entries = pending
    pending = []

    if (timeoutId) {
      clearTimeout(timeoutId)
      timeoutId = null
    }

    flushFn(entries)
  }

  const scheduleFlush = (): void => {
    if (timeoutId) return

    timeoutId = setTimeout(() => {
      timeoutId = null
      flush()
    }, debounceMs)
  }

  return {
    add(entry: T): void {
      if (pending.length >= MAX_PENDING_BUFFER) return
      pending.push(entry)

      const effectiveMax = getEffectiveMaxBatchSize()
      if (pending.length >= effectiveMax) {
        flush()
      } else {
        scheduleFlush()
      }
    },

    flush(): void {
      flush()
    },

    clear(): void {
      pending = []
      if (timeoutId) {
        clearTimeout(timeoutId)
        timeoutId = null
      }
    }
  }
}
