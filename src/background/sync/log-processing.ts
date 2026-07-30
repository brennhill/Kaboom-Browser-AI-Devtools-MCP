/**
 * Purpose: Normalizes captured log entries and applies level filtering.
 * Docs: docs/features/feature/backend-log-streaming/index.md
 */

import type { LogEntry } from '../../types/capture/telemetry.js'

function truncateArg(arg: unknown, maxSize = 10240): unknown {
  if (arg === null || arg === undefined) return arg
  try {
    const serialized = JSON.stringify(arg)
    if (serialized.length <= maxSize) return arg
    if (typeof arg === 'string') return arg.slice(0, maxSize) + '... [truncated]'
    return serialized.slice(0, maxSize) + '...[truncated]'
  } catch {
    return typeof arg === 'object' ? '[Circular or unserializable object]' : String(arg)
  }
}

export function formatLogEntry(entry: LogEntry): LogEntry {
  const formatted = { ...entry } as LogEntry & { ts?: string; args?: unknown[] }
  if (!formatted.ts) {
    ;(formatted as { ts: string }).ts = new Date().toISOString()
  }
  if (Array.isArray(formatted.args)) {
    formatted.args = formatted.args.map((arg) => truncateArg(arg))
  }
  return formatted
}

export function shouldCaptureLog(logLevel: string, filterLevel: string, logType?: string): boolean {
  if (logType === 'network' || logType === 'exception') return true
  const levels = ['debug', 'log', 'info', 'warn', 'error']
  const filter = filterLevel === 'all' ? 'debug' : filterLevel
  return levels.indexOf(logLevel) >= levels.indexOf(filter)
}
