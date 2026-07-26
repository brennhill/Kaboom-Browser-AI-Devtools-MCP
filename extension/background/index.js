/**
 * Purpose: Main background service worker hub -- owns debug logging, log handling, connection management, and batcher wiring.
 * Why: Central export point that delegates to specialized modules while owning cross-cutting concerns.
 * Docs: docs/features/feature/backend-log-streaming/index.md
 */
import { getServerUrl, getConnectionStatus, getExtensionLogQueue, pushExtensionLog, capExtensionLogs, getCurrentLogLevel, isScreenshotOnError, _setDebugModeRaw, setConnectionStatus, setConnectionCheckRunning, clearExtensionLogQueue, EXTENSION_SESSION_ID, isAiControlled, isAiWebPilotEnabled, isConnectionCheckRunning as isConnectionCheckRunningFlag, isDebugMode, applyCaptureOverrides } from './state.js';
import { addDebugLogEntry, getDebugLog as getDebugLogEntries, clearDebugLog as clearDebugLogEntries, isSourceMapEnabled, resolveStackTrace, processErrorGroup, canTakeScreenshot, recordScreenshot } from './caches/state-manager.js';
import { createCircuitBreaker, RATE_LIMIT_CONFIG, shouldCaptureLog, formatLogEntry, captureScreenshot, updateBadge, checkServerHealth } from './sync/communication.js';
import { DebugCategory } from './debug.js';
import { getRequestHeaders } from './sync/server.js';
import { handlePendingQuery as handlePendingQueryImpl, handlePilotCommand as handlePilotCommandImpl } from './pending-queries.js';
import { updateVersionFromHealth } from './sync/version-check.js';
import { createBatcherInstances } from './sync/batcher-instances.js';
import { KABOOM_LOG_PREFIX } from '../lib/brand.js';
import { errorMessage, isNoReceiverError } from '../lib/error-utils.js';
import { startSyncClient as startSyncClientImpl, resetSyncClientConnection as resetSyncClientConnectionImpl } from './sync/sync-manager.js';
// Re-export for consumers that already import from here
export { DEFAULT_SERVER_URL } from '../lib/constants.js';
// =============================================================================
// DEBUG LOGGING
// =============================================================================
// Re-export DebugCategory from debug module (to avoid circular dependencies)
export { DebugCategory } from './debug.js';
/**
 * Log a diagnostic message only when debug mode is enabled
 */
function diagnosticLog(message) {
    if (isDebugMode()) {
        console.log(message);
    }
}
/**
 * Log a debug message (only when debug mode is enabled)
 */
export function debugLog(category, message, data = null) {
    const timestamp = new Date().toISOString();
    // Cast category to DebugCategory - callers use DebugCategory constants
    const entry = {
        ts: timestamp,
        category: category,
        message,
        ...(data !== null ? { data } : {})
    };
    addDebugLogEntry(entry);
    // Always queue debug logs, even while disconnected, so the next successful
    // sync can flush the full failure timeline to the daemon for root-cause analysis.
    pushExtensionLog({
        timestamp,
        level: 'debug',
        message,
        source: 'background',
        category,
        ...(data !== null ? { data } : {})
    });
    capExtensionLogs(2000);
    if (isDebugMode()) {
        const prefix = `${KABOOM_LOG_PREFIX.slice(0, -1)}:${category}]`;
        if (data !== null) {
            console.log(prefix, message, data); // nosemgrep: javascript.lang.security.audit.unsafe-formatstring.unsafe-formatstring -- console.log with internal error message, not user-controlled
        }
        else {
            console.log(prefix, message); // nosemgrep: javascript.lang.security.audit.unsafe-formatstring.unsafe-formatstring -- console.log with internal error message, not user-controlled
        }
    }
}
;
globalThis.__KABOOM_DEBUG_LOG__ = debugLog;
/**
 * Get all debug log entries
 */
export function getDebugLog() {
    return getDebugLogEntries();
}
/**
 * Clear debug log buffer
 */
export function clearDebugLog() {
    clearDebugLogEntries();
}
/**
 * Export debug log as JSON string
 */
export function exportDebugLog() {
    return JSON.stringify(// WIRE-OK: local debug export, not sent to server
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
/**
 * Set debug mode enabled/disabled
 */
export function setDebugMode(enabled) {
    _setDebugModeRaw(enabled);
    debugLog(DebugCategory.SETTINGS, `Debug mode ${enabled ? 'enabled' : 'disabled'}`);
}
// =============================================================================
// SHARED CIRCUIT BREAKER
// =============================================================================
export const sharedServerCircuitBreaker = createCircuitBreaker(() => Promise.reject(new Error('shared circuit breaker')), {
    maxFailures: RATE_LIMIT_CONFIG.maxFailures,
    resetTimeout: RATE_LIMIT_CONFIG.resetTimeout,
    initialBackoff: 0,
    maxBackoff: 0
});
// =============================================================================
// BATCHERS (delegated to batcher-instances.ts)
// =============================================================================
const _batchers = createBatcherInstances({
    getServerUrl: () => getServerUrl(),
    getConnectionStatus: () => getConnectionStatus(),
    setConnectionStatus: (patch) => {
        setConnectionStatus(patch);
    },
    debugLog
}, sharedServerCircuitBreaker);
export const logBatcherWithCB = _batchers.logBatcherWithCB;
export const logBatcher = _batchers.logBatcher;
const wsBatcherWithCB = _batchers.wsBatcherWithCB;
export const wsBatcher = _batchers.wsBatcher;
export const enhancedActionBatcherWithCB = _batchers.enhancedActionBatcherWithCB;
export const enhancedActionBatcher = _batchers.enhancedActionBatcher;
export const networkBodyBatcherWithCB = _batchers.networkBodyBatcherWithCB;
export const networkBodyBatcher = _batchers.networkBodyBatcher;
export const perfBatcherWithCB = _batchers.perfBatcherWithCB;
export const perfBatcher = _batchers.perfBatcher;
// =============================================================================
// LOG HANDLING
// =============================================================================
async function tryResolveSourceMap(entry) {
    if (!isSourceMapEnabled())
        return entry;
    if (!entry.stack)
        return entry;
    try {
        const resolvedStack = await resolveStackTrace(entry.stack, debugLog);
        const existingEnrichments = entry._enrichments;
        const enrichments = existingEnrichments ? [...existingEnrichments] : [];
        if (!enrichments.includes('sourceMap')) {
            enrichments.push('sourceMap');
        }
        debugLog(DebugCategory.CAPTURE, 'Stack trace resolved via source map');
        return {
            ...entry,
            stack: resolvedStack,
            _sourceMapResolved: true,
            _enrichments: enrichments
        };
    }
    catch (err) {
        debugLog(DebugCategory.ERROR, 'Source map resolution failed', { error: errorMessage(err) });
        return entry;
    }
}
export async function handleLogMessage(payload, sender, tabId) {
    if (!shouldCaptureLog(payload.level, getCurrentLogLevel(), payload.type)) {
        debugLog(DebugCategory.CAPTURE, `Log filtered out: level=${payload.level}, type=${payload.type}` // nosemgrep: missing-template-string-indicator
        );
        return;
    }
    let entry = formatLogEntry(payload);
    const resolvedTabId = tabId ?? sender?.tab?.id;
    if (resolvedTabId !== null && resolvedTabId !== undefined) {
        entry = { ...entry, tabId: resolvedTabId };
    }
    // nosemgrep: missing-template-string-indicator
    debugLog(DebugCategory.CAPTURE, `Log received: type=${entry.type}, level=${entry.level}`, {
        url: entry.url,
        enrichments: entry._enrichments
    });
    entry = await tryResolveSourceMap(entry);
    const { shouldSend, entry: processedEntry } = processErrorGroup(entry);
    if (shouldSend && processedEntry) {
        logBatcher.add(processedEntry);
        // nosemgrep: missing-template-string-indicator
        debugLog(DebugCategory.CAPTURE, `Log queued for server: type=${processedEntry.type}`, {
            aggregatedCount: processedEntry._aggregatedCount
        });
        maybeAutoScreenshot(processedEntry, sender);
    }
    else {
        debugLog(DebugCategory.CAPTURE, 'Log deduplicated (error grouping)');
    }
}
async function maybeAutoScreenshot(errorEntry, sender) {
    if (!isScreenshotOnError())
        return;
    if (!sender?.tab?.id)
        return;
    if (errorEntry.level !== 'error')
        return;
    const entryType = errorEntry.type;
    if (entryType !== 'exception' && entryType !== 'network')
        return;
    const errorId = `err_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`;
    errorEntry._errorId = errorId;
    const result = await captureScreenshot(sender.tab.id, getServerUrl(), errorId, entryType || null, canTakeScreenshot, recordScreenshot, debugLog);
    if (result.success && result.entry) {
        logBatcher.add(result.entry);
    }
}
export async function handleClearLogs() {
    try {
        const response = await fetch(`${getServerUrl()}/logs`, { method: 'DELETE', headers: getRequestHeaders() });
        if (!response.ok)
            return { success: false, error: `HTTP ${response.status}` };
        setConnectionStatus({ entries: 0, errorCount: 0 });
        updateBadge(getConnectionStatus());
        return { success: true };
    }
    catch (error) {
        return { success: false, error: errorMessage(error) };
    }
}
// =============================================================================
// CONNECTION MANAGEMENT
// =============================================================================
/**
 * Check if a connection check is currently running (for testing)
 */
export function isConnectionCheckRunning() {
    return isConnectionCheckRunningFlag();
}
// #lizard forgives
function updateVersionFromHealthSafe(health) {
    try {
        updateVersionFromHealth({ version: health.version, available_version: health.available_version }, debugLog);
    }
    catch (err) {
        debugLog(DebugCategory.CONNECTION, 'Failed to update version info', { error: errorMessage(err) });
    }
}
function applyHealthLogs(health) {
    if (!health.logs)
        return;
    const status = getConnectionStatus();
    setConnectionStatus({
        logFile: health.logs.logFile || status.logFile,
        logFileSize: health.logs.logFileSize,
        entries: health.logs.entries ?? status.entries,
        maxEntries: health.logs.maxEntries ?? status.maxEntries
    });
}
function applyVersionMismatchCheck(health) {
    if (!health.connected || !health.version || typeof chrome === 'undefined')
        return;
    const extVersion = chrome.runtime.getManifest().version;
    setConnectionStatus({
        serverVersion: health.version,
        extensionVersion: extVersion,
        versionMismatch: health.version.split('.')[0] !== extVersion.split('.')[0]
    });
}
function logConnectionChange(wasConnected, health) {
    if (wasConnected === health.connected)
        return;
    debugLog(DebugCategory.CONNECTION, health.connected ? 'Connected to server' : 'Disconnected from server', {
        entries: getConnectionStatus().entries,
        error: health.error || null,
        serverVersion: health.version || null
    });
}
function broadcastStatusUpdate() {
    if (typeof chrome === 'undefined' || !chrome.runtime)
        return;
    chrome.runtime
        .sendMessage({ type: 'status_update', status: { ...getConnectionStatus(), aiControlled: isAiControlled() } })
        .catch((err) => {
        // A closed popup means no listener for this broadcast — expected, not an
        // error. Only surface genuine failures. (#status-update-noise)
        if (isNoReceiverError(err))
            return;
        console.error(`${KABOOM_LOG_PREFIX} Error sending status update:`, err);
    });
}
// eslint-disable-next-line security-node/detect-unhandled-async-errors
export async function checkConnectionAndUpdate() {
    if (isConnectionCheckRunningFlag()) {
        debugLog(DebugCategory.CONNECTION, 'Skipping connection check - already running');
        return;
    }
    setConnectionCheckRunning(true);
    try {
        const health = await checkServerHealth(getServerUrl());
        const wasConnected = getConnectionStatus().connected;
        if (health.connected) {
            updateVersionFromHealthSafe(health);
        }
        setConnectionStatus({ ...health, connected: health.connected });
        applyHealthLogs(health);
        applyVersionMismatchCheck(health);
        updateBadge(getConnectionStatus());
        logConnectionChange(wasConnected, health);
        // Always start sync client - it handles failures gracefully with 1s retry
        startSyncClientImpl(syncManagerDeps);
        broadcastStatusUpdate();
    }
    finally {
        setConnectionCheckRunning(false);
    }
}
// =============================================================================
// SYNC CLIENT (delegated to sync-manager.ts)
// =============================================================================
/** Shared deps object for sync-manager — created once, closures read live state */
const syncManagerDeps = {
    getServerUrl: () => getServerUrl(),
    getExtSessionId: () => EXTENSION_SESSION_ID,
    getConnectionStatus: () => getConnectionStatus(),
    setConnectionStatus: (patch) => {
        setConnectionStatus(patch);
    },
    getAiControlled: () => isAiControlled(),
    getAiWebPilotEnabledCache: () => isAiWebPilotEnabled(),
    getExtensionLogQueue: () => getExtensionLogQueue(),
    clearExtensionLogQueue: () => clearExtensionLogQueue(),
    applyCaptureOverrides,
    debugLog
};
/**
 * Reset sync client connection (call when user enables pilot/tracking)
 */
export function resetSyncClientConnection() {
    resetSyncClientConnectionImpl(debugLog);
}
// Re-export statically imported functions (Service Workers don't support dynamic import())
export const handlePendingQuery = handlePendingQueryImpl;
export const handlePilotCommand = handlePilotCommandImpl;
//# sourceMappingURL=index.js.map