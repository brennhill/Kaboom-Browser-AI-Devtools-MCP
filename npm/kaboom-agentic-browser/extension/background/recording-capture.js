/**
 * Purpose: Acquires tab capture streams, manages offscreen documents, and handles user gesture flow for video recording.
 * Docs: docs/features/feature/flow-recording/index.md
 */
// recording-capture.ts — Tab capture stream acquisition, offscreen document management, and user gesture flow.
// Extracted from recording.ts to separate media plumbing from recording lifecycle.
import { scaleTimeout } from '../lib/timeouts.js';
import { StorageKey } from '../lib/constants.js';
import { sendTabToast } from './event-listeners.js';
import { errorMessage } from '../lib/error-utils.js';
import { delay } from '../lib/timeout-utils.js';
import { buildRecordingToastLabel } from './recording-utils.js';
import { setLocal, removeLocal } from '../lib/storage-utils.js';
import { KABOOM_RECORDING_LOG_PREFIX } from '../lib/brand.js';
const LOG = KABOOM_RECORDING_LOG_PREFIX;
const AWAITING_APPROVAL_BADGE_TEXT = '?';
const AWAITING_APPROVAL_BADGE_COLOR = '#d29922';
let awaitingApprovalBadgeInterval = null;
function applyAwaitingApprovalBadge() {
    if (!chrome.action)
        return;
    try {
        chrome.action.setBadgeText({ text: AWAITING_APPROVAL_BADGE_TEXT });
        chrome.action.setBadgeBackgroundColor({ color: AWAITING_APPROVAL_BADGE_COLOR });
    }
    catch {
        // Badge updates are best-effort.
    }
}
function startAwaitingApprovalBadge() {
    stopAwaitingApprovalBadge();
    applyAwaitingApprovalBadge();
    // Re-apply periodically so health badge updates don't overwrite waiting state.
    awaitingApprovalBadgeInterval = setInterval(applyAwaitingApprovalBadge, scaleTimeout(1000));
}
function stopAwaitingApprovalBadge() {
    if (awaitingApprovalBadgeInterval) {
        clearInterval(awaitingApprovalBadgeInterval);
        awaitingApprovalBadgeInterval = null;
    }
    if (!chrome.action)
        return;
    try {
        chrome.action.setBadgeText({ text: '' });
    }
    catch {
        // Badge updates are best-effort.
    }
}
/** Ensure the offscreen document exists for recording. */
export async function ensureOffscreenDocument() {
    // Check if an offscreen document already exists
    const contexts = await chrome.runtime.getContexts({
        contextTypes: [chrome.runtime.ContextType.OFFSCREEN_DOCUMENT]
    });
    if (contexts.length > 0)
        return;
    await chrome.offscreen.createDocument({
        url: 'offscreen.html',
        reasons: [chrome.offscreen.Reason.USER_MEDIA],
        justification: 'Tab video recording via MediaRecorder'
    });
}
/**
 * Get a media stream ID, recovering from "active stream" errors by closing the
 * stale offscreen document (which releases leaked streams) and retrying once.
 */
export async function getStreamIdWithRecovery(tabId) {
    try {
        return await getStreamId(tabId);
    }
    catch (err) {
        if (errorMessage(err)?.includes('active stream')) {
            console.warn(LOG, 'Active stream detected — closing offscreen document to release leaked streams');
            try {
                await chrome.offscreen.closeDocument();
            }
            catch {
                /* might not exist */
            }
            // Brief pause to let Chrome release the capture
            await delay(scaleTimeout(200));
            console.log(LOG, 'Retrying getMediaStreamId after cleanup');
            return await getStreamId(tabId);
        }
        throw err;
    }
}
/** Wrapper around chrome.tabCapture.getMediaStreamId with logging. */
function getStreamId(tabId) {
    return new Promise((resolve, reject) => {
        chrome.tabCapture.getMediaStreamId({ targetTabId: tabId }, (id) => {
            if (chrome.runtime.lastError) {
                console.error(LOG, 'getMediaStreamId FAILED:', chrome.runtime.lastError.message);
                reject(new Error(chrome.runtime.lastError.message ?? 'getMediaStreamId failed'));
            }
            else {
                console.log(LOG, 'Got stream ID:', id?.substring(0, 20) + '...');
                resolve(id);
            }
        });
    });
}
/**
 * Request user gesture for recording permission (used for MCP-initiated recordings).
 * Shows a toast prompting the user to open the Kaboom popup and approve.
 */
export async function requestRecordingGesture(tab, name, fps, audio, mediaType) {
    chrome.tabs.update(tab.id, { active: true });
    sendTabToast(tab.id, `\u2191 Open KaBOOM!`, `Approve ${mediaType.toLowerCase()} recording request`, 'audio', scaleTimeout(30000));
    await setLocal(StorageKey.PENDING_RECORDING, { name, fps, audio, tabId: tab.id, url: tab.url });
    startAwaitingApprovalBadge();
    let gestureResult;
    try {
        gestureResult = await waitForRecordingGesture(scaleTimeout(30000));
    }
    finally {
        stopAwaitingApprovalBadge();
        await removeLocal(StorageKey.PENDING_RECORDING);
    }
    if (gestureResult === 'denied') {
        console.log(LOG, 'GESTURE_DENIED: User denied recording request from popup');
        return {
            status: 'error',
            name: '',
            error: `RECORD_START: ${mediaType} recording request was denied in the KaBOOM! popup.`
        };
    }
    if (gestureResult !== 'granted') {
        console.log(LOG, 'GESTURE_TIMEOUT: User did not approve recording request within 30s');
        sendTabToast(tab.id, `\u2191 Open KaBOOM!`, `Approve ${mediaType.toLowerCase()} recording request`, 'audio', scaleTimeout(8000));
        return {
            status: 'error',
            name: '',
            error: `RECORD_START: ${mediaType} recording requires popup approval. Open the KaBOOM! popup, click Approve, then try again.`
        };
    }
    sendTabToast(tab.id, buildRecordingToastLabel(tab.url), '', 'success', scaleTimeout(2000));
    return { status: 'ok', name };
}
/** Wait for popup approval decision (grant/deny) with timeout fallback. */
function waitForRecordingGesture(timeoutMs) {
    return new Promise((resolve) => {
        const timeout = setTimeout(() => {
            chrome.runtime.onMessage.removeListener(listener);
            resolve('timeout');
        }, timeoutMs);
        const listener = (message) => {
            if (message.type === 'recording_gesture_granted') {
                clearTimeout(timeout);
                chrome.runtime.onMessage.removeListener(listener);
                resolve('granted');
                return;
            }
            if (message.type === 'recording_gesture_denied') {
                clearTimeout(timeout);
                chrome.runtime.onMessage.removeListener(listener);
                resolve('denied');
            }
        };
        chrome.runtime.onMessage.addListener(listener);
    });
}
//# sourceMappingURL=recording-capture.js.map