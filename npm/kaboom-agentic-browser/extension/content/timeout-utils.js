/**
 * Purpose: Inlined timeout and promise utilities for the content script context where ES module imports are unavailable.
 * Why: Content scripts (MV3) cannot dynamically import from lib/, so these patterns are duplicated here.
 */
/**
 * @fileoverview Timeout Utilities for Content Script
 * Inlined utilities for promise timeout handling
 * Content scripts cannot use ES module imports, so these utilities are duplicated
 */
/** Custom error for timeout operations */
export class TimeoutError extends Error {
    fallback;
    constructor(message, fallback) {
        super(message);
        this.fallback = fallback;
        this.name = 'TimeoutError';
    }
}
/** Create a deferred promise for external resolution */
export function createDeferredPromise() {
    let resolve;
    let reject;
    const promise = new Promise((res, rej) => {
        resolve = res;
        reject = rej;
    });
    return { promise, resolve, reject };
}
/** Race a promise against a timeout with cleanup on timeout */
export async function promiseRaceWithCleanup(promise, timeoutMs, timeoutFallback, cleanup) {
    try {
        return await Promise.race([
            promise,
            new Promise((_, reject) => setTimeout(() => {
                cleanup?.();
                if (timeoutFallback !== undefined) {
                    reject(new TimeoutError(`Operation timed out after ${timeoutMs}ms`, timeoutFallback));
                }
                else {
                    reject(new TimeoutError(`Operation timed out after ${timeoutMs}ms`));
                }
            }, timeoutMs))
        ]);
    }
    catch (err) {
        if (err instanceof TimeoutError && err.fallback !== undefined) {
            return err.fallback;
        }
        throw err;
    }
}
//# sourceMappingURL=timeout-utils.js.map