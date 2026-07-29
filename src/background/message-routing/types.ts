/**
 * Purpose: Shared typed contract for feature-owned background message handlers.
 */
import type { BackgroundMessage, ChromeMessageSender } from '../../types/index.js'

export type SendResponse = (response?: unknown) => void
export interface MessageHandlerOwner {
  readonly feature: string
  handle(message: BackgroundMessage, sender: ChromeMessageSender, sendResponse: SendResponse): boolean | undefined
}
