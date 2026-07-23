/**
 * Purpose: Implements debounced batching with circuit breaker integration for rate-limited server requests.
 * Why: Prevents connection storms and provides backoff when the MCP server is unavailable.
 * Docs: docs/features/feature/backend-log-streaming/index.md
 */
/**
 * @fileoverview Batchers - Batcher creation and circuit breaker integration for
 * debounced batching of server requests.
 */
import type { MemoryPressureState } from '../types/index.js';
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
        getState: () => import('./circuit-breaker.js').CircuitBreakerState;
        getStats: () => import('../types/index.js').CircuitBreakerStats;
        reset: () => void;
    };
    getConnectionStatus: () => {
        connected: boolean;
    };
}
/** Batcher configuration options */
export interface BatcherConfig {
    debounceMs?: number;
    maxBatchSize?: number;
    retryBudget?: number;
    maxFailures?: number;
    resetTimeout?: number;
    sharedCircuitBreaker?: CircuitBreaker;
}
/** Log batcher options */
export interface LogBatcherOptions {
    debounceMs?: number;
    maxBatchSize?: number;
    memoryPressureGetter?: () => MemoryPressureState;
}
/**
 * Creates a batcher wired with circuit breaker logic for rate limiting.
 */
export declare function createBatcherWithCircuitBreaker<T>(sendFn: (entries: T[]) => Promise<unknown>, options?: BatcherConfig): BatcherWithCircuitBreaker<T>;
/**
 * Create a simple log batcher without circuit breaker
 */
export declare function createLogBatcher<T>(flushFn: (entries: T[]) => void, options?: LogBatcherOptions): Batcher<T>;
//# sourceMappingURL=batchers.d.ts.map