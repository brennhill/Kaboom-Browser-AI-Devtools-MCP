/**
 * Purpose: Owns live tracked-tab lookup, tab readiness, active-tab selection, and focus-safe capture.
 * Docs: docs/features/feature/tab-tracking-ux/index.md
 */

import { delay } from '../../lib/timeout-utils.js'
import { scaleTimeout } from '../../lib/timeouts.js'
import { readTrackedTab } from '../../lib/tabs/tracked-tab-storage.js'
import { setKaboomOverlayVisibility } from './content-script-bridge.js'

export interface TrackedTabInfo {
  trackedTabId: number | null
  trackedTabUrl: string | null
  trackedTabTitle: string | null
  tabStatus: 'loading' | 'complete' | null
  trackedTabActive: boolean | null
}

export async function waitForTabLoad(tabId: number, timeoutMs = scaleTimeout(5000)): Promise<boolean> {
  const startTime = Date.now()
  while (Date.now() - startTime < timeoutMs) {
    try {
      if ((await chrome.tabs.get(tabId)).status === 'complete') return true
    } catch {
      // EXPECTED_ABSENCE: tracked-tab closure during polling is normal; logging would duplicate the resulting recovery state.
      return false
    }
    await delay(scaleTimeout(100))
  }
  return false
}

export async function getTrackedTabInfo(): Promise<TrackedTabInfo> {
  const result = await readTrackedTab()
  const tabId = result.id || null
  let tabStatus: 'loading' | 'complete' | null = null
  let trackedTabActive: boolean | null = null
  if (tabId && typeof chrome !== 'undefined' && chrome.tabs) {
    try {
      const tab = await chrome.tabs.get(tabId)
      if (tab.status === 'loading' || tab.status === 'complete') tabStatus = tab.status
      trackedTabActive = !!tab.active
    } catch {
      // EXPECTED_ABSENCE: UI recipients can normally disappear during navigation
      // or teardown; logging it would misleadingly report a normal lifecycle race as failure.
      // The tracked tab may have closed.
    }
  }
  return {
    trackedTabId: tabId,
    trackedTabUrl: result.url || null,
    trackedTabTitle: result.title || null,
    tabStatus,
    trackedTabActive
  }
}

export async function getActiveTab(): Promise<chrome.tabs.Tab | null> {
  const [tab] = await chrome.tabs.query({ active: true, currentWindow: true })
  return tab?.id ? tab : null
}

export async function captureVisibleTabSafe(
  tabId: number,
  windowId: number,
  options: { format: 'jpeg' | 'png'; quality?: number }
): Promise<string> {
  const [activeTab] = await chrome.tabs.query({ active: true, windowId })
  const wasActive = activeTab?.id === tabId
  if (!wasActive) await chrome.tabs.update(tabId, { active: true })
  await setKaboomOverlayVisibility(tabId, false)
  try {
    return await chrome.tabs.captureVisibleTab(windowId, options)
  } finally {
    await setKaboomOverlayVisibility(tabId, true)
    if (!wasActive && activeTab?.id) {
      await chrome.tabs.update(activeTab.id, { active: true }).catch(() => {
        // EXPECTED_ABSENCE: closure of the prior active tab is normal during
        // capture; logging it would misleadingly mark the successful capture failed.
      })
    }
  }
}
