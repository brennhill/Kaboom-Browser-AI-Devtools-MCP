/**
 * Purpose: Shared constants, types, and mutable state for the terminal widget.
 * Why: Centralises state and constants so split modules reference the same values
 *      without circular dependencies.
 * Docs: docs/features/feature/terminal/index.md
 */
export declare const WIDGET_ID = "kaboom-terminal-widget";
export declare const IFRAME_ID = "kaboom-terminal-iframe";
export declare const HEADER_ID = "kaboom-terminal-header";
export declare const TERMINAL_PROVIDER_BADGE_ID = "kaboom-terminal-provider-badge";
export declare const TERMINAL_BODY_ID = "kaboom-terminal-body";
export declare const DISCONNECT_TERMINAL_BUTTON_ID = "kaboom-terminal-disconnect-button";
export declare const ANNOTATE_TERMINAL_BUTTON_ID = "kaboom-terminal-annotate-button";
export declare const REDRAW_TERMINAL_BUTTON_ID = "kaboom-terminal-redraw-button";
export declare const MINIMIZE_TERMINAL_BUTTON_ID = "kaboom-terminal-minimize-button";
export declare const CLOSE_TERMINAL_BUTTON_ID = "kaboom-terminal-close-button";
export declare const START_TERMINAL_BUTTON_ID = "kaboom-terminal-start-button";
export declare const ROOT_FOLDER_INPUT_ID = "kaboom-terminal-root-folder-input";
export declare const ROOT_FOLDER_SAVE_BUTTON_ID = "kaboom-terminal-root-folder-save";
export declare const ROOT_FOLDER_BAR_ID = "kaboom-terminal-root-folder-bar";
export declare const ROOT_FOLDER_BROWSE_BUTTON_ID = "kaboom-terminal-root-folder-browse";
export declare const ROOT_FOLDER_PICKER_ID = "kaboom-terminal-root-folder-picker";
export declare const ROOT_FOLDER_PICKER_UP_ID = "kaboom-terminal-root-folder-up";
export declare const ROOT_FOLDER_PICKER_USE_ID = "kaboom-terminal-root-folder-use";
export declare const TERMINAL_WRITE_SUBMIT_DELAY_MS = 600;
export declare const TERMINAL_TYPING_IDLE_MS = 1500;
export declare const TERMINAL_GUARD_POLL_MS = 200;
export declare const TERMINAL_GUARD_TOAST_INTERVAL_MS = 3000;
/** Maximum number of agent writes held while the terminal is unreachable. */
export declare const MAX_QUEUED_WRITES = 200;
/**
 * Maximum total SIZE of that backlog, in UTF-8 bytes.
 *
 * The entry count alone is not a bound on anything that matters: 200 one-megabyte
 * writes is a legal state under it, i.e. ~200 MB pinned in the side panel with
 * nothing to stop it (finding S14). Writes are only queued while the socket is
 * down, so this also mirrors the daemon's own 1 MB PTY write-buffer cap — more
 * than this could never be delivered in one go anyway.
 */
export declare const MAX_QUEUED_WRITE_BYTES: number;
export declare const TERMINAL_RECONNECT_BASE_DELAY_MS = 1000;
export declare const TERMINAL_RECONNECT_MAX_DELAY_MS = 10000;
export declare const TERMINAL_MAX_RECONNECT_ATTEMPTS = 6;
export declare const TERMINAL_RECONNECT_JITTER_RATIO = 0.25;
/**
 * WORST-CASE wall-clock time from the first disconnect until the iframe gives up
 * and posts `reconnect_exhausted` (which is what triggers the parent's
 * validate-and-rebuild recovery). The iframe waits before EVERY attempt, including
 * the one that trips the cap — the `reconnectAttempts > MAX_RECONNECT_ATTEMPTS`
 * check runs after the increment, inside the timer — so there are MAX+1 waits:
 * 1+2+4+8+10+10+10 = 45s, and up to 25% more once jitter is applied.
 *
 * Worst case, not average, is the right number here: the write-guard budget must
 * cover the slowest run, or it goes back to dropping the queue early.
 */
export declare function terminalReconnectExhaustionMs(): number;
export declare const TERMINAL_GUARD_RECOVERY_GRACE_MS = 10000;
export declare const TERMINAL_GUARD_MAX_WAIT_MS: number;
export interface TerminalConfig {
    cmd?: string;
    args?: string[];
    dir?: string;
    serverUrl?: string;
}
export interface TerminalSessionState {
    token: string;
    sessionId: string;
}
export type TerminalUIState = 'open' | 'closed' | 'minimized';
export interface TerminalWidgetState {
    widgetEl: HTMLDivElement | null;
    iframeEl: HTMLIFrameElement | null;
    sessionState: TerminalSessionState | null;
    visible: boolean;
    serverUrl: string;
    terminalFocused: boolean;
    lastTypingAt: number;
    queuedWrites: string[];
    queuedWriteFlushTimer: ReturnType<typeof setTimeout> | null;
    queuedSubmitTimer: ReturnType<typeof setTimeout> | null;
    queuedWriteInFlight: boolean;
    lastGuardToastAt: number;
    terminalConnected: boolean;
    guardBlockedSince: number;
}
export declare const state: TerminalWidgetState;
/** Reset all mutable state to initial values. Used by tests to isolate module-cached state. */
export declare function resetAllState(): void;
//# sourceMappingURL=terminal-widget-types.d.ts.map