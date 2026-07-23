/**
 * Purpose: Reusable Promise patterns -- timeout races, cleanup on timeout, and deferred promises for external resolution.
 */
/**
 * @fileoverview Timeout and Promise utilities - Consolidated patterns for handling timeouts,
 * promise races, and async operations with cleanup.
 *
 * Core utilities (5 functions + 1 cleanup variant):
 * - withTimeout: Race a promise against a timeout with optional fallback
 * - withTimeoutAndCleanup: Same as withTimeout but runs a cleanup callback on timeout
 * - delay: Simple promise-based delay
 * - retryWithBackoff: Retry with exponential backoff
 * - fetchWithTimeout: Fetch with AbortController-based timeout
 * - createDeferredPromise: Deferred promise for external resolution
 */
/**
 * Deferred Promise - Holds resolve/reject callbacks for external resolution
 * Useful for resolving a promise from outside async/await context
 * @template T The type of the resolved value
 */
export interface DeferredPromise<T> {
    promise: Promise<T>;
    resolve: (value: T | PromiseLike<T>) => void;
    reject: (reason?: unknown) => void;
}
/**
 * Create a deferred promise
 * @template T The type of the resolved value
 * @returns Deferred promise with resolve/reject methods
 *
 * @example
 * const deferred = createDeferredPromise<number>();
 * setTimeout(() => deferred.resolve(42), 100);
 * const result = await deferred.promise; // 42
 */
export declare function createDeferredPromise<T>(): DeferredPromise<T>;
/**
 * Options for withTimeoutAndCleanup
 */
export interface TimeoutCleanupOptions<T> {
    /** Value to return on timeout instead of rejecting */
    fallback?: T;
    /** Function called when timeout fires (e.g., to remove event listeners) */
    cleanup?: () => void;
}
/**
 * Race a promise against a timeout with cleanup on timeout.
 * Merges the former messageWithTimeout, promiseRaceWithCleanup, and executeWithTimeoutAndCleanup.
 *
 * When the timeout fires, the cleanup callback runs and either the fallback is returned
 * (if provided) or a TimeoutError is thrown.
 *
 * @template T The type of the promise value
 * @param promise The promise to race against timeout
 * @param timeoutMs Timeout in milliseconds
 * @param options Optional fallback and cleanup callbacks
 * @returns Promise that resolves to result, fallback, or rejects on timeout
 *
 * @example
 * // With fallback and cleanup
 * const result = await withTimeoutAndCleanup(
 *   deferred.promise,
 *   5000,
 *   { fallback: { entries: [] }, cleanup: () => removeEventListener('message', handler) }
 * );
 *
 * @example
 * // Cleanup only, no fallback (rejects on timeout)
 * const result = await withTimeoutAndCleanup(
 *   deferred.promise,
 *   30000,
 *   { cleanup: () => pendingRequests.delete(requestId) }
 * );
 */
export declare function withTimeoutAndCleanup<T>(promise: Promise<T>, timeoutMs: number, options?: TimeoutCleanupOptions<T>): Promise<T>;
/**
 * Create a promise that resolves after a delay
 * Useful for retry logic or deferring operations
 *
 * @param delayMs Delay in milliseconds
 * @returns Promise that resolves after the delay
 *
 * @example
 * await delay(1000); // Wait 1 second
 */
export declare function delay(delayMs: number): Promise<void>;
/**
 * Fetch a URL with an AbortController-based timeout.
 * Consolidates the recurring AbortController + setTimeout + clearTimeout pattern.
 *
 * @param url URL to fetch
 * @param options Standard RequestInit (headers, method, body, etc.)
 * @param timeoutMs Timeout in milliseconds before aborting
 * @returns The fetch Response
 */
export declare function fetchWithTimeout(url: string, options: RequestInit, timeoutMs: number): Promise<Response>;
//# sourceMappingURL=timeout-utils.d.ts.map