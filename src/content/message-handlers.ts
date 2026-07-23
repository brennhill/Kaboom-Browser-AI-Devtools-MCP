/**
 * Purpose: Handles incoming chrome.runtime messages from the background script -- pings, setting toggles, highlights, JS execution, state management, and draw mode.
 * Docs: docs/features/feature/interact-explore/index.md
 */

/**
 * @fileoverview Message Handlers Module
 * Handles messages from background script
 */

import type {
  ContentMessage,
  ContentPingResponse,
  WebSocketCaptureMode,
  HighlightResponse,
  WaterfallEntry,
  StateAction,
  BrowserStateSnapshot,
  A11yAuditResult
} from '../types/index.js'
import type { SettingMessage } from './types.js'
import {
  registerHighlightRequest,
  hasHighlightRequest,
  deleteHighlightRequest,
  registerExecuteRequest,
  hasExecuteRequest,
  deleteExecuteRequest,
  registerA11yRequest,
  hasA11yRequest,
  deleteA11yRequest,
  registerDomRequest,
  hasDomRequest,
  deleteDomRequest
} from './request-tracking.js'
import { createDeferredPromise, withTimeoutAndCleanup } from '../lib/timeout-utils.js'
import { isInjectScriptLoaded, getPageNonce, ensureInjectBridgeReady } from './script-injection.js'
import { ASYNC_COMMAND_TIMEOUT_MS, INJECT_FORWARDED_SETTINGS, SettingName } from '../lib/constants.js'
import { extractReadable as extractReadableContent } from './extractors/readable.js'
import { extractMarkdown as extractMarkdownContent } from './extractors/markdown.js'
import { extractPageSummary as extractPageSummaryContent } from './extractors/page-summary.js'
import { errorMessage } from '../lib/error-utils.js'

/** Auto-incrementing request ID — avoids Date.now() collisions for concurrent queries */
let nextRequestId = 1

/** Parse query params from string (JSON) or object form into a plain object */
function parseQueryParams(params: string | Record<string, unknown>): Record<string, unknown> {
  if (typeof params === 'string') {
    try {
      return JSON.parse(params)
    } catch {
      return {}
    }
  }
  return typeof params === 'object' ? params : {}
}

/** Send a nonce-authenticated message to inject.js (MAIN world) */
function postToInject(data: Record<string, unknown>): void {
  window.postMessage({ ...data, _nonce: getPageNonce() }, window.location.origin)
}

// Feature toggle message types forwarded from background to inject.js — imported from canonical constants.
export const TOGGLE_MESSAGES = INJECT_FORWARDED_SETTINGS

/**
 * Security: Validate sender is from the extension background script
 * Prevents content script from trusting messages from compromised page context
 */
export function isValidBackgroundSender(sender: chrome.runtime.MessageSender): boolean {
  // Messages from background should NOT have a tab (or have tab with chrome-extension:// url)
  // Messages from content scripts have tab.id
  // We only want messages from the background service worker
  return typeof sender.id === 'string' && sender.id === chrome.runtime.id
}

/**
 * Forward a highlight message from background to inject.js
 */
export function forwardHighlightMessage(message: {
  params: { selector: string; duration_ms?: number }
}): Promise<HighlightResponse> {
  return ensureInjectBridgeReady(1500).then((ready) => {
    if (!ready) {
      return {
        success: false,
        error: isInjectScriptLoaded() ? 'inject_not_responding' : 'inject_not_loaded'
      }
    }

    const requestId = registerHighlightRequest((result) => deferred.resolve(result))
    const deferred = createDeferredPromise<HighlightResponse>()

    // Post message to page context (inject.js)
    postToInject({
      type: 'kaboom_highlight_request',
      requestId,
      params: message.params
    })

    // Timeout fallback + cleanup stale entries after 30 seconds
    return withTimeoutAndCleanup(deferred.promise, 30000, {
      fallback: { success: false, error: 'timeout' },
      cleanup: () => {
        if (hasHighlightRequest(requestId)) {
          deleteHighlightRequest(requestId)
        }
      }
    })
  })
}

/**
 * Handle state capture/restore commands
 */
