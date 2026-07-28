/**
 * Purpose: Tab-state accessors, settings persistence, and content-script helpers.
 * Split from event-listeners.ts to keep files under 800 LOC.
 */

import { scaleTimeout } from '../../lib/timeouts.js'
import { syncTerminalPanelAvailability } from './side-panel-availability.js'
import { delay } from '../../lib/timeout-utils.js'
import { KABOOM_LOG_PREFIX } from '../../lib/brand.js'
import { StorageKey } from '../../lib/constants.js'
import { persist } from '../../lib/storage/io.js'
import { getLocal, getLocals, setLocal, setLocals } from '../../lib/storage/local.js'
import { TRACKED_TAB_STORAGE_KEYS } from '../../lib/tabs/tracked-tab-storage.js'

// Shared tracked-tab storage helpers (also used by the popup) — single source of truth.
export { setTrackedTab, clearTrackedTab, TRACKED_TAB_STORAGE_KEYS } from '../../lib/tabs/tracked-tab-storage.js'

// =============================================================================
// CONTENT SCRIPT HELPERS
// =============================================================================

/**
 * Ping content script to check if it's loaded
 */
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
    return false
  }
}

/**
 * Wait for tab to finish loading
 */
export async function waitForTabLoad(tabId: number, timeoutMs = scaleTimeout(5000)): Promise<boolean> {
  const startTime = Date.now()
  while (Date.now() - startTime < timeoutMs) {
    try {
      const tab = await chrome.tabs.get(tabId)
      if (tab.status === 'complete') return true
    } catch {
      return false
    }
    await delay(scaleTimeout(100))
  }
  return false
}

/**
 * Forward a message to all content scripts
 */
export async function forwardToAllContentScripts(
  message: { type: string; [key: string]: unknown },
  debugLogFn?: (category: string, message: string, data?: unknown) => void
): Promise<void> {
  if (typeof chrome === 'undefined' || !chrome.tabs) return

  const tabs = await chrome.tabs.query({})
  for (const tab of tabs) {
    if (tab.id) {
      chrome.tabs.sendMessage(tab.id, message).catch((err: Error) => {
        if (
          !err.message?.includes('Receiving end does not exist') &&
          !err.message?.includes('Could not establish connection')
        ) {
          if (debugLogFn) {
            debugLogFn('error', 'Unexpected error forwarding setting to tab', {
              tabId: tab.id,
              error: err.message
            })
          }
        }
      })
    }
  }
}

// =============================================================================
// SETTINGS LOADING
// =============================================================================

/** Settings returned by loadSavedSettings */
export interface SavedSettings {
  serverUrl?: string
  logLevel?: string
  screenshotOnError?: boolean
  sourceMapEnabled?: boolean
  debugMode?: boolean
}

/**
 * Load saved settings from chrome.storage.local
 */
export async function loadSavedSettings(): Promise<SavedSettings> {
  try {
    const result = (await getLocals([
      StorageKey.SERVER_URL,
      StorageKey.LOG_LEVEL,
      StorageKey.SCREENSHOT_ON_ERROR,
      StorageKey.SOURCE_MAP_ENABLED,
      StorageKey.DEBUG_MODE
    ])) as SavedSettings
    return result
  } catch {
    console.warn(`${KABOOM_LOG_PREFIX} Could not load saved settings - using defaults`)
    return {}
  }
}

/**
 * Load AI Web Pilot enabled state from storage
 */
export async function loadAiWebPilotState(logFn?: (message: string) => void): Promise<boolean> {
  const startTime = performance.now()
  // Fail closed: with no storage we cannot confirm the capability is enabled, and
  // `undefined !== false` would otherwise report it as ON. Matches loadDebugModeState.
  if (typeof chrome === 'undefined' || !chrome.storage) return false
  const aiEnabled = await getLocal(StorageKey.AI_WEB_PILOT_ENABLED)
  const wasLoaded = aiEnabled !== false
  const loadTime = performance.now() - startTime
  if (logFn) {
    logFn(`${KABOOM_LOG_PREFIX} AI Web Pilot loaded on startup: ${wasLoaded} (took ${loadTime.toFixed(1)}ms)`)
  }
  return wasLoaded
}

/**
 * Load debug mode state from storage
 */
export async function loadDebugModeState(): Promise<boolean> {
  const debugMode = await getLocal(StorageKey.DEBUG_MODE)
  return debugMode === true
}

/**
 * Save setting to chrome.storage.local
 */
export function saveSetting(key: string, value: unknown): void {
  persist(setLocal(key, value), `setting:${key}`)
}

// =============================================================================
// TRACKED TAB STATE
// =============================================================================

/** Tracked tab info type */
export interface TrackedTabInfo {
  trackedTabId: number | null
  trackedTabUrl: string | null
  trackedTabTitle: string | null
  tabStatus: 'loading' | 'complete' | null
  trackedTabActive: boolean | null
}

export interface TerminalWorkspaceTarget {
  hostTabId: number
  mainTabId: number
  tabGroupId: number
}

const TERMINAL_WORKSPACE_STORAGE_KEYS = [
  StorageKey.TERMINAL_WORKSPACE_GROUP_ID,
  StorageKey.TERMINAL_WORKSPACE_MAIN_TAB_ID,
  StorageKey.TRACKED_TAB_ID
]

function getUngroupedTabGroupId(): number {
  return chrome.tabGroups?.TAB_GROUP_ID_NONE ?? -1
}

function isGroupedTab(groupId: number | undefined): groupId is number {
  return typeof groupId === 'number' && Number.isFinite(groupId) && groupId !== getUngroupedTabGroupId()
}

async function safeGetTab(tabId: number | null | undefined): Promise<chrome.tabs.Tab | null> {
  if (typeof tabId !== 'number') return null
  try {
    return await chrome.tabs.get(tabId)
  } catch {
    return null
  }
}

async function focusTab(tab: chrome.tabs.Tab): Promise<void> {
  if (!tab.id) return
  try {
    await chrome.tabs.update(tab.id, { active: true })
  } catch {
    // Best effort.
  }
  if (typeof tab.windowId !== 'number' || !chrome.windows?.update) return
  try {
    await chrome.windows.update(tab.windowId, { focused: true })
  } catch {
    // Best effort.
  }
}

/**
 * Whether the optional `tabGroups` permission has been granted. Terminal
 * workspace grouping is purely cosmetic (an orange "KaBOOM!" tab group), so the
 * permission is `optional_permissions` in the manifest — a required permission
 * would force every existing user through a disable/re-approve prompt on update
 * for a label. Grouping is therefore opt-in: skipped, never prompted, until the
 * user grants tabGroups. When the permissions API is absent (test/edge runtimes)
 * fall back to whether the namespaces are exposed at all.
 */
async function hasTabGroupsPermission(): Promise<boolean> {
  if (typeof chrome.permissions?.contains === 'function') {
    try {
      return await chrome.permissions.contains({ permissions: ['tabGroups'] })
    } catch {
      return false
    }
  }
  return typeof chrome.tabs?.group === 'function' && typeof chrome.tabGroups?.update === 'function'
}

async function createTerminalWorkspaceGroup(tabId: number): Promise<number | null> {
  if (!(await hasTabGroupsPermission())) return null
  if (!chrome.tabs.group || !chrome.tabGroups?.update) return null
  try {
    const groupId = await chrome.tabs.group({ tabIds: [tabId] })
    const color = chrome.tabGroups.Color?.ORANGE
    const update = color
      ? { title: 'KaBOOM!', color, collapsed: false }
      : { title: 'KaBOOM!', collapsed: false }
    await chrome.tabGroups.update(groupId, update)
    return groupId
  } catch {
    return null
  }
}

/**
 * Get tracked tab information, including Chrome tab status.
 */
export async function getTrackedTabInfo(): Promise<TrackedTabInfo> {
  const result = (await getLocals(TRACKED_TAB_STORAGE_KEYS)) as {
    trackedTabId?: number
    trackedTabUrl?: string
    trackedTabTitle?: string
  }

  const tabId = result.trackedTabId || null
  let tabStatus: 'loading' | 'complete' | null = null
  let trackedTabActive: boolean | null = null

  // Query Chrome tab API for live tab status if we have a tracked tab
  if (tabId && typeof chrome !== 'undefined' && chrome.tabs) {
    try {
      const tab = await chrome.tabs.get(tabId)
      if (tab.status === 'loading' || tab.status === 'complete') {
        tabStatus = tab.status
      }
      trackedTabActive = !!tab.active
    } catch {
      // Tab may have been closed -- tabStatus stays null
    }
  }

  return {
    trackedTabId: tabId,
    trackedTabUrl: result.trackedTabUrl || null,
    trackedTabTitle: result.trackedTabTitle || null,
    tabStatus,
    trackedTabActive
  }
}

export async function resolveTerminalWorkspaceTarget(requestTabId?: number): Promise<TerminalWorkspaceTarget | null> {
  const result = (await getLocals(TERMINAL_WORKSPACE_STORAGE_KEYS)) as {
    trackedTabId?: number
    kaboom_terminal_workspace_group_id?: number
    kaboom_terminal_workspace_main_tab_id?: number
  }

  const trackedTabId = typeof result.trackedTabId === 'number' ? result.trackedTabId : null
  const storedMainTabId =
    typeof result.kaboom_terminal_workspace_main_tab_id === 'number'
      ? result.kaboom_terminal_workspace_main_tab_id
      : null

  const preferredMainTabId = trackedTabId ?? storedMainTabId ?? requestTabId ?? null
  const requestTab = await safeGetTab(requestTabId)
  let mainTab = await safeGetTab(preferredMainTabId)
  if (!mainTab && requestTab) {
    mainTab = requestTab
  }
  if (!mainTab) return null
  const mainTabId = mainTab?.id
  if (typeof mainTabId !== 'number') return null

  let tabGroupId = isGroupedTab(mainTab.groupId) ? mainTab.groupId : null
  if (tabGroupId === null) {
    tabGroupId = await createTerminalWorkspaceGroup(mainTabId)
    if (tabGroupId === null) {
      tabGroupId = mainTab.groupId ?? getUngroupedTabGroupId()
    } else {
      mainTab = (await safeGetTab(mainTabId)) ?? mainTab
    }
  }

  let hostTabId = mainTabId
  if (requestTab?.id && requestTab.groupId === tabGroupId) {
    hostTabId = requestTab.id
  } else {
    await focusTab(mainTab)
  }

  await setLocals({
    [StorageKey.TERMINAL_WORKSPACE_GROUP_ID]: tabGroupId,
    [StorageKey.TERMINAL_WORKSPACE_MAIN_TAB_ID]: mainTabId
  })

  return {
    hostTabId,
    mainTabId,
    tabGroupId
  }
}

/**
 * Get all extension config settings.
 */
export async function getAllConfigSettings(): Promise<Record<string, boolean | string | undefined>> {
  const result = (await getLocals([
    StorageKey.AI_WEB_PILOT_ENABLED,
    StorageKey.WEBSOCKET_CAPTURE_ENABLED,
    StorageKey.NETWORK_WATERFALL_ENABLED,
    StorageKey.PERFORMANCE_MARKS_ENABLED,
    StorageKey.ACTION_REPLAY_ENABLED,
    StorageKey.SCREENSHOT_ON_ERROR,
    StorageKey.SOURCE_MAP_ENABLED,
    StorageKey.NETWORK_BODY_CAPTURE_ENABLED
  ])) as Record<string, boolean | string | undefined>

  return result
}

// =============================================================================
// ACTIVE TAB LOOKUP
// =============================================================================

/**
 * Query for the currently active tab in the current window.
 * Returns null if no active tab or no tab id.
 */
export async function getActiveTab(): Promise<chrome.tabs.Tab | null> {
  const activeTabs = await chrome.tabs.query({ active: true, currentWindow: true })
  const tab = activeTabs[0]
  if (!tab?.id) {
    return null
  }
  return tab
}

// =============================================================================
// FOCUS-SAFE TAB CAPTURE
// =============================================================================

/**
 * Capture a screenshot of a tab without permanently stealing focus.
 * chrome.tabs.captureVisibleTab() requires the tab to be active. If the target
 * tab isn't currently active, we briefly activate it, capture, then restore
 * the previously active tab so the user's workflow isn't interrupted.
 */
export async function captureVisibleTabSafe(
  tabId: number,
  windowId: number,
  options: { format: 'jpeg' | 'png'; quality?: number }
): Promise<string> {
  const [activeTab] = await chrome.tabs.query({ active: true, windowId })
  const wasActive = activeTab?.id === tabId

  if (!wasActive) {
    await chrome.tabs.update(tabId, { active: true })
  }

  // Hide Kaboom UI overlay so it doesn't appear in screenshots.
  await setKaboomOverlayVisibility(tabId, false)

  try {
    return await chrome.tabs.captureVisibleTab(windowId, options)
  } finally {
    await setKaboomOverlayVisibility(tabId, true)
    if (!wasActive && activeTab?.id) {
      await chrome.tabs.update(activeTab.id, { active: true }).catch(() => {
        /* original tab may have been closed during capture */
      })
    }
  }
}

/**
 * Toggles visibility of all Kaboom UI overlays (hover launcher, draw mode)
 * in the target tab. Uses executeScript for speed — no message round-trip needed.
 */
export async function setKaboomOverlayVisibility(tabId: number, visible: boolean): Promise<void> {
  try {
    await chrome.scripting.executeScript({
      target: { tabId },
      func: (show: boolean) => {
        const ids = [
          'kaboom-tracked-hover-launcher',
          'kaboom-draw-toolbar'
        ]
        for (const id of ids) {
          const el = document.getElementById(id)
          if (!el) continue
          if (show) {
            // Restore display:flex (the launcher root is a flex container).
            // Setting '' would clear it to block, breaking the layout.
            el.style.display = 'flex'
          } else {
            el.style.display = 'none'
          }
        }
      },
      args: [visible]
    })
  } catch {
    // Tab may not have content script injected — safe to ignore
  }
}

// =============================================================================
// TAB TOAST
// =============================================================================

/**
 * Send a kaboom_action_toast message to a tab.
 * Silently ignores errors (content script may not be loaded).
 */
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
      /* content script may not be loaded */
    })
}
