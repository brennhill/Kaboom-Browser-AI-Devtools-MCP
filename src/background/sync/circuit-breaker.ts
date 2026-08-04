/**
 * Purpose: Implements the circuit breaker pattern with exponential backoff to protect server communication from cascading failures.
 * Docs: docs/features/feature/backend-log-streaming/index.md
 */

/**
 * @fileoverview Circuit Breaker - Implements circuit breaker pattern with
 * exponential backoff for protecting server communication.
 */

import type { CircuitBreakerState, CircuitBreakerStats } from '../../types/runtime/state.js'

/** State change callback type */
export type CircuitBreakerStateChangeCallback = (
  oldState: CircuitBreakerState,
  newState: CircuitBreakerState,
  reason: string
) => void

/** Circuit breaker options */
export interface CircuitBreakerOptions {
  maxFailures?: number
  resetTimeout?: number
  initialBackoff?: number
  maxBackoff?: number
  onStateChange?: CircuitBreakerStateChangeCallback
  now?: () => number
}

/** Transition history entry */
export interface CircuitBreakerTransition {
  from: CircuitBreakerState
  to: CircuitBreakerState
  reason: string
  timestamp: number
}

/** Extended circuit breaker stats */
export interface CircuitBreakerExtendedStats extends CircuitBreakerStats {
  lastFailureTime: number
  lastResetReason: string | null
  transitionHistory: CircuitBreakerTransition[]
}

/** Circuit breaker instance */
export interface CircuitBreaker {
  execute: <T>(args: unknown) => Promise<T>
  getState: () => CircuitBreakerState
  getStats: () => CircuitBreakerStats
  getExtendedStats: () => CircuitBreakerExtendedStats
  reset: (reason?: string) => void
  recordFailure: () => void
  onStateChange: (callback: CircuitBreakerStateChangeCallback) => () => void
}

/**
 * Circuit breaker with exponential backoff for server communication.
 * Prevents the extension from hammering a down/slow server.
 */
export function createCircuitBreaker(
  sendFn: (args: unknown) => Promise<unknown>,
  options: CircuitBreakerOptions = {}
): CircuitBreaker {
  const maxFailures = options.maxFailures ?? 5
  const resetTimeout = options.resetTimeout ?? 30000
  const initialBackoff = options.initialBackoff ?? 1000
  const maxBackoff = options.maxBackoff ?? 30000
  const now = options.now ?? Date.now

  let state: CircuitBreakerState = 'closed'
  let consecutiveFailures = 0
  let totalFailures = 0
  let totalSuccesses = 0
  let currentBackoff = 0
  let lastFailureTime = 0
  let probeInFlight = false
  let lastResetReason: string | null = null
  const stateChangeCallbacks: CircuitBreakerStateChangeCallback[] = []
  const transitionHistory: CircuitBreakerTransition[] = []
  const maxHistorySize = 20

  // Add initial callback if provided
  if (options.onStateChange) {
    stateChangeCallbacks.push(options.onStateChange)
  }

  function recordTransition(from: CircuitBreakerState, to: CircuitBreakerState, reason: string): void {
    if (from === to) return

    transitionHistory.push({ from, to, reason, timestamp: now() })
    if (transitionHistory.length > maxHistorySize) {
      transitionHistory.shift()
    }

    // Notify callbacks
    for (const callback of stateChangeCallbacks) {
      try {
        callback(from, to, reason)
      } catch (err) {
        console.error('[CircuitBreaker] State change callback error:', err)
      }
    }
  }

  function getState(): CircuitBreakerState {
    const oldState = state
    if (state === 'open' && now() - lastFailureTime >= resetTimeout) {
      state = 'half-open'
      recordTransition(oldState, state, 'reset_timeout_elapsed')
    }
    return state
  }

  // #lizard forgives
  function getStats(): CircuitBreakerStats {
    return {
      state: getState(),
      consecutiveFailures,
      totalFailures,
      totalSuccesses,
      currentBackoff
    }
  }

  function getExtendedStats(): CircuitBreakerExtendedStats {
    return {
      ...getStats(),
      lastFailureTime,
      lastResetReason,
      transitionHistory: [...transitionHistory]
    }
  }

  function reset(reason: string = 'manual_reset'): void {
    const oldState = state
    state = 'closed'
    consecutiveFailures = 0
    currentBackoff = 0
    probeInFlight = false
    lastResetReason = reason
    recordTransition(oldState, 'closed', reason)
    console.log(`[CircuitBreaker] Reset: ${reason}`)
  }

  function onSuccess(): void {
    const oldState = state
    consecutiveFailures = 0
    currentBackoff = 0
    totalSuccesses++
    state = 'closed'
    probeInFlight = false
    if (oldState !== 'closed') {
      recordTransition(oldState, 'closed', 'request_success')
    }
  }

  function onFailure(): void {
    const oldState = state
    consecutiveFailures++
    totalFailures++
    lastFailureTime = now()
    probeInFlight = false

    if (consecutiveFailures >= maxFailures && state !== 'open') {
      state = 'open'
      recordTransition(oldState, 'open', `consecutive_failures_${consecutiveFailures}`)
    }

    if (consecutiveFailures > 1) {
      currentBackoff = Math.min(initialBackoff * Math.pow(2, consecutiveFailures - 2), maxBackoff)
    } else {
      currentBackoff = 0
    }
  }

  async function execute<T>(args: unknown): Promise<T> {
    const currentState = getState()

    if (currentState === 'open') {
      throw new Error(
        `Server connection blocked: circuit breaker is open after ${consecutiveFailures} failures. Retrying in ${currentBackoff}ms.`
      )
    }

    if (currentState === 'half-open') {
      if (probeInFlight) {
        throw new Error('Server connection blocked: circuit breaker testing connection recovery')
      }
      probeInFlight = true
    }

    if (currentBackoff > 0) {
      await new Promise<void>((r) => {
        setTimeout(r, currentBackoff)
      })
    }

    try {
      const result = (await sendFn(args)) as T
      onSuccess()
      return result
    } catch (err) {
      onFailure()
      throw err
    }
  }

  function recordFailure(): void {
    const oldState = state
    consecutiveFailures++
    totalFailures++
    lastFailureTime = now()
    if (consecutiveFailures >= maxFailures && state !== 'open') {
      state = 'open'
      recordTransition(oldState, 'open', `consecutive_failures_${consecutiveFailures}`)
    }
    currentBackoff =
      consecutiveFailures >= 2 ? Math.min(initialBackoff * Math.pow(2, consecutiveFailures - 2), maxBackoff) : 0
  }

  function onStateChange(callback: CircuitBreakerStateChangeCallback): () => void {
    stateChangeCallbacks.push(callback)
    return () => {
      const index = stateChangeCallbacks.indexOf(callback)
      if (index > -1) stateChangeCallbacks.splice(index, 1)
    }
  }

  return { execute, getState, getStats, getExtendedStats, reset, recordFailure, onStateChange }
}
