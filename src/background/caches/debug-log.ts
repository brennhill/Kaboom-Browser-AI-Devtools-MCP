/**
 * Purpose: Owns the bounded in-memory background debug log.
 * Docs: docs/features/feature/backend-log-streaming/index.md
 */

import type { DebugLogEntry } from '../../types/runtime/debug.js'

const DEBUG_LOG_MAX_ENTRIES = 200
const debugLogBuffer: DebugLogEntry[] = []

export function getDebugLog(): DebugLogEntry[] {
  return [...debugLogBuffer]
}

export function addDebugLogEntry(entry: DebugLogEntry): void {
  debugLogBuffer.push(entry)
  if (debugLogBuffer.length > DEBUG_LOG_MAX_ENTRIES) {
    const evictCount = Math.ceil(DEBUG_LOG_MAX_ENTRIES * 0.25)
    debugLogBuffer.splice(0, evictCount)
  }
}

export function clearDebugLog(): void {
  debugLogBuffer.length = 0
}
