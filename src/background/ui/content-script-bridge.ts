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

/**
 * Hide or restore every Kaboom overlay in a tab, so a screenshot captures the page and not
 * our own UI.
 *
 * Overlays are selected by the `data-kaboom-overlay` marker attribute, never by an id list.
 * The previous implementation hid a hardcoded ['kaboom-tracked-hover-launcher',
 * 'kaboom-draw-toolbar'] — and nothing in the codebase ever created `kaboom-draw-toolbar`
 * (the draw roots are kaboom-draw-overlay/-badge/-instruction), so every screenshot taken
 * during draw mode contained Kaboom's own overlay and the agent then read its own UI as page
 * content. A marker cannot be forgotten by a new overlay the way a list can.
 *
 * The original inline `display` is stashed and restored: the old code forced `flex` on
 * restore, which silently rewrote the layout of any overlay that was not a flex container.
 */
export async function setKaboomOverlayVisibility(tabId: number, visible: boolean): Promise<void> {
  try {
    await chrome.scripting.executeScript({
      target: { tabId },
      func: (show: boolean) => {
        const STASH = 'data-kaboom-display-before-capture'
        for (const node of document.querySelectorAll('[data-kaboom-overlay]')) {
          const element = node as HTMLElement
          if (show) {
            const previous = element.getAttribute(STASH)
            if (previous === null) continue
            element.style.display = previous
            element.removeAttribute(STASH)
          } else {
            if (element.hasAttribute(STASH)) continue
            element.setAttribute(STASH, element.style.display)
            element.style.display = 'none'
          }
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

/**
 * Drive the supervision overlay in a tab (kaboom-05ue.3).
 *
 * Fire-and-forget: a missing content script must never fail the action the user asked for.
 * The overlay is decoration around the work, not a precondition of it.
 */
export function sendAgentIndicator(
  tabId: number,
  phase: 'driving' | 'idle' | 'cursor' | 'heartbeat',
  detail: { action?: string; x?: number; y?: number } = {}
): void {
  chrome.tabs
    .sendMessage(tabId, { type: 'kaboom_agent_indicator' as const, phase, ...detail })
    .catch(() => {
      // EXPECTED_ABSENCE: restricted, navigating, and closed tabs normally have no content
      // script, so a missing recipient is expected rather than a fault. The overlay is
      // decoration around the action, not a precondition of it, so logging here would mark
      // a completed action failed because its indicator could not be drawn.
    })
}