export async function handleStateCommand(
  params:
    | {
        action?: StateAction
        name?: string
        state?: BrowserStateSnapshot
        include_url?: boolean
      }
    | undefined
): Promise<{ error?: string; [key: string]: unknown }> {
  const { action, name, state, include_url } = params || {}

  // Create a promise to receive response from inject.js
  const messageId = `state_${Date.now()}_${Math.random().toString(36).slice(2)}`
  const deferred = createDeferredPromise<{ error?: string; [key: string]: unknown }>()

  // Set up listener for response from inject.js
  const responseHandler = (
    event: MessageEvent<{ type?: string; messageId?: string; result?: { error?: string; [key: string]: unknown } }>
  ) => {
    if (event.source !== window) return
    if (event.data?.type === 'kaboom_state_response' && event.data?.messageId === messageId) {
      window.removeEventListener('message', responseHandler)
      deferred.resolve(event.data.result || { error: 'No result from state command' })
    }
  }
  window.addEventListener('message', responseHandler)

  // Send command to inject.js (include state for restore action)
  postToInject({
    type: 'kaboom_state_command',
    messageId,
    action,
    name,
    state,
    include_url
  })

  // Timeout after 5 seconds with cleanup
  return withTimeoutAndCleanup(deferred.promise, 5000, {
    fallback: { error: 'State command timeout' },
    cleanup: () => window.removeEventListener('message', responseHandler)
  })
}

/**
 * Handle KABOOM_PING message
 */
export function handlePing(sendResponse: (response: ContentPingResponse) => void): boolean {
  sendResponse({ status: 'alive', timestamp: Date.now() })
  return true
}

/**
 * Handle toggle messages
 */
export function handleToggleMessage(
  message: ContentMessage & { enabled?: boolean; mode?: WebSocketCaptureMode; url?: string }
): void {
  if (!TOGGLE_MESSAGES.has(message.type)) return

  const payload: SettingMessage = { type: 'kaboom_setting', setting: message.type }
  if (message.type === SettingName.WEBSOCKET_CAPTURE_MODE) {
    payload.mode = message.mode
  } else if (message.type === SettingName.SERVER_URL) {
    payload.url = message.url
  } else {
    payload.enabled = message.enabled
  }
  // SECURITY: Use explicit targetOrigin (window.location.origin) not "*"
  window.postMessage({ ...payload, _nonce: getPageNonce() }, window.location.origin)
}

// ============================================
// Execute JS Handlers (MAIN world via inject script)
// Background handles world routing and fallback to chrome.scripting API.
// ============================================

type ExecuteJsResponse = { success: boolean; error?: string; message?: string; result?: unknown; stack?: string }

/**
 * Execute JS in the MAIN world via inject script, with safety timeout.
 */
function executeInMainWorld(
  params: { script?: string; timeout_ms?: number },
  sendResponse: (result: ExecuteJsResponse) => void
): void {
  const timeoutMs = params.timeout_ms || 5000
  const requestId = registerExecuteRequest(sendResponse)

  // Safety timeout: user's timeout + 2s buffer (NOT fixed 30s)
  // If inject script responds, its own timeout handles slow scripts.
  // This only fires if inject script never responds at all.
  const safetyTimeoutMs = timeoutMs + 2000
  setTimeout(() => {
    if (hasExecuteRequest(requestId)) {
      deleteExecuteRequest(requestId)
      sendResponse({
        success: false,
        error: 'inject_not_responding',
        message: `Inject script did not respond within ${safetyTimeoutMs}ms. The tab may not be tracked or the inject script failed to load.`
      })
    }
  }, safetyTimeoutMs)

  postToInject({
    type: 'kaboom_execute_js',
    requestId,
    script: params.script || '',
    timeoutMs
  })
}

/**
 * Handle kaboom_execute_js message.
 * Always executes in MAIN world via inject script.
 * Returns inject_not_loaded error if inject script isn't available,
 * so background can fallback to chrome.scripting API.
 */
export function handleExecuteJs(
  params: { script?: string; timeout_ms?: number },
  sendResponse: (result: ExecuteJsResponse) => void
): boolean {
  const injectReadyWaitMs = Math.max(750, Math.min(3000, (params.timeout_ms || 5000) + 500))
  void ensureInjectBridgeReady(injectReadyWaitMs).then((ready) => {
    if (!ready) {
      const fallbackError = isInjectScriptLoaded() ? 'inject_not_responding' : 'inject_not_loaded'
      sendResponse({
        success: false,
        error: fallbackError,
        message:
          fallbackError === 'inject_not_loaded'
            ? 'Inject script not loaded in page context. Tab may not be tracked.'
            : `Inject script did not respond within ${injectReadyWaitMs}ms. The tab may not be tracked or the inject script failed to load.`
      })
      return
    }

    executeInMainWorld(params, sendResponse)
  })
  return true
}

/**
 * Handle KABOOM_EXECUTE_QUERY message (async command path)
 */
export function handleExecuteQuery(
  params: string | Record<string, unknown>,
  sendResponse: (result: ExecuteJsResponse) => void
): boolean {
  let parsedParams: { script?: string; timeout_ms?: number } = {}
  if (typeof params === 'string') {
    try {
      parsedParams = JSON.parse(params)
    } catch {
      parsedParams = {}
    }
  } else if (typeof params === 'object') {
    parsedParams = params as { script?: string; timeout_ms?: number }
  }

  return handleExecuteJs(parsedParams, sendResponse)
}

/**
 * Handle A11Y_QUERY message
 */
export function handleA11yQuery(
  params: string | Record<string, unknown>,
  sendResponse: (result: A11yAuditResult | { error: string }) => void
): boolean {
  const parsedParams = parseQueryParams(params)
  const requestId = registerA11yRequest(sendResponse)

  // Timeout fallback: respond with error and cleanup the real pending map
  setTimeout(() => {
    if (hasA11yRequest(requestId)) {
      deleteA11yRequest(requestId)
      sendResponse({ error: 'Accessibility audit timeout' })
    }
  }, ASYNC_COMMAND_TIMEOUT_MS)

  // Forward to inject.js via postMessage
  postToInject({
    type: 'kaboom_a11y_query',
    requestId,
    params: parsedParams
  })

  return true
}

/**
 * Handle DOM_QUERY message
 */
export function handleDomQuery(
  params: string | Record<string, unknown>,
  sendResponse: (result: { error?: string; matches?: unknown[] }) => void
): boolean {
  const parsedParams = parseQueryParams(params)
  const requestId = registerDomRequest(sendResponse)

  // Timeout fallback: respond with error and cleanup the real pending map
  setTimeout(() => {
    if (hasDomRequest(requestId)) {
      deleteDomRequest(requestId)
      sendResponse({ error: 'DOM query timeout' })
    }
  }, ASYNC_COMMAND_TIMEOUT_MS)

  // Forward to inject.js via postMessage
  postToInject({
    type: 'kaboom_dom_query',
    requestId,
    params: parsedParams
  })

  return true
}

/**
 * Handle GET_NETWORK_WATERFALL message
 */
export function handleGetNetworkWaterfall(sendResponse: (result: { entries: WaterfallEntry[] }) => void): boolean {
  const requestId = nextRequestId++
  const deferred = createDeferredPromise<{ entries: WaterfallEntry[] }>()

  // Set up a one-time listener for the response — match requestId to prevent cross-wiring
  const responseHandler = (
    event: MessageEvent<{ type?: string; requestId?: number; entries?: WaterfallEntry[]; _nonce?: string }>
  ) => {
    if (event.source !== window) return
    // Validate nonce on response messages (spoofing prevention).
    // Accept responses with no nonce for backwards compat during migration.
    const nonce = event.data?._nonce
    if (nonce && nonce !== getPageNonce()) return
    if (event.data?.type === 'kaboom_waterfall_response' && event.data?.requestId === requestId) {
      window.removeEventListener('message', responseHandler)
      deferred.resolve({ entries: event.data.entries || [] })
    }
  }

  window.addEventListener('message', responseHandler)

  // Post message to page context
  postToInject({
    type: 'kaboom_get_waterfall',
    requestId
  })

  // Timeout fallback: respond with empty array after 5 seconds
  withTimeoutAndCleanup(deferred.promise, 5000, {
    fallback: { entries: [] },
    cleanup: () => window.removeEventListener('message', responseHandler)
  }).then(
    (result) => {
      sendResponse(result)
    },
    () => {
      sendResponse({ entries: [] })
    }
  )

  return true
}

/**
 * Generic inject-query forwarder: parse params, post to inject, wait for response with timeout.
 * Consolidates the identical pattern used by computed_styles, form_discovery, and link_health.
 */
function forwardInjectQuery(
  queryType: string,
  responseType: string,
  label: string,
  params: string | Record<string, unknown>,
  sendResponse: (result: unknown) => void
): boolean {
  const parsedParams = parseQueryParams(params)
  const requestId = nextRequestId++
  const deferred = createDeferredPromise<unknown>()

  const responseHandler = (
    event: MessageEvent<{ type?: string; requestId?: number; result?: unknown; _nonce?: string }>
  ) => {
    if (event.source !== window) return
    // Validate nonce on response messages (spoofing prevention).
    // Accept responses with no nonce for backwards compat during migration.
    const nonce = event.data?._nonce
    if (nonce && nonce !== getPageNonce()) return
    if (event.data?.type === responseType && event.data?.requestId === requestId) {
      window.removeEventListener('message', responseHandler)
      deferred.resolve(event.data.result || { error: `No result from ${label}` })
    }
  }

  window.addEventListener('message', responseHandler)
  postToInject({ type: queryType, requestId, params: parsedParams })

  withTimeoutAndCleanup(deferred.promise, ASYNC_COMMAND_TIMEOUT_MS, {
    fallback: { error: `${label} timeout` },
    cleanup: () => window.removeEventListener('message', responseHandler)
  }).then(
    (result) => sendResponse(result),
    () => sendResponse({ error: `${label} failed` })
  )

  return true
}

export function handleComputedStylesQuery(
  params: string | Record<string, unknown>,
  sendResponse: (result: unknown) => void
): boolean {
  return forwardInjectQuery(
    'kaboom_computed_styles_query',
    'kaboom_computed_styles_response',
    'Computed styles query',
    params,
    sendResponse
  )
}

export function handleFormDiscoveryQuery(
  params: string | Record<string, unknown>,
  sendResponse: (result: unknown) => void
): boolean {
  return forwardInjectQuery(
    'kaboom_form_discovery_query',
    'kaboom_form_discovery_response',
    'Form discovery',
    params,
    sendResponse
  )
}

export function handleFormStateQuery(
  params: string | Record<string, unknown>,
  sendResponse: (result: unknown) => void
): boolean {
  return forwardInjectQuery(
    'kaboom_form_state_query',
    'kaboom_form_state_response',
    'Form state',
    params,
    sendResponse
  )
}

export function handleDataTableQuery(
  params: string | Record<string, unknown>,
  sendResponse: (result: unknown) => void
): boolean {
  return forwardInjectQuery(
    'kaboom_data_table_query',
    'kaboom_data_table_response',
    'Data table extraction',
    params,
    sendResponse
  )
}

export function handleLinkHealthQuery(
  params: string | Record<string, unknown>,
  sendResponse: (result: unknown) => void
): boolean {
  return forwardInjectQuery(
    'kaboom_link_health_query',
    'kaboom_link_health_response',
    'Link health check',
    params,
    sendResponse
  )
}

// ============================================
// Content-Script-Native Extractors (ISOLATED world, CSP-safe)
// Issue #257: These run directly in the content script — no inject bridge needed.
// ============================================

/**
 * Handle GET_READABLE message — extract readable content directly in ISOLATED world.
 */
export function handleGetReadable(sendResponse: (result: unknown) => void): boolean {
  try {
    sendResponse(extractReadableContent())
  } catch (err) {
    sendResponse({ error: 'get_readable_failed', message: errorMessage(err, 'Readable extraction failed') })
  }
  // Synchronous — sendResponse called inline, no async channel needed.
  return false
}

/**
 * Handle GET_MARKDOWN message — extract markdown content directly in ISOLATED world.
 */
export function handleGetMarkdown(sendResponse: (result: unknown) => void): boolean {
  try {
    sendResponse(extractMarkdownContent())
  } catch (err) {
    sendResponse({ error: 'get_markdown_failed', message: errorMessage(err, 'Markdown extraction failed') })
  }
  // Synchronous — sendResponse called inline, no async channel needed.
  return false
}

/**
 * Handle PAGE_SUMMARY message — extract page summary directly in ISOLATED world.
 */
export function handlePageSummary(sendResponse: (result: unknown) => void): boolean {
  try {
    sendResponse(extractPageSummaryContent())
  } catch (err) {
    sendResponse({ error: 'page_summary_failed', message: errorMessage(err, 'Page summary extraction failed') })
  }
  // Synchronous — sendResponse called inline, no async channel needed.
  return false
}
