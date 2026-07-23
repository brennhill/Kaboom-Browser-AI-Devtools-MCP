/**
 * Purpose: Manages pending request/response pairs (highlight, execute_js, a11y, DOM queries) with timeout cleanup for AI Web Pilot features.
 * Docs: docs/features/feature/interact-explore/index.md
 */
// Pending highlight response resolvers (keyed by request ID)
const pendingHighlightRequests = new Map();
let highlightRequestId = 0;
// Pending execute requests waiting for responses from inject.js
const pendingExecuteRequests = new Map();
let executeRequestId = 0;
// Pending a11y audit requests waiting for responses from inject.js
const pendingA11yRequests = new Map();
let a11yRequestId = 0;
// Pending DOM query requests waiting for responses from inject.js
const pendingDomRequests = new Map();
let domRequestId = 0;
// Periodic cleanup timer (Issue #2 fix)
const CLEANUP_INTERVAL_MS = 30000; // 30 seconds
let cleanupTimer = null;
// Track request timestamps for stale detection
const requestTimestamps = new Map();
/**
 * Get request timestamps for stale detection (Issue #2 fix).
 * Returns array of [requestId, timestamp] pairs for cleanup.
 */
function getRequestTimestamps() {
    const timestamps = [];
    for (const [id, timestamp] of requestTimestamps) {
        timestamps.push([id, timestamp]);
    }
    return timestamps;
}
/**
 * Clear all pending request Maps on page unload (Issue 2 fix).
 * Prevents memory leaks and stale request accumulation across navigations.
 */
export function clearPendingRequests() {
    pendingHighlightRequests.clear();
    pendingExecuteRequests.clear();
    pendingA11yRequests.clear();
    pendingDomRequests.clear();
    requestTimestamps.clear();
}
/**
 * Perform periodic cleanup of stale requests (Issue #2 fix).
 * Removes requests older than 60 seconds as a fallback when pagehide/beforeunload don't fire.
 */
function performPeriodicCleanup() {
    const now = Date.now();
    const staleThreshold = 60000; // 60 seconds
    for (const [id, timestamp] of getRequestTimestamps()) {
        if (now - timestamp > staleThreshold) {
            // Remove stale request from all maps
            pendingHighlightRequests.delete(id);
            pendingExecuteRequests.delete(id);
            pendingA11yRequests.delete(id);
            pendingDomRequests.delete(id);
            requestTimestamps.delete(id);
        }
    }
}
/**
 * Get statistics about pending requests (for testing/debugging)
 * @returns Counts of pending requests by type
 */
export function getPendingRequestStats() {
    return {
        highlight: pendingHighlightRequests.size,
        execute: pendingExecuteRequests.size,
        a11y: pendingA11yRequests.size,
        dom: pendingDomRequests.size
    };
}
/**
 * Get the next highlight request ID and register a resolver
 */
export function registerHighlightRequest(resolve) {
    const requestId = ++highlightRequestId;
    pendingHighlightRequests.set(requestId, resolve);
    return requestId;
}
/**
 * Resolve a highlight request
 */
export function resolveHighlightRequest(requestId, result) {
    const resolve = pendingHighlightRequests.get(requestId);
    if (resolve) {
        pendingHighlightRequests.delete(requestId);
        resolve(result);
    }
}
/**
 * Check if a highlight request exists
 */
export function hasHighlightRequest(requestId) {
    return pendingHighlightRequests.has(requestId);
}
/**
 * Delete a highlight request without resolving
 */
export function deleteHighlightRequest(requestId) {
    pendingHighlightRequests.delete(requestId);
}
/**
 * Get the next execute request ID and register a resolver
 */
export function registerExecuteRequest(resolve) {
    const requestId = ++executeRequestId;
    pendingExecuteRequests.set(requestId, resolve);
    return requestId;
}
/**
 * Resolve an execute request
 */
export function resolveExecuteRequest(requestId, result) {
    const resolve = pendingExecuteRequests.get(requestId);
    if (resolve) {
        pendingExecuteRequests.delete(requestId);
        resolve(result);
    }
}
/**
 * Check if an execute request exists
 */
export function hasExecuteRequest(requestId) {
    return pendingExecuteRequests.has(requestId);
}
/**
 * Delete an execute request without resolving
 */
export function deleteExecuteRequest(requestId) {
    pendingExecuteRequests.delete(requestId);
}
/**
 * Get the next a11y request ID and register a resolver
 */
export function registerA11yRequest(resolve) {
    const requestId = ++a11yRequestId;
    pendingA11yRequests.set(requestId, resolve);
    return requestId;
}
/**
 * Resolve an a11y request
 */
export function resolveA11yRequest(requestId, result) {
    const resolve = pendingA11yRequests.get(requestId);
    if (resolve) {
        pendingA11yRequests.delete(requestId);
        resolve(result);
    }
}
/**
 * Check if an a11y request exists
 */
export function hasA11yRequest(requestId) {
    return pendingA11yRequests.has(requestId);
}
/**
 * Delete an a11y request without resolving
 */
export function deleteA11yRequest(requestId) {
    pendingA11yRequests.delete(requestId);
}
/**
 * Get the next DOM request ID and register a resolver
 */
export function registerDomRequest(resolve) {
    const requestId = ++domRequestId;
    pendingDomRequests.set(requestId, resolve);
    return requestId;
}
/**
 * Resolve a DOM request
 */
export function resolveDomRequest(requestId, result) {
    const resolve = pendingDomRequests.get(requestId);
    if (resolve) {
        pendingDomRequests.delete(requestId);
        resolve(result);
    }
}
/**
 * Check if a DOM request exists
 */
export function hasDomRequest(requestId) {
    return pendingDomRequests.has(requestId);
}
/**
 * Delete a DOM request without resolving
 */
export function deleteDomRequest(requestId) {
    pendingDomRequests.delete(requestId);
}
/**
 * Cleanup periodic timer (Issue #2 fix).
 * Should be called when content script is shutting down.
 */
export function cleanupRequestTracking() {
    if (cleanupTimer) {
        clearInterval(cleanupTimer);
        cleanupTimer = null;
    }
    clearPendingRequests();
}
/**
 * Initialize request tracking (register cleanup handlers)
 */
export function initRequestTracking() {
    // Register cleanup handlers for page unload/navigation (Issue 2 fix)
    // Using 'pagehide' (modern, fires on both close and navigation) + 'beforeunload' (legacy fallback)
    window.addEventListener('pagehide', clearPendingRequests);
    window.addEventListener('beforeunload', clearPendingRequests);
    // Start periodic cleanup timer (Issue #2 fix)
    // Provides fallback when pagehide/beforeunload don't fire (e.g., page crash)
    cleanupTimer = setInterval(performPeriodicCleanup, CLEANUP_INTERVAL_MS);
}
//# sourceMappingURL=request-tracking.js.map