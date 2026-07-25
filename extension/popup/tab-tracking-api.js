/**
 * Purpose: Chrome API and storage operations for tab tracking — track/untrack lifecycle, tab switching.
 * Why: Separates browser API side-effects from DOM UI state rendering in tab-tracking.
 * Docs: docs/features/feature/tab-tracking-ux/index.md
 */
import { KABOOM_LOG_PREFIX } from '../lib/brand.js';
import { StorageKey } from '../lib/constants.js';
import { getLocal, persist } from '../lib/storage-utils.js';
import { clearTrackedTab } from '../lib/tracked-tab-storage.js';
import { trackTab, untrackTab } from '../lib/tab-tracking-core.js';
import { requestAudit } from '../lib/request-audit.js';
/**
 * Handle launching the tracked-site audit workflow from popup controls.
 */
export async function handleAuditClick(pageUrl, tabId) {
    await requestAudit(pageUrl, tabId);
}
/**
 * Handle stop tracking from the compact tracking bar stop button.
 */
export async function handleStopTracking(showIdleState) {
    const prevTabId = await getLocal(StorageKey.TRACKED_TAB_ID);
    if (!prevTabId)
        return;
    // Shared core clears storage and notifies the content script; the popup cannot
    // reach the recording state, so it asks the background to stop any recording.
    await untrackTab(prevTabId, () => {
        chrome.runtime.sendMessage({ type: 'screen_recording_stop' }, () => {
            if (chrome.runtime.lastError) {
                /* no recording active — expected */
            }
        });
    });
    const btn = document.getElementById('track-page-btn');
    if (btn)
        showIdleState(btn);
    console.log(KABOOM_LOG_PREFIX, 'Stopped tracking via bar stop button');
}
/**
 * Handle clicking on the tracked URL.
 * Switches to the tracked tab.
 */
export async function handleUrlClick(tabId) {
    if (!tabId)
        return;
    try {
        // Switch to the tracked tab and bring its window to focus
        await chrome.tabs.update(tabId, { active: true });
        const tab = await chrome.tabs.get(tabId);
        if (tab.windowId) {
            await chrome.windows.update(tab.windowId, { focused: true });
        }
        console.log(KABOOM_LOG_PREFIX, 'Switched to tracked tab:', tabId);
    }
    catch (err) {
        console.error(KABOOM_LOG_PREFIX, 'Failed to switch to tracked tab:', err);
        // Tab might have been closed - clear tracking (best-effort, logs on failure)
        persist(clearTrackedTab(), 'tracked-tab-clear');
    }
}
/**
 * Handle Track This Tab button click.
 * Toggles tracking on/off for the current tab.
 * Blocks tracking on internal Chrome pages.
 */
export async function handleTrackPageClick(showInternalPageState, showCloakedState, showTrackingState, showIdleState) {
    const btn = document.getElementById('track-page-btn');
    // Check if we're currently tracking
    const trackedTabId = await getLocal(StorageKey.TRACKED_TAB_ID);
    if (trackedTabId) {
        // Untrack — delegate to the shared stop handler
        await handleStopTracking(showIdleState);
        return;
    }
    // Track current tab. All guards (internal page, cloaked domain) and the
    // content-script injection live in the shared core so the popup and the
    // context menu enforce them identically (repo rule 19).
    const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });
    if (!tab)
        return;
    const outcome = await trackTab(tab);
    if (outcome === 'internal_page') {
        if (btn)
            showInternalPageState(btn);
        return;
    }
    if (outcome === 'cloaked') {
        if (btn)
            showCloakedState(btn);
        return;
    }
    if (btn)
        showTrackingState(btn, tab.url, tab.id);
    console.log(KABOOM_LOG_PREFIX, 'Now tracking tab:', tab.id, tab.url);
}
//# sourceMappingURL=tab-tracking-api.js.map