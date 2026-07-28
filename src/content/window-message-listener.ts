/**
 * Purpose: Listens for window.postMessage events from inject.js and resolves pending request promises or forwards telemetry to the background.
 * Docs: docs/features/feature/observe/index.md
 */

/**
 * @fileoverview Window Message Listener Module
 * Handles window.postMessage events from inject.js
 */

import type { HighlightResponse, ExecuteJsResult, A11yAuditResult, DomQueryResult } from '../types/index.js'
import type { PageMessageEventData, BackgroundMessageFromContent } from './types.js'
import {
  resolveHighlightRequest,
  resolveExecuteRequest,
  resolveA11yRequest,
  resolveDomRequest
} from './request-tracking.js'
import { MESSAGE_MAP, safeSendMessage } from './message-forwarding.js'
import { getIsTrackedTab, getCurrentTabId } from './tab-tracking.js'
import { getPageNonce } from './script-injection.js'

/**
 * Initialize consolidated window message listener
 * Handles all messages from inject.js
 */
type ResponseResolver = (requestId: number | string, result: unknown) => void

const RESPONSE_HANDLERS: Record<string, ResponseResolver> = {
  kaboom_highlight_response: (id, result) => resolveHighlightRequest(id as number, result as HighlightResponse),
  kaboom_execute_js_result: (id, result) => resolveExecuteRequest(id as number, result as ExecuteJsResult),
  kaboom_a11y_query_response: (id, result) => resolveA11yRequest(id as number, result as A11yAuditResult),
  kaboom_dom_query_response: (id, result) => resolveDomRequest(id as number, result as DomQueryResult)
}

export function initWindowMessageListener(): void {
  window.addEventListener('message', (event: MessageEvent<PageMessageEventData>) => {
    if (event.source !== window || event.origin !== window.location.origin) return

    const { type: messageType, requestId, result, payload } = event.data || {}

    const responseHandler = messageType ? RESPONSE_HANDLERS[messageType] : undefined
    if (responseHandler) {
      const nonce = (event.data as { _nonce?: string })?._nonce
      if (nonce !== getPageNonce()) return
      if (requestId !== undefined) responseHandler(requestId, result)
      return
    }

    // Tab isolation filter: only forward captured data from the tracked tab.
    // Response messages (highlight, execute JS, a11y) are NOT filtered because
    // they are responses to explicit commands from the background script.
    if (!getIsTrackedTab()) return

    if (messageType && messageType in MESSAGE_MAP && payload && typeof payload === 'object') {
      const mappedType = MESSAGE_MAP[messageType]
      if (mappedType) {
        safeSendMessage({
          type: mappedType,
          payload,
          tabId: getCurrentTabId()
        } as BackgroundMessageFromContent)
      }
    }
  })
}
