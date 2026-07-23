/**
 * Purpose: Reusable Promise patterns -- timeout races, cleanup on timeout, and deferred promises for external resolution.
 */
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
export function createDeferredPromise() {
    let resolve;
    let reject;
    const promise = new Promise((res, rej) => {
        resolve = res;
        reject = rej;
    });
    return { promise, resolve, reject };
}
/**
 * Custom error for timeout operations that optionally carries a fallback value
 */
class TimeoutError extends Error {
    fallback;
    constructor(message, fallback) {
        super(message);
        this.fallback = fallback;
        this.name = 'TimeoutError';
    }
}
/**
 * Wrap a promise with a timeout fallback
 * Returns the result of the promise if it resolves before timeout,
 * otherwise returns the fallback value (or rejects if no fallback)
 *
 * Subsumes the former promiseWithTimeout (no fallback) and executeWithTimeout (callback variant).
 *
 * @template T The type of the promise value
 * @param promise The promise to wrap
 * @param timeoutMs Timeout in milliseconds
 * @param fallback Optional fallback value to return on timeout
 * @returns Promise that resolves to the result or fallback (or rejects on timeout if no fallback)
 *
 * @example
 * // With fallback value
 * const result = await withTimeout(fetch('/api'), 5000, { ok: false });
 *
 * @example
 * // Without fallback (rejects on timeout)
 * try {
 *   const result = await withTimeout(slowOperation(), 3000);
 * } catch (err) {
 *   // Handle timeout
 * }
 */
async function withTimeout(promise, timeoutMs, fallback) {
    return Promise.race([
        promise,
        new Promise((_, reject) => {
            setTimeout(() => {
                if (fallback !== undefined) {
                    reject(new TimeoutError(`Operation timed out after ${timeoutMs}ms`, fallback));
                }
                else {
                    reject(new TimeoutError(`Operation timed out after ${timeoutMs}ms`));
                }
            }, timeoutMs);
        })
    ]).catch((err) => {
        if (err instanceof TimeoutError && err.fallback !== undefined) {
            return err.fallback;
        }
        throw err;
    });
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
export async function withTimeoutAndCleanup(promise, timeoutMs, options) {
    const fallback = options?.fallback;
    const cleanup = options?.cleanup;
    let timeoutId;
    try {
        return await Promise.race([
            promise,
            new Promise((_, reject) => {
                timeoutId = setTimeout(() => {
                    cleanup?.();
                    if (fallback !== undefined) {
                        reject(new TimeoutError(`Operation timed out after ${timeoutMs}ms`, fallback));
                    }
                    else {
                        reject(new TimeoutError(`Operation timed out after ${timeoutMs}ms`));
                    }
                }, timeoutMs);
            })
        ]);
    }
    catch (err) {
        if (err instanceof TimeoutError && err.fallback !== undefined) {
            return err.fallback;
        }
        throw err;
    }
    finally {
        if (timeoutId)
            clearTimeout(timeoutId);
    }
}
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
export function delay(delayMs) {
    return new Promise((resolve) => {
        setTimeout(resolve, delayMs);
    });
}
/**
 * Retry a promise-returning function with exponential backoff
 * Useful for flaky operations like network requests
 *
 * @template T The type of the result
 * @param fn Function that returns a promise
 * @param maxAttempts Maximum number of attempts
 * @param initialDelayMs Initial delay before first retry (doubles each attempt)
 * @returns Promise that resolves if any attempt succeeds, or rejects if all fail
 *
 * @example
 * const result = await retryWithBackoff(
 *   () => fetch('/api/data'),
 *   3,
 *   100
 * );
 */
async function retryWithBackoff(fn, maxAttempts = 3, initialDelayMs = 100) {
    let lastError;
    for (let attempt = 0; attempt < maxAttempts; attempt++) {
        try {
            return await fn();
        }
        catch (err) {
            lastError = err;
            if (attempt < maxAttempts - 1) {
                const delayMs = initialDelayMs * Math.pow(2, attempt);
                await delay(delayMs);
            }
        }
    }
    throw lastError;
}
/**
 * Fetch a URL with an AbortController-based timeout.
 * Consolidates the recurring AbortController + setTimeout + clearTimeout pattern.
 *
 * @param url URL to fetch
 * @param options Standard RequestInit (headers, method, body, etc.)
 * @param timeoutMs Timeout in milliseconds before aborting
 * @returns The fetch Response
 */
export async function fetchWithTimeout(url, options, timeoutMs) {
    const controller = new AbortController();
    const id = setTimeout(() => controller.abort(), timeoutMs);
    try {
        return await fetch(url, { ...options, signal: controller.signal });
    }
    finally {
        clearTimeout(id);
    }
}
//# sourceMappingURL=timeout-utils.js.map