/**
 * Purpose: Implements the circuit breaker pattern with exponential backoff to protect server communication from cascading failures.
 * Docs: docs/features/feature/backend-log-streaming/index.md
 */
/**
 * @fileoverview Circuit Breaker - Implements circuit breaker pattern with
 * exponential backoff for protecting server communication.
 */
import type { CircuitBreakerState, CircuitBreakerStats } from '../types/index.js';
export type { CircuitBreakerState, CircuitBreakerStats };
/** State change callback type */
export type CircuitBreakerStateChangeCallback = (oldState: CircuitBreakerState, newState: CircuitBreakerState, reason: string) => void;
/** Circuit breaker options */
export interface CircuitBreakerOptions {
    maxFailures?: number;
    resetTimeout?: number;
    initialBackoff?: number;
    maxBackoff?: number;
    onStateChange?: CircuitBreakerStateChangeCallback;
}
/** Transition history entry */
export interface CircuitBreakerTransition {
    from: CircuitBreakerState;
    to: CircuitBreakerState;
    reason: string;
    timestamp: number;
}
/** Extended circuit breaker stats */
export interface CircuitBreakerExtendedStats extends CircuitBreakerStats {
    lastFailureTime: number;
    lastResetReason: string | null;
    transitionHistory: CircuitBreakerTransition[];
}
/** Circuit breaker instance */
export interface CircuitBreaker {
    execute: <T>(args: unknown) => Promise<T>;
    getState: () => CircuitBreakerState;
    getStats: () => CircuitBreakerStats;
    getExtendedStats: () => CircuitBreakerExtendedStats;
    reset: (reason?: string) => void;
    recordFailure: () => void;
    onStateChange: (callback: CircuitBreakerStateChangeCallback) => () => void;
}
/**
 * Circuit breaker with exponential backoff for server communication.
 * Prevents the extension from hammering a down/slow server.
 */
export declare function createCircuitBreaker(sendFn: (args: unknown) => Promise<unknown>, options?: CircuitBreakerOptions): CircuitBreaker;
//# sourceMappingURL=circuit-breaker.d.ts.map