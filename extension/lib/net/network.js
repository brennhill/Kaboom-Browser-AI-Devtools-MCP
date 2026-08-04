/**
 * Purpose: Network waterfall capture (PerformanceResourceTiming), fetch body interception with size limits, and sensitive header sanitization.
 * Docs: docs/features/feature/observe/index.md
 */
import { completeRequestAttribution, enrichWaterfallEntries, recordRequestAttribution, resetRequestAttribution } from './request-attribution.js';
import { MAX_WATERFALL_ENTRIES, WATERFALL_TIME_WINDOW_MS, REQUEST_BODY_MAX, RESPONSE_BODY_MAX, BODY_READ_TIMEOUT_MS, SENSITIVE_HEADER_PATTERNS, BINARY_CONTENT_TYPES } from '../constants.js';
// =============================================================================
// MODULE STATE
// =============================================================================
// Configured server URL for filtering (updated via setServerUrl)
let configuredServerUrl = '';
// Network Waterfall state
let networkWaterfallEnabled = false;
const pendingRequests = new Map(); // requestId -> { url, method, startTime }
let requestIdCounter = 0;
// Network body capture state
let networkBodyCaptureEnabled = true; // Default: capture request/response bodies
/** URL patterns for auth endpoints whose response bodies should be redacted */
// nosemgrep: javascript.lang.security.audit.detect-non-literal-regexp.detect-non-literal-regexp -- static literal regex, no ReDoS risk (linear alternation of fixed strings)
const SENSITIVE_URL_PATTERNS = /\/(auth|login|signin|signup|token|oauth|session|api[_-]?key|password|register)\b/i;
// =============================================================================
// NETWORK WATERFALL
// =============================================================================
/**
 * Parse a PerformanceResourceTiming entry into waterfall phases
 * @param timing - The timing entry
 * @returns Parsed waterfall entry
 */
export function parseResourceTiming(timing) {
    const extended = timing;
    const compressionRatio = timing.encodedBodySize > 0 ? timing.decodedBodySize / timing.encodedBodySize : 0;
    return {
        name: timing.name,
        url: timing.name,
        initiator_type: timing.initiatorType,
        start_time: timing.startTime,
        duration: timing.duration,
        fetch_start: timing.fetchStart || undefined,
        response_end: timing.responseEnd || undefined,
        transfer_size: timing.transferSize || 0,
        encoded_body_size: timing.encodedBodySize || 0,
        decoded_body_size: timing.decodedBodySize || 0,
        queueing_ms: phaseDuration(timing.startTime, timing.fetchStart),
        dns_ms: phaseDuration(timing.domainLookupStart, timing.domainLookupEnd),
        tls_ms: phaseDuration(timing.secureConnectionStart, timing.connectEnd),
        connect_ms: phaseDuration(timing.connectStart, timing.connectEnd),
        ttfb_ms: phaseDuration(timing.requestStart, timing.responseStart),
        download_ms: phaseDuration(timing.responseStart, timing.responseEnd),
        protocol: timing.nextHopProtocol || undefined,
        cache_source: inferCacheSource(timing, extended.deliveryType),
        compression_ratio: compressionRatio > 0 ? Math.round(compressionRatio * 100) / 100 : undefined,
        status: extended.responseStatus || undefined,
        server_timing: timing.serverTiming?.map((metric) => ({
            name: metric.name,
            duration_ms: metric.duration || undefined,
            description: metric.description || undefined
        }))
    };
}
function phaseDuration(start, end) {
    if (!Number.isFinite(start) || !Number.isFinite(end) || end < start || (start === 0 && end === 0))
        return undefined;
    return Math.round((end - start) * 1000) / 1000;
}
function inferCacheSource(timing, deliveryType) {
    if (deliveryType)
        return deliveryType;
    if (timing.transferSize === 0 && timing.decodedBodySize > 0)
        return 'memory_or_disk_cache';
    if (timing.transferSize > 0)
        return 'network';
    return 'unavailable';
}
/**
 * Get network waterfall entries
 * @param options - Options for filtering
 * @returns Array of waterfall entries
 */
