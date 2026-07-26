/**
 * Purpose: Network waterfall capture (PerformanceResourceTiming), fetch body interception with size limits, and sensitive header sanitization.
 * Docs: docs/features/feature/observe/index.md
 */
/**
 * @fileoverview Network waterfall and body capture.
 * Provides PerformanceResourceTiming parsing, pending request tracking,
 * fetch body capture with size limits, and sensitive header sanitization.
 */
import type { WaterfallEntry, PendingRequest } from '../../types/index.js';
/**
 * Options for filtering network waterfall entries
 */
interface WaterfallFilterOptions {
    since?: number;
    initiatorTypes?: string[];
}
/**
 * Truncation result for request/response bodies
 */
interface TruncationResult {
    body: string | null;
    truncated: boolean;
}
/**
 * Request info for tracking
 */
interface RequestInfo {
    url: string;
    method: string;
    startTime: number;
}
/**
 * Parse a PerformanceResourceTiming entry into waterfall phases
 * @param timing - The timing entry
 * @returns Parsed waterfall entry
 */
export declare function parseResourceTiming(timing: PerformanceResourceTiming): WaterfallEntry;
/**
 * Get network waterfall entries
 * @param options - Options for filtering
 * @returns Array of waterfall entries
 */
export declare function getNetworkWaterfall(options?: WaterfallFilterOptions): WaterfallEntry[];
/**
 * Track a pending request
 * @param request - Request info { url, method, startTime }
 * @returns Request ID
 */
export declare function trackPendingRequest(request: RequestInfo): string;
/**
 * Complete a pending request
 * @param requestId - The request ID to complete
 */
export declare function completePendingRequest(requestId: string): void;
/**
 * Get all pending requests
 * @returns Array of pending requests
 */
export declare function getPendingRequests(): PendingRequest[];
/**
 * Clear all pending requests
 */
export declare function clearPendingRequests(): void;
/**
 * Network waterfall snapshot for an error
 */
interface NetworkWaterfallSnapshot {
    type: 'network_waterfall';
    ts: string;
    _errorTs: string;
    entries: WaterfallEntry[];
    pending: PendingRequest[];
}
/**
 * Error entry with timestamp
 */
interface ErrorEntry {
    ts: string;
}
/**
 * Get network waterfall snapshot for an error
 * @param errorEntry - The error entry
 * @returns The waterfall snapshot
 */
export declare function getNetworkWaterfallForError(errorEntry: ErrorEntry): Promise<NetworkWaterfallSnapshot | null>;
/**
 * Set whether network waterfall is enabled
 * @param enabled - Whether to enable network waterfall
 */
export declare function setNetworkWaterfallEnabled(enabled: boolean): void;
/**
 * Check if network waterfall is enabled
 * @returns Whether network waterfall is enabled
 */
export declare function isNetworkWaterfallEnabled(): boolean;
/**
 * Set whether network body capture is enabled
 * @param enabled - Whether to enable body capture
 */
export declare function setNetworkBodyCaptureEnabled(enabled: boolean): void;
/**
 * Check if network body capture is enabled
 * @returns Whether body capture is enabled
 */
export declare function isNetworkBodyCaptureEnabled(): boolean;
/**
 * Set the configured server URL for capture filtering.
 * Called when the server URL is loaded from settings.
 * @param url - The server URL (e.g., 'http://localhost:7890')
 */
export declare function setServerUrl(url: string): void;
/**
 * Check if a URL should be captured (not kaboom server or extension)
 * @param url - The URL to check
 * @returns True if the URL should be captured
 */
export declare function shouldCaptureUrl(url: string): boolean;
/**
 * Sanitize headers by removing sensitive ones
 * @param headers - Headers to sanitize
 * @returns Sanitized headers object
 */
export declare function sanitizeHeaders(headers: HeadersInit | Headers | Record<string, string> | null): Record<string, string>;
/**
 * Truncate request body at 8KB limit
 * @param body - The request body
 * @returns Truncation result
 */
export declare function truncateRequestBody(body: string | null | undefined): TruncationResult;
/**
 * Truncate response body at 16KB limit
 * @param body - The response body
 * @returns Truncation result
 */
export declare function truncateResponseBody(body: string | null | undefined): TruncationResult;
/**
 * Read a response body, returning text for text types and size info for binary
 * @param response - The cloned response object
 * @returns The body content or binary size placeholder
 */
export declare function readResponseBody(response: Response): Promise<string>;
/**
 * Read response body with a timeout
 * @param response - The cloned response object
 * @param timeoutMs - Timeout in milliseconds
 * @returns The body or timeout message
 */
export declare function readResponseBodyWithTimeout(response: Response, timeoutMs?: number): Promise<string>;
/**
 * Reset all module state for testing purposes
 * Clears pending requests, resets counters, and restores default settings.
 * Call this in beforeEach/afterEach test hooks to prevent test pollution.
 */
export declare function resetForTesting(): void;
/**
 * Type alias for fetch-like functions (avoids overload complexity)
 */
type FetchLike = (input: RequestInfo | URL, init?: RequestInit) => Promise<Response>;
/**
 * Wrap XMLHttpRequest to capture request/response bodies.
 * Mirrors the fetch body capture behavior for XHR requests.
 * If the early-patch script ran first, uses the saved originals (not the early wrappers).
 */
export declare function wrapXHRWithBodies(): void;
/**
 * Restore original XMLHttpRequest.prototype.open and .send
 */
export declare function unwrapXHR(): void;
/**
 * Adopt network bodies buffered by the early-patch script (fetch + XHR).
 * Mirrors adoptEarlyConnections() in websocket.ts: reads from
 * window.__KABOOM_EARLY_BODIES__, posts each as KABOOM_NETWORK_BODY
 * to the content script, then cleans up globals.
 * Called once during Phase 2 installation.
 */
export declare function adoptEarlyBodies(): void;
export declare function wrapFetchWithBodies(fetchFn: FetchLike): FetchLike;
export {};
//# sourceMappingURL=network.d.ts.map