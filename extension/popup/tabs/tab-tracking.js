/**
 * Purpose: Manages popup tab-tracking UI state and track/untrack transitions for the active browser tab.
 * Why: Keeps the tracked-tab lifecycle explicit so content-script injection and status UX stay synchronized.
 * Docs: docs/features/feature/tab-tracking-ux/index.md
 */
/**
 * @fileoverview Tab Tracking Module for Popup
 * Manages the "Track This Tab" button and tracking status
 */
import { isInternalUrl } from '../../lib/tabs/internal-url.js';
import { StorageKey } from '../../lib/constants.js';
import { onStorageChanged } from '../../lib/storage/changes.js';
import { readTrackedTab } from '../../lib/tabs/tracked-tab-storage.js';
import { isDomainCloaked } from '../../lib/tabs/cloaked-domains.js';
import { handleAuditClick, handleStopTracking, handleUrlClick, handleTrackPageClick as handleTrackPageClickAPI } from './tab-tracking-api.js';
let trackingStorageSyncInstalled = false;
let trackingRuntimeSyncInstalled = false;
/**
 * Audit launches the QA-scan workflow through the terminal side panel. It stays
 * hidden until the side-panel/terminal path is fully verified, so users can't
 * reach a half-working flow. Flip to `true` to restore it (see the matching flag
 * in content/ui/tracked-hover-launcher.ts).
 */
const AUDIT_BUTTON_ENABLED = false;
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
function showTrackingState(btn, trackedTabTitle, trackedTabUrl, trackedTabId, continuity) {
    // Hide the hero button area
    const heroEl = document.getElementById('track-hero');
    if (heroEl)
        heroEl.style.display = 'none';
    const noTrackEl = document.getElementById('no-tracking-warning');
    if (noTrackEl)
        noTrackEl.style.display = 'none';
    // Show the compact tracking bar
    const trackingBar = document.getElementById('tracking-bar');
    const trackingBarTitle = document.getElementById('tracking-bar-title');
    const trackingBarUrl = document.getElementById('tracking-bar-url');
    const trackingBarAudit = document.getElementById('tracking-bar-audit');
    const trackingBarStop = document.getElementById('tracking-bar-stop');
    if (trackingBar)
        trackingBar.style.display = 'flex';
    if (trackingBarTitle) {
        const progress = trackingProgressLabel(continuity?.phase);
        trackingBarTitle.textContent = progress
            ? `${progress} · ${trackedTabTitle || 'Tracked tab'}`
            : trackedTabTitle || 'Tracked tab';
    }
    if (trackingBarUrl && trackedTabUrl) {
        trackingBarUrl.textContent = trackedTabUrl;
        trackingBarUrl.onclick = () => {
            void handleUrlClick(trackedTabId);
        };
    }
    if (trackingBarAudit) {
        if (AUDIT_BUTTON_ENABLED) {
            trackingBarAudit.textContent = 'Audit';
            trackingBarAudit.style.display = 'inline-flex';
            trackingBarAudit.onclick = () => {
                // Pass the tracked tab id: the popup has no sender.tab, so the background
                // cannot resolve a tab without an await — which would expire the user
                // gesture and stop chrome.sidePanel.open() from opening the panel.
                void handleAuditClick(trackedTabUrl, trackedTabId);
            };
        }
        else {
            trackingBarAudit.style.display = 'none';
            trackingBarAudit.onclick = null;
        }
    }
    if (trackingBarStop) {
        trackingBarStop.onclick = (e) => {
            e.stopPropagation();
            void handleStopTracking(showIdleState);
        };
    }
}
function trackingProgressLabel(phase) {
    switch (phase) {
        case 'navigation_started':
        case 'provisional_url':
            return 'Navigating';
        case 'content_injecting':
            return 'Reconnecting page';
        case 'extension_reconnecting':
            return 'Reconnecting';
        case 'recovery_failed':
            return 'Recovery needs attention';
        default:
            return '';
    }
}
async function readTrackingContinuity() {
    try {
        const response = (await chrome.runtime.sendMessage({
            type: 'get_tracking_state'
        }));
        return response?.state?.continuity;
    }
    catch {
        return undefined;
    }
}
function showStaleState(btn, trackedTabTitle, trackedTabUrl) {
    showIdleState(btn);
    btn.textContent = 'Track Current Tab';
    btn.title = 'The previously tracked tab is gone. Track the current tab instead.';
    const warning = document.getElementById('no-tracking-warning');
    if (warning)
        warning.textContent = 'The previously tracked tab is no longer available.';
    const identity = document.getElementById('stale-tracking-identity');
    if (identity) {
        identity.textContent = [trackedTabTitle, trackedTabUrl].filter(Boolean).join(' — ');
        identity.style.display = identity.textContent ? 'block' : 'none';
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
    if (noTrackEl) {
        noTrackEl.style.display = 'block';
        noTrackEl.textContent = 'No tab tracked — data capture disabled';
    }
    const staleIdentity = document.getElementById('stale-tracking-identity');
    if (staleIdentity) {
        staleIdentity.textContent = '';
        staleIdentity.style.display = 'none';
    }
}
function syncTrackButtonState(btn) {
    void Promise.all([readTrackedTab(), readTrackingContinuity()]).then(([tracked, continuity]) => {
        const { id: trackedTabId, url: trackedTabUrl, title: trackedTabTitle } = tracked;
        chrome.tabs.query({ active: true, currentWindow: true }, (tabs) => {
            const currentUrl = tabs?.[0]?.url;
            if (trackedTabId) {
                void chrome.tabs.get(trackedTabId).then(() => showTrackingState(btn, trackedTabTitle, trackedTabUrl, trackedTabId, continuity), () => showStaleState(btn, trackedTabTitle, trackedTabUrl));
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
                catch {
                    /* malformed URL */
                }
                isDomainCloaked(hostname)
                    .then((cloaked) => {
                    if (cloaked) {
                        showCloakedState(btn);
                    }
                    else {
                        showIdleState(btn);
                    }
                })
                    .catch(() => showIdleState(btn));
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
        if (!changes[StorageKey.TRACKED_TAB_ID] &&
            !changes[StorageKey.TRACKED_TAB_URL] &&
            !changes[StorageKey.TRACKED_TAB_TITLE])
            return;
        syncTrackButtonState(btn);
    });
}
function installTrackingRuntimeSync(btn) {
    if (trackingRuntimeSyncInstalled)
        return;
    trackingRuntimeSyncInstalled = true;
    chrome.runtime.onMessage.addListener((message) => {
        if (message.type === 'tracking_continuity_changed')
            syncTrackButtonState(btn);
    });
}
export function initTrackPageButton() {
    const btn = document.getElementById('track-page-btn');
    if (!btn)
        return;
    syncTrackButtonState(btn);
    installTrackingStorageSync(btn);
    installTrackingRuntimeSync(btn);
    btn.addEventListener('click', () => {
        void handleTrackPageClickAPI(showInternalPageState, showCloakedState, showTrackingState, showIdleState);
    });
}
//# sourceMappingURL=tab-tracking.js.map