export function getNetworkWaterfall(options = {}) {
    if (typeof performance === 'undefined' || !performance)
        return [];
    try {
        let entries = performance.getEntriesByType('resource') || [];
        // Filter by time range
        if (options.since) {
            entries = entries.filter((e) => e.startTime >= options.since);
        }
        // Filter by initiator type
        if (options.initiatorTypes) {
            entries = entries.filter((e) => options.initiatorTypes.includes(e.initiatorType));
        }
        // Exclude data URLs
        entries = entries.filter((e) => !e.name.startsWith('data:'));
        // Sort by start time
        entries.sort((a, b) => a.startTime - b.startTime);
        // Limit entries
        if (entries.length > MAX_WATERFALL_ENTRIES) {
            entries = entries.slice(-MAX_WATERFALL_ENTRIES);
        }
        return enrichWaterfallEntries(entries.map(parseResourceTiming));
    }
    catch {
        return [];
    }
}
/**
 * Track a pending request
 * @param request - Request info { url, method, startTime }
 * @returns Request ID
 */
export function trackPendingRequest(request) {
    const id = `req_${++requestIdCounter}`;
    pendingRequests.set(id, {
        ...request,
        id
    });
    return id;
}
/**
 * Complete a pending request
 * @param requestId - The request ID to complete
 */
export function completePendingRequest(requestId) {
    pendingRequests.delete(requestId);
}
/**
 * Get all pending requests
 * @returns Array of pending requests
 */
export function getPendingRequests() {
    return Array.from(pendingRequests.values());
}
/**
 * Clear all pending requests
 */
export function clearPendingRequests() {
    pendingRequests.clear();
    resetRequestAttribution();
}
/**
 * Get network waterfall snapshot for an error
 * @param errorEntry - The error entry
 * @returns The waterfall snapshot
 */
export async function getNetworkWaterfallForError(errorEntry) {
    if (!networkWaterfallEnabled)
        return null;
    const now = typeof performance !== 'undefined' && performance?.now ? performance.now() : 0;
    const since = Math.max(0, now - WATERFALL_TIME_WINDOW_MS);
    const entries = getNetworkWaterfall({ since });
    const pending = getPendingRequests();
    return {
        type: 'network_waterfall',
        ts: new Date().toISOString(),
        _errorTs: errorEntry.ts,
        entries,
        pending
    };
}
/**
 * Set whether network waterfall is enabled
 * @param enabled - Whether to enable network waterfall
 */
export function setNetworkWaterfallEnabled(enabled) {
    networkWaterfallEnabled = enabled;
}
/**
 * Check if network waterfall is enabled
 * @returns Whether network waterfall is enabled
 */
export function isNetworkWaterfallEnabled() {
    return networkWaterfallEnabled;
}
// =============================================================================
// NETWORK BODY CAPTURE
// =============================================================================
/**
 * Set whether network body capture is enabled
 * @param enabled - Whether to enable body capture
 */
export function setNetworkBodyCaptureEnabled(enabled) {
    networkBodyCaptureEnabled = enabled;
}
/**
 * Check if network body capture is enabled
 * @returns Whether body capture is enabled
 */
export function isNetworkBodyCaptureEnabled() {
    return networkBodyCaptureEnabled;
}
/**
 * Set the configured server URL for capture filtering.
 * Called when the server URL is loaded from settings.
 * @param url - The server URL (e.g., 'http://localhost:7890')
 */
export function setServerUrl(url) {
    configuredServerUrl = url || '';
}
/**
 * Check if a URL should be captured (not kaboom server or extension)
 * @param url - The URL to check
 * @returns True if the URL should be captured
 */
export function shouldCaptureUrl(url) {
    if (!url)
        return true;
    // Filter against the configured server URL if set
    if (configuredServerUrl) {
        try {
            const serverParsed = new URL(configuredServerUrl);
            const hostPort = serverParsed.host; // e.g., 'localhost:7890'
            if (url.includes(hostPort))
                return false;
        }
        catch {
            // EXPECTED_ABSENCE: page-owned access can normally throw for detached,
            // cross-origin, or hostile objects; logging it would misleadingly blame Kaboom for page behavior.
            // Fall through to hardcoded defaults
        }
    }
    // Hardcoded fallback for default server URL
    if (url.includes('localhost:7890') || url.includes('127.0.0.1:7890'))
        return false;
    if (url.startsWith('chrome-extension://'))
        return false;
    return true;
}
/**
 * Sanitize headers by removing sensitive ones
 * @param headers - Headers to sanitize
 * @returns Sanitized headers object
 */
