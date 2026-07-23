/**
 * Purpose: In-browser terminal widget orchestrator — public API, write guard, mount/unmount.
 * Why: Provides a Lovable-like experience — chat with any CLI (claude, codex, aider) from
 * a browser overlay while seeing code edits reflected via hot reload on the tracked page.
 * Docs: docs/features/feature/terminal/index.md
 */
import { showActionToast } from './toast.js';
import { state, resetAllState, getTerminalServerUrl, HEADER_ID, MINIMIZE_TERMINAL_BUTTON_ID, TERMINAL_WRITE_SUBMIT_DELAY_MS, TERMINAL_TYPING_IDLE_MS, TERMINAL_GUARD_POLL_MS, TERMINAL_GUARD_TOAST_INTERVAL_MS } from './terminal-widget-types.js';
import { getServerUrl, getTerminalConfig, persistUIState, loadPersistedSession, clearPersistedSession, validateSession, startSession } from './terminal-widget-session.js';
import { registerUICallbacks, createWidget, handleIframeMessage, notifyIframe, toggleMinimize } from './terminal-widget-ui.js';
// =============================================================================
// WRITE GUARD — defer queued writes while user is typing in the terminal
// =============================================================================
function resetWriteGuardState() {
    state.queuedWrites = [];
    state.terminalFocused = false;
    state.lastTypingAt = 0;
    state.queuedWriteInFlight = false;
    state.lastGuardToastAt = 0;
    if (state.queuedWriteFlushTimer !== null) {
        clearTimeout(state.queuedWriteFlushTimer);
        state.queuedWriteFlushTimer = null;
    }
    if (state.queuedSubmitTimer !== null) {
        clearTimeout(state.queuedSubmitTimer);
        state.queuedSubmitTimer = null;
    }
}
function shouldDeferQueuedWrite(nowMs = Date.now()) {
    if (!state.terminalFocused)
        return false;
    return nowMs - state.lastTypingAt < TERMINAL_TYPING_IDLE_MS;
}
function maybeShowQueuedWriteToast(nowMs = Date.now()) {
    if (nowMs - state.lastGuardToastAt < TERMINAL_GUARD_TOAST_INTERVAL_MS)
        return;
    state.lastGuardToastAt = nowMs;
    showActionToast('waiting for user to stop typing', 'Queued terminal action', 'warning', 1800);
}
function scheduleQueuedWriteFlush(delayMs = 0) {
    if (state.queuedWriteFlushTimer !== null)
        clearTimeout(state.queuedWriteFlushTimer);
    state.queuedWriteFlushTimer = setTimeout(() => {
        state.queuedWriteFlushTimer = null;
        flushQueuedWrites();
    }, delayMs);
}
function scheduleQueuedSubmit(delayMs) {
    if (state.queuedSubmitTimer !== null)
        clearTimeout(state.queuedSubmitTimer);
    state.queuedSubmitTimer = setTimeout(() => {
        state.queuedSubmitTimer = null;
        if (!state.visible || !state.iframeEl) {
            resetWriteGuardState();
            return;
        }
        if (!state.terminalConnected) {
            scheduleQueuedSubmit(TERMINAL_GUARD_POLL_MS);
            return;
        }
        if (shouldDeferQueuedWrite()) {
            maybeShowQueuedWriteToast();
            scheduleQueuedSubmit(TERMINAL_GUARD_POLL_MS);
            return;
        }
        notifyIframe('write', { text: '\r' });
        notifyIframe('focus');
        state.queuedWriteInFlight = false;
        if (state.queuedWrites.length > 0) {
            scheduleQueuedWriteFlush(0);
        }
    }, delayMs);
}
function flushQueuedWrites() {
    if (!state.visible || !state.iframeEl) {
        resetWriteGuardState();
        return;
    }
    if (!state.terminalConnected) {
        scheduleQueuedWriteFlush(TERMINAL_GUARD_POLL_MS);
        return;
    }
    if (state.queuedWriteInFlight)
        return;
    if (state.queuedWrites.length === 0) {
        state.lastGuardToastAt = 0;
        return;
    }
    if (shouldDeferQueuedWrite()) {
        maybeShowQueuedWriteToast();
        scheduleQueuedWriteFlush(TERMINAL_GUARD_POLL_MS);
        return;
    }
    const nextWrite = state.queuedWrites.shift();
    if (!nextWrite)
        return;
    state.lastGuardToastAt = 0;
    state.queuedWriteInFlight = true;
    notifyIframe('redraw');
    notifyIframe('write', { text: nextWrite });
    scheduleQueuedSubmit(TERMINAL_WRITE_SUBMIT_DELAY_MS);
}
// =============================================================================
// PUBLIC API
// =============================================================================
function hideTerminal() {
    if (!state.widgetEl)
        return;
    state.visible = false;
    state.widgetEl.style.opacity = '0';
    state.widgetEl.style.transform = 'translateY(20px) scale(0.98)';
    state.widgetEl.style.pointerEvents = 'none';
    resetWriteGuardState();
    persistUIState('closed');
    // Session stays alive — can reconnect via toggle or page reload
}
/** Kill the PTY session on the daemon and tear down the widget completely. */
async function exitTerminalSession() {
    // Stop the PTY on the daemon (with timeout so the UI never hangs).
    if (state.sessionState) {
        try {
            const termUrl = getTerminalServerUrl(state.serverUrl);
            await fetch(`${termUrl}/terminal/stop`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ id: state.sessionState.sessionId }),
                signal: AbortSignal.timeout(3000)
            });
        }
        catch { /* daemon unreachable or timeout — tear down locally */ }
    }
    clearPersistedSession();
    unmountTerminal();
}
function showTerminal() {
    if (!state.widgetEl)
        return;
    state.visible = true;
    state.widgetEl.style.opacity = '1';
    state.widgetEl.style.transform = 'translateY(0) scale(1)';
    state.widgetEl.style.pointerEvents = 'auto';
    notifyIframe('focus');
    persistUIState(state.minimized ? 'minimized' : 'open');
}
export function isTerminalVisible() {
    return state.visible;
}
export async function toggleTerminal() {
    if (state.visible && state.widgetEl) {
        hideTerminal();
        return;
    }
    // If widget exists but hidden, just show it
    if (state.widgetEl && state.sessionState) {
        showTerminal();
        return;
    }
    // Try to reconnect to a persisted session first
    await getServerUrl();
    const persisted = await loadPersistedSession();
    if (persisted.session) {
        const alive = await validateSession(persisted.session.token);
        if (alive) {
            state.sessionState = persisted.session;
            mountWidget(persisted.session.token, persisted.uiState === 'minimized');
            return;
        }
        // Session died — clear stale state and start fresh
        clearPersistedSession();
    }
    // Start a new session
    const config = await getTerminalConfig();
    const ss = await startSession(config);
    if (!ss)
        return;
    state.sessionState = ss;
    mountWidget(ss.token, false);
}
/** Restore terminal on page load if it was previously open/minimized. */
export async function restoreTerminalIfNeeded() {
    const persisted = await loadPersistedSession();
    if (!persisted.session || persisted.uiState === 'closed')
        return;
    await getServerUrl();
    const alive = await validateSession(persisted.session.token);
    if (!alive) {
        // Session died (daemon restart, process exited) but UI was open — start fresh.
        clearPersistedSession();
        const config = await getTerminalConfig();
        const ss = await startSession(config);
        if (!ss)
            return;
        state.sessionState = ss;
        mountWidget(ss.token, persisted.uiState === 'minimized');
        return;
    }
    state.sessionState = persisted.session;
    mountWidget(persisted.session.token, persisted.uiState === 'minimized');
}
/** Write text to the terminal PTY stdin via the iframe postMessage bridge, then press Enter to submit. */
const MAX_QUEUED_WRITES = 200;
export function writeToTerminal(text) {
    if (!state.visible || !state.iframeEl)
        return;
    // Strip trailing whitespace/newlines — we'll send our own Enter to submit.
    const trimmed = text.replace(/[\r\n\s]+$/, '');
    if (!trimmed)
        return;
    state.queuedWrites.push(trimmed);
    if (state.queuedWrites.length > MAX_QUEUED_WRITES) {
        state.queuedWrites = state.queuedWrites.slice(-MAX_QUEUED_WRITES);
    }
    scheduleQueuedWriteFlush(0);
}
// =============================================================================
// MOUNT / UNMOUNT
// =============================================================================
function mountWidget(token, startMinimized) {
    if (state.widgetEl) {
        state.widgetEl.remove();
        state.widgetEl = null;
    }
    state.widgetEl = createWidget(token);
    const target = document.body || document.documentElement;
    if (!target)
        return;
    target.appendChild(state.widgetEl);
    // Animate in
    state.widgetEl.style.opacity = '0';
    state.widgetEl.style.transform = 'translateY(20px) scale(0.98)';
    requestAnimationFrame(() => {
        showTerminal();
        // Apply minimized state after show animation
        if (startMinimized) {
            const header = state.widgetEl?.querySelector('#' + HEADER_ID);
            const minimizeTerminalButton = header?.querySelector('#' + MINIMIZE_TERMINAL_BUTTON_ID);
            if (state.widgetEl && header && minimizeTerminalButton) {
                toggleMinimize(state.widgetEl, minimizeTerminalButton, header);
            }
        }
    });
}
function unmountTerminal() {
    window.removeEventListener('message', handleIframeMessage);
    resetWriteGuardState();
    state.terminalConnected = false;
    if (state.widgetEl) {
        state.widgetEl.remove();
        state.widgetEl = null;
    }
    state.iframeEl = null;
    state.resizeHandleEl = null;
    state.sessionState = null;
    state.visible = false;
    state.minimized = false;
    state.savedHeight = '';
}
// =============================================================================
// REGISTER UI CALLBACKS — wire up functions that the UI module needs from here.
// =============================================================================
registerUICallbacks({
    hideTerminal,
    exitTerminalSession,
    resetWriteGuardState,
    scheduleQueuedWriteFlush
});
/** Reset all shared state — test-only. Needed because split sub-modules are cached by Node ESM. */
function _resetForTesting() {
    window.removeEventListener('message', handleIframeMessage);
    resetAllState();
}
//# sourceMappingURL=terminal-widget.js.map