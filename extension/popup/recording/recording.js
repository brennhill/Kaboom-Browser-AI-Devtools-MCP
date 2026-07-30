/**
 * Purpose: Implements popup recording controls, mic-permission flow, and saved-recording reveal behavior.
 * Why: Gives users reliable start/stop control with explicit permission/error handling for tab capture sessions.
 * Docs: docs/features/feature/playback-engine/index.md
 * Docs: docs/features/feature/tab-recording/index.md
 */
/**
 * @fileoverview Recording UI Module for Popup
 * Manages recording controls, timer display, and mic permission flow.
 */
import { StorageKey } from '../../lib/constants.js';
import { KABOOM_RECORDING_LOG_PREFIX } from '../../lib/brand.js';
import { onStorageChanged } from '../../lib/storage/changes.js';
import { persist } from '../../lib/storage/io.js';
import { removeLocal } from '../../lib/storage/local.js';
import { readLocalState } from '../../lib/storage/validated.js';
import { reportStateRecovery } from '../../lib/storage/recovery.js';
import { sendRecordingGestureDecision, handleStartClick, handleStopClick } from './recording-io.js';
function isAudioMode(value) {
    return value === '' || value === 'tab' || value === 'mic' || value === 'both';
}
function isPendingRecordingIntent(value) {
    if (typeof value !== 'object' || value === null || Array.isArray(value))
        return false;
    const intent = value;
    return ((intent.highlight === undefined || typeof intent.highlight === 'boolean') &&
        (intent.name === undefined || typeof intent.name === 'string') &&
        (intent.fps === undefined || typeof intent.fps === 'number') &&
        (intent.audio === undefined || isAudioMode(intent.audio)) &&
        (intent.tabId === undefined || typeof intent.tabId === 'number') &&
        (intent.url === undefined || typeof intent.url === 'string'));
}
function isPopupRecordingState(value) {
    return (typeof value === 'object' &&
        value !== null &&
        (typeof value.active === 'boolean' ||
            value.active === undefined) &&
        (typeof value.name === 'string' || value.name === undefined) &&
        (typeof value.startTime === 'number' ||
            value.startTime === undefined));
}
function recordingDiagnostic(detail) {
    return {
        name: 'screen_recording_state',
        detail,
        fix: 'Start the screen recording flow again.'
    };
}
const START_LABEL = 'Record screen';
const STOP_LABEL = 'Stop recording';
const HIGHLIGHT_LABEL = '\u25CF \u00AB Click here to record';
const RECENT_RECORDING_START_MS = 8000;
const TOP_NOTICE_DURATION_MS = 4000;
const LOG = `${KABOOM_RECORDING_LOG_PREFIX} Popup:`;
const AUDIO_LABELS = {
    '': 'Video only',
    tab: 'Video + tab audio',
    mic: 'Video + microphone',
    both: 'Video + tab + mic'
};
let topNoticeTimer = null;
function getRecordSection(els) {
    const closest = els.row.closest;
    if (typeof closest !== 'function')
        return null;
    return closest.call(els.row, '.section');
}
function applyRecordHighlight(els) {
    const section = getRecordSection(els);
    if (section)
        section.classList.add('record-highlight');
    els.label.textContent = HIGHLIGHT_LABEL;
}
function removeRecordHighlight(els) {
    const section = getRecordSection(els);
    if (section)
        section.classList.remove('record-highlight');
    if (els.label.textContent === HIGHLIGHT_LABEL) {
        els.label.textContent = START_LABEL;
    }
}
// #lizard forgives
function showRecording(els, state, name, startTime) {
    const wasRecording = state.isRecording;
    removeRecordHighlight(els);
    state.isRecording = true;
    els.row.classList.add('is-recording');
    els.label.textContent = STOP_LABEL;
    els.statusEl.textContent = '';
    if (els.optionsEl)
        els.optionsEl.style.display = 'none';
    if (state.timerInterval)
        clearInterval(state.timerInterval);
    state.timerInterval = setInterval(() => {
        const elapsed = Math.round((Date.now() - startTime) / 1000);
        const mins = Math.floor(elapsed / 60);
        const secs = elapsed % 60;
        els.statusEl.textContent = `${mins}:${secs.toString().padStart(2, '0')}`;
    }, 1000);
    if (!wasRecording && Date.now() - startTime <= RECENT_RECORDING_START_MS) {
        showTopNotice(els, 'Recording started');
    }
}
function showIdle(els, state) {
    state.isRecording = false;
    removeRecordHighlight(els);
    els.row.classList.remove('is-recording');
    els.label.textContent = START_LABEL;
    els.statusEl.textContent = '';
    if (els.optionsEl)
        els.optionsEl.style.display = 'block';
    if (state.timerInterval) {
        clearInterval(state.timerInterval);
        state.timerInterval = null;
    }
}
function describePendingRecording(pending) {
    const parts = [];
    if (pending.name)
        parts.push(`Name: ${pending.name}`);
    if (typeof pending.fps === 'number')
        parts.push(`FPS: ${pending.fps}`);
    const audioLabel = AUDIO_LABELS[pending.audio ?? ''] ?? AUDIO_LABELS[''];
    parts.push(`Mode: ${audioLabel}`);
    return parts.join(' \u00b7 ');
}
function setApprovalPendingState(els, approvalEls, state, pending) {
    const setRowAriaDisabled = (value) => {
        const setAttr = els.row.setAttribute;
        const removeAttr = els.row.removeAttribute;
        if (value !== null) {
            if (typeof setAttr === 'function')
                setAttr.call(els.row, 'aria-disabled', value);
            return;
        }
        if (typeof removeAttr === 'function')
            removeAttr.call(els.row, 'aria-disabled');
    };
    const approvalPending = Boolean(pending && !pending.highlight && !state.isRecording);
    if (approvalPending) {
        if (approvalEls.detail && pending)
            approvalEls.detail.textContent = describePendingRecording(pending);
        if (approvalEls.card)
            approvalEls.card.style.display = 'block';
        els.row.classList.add('is-disabled');
        setRowAriaDisabled('true');
        if (els.optionsEl)
            els.optionsEl.style.display = 'none';
        return;
    }
    if (approvalEls.detail)
        approvalEls.detail.textContent = '';
    if (approvalEls.card)
        approvalEls.card.style.display = 'none';
    els.row.classList.remove('is-disabled');
    setRowAriaDisabled(null);
    if (!state.isRecording && els.optionsEl)
        els.optionsEl.style.display = 'block';
}
function showTopNotice(els, text) {
    const notice = els.topNoticeEl;
    if (!notice)
        return;
    notice.textContent = text;
    notice.style.display = 'block';
    if (topNoticeTimer)
        clearTimeout(topNoticeTimer);
    topNoticeTimer = setTimeout(() => {
        notice.style.display = 'none';
    }, TOP_NOTICE_DURATION_MS);
}
function showSavedLink(saveInfoEl, displayName, filePath) {
    saveInfoEl.textContent = 'Saved: ';
    const link = document.createElement('a');
    link.href = '#';
    link.id = 'reveal-recording';
    link.textContent = displayName;
    link.style.color = '#58a6ff';
    link.style.textDecoration = 'underline';
    link.style.cursor = 'pointer';
    saveInfoEl.appendChild(link);
    const linkEl = document.getElementById('reveal-recording');
    if (linkEl) {
        linkEl.addEventListener('click', (e) => {
            e.preventDefault();
            chrome.runtime.sendMessage({ type: 'reveal_file', path: filePath }, (result) => {
                if (result?.error) {
                    saveInfoEl.textContent = `Could not open folder: ${result.error}`;
                    saveInfoEl.style.color = '#f85149';
                    setTimeout(() => {
                        saveInfoEl.style.display = 'none';
                    }, 5000);
                }
            });
        });
    }
}
function showSaveResult(saveInfoEl, resp) {
    if (resp?.status !== 'saved' || !resp.name || !saveInfoEl)
        return;
    const displayName = resp.name.replace(/--\d{4}-\d{2}-\d{2}-\d{4}(-\d+)?$/, '');
    if (resp.path) {
        showSavedLink(saveInfoEl, displayName, resp.path);
    }
    else {
        saveInfoEl.textContent = `Saved: ${displayName}`;
    }
    saveInfoEl.style.display = 'block';
    setTimeout(() => {
        saveInfoEl.style.display = 'none';
    }, 12000);
}
function showStartError(saveInfoEl, errorText) {
    if (!saveInfoEl)
        return;
    saveInfoEl.textContent = errorText;
    saveInfoEl.style.display = 'block';
    saveInfoEl.style.background = 'rgba(248, 81, 73, 0.1)';
    saveInfoEl.style.color = '#f85149';
    setTimeout(() => {
        saveInfoEl.style.display = 'none';
        saveInfoEl.style.background = 'rgba(63, 185, 80, 0.1)';
        saveInfoEl.style.color = '#3fb950';
    }, 5000);
}
export function setupRecordingUI() {
    const row = document.getElementById('record-row');
    const label = document.getElementById('record-label');
    const statusEl = document.getElementById('recording-status');
    if (!row || !label || !statusEl)
        return;
    const els = {
        row,
        label,
        statusEl,
        optionsEl: document.getElementById('record-options'),
        saveInfoEl: document.getElementById('record-save-info'),
        topNoticeEl: document.getElementById('record-top-notice')
    };
    const approvalEls = {
        card: document.getElementById('record-approval-card'),
        detail: document.getElementById('record-approval-detail'),
        approveBtn: document.getElementById('record-approve-btn'),
        denyBtn: document.getElementById('record-deny-btn')
    };
    const state = { isRecording: false, timerInterval: null };
    let pendingRecordingIntent = null;
    const updatePendingRecording = (pendingValue) => {
        const pending = pendingValue;
        if (pending?.highlight && !state.isRecording) {
            applyRecordHighlight(els);
            pendingRecordingIntent = null;
            setApprovalPendingState(els, approvalEls, state, null);
            persist(removeLocal(StorageKey.PENDING_RECORDING), 'pending-recording-clear');
            return;
        }
        pendingRecordingIntent = pending && !pending.highlight ? pending : null;
        if (!pendingRecordingIntent && !state.isRecording)
            removeRecordHighlight(els);
        setApprovalPendingState(els, approvalEls, state, pendingRecordingIntent);
    };
    const clearPendingRecordingIntent = () => {
        pendingRecordingIntent = null;
        setApprovalPendingState(els, approvalEls, state, null);
        persist(removeLocal(StorageKey.PENDING_RECORDING), 'pending-recording-clear');
    };
    // Row is visible immediately with default "not recording" state.
    // Storage read updates it async — visual change is minimal (button label toggle).
    void readLocalState({
        key: StorageKey.RECORDING,
        fallback: null,
        validate: isPopupRecordingState,
        diagnostic: recordingDiagnostic('Saved screen recording was invalid or unreadable; idle state is active.')
    }).then(async (rec) => {
        console.log(LOG, 'recording state from storage:', rec);
        if (rec?.active && rec.name && rec.startTime) {
            console.log(LOG, 'resuming recording UI for', rec.name);
            showRecording(els, state, rec.name, rec.startTime);
        }
        // Check for highlight request from hover launcher
        const pendingValue = await readLocalState({
            key: StorageKey.PENDING_RECORDING,
            fallback: null,
            validate: isPendingRecordingIntent,
            diagnostic: recordingDiagnostic('Saved pending recording request was invalid or unreadable; it was cancelled.')
        });
        updatePendingRecording(pendingValue);
    });
    onStorageChanged((changes, areaName) => {
        if (areaName === 'local' && changes[StorageKey.RECORDING]) {
            const rec = changes[StorageKey.RECORDING].newValue;
            if (rec !== undefined && !isPopupRecordingState(rec)) {
                reportStateRecovery(recordingDiagnostic('Screen recording changed to an invalid value; idle state is active.'));
                showIdle(els, state);
                return;
            }
            console.log(LOG, 'recording state changed:', rec);
            if (rec?.active && rec.name && rec.startTime) {
                showRecording(els, state, rec.name, rec.startTime);
            }
            else {
                showIdle(els, state);
            }
            setApprovalPendingState(els, approvalEls, state, pendingRecordingIntent);
            return;
        }
        if (areaName === 'local' && changes[StorageKey.PENDING_RECORDING]) {
            const pending = changes[StorageKey.PENDING_RECORDING].newValue;
            if (pending !== undefined && !isPendingRecordingIntent(pending)) {
                reportStateRecovery(recordingDiagnostic('Pending recording changed to an invalid value; it was cancelled.'));
                updatePendingRecording(null);
                return;
            }
            updatePendingRecording(pending);
        }
    });
    approvalEls.approveBtn?.addEventListener('click', (event) => {
        event.preventDefault();
        sendRecordingGestureDecision('recording_gesture_granted');
        clearPendingRecordingIntent();
    });
    approvalEls.denyBtn?.addEventListener('click', (event) => {
        event.preventDefault();
        sendRecordingGestureDecision('recording_gesture_denied');
        clearPendingRecordingIntent();
    });
    void readLocalState({
        key: StorageKey.PENDING_MIC_RECORDING,
        fallback: null,
        validate: (value) => typeof value === 'object' &&
            value !== null &&
            (value.audioMode === undefined ||
                isAudioMode(value.audioMode)),
        diagnostic: recordingDiagnostic('Saved microphone recording request was invalid or unreadable; it was cancelled.')
    }).then(async (intent) => {
        console.log(LOG, 'pending mic recording intent:', intent);
        if (!intent?.audioMode)
            return;
        console.log(LOG, 'consuming mic intent, pre-selecting audio mode:', intent.audioMode);
        await removeLocal(StorageKey.PENDING_MIC_RECORDING);
        chrome.tabs.query({ active: true, currentWindow: true }, (tabs) => {
            if (tabs[0]?.id) {
                chrome.tabs
                    .sendMessage(tabs[0].id, {
                    type: 'kaboom_action_toast',
                    text: '',
                    detail: '',
                    state: 'success',
                    duration_ms: 1
                })
                    .catch(() => { });
            }
        });
        const audioSelect = document.getElementById('record-audio-mode');
        if (audioSelect)
            audioSelect.value = intent.audioMode;
    });
    void readLocalState({
        key: StorageKey.RECORD_AUDIO_PREF,
        fallback: '',
        validate: (value) => isAudioMode(value),
        diagnostic: recordingDiagnostic('Saved recording audio preference was invalid or unreadable; video-only is active.')
    }).then((saved) => {
        if (saved) {
            const audioSelect = document.getElementById('record-audio-mode');
            if (audioSelect)
                audioSelect.value = saved;
        }
    });
    row.addEventListener('click', () => {
        console.log(LOG, 'record row clicked, isRecording:', state.isRecording);
        if (pendingRecordingIntent && !state.isRecording) {
            console.log(LOG, 'record row click ignored while approval is pending');
            return;
        }
        removeRecordHighlight(els);
        if (state.isRecording) {
            handleStopClick(els, state, showIdle, showSaveResult);
        }
        else {
            handleStartClick(els, state, showRecording, showIdle, showStartError);
        }
    });
}
//# sourceMappingURL=recording.js.map