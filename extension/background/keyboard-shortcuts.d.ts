/**
 * Purpose: Keyboard shortcut listeners for draw mode, action-sequence recording, and screen recording.
 * Split from event-listeners.ts to keep files under 800 LOC.
 */
export interface RecordingShortcutHandlers {
    isRecording: () => boolean;
    startRecording: (name: string, fps?: number, queryId?: string, audio?: string, fromPopup?: boolean, targetTabId?: number) => Promise<{
        status: string;
        error?: string;
    }>;
    stopRecording: (truncated?: boolean) => Promise<{
        status: string;
        error?: string;
    }>;
}
export declare function buildActionSequenceRecordingName(now?: Date): string;
export interface ScreenRecordingHandlers {
    isRecording: () => boolean;
    startRecording: (name: string, fps?: number, queryId?: string, audio?: string, fromPopup?: boolean, targetTabId?: number) => Promise<{
        status: string;
        name: string;
        startTime?: number;
        error?: string;
    }>;
    stopRecording: (truncated?: boolean) => Promise<{
        status: string;
        name: string;
        duration_seconds?: number;
        size_bytes?: number;
        truncated?: boolean;
        path?: string;
        error?: string;
    }>;
}
export declare function toggleScreenRecording(handlers: ScreenRecordingHandlers, tab: chrome.tabs.Tab, logFn?: (message: string) => void): Promise<void>;
/**
 * Install keyboard shortcut listener for draw mode toggle (Ctrl+Shift+D / Cmd+Shift+D).
 * Sends KABOOM_DRAW_MODE_START or KABOOM_DRAW_MODE_STOP to the active tab's content script.
 */
export declare function installDrawModeCommandListener(logFn?: (message: string) => void): void;
/**
 * Install the keyboard shortcut that opens the terminal side panel
 * (`open_terminal_panel` in the manifest).
 *
 * The command ships UNBOUND on purpose: Chrome refuses to load a manifest with
 * more than four commands carrying a `suggested_key`, and four are already
 * taken. Users assign a key at chrome://extensions/shortcuts; until then the
 * context menu is the zero-setup gesture-native route.
 *
 * Why a command and not just the in-page launcher button: `chrome.sidePanel.open()`
 * needs an active user gesture, and Chrome grants `runtime.onMessage` listeners
 * only a *restricted* gesture that sidePanel.open() rejects on some Chrome/Brave
 * builds (crbug 355266358). `commands.onCommand` gets a full gesture and hands us
 * the active tab synchronously, so this path does not depend on gesture forwarding.
 *
 * Nothing may be awaited before openTerminalSidePanel() — `tab` comes straight
 * from the listener argument precisely so no lookup is needed.
 */
export declare function installTerminalPanelCommandListener(logFn?: (message: string) => void): void;
/**
 * Install keyboard shortcut listener for action-sequence recording toggle.
 * Shortcut is defined in manifest as `toggle_action_sequence_recording`.
 */
export declare function installRecordingShortcutCommandListener(handlers: RecordingShortcutHandlers, logFn?: (message: string) => void): void;
/**
 * Install keyboard shortcut listener for screen recording toggle (Alt+Shift+R).
 */
export declare function installScreenRecordingCommandListener(handlers: ScreenRecordingHandlers, logFn?: (message: string) => void): void;
//# sourceMappingURL=keyboard-shortcuts.d.ts.map