/**
 * Purpose: Resolves, focuses, groups, and persists the terminal panel's tab workspace.
 * Docs: docs/features/feature/terminal/index.md
 */

import { StorageKey } from '../../lib/constants.js'
import { getLocals, setLocals } from '../../lib/storage/local.js'

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
  if (!(await hasTabGroupsPermission()) || !chrome.tabs.group || !chrome.tabGroups?.update) return null
  try {
    const groupId = await chrome.tabs.group({ tabIds: [tabId] })
    const color = chrome.tabGroups.Color?.ORANGE
    await chrome.tabGroups.update(
      groupId,
      color ? { title: 'KaBOOM!', color, collapsed: false } : { title: 'KaBOOM!', collapsed: false }
    )
    return groupId
  } catch {
    return null
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
  const requestTab = await safeGetTab(requestTabId)
  let mainTab = await safeGetTab(trackedTabId ?? storedMainTabId ?? requestTabId ?? null)
  if (!mainTab && requestTab) mainTab = requestTab
  if (!mainTab?.id) return null
  const mainTabId = mainTab.id

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
  return { hostTabId, mainTabId, tabGroupId }
}
