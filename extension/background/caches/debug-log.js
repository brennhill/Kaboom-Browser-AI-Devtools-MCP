/**
 * Purpose: Owns the bounded in-memory background debug log.
 * Docs: docs/features/feature/backend-log-streaming/index.md
 */
const DEBUG_LOG_MAX_ENTRIES = 200;
const debugLogBuffer = [];
export function getDebugLog() {
    return [...debugLogBuffer];
}
export function addDebugLogEntry(entry) {
    debugLogBuffer.push(entry);
    if (debugLogBuffer.length > DEBUG_LOG_MAX_ENTRIES) {
        const evictCount = Math.ceil(DEBUG_LOG_MAX_ENTRIES * 0.25);
        debugLogBuffer.splice(0, evictCount);
    }
}
export function clearDebugLog() {
    debugLogBuffer.length = 0;
}
//# sourceMappingURL=debug-log.js.map