/**
 * Purpose: Shared constants, types, and mutable state for the terminal widget.
 * Why: Centralises state and constants so split modules reference the same values
 *      without circular dependencies.
 * Docs: docs/features/feature/terminal/index.md
 */
export declare const WIDGET_ID = "kaboom-terminal-widget";
export declare const IFRAME_ID = "kaboom-terminal-iframe";
export declare const HEADER_ID = "kaboom-terminal-header";
export declare const TERMINAL_BODY_ID = "kaboom-terminal-body";
export declare const DISCONNECT_TERMINAL_BUTTON_ID = "kaboom-terminal-disconnect-button";
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
}
export declare const state: TerminalWidgetState;
/** Reset all mutable state to initial values. Used by tests to isolate module-cached state. */
export declare function resetAllState(): void;
/** Compute the terminal server URL from a base daemon URL (port + TERMINAL_PORT_OFFSET). */
export declare function getTerminalServerUrl(baseUrl: string): string;
//# sourceMappingURL=terminal-widget-types.d.ts.map