// #lizard forgives
export function sanitizeHeaders(headers) {
    if (!headers)
        return {};
    const result = {};
    if (headers instanceof Headers || typeof headers.forEach === 'function') {
        // Headers object or Map
        ;
        headers.forEach((value, key) => {
            if (!SENSITIVE_HEADER_PATTERNS.test(key)) {
                result[key] = value;
            }
        });
    }
    else if (typeof headers.entries === 'function') {
        for (const [key, value] of headers.entries()) {
            if (!SENSITIVE_HEADER_PATTERNS.test(key)) {
                result[key] = value;
            }
        }
    }
    else if (typeof headers === 'object') {
        for (const [key, value] of Object.entries(headers)) {
            if (!SENSITIVE_HEADER_PATTERNS.test(key)) {
                result[key] = value;
            }
        }
    }
    return result;
}
/**
 * Truncate request body at 8KB limit
 * @param body - The request body
 * @returns Truncation result
 */
export function truncateRequestBody(body) {
    if (body === null || body === undefined)
        return { body: null, truncated: false };
    if (body.length <= REQUEST_BODY_MAX)
        return { body, truncated: false };
    return { body: body.slice(0, REQUEST_BODY_MAX), truncated: true };
}
/**
 * Truncate response body at 16KB limit
 * @param body - The response body
 * @returns Truncation result
 */
export function truncateResponseBody(body) {
    if (body === null || body === undefined)
        return { body: null, truncated: false };
    if (body.length <= RESPONSE_BODY_MAX)
        return { body, truncated: false };
    return { body: body.slice(0, RESPONSE_BODY_MAX), truncated: true };
}
/**
 * Read a response body, returning text for text types and size info for binary
 * @param response - The cloned response object
 * @returns The body content or binary size placeholder
 */
export async function readResponseBody(response) {
    const contentType = response.headers?.get?.('content-type') || '';
    if (BINARY_CONTENT_TYPES.test(contentType)) {
        const blob = await response.blob();
        return `[Binary: ${blob.size} bytes, ${contentType}]`;
    }
    // Text-like or unknown content type: try reading as text
    return await response.text();
}
/**
 * Read response body with a timeout
 * @param response - The cloned response object
 * @param timeoutMs - Timeout in milliseconds
 * @returns The body or timeout message
 */
export async function readResponseBodyWithTimeout(response, timeoutMs = BODY_READ_TIMEOUT_MS) {
    return Promise.race([
        readResponseBody(response),
        new Promise((resolve) => {
            setTimeout(() => resolve('[Skipped: body read timeout]'), timeoutMs);
        })
    ]);
}
/**
 * Reset all module state for testing purposes
 * Clears pending requests, resets counters, and restores default settings.
 * Call this in beforeEach/afterEach test hooks to prevent test pollution.
 */
export function resetForTesting() {
    configuredServerUrl = '';
    networkWaterfallEnabled = false;
    pendingRequests.clear();
    requestIdCounter = 0;
    networkBodyCaptureEnabled = true;
    unwrapXHR();
    // Clean up early-patch globals if present
    if (typeof window !== 'undefined') {
        delete window.__KABOOM_ORIGINAL_FETCH__;
        delete window.__KABOOM_ORIGINAL_XHR_OPEN__;
        delete window.__KABOOM_ORIGINAL_XHR_SEND__;
        delete window.__KABOOM_EARLY_BODIES__;
    }
}
/**
 * Wrap a fetch function to capture request/response bodies
 * @param fetchFn - The original fetch function
 * @returns Wrapped fetch that captures bodies
 */
