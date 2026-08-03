/**
 * Purpose: Manages pending request/response pairs (highlight, execute_js, a11y, DOM queries) with timeout cleanup for AI Web Pilot features.
 * Docs: docs/features/feature/interact-explore/index.md
 */
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
let initialized = false;
function registerRequest(requests, requestId, resolve, timeoutMs, onTimeout, onCancel) {
    const request = { resolve, timer: null, cancel: onCancel };
    requests.set(requestId, request);
    if (timeoutMs !== undefined && onTimeout !== undefined) {
        request.timer = setTimeout(() => {
            if (!requests.delete(requestId))
                return;
            request.timer = null;
            onTimeout();
        }, timeoutMs);
    }
    return requestId;
}
function resolveRequest(requests, requestId, result) {
    const request = requests.get(requestId);
    if (!request)
        return;
    requests.delete(requestId);
    if (request.timer !== null)
        clearTimeout(request.timer);
    request.resolve(result);
}
function deleteRequest(requests, requestId) {
    const request = requests.get(requestId);
    if (!request)
        return;
    requests.delete(requestId);
    if (request.timer !== null)
        clearTimeout(request.timer);
}
function clearRequests(requests) {
    for (const request of requests.values()) {
        if (request.timer !== null)
            clearTimeout(request.timer);
        request.cancel?.();
    }
    requests.clear();
}
/**
 * Clear all pending request Maps on page unload (Issue 2 fix).
 * Prevents memory leaks and stale request accumulation across navigations.
 */
export function clearPendingRequests() {
    clearRequests(pendingHighlightRequests);
    clearRequests(pendingExecuteRequests);
    clearRequests(pendingA11yRequests);
    clearRequests(pendingDomRequests);
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
export function registerHighlightRequest(resolve, onCancel) {
    const requestId = ++highlightRequestId;
    return registerRequest(pendingHighlightRequests, requestId, resolve, undefined, undefined, onCancel);
}
/**
 * Resolve a highlight request
 */
export function resolveHighlightRequest(requestId, result) {
    resolveRequest(pendingHighlightRequests, requestId, result);
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
    deleteRequest(pendingHighlightRequests, requestId);
}
/**
 * Get the next execute request ID and register a resolver
 */
export function registerExecuteRequest(resolve, timeoutMs, onTimeout) {
    const requestId = ++executeRequestId;
    return registerRequest(pendingExecuteRequests, requestId, resolve, timeoutMs, onTimeout, onTimeout);
}
/**
 * Resolve an execute request
 */
export function resolveExecuteRequest(requestId, result) {
    resolveRequest(pendingExecuteRequests, requestId, result);
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
    deleteRequest(pendingExecuteRequests, requestId);
}
/**
 * Get the next a11y request ID and register a resolver
 */
export function registerA11yRequest(resolve, timeoutMs, onTimeout) {
    const requestId = ++a11yRequestId;
    return registerRequest(pendingA11yRequests, requestId, resolve, timeoutMs, onTimeout, onTimeout);
}
/**
 * Resolve an a11y request
 */
export function resolveA11yRequest(requestId, result) {
    resolveRequest(pendingA11yRequests, requestId, result);
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
    deleteRequest(pendingA11yRequests, requestId);
}
/**
 * Get the next DOM request ID and register a resolver
 */
export function registerDomRequest(resolve, timeoutMs, onTimeout) {
    const requestId = ++domRequestId;
    return registerRequest(pendingDomRequests, requestId, resolve, timeoutMs, onTimeout, onTimeout);
}
/**
 * Resolve a DOM request
 */
export function resolveDomRequest(requestId, result) {
    resolveRequest(pendingDomRequests, requestId, result);
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
    deleteRequest(pendingDomRequests, requestId);
}
/**
 * Cleanup periodic timer (Issue #2 fix).
 * Should be called when content script is shutting down.
 */
export function cleanupRequestTracking() {
    if (initialized) {
        window.removeEventListener('pagehide', clearPendingRequests);
        window.removeEventListener('beforeunload', clearPendingRequests);
        initialized = false;
    }
    clearPendingRequests();
}
/**
 * Initialize request tracking (register cleanup handlers)
 */
export function initRequestTracking() {
    if (initialized)
        return;
    // Register cleanup handlers for page unload/navigation (Issue 2 fix)
    // Using 'pagehide' (modern, fires on both close and navigation) + 'beforeunload' (legacy fallback)
    window.addEventListener('pagehide', clearPendingRequests);
    window.addEventListener('beforeunload', clearPendingRequests);
    initialized = true;
}
//# sourceMappingURL=request-tracking.js.map