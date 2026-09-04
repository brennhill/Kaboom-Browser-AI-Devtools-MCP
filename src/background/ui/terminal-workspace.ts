/**
 * Purpose: Resolves, focuses, groups, and persists the terminal panel's tab workspace.
 * Docs: docs/features/feature/terminal/index.md
 */

import { StorageKey } from '../../lib/constants.js'
import { getLocals, setLocals } from '../../lib/storage/local.js'
import { reportStateRecovery, resolveStateRecovery } from '../runtime-state/state-recovery.js'
import { canGroupTabs } from '../tab-groups/driven-tab-group.js'

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
    // EXPECTED_ABSENCE: a saved workspace tab closing is normal; logging would mislabel the expected recreation path.
    return null
  }
}

async function focusTab(tab: chrome.tabs.Tab): Promise<void> {
  if (!tab.id) return
  try {
    await chrome.tabs.update(tab.id, { active: true })
  } catch {
    // EXPECTED_ABSENCE: UI recipients can normally disappear during navigation
    // or teardown; logging it would misleadingly report a normal lifecycle race as failure.
    // Best effort.
  }
  if (typeof tab.windowId !== 'number' || !chrome.windows?.update) return
  try {
    await chrome.windows.update(tab.windowId, { focused: true })
  } catch {
    // EXPECTED_ABSENCE: UI recipients can normally disappear during navigation
    // or teardown; logging it would misleadingly report a normal lifecycle race as failure.
    // Best effort.
  }
}

async function createTerminalWorkspaceGroup(tabId: number): Promise<number | null> {
  if (!(await canGroupTabs()) || !chrome.tabs.group || !chrome.tabGroups?.update) return null
  try {
    const groupId = await chrome.tabs.group({ tabIds: [tabId] })
    const color = chrome.tabGroups.Color?.ORANGE
    await chrome.tabGroups.update(
      groupId,
      color ? { title: 'KaBOOM!', color, collapsed: false } : { title: 'KaBOOM!', collapsed: false }
    )
    return groupId
  } catch {
    // EXPECTED_ABSENCE: grouping racing tab teardown is normal; logging would mislabel the usable ungrouped fallback.
    return null
  }
}

interface StoredTerminalWorkspaceTargets {
  trackedTabId: number | null
  mainTabId: number | null
}

async function readStoredWorkspaceTargets(): Promise<StoredTerminalWorkspaceTargets> {
  let result: {
    trackedTabId?: number
    kaboom_terminal_workspace_group_id?: number
    kaboom_terminal_workspace_main_tab_id?: number
  }
  try {
    const stored = await getLocals(TERMINAL_WORKSPACE_STORAGE_KEYS)
    const valid = Object.values(stored).every(
      (value) => value === undefined || (typeof value === 'number' && Number.isInteger(value))
    )
    if (!valid) {
      reportTerminalWorkspaceRecovery('Saved terminal workspace was malformed; the active or tracked tab is used.')
      result = {}
    } else {
      result = stored
    }
  } catch {
    reportTerminalWorkspaceRecovery('Saved terminal workspace could not be read; the active or tracked tab is used.')
    result = {}
  }
  return {
    trackedTabId: typeof result.trackedTabId === 'number' ? result.trackedTabId : null,
    mainTabId:
      typeof result.kaboom_terminal_workspace_main_tab_id === 'number'
        ? result.kaboom_terminal_workspace_main_tab_id
        : null
  }
}

async function resolveWorkspaceGroupId(
  mainTab: chrome.tabs.Tab,
  mainTabId: number
): Promise<{ tabGroupId: number; mainTab: chrome.tabs.Tab }> {
  let tabGroupId = isGroupedTab(mainTab.groupId) ? mainTab.groupId : null
  if (tabGroupId === null) {
    tabGroupId = await createTerminalWorkspaceGroup(mainTabId)
    if (tabGroupId === null) {
      tabGroupId = mainTab.groupId ?? getUngroupedTabGroupId()
    } else {
      mainTab = (await safeGetTab(mainTabId)) ?? mainTab
    }
  }
  return { tabGroupId, mainTab }
}

export async function resolveTerminalWorkspaceTarget(requestTabId?: number): Promise<TerminalWorkspaceTarget | null> {
  const stored = await readStoredWorkspaceTargets()
  const trackedTabId = stored.trackedTabId
  const storedMainTabId = stored.mainTabId
  const requestTab = await safeGetTab(requestTabId)
  let mainTab = await safeGetTab(trackedTabId ?? storedMainTabId ?? requestTabId ?? null)
  if (!mainTab && requestTab) mainTab = requestTab
  if (!mainTab?.id) return null
  const mainTabId = mainTab.id

  const workspace = await resolveWorkspaceGroupId(mainTab, mainTabId)
  mainTab = workspace.mainTab
  const tabGroupId = workspace.tabGroupId

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
  resolveStateRecovery('terminal_workspace_state')
  return { hostTabId, mainTabId, tabGroupId }
}

function reportTerminalWorkspaceRecovery(detail: string): void {
  reportStateRecovery({
    name: 'terminal_workspace_state',
    detail,
    fix: 'Open the terminal panel again to save a fresh workspace.'
  })
}