function extractFetchInfo(input, init) {
    let url = '';
    let method = 'GET';
    if (typeof input === 'string') {
        url = input;
    }
    else if (input && input.url) {
        url = input.url;
        method = input.method || 'GET';
    }
    if (init) {
        method = init.method || method;
    }
    return { url, method, requestBody: init?.body || null };
}
async function readCapturedBody(url, cloned, contentType) {
    if (SENSITIVE_URL_PATTERNS.test(url))
        return '[REDACTED: auth endpoint]';
    if (!cloned)
        return '';
    if (BINARY_CONTENT_TYPES.test(contentType)) {
        const blob = await cloned.blob();
        return `[Binary: ${blob.size} bytes, ${contentType}]`;
    }
    return readResponseBodyWithTimeout(cloned);
}
function postNetworkBody(win, url, method, response, contentType, requestBody, duration, truncResp, truncReq, responseTruncated) {
    const message = {
        type: 'kaboom_network_body',
        payload: {
            url,
            method,
            status: response.status,
            content_type: contentType,
            request_body: truncReq || (typeof requestBody === 'string' ? requestBody : undefined),
            response_body: truncResp,
            ...(responseTruncated ? { response_truncated: true } : {}),
            duration
        }
    };
    win.postMessage(message, window.location.origin);
}
// =============================================================================
// XHR BODY CAPTURE
// =============================================================================
let originalXHROpen = null;
let originalXHRSend = null;
/**
 * Wrap XMLHttpRequest to capture request/response bodies.
 * Mirrors the fetch body capture behavior for XHR requests.
 * If the early-patch script ran first, uses the saved originals (not the early wrappers).
 */
export function wrapXHRWithBodies() {
    if (typeof XMLHttpRequest === 'undefined')
        return;
    // Check for early-patch: use the saved originals, not the early-patch wrappers
    const earlyOpen = typeof window !== 'undefined' ? window.__KABOOM_ORIGINAL_XHR_OPEN__ : undefined;
    const earlySend = typeof window !== 'undefined' ? window.__KABOOM_ORIGINAL_XHR_SEND__ : undefined;
    originalXHROpen = earlyOpen || XMLHttpRequest.prototype.open;
    originalXHRSend = earlySend || XMLHttpRequest.prototype.send;
    XMLHttpRequest.prototype.open = function (method, url, ...rest) {
        ;
        this.__kaboomMethod = method;
        this.__kaboomUrl = typeof url === 'string' ? url : url.toString();
        return originalXHROpen.apply(this, [method, url, ...rest]);
    };
    XMLHttpRequest.prototype.send = function (body) {
        const url = this.__kaboomUrl || '';
        const method = this.__kaboomMethod || 'GET';
        if (shouldCaptureUrl(url) && networkBodyCaptureEnabled) {
            recordRequestAttribution(url, { stack: new Error().stack });
            const startTime = Date.now();
            const requestBody = typeof body === 'string' ? body : null;
            this.addEventListener('load', function () {
                try {
                    const duration = Date.now() - startTime;
                    const contentType = this.getResponseHeader('content-type') || '';
                    completeRequestAttribution(url, {
                        status: this.status,
                        server_timing: this.getResponseHeader('server-timing'),
                        request_id: this.getResponseHeader('x-request-id'),
                        traceparent: this.getResponseHeader('traceparent'),
                        content_encoding: this.getResponseHeader('content-encoding')
                    });
                    if (BINARY_CONTENT_TYPES.test(contentType))
                        return;
                    const responseType = this.responseType;
                    // Only capture text-like responses
                    if (responseType && responseType !== '' && responseType !== 'text' && responseType !== 'json')
                        return;
                    let responseBody = null;
                    try {
                        responseBody = this.responseText;
                    }
                    catch {
                        // EXPECTED_ABSENCE: inaccessible binary/cross-origin responseText is normal; logging would mislabel metadata capture.
                        return;
                    }
                    const rawReq = SENSITIVE_URL_PATTERNS.test(url) ? '[REDACTED: auth endpoint]' : requestBody;
                    const { body: truncReq } = truncateRequestBody(rawReq);
                    const { body: truncResp, truncated: respTruncated } = truncateResponseBody(responseBody);
                    const win = typeof window !== 'undefined' ? window : null;
                    if (win) {
                        // Build a minimal Response-like shim for postNetworkBody
                        const responseShim = {
                            status: this.status,
                            headers: { get: (h) => this.getResponseHeader(h) }
                        };
                        postNetworkBody(win, url, method, responseShim, contentType, requestBody, duration, truncResp || '', truncReq, respTruncated);
                    }
                }
                catch {
                    // EXPECTED_ABSENCE: page-owned access can normally throw for detached,
                    // cross-origin, or hostile objects; logging it would misleadingly blame Kaboom for page behavior.
                    /* silent — body capture failure should not affect user code */
                }
            });
        }
        return originalXHRSend.call(this, body);
    };
}
/**
 * Restore original XMLHttpRequest.prototype.open and .send
 */
export function unwrapXHR() {
    if (originalXHROpen)
        XMLHttpRequest.prototype.open = originalXHROpen;
    if (originalXHRSend)
        XMLHttpRequest.prototype.send = originalXHRSend;
    originalXHROpen = null;
    originalXHRSend = null;
}
// =============================================================================
// EARLY BODY ADOPTION
// =============================================================================
/**
 * Adopt network bodies buffered by the early-patch script (fetch + XHR).
 * Mirrors adoptEarlyConnections() in websocket.ts: reads from
 * window.__KABOOM_EARLY_BODIES__, posts each as KABOOM_NETWORK_BODY
 * to the content script, then cleans up globals.
 * Called once during Phase 2 installation.
 */
export function adoptEarlyBodies() {
    if (typeof window === 'undefined')
        return;
    const earlyBodies = window.__KABOOM_EARLY_BODIES__;
    if (!earlyBodies || earlyBodies.length === 0) {
        // Clean up globals even if no bodies
        delete window.__KABOOM_ORIGINAL_FETCH__;
        delete window.__KABOOM_ORIGINAL_XHR_OPEN__;
        delete window.__KABOOM_ORIGINAL_XHR_SEND__;
        delete window.__KABOOM_EARLY_BODIES__;
        return;
    }
    let adopted = 0;
    for (const entry of earlyBodies) {
        if (!shouldCaptureUrl(entry.url))
            continue;
        if (!networkBodyCaptureEnabled)
            continue;
        adopted++;
        const { body: truncResp, truncated: respTruncated } = truncateResponseBody(entry.response_body);
        const message = {
            type: 'kaboom_network_body',
            payload: {
                url: entry.url,
                method: entry.method,
                status: entry.status,
                content_type: entry.content_type,
                response_body: truncResp || '',
                ...(respTruncated ? { response_truncated: true } : {}),
                duration: 0 // Duration unknown for early-captured bodies
            }
        };
        window.postMessage(message, window.location.origin);
    }
    if (adopted > 0) {
        console.log(`[KaBOOM!] Adopted ${adopted} early network body(ies)`);
    }
    // Clean up early-patch globals
    delete window.__KABOOM_ORIGINAL_FETCH__;
    delete window.__KABOOM_ORIGINAL_XHR_OPEN__;
    delete window.__KABOOM_ORIGINAL_XHR_SEND__;
    delete window.__KABOOM_EARLY_BODIES__;
}
export function wrapFetchWithBodies(fetchFn) {
    return async function (input, init) {
        const { url, method, requestBody } = extractFetchInfo(input, init);
        if (!shouldCaptureUrl(url))
            return fetchFn(input, init);
        recordRequestAttribution(url, {
            stack: new Error().stack,
            priority: init?.priority
        });
        const startTime = Date.now();
        const response = await fetchFn(input, init);
        const duration = Date.now() - startTime;
        const contentType = response.headers?.get?.('content-type') || '';
        completeRequestAttribution(url, {
            status: response.status,
            server_timing: response.headers?.get?.('server-timing'),
            request_id: response.headers?.get?.('x-request-id'),
            traceparent: response.headers?.get?.('traceparent'),
            content_encoding: response.headers?.get?.('content-encoding')
        });
        const cloned = response.clone ? response.clone() : null;
        const win = typeof window !== 'undefined' ? window : null;
        Promise.resolve()
            .then(async () => {
            try {
                const responseBody = await readCapturedBody(url, cloned, contentType);
                const { body: truncResp, truncated: respTruncated } = truncateResponseBody(responseBody);
                const rawReq = SENSITIVE_URL_PATTERNS.test(url)
                    ? '[REDACTED: auth endpoint]'
                    : typeof requestBody === 'string'
                        ? requestBody
                        : null;
                const { body: truncReq } = truncateRequestBody(rawReq);
                if (win && networkBodyCaptureEnabled) {
                    postNetworkBody(win, url, method, response, contentType, requestBody, duration, truncResp || responseBody, truncReq, respTruncated);
                }
            }
            catch {
                // EXPECTED_ABSENCE: page-owned access can normally throw for detached,
                // cross-origin, or hostile objects; logging it would misleadingly blame Kaboom for page behavior.
                /* Body capture failure should not affect user code */
            }
        })
            .catch((err) => {
            console.debug('[KaBOOM!] Network body capture error:', err);
        });
        return response;
    };
}
//# sourceMappingURL=network.js.map