/**
 * Purpose: Own screenshot and draw-mode runtime message handling.
 */
import type { DrawModeCompletedMessage, LogEntry } from '../../types/index.js'
import { errorMessage } from '../../lib/error-utils.js'
import { postDaemonJSON } from '../../lib/daemon-http.js'
import { setKaboomOverlayVisibility } from '../ui/tab-state.js'
import { trackUIFeature } from '../ui/ui-usage-tracker.js'
import type { MessageHandlerOwner, SendResponse } from './types.js'

export interface CaptureHandlerDependencies {
  getServerUrl: () => string
  captureScreenshot: (
    tabId: number,
    relatedErrorId: string | null
  ) => Promise<{
    success: boolean
    entry?: LogEntry
    error?: string
  }>
  addLog: (entry: LogEntry) => void
  debugLog: (category: string, message: string, data?: unknown) => void
}

function captureActiveTab(sendResponse: SendResponse, deps: CaptureHandlerDependencies): void {
  if (typeof chrome === 'undefined' || !chrome.tabs) {
    sendResponse({ success: false, error: 'Chrome tabs API not available' })
    return
  }
  chrome.tabs.query({ active: true, currentWindow: true }, async (tabs) => {
    const tabId = tabs[0]?.id
    if (!tabId) {
      sendResponse({ success: false, error: 'No active tab' })
      return
    }
    try {
      const result = await deps.captureScreenshot(tabId, null)
      if (result.success && result.entry) deps.addLog(result.entry)
      sendResponse(result)
    } catch (error) {
      sendResponse({ success: false, error: errorMessage(error) })
    }
  })
}

async function captureDrawOverlay(tabId: number | undefined, sendResponse: SendResponse): Promise<void> {
  if (!tabId) {
    sendResponse({ dataUrl: '' })
    return
  }
  try {
    const tab = await chrome.tabs.get(tabId)
    await setKaboomOverlayVisibility(tabId, false)
    const dataUrl = await chrome.tabs.captureVisibleTab(tab.windowId, { format: 'png' })
    await setKaboomOverlayVisibility(tabId, true)
    sendResponse({ dataUrl })
  } catch {
    await setKaboomOverlayVisibility(tabId, true).catch(() => {})
    sendResponse({ dataUrl: '' })
  }
}

async function deliverDrawResult(
  message: DrawModeCompletedMessage,
  tabId: number | undefined,
  deps: CaptureHandlerDependencies
): Promise<void> {
  if (!tabId) return
  const body: Record<string, unknown> = {
    screenshot_data_url: message.screenshot_data_url || '',
    annotations: message.annotations || [],
    element_details: message.elementDetails || {},
    page_url: message.page_url || '',
    tab_id: tabId,
    correlation_id: message.correlation_id || ''
  }
  if (message.annot_session_name) body.annot_session_name = message.annot_session_name
  try {
    const response = await postDaemonJSON(`${deps.getServerUrl()}/draw-mode/complete`, body)
    if (!response.ok) deps.debugLog('error', `Draw mode POST failed: ${response.status}`)
  } catch (error) {
    deps.debugLog('error', `Draw mode completion error: ${errorMessage(error)}. Server may be unreachable.`)
  }
}

export function createCaptureMessageHandler(deps: CaptureHandlerDependencies): MessageHandlerOwner {
  return {
    feature: 'capture',
    handle(message, sender, sendResponse) {
      switch (message.type) {
        case 'capture_screenshot':
          trackUIFeature('screenshot')
          captureActiveTab(sendResponse, deps)
          return true
        case 'kaboom_capture_screenshot':
          void captureDrawOverlay(sender.tab?.id, sendResponse)
          return true
        case 'draw_mode_completed':
          void deliverDrawResult(message, sender.tab?.id, deps)
          return false
        case 'track_ui_feature':
          trackUIFeature(message.feature)
          return false
        default:
          return undefined
      }
    }
  }
}
