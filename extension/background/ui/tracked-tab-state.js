/**
 * Purpose: Owns live tracked-tab lookup, tab readiness, active-tab selection, and focus-safe capture.
 * Docs: docs/features/feature/tab-tracking-ux/index.md
 */
import { delay } from '../../lib/timeout-utils.js';
import { scaleTimeout } from '../../lib/timeouts.js';
import { getLocals } from '../../lib/storage/local.js';
import { TRACKED_TAB_STORAGE_KEYS } from '../../lib/tabs/tracked-tab-storage.js';
import { setKaboomOverlayVisibility } from './content-script-bridge.js';
export async function waitForTabLoad(tabId, timeoutMs = scaleTimeout(5000)) {
    const startTime = Date.now();
    while (Date.now() - startTime < timeoutMs) {
        try {
            if ((await chrome.tabs.get(tabId)).status === 'complete')
                return true;
        }
        catch {
            return false;
        }
        await delay(scaleTimeout(100));
    }
    return false;
}
export async function getTrackedTabInfo() {
    const result = (await getLocals(TRACKED_TAB_STORAGE_KEYS));
    const tabId = result.trackedTabId || null;
    let tabStatus = null;
    let trackedTabActive = null;
    if (tabId && typeof chrome !== 'undefined' && chrome.tabs) {
        try {
            const tab = await chrome.tabs.get(tabId);
            if (tab.status === 'loading' || tab.status === 'complete')
                tabStatus = tab.status;
            trackedTabActive = !!tab.active;
        }
        catch {
            // The tracked tab may have closed.
        }
    }
    return {
        trackedTabId: tabId,
        trackedTabUrl: result.trackedTabUrl || null,
        trackedTabTitle: result.trackedTabTitle || null,
        tabStatus,
        trackedTabActive
    };
}
export async function getActiveTab() {
    const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });
    return tab?.id ? tab : null;
}
export async function captureVisibleTabSafe(tabId, windowId, options) {
    const [activeTab] = await chrome.tabs.query({ active: true, windowId });
    const wasActive = activeTab?.id === tabId;
    if (!wasActive)
        await chrome.tabs.update(tabId, { active: true });
    await setKaboomOverlayVisibility(tabId, false);
    try {
        return await chrome.tabs.captureVisibleTab(windowId, options);
    }
    finally {
        await setKaboomOverlayVisibility(tabId, true);
        if (!wasActive && activeTab?.id) {
            await chrome.tabs.update(activeTab.id, { active: true }).catch(() => {
                // The original tab may have closed during capture.
            });
        }
    }
}
//# sourceMappingURL=tracked-tab-state.js.map