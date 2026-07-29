/**
 * Purpose: Typing-aware write queue for the terminal — defers agent writes while
 * the user is typing, and holds submits until the socket is back.
 * Why: Text injected mid-keystroke corrupts what the user was writing, and an
 * Enter sent while disconnected is simply lost.
 * Docs: docs/features/feature/terminal/index.md
 */
import { showActionToast } from './toast.js';
import { getTerminalServerUrl } from '../../lib/terminal-server.js';
import { state, TERMINAL_WRITE_SUBMIT_DELAY_MS, TERMINAL_TYPING_IDLE_MS, TERMINAL_GUARD_POLL_MS, TERMINAL_GUARD_TOAST_INTERVAL_MS, TERMINAL_GUARD_MAX_WAIT_MS, MAX_QUEUED_WRITES, MAX_QUEUED_WRITE_BYTES } from './terminal-widget-types.js';
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
/**
 * UTF-8 byte length of a string, counted without allocating an encoded copy.
 * TextEncoder().encode(s).length would materialise a second megabyte-sized buffer
 * for exactly the writes this bound exists to reject.
 */
function utf8ByteLength(text) {
    let bytes = 0;
    for (let i = 0; i < text.length; i++) {
        const code = text.charCodeAt(i);
        if (code < 0x80)
            bytes += 1;
        else if (code < 0x800)
            bytes += 2;
        else if (code >= 0xd800 && code <= 0xdbff && i + 1 < text.length) {
            bytes += 4; // surrogate pair -> one 4-byte code point
            i++;
        }
        else
            bytes += 3;
    }
    return bytes;
}
/**
 * Enqueue a write, bounding the backlog by BOTH entry count and total bytes.
 *
 * The count cap alone bounded nothing that matters — 200 one-megabyte writes was a
 * legal state (finding S14). Eviction is oldest-first and runs down to empty, so a
 * single write larger than the byte cap is evicted like any other rather than
 * lodging in the queue forever.
 *
 * Dropping a write is a state-mutating loss, so it must not be silent (rule 25):
 * warn to the console (which the daemon captures via observe(what:"errors")) so an
 * overflow is diagnosable rather than a write vanishing without a trace.
 *
 * Lives here, with the rest of the queue's lifecycle (reset, drain, drop-on-give-up),
 * so the bound cannot be bypassed by a second enqueue site (rule 19).
 */
export function enqueueBoundedWrite(text) {
    state.queuedWrites.push(text);
    while (state.queuedWrites.length > MAX_QUEUED_WRITES) {
        const dropped = state.queuedWrites.shift();
        console.warn(`[KaBOOM! terminal] write queue full (${MAX_QUEUED_WRITES} entries) — dropped oldest queued write: "${(dropped ?? '').slice(0, 40)}"`);
    }
    let queuedBytes = 0;
    for (const queued of state.queuedWrites)
        queuedBytes += utf8ByteLength(queued);
    while (queuedBytes > MAX_QUEUED_WRITE_BYTES && state.queuedWrites.length > 0) {
        const dropped = state.queuedWrites.shift() ?? '';
        queuedBytes -= utf8ByteLength(dropped);
        console.warn(`[KaBOOM! terminal] write queue over ${MAX_QUEUED_WRITE_BYTES} bytes — dropped oldest queued write (${utf8ByteLength(dropped)} bytes): "${dropped.slice(0, 40)}"`);
    }
}
export function resetWriteGuardState() {
    state.queuedWrites = [];
    state.terminalFocused = false;
    state.lastTypingAt = 0;
    state.queuedWriteInFlight = false;
    state.lastGuardToastAt = 0;
    state.guardBlockedSince = 0;
    if (state.queuedWriteFlushTimer !== null) {
        clearTimeout(state.queuedWriteFlushTimer);
        state.queuedWriteFlushTimer = null;
    }
    if (state.queuedSubmitTimer !== null) {
        clearTimeout(state.queuedSubmitTimer);
        state.queuedSubmitTimer = null;
    }
}
/**
 * Mark the guard as blocked (waiting for the socket or for typing to stop) if it
 * is not already, stamping when the wait began. Idempotent while blocked so the
 * escape-hatch deadline measures the *total* stuck time, not the last poll.
 */
function markGuardBlocked(nowMs) {
    if (state.guardBlockedSince === 0)
        state.guardBlockedSince = nowMs;
}
/**
 * Escape hatch for the genuine wedge: the socket has stayed DOWN
 * (`!terminalConnected`) for longer than TERMINAL_GUARD_MAX_WAIT_MS, so give up
 * LOUDLY instead of polling forever. Only ever called from the socket-down
 * branches — the "terminal not reachable" message must be TRUE. The typing-defer
 * branches are self-limiting and reset guardBlockedSince instead of tripping
 * this, so `guardBlockedSince` measures contiguous unreachable time only.
 * Returns true if it gave up — callers must then stop.
 */
function guardGaveUpAfterMaxWait(nowMs) {
    if (state.guardBlockedSince === 0)
        return false;
    if (nowMs - state.guardBlockedSince < TERMINAL_GUARD_MAX_WAIT_MS)
        return false;
    const dropped = state.queuedWrites.length + (state.queuedWriteInFlight ? 1 : 0);
    const noun = dropped === 1 ? 'action' : 'actions';
    showActionToast(`terminal not reachable — dropped ${dropped} queued ${noun}`, 'Terminal write gave up', 'error', 4000);
    resetWriteGuardState();
    return true;
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
        const now = Date.now();
        if (!state.terminalConnected) {
            if (guardGaveUpAfterMaxWait(now))
                return;
            markGuardBlocked(now);
            scheduleQueuedSubmit(TERMINAL_GUARD_POLL_MS);
            return;
        }
        if (shouldDeferQueuedWrite(now)) {
            // The terminal is CONNECTED — the only reason we are waiting is that the
            // user is typing, which is self-limiting (it clears once typing stops).
            // Do NOT accrue unreachable-time or trip the escape hatch here: doing so
            // dropped a healthy write after 30s of continuous typing and told the user
            // "terminal not reachable", which was false. Reset the unreachable clock so
            // it only ever measures contiguous socket-down time, then defer politely.
            state.guardBlockedSince = 0;
            maybeShowQueuedWriteToast(now);
            scheduleQueuedSubmit(TERMINAL_GUARD_POLL_MS);
            return;
        }
        notifyIframe('write', { text: '\r' });
        notifyIframe('focus');
        state.queuedWriteInFlight = false;
        state.guardBlockedSince = 0; // Enter delivered — progress made.
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
    const now = Date.now();
    if (!state.terminalConnected) {
        if (guardGaveUpAfterMaxWait(now))
            return;
        markGuardBlocked(now);
        scheduleQueuedWriteFlush(TERMINAL_GUARD_POLL_MS);
        return;
    }
    if (state.queuedWriteInFlight)
        return;
    if (state.queuedWrites.length === 0) {
        state.lastGuardToastAt = 0;
        state.guardBlockedSince = 0;
        return;
    }
    if (shouldDeferQueuedWrite(now)) {
        // Connected — waiting only on the user to stop typing (self-limiting). Do NOT
        // trip the escape hatch here; that dropped healthy writes during continuous
        // typing and falsely reported "terminal not reachable". Reset the
        // unreachable clock (it tracks socket-down time only) and defer politely.
        state.guardBlockedSince = 0;
        maybeShowQueuedWriteToast(now);
        scheduleQueuedWriteFlush(TERMINAL_GUARD_POLL_MS);
        return;
    }
    const nextWrite = state.queuedWrites.shift();
    if (!nextWrite)
        return;
    state.lastGuardToastAt = 0;
    state.guardBlockedSince = 0; // Write dispatched — progress made.
    state.queuedWriteInFlight = true;
    notifyIframe('redraw');
    notifyIframe('write', { text: nextWrite });
    scheduleQueuedSubmit(TERMINAL_WRITE_SUBMIT_DELAY_MS);
}
//# sourceMappingURL=terminal-write-guard.js.map