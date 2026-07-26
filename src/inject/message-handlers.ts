/**
 * Purpose: Dispatches window.postMessage commands from the content script to specialized inject-context handlers (settings, state, JS execution, DOM/a11y queries).
 * Docs: docs/features/feature/interact-explore/index.md
 */

// message-handlers.ts — Message dispatch from content script to inject-context handlers.

/**
 * @fileoverview Message Handlers - Dispatches messages from content script to
 * specialized modules for settings, state management, JavaScript execution,
 * and DOM/accessibility queries.
 */

import type { BrowserStateSnapshot } from '../types/index.js'

import { executeDOMQuery, runAxeAuditWithTimeout, type DOMQueryParams } from '../lib/analysis/dom-queries.js'
import { checkLinkHealth } from '../lib/analysis/link-health.js'
import { queryComputedStyles } from './computed-styles.js'
import { discoverForms } from './form-discovery.js'
import { extractDataTables } from './data-table.js'
import { getNetworkWaterfall } from '../lib/net/network.js'

import { executeJavaScript } from './execute-js.js'
import { errorMessage } from '../lib/error-utils.js'
import {
  isValidSettingPayload,
  handleSetting,
  handleStateCommand,
  type SettingMessageData,
  type StateCommandMessageData
} from './settings.js'

// Re-export for barrel (src/inject/index.ts)
export { executeJavaScript, safeSerializeForExecute } from './execute-js.js'

/** Read the page nonce set by the content script on the inject script element */
let pageNonce = ''
if (typeof document !== 'undefined' && typeof document.querySelector === 'function') {
  const nonceEl = document.querySelector('script[data-kaboom-nonce]')
  if (nonceEl) {
    pageNonce = nonceEl.getAttribute('data-kaboom-nonce') || ''
  }
}

/** Send a nonce-authenticated response back to the content script */
function postResponse(data: Record<string, unknown>): void {
  window.postMessage({ ...data, _nonce: pageNonce }, window.location.origin)
}

/**
 * Execute JS request message from content script
 */
interface ExecuteJsRequestMessageData {
  type: 'kaboom_execute_js'
  requestId: number | string
  script: string
  timeoutMs?: number
}

/**
 * A11y query request message from content script
 */
interface A11yQueryRequestMessageData {
  type: 'kaboom_a11y_query'
  requestId: number | string
  params?: Record<string, unknown>
}

/**
 * DOM query request message from content script
 */
interface DomQueryRequestMessageData {
  type: 'kaboom_dom_query'
  requestId: number | string
  params?: Record<string, unknown>
}

/**
 * Highlight request message from content script
 */
interface HighlightRequestMessageData {
  type: 'kaboom_highlight_request'
  requestId: number | string
  params?: {
    selector: string
    duration_ms?: number
  }
}

/**
 * Get waterfall request message from content script
 */
interface GetWaterfallRequestMessageData {
  type: 'kaboom_get_waterfall'
  requestId: number | string
}

/**
 * Link health query request message from content script
 */
interface LinkHealthQueryRequestMessageData {
  type: 'kaboom_link_health_query'
  requestId: number | string
  params?: Record<string, unknown>
}

/**
 * Computed styles query request message from content script
 */
interface ComputedStylesQueryRequestMessageData {
  type: 'kaboom_computed_styles_query'
  requestId: number | string
  params?: Record<string, unknown>
}

/**
 * Form discovery query request message from content script
 */
interface FormDiscoveryQueryRequestMessageData {
  type: 'kaboom_form_discovery_query'
  requestId: number | string
  params?: Record<string, unknown>
}

/**
 * Form state query request message from content script
 */
interface FormStateQueryRequestMessageData {
  type: 'kaboom_form_state_query'
  requestId: number | string
  params?: Record<string, unknown>
}

/**
 * Data table query request message from content script
 */
interface DataTableQueryRequestMessageData {
  type: 'kaboom_data_table_query'
  requestId: number | string
  params?: Record<string, unknown>
}

/**
 * Bridge readiness ping from content script to inject context
 */
interface BridgePingMessageData {
  type: 'kaboom_inject_bridge_ping'
  requestId: number | string
}

/**
 * Union of all page message data types
 */
type PageMessageData =
  | SettingMessageData
  | StateCommandMessageData
  | ExecuteJsRequestMessageData
  | A11yQueryRequestMessageData
  | DomQueryRequestMessageData
  | HighlightRequestMessageData
  | GetWaterfallRequestMessageData
  | LinkHealthQueryRequestMessageData
  | ComputedStylesQueryRequestMessageData
  | FormDiscoveryQueryRequestMessageData
  | FormStateQueryRequestMessageData
  | DataTableQueryRequestMessageData
  | BridgePingMessageData

/**
 * Handle link health check request from content script
 */
async function handleLinkHealthQuery(data: LinkHealthQueryRequestMessageData): Promise<unknown> {
  try {
    const params = data.params || {}
    const result = await checkLinkHealth(params)
    return result
  } catch (err) {
    return {
      error: 'link_health_error',
      message: errorMessage(err, 'Failed to check link health')
    }
  }
}

/**
 * Install message listener for handling content script messages
 */
function handleLinkHealthMessage(data: LinkHealthQueryRequestMessageData): void {
  handleLinkHealthQuery(data)
    .then((result) => {
      postResponse({ type: 'kaboom_link_health_response', requestId: data.requestId, result })
    })
    .catch((err: Error) => {
      postResponse({
        type: 'kaboom_link_health_response',
        requestId: data.requestId,
        result: { error: 'link_health_error', message: err.message || 'Failed to check link health' }
      })
    })
}

export function installMessageListener(
  captureStateFn: () => BrowserStateSnapshot,
  restoreStateFn: (state: BrowserStateSnapshot, includeUrl: boolean) => unknown
): void {
  if (typeof window === 'undefined') return

  const messageHandlers: Record<string, (data: PageMessageData) => void> = {
    kaboom_setting: (data) => {
      const settingData = data as SettingMessageData
      if (isValidSettingPayload(settingData)) handleSetting(settingData)
    },
    kaboom_state_command: (data) =>
      handleStateCommand(data as StateCommandMessageData, captureStateFn, restoreStateFn),
    kaboom_execute_js: (data) => handleExecuteJs(data as ExecuteJsRequestMessageData),
    kaboom_a11y_query: (data) => handleA11yQuery(data as A11yQueryRequestMessageData),
    kaboom_dom_query: (data) => handleDomQuery(data as DomQueryRequestMessageData),
    kaboom_get_waterfall: (data) => handleGetWaterfall(data as GetWaterfallRequestMessageData),
    kaboom_link_health_query: (data) => handleLinkHealthMessage(data as LinkHealthQueryRequestMessageData),
    kaboom_computed_styles_query: (data) =>
      handleComputedStylesMessage(data as ComputedStylesQueryRequestMessageData),
    kaboom_form_discovery_query: (data) => handleFormDiscoveryMessage(data as FormDiscoveryQueryRequestMessageData),
    kaboom_form_state_query: (data) => handleFormStateMessage(data as FormStateQueryRequestMessageData),
    kaboom_data_table_query: (data) => handleDataTableMessage(data as DataTableQueryRequestMessageData),
    kaboom_inject_bridge_ping: (data) => handleBridgePingMessage(data as BridgePingMessageData)
  }

  window.addEventListener('message', (event: MessageEvent<PageMessageData>) => {
    if (event.source !== window || event.origin !== window.location.origin) return
    if (pageNonce && (event.data as unknown as { _nonce?: string })?._nonce !== pageNonce) return

    const msgType = event.data?.type
    if (!msgType) return

    const handler = messageHandlers[msgType] // nosemgrep: unsafe-dynamic-method
    if (handler) handler(event.data)
  })
}

function handleBridgePingMessage(data: BridgePingMessageData): void {
  postResponse({
    type: 'kaboom_inject_bridge_pong',
    requestId: data.requestId
  })
}

function handleComputedStylesMessage(data: ComputedStylesQueryRequestMessageData): void {
  try {
    const params = (data.params || {}) as { selector?: string; properties?: string[] }
    const result = queryComputedStyles({
      selector: params.selector || '*',
      properties: params.properties
    })
    postResponse({
      type: 'kaboom_computed_styles_response',
      requestId: data.requestId,
      result: { elements: result, count: result.length }
    })
  } catch (err) {
    postResponse({
      type: 'kaboom_computed_styles_response',
      requestId: data.requestId,
      result: { error: 'computed_styles_error', message: errorMessage(err, 'Failed to query computed styles') }
    })
  }
}

function handleFormDiscoveryMessage(data: FormDiscoveryQueryRequestMessageData): void {
  try {
    const params = (data.params || {}) as { selector?: string; mode?: string }
    const result = discoverForms({
      selector: params.selector,
      mode: params.mode === 'validate' ? 'validate' : 'discover'
    })
    postResponse({
      type: 'kaboom_form_discovery_response',
      requestId: data.requestId,
      result: { forms: result, count: result.length }
    })
  } catch (err) {
    postResponse({
      type: 'kaboom_form_discovery_response',
      requestId: data.requestId,
      result: { error: 'form_discovery_error', message: errorMessage(err, 'Failed to discover forms') }
    })
  }
}

function handleFormStateMessage(data: FormStateQueryRequestMessageData): void {
  try {
    const params = (data.params || {}) as { selector?: string }
    const forms = discoverForms({
      selector: params.selector,
      mode: 'discover'
    })
    postResponse({
      type: 'kaboom_form_state_response',
      requestId: data.requestId,
      result: { forms, count: forms.length }
    })
  } catch (err) {
    postResponse({
      type: 'kaboom_form_state_response',
      requestId: data.requestId,
      result: { error: 'form_state_error', message: errorMessage(err, 'Failed to extract form state') }
    })
  }
}

function handleDataTableMessage(data: DataTableQueryRequestMessageData): void {
  try {
    const params = (data.params || {}) as { selector?: string; max_rows?: number; max_cols?: number }
    const result = extractDataTables({
      selector: params.selector,
      max_rows: params.max_rows,
      max_cols: params.max_cols
    })
    postResponse({
      type: 'kaboom_data_table_response',
      requestId: data.requestId,
      result
    })
  } catch (err) {
    postResponse({
      type: 'kaboom_data_table_response',
      requestId: data.requestId,
      result: { error: 'data_table_error', message: errorMessage(err, 'Failed to extract table data') }
    })
  }
}

function handleExecuteJs(data: ExecuteJsRequestMessageData): void {
  const { requestId, script, timeoutMs } = data

  // Validate parameters
  if (typeof script !== 'string') {
    console.warn('[KaBOOM!] Script must be a string')
    postResponse({
      type: 'kaboom_execute_js_result',
      requestId,
      result: { success: false, error: 'invalid_script', message: 'Script must be a string' }
    })
    return
  }

  if (typeof requestId !== 'number' && typeof requestId !== 'string') {
    console.warn('[KaBOOM!] Invalid requestId type')
    return
  }

  executeJavaScript(script, timeoutMs)
    .then((result) => {
      postResponse({
        type: 'kaboom_execute_js_result',
        requestId,
        result
      })
    })
    .catch((err: Error) => {
      console.error('[KaBOOM!] Failed to execute JS:', err)
      postResponse({
        type: 'kaboom_execute_js_result',
        requestId,
        result: { success: false, error: 'execution_failed', message: err.message }
      })
    })
}

function handleA11yQuery(data: A11yQueryRequestMessageData): void {
  const { requestId, params } = data

  if (typeof runAxeAuditWithTimeout !== 'function') {
    postResponse({
      type: 'kaboom_a11y_query_response',
      requestId,
      result: {
        error: 'runAxeAuditWithTimeout not available - try reloading the extension'
      }
    })
    return
  }

  try {
    runAxeAuditWithTimeout(params || {})
      .then((result) => {
        postResponse({
          type: 'kaboom_a11y_query_response',
          requestId,
          result
        })
      })
      .catch((err: Error) => {
        console.error('[KaBOOM!] Accessibility audit error:', err)
        postResponse({
          type: 'kaboom_a11y_query_response',
          requestId,
          result: { error: err.message || 'Accessibility audit failed' }
        })
      })
  } catch (err) {
    console.error('[KaBOOM!] Failed to run accessibility audit:', err)
    postResponse({
      type: 'kaboom_a11y_query_response',
      requestId,
      result: { error: errorMessage(err, 'Failed to run accessibility audit') }
    })
  }
}

function handleDomQuery(data: DomQueryRequestMessageData): void {
  const { requestId, params } = data

  if (typeof executeDOMQuery !== 'function') {
    postResponse({
      type: 'kaboom_dom_query_response',
      requestId,
      result: {
        error: 'executeDOMQuery not available - try reloading the extension'
      }
    })
    return
  }

  try {
    executeDOMQuery((params || {}) as unknown as DOMQueryParams)
      .then((result) => {
        postResponse({
          type: 'kaboom_dom_query_response',
          requestId,
          result
        })
      })
      .catch((err: Error) => {
        console.error('[KaBOOM!] DOM query error:', err)
        postResponse({
          type: 'kaboom_dom_query_response',
          requestId,
          result: { error: err.message || 'DOM query failed' }
        })
      })
  } catch (err) {
    console.error('[KaBOOM!] Failed to run DOM query:', err)
    postResponse({
      type: 'kaboom_dom_query_response',
      requestId,
      result: { error: errorMessage(err, 'Failed to run DOM query') }
    })
  }
}

function handleGetWaterfall(data: GetWaterfallRequestMessageData): void {
  const { requestId } = data

  try {
    const entries = getNetworkWaterfall({})

    postResponse({
      type: 'kaboom_waterfall_response',
      requestId,
      entries: entries || [],
      page_url: window.location.href
    })
  } catch (err) {
    console.error('[KaBOOM!] Failed to get network waterfall:', err)
    postResponse({
      type: 'kaboom_waterfall_response',
      requestId,
      entries: []
    })
  }
}
