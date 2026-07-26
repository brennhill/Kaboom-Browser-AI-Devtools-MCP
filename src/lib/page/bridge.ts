/**
 * Purpose: Posts log events from the inject context to the content script via window.postMessage, enriching errors with context annotations and action replay.
 * Docs: docs/features/feature/observe/index.md
 */

/**
 * @fileoverview Message bridge for posting log events to the content script.
 * Enriches error-level messages with context annotations and user action replay.
 */

import { getContextAnnotations } from './context.js'
import { getActionBuffer } from './actions.js'

export interface BridgePayload {
  level?: string
  message?: string
  error?: string
  args?: unknown[]
  filename?: string
  lineno?: number
  [key: string]: unknown
}

/**
 * Post a log message to the content script
 */
// #lizard forgives
export function postLog(payload: BridgePayload): void {
  // Include context annotations and action replay for errors
  const context = getContextAnnotations()
  const actions = payload.level === 'error' ? getActionBuffer() : null

  // Build enrichments list to help AI understand what data is attached
  const enrichments: string[] = []
  if (context && payload.level === 'error') enrichments.push('context')
  if (actions && actions.length > 0) enrichments.push('userActions')

  // Extract fields we want from payload (exclude ts, message, source, url to avoid overwriting enrichments)
  const { level, type, args, error, stack, ...otherFields } = payload

  window.postMessage(
    {
      type: 'kaboom_log',
      payload: {
        // Enriched fields (these are the source of truth)
        ts: new Date().toISOString(),
        url: window.location.href,
        message:
          payload.message ||
          payload.error ||
          (payload.args?.[0] !== null && payload.args?.[0] !== undefined ? String(payload.args[0]) : ''),
        source: payload.filename ? `${payload.filename}:${payload.lineno || 0}` : '',
        // Core fields from payload
        level,
        ...(type ? { type } : {}),
        ...(args ? { args } : {}),
        ...(error ? { error } : {}),
        ...(stack ? { stack } : {}),
        // Optional enrichments
        ...(enrichments.length > 0 ? { _enrichments: enrichments } : {}),
        ...(context && payload.level === 'error' ? { _context: context } : {}),
        ...(actions && actions.length > 0 ? { _actions: actions } : {}),
        // Any other fields from payload (excluding the ones we destructured)
        ...otherFields
      }
    },
    window.location.origin
  )
}
