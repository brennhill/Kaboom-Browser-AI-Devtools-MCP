/**
 * Purpose: Defines structured log and telemetry entry contracts captured by the extension runtime.
 * Why: Normalizes log payloads so ingestion, redaction, and downstream analysis operate on consistent shapes.
 * Docs: docs/features/feature/normalized-log-schema/index.md
 */

/**
 * @fileoverview Telemetry Data Types
 * Log entries, console logs, network errors, exceptions, and screenshots
 */

import type { AiContextData } from './ai-context.js'

/**
 * Log levels supported by the extension
 */
export type LogLevel = 'debug' | 'log' | 'info' | 'warn' | 'error'

/**
 * Log level filter including 'all' option
 */
export type LogLevelFilter = LogLevel | 'all'

/**
 * Log entry types
 */
export type LogType = 'console' | 'network' | 'exception' | 'screenshot'

/**
 * Base log entry with common fields
 */
export interface BaseLogEntry {
  readonly ts: string
  readonly level: LogLevel
  readonly type?: LogType
  readonly tabId?: number
  readonly _enrichments?: readonly string[]
  readonly _context?: Readonly<Record<string, unknown>>
}

/**
 * Console log entry
 */
export interface ConsoleLogEntry extends BaseLogEntry {
  readonly type: 'console'
  readonly args?: readonly unknown[]
  readonly message?: string
}

/**
 * Network error log entry
 */
export interface NetworkLogEntry extends BaseLogEntry {
  readonly type: 'network'
  readonly level: 'error'
  readonly method: string
  readonly url: string
  readonly status?: number
  readonly statusText?: string
  readonly duration?: number
  readonly response?: string
  readonly error?: string
  readonly headers?: Readonly<Record<string, string>>
}

/**
 * Exception log entry
 */
export interface ExceptionLogEntry extends BaseLogEntry {
  readonly type: 'exception'
  readonly level: 'error'
  readonly message: string
  readonly stack?: string
  readonly filename?: string
  readonly lineno?: number
  readonly colno?: number
  readonly _sourceMapResolved?: boolean
  readonly _errorId?: string
  readonly _aiContext?: AiContextData
}

/**
 * Screenshot log entry
 */
export interface ScreenshotLogEntry extends BaseLogEntry {
  readonly type: 'screenshot'
  readonly url?: string
  readonly screenshotFile?: string
  readonly trigger: 'error' | 'manual'
  readonly relatedErrorId?: string
  readonly _screenshotFailed?: boolean
  readonly error?: string
}

/**
 * Union of all log entry types
 */
export type LogEntry = ConsoleLogEntry | NetworkLogEntry | ExceptionLogEntry | ScreenshotLogEntry

/**
 * Processed log entry with optional aggregation metadata
 */
export interface ProcessedLogEntry extends BaseLogEntry {
  readonly _aggregatedCount?: number
  readonly _firstSeen?: string
  readonly _lastSeen?: string
  readonly _previousOccurrences?: number
}
