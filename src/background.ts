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
 * Direct use of focused internal modules
 * should go through initialization in init.ts, not through the facade.
 *
 * Main modules:
 * - background/index.ts: Core state and batchers
 * - background/init.ts: Extension startup
 * - background/sync/: Circuit breaking, batching, transport, log processing, and screenshots
 * - background/caches/: Error groups, limits, snapshots, and debug-log ownership
 * - background/polling.ts: Polling loops (internal)
 */

import { initializeExtension } from './background/init.js'
import { EXTENSION_SESSION_ID } from './background/state.js'

// =============================================================================
// === PUBLIC API: CONSTANTS (Test & Init)
// =============================================================================

// Memory enforcement constants
export {
  MEMORY_SOFT_LIMIT,
  MEMORY_HARD_LIMIT,
  MEMORY_CHECK_INTERVAL_MS,
  MEMORY_AVG_LOG_ENTRY_SIZE,
  MEMORY_AVG_WS_EVENT_SIZE,
  MEMORY_AVG_NETWORK_BODY_SIZE,
  MEMORY_AVG_ACTION_SIZE
} from './background/caches/cache-limits.js'

// Rate limiting constants
export { RATE_LIMIT_CONFIG } from './background/sync/batchers.js'

// =============================================================================
// === PUBLIC API: CORE STATE
// =============================================================================

export {
  EXTENSION_SESSION_ID,
  getServerUrl,
  isDebugMode,
  getConnectionStatus,
  getCurrentLogLevel,
  isScreenshotOnError,
  getExtensionLogQueue
} from './background/state.js'

export { DebugCategory } from './background/index.js'

// =============================================================================
// === PUBLIC API: DEBUG LOGGING
// =============================================================================

export { debugLog, getDebugLog, clearDebugLog, exportDebugLog } from './background/index.js'

// =============================================================================
// === PUBLIC API: BATCHERS & CIRCUIT BREAKER
// =============================================================================

export {
  sharedServerCircuitBreaker,
  logBatcher,
  wsBatcher,
  enhancedActionBatcher,
  networkBodyBatcher,
  perfBatcher
} from './background/index.js'

// =============================================================================
// === PUBLIC API: CORE HANDLERS
// =============================================================================

export {
  handleLogMessage,
  handleClearLogs,
  isConnectionCheckRunning,
  checkConnectionAndUpdate
} from './background/index.js'

export { applyCaptureOverrides } from './background/state.js'

// =============================================================================
// === PUBLIC API: VERSION CHECKING
// =============================================================================

export {
  getExtensionVersion,
  isNewVersionAvailable,
  getAvailableVersion,
  updateVersionFromHealth,
  updateVersionBadge,
  getUpdateInfo,
  resetVersionCheck
} from './background/sync/version-check.js'

// =============================================================================
// === PUBLIC API: PENDING QUERIES & PILOT
// =============================================================================

export { handlePendingQuery, handlePilotCommand } from './background/index.js'
export { isAiWebPilotEnabled, markInitComplete } from './background/state.js'

// =============================================================================
// === PUBLIC API: STATE MANAGEMENT (Tests, Initialization)
// =============================================================================

// Error and memory management
export {
  createErrorSignature,
  processErrorGroup,
  flushErrorGroups,
  cleanupStaleErrorGroups
} from './background/caches/error-groups.js'

export {
  canTakeScreenshot,
  recordScreenshot,
  estimateBufferMemory,
  checkMemoryPressure,
  getMemoryPressureState,
  resetMemoryPressureState
} from './background/caches/cache-limits.js'

export {
  getProcessingQueriesState,
  cleanupStaleProcessingQueries
} from './background/caches/snapshots.js'

// Context and annotations
export {
  measureContextSize,
  checkContextAnnotations,
  getContextWarning,
  resetContextWarning
} from './background/caches/snapshots.js'

// Source map management
export { setSourceMapEnabled, isSourceMapEnabled, clearSourceMapCache } from './background/caches/cache-limits.js'

// Cache limits and source map cache
export {
  SOURCE_MAP_CACHE_SIZE,
  setSourceMapCacheEntry,
  getSourceMapCacheEntry,
  getSourceMapCacheSize
} from './background/caches/cache-limits.js'

// =============================================================================
// === PUBLIC API: COMMUNICATION (Tests)
// =============================================================================

export {
  createCircuitBreaker
} from './background/sync/circuit-breaker.js'

export {
  createBatcherWithCircuitBreaker,
  createLogBatcher
} from './background/sync/batchers.js'

export {
  sendLogsToServer,
  sendEnhancedActionsToServer,
  checkServerHealth,
  updateBadge
} from './background/sync/server.js'

export {
  formatLogEntry,
  shouldCaptureLog
} from './background/sync/log-processing.js'

// =============================================================================
// === PUBLIC API: STATE SNAPSHOTS (Initialization)
// =============================================================================

export {
  saveStateSnapshot,
  loadStateSnapshot,
  listStateSnapshots,
  deleteStateSnapshot
} from './background/message-handlers.js'


// =============================================================================
// INITIALIZATION — Only in Chrome extension context, not in Node.js test environment
// =============================================================================

if (typeof (globalThis as Record<string, unknown>).process === 'undefined') {
  const _moduleLoadTime = performance.now()
  console.log(`[DIAGNOSTIC] Module load start at ${_moduleLoadTime.toFixed(2)}ms (${new Date().toISOString()})`)
  console.log(`[KaBOOM!] Background service worker loaded - session ${EXTENSION_SESSION_ID}`)
  initializeExtension()
}
