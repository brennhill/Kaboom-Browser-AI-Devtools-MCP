/**
 * Purpose: Defines debug log category constants used across background modules.
 * Why: Standalone module to break circular dependencies between index.ts and its consumers.
 */

/**
 * @fileoverview Debug Logging Utilities
 * Standalone module to avoid circular dependencies.
 */
import type { DebugLogEntry } from '../types/runtime/debug.js'
import { KABOOM_LOG_PREFIX } from '../lib/brand.js'
import { addDebugLogEntry } from './caches/debug-log.js'
import { getDebugLog as getDebugLogEntries, clearDebugLog as clearDebugLogEntries } from './caches/debug-log.js'
import { isSourceMapEnabled } from './caches/cache-limits.js'
import { getConnectionStatus } from './runtime-state/connection-state.js'
import { pushExtensionLog } from './runtime-state/log-queue.js'
import {
  getCurrentLogLevel,
  isDebugMode,
  isScreenshotOnError,
  setDebugModeRaw
} from './runtime-state/settings-state.js'

/** Log categories for debug output */
export const DebugCategory = {
  CONNECTION: 'connection' as const,
  CAPTURE: 'capture' as const,
  ERROR: 'error' as const,
  LIFECYCLE: 'lifecycle' as const,
  SETTINGS: 'settings' as const,
  SOURCEMAP: 'sourcemap' as const,
  QUERY: 'query' as const
}

export type DebugCategoryType = (typeof DebugCategory)[keyof typeof DebugCategory]

export function debugLog(category: string, message: string, data: unknown = null): void {
  const timestamp = new Date().toISOString()
  const entry: DebugLogEntry = {
    ts: timestamp,
    category: category as DebugLogEntry['category'],
    message,
    ...(data !== null ? { data } : {})
  }

  addDebugLogEntry(entry)
  pushExtensionLog({
    timestamp,
    level: 'debug',
    message,
    source: 'background',
    category,
    ...(data !== null ? { data } : {})
  })
  if (!isDebugMode()) return
  const prefix = `${KABOOM_LOG_PREFIX.slice(0, -1)}:${category}]`
  if (data !== null) {
    console.log(prefix, message, data)
  } else {
    console.log(prefix, message)
  }
}

export function getDebugLog(): DebugLogEntry[] {
  return getDebugLogEntries()
}

export function clearDebugLog(): void {
  clearDebugLogEntries()
}

export function exportDebugLog(): string {
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
    },
    null,
    2
  )
}

export function setDebugMode(enabled: boolean): void {
  setDebugModeRaw(enabled)
  debugLog(DebugCategory.SETTINGS, `Debug mode ${enabled ? 'enabled' : 'disabled'}`)
}

;(globalThis as { __KABOOM_DEBUG_LOG__?: typeof debugLog }).__KABOOM_DEBUG_LOG__ = debugLog
