/**
 * Purpose: Chrome API and storage operations for tab tracking — track/untrack lifecycle, tab switching.
 * Why: Separates browser API side-effects from DOM UI state rendering in tab-tracking.
 * Docs: docs/features/feature/tab-tracking-ux/index.md
 */
import { isInternalUrl } from './ui-utils.js';
import { KABOOM_LOG_PREFIX } from '../lib/brand.js';
import { StorageKey } from '../lib/constants.js';
import { getLocal } from '../lib/storage-utils.js';
import { setTrackedTab, clearTrackedTab } from '../lib/tracked-tab-storage.js';
import { isDomainCloaked } from '../lib/cloaked-domains.js';
import { requestAudit } from '../lib/request-audit.js';
/**
 * Handle launching the tracked-site audit workflow from popup controls.
 */
export async function handleAuditClick(pageUrl) {
    await requestAudit(pageUrl);
}
/**
 * Handle stop tracking from the compact tracking bar stop button.
 */
export async function handleStopTracking(showIdleState) {
    const prevTabId = await getLocal(StorageKey.TRACKED_TAB_ID);
    if (!prevTabId)
        return;
    await clearTrackedTab();
    const btn = document.getElementById('track-page-btn');
    if (btn)
        showIdleState(btn);
    // Stop recording if active
    chrome.runtime.sendMessage({ type: 'screen_recording_stop' }, () => {
        if (chrome.runtime.lastError) {
            /* no recording active — expected */
        }
    });
    // Notify content script so favicon restores without reload
    chrome.tabs
        .sendMessage(prevTabId, {
        type: 'tracking_state_changed',
        state: { isTracked: false, aiPilotEnabled: false }
    })
        .catch(() => {
        /* tab may be closed */
    });
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
        // Tab might have been closed - clear tracking (fire-and-forget, never throws)
        clearTrackedTab().catch(() => { });
    }
}
/**
 * Handle Track This Tab button click.
 * Toggles tracking on/off for the current tab.
 * Blocks tracking on internal Chrome pages.
 */
// #lizard forgives
export async function handleTrackPageClick(showInternalPageState, showCloakedState, showTrackingState, showIdleState) {
    const btn = document.getElementById('track-page-btn');
    // Check if we're currently tracking
    const trackedTabId = await getLocal(StorageKey.TRACKED_TAB_ID);
    if (trackedTabId) {
        // Untrack — delegate to the shared stop handler
        await handleStopTracking(showIdleState);
        return;
    }
    // Track current tab
    const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });
    if (!tab)
        return;
    // Block tracking on internal Chrome pages
    if (isInternalUrl(tab.url)) {
        if (btn)
            showInternalPageState(btn);
        return;
    }
    // Block tracking on cloaked domains
    let hostname = '';
    try {
        hostname = tab.url ? new URL(tab.url).hostname : '';
    }
    catch { /* malformed URL */ }
    if (await isDomainCloaked(hostname)) {
        if (btn)
            showCloakedState(btn);
        return;
    }
    await setTrackedTab(tab);
    if (btn)
        showTrackingState(btn, tab.url, tab.id);
    console.log(KABOOM_LOG_PREFIX, 'Now tracking tab:', tab.id, tab.url);
    // Only reload if content script is not already injected
    if (tab.id) {
        const tabId = tab.id;
        chrome.tabs.sendMessage(tabId, { type: 'kaboom_ping' }, (response) => {
            if (chrome.runtime.lastError || !response?.status) {
                // Content script not loaded — reload to inject it
                console.log(KABOOM_LOG_PREFIX, 'Content script not found, reloading tab', tabId);
                chrome.tabs.reload(tabId);
            }
            else {
                // Content script already running — notify it of tracking change
                console.log(KABOOM_LOG_PREFIX, 'Content script already loaded, skipping reload');
                chrome.tabs.sendMessage(tabId, {
                    type: 'tracking_state_changed',
                    state: { isTracked: true, aiPilotEnabled: false }
                });
            }
        });
    }
}
//# sourceMappingURL=tab-tracking-api.js.map