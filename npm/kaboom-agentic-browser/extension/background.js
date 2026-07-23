/**
 * Purpose: Exposes the extension background facade and re-exports stable public runtime APIs.
 * Why: Keeps service-worker internals modular while preserving a single import surface for startup and tests.
 * Docs: docs/features/feature/analyze-tool/index.md
 * Docs: docs/features/feature/interact-explore/index.md
 * Docs: docs/features/feature/observe/index.md
 */
/**
 * @fileoverview Background Service Worker Facade - Minimal Public API
 *
 * This facade provides a clean, minimal public API for the extension.
 * Direct use of internal modules (communication/, state-manager/, polling/)
 * should go through initialization in init.ts, not through the facade.
 *
 * Main modules:
 * - background/index.ts: Core state and batchers
 * - background/init.ts: Extension startup
 * - background/communication.ts: Server communication (internal)
 * - background/state-manager.ts: State management (internal)
 * - background/polling.ts: Polling loops (internal)
 */
import { initializeExtension } from './background/init.js';
import { EXTENSION_SESSION_ID } from './background/state.js';
// =============================================================================
// === PUBLIC API: CONSTANTS (Test & Init)
// =============================================================================
// Memory enforcement constants
export { MEMORY_SOFT_LIMIT, MEMORY_HARD_LIMIT, MEMORY_CHECK_INTERVAL_MS, MEMORY_AVG_LOG_ENTRY_SIZE, MEMORY_AVG_WS_EVENT_SIZE, MEMORY_AVG_NETWORK_BODY_SIZE, MEMORY_AVG_ACTION_SIZE } from './background/state-manager.js';
// Rate limiting constants
export { RATE_LIMIT_CONFIG } from './background/communication.js';
// =============================================================================
// === PUBLIC API: CORE STATE
// =============================================================================
export { EXTENSION_SESSION_ID, getServerUrl, isDebugMode, getConnectionStatus, getCurrentLogLevel, isScreenshotOnError, getExtensionLogQueue } from './background/state.js';
export { DebugCategory } from './background/index.js';
// =============================================================================
// === PUBLIC API: DEBUG LOGGING
// =============================================================================
export { debugLog, getDebugLog, clearDebugLog, exportDebugLog } from './background/index.js';
// =============================================================================
// === PUBLIC API: BATCHERS & CIRCUIT BREAKER
// =============================================================================
export { sharedServerCircuitBreaker, logBatcher, wsBatcher, enhancedActionBatcher, networkBodyBatcher, perfBatcher } from './background/index.js';
// =============================================================================
// === PUBLIC API: CORE HANDLERS
// =============================================================================
export { handleLogMessage, handleClearLogs, isConnectionCheckRunning, checkConnectionAndUpdate } from './background/index.js';
export { applyCaptureOverrides } from './background/state.js';
// =============================================================================
// === PUBLIC API: POLLING WRAPPERS
// =============================================================================
export { sendStatusPingWrapper } from './background/index.js';
// =============================================================================
// === PUBLIC API: VERSION CHECKING
// =============================================================================
export { getExtensionVersion, isNewVersionAvailable, getAvailableVersion, updateVersionFromHealth, updateVersionBadge, getUpdateInfo, resetVersionCheck } from './background/version-check.js';
// =============================================================================
// === PUBLIC API: PENDING QUERIES & PILOT
// =============================================================================
export { handlePendingQuery, handlePilotCommand } from './background/index.js';
export { isAiWebPilotEnabled, markInitComplete } from './background/state.js';
// =============================================================================
// === PUBLIC API: STATE MANAGEMENT (Tests, Initialization)
// =============================================================================
// Error and memory management
export { createErrorSignature, processErrorGroup, flushErrorGroups, cleanupStaleErrorGroups, canTakeScreenshot, recordScreenshot, estimateBufferMemory, checkMemoryPressure, getMemoryPressureState, resetMemoryPressureState, getProcessingQueriesState, cleanupStaleProcessingQueries } from './background/state-manager.js';
// Context and annotations
export { measureContextSize, checkContextAnnotations, getContextWarning, resetContextWarning } from './background/state-manager.js';
// Source map management
export { setSourceMapEnabled, isSourceMapEnabled, clearSourceMapCache } from './background/state-manager.js';
// Cache limits and source map cache
export { SOURCE_MAP_CACHE_SIZE, setSourceMapCacheEntry, getSourceMapCacheEntry, getSourceMapCacheSize } from './background/cache-limits.js';
// =============================================================================
// === PUBLIC API: COMMUNICATION (Tests)
// =============================================================================
export { createCircuitBreaker, createBatcherWithCircuitBreaker, createLogBatcher, sendLogsToServer, sendEnhancedActionsToServer, checkServerHealth, updateBadge, formatLogEntry, shouldCaptureLog } from './background/communication.js';
// =============================================================================
// === PUBLIC API: STATE SNAPSHOTS (Initialization)
// =============================================================================
export { saveStateSnapshot, loadStateSnapshot, listStateSnapshots, deleteStateSnapshot } from './background/message-handlers.js';
// =============================================================================
// INITIALIZATION — Only in Chrome extension context, not in Node.js test environment
// =============================================================================
if (typeof globalThis.process === 'undefined') {
    const _moduleLoadTime = performance.now();
    console.log(`[DIAGNOSTIC] Module load start at ${_moduleLoadTime.toFixed(2)}ms (${new Date().toISOString()})`);
    console.log(`[KaBOOM!] Background service worker loaded - session ${EXTENSION_SESSION_ID}`);
    initializeExtension();
}
//# sourceMappingURL=background.js.map