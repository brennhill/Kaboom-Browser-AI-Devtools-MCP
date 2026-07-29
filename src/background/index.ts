/**
 * Purpose: Main background service worker hub -- owns debug logging, log handling, connection management, and batcher wiring.
 * Why: Central export point that delegates to specialized modules while owning cross-cutting concerns.
 * Docs: docs/features/feature/backend-log-streaming/index.md
 */

/**
 * @fileoverview Main Background Service Worker — Business logic and export hub.
 * Mutable state lives in state.ts; this module owns debug logging, log handling,
 * connection management, and batcher wiring. Delegates batcher instance creation
 * to batcher-instances.ts and sync client lifecycle to sync-manager.ts.
 */

import type { LogEntry, ChromeMessageSender } from '../types/index.js'

import {
  getConnectionStatus,
  setConnectionStatus,
  setConnectionCheckRunning,
  isConnectionCheckRunning as isConnectionCheckRunningFlag,
  applyConnectionOverrides,
  type MutableConnectionStatus
} from './runtime-state/connection-state.js'
import {
  getServerUrl,
  getCurrentLogLevel,
  isScreenshotOnError,
  setDebugModeRaw,
  isAiControlled,
  isDebugMode,
  applySettingOverrides
} from './runtime-state/settings-state.js'
import { isAiWebPilotEnabled } from './runtime-state/pilot-state.js'
import {
  getExtensionLogQueueSnapshot,
  pushExtensionLog,
  capExtensionLogs,
  acknowledgeExtensionLogQueue
} from './runtime-state/log-queue.js'
import { EXTENSION_SESSION_ID } from './runtime-state/startup-state.js'
import {
  addDebugLogEntry,
  getDebugLog as getDebugLogEntries,
  clearDebugLog as clearDebugLogEntries
} from './caches/debug-log.js'
import { isSourceMapEnabled, canTakeScreenshot, recordScreenshot } from './caches/cache-limits.js'
import { processErrorGroup } from './caches/error-groups.js'
import { resolveStackTrace } from './caches/snapshots.js'
import { createCircuitBreaker } from './sync/circuit-breaker.js'
import { RATE_LIMIT_CONFIG } from './sync/batchers.js'
import { shouldCaptureLog, formatLogEntry } from './sync/log-processing.js'
import { captureScreenshot } from './sync/screenshot.js'
import { updateBadge, checkServerHealth } from './sync/server.js'
import { getTrackedTabInfo } from './ui/tab-state.js'
import { DebugCategory } from './debug.js'
import { getRequestHeaders } from './sync/server.js'
import { updateVersionFromHealth } from './sync/version-check.js'
import { createBatcherInstances } from './sync/batcher-instances.js'
import { KABOOM_LOG_PREFIX } from '../lib/brand.js'
import { errorMessage, isNoReceiverError } from '../lib/error-utils.js'
import {
  startSyncClient as startSyncClientImpl,
  resetSyncClientConnection as resetSyncClientConnectionImpl
} from './sync/sync-manager.js'

// =============================================================================
// DEBUG LOGGING
// =============================================================================

/**
 * Log a diagnostic message only when debug mode is enabled
 */
function diagnosticLog(message: string): void {
  if (isDebugMode()) {
    console.log(message)
  }
}

/**
 * Log a debug message (only when debug mode is enabled)
 */
export function debugLog(category: string, message: string, data: unknown = null): void {
  const timestamp = new Date().toISOString()
  // Cast category to DebugCategory - callers use DebugCategory constants
  const entry: import('../types/index.js').DebugLogEntry = {
    ts: timestamp,
    category: category as import('../types/index.js').DebugCategory,
    message,
    ...(data !== null ? { data } : {})
  }

  addDebugLogEntry(entry)

  // Always queue debug logs, even while disconnected, so the next successful
  // sync can flush the full failure timeline to the daemon for root-cause analysis.
  pushExtensionLog({
    timestamp,
    level: 'debug',
    message,
    source: 'background',
    category,
    ...(data !== null ? { data } : {})
  })
  capExtensionLogs(2000)

  if (isDebugMode()) {
    const prefix = `${KABOOM_LOG_PREFIX.slice(0, -1)}:${category}]`
    if (data !== null) {
      console.log(prefix, message, data) // nosemgrep: javascript.lang.security.audit.unsafe-formatstring.unsafe-formatstring -- console.log with internal error message, not user-controlled
    } else {
      console.log(prefix, message) // nosemgrep: javascript.lang.security.audit.unsafe-formatstring.unsafe-formatstring -- console.log with internal error message, not user-controlled
    }
  }
}

;(globalThis as { __KABOOM_DEBUG_LOG__?: typeof debugLog }).__KABOOM_DEBUG_LOG__ = debugLog

/**
 * Get all debug log entries
 */
export function getDebugLog() {
  return getDebugLogEntries()
}

/**
 * Clear debug log buffer
 */
