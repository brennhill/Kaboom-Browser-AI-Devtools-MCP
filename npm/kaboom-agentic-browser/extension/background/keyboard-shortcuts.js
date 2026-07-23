/**
 * Purpose: Keyboard shortcut listeners for draw mode, action-sequence recording, and screen recording.
 * Split from event-listeners.ts to keep files under 800 LOC.
 */
// =============================================================================
// RECORDING SHORTCUT TYPES & HELPERS
// =============================================================================
import { errorMessage } from '../lib/error-utils.js';
import { getActiveTab, sendTabToast } from './event-listeners.js';
import { toggleDrawModeForTab } from './draw-mode-toggle.js';
import { buildScreenRecordingSlug } from './recording-utils.js';
import { trackUIFeature } from './ui-usage-tracker.js';
export function buildActionSequenceRecordingName(now = new Date()) {
    const yyyy = now.getFullYear();
    const mm = String(now.getMonth() + 1).padStart(2, '0');
    const dd = String(now.getDate()).padStart(2, '0');
    const hh = String(now.getHours()).padStart(2, '0');
    const min = String(now.getMinutes()).padStart(2, '0');
    const ss = String(now.getSeconds()).padStart(2, '0');
    return `action-sequence--${yyyy}-${mm}-${dd}-${hh}${min}${ss}`;
}
export async function toggleScreenRecording(handlers, tab, logFn) {
    if (handlers.isRecording()) {
        const result = await handlers.stopRecording();
        if (result.status === 'saved') {
            sendTabToast(tab.id, 'Recording saved', result.name || '', 'success', 3000);
        }
        return;
    }
    const slug = buildScreenRecordingSlug(tab.url);
    const result = await handlers.startRecording(slug, 15, '', '', true, tab.id);
    if (result.status !== 'recording' && tab.id) {
        sendTabToast(tab.id, 'Recording failed', result.error || 'Could not start screen recording', 'error', 4000);
        if (logFn)
            logFn(`Screen recording start failed: ${result.error}`);
    }
}
// =============================================================================
// DRAW MODE KEYBOARD SHORTCUT
// =============================================================================
/**
 * Install keyboard shortcut listener for draw mode toggle (Ctrl+Shift+D / Cmd+Shift+D).
 * Sends KABOOM_DRAW_MODE_START or KABOOM_DRAW_MODE_STOP to the active tab's content script.
 */
export function installDrawModeCommandListener(logFn) {
    if (typeof chrome === 'undefined' || !chrome.commands)
        return;
    chrome.commands.onCommand.addListener(async (command) => {
        if (command !== 'toggle_draw_mode')
            return;
        try {
            const tab = await getActiveTab();
            if (!tab?.id)
                return;
            try {
                trackUIFeature('annotations');
                await toggleDrawModeForTab(tab.id);
            }
            catch {
                if (logFn)
                    logFn('Cannot reach content script for draw mode toggle');
                sendTabToast(tab.id, 'Draw mode unavailable', 'Refresh the page and try again', 'error', 3000);
            }
        }
        catch (err) {
            if (logFn)
                logFn(`Draw mode keyboard shortcut error: ${errorMessage(err)}`);
        }
    });
}
// =============================================================================
// ACTION-SEQUENCE RECORDING SHORTCUT
// =============================================================================
/**
 * Install keyboard shortcut listener for action-sequence recording toggle.
 * Shortcut is defined in manifest as `toggle_action_sequence_recording`.
 */
export function installRecordingShortcutCommandListener(handlers, logFn) {
    if (typeof chrome === 'undefined' || !chrome.commands)
        return;
    chrome.commands.onCommand.addListener(async (command) => {
        if (command !== 'toggle_action_sequence_recording')
            return;
        try {
            const tab = await getActiveTab();
            if (!tab?.id)
                return;
            if (handlers.isRecording()) {
                const stopResult = await handlers.stopRecording(false);
                if (stopResult.status !== 'saved' && stopResult.status !== 'stopped') {
                    sendTabToast(tab.id, 'Stop recording failed', stopResult.error || 'Could not stop action sequence recording', 'error', 3500);
                }
                return;
            }
            const name = buildActionSequenceRecordingName();
            const startResult = await handlers.startRecording(name, 15, '', '', true, tab.id);
            if (startResult.status !== 'recording') {
                sendTabToast(tab.id, 'Start recording failed', startResult.error || 'Open the extension popup and try Record action sequence', 'error', 3500);
            }
        }
        catch (err) {
            if (logFn)
                logFn(`Recording shortcut error: ${errorMessage(err)}`);
        }
    });
}
// =============================================================================
// SCREEN RECORDING KEYBOARD SHORTCUT
// =============================================================================
/**
 * Install keyboard shortcut listener for screen recording toggle (Alt+Shift+R).
 */
export function installScreenRecordingCommandListener(handlers, logFn) {
    if (typeof chrome === 'undefined' || !chrome.commands)
        return;
    chrome.commands.onCommand.addListener(async (command) => {
        if (command !== 'toggle_screen_recording')
            return;
        try {
            const tab = await getActiveTab();
            if (!tab?.id)
                return;
            trackUIFeature('video');
            await toggleScreenRecording(handlers, tab, logFn);
        }
        catch (err) {
            if (logFn)
                logFn(`Screen recording shortcut error: ${errorMessage(err)}`);
        }
    });
}
//# sourceMappingURL=keyboard-shortcuts.js.map