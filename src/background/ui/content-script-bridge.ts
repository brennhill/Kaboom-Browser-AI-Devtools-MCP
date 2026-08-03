/**
 * Purpose: Owns background-to-content-script liveness, broadcast, overlay, and toast messaging.
 * Docs: docs/features/feature/browser-extension-enhancement/index.md
 */

import { scaleTimeout } from '../../lib/timeouts.js'

export async function pingContentScript(tabId: number, timeoutMs = scaleTimeout(500)): Promise<boolean> {
  try {
    const response = (await Promise.race([
      chrome.tabs.sendMessage(tabId, { type: 'kaboom_ping' }),
      new Promise<never>((_, reject) => {
        setTimeout(
          () => reject(new Error(`Content script ping timeout after ${timeoutMs}ms on tab ${tabId}`)),
          timeoutMs
        )
      })
    ])) as { status?: string }
    return response?.status === 'alive'
  } catch {
    // EXPECTED_ABSENCE: a missing content script during navigation is normal; logging would duplicate bounded readiness recovery.
    return false
  }
}

export async function forwardToAllContentScripts(
  message: { type: string; [key: string]: unknown },
  debugLogFn?: (category: string, message: string, data?: unknown) => void
): Promise<void> {
  if (typeof chrome === 'undefined' || !chrome.tabs) return

  const tabs = await chrome.tabs.query({})
  for (const tab of tabs) {
    if (!tab.id) continue
    chrome.tabs.sendMessage(tab.id, message).catch((err: Error) => {
      if (
        err.message?.includes('Receiving end does not exist') ||
        err.message?.includes('Could not establish connection')
      ) {
        return
      }
      debugLogFn?.('error', 'Unexpected error forwarding setting to tab', {
        tabId: tab.id,
        error: err.message
      })
    })
  }
}

export async function setKaboomOverlayVisibility(tabId: number, visible: boolean): Promise<void> {
  try {
    await chrome.scripting.executeScript({
      target: { tabId },
      func: (show: boolean) => {
        for (const id of ['kaboom-tracked-hover-launcher', 'kaboom-draw-toolbar']) {
          const element = document.getElementById(id)
          if (element) element.style.display = show ? 'flex' : 'none'
        }
      },
      args: [visible]
    })
  } catch {
    // EXPECTED_ABSENCE: UI recipients can normally disappear during navigation
    // or teardown; logging it would misleadingly report a normal lifecycle race as failure.
    // The tab may not have a content script.
  }
}

export function sendTabToast(
  tabId: number,
  text: string,
  detail = '',
  state: 'trying' | 'success' | 'warning' | 'error' | 'audio' = 'success',
  duration_ms = 3000
): void {
  chrome.tabs
    .sendMessage(tabId, {
      type: 'kaboom_action_toast' as const,
      text,
      detail,
      state,
      duration_ms
    })
    .catch(() => {
      // EXPECTED_ABSENCE: a missing content recipient is normal for restricted or
      // navigating tabs; logging it would misleadingly mark an optional toast failed.
    })
}
