/**
 * Purpose: Chrome runtime messaging, storage, and mic permission logic for recording controls.
 * Why: Separates browser API side-effects from recording UI rendering.
 * Docs: docs/features/feature/playback-engine/index.md
 * Docs: docs/features/feature/tab-recording/index.md
 */
import { StorageKey } from '../lib/constants.js';
import { KABOOM_RECORDING_LOG_PREFIX } from '../lib/brand.js';
import { errorMessage } from '../lib/error-utils.js';
import { setLocal, removeLocal } from '../lib/storage-utils.js';
const LOG = `${KABOOM_RECORDING_LOG_PREFIX} Popup:`;
export function sendRecordingGestureDecision(type) {
    chrome.runtime.sendMessage({ type }, () => {
        void chrome.runtime.lastError;
    });
}
function showMicPermissionPrompt(saveInfoEl, audioMode) {
    chrome.tabs.query({ active: true, currentWindow: true }, (activeTabs) => {
        void setLocal(StorageKey.PENDING_MIC_RECORDING, { audioMode, returnTabId: activeTabs[0]?.id });
    });
    saveInfoEl.innerHTML =
        'Microphone access needed. <a href="#" id="grant-mic-link" style="color: #58a6ff; text-decoration: underline; cursor: pointer">Grant access</a>';
    saveInfoEl.style.display = 'block';
    saveInfoEl.style.background = 'rgba(248, 81, 73, 0.1)';
    saveInfoEl.style.color = '#f85149';
    const link = document.getElementById('grant-mic-link');
    if (link) {
        link.addEventListener('click', (e) => {
            e.preventDefault();
            chrome.tabs.create({ url: chrome.runtime.getURL('mic-permission.html') });
        });
    }
}
function sendRecordStart(els, state, audioMode, showRecording, showIdle, showStartError) {
    console.log(LOG, 'sendStart() called, sending screen_recording_start with audio:', audioMode);
    chrome.runtime.sendMessage({ type: 'screen_recording_start', audio: audioMode }, (resp) => {
        console.log(LOG, 'screen_recording_start response:', resp);
        if (chrome.runtime.lastError) {
            console.error(LOG, 'screen_recording_start lastError:', chrome.runtime.lastError.message);
        }
        if (resp?.status === 'recording' && resp.name) {
            showRecording(els, state, resp.name, resp.startTime ?? Date.now());
        }
        else {
            showIdle(els, state);
            if (resp?.error)
                showStartError(els.saveInfoEl, resp.error);
        }
    });
}
// #lizard forgives
function tryMicPermissionThenStart(els, state, audioMode, showRecording, showIdle, showStartError) {
    console.log(LOG, 'trying getUserMedia from popup...');
    navigator.mediaDevices
        .getUserMedia({ audio: true })
        .then((micStream) => {
        console.log(LOG, 'getUserMedia succeeded from popup');
        micStream.getTracks().forEach((t) => t.stop());
        void setLocal(StorageKey.MIC_GRANTED, true);
        sendRecordStart(els, state, audioMode, showRecording, showIdle, showStartError);
    })
        .catch((err) => {
        console.log(LOG, 'getUserMedia FAILED:', err.name, errorMessage(err));
        void removeLocal(StorageKey.MIC_GRANTED);
        showIdle(els, state);
        if (els.saveInfoEl)
            showMicPermissionPrompt(els.saveInfoEl, audioMode);
    });
}
export function handleStartClick(els, state, showRecording, showIdle, showStartError) {
    const audioSelect = document.getElementById('record-audio-mode');
    const audioMode = audioSelect?.value ?? '';
    void setLocal(StorageKey.RECORD_AUDIO_PREF, audioMode);
    if (els.optionsEl)
        els.optionsEl.style.display = 'none';
    if (els.saveInfoEl)
        els.saveInfoEl.style.display = 'none';
    els.label.textContent = 'Starting...';
    if (audioMode === 'mic' || audioMode === 'both') {
        console.log(LOG, 'mic/both mode — checking stored mic approval');
        tryMicPermissionThenStart(els, state, audioMode, showRecording, showIdle, showStartError);
    }
    else {
        sendRecordStart(els, state, audioMode, showRecording, showIdle, showStartError);
    }
}
export function handleStopClick(els, state, showIdle, showSaveResult) {
    els.row.classList.remove('is-recording');
    els.label.textContent = 'Saving...';
    console.log(LOG, 'sending screen_recording_stop');
    chrome.runtime.sendMessage({ type: 'screen_recording_stop' }, (resp) => {
        console.log(LOG, 'screen_recording_stop response:', resp);
        if (chrome.runtime.lastError) {
            console.error(LOG, 'screen_recording_stop lastError:', chrome.runtime.lastError.message);
        }
        showIdle(els, state);
        showSaveResult(els.saveInfoEl, resp);
    });
}
//# sourceMappingURL=recording-io.js.map