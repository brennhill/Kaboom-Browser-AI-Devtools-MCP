/**
 * Purpose: Manages pending request/response pairs (highlight, execute_js, a11y, DOM queries) with timeout cleanup for AI Web Pilot features.
 * Docs: docs/features/feature/interact-explore/index.md
 */
/**
 * @fileoverview Request Tracking Module
 * Manages pending requests for AI Web Pilot features
 * Includes periodic cleanup timer to handle edge cases where pagehide/beforeunload don't fire.
 */
import type { HighlightResponse, ExecuteJsResult, A11yAuditResult, DomQueryResult } from '../types/index.js';
import type { PendingRequestStats } from './types.js';
/**
 * Clear all pending request Maps on page unload (Issue 2 fix).
 * Prevents memory leaks and stale request accumulation across navigations.
 */
export declare function clearPendingRequests(): void;
/**
 * Get statistics about pending requests (for testing/debugging)
 * @returns Counts of pending requests by type
 */
export declare function getPendingRequestStats(): PendingRequestStats;
/**
 * Get the next highlight request ID and register a resolver
 */
export declare function registerHighlightRequest(resolve: (result: HighlightResponse) => void): number;
/**
 * Resolve a highlight request
 */
export declare function resolveHighlightRequest(requestId: number, result: HighlightResponse): void;
/**
 * Check if a highlight request exists
 */
export declare function hasHighlightRequest(requestId: number): boolean;
/**
 * Delete a highlight request without resolving
 */
export declare function deleteHighlightRequest(requestId: number): void;
/**
 * Get the next execute request ID and register a resolver
 */
export declare function registerExecuteRequest(resolve: (result: ExecuteJsResult) => void): number;
/**
 * Resolve an execute request
 */
export declare function resolveExecuteRequest(requestId: number, result: ExecuteJsResult): void;
/**
 * Check if an execute request exists
 */
export declare function hasExecuteRequest(requestId: number): boolean;
/**
 * Delete an execute request without resolving
 */
export declare function deleteExecuteRequest(requestId: number): void;
/**
 * Get the next a11y request ID and register a resolver
 */
export declare function registerA11yRequest(resolve: (result: A11yAuditResult) => void): number;
/**
 * Resolve an a11y request
 */
export declare function resolveA11yRequest(requestId: number, result: A11yAuditResult): void;
/**
 * Check if an a11y request exists
 */
export declare function hasA11yRequest(requestId: number): boolean;
/**
 * Delete an a11y request without resolving
 */
export declare function deleteA11yRequest(requestId: number): void;
/**
 * Get the next DOM request ID and register a resolver
 */
export declare function registerDomRequest(resolve: (result: DomQueryResult) => void): number;
/**
 * Resolve a DOM request
 */
export declare function resolveDomRequest(requestId: number, result: DomQueryResult): void;
/**
 * Check if a DOM request exists
 */
export declare function hasDomRequest(requestId: number): boolean;
/**
 * Delete a DOM request without resolving
 */
export declare function deleteDomRequest(requestId: number): void;
/**
 * Cleanup periodic timer (Issue #2 fix).
 * Should be called when content script is shutting down.
 */
export declare function cleanupRequestTracking(): void;
/**
 * Initialize request tracking (register cleanup handlers)
 */
export declare function initRequestTracking(): void;
//# sourceMappingURL=request-tracking.d.ts.map