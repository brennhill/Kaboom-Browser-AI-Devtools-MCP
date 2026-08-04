/**
 * Purpose: Implements debounced batching with circuit breaker integration for rate-limited server requests.
 * Why: Prevents connection storms and provides backoff when the MCP server is unavailable.
 * Docs: docs/features/feature/backend-log-streaming/index.md
 */
/**
 * @fileoverview Batchers - Batcher creation and circuit breaker integration for
 * debounced batching of server requests.
 */
import type { CircuitBreakerState, CircuitBreakerStats } from '../../types/runtime/state.js';
import type { TimeoutId } from '../../types/utils.js';
import { type CircuitBreaker } from './circuit-breaker.js';
/** Rate limit configuration */
export declare const RATE_LIMIT_CONFIG: {
    maxFailures: number;
    resetTimeout: number;
    backoffSchedule: readonly number[];
    retryBudget: number;
};
/** Batcher instance */
export interface Batcher<T> {
    add: (entry: T) => void;
    flush: () => Promise<void> | void;
    clear: () => void;
    getPending?: () => T[];
}
/** Batcher with circuit breaker result */
export interface BatcherWithCircuitBreaker<T> {
    batcher: Batcher<T>;
    circuitBreaker: {
        getState: () => CircuitBreakerState;
        getStats: () => CircuitBreakerStats;
        reset: () => void;
    };
    getConnectionStatus: () => {
        connected: boolean;
    };
    getPressureStats: () => BatcherPressureStats;
}
export interface BatcherPressureStats {
    accepted: number;
    delivered: number;
    dropped: number;
    pending: number;
    capacity: number;
    saturated: boolean;
}
export interface BatcherPressureEvent {
    reason: 'capacity' | 'requeue_overflow' | 'clear';
    dropped: number;
    pending: number;
    capacity: number;
    total_dropped: number;
}
export interface BatcherRuntime {
    now: () => number;
    setTimeout: (callback: () => void, delay: number) => TimeoutId;
    clearTimeout: (id: TimeoutId) => void;
}
/** Batcher configuration options */
export interface BatcherConfig {
    debounceMs?: number;
    maxBatchSize?: number;
    retryBudget?: number;
    maxFailures?: number;
    resetTimeout?: number;
    sharedCircuitBreaker?: CircuitBreaker;
    maxPendingEntries?: number;
    onPressure?: (event: BatcherPressureEvent) => void;
    onPressureRecovered?: () => void;
    runtime?: BatcherRuntime;
}
/**
 * Creates a batcher wired with circuit breaker logic for rate limiting.
 */
export declare function createBatcherWithCircuitBreaker<T>(sendFn: (entries: T[]) => Promise<unknown>, options?: BatcherConfig): BatcherWithCircuitBreaker<T>;
//# sourceMappingURL=batchers.d.ts.map