/**
 * Purpose: Defines debug log category constants used across background modules.
 * Why: Standalone module to break circular dependencies between index.ts and its consumers.
 */
import { KABOOM_LOG_PREFIX } from '../lib/brand.js';
import { addDebugLogEntry } from './caches/debug-log.js';
import { getDebugLog as getDebugLogEntries, clearDebugLog as clearDebugLogEntries } from './caches/debug-log.js';
import { isSourceMapEnabled } from './caches/cache-limits.js';
import { getConnectionStatus } from './runtime-state/connection-state.js';
import { pushExtensionLog } from './runtime-state/log-queue.js';
import { getCurrentLogLevel, isDebugMode, isScreenshotOnError, setDebugModeRaw } from './runtime-state/settings-state.js';
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
export function getDebugLog() {
    return getDebugLogEntries();
}
export function clearDebugLog() {
    clearDebugLogEntries();
}
export function exportDebugLog() {
    return JSON.stringify(
    // WIRE-OK: local debug export consumed by the extension UI, not an HTTP payload.
    {
        exportedAt: new Date().toISOString(),
        version: typeof chrome !== 'undefined' ? chrome.runtime.getManifest().version : 'test',
        debugMode: isDebugMode(),
        connectionStatus: getConnectionStatus(),
        settings: {
            logLevel: getCurrentLogLevel(),
            screenshotOnError: isScreenshotOnError(),
            sourceMapEnabled: isSourceMapEnabled()
        },
        entries: getDebugLogEntries()
    }, null, 2);
}
export function setDebugMode(enabled) {
    setDebugModeRaw(enabled);
    debugLog(DebugCategory.SETTINGS, `Debug mode ${enabled ? 'enabled' : 'disabled'}`);
}
;
globalThis.__KABOOM_DEBUG_LOG__ = debugLog;
//# sourceMappingURL=debug.js.map