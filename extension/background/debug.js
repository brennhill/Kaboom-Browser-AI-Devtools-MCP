/**
 * Purpose: Defines debug log category constants used across background modules.
 * Why: Standalone module to break circular dependencies between index.ts and its consumers.
 */
import { KABOOM_LOG_PREFIX } from '../lib/brand.js';
import { addDebugLogEntry } from './caches/debug-log.js';
import { pushExtensionLog, capExtensionLogs } from './runtime-state/log-queue.js';
import { isDebugMode } from './runtime-state/settings-state.js';
/** Log categories for debug output */
export const DebugCategory = {
    CONNECTION: 'connection',
    CAPTURE: 'capture',
    ERROR: 'error',
    LIFECYCLE: 'lifecycle',
    SETTINGS: 'settings',
    SOURCEMAP: 'sourcemap',
    QUERY: 'query'
};
export function debugLog(category, message, data = null) {
    const timestamp = new Date().toISOString();
    const entry = {
        ts: timestamp,
        category: category,
        message,
        ...(data !== null ? { data } : {})
    };
    addDebugLogEntry(entry);
    pushExtensionLog({
        timestamp,
        level: 'debug',
        message,
        source: 'background',
        category,
        ...(data !== null ? { data } : {})
    });
    capExtensionLogs(2000);
    if (!isDebugMode())
        return;
    const prefix = `${KABOOM_LOG_PREFIX.slice(0, -1)}:${category}]`;
    if (data !== null) {
        console.log(prefix, message, data);
    }
    else {
        console.log(prefix, message);
    }
}
;
globalThis.__KABOOM_DEBUG_LOG__ = debugLog;
//# sourceMappingURL=debug.js.map