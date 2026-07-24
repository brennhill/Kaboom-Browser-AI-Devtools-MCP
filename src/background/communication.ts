/**
 * Purpose: Facade that re-exports communication primitives (circuit breaker, batchers, server HTTP) and provides log formatting and screenshot capture.
 * Why: Single import point for communication functions, avoiding scattered imports across consumers.
 * Docs: docs/features/feature/backend-log-streaming/index.md
 */

/**
 * @fileoverview Communication - Facade that re-exports communication functions
 * from modular subcomponents: circuit-breaker.ts, batchers.ts, and server.ts
 */

// Re-export circuit breaker functions
export { createCircuitBreaker, type CircuitBreakerOptions, type CircuitBreaker } from './circuit-breaker.js'

// Re-export batcher functions and types
export {
  createBatcherWithCircuitBreaker,
  createLogBatcher,
  RATE_LIMIT_CONFIG,
  type Batcher,
  type BatcherWithCircuitBreaker,
  type BatcherConfig,
  type LogBatcherOptions
} from './batchers.js'

// Re-export server communication functions
export {
  sendLogsToServer,
  sendWSEventsToServer,
  sendNetworkBodiesToServer,
  sendEnhancedActionsToServer,
  sendPerformanceSnapshotsToServer,
  checkServerHealth,
  updateBadge,
  type ServerHealthResponse
} from './server.js'

// Import for logging formatting functions (still in this file for now)
import type { LogEntry } from '../types/index.js'
import { getRequestHeaders } from './server.js'
import { errorMessage } from '../lib/error-utils.js'
import { captureVisibleTabSafe } from './tab-state.js'

/**
 * Truncate a single argument if too large
 */
function truncateArg(arg: unknown, maxSize = 10240): unknown {
  if (arg === null || arg === undefined) return arg

  try {
    const serialized = JSON.stringify(arg)
    if (serialized.length > maxSize) {
      if (typeof arg === 'string') {
        return arg.slice(0, maxSize) + '... [truncated]'
      }
      return serialized.slice(0, maxSize) + '...[truncated]'
    }
    return arg
  } catch {
    if (typeof arg === 'object') {
      return '[Circular or unserializable object]'
    }
    return String(arg)
  }
}

/**
 * Format a log entry with timestamp and truncation
 */
export function formatLogEntry(entry: LogEntry): LogEntry {
  const formatted = { ...entry } as LogEntry & { ts?: string; args?: unknown[] }

  if (!formatted.ts) {
    ;(formatted as { ts: string }).ts = new Date().toISOString()
  }

  if ('args' in formatted && Array.isArray(formatted.args)) {
    formatted.args = formatted.args.map((arg: unknown) => truncateArg(arg))
  }

  return formatted as LogEntry
}

/**
 * Determine if a log should be captured based on level filter
 */
export function shouldCaptureLog(logLevel: string, filterLevel: string, logType?: string): boolean {
  if (logType === 'network' || logType === 'exception') {
    return true
  }

  const levels = ['debug', 'log', 'info', 'warn', 'error']
  const logIndex = levels.indexOf(logLevel)
  const filterIndex = levels.indexOf(filterLevel === 'all' ? 'debug' : filterLevel)

  return logIndex >= filterIndex
}

/**
 * Capture a screenshot of the visible tab area
 */
export async function captureScreenshot(
  tabId: number,
  serverUrl: string,
  relatedErrorId: string | null,
  errorType: string | null,
  canTakeScreenshotFn: (tabId: number) => { allowed: boolean; reason?: string; nextAllowedIn?: number | null },
  recordScreenshotFn: (tabId: number) => void,
  debugLogFn?: (category: string, message: string, data?: unknown) => void
): Promise<{
  success: boolean
  entry?: LogEntry
  error?: string
  nextAllowedIn?: number | null
}> {
  const rateCheck = canTakeScreenshotFn(tabId)
  if (!rateCheck.allowed) {
    if (debugLogFn) {
      debugLogFn('capture', `Screenshot rate limited: ${rateCheck.reason}`, {
        tabId,
        nextAllowedIn: rateCheck.nextAllowedIn
      })
    }
    return {
      success: false,
      error: `Rate limited: ${rateCheck.reason}`,
      nextAllowedIn: rateCheck.nextAllowedIn
    }
  }

  try {
    const tab = await chrome.tabs.get(tabId)
    const dataUrl = await captureVisibleTabSafe(tabId, tab.windowId, {
      format: 'jpeg',
      quality: 80
    })

    recordScreenshotFn(tabId)

    const response = await fetch(`${serverUrl}/screenshots`, {
      method: 'POST',
      headers: getRequestHeaders(),
      body: JSON.stringify({
        data_url: dataUrl,
        url: tab.url,
        correlation_id: relatedErrorId || ''
      })
    })

    if (!response.ok) {
      throw new Error(`Failed to upload screenshot: server returned HTTP ${response.status} ${response.statusText}`)
    }

    const result = (await response.json()) as { filename: string }

    const screenshotEntry: LogEntry = {
      ts: new Date().toISOString(),
      type: 'screenshot',
      level: 'info',
      url: tab.url,
      _enrichments: ['screenshot'],
      screenshotFile: result.filename,
      trigger: relatedErrorId ? 'error' : 'manual',
      ...(relatedErrorId ? { relatedErrorId } : {})
    } as LogEntry

    if (debugLogFn) {
      debugLogFn('capture', `Screenshot saved: ${result.filename}`, {
        trigger: relatedErrorId ? 'error' : 'manual',
        relatedErrorId
      })
    }

    return { success: true, entry: screenshotEntry }
  } catch (error) {
    if (debugLogFn) {
      debugLogFn('error', 'Screenshot capture failed', { error: errorMessage(error) })
    }
    return { success: false, error: errorMessage(error) }
  }
}
