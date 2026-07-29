/**
 * Purpose: Own terminal, chat, audit, and simple utility runtime messages.
 */
import { errorMessage } from '../../lib/error-utils.js'
import { resolveTerminalServerUrl } from '../../lib/terminal-server.js'
import { pushChatMessage } from '../push-handler.js'
import { openTerminalSidePanel } from '../ui/terminal-panel.js'
import type { MessageHandlerOwner, SendResponse } from './types.js'

export interface UtilityHandlerDependencies {
  getServerUrl: () => string
}
const QA_SCAN_PROMPT =
  'The user clicked "Audit". Run the KaBOOM! audit workflow for the tracked site. Use /kaboom/audit if available, otherwise /audit, and produce the six-lane Phase 1 report.'
const QA_SCAN_FETCH_TIMEOUT_MS = 3000

async function pushChat(message: string, pageUrl: string, tabId: number, respond: SendResponse): Promise<void> {
  try {
    const result = await pushChatMessage(message, pageUrl, tabId)
    respond(
      result
        ? { success: true, status: result.status, event_id: result.event_id }
        : { success: false, error: 'Failed to push message' }
    )
  } catch (error) {
    respond({ success: false, error: errorMessage(error) })
  }
}

async function requestAudit(pageUrl: string, serverUrl: string, respond: SendResponse): Promise<void> {
  const terminalUrl = await resolveTerminalServerUrl(serverUrl)
  for (const [path, body, method] of [
    ['/terminal/inject', { text: QA_SCAN_PROMPT }, 'terminal_inject'],
    ['/intent', { page_url: pageUrl, action: 'qa_scan' }, 'intent_stored']
  ] as const) {
    try {
      const controller = new AbortController()
      const timer = setTimeout(() => controller.abort(), QA_SCAN_FETCH_TIMEOUT_MS)
      const response = await fetch(`${terminalUrl}${path}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
        signal: controller.signal
      })
      clearTimeout(timer)
      if (response.ok) {
        respond({ success: true, method })
        return
      }
    } catch {
      /* try the next local endpoint */
    }
  }
  respond({ success: false, error: 'No terminal session and intent store unreachable' })
}

export function createUtilityMessageHandler(deps: UtilityHandlerDependencies): MessageHandlerOwner {
  return {
    feature: 'utility',
    handle(message, sender, sendResponse) {
      switch (message.type) {
        case 'get_tab_id':
          sendResponse({ tabId: sender.tab?.id })
          return true
        case 'open_terminal_panel':
          openTerminalSidePanel((message as { tab_id?: number }).tab_id ?? sender.tab?.id)
            .then(sendResponse)
            .catch((error) => sendResponse({ success: false, error: errorMessage(error) }))
          return true
        case 'kaboom_push_chat':
          void pushChat(message.message, message.page_url, sender.tab?.id ?? 0, sendResponse)
          return true
        case 'qa_scan_requested':
          void requestAudit(message.page_url || '', deps.getServerUrl(), sendResponse)
          return true
        default:
          return undefined
      }
    }
  }
}
