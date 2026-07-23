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
export declare class TimeoutError extends Error {
    fallback?: unknown | undefined;
    constructor(message: string, fallback?: unknown | undefined);
}
/** Deferred Promise interface */
export interface DeferredPromise<T> {
    promise: Promise<T>;
    resolve: (value: T | PromiseLike<T>) => void;
    reject: (reason?: unknown) => void;
}
/** Create a deferred promise for external resolution */
export declare function createDeferredPromise<T>(): DeferredPromise<T>;
/** Race a promise against a timeout with cleanup on timeout */
export declare function promiseRaceWithCleanup<T>(promise: Promise<T>, timeoutMs: number, timeoutFallback: T | undefined, cleanup?: () => void): Promise<T>;
//# sourceMappingURL=timeout-utils.d.ts.map