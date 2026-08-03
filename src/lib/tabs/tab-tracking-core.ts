/**
 * Purpose: Context-independent core for starting/stopping tab tracking, shared by
 *          the popup and the context menu (repo rule 19).
 * Why: The context-menu "Control Tab" path used to persist tracking directly,
 *      bypassing the internal-page and cloaked-domain guards the popup enforced —
 *      tracking a cloaked domain is a privacy leak (rule 7). This is the single
 *      gate both entry points go through, so the guards can never diverge again.
 * Docs: docs/features/feature/tab-tracking-ux/index.md
 */

import { isInternalUrl } from './internal-url.js'
import { isDomainCloaked } from './cloaked-domains.js'
import { setTrackedTab, clearTrackedTab } from './tracked-tab-storage.js'

/** Result of an attempt to start tracking. Only 'tracked' persisted anything. */
export type TrackTabOutcome = 'tracked' | 'internal_page' | 'cloaked'

function hostnameOf(url: string | undefined): string {
  try {
    return url ? new URL(url).hostname : ''
  } catch {
    // EXPECTED_ABSENCE: malformed tab URLs are expected inputs; logging would mislabel the guarded uncloaked fallback.
    return ''
  }
}

/**
 * Tell a tab's content script the tracking state changed. Best-effort: the tab
 * may be closed or lack a content script — the authoritative state is in storage.
 */
function notifyTrackingState(tabId: number, isTracked: boolean): void {
  chrome.tabs
    .sendMessage(tabId, { type: 'tracking_state_changed', state: { isTracked, aiPilotEnabled: false } })
    .catch(() => {
      // EXPECTED_ABSENCE: it is normal for a closed or reinjecting tracked tab to lack a
      // recipient; logging it would misleadingly contradict authoritative storage.
    })
}

/**
 * Make sure the tracked tab has a live content script: ping it, reload to inject
 * when it is missing, otherwise notify it of the new state without a reload.
 */
function ensureContentScript(tabId: number): void {
  chrome.tabs.sendMessage(tabId, { type: 'kaboom_ping' }, (response?: { status?: string }) => {
    if (chrome.runtime.lastError || !response?.status) {
      chrome.tabs.reload(tabId)
    } else {
      notifyTrackingState(tabId, true)
    }
  })
}

/**
 * Start tracking `tab`, enforcing the internal-page and cloaked-domain guards.
 * Returns the outcome so UI callers can render the right state; any outcome other
 * than 'tracked' means nothing was persisted.
 */
export async function trackTab(tab: Pick<chrome.tabs.Tab, 'id' | 'url' | 'title'>): Promise<TrackTabOutcome> {
  if (isInternalUrl(tab.url)) return 'internal_page'
  if (await isDomainCloaked(hostnameOf(tab.url))) return 'cloaked'
  await setTrackedTab(tab)
  if (tab.id) ensureContentScript(tab.id)
  return 'tracked'
}

/**
 * Stop tracking. `onStopped` lets each context stop screen recording its own way
 * — the popup messages the background, the background stops the handler directly,
 * because a runtime message does not self-deliver inside the service worker.
 */
export async function untrackTab(prevTabId: number | undefined, onStopped?: () => void | Promise<void>): Promise<void> {
  await clearTrackedTab()
  if (onStopped) await onStopped()
  if (typeof prevTabId === 'number') notifyTrackingState(prevTabId, false)
}
