/**
 * Purpose: Facade re-exporting state management functions from error-groups, cache-limits, and snapshots sub-modules.
 * Why: Provides a single import point so consumers do not need to know which sub-module owns each function.
 */
/**
 * @fileoverview State Manager Facade
 * Re-exports all state management functions from specialized submodules:
 * - error-groups.ts: Error deduplication and grouping
 * - cache-limits.ts: Memory management and source map caching
 * - snapshots.ts: Stack trace resolution and processing queries
 */
// Re-export error grouping functions and types
export { createErrorSignature, processErrorGroup, cleanupStaleErrorGroups, flushErrorGroups, } from './error-groups.js';
// Re-export cache and memory management
export { canTakeScreenshot, recordScreenshot, clearScreenshotTimestamps, estimateBufferMemory, checkMemoryPressure, getMemoryPressureState, resetMemoryPressureState, isNetworkBodyCaptureDisabled, setSourceMapEnabled, isSourceMapEnabled, clearSourceMapCache, MEMORY_SOFT_LIMIT, MEMORY_HARD_LIMIT, MEMORY_CHECK_INTERVAL_MS, MEMORY_AVG_LOG_ENTRY_SIZE, MEMORY_AVG_WS_EVENT_SIZE, MEMORY_AVG_NETWORK_BODY_SIZE, MEMORY_AVG_ACTION_SIZE, MAX_PENDING_BUFFER } from './cache-limits.js';
// Re-export source map and context monitoring
export { measureContextSize, checkContextAnnotations, getContextWarning, resetContextWarning, resolveStackTrace, getProcessingQueriesState, addProcessingQuery, removeProcessingQuery, isQueryProcessing, cleanupStaleProcessingQueries } from './snapshots.js';
/** Debug log buffer */
const debugLogBuffer = [];
/** Debug log buffer size */
const DEBUG_LOG_MAX_ENTRIES = 200;
/**
 * Get all debug log entries
 */
export function getDebugLog() {
    return [...debugLogBuffer];
}
/**
 * Add entry to debug log buffer.
 * Uses batch splice (25% eviction) instead of per-entry shift() to amortize O(n) cost.
 */
export function addDebugLogEntry(entry) {
    debugLogBuffer.push(entry);
    if (debugLogBuffer.length > DEBUG_LOG_MAX_ENTRIES) {
        const evictCount = Math.ceil(DEBUG_LOG_MAX_ENTRIES * 0.25);
        debugLogBuffer.splice(0, evictCount);
    }
}
/**
 * Clear debug log buffer
 */
export function clearDebugLog() {
    debugLogBuffer.length = 0;
}
//# sourceMappingURL=state-manager.js.map