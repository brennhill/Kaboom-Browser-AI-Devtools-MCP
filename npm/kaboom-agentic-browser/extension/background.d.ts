/**
 * Purpose: Exposes the extension background facade and re-exports stable public runtime APIs.
 * Why: Keeps service-worker internals modular while preserving a single import surface for startup and tests.
 * Docs: docs/features/feature/analyze-tool/index.md
 * Docs: docs/features/feature/interact-explore/index.md
 * Docs: docs/features/feature/observe/index.md
 */
export { MEMORY_SOFT_LIMIT, MEMORY_HARD_LIMIT, MEMORY_CHECK_INTERVAL_MS, MEMORY_AVG_LOG_ENTRY_SIZE, MEMORY_AVG_WS_EVENT_SIZE, MEMORY_AVG_NETWORK_BODY_SIZE, MEMORY_AVG_ACTION_SIZE } from './background/state-manager.js';
export { RATE_LIMIT_CONFIG } from './background/communication.js';
export { EXTENSION_SESSION_ID, getServerUrl, isDebugMode, getConnectionStatus, getCurrentLogLevel, isScreenshotOnError, getExtensionLogQueue } from './background/state.js';
export { DebugCategory } from './background/index.js';
export { debugLog, getDebugLog, clearDebugLog, exportDebugLog } from './background/index.js';
export { sharedServerCircuitBreaker, logBatcher, wsBatcher, enhancedActionBatcher, networkBodyBatcher, perfBatcher } from './background/index.js';
export { handleLogMessage, handleClearLogs, isConnectionCheckRunning, checkConnectionAndUpdate } from './background/index.js';
export { applyCaptureOverrides } from './background/state.js';
export { sendStatusPingWrapper } from './background/index.js';
export { getExtensionVersion, isNewVersionAvailable, getAvailableVersion, updateVersionFromHealth, updateVersionBadge, getUpdateInfo, resetVersionCheck } from './background/version-check.js';
export { handlePendingQuery, handlePilotCommand } from './background/index.js';
export { isAiWebPilotEnabled, markInitComplete } from './background/state.js';
export { createErrorSignature, processErrorGroup, flushErrorGroups, cleanupStaleErrorGroups, canTakeScreenshot, recordScreenshot, estimateBufferMemory, checkMemoryPressure, getMemoryPressureState, resetMemoryPressureState, getProcessingQueriesState, cleanupStaleProcessingQueries } from './background/state-manager.js';
export { measureContextSize, checkContextAnnotations, getContextWarning, resetContextWarning } from './background/state-manager.js';
export { setSourceMapEnabled, isSourceMapEnabled, clearSourceMapCache } from './background/state-manager.js';
export { SOURCE_MAP_CACHE_SIZE, setSourceMapCacheEntry, getSourceMapCacheEntry, getSourceMapCacheSize } from './background/cache-limits.js';
export { createCircuitBreaker, createBatcherWithCircuitBreaker, createLogBatcher, sendLogsToServer, sendEnhancedActionsToServer, checkServerHealth, updateBadge, formatLogEntry, shouldCaptureLog } from './background/communication.js';
export { saveStateSnapshot, loadStateSnapshot, listStateSnapshots, deleteStateSnapshot } from './background/message-handlers.js';
//# sourceMappingURL=background.d.ts.map