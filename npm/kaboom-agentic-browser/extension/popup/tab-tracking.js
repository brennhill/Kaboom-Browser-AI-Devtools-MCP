/**
 * Purpose: Manages popup tab-tracking UI state and track/untrack transitions for the active browser tab.
 * Why: Keeps the tracked-tab lifecycle explicit so content-script injection and status UX stay synchronized.
 * Docs: docs/features/feature/tab-tracking-ux/index.md
 */
/**
 * @fileoverview Tab Tracking Module for Popup
 * Manages the "Track This Tab" button and tracking status
 */
import { isInternalUrl } from './ui-utils.js';
import { StorageKey } from '../lib/constants.js';
import { getLocals, onStorageChanged } from '../lib/storage-utils.js'; // async API only
import { isDomainCloaked } from '../lib/cloaked-domains.js';
import { handleAuditClick, handleStopTracking, handleUrlClick, handleTrackPageClick as handleTrackPageClickAPI } from './tab-tracking-api.js';
let trackingStorageSyncInstalled = false;
function hideAuditButton() {
    const trackingBarAudit = document.getElementById('tracking-bar-audit');
    if (!trackingBarAudit)
        return;
    trackingBarAudit.style.display = 'none';
    trackingBarAudit.onclick = null;
}
/**
 * Initialize the Track This Tab button.
 * Shows current tracking status and handles track/untrack.
 * Disables the button on internal Chrome pages where tracking is impossible.
 */
function showInternalPageState(btn) {
    const trackingBar = document.getElementById('tracking-bar');
    if (trackingBar)
        trackingBar.style.display = 'none';
    hideAuditButton();
    btn.disabled = true;
    btn.textContent = 'Cannot Track Internal Pages';
    btn.title = 'Chrome blocks extensions on internal pages like chrome:// and about:';
    Object.assign(btn.style, { opacity: '0.5', background: '#252525', color: '#888', borderColor: '#333' });
}
function showCloakedState(btn) {
    const trackingBar = document.getElementById('tracking-bar');
    if (trackingBar)
        trackingBar.style.display = 'none';
    hideAuditButton();
    btn.disabled = true;
    btn.textContent = 'Tracking Disabled on This Site';
    btn.title = 'This domain is in the cloaked domains list. KaBOOM! is disabled here to prevent interference.';
    Object.assign(btn.style, { opacity: '0.5', background: '#252525', color: '#888', borderColor: '#333' });
}
function showTrackingState(btn, trackedTabUrl, trackedTabId) {
    // Hide the hero button area
    const heroEl = document.getElementById('track-hero');
    if (heroEl)
        heroEl.style.display = 'none';
    const noTrackEl = document.getElementById('no-tracking-warning');
    if (noTrackEl)
        noTrackEl.style.display = 'none';
    // Show the compact tracking bar
    const trackingBar = document.getElementById('tracking-bar');
    const trackingBarUrl = document.getElementById('tracking-bar-url');
    const trackingBarAudit = document.getElementById('tracking-bar-audit');
    const trackingBarStop = document.getElementById('tracking-bar-stop');
    if (trackingBar)
        trackingBar.style.display = 'flex';
    if (trackingBarUrl && trackedTabUrl) {
        trackingBarUrl.textContent = trackedTabUrl;
        trackingBarUrl.onclick = () => {
            void handleUrlClick(trackedTabId);
        };
    }
    if (trackingBarAudit) {
        trackingBarAudit.textContent = 'Audit';
        trackingBarAudit.style.display = 'inline-flex';
        trackingBarAudit.onclick = () => {
            void handleAuditClick(trackedTabUrl);
        };
    }
    if (trackingBarStop) {
        trackingBarStop.onclick = (e) => {
            e.stopPropagation();
            void handleStopTracking(showIdleState);
        };
    }
}
function showIdleState(btn) {
    // Show the hero button area
    const heroEl = document.getElementById('track-hero');
    if (heroEl)
        heroEl.style.display = '';
    btn.textContent = 'Track This Tab';
    Object.assign(btn.style, {
        background: '#1a3a5c',
        color: '#58a6ff',
        borderColor: '#58a6ff',
        fontSize: '16px',
        fontWeight: '600',
        padding: '14px 16px',
        borderWidth: '2px'
    });
    const heroDesc = document.getElementById('track-hero-desc');
    if (heroDesc)
        heroDesc.style.display = '';
    // Hide the tracking bar
    const trackingBar = document.getElementById('tracking-bar');
    if (trackingBar)
        trackingBar.style.display = 'none';
    hideAuditButton();
    // Show "no tracking" warning
    const noTrackEl = document.getElementById('no-tracking-warning');
    if (noTrackEl)
        noTrackEl.style.display = 'block';
}
function syncTrackButtonState(btn) {
    void getLocals([StorageKey.TRACKED_TAB_ID, StorageKey.TRACKED_TAB_URL]).then((result) => {
        const trackedTabId = result[StorageKey.TRACKED_TAB_ID];
        const trackedTabUrl = result[StorageKey.TRACKED_TAB_URL];
        chrome.tabs.query({ active: true, currentWindow: true }, (tabs) => {
            const currentUrl = tabs?.[0]?.url;
            if (trackedTabId) {
                showTrackingState(btn, trackedTabUrl, trackedTabId);
            }
            else if (isInternalUrl(currentUrl)) {
                showInternalPageState(btn);
            }
            else {
                // Check cloaked domains (async)
                let hostname = '';
                try {
                    hostname = currentUrl ? new URL(currentUrl).hostname : '';
                }
                catch { /* malformed URL */ }
                isDomainCloaked(hostname).then((cloaked) => {
                    if (cloaked) {
                        showCloakedState(btn);
                    }
                    else {
                        showIdleState(btn);
                    }
                }).catch(() => showIdleState(btn));
            }
        });
    });
}
function installTrackingStorageSync(btn) {
    if (trackingStorageSyncInstalled)
        return;
    trackingStorageSyncInstalled = true;
    onStorageChanged((changes, areaName) => {
        if (areaName !== 'local')
            return;
        if (!changes[StorageKey.TRACKED_TAB_ID] && !changes[StorageKey.TRACKED_TAB_URL])
            return;
        syncTrackButtonState(btn);
    });
}
export function initTrackPageButton() {
    const btn = document.getElementById('track-page-btn');
    if (!btn)
        return;
    syncTrackButtonState(btn);
    installTrackingStorageSync(btn);
    btn.addEventListener('click', () => {
        void handleTrackPageClickAPI(showInternalPageState, showCloakedState, showTrackingState, showIdleState);
    });
}
// Re-export for consumers that import handleTrackPageClick directly
export async function handleTrackPageClick() {
    return handleTrackPageClickAPI(showInternalPageState, showCloakedState, showTrackingState, showIdleState);
}
//# sourceMappingURL=tab-tracking.js.map