/**
 * Purpose: Popup UI module for action workflow (event) recording — start/stop via daemon HTTP API.
 * Why: Separates event recording controls from screen recording, keeping each feature self-contained.
 * Docs: docs/features/feature/flow-recording/index.md
 */
import { DEFAULT_SERVER_URL, StorageKey } from '../lib/constants.js';
import { postDaemonJSON } from '../lib/daemon-http.js';
import { getLocal, setLocal, removeLocal } from '../lib/storage-utils.js';
const START_LABEL = 'Record action workflow';
const STOP_LABEL = 'Stop recording';
function showRecording(els, state) {
    state.isRecording = true;
    els.row.classList.add('is-recording');
    els.label.textContent = STOP_LABEL;
    els.statusEl.textContent = '';
    if (state.timerInterval)
        clearInterval(state.timerInterval);
    const start = state.startTime ?? Date.now();
    state.timerInterval = setInterval(() => {
        const elapsed = Math.round((Date.now() - start) / 1000);
        const mins = Math.floor(elapsed / 60);
        const secs = elapsed % 60;
        els.statusEl.textContent = `${mins}:${secs.toString().padStart(2, '0')}`;
    }, 1000);
}
function showIdle(els, state) {
    state.isRecording = false;
    state.recordingId = null;
    state.startTime = null;
    els.row.classList.remove('is-recording');
    els.label.textContent = START_LABEL;
    els.statusEl.textContent = '';
    if (state.timerInterval) {
        clearInterval(state.timerInterval);
        state.timerInterval = null;
    }
}
function showError(els, message) {
    els.statusEl.textContent = message;
    els.statusEl.style.color = '#f85149';
    setTimeout(() => {
        els.statusEl.textContent = '';
        els.statusEl.style.color = '';
    }, 5000);
}
async function getServerUrl() {
    const value = await getLocal(StorageKey.SERVER_URL);
    return value || DEFAULT_SERVER_URL;
}
function getConfigureError(data) {
    const message = data.error?.message;
    return typeof message === 'string' && message.length > 0 ? message : null;
}
function extractRecordingID(data) {
    const text = data.result?.content?.[0]?.text ?? '';
    const idMatch = text.match(/"recording_id"\s*:\s*"([^"]+)"/);
    return idMatch?.[1] ?? null;
}
async function callConfigureFromPopup(argumentsPayload) {
    const serverUrl = await getServerUrl();
    const resp = await postDaemonJSON(`${serverUrl}/mcp`, {
        jsonrpc: '2.0',
        id: Date.now(),
        method: 'tools/call',
        params: {
            name: 'configure',
            arguments: argumentsPayload
        }
    });
    if (!resp.ok) {
        throw new Error(`Server error: HTTP ${resp.status}`);
    }
    return (await resp.json());
}
async function startActionRecording(els, state) {
    els.label.textContent = 'Starting...';
    try {
        const data = await callConfigureFromPopup({
            what: 'event_recording_start',
            name: `workflow-${Date.now()}`
        });
        const configureError = getConfigureError(data);
        if (configureError) {
            showIdle(els, state);
            showError(els, configureError);
            return;
        }
        state.recordingId = extractRecordingID(data);
        state.startTime = Date.now();
        // Persist state so reopening popup shows recording in progress
        void setLocal(StorageKey.ACTION_RECORDING, {
            active: true,
            recordingId: state.recordingId,
            startTime: state.startTime
        });
        showRecording(els, state);
    }
    catch (err) {
        showIdle(els, state);
        showError(els, `Connection failed: ${err instanceof Error ? err.message : String(err)}`);
    }
}
async function stopActionRecording(els, state) {
    els.label.textContent = 'Stopping...';
    try {
        const data = await callConfigureFromPopup({
            what: 'event_recording_stop',
            recording_id: state.recordingId ?? ''
        });
        const configureError = getConfigureError(data);
        if (configureError) {
            showError(els, configureError);
        }
        void removeLocal(StorageKey.ACTION_RECORDING);
        showIdle(els, state);
    }
    catch (err) {
        showIdle(els, state);
        showError(els, `Connection failed: ${err instanceof Error ? err.message : String(err)}`);
    }
}
export function setupActionRecordingUI() {
    const row = document.getElementById('action-record-row');
    const label = document.getElementById('action-record-label');
    const statusEl = document.getElementById('action-recording-status');
    if (!row || !label || !statusEl)
        return;
    const els = { row, label, statusEl };
    const state = {
        isRecording: false,
        recordingId: null,
        timerInterval: null,
        startTime: null
    };
    // Restore state if popup was closed during recording
    void getLocal(StorageKey.ACTION_RECORDING).then((value) => {
        const saved = value;
        if (saved?.active && saved.recordingId) {
            state.recordingId = saved.recordingId;
            state.startTime = saved.startTime ?? Date.now();
            showRecording(els, state);
        }
    });
    row.addEventListener('click', () => {
        if (state.isRecording) {
            void stopActionRecording(els, state);
        }
        else {
            void startActionRecording(els, state);
        }
    });
}
//# sourceMappingURL=action-recording.js.map