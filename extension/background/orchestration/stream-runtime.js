/**
 * Purpose: Owns background telemetry batchers, log ingestion, automatic screenshots, and log clearing.
 * Docs: docs/features/feature/backend-log-streaming/index.md
 */
import { errorMessage } from '../../lib/error-utils.js';
import { isSourceMapEnabled, canTakeScreenshot, recordScreenshot } from '../caches/cache-limits.js';
import { processErrorGroup } from '../caches/error-groups.js';
import { resolveStackTrace } from '../caches/snapshots.js';
import { DebugCategory, debugLog } from '../debug.js';
import { getConnectionStatus, setConnectionStatus } from '../runtime-state/connection-state.js';
import { getCurrentLogLevel, getServerUrl, isScreenshotOnError } from '../runtime-state/settings-state.js';
import { createBatcherInstances } from '../sync/batcher-instances.js';
import { RATE_LIMIT_CONFIG } from '../sync/batchers.js';
import { createCircuitBreaker } from '../sync/circuit-breaker.js';
import { shouldCaptureLog, formatLogEntry } from '../sync/log-processing.js';
import { captureScreenshot } from '../sync/screenshot.js';
import { getRequestHeaders, updateBadge } from '../sync/server.js';
export const sharedServerCircuitBreaker = createCircuitBreaker(() => Promise.reject(new Error('shared circuit breaker')), {
    maxFailures: RATE_LIMIT_CONFIG.maxFailures,
    resetTimeout: RATE_LIMIT_CONFIG.resetTimeout,
    initialBackoff: 0,
    maxBackoff: 0
});
const batchers = createBatcherInstances({
    getServerUrl,
    getConnectionStatus,
    setConnectionStatus,
    debugLog
}, sharedServerCircuitBreaker);
export const logBatcher = batchers.logBatcher;
export const wsBatcher = batchers.wsBatcher;
export const enhancedActionBatcher = batchers.enhancedActionBatcher;
export const networkBodyBatcher = batchers.networkBodyBatcher;
export const perfBatcher = batchers.perfBatcher;
async function tryResolveSourceMap(entry) {
    if (!isSourceMapEnabled() || !entry.stack)
        return entry;
    try {
        const resolvedStack = await resolveStackTrace(entry.stack, debugLog);
        const enrichments = [...(entry._enrichments ?? [])];
        if (!enrichments.includes('sourceMap'))
            enrichments.push('sourceMap');
        debugLog(DebugCategory.CAPTURE, 'Stack trace resolved via source map');
        return { ...entry, stack: resolvedStack, _sourceMapResolved: true, _enrichments: enrichments };
    }
    catch (error) {
        debugLog(DebugCategory.ERROR, 'Source map resolution failed', { error: errorMessage(error) });
        return entry;
    }
}
async function maybeAutoScreenshot(errorEntry, sender) {
    if (!isScreenshotOnError() || !sender?.tab?.id || errorEntry.level !== 'error')
        return;
    const entryType = errorEntry.type;
    if (entryType !== 'exception' && entryType !== 'network')
        return;
    const errorId = `err_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`;
    errorEntry._errorId = errorId;
    const result = await captureScreenshot(sender.tab.id, getServerUrl(), errorId, canTakeScreenshot, recordScreenshot, debugLog);
    if (result.success && result.entry)
        logBatcher.add(result.entry);
}
export async function handleLogMessage(payload, sender, tabId) {
    if (!shouldCaptureLog(payload.level, getCurrentLogLevel(), payload.type)) {
        debugLog(DebugCategory.CAPTURE, `Log filtered out: level=${payload.level}, type=${payload.type}`);
        return;
    }
    let entry = formatLogEntry(payload);
    const resolvedTabId = tabId ?? sender?.tab?.id;
    if (resolvedTabId !== null && resolvedTabId !== undefined)
        entry = { ...entry, tabId: resolvedTabId };
    debugLog(DebugCategory.CAPTURE, `Log received: type=${entry.type}, level=${entry.level}`, {
        url: entry.url,
        enrichments: entry._enrichments
    });
    entry = await tryResolveSourceMap(entry);
    const { shouldSend, entry: processedEntry } = processErrorGroup(entry);
    if (!shouldSend || !processedEntry) {
        debugLog(DebugCategory.CAPTURE, 'Log deduplicated (error grouping)');
        return;
    }
    logBatcher.add(processedEntry);
    debugLog(DebugCategory.CAPTURE, `Log queued for server: type=${processedEntry.type}`, {
        aggregatedCount: processedEntry._aggregatedCount
    });
    void maybeAutoScreenshot(processedEntry, sender);
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
//# sourceMappingURL=stream-runtime.js.map