export function clearDebugLog(): void {
  clearDebugLogEntries()
}

/**
 * Export debug log as JSON string
 */
export function exportDebugLog(): string {
  return JSON.stringify(
    // WIRE-OK: local debug export, not sent to server
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
    },
    null,
    2
  )
}

/**
 * Set debug mode enabled/disabled
 */
export function setDebugMode(enabled: boolean): void {
  setDebugModeRaw(enabled)
  debugLog(DebugCategory.SETTINGS, `Debug mode ${enabled ? 'enabled' : 'disabled'}`)
}

// =============================================================================
// SHARED CIRCUIT BREAKER
// =============================================================================

export const sharedServerCircuitBreaker = createCircuitBreaker(
  () => Promise.reject(new Error('shared circuit breaker')),
  {
    maxFailures: RATE_LIMIT_CONFIG.maxFailures,
    resetTimeout: RATE_LIMIT_CONFIG.resetTimeout,
    initialBackoff: 0,
    maxBackoff: 0
  }
)

// =============================================================================
// BATCHERS (delegated to batcher-instances.ts)
// =============================================================================

const _batchers = createBatcherInstances(
  {
    getServerUrl: () => getServerUrl(),
    getConnectionStatus: () => getConnectionStatus(),
    setConnectionStatus: (patch) => {
      setConnectionStatus(patch)
    },
    debugLog
  },
  sharedServerCircuitBreaker
)

export const logBatcher = _batchers.logBatcher
const wsBatcherWithCB = _batchers.wsBatcherWithCB
export const wsBatcher = _batchers.wsBatcher
export const enhancedActionBatcher = _batchers.enhancedActionBatcher
export const networkBodyBatcher = _batchers.networkBodyBatcher
export const perfBatcher = _batchers.perfBatcher

// =============================================================================
// LOG HANDLING
// =============================================================================

async function tryResolveSourceMap(entry: LogEntry): Promise<LogEntry> {
  if (!isSourceMapEnabled()) return entry
  if (!(entry as { stack?: string }).stack) return entry

  try {
    const resolvedStack = await resolveStackTrace((entry as { stack: string }).stack, debugLog)
    const existingEnrichments = (entry as { _enrichments?: readonly string[] })._enrichments
    const enrichments: string[] = existingEnrichments ? [...existingEnrichments] : []
    if (!enrichments.includes('sourceMap')) {
      enrichments.push('sourceMap')
    }
    debugLog(DebugCategory.CAPTURE, 'Stack trace resolved via source map')
    return {
      ...entry,
      stack: resolvedStack,
      _sourceMapResolved: true,
      _enrichments: enrichments
    } as LogEntry
  } catch (err) {
    debugLog(DebugCategory.ERROR, 'Source map resolution failed', { error: errorMessage(err) })
    return entry
  }
}

export async function handleLogMessage(payload: LogEntry, sender: ChromeMessageSender, tabId?: number): Promise<void> {
  if (!shouldCaptureLog(payload.level, getCurrentLogLevel(), (payload as { type?: string }).type)) {
    debugLog(
      DebugCategory.CAPTURE,
      `Log filtered out: level=${payload.level}, type=${(payload as { type?: string }).type}` // nosemgrep: missing-template-string-indicator
    )
    return
  }

  let entry = formatLogEntry(payload)

  const resolvedTabId = tabId ?? sender?.tab?.id
  if (resolvedTabId !== null && resolvedTabId !== undefined) {
    entry = { ...entry, tabId: resolvedTabId } as LogEntry
  }

  // nosemgrep: missing-template-string-indicator
  debugLog(DebugCategory.CAPTURE, `Log received: type=${(entry as { type?: string }).type}, level=${entry.level}`, {
    url: (entry as { url?: string }).url,
    enrichments: (entry as { _enrichments?: string[] })._enrichments
  })

  entry = await tryResolveSourceMap(entry)

  const { shouldSend, entry: processedEntry } = processErrorGroup(entry)

  if (shouldSend && processedEntry) {
    logBatcher.add(processedEntry)
    // nosemgrep: missing-template-string-indicator
    debugLog(DebugCategory.CAPTURE, `Log queued for server: type=${(processedEntry as { type?: string }).type}`, {
      aggregatedCount: (processedEntry as { _aggregatedCount?: number })._aggregatedCount
    })

    maybeAutoScreenshot(processedEntry, sender)
  } else {
    debugLog(DebugCategory.CAPTURE, 'Log deduplicated (error grouping)')
  }
}

async function maybeAutoScreenshot(errorEntry: LogEntry, sender: ChromeMessageSender): Promise<void> {
  if (!isScreenshotOnError()) return
  if (!sender?.tab?.id) return
  if (errorEntry.level !== 'error') return

  const entryType = (errorEntry as { type?: string }).type
  if (entryType !== 'exception' && entryType !== 'network') return

  const errorId = `err_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`
  ;(errorEntry as { _errorId?: string })._errorId = errorId

  const result = await captureScreenshot(
    sender.tab.id,
    getServerUrl(),
    errorId,
    canTakeScreenshot,
    recordScreenshot,
    debugLog
  )

  if (result.success && result.entry) {
    logBatcher.add(result.entry)
  }
}

export async function handleClearLogs(): Promise<{ success: boolean; error?: string }> {
  try {
    const response = await fetch(`${getServerUrl()}/logs`, { method: 'DELETE', headers: getRequestHeaders() })
    if (!response.ok) return { success: false, error: `HTTP ${response.status}` }
    setConnectionStatus({ entries: 0, errorCount: 0 })
    updateBadge(getConnectionStatus())
    return { success: true }
  } catch (error) {
    return { success: false, error: errorMessage(error) }
  }
}

// =============================================================================
// CONNECTION MANAGEMENT
// =============================================================================

/**
 * Check if a connection check is currently running (for testing)
 */
export function isConnectionCheckRunning(): boolean {
  return isConnectionCheckRunningFlag()
}

// #lizard forgives
function updateVersionFromHealthSafe(health: { version?: string; available_version?: string }): void {
  try {
    updateVersionFromHealth({ version: health.version, available_version: health.available_version }, debugLog)
  } catch (err) {
    debugLog(DebugCategory.CONNECTION, 'Failed to update version info', { error: errorMessage(err) })
  }
}

function applyHealthLogs(health: {
  logs?: { logFile?: string; logFileSize?: number; entries?: number; maxEntries?: number }
}): void {
  if (!health.logs) return
  const status = getConnectionStatus()
  setConnectionStatus({
    logFile: health.logs.logFile || status.logFile,
    logFileSize: health.logs.logFileSize,
    entries: health.logs.entries ?? status.entries,
    maxEntries: health.logs.maxEntries ?? status.maxEntries
  })
}

function applyVersionMismatchCheck(health: { connected: boolean; version?: string }): void {
  if (!health.connected || !health.version || typeof chrome === 'undefined') return
  const extVersion = chrome.runtime.getManifest().version
  setConnectionStatus({
    serverVersion: health.version,
    extensionVersion: extVersion,
    versionMismatch: health.version.split('.')[0] !== extVersion.split('.')[0]
  })
}

function logConnectionChange(
  wasConnected: boolean,
  health: { connected: boolean; error?: string; version?: string }
): void {
  if (wasConnected === health.connected) return
  debugLog(DebugCategory.CONNECTION, health.connected ? 'Connected to server' : 'Disconnected from server', {
    entries: getConnectionStatus().entries,
    error: health.error || null,
    serverVersion: health.version || null
  })
}

function broadcastStatusUpdate(): void {
  if (typeof chrome === 'undefined' || !chrome.runtime) return
  chrome.runtime
    .sendMessage({ type: 'status_update', status: { ...getConnectionStatus(), aiControlled: isAiControlled() } })
    .catch((err) => {
      // A closed popup means no listener for this broadcast — expected, not an
      // error. Only surface genuine failures. (#status-update-noise)
      if (isNoReceiverError(err)) return
      console.error(`${KABOOM_LOG_PREFIX} Error sending status update:`, err)
    })
}

// eslint-disable-next-line security-node/detect-unhandled-async-errors
export async function checkConnectionAndUpdate(): Promise<void> {
  if (isConnectionCheckRunningFlag()) {
    debugLog(DebugCategory.CONNECTION, 'Skipping connection check - already running')
    return
  }
  setConnectionCheckRunning(true)

  try {
    const health = await checkServerHealth(getServerUrl())
    const wasConnected = getConnectionStatus().connected

    if (health.connected) {
      updateVersionFromHealthSafe(health)
    }

    setConnectionStatus({ ...health, connected: health.connected })
    applyHealthLogs(health)
    applyVersionMismatchCheck(health)

    updateBadge(getConnectionStatus())
    logConnectionChange(wasConnected, health)

    // Always start sync client - it handles failures gracefully with 1s retry
    startSyncClientImpl(syncManagerDeps)
    broadcastStatusUpdate()
  } finally {
    setConnectionCheckRunning(false)
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
  setConnectionStatus: (patch: Partial<MutableConnectionStatus>) => {
    setConnectionStatus(patch)
  },
  getAiControlled: () => isAiControlled(),
  getAiWebPilotEnabledCache: () => isAiWebPilotEnabled(),
  getExtensionLogQueue: () => getExtensionLogQueueSnapshot(),
  acknowledgeExtensionLogQueue: (sentCount: number) => acknowledgeExtensionLogQueue(sentCount),
  applyCaptureOverrides: (overrides: Record<string, string>) => {
    applySettingOverrides(overrides)
    applyConnectionOverrides(overrides)
  },
  debugLog
}

/**
 * Reset sync client connection (call when user enables pilot/tracking)
 */
export function resetSyncClientConnection(): void {
  resetSyncClientConnectionImpl(debugLog)
}
