/**
 * Purpose: Captures and uploads visible-tab screenshots for background error enrichment.
 * Docs: docs/features/feature/backend-log-streaming/index.md
 */

import type { LogEntry } from '../../types/index.js'
import { errorMessage } from '../../lib/error-utils.js'
import { captureVisibleTabSafe } from '../ui/tab-state.js'
import { getRequestHeaders } from './server.js'

interface ScreenshotRateCheck {
  allowed: boolean
  reason?: string
  nextAllowedIn?: number | null
}

interface ScreenshotResult {
  success: boolean
  entry?: LogEntry
  error?: string
  nextAllowedIn?: number | null
}

export async function captureScreenshot(
  tabId: number,
  serverUrl: string,
  relatedErrorId: string | null,
  canTakeScreenshot: (tabId: number) => ScreenshotRateCheck,
  recordScreenshot: (tabId: number) => void,
  debugLog?: (category: string, message: string, data?: unknown) => void
): Promise<ScreenshotResult> {
  const rateCheck = canTakeScreenshot(tabId)
  if (!rateCheck.allowed) {
    debugLog?.('capture', `Screenshot rate limited: ${rateCheck.reason}`, {
      tabId,
      nextAllowedIn: rateCheck.nextAllowedIn
    })
    return {
      success: false,
      error: `Rate limited: ${rateCheck.reason}`,
      nextAllowedIn: rateCheck.nextAllowedIn
    }
  }

  try {
    const tab = await chrome.tabs.get(tabId)
    const dataUrl = await captureVisibleTabSafe(tabId, tab.windowId, { format: 'jpeg', quality: 80 })
    recordScreenshot(tabId)

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
    const entry = {
      ts: new Date().toISOString(),
      type: 'screenshot',
      level: 'info',
      url: tab.url,
      _enrichments: ['screenshot'],
      screenshotFile: result.filename,
      trigger: relatedErrorId ? 'error' : 'manual',
      ...(relatedErrorId ? { relatedErrorId } : {})
    } as LogEntry

    debugLog?.('capture', `Screenshot saved: ${result.filename}`, {
      trigger: relatedErrorId ? 'error' : 'manual',
      relatedErrorId
    })
    return { success: true, entry }
  } catch (error) {
    debugLog?.('error', 'Screenshot capture failed', { error: errorMessage(error) })
    return { success: false, error: errorMessage(error) }
  }
}
