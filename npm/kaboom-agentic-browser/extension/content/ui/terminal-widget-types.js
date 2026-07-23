/**
 * Purpose: Shared constants, types, and mutable state for the terminal widget.
 * Why: Centralises state and constants so split modules reference the same values
 *      without circular dependencies.
 * Docs: docs/features/feature/terminal/index.md
 */
import { DEFAULT_SERVER_URL, TERMINAL_PORT_OFFSET } from '../../lib/constants.js';
// ---------------------------------------------------------------------------
// DOM element IDs
// ---------------------------------------------------------------------------
export const WIDGET_ID = 'kaboom-terminal-widget';
export const IFRAME_ID = 'kaboom-terminal-iframe';
export const HEADER_ID = 'kaboom-terminal-header';
export const DISCONNECT_TERMINAL_BUTTON_ID = 'kaboom-terminal-disconnect-button';
export const REDRAW_TERMINAL_BUTTON_ID = 'kaboom-terminal-redraw-button';
export const MINIMIZE_TERMINAL_BUTTON_ID = 'kaboom-terminal-minimize-button';
export const CLOSE_TERMINAL_BUTTON_ID = 'kaboom-terminal-close-button';
// ---------------------------------------------------------------------------
// Layout defaults
// ---------------------------------------------------------------------------
export const DEFAULT_WIDGET_WIDTH = '50vw';
export const DEFAULT_WIDGET_HEIGHT = '40vh';
export const MIN_WIDGET_WIDTH = '400px';
export const MIN_WIDGET_HEIGHT = '250px';
export const MAX_WIDGET_WIDTH = '100vw';
export const MAX_WIDGET_HEIGHT = '80vh';
export const MINIMIZED_WIDGET_HEIGHT = '32px';
// ---------------------------------------------------------------------------
// Timing constants
// ---------------------------------------------------------------------------
export const TERMINAL_WRITE_SUBMIT_DELAY_MS = 600;
export const TERMINAL_TYPING_IDLE_MS = 1500;
export const TERMINAL_GUARD_POLL_MS = 200;
export const TERMINAL_GUARD_TOAST_INTERVAL_MS = 3000;
export const state = {
    widgetEl: null,
    iframeEl: null,
    resizeHandleEl: null,
    sessionState: null,
    visible: false,
    minimized: false,
    savedHeight: '',
    serverUrl: DEFAULT_SERVER_URL,
    terminalFocused: false,
    lastTypingAt: 0,
    queuedWrites: [],
    queuedWriteFlushTimer: null,
    queuedSubmitTimer: null,
    queuedWriteInFlight: false,
    lastGuardToastAt: 0,
    terminalConnected: false
};
/** Reset all mutable state to initial values. Used by tests to isolate module-cached state. */
export function resetAllState() {
    state.widgetEl = null;
    state.iframeEl = null;
    state.resizeHandleEl = null;
    state.sessionState = null;
    state.visible = false;
    state.minimized = false;
    state.savedHeight = '';
    state.serverUrl = DEFAULT_SERVER_URL;
    state.terminalFocused = false;
    state.lastTypingAt = 0;
    state.queuedWrites = [];
    if (state.queuedWriteFlushTimer !== null)
        clearTimeout(state.queuedWriteFlushTimer);
    state.queuedWriteFlushTimer = null;
    if (state.queuedSubmitTimer !== null)
        clearTimeout(state.queuedSubmitTimer);
    state.queuedSubmitTimer = null;
    state.queuedWriteInFlight = false;
    state.lastGuardToastAt = 0;
    state.terminalConnected = false;
}
// ---------------------------------------------------------------------------
// Utility: compute terminal server URL from a base daemon URL.
// ---------------------------------------------------------------------------
/** Compute the terminal server URL from a base daemon URL (port + TERMINAL_PORT_OFFSET). */
export function getTerminalServerUrl(baseUrl) {
    const url = new URL(baseUrl);
    url.port = String(parseInt(url.port || '7890', 10) + TERMINAL_PORT_OFFSET);
    return url.origin;
}
//# sourceMappingURL=terminal-widget-types.js.map