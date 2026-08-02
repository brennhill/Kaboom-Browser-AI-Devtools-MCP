/**
 * Purpose: Validate trusted runtime-message senders and dispatch to feature-owned handlers.
 * Why: The router owns security and ordering only; change-coupled behavior and dependencies
 * live with each feature handler.
 */
import type { BackgroundMessage } from '../types/runtime-messages.js'
import type { ChromeMessageSender } from '../types/runtime/chrome.js'
import type { MessageHandlerOwner, SendResponse } from './message-routing/types.js'
import { errorMessage } from '../lib/error-utils.js'

export interface MessageRouterDependencies {
  handlers: readonly MessageHandlerOwner[]
  debugLog: (category: string, message: string, data?: unknown) => void
}

function isValidMessageSender(sender: ChromeMessageSender & { id?: string }): boolean {
  if (sender.tab?.id !== undefined && sender.tab?.url) return true
  return typeof chrome !== 'undefined' && Boolean(chrome.runtime) && sender.id === chrome.runtime.id
}

function dispatch(
  message: BackgroundMessage,
  sender: ChromeMessageSender,
  sendResponse: SendResponse,
  handlers: readonly MessageHandlerOwner[]
): boolean {
  for (const owner of handlers) {
    const result = owner.handle(message, sender, sendResponse)
    if (result !== undefined) return result
  }
  // Recording listeners own their separate message types.
  return false
}

export function installMessageListener(deps: MessageRouterDependencies): void {
  if (typeof chrome === 'undefined' || !chrome.runtime) return
  chrome.runtime.onMessage.addListener(
    (message: BackgroundMessage, sender: chrome.runtime.MessageSender, sendResponse: SendResponse): boolean => {
      if (!isValidMessageSender(sender as ChromeMessageSender & { id?: string })) {
        deps.debugLog('error', 'Rejected message from untrusted sender', {
          senderId: sender.id,
          senderUrl: sender.url
        })
        return false
      }
      try {
        return dispatch(message, sender as ChromeMessageSender, sendResponse, deps.handlers)
      } catch (error) {
        const correlationId = `runtime_message_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`
        deps.debugLog('error', 'Runtime message handler failed', {
          correlation_id: correlationId,
          message_type: message.type,
          error: errorMessage(error)
        })
        sendResponse({ success: false, error: 'message_handler_failed' })
        return false
      }
    }
  )
}
