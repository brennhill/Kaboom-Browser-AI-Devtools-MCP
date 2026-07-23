/**
 * Purpose: Shared constants, types, and mutable state for the terminal widget.
 * Why: Centralises state and constants so split modules reference the same values
 *      without circular dependencies.
 * Docs: docs/features/feature/terminal/index.md
 */
export declare const WIDGET_ID = "kaboom-terminal-widget";
export declare const IFRAME_ID = "kaboom-terminal-iframe";
export declare const HEADER_ID = "kaboom-terminal-header";
export declare const DISCONNECT_TERMINAL_BUTTON_ID = "kaboom-terminal-disconnect-button";
export declare const REDRAW_TERMINAL_BUTTON_ID = "kaboom-terminal-redraw-button";
export declare const MINIMIZE_TERMINAL_BUTTON_ID = "kaboom-terminal-minimize-button";
export declare const CLOSE_TERMINAL_BUTTON_ID = "kaboom-terminal-close-button";
export declare const DEFAULT_WIDGET_WIDTH = "50vw";
export declare const DEFAULT_WIDGET_HEIGHT = "40vh";
export declare const MIN_WIDGET_WIDTH = "400px";
export declare const MIN_WIDGET_HEIGHT = "250px";
export declare const MAX_WIDGET_WIDTH = "100vw";
export declare const MAX_WIDGET_HEIGHT = "80vh";
export declare const MINIMIZED_WIDGET_HEIGHT = "32px";
export declare const TERMINAL_WRITE_SUBMIT_DELAY_MS = 600;
export declare const TERMINAL_TYPING_IDLE_MS = 1500;
export declare const TERMINAL_GUARD_POLL_MS = 200;
export declare const TERMINAL_GUARD_TOAST_INTERVAL_MS = 3000;
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
    resizeHandleEl: HTMLDivElement | null;
    sessionState: TerminalSessionState | null;
    visible: boolean;
    minimized: boolean;
    savedHeight: string;
    serverUrl: string;
    terminalFocused: boolean;
    lastTypingAt: number;
    queuedWrites: string[];
    queuedWriteFlushTimer: ReturnType<typeof setTimeout> | null;
    queuedSubmitTimer: ReturnType<typeof setTimeout> | null;
    queuedWriteInFlight: boolean;
    lastGuardToastAt: number;
    terminalConnected: boolean;
}
export declare const state: TerminalWidgetState;
/** Reset all mutable state to initial values. Used by tests to isolate module-cached state. */
export declare function resetAllState(): void;
/** Compute the terminal server URL from a base daemon URL (port + TERMINAL_PORT_OFFSET). */
export declare function getTerminalServerUrl(baseUrl: string): string;
//# sourceMappingURL=terminal-widget-types.d.ts.map