/**
 * Purpose: Resolves, focuses, groups, and persists the terminal panel's tab workspace.
 * Docs: docs/features/feature/terminal/index.md
 */
import { StorageKey } from '../../lib/constants.js';
import { getLocals, setLocals } from '../../lib/storage/local.js';
import { reportStateRecovery, resolveStateRecovery } from '../runtime-state/state-recovery.js';
const TERMINAL_WORKSPACE_STORAGE_KEYS = [
    StorageKey.TERMINAL_WORKSPACE_GROUP_ID,
    StorageKey.TERMINAL_WORKSPACE_MAIN_TAB_ID,
    StorageKey.TRACKED_TAB_ID
];
function getUngroupedTabGroupId() {
    return chrome.tabGroups?.TAB_GROUP_ID_NONE ?? -1;
}
function isGroupedTab(groupId) {
    return typeof groupId === 'number' && Number.isFinite(groupId) && groupId !== getUngroupedTabGroupId();
}
async function safeGetTab(tabId) {
    if (typeof tabId !== 'number')
        return null;
    try {
        return await chrome.tabs.get(tabId);
    }
    catch {
        // EXPECTED_ABSENCE: a saved workspace tab closing is normal; logging would mislabel the expected recreation path.
        return null;
    }
}
async function focusTab(tab) {
    if (!tab.id)
        return;
    try {
        await chrome.tabs.update(tab.id, { active: true });
    }
    catch {
        // EXPECTED_ABSENCE: UI recipients can normally disappear during navigation
        // or teardown; logging it would misleadingly report a normal lifecycle race as failure.
        // Best effort.
    }
    if (typeof tab.windowId !== 'number' || !chrome.windows?.update)
        return;
    try {
        await chrome.windows.update(tab.windowId, { focused: true });
    }
    catch {
        // EXPECTED_ABSENCE: UI recipients can normally disappear during navigation
        // or teardown; logging it would misleadingly report a normal lifecycle race as failure.
        // Best effort.
    }
}
async function hasTabGroupsPermission() {
    if (typeof chrome.permissions?.contains === 'function') {
        try {
            return await chrome.permissions.contains({ permissions: ['tabGroups'] });
        }
        catch {
            // EXPECTED_ABSENCE: missing optional tabGroups support is normal; logging would mislabel an ungrouped workspace as failure.
            return false;
        }
    }
    return typeof chrome.tabs?.group === 'function' && typeof chrome.tabGroups?.update === 'function';
}
async function createTerminalWorkspaceGroup(tabId) {
    if (!(await hasTabGroupsPermission()) || !chrome.tabs.group || !chrome.tabGroups?.update)
        return null;
    try {
        const groupId = await chrome.tabs.group({ tabIds: [tabId] });
        const color = chrome.tabGroups.Color?.ORANGE;
        await chrome.tabGroups.update(groupId, color ? { title: 'KaBOOM!', color, collapsed: false } : { title: 'KaBOOM!', collapsed: false });
        return groupId;
    }
    catch {
        // EXPECTED_ABSENCE: grouping racing tab teardown is normal; logging would mislabel the usable ungrouped fallback.
        return null;
    }
}
async function readStoredWorkspaceTargets() {
    let result;
    try {
        const stored = await getLocals(TERMINAL_WORKSPACE_STORAGE_KEYS);
        const valid = Object.values(stored).every((value) => value === undefined || (typeof value === 'number' && Number.isInteger(value)));
        if (!valid) {
            reportTerminalWorkspaceRecovery('Saved terminal workspace was malformed; the active or tracked tab is used.');
            result = {};
        }
        else {
            result = stored;
        }
    }
    catch {
        reportTerminalWorkspaceRecovery('Saved terminal workspace could not be read; the active or tracked tab is used.');
        result = {};
    }
    return {
        trackedTabId: typeof result.trackedTabId === 'number' ? result.trackedTabId : null,
        mainTabId: typeof result.kaboom_terminal_workspace_main_tab_id === 'number'
            ? result.kaboom_terminal_workspace_main_tab_id
            : null
    };
}
async function resolveWorkspaceGroupId(mainTab, mainTabId) {
    let tabGroupId = isGroupedTab(mainTab.groupId) ? mainTab.groupId : null;
    if (tabGroupId === null) {
        tabGroupId = await createTerminalWorkspaceGroup(mainTabId);
        if (tabGroupId === null) {
            tabGroupId = mainTab.groupId ?? getUngroupedTabGroupId();
        }
        else {
            mainTab = (await safeGetTab(mainTabId)) ?? mainTab;
        }
    }
    return { tabGroupId, mainTab };
}
export async function resolveTerminalWorkspaceTarget(requestTabId) {
    const stored = await readStoredWorkspaceTargets();
    const trackedTabId = stored.trackedTabId;
    const storedMainTabId = stored.mainTabId;
    const requestTab = await safeGetTab(requestTabId);
    let mainTab = await safeGetTab(trackedTabId ?? storedMainTabId ?? requestTabId ?? null);
    if (!mainTab && requestTab)
        mainTab = requestTab;
    if (!mainTab?.id)
        return null;
    const mainTabId = mainTab.id;
    const workspace = await resolveWorkspaceGroupId(mainTab, mainTabId);
    mainTab = workspace.mainTab;
    const tabGroupId = workspace.tabGroupId;
    let hostTabId = mainTabId;
    if (requestTab?.id && requestTab.groupId === tabGroupId) {
        hostTabId = requestTab.id;
    }
    else {
        await focusTab(mainTab);
    }
    await setLocals({
        [StorageKey.TERMINAL_WORKSPACE_GROUP_ID]: tabGroupId,
        [StorageKey.TERMINAL_WORKSPACE_MAIN_TAB_ID]: mainTabId
    });
    resolveStateRecovery('terminal_workspace_state');
    return { hostTabId, mainTabId, tabGroupId };
}
function reportTerminalWorkspaceRecovery(detail) {
    reportStateRecovery({
        name: 'terminal_workspace_state',
        detail,
        fix: 'Open the terminal panel again to save a fresh workspace.'
    });
}
//# sourceMappingURL=terminal-workspace.js.map