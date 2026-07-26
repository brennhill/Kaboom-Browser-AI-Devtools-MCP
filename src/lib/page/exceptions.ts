/**
 * Purpose: Captures unhandled exceptions and promise rejections via window.onerror/unhandledrejection, enriching them with AI context before posting.
 * Docs: docs/features/feature/observe/index.md
 */

/**
 * @fileoverview Exception and unhandled rejection capture.
 * Monkey-patches window.onerror and listens for unhandledrejection events,
 * enriching errors with AI context before posting via bridge.
 */

import { postLog, type BridgePayload } from './bridge.js'
import { enrichErrorWithAiContext } from '../ai-context/ai-context.js'

interface ExceptionEntry extends Record<string, unknown> {
  level: 'error'
  type: 'exception'
  message: string
  source?: string
  filename?: string
  lineno?: number
  colno?: number
  stack?: string
}

// Exception capture state
let originalOnerror: OnErrorEventHandler | null = null
let unhandledrejectionHandler: ((event: PromiseRejectionEvent) => void) | null = null

/**
 * Install exception capture
 */
function enrichAndPost(entry: ExceptionEntry): void {
  void (async (): Promise<void> => {
    try {
      const enriched = await enrichErrorWithAiContext(entry)
      postLog(enriched as unknown as BridgePayload)
    } catch {
      postLog(entry as unknown as BridgePayload)
    }
  })().catch((err: Error) => {
    console.error('[KaBOOM!] Exception enrichment error:', err)
    try {
      postLog(entry as unknown as BridgePayload)
    } catch (postErr) {
      console.error('[KaBOOM!] Failed to log entry:', postErr)
    }
  })
}

function extractRejectionInfo(reason: unknown): { message: string; stack: string } {
  if (reason instanceof Error) return { message: reason.message, stack: reason.stack || '' }
  if (typeof reason === 'string') return { message: reason, stack: '' }
  return { message: String(reason), stack: '' }
}

export function installExceptionCapture(): void {
  originalOnerror = window.onerror

  window.onerror = function (
    message: string | Event,
    filename?: string,
    lineno?: number,
    colno?: number,
    error?: Error
  ): boolean | void {
    const messageStr = typeof message === 'string' ? message : (message as Event).type || 'Error'
    const entry: ExceptionEntry = {
      level: 'error',
      type: 'exception',
      message: messageStr,
      source: filename ? `${filename}:${lineno || 0}` : '',
      filename: filename || '',
      lineno: lineno || 0,
      colno: colno || 0,
      stack: error?.stack || ''
    }
    enrichAndPost(entry)
    if (originalOnerror) return originalOnerror(message, filename, lineno, colno, error)
    return false
  }

  unhandledrejectionHandler = function (event: PromiseRejectionEvent): void {
    const { message, stack } = extractRejectionInfo(event.reason)
    enrichAndPost({
      level: 'error',
      type: 'exception',
      message: `Unhandled Promise Rejection: ${message}`,
      stack
    })
  }

  window.addEventListener('unhandledrejection', unhandledrejectionHandler)
}

/**
 * Uninstall exception capture
 */
export function uninstallExceptionCapture(): void {
  if (originalOnerror !== null) {
    window.onerror = originalOnerror
    originalOnerror = null
  }

  if (unhandledrejectionHandler) {
    window.removeEventListener('unhandledrejection', unhandledrejectionHandler)
    unhandledrejectionHandler = null
  }
}
