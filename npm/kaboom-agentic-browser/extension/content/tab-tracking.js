/**
 * Purpose: Tracks whether this content script's tab is the currently tracked tab via chrome.storage change listeners.
 * Docs: docs/features/feature/tab-tracking-ux/index.md
 */
/**
 * @fileoverview Tab Tracking Module
 * Manages tracking status for the current tab
 */
import { StorageKey } from '../lib/constants.js';
import { getLocal, onStorageChanged } from '../lib/storage-utils.js';
// Whether this content script's tab is the currently tracked tab
let isTrackedTab = false;
// The tab ID of this content script's tab
let currentTabId = null;
/**
 * Update tracking status by checking storage and current tab ID.
 * Called on script load, storage changes, and tab activation.
 */
export async function updateTrackingStatus() {
    try {
        const trackedTabId = (await getLocal(StorageKey.TRACKED_TAB_ID));
        // Request tab ID from background script (content scripts can't access chrome.tabs)
        const response = (await chrome.runtime.sendMessage({ type: 'get_tab_id' }));
        currentTabId = response?.tabId ?? null;
        isTrackedTab = currentTabId !== null && currentTabId !== undefined && currentTabId === trackedTabId;
    }
    catch {
        // Graceful degradation: if we can't check, assume not tracked
        isTrackedTab = false;
    }
}
/**
 * Get the current tracking status
 */
export function getIsTrackedTab() {
    return isTrackedTab;
}
/**
 * Get the current tab ID
 */
export function getCurrentTabId() {
    return currentTabId;
}
/**
 * Initialize tab tracking (call once on script load).
 * Returns a promise that resolves when initial tracking status is known.
 * The onChange callback fires after each status update (initial + storage changes).
 */
export function initTabTracking(onChange) {
    const ready = updateTrackingStatus().then(() => {
        onChange?.(isTrackedTab);
    });
    onStorageChanged(async (changes) => {
        if (changes[StorageKey.TRACKED_TAB_ID]) {
            await updateTrackingStatus();
            onChange?.(isTrackedTab);
        }
    });
    return ready;
}
//# sourceMappingURL=tab-tracking.js.map