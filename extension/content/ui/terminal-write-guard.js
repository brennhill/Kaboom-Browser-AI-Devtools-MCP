/**
 * Purpose: Typing-aware write queue for the terminal — defers agent writes while
 * the user is typing, and holds submits until the socket is back.
 * Why: Text injected mid-keystroke corrupts what the user was writing, and an
 * Enter sent while disconnected is simply lost.
 * Docs: docs/features/feature/terminal/index.md
 */
import { showActionToast } from './toast.js';
import { state, getTerminalServerUrl, TERMINAL_WRITE_SUBMIT_DELAY_MS, TERMINAL_TYPING_IDLE_MS, TERMINAL_GUARD_POLL_MS, TERMINAL_GUARD_TOAST_INTERVAL_MS } from './terminal-widget-types.js';
/**
 * Post a command to the terminal iframe. No-op when it is not mounted.
 *
 * `target: 'kaboom-terminal'` is MANDATORY: terminal.html's message listener
 * drops any message whose `target` is not exactly that. The eb248ff6 refactor
 * dropped this field, so every agent/annotation write, focus, and redraw was
 * silently discarded — user keystrokes hid the gap because they go straight to
 * the socket (iframe -> WS), never through here. Post to the terminal server's
 * own origin so the message can't leak to a swapped-in frame; fall back to '*'
 * only when the URL is unparseable.
 */
export function notifyIframe(command, data = {}) {
    if (!state.iframeEl?.contentWindow)
        return;
    let origin = '*';
    try {
        origin = getTerminalServerUrl(state.serverUrl);
    }
    catch {
        // Unparseable server URL — fall back to wildcard so the write still lands.
    }
    state.iframeEl.contentWindow.postMessage({ target: 'kaboom-terminal', command, ...data }, origin);
}
export function resetWriteGuardState() {
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
export function shouldDeferQueuedWrite(nowMs = Date.now()) {
    if (!state.terminalFocused)
        return false;
    return nowMs - state.lastTypingAt < TERMINAL_TYPING_IDLE_MS;
}
export function maybeShowQueuedWriteToast(nowMs = Date.now()) {
    if (nowMs - state.lastGuardToastAt < TERMINAL_GUARD_TOAST_INTERVAL_MS)
        return;
    state.lastGuardToastAt = nowMs;
    showActionToast('waiting for user to stop typing', 'Queued terminal action', 'warning', 1800);
}
export function scheduleQueuedWriteFlush(delayMs = 0) {
    if (state.queuedWriteFlushTimer !== null)
        clearTimeout(state.queuedWriteFlushTimer);
    state.queuedWriteFlushTimer = setTimeout(() => {
        state.queuedWriteFlushTimer = null;
        flushQueuedWrites();
    }, delayMs);
}
export function scheduleQueuedSubmit(delayMs) {
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
export function flushQueuedWrites() {
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
//# sourceMappingURL=terminal-write-guard.js.map