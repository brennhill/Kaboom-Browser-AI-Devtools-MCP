/**
 * Purpose: Tab-state accessors, settings persistence, and content-script helpers.
 * Split from event-listeners.ts to keep files under 800 LOC.
 */
export { setTrackedTab, clearTrackedTab, TRACKED_TAB_STORAGE_KEYS } from '../lib/tabs/tracked-tab-storage.js';
/**
 * Ping content script to check if it's loaded
 */
export declare function pingContentScript(tabId: number, timeoutMs?: number): Promise<boolean>;
/**
 * Wait for tab to finish loading
 */
export declare function waitForTabLoad(tabId: number, timeoutMs?: number): Promise<boolean>;
/**
 * Forward a message to all content scripts
 */
export declare function forwardToAllContentScripts(message: {
    type: string;
    [key: string]: unknown;
}, debugLogFn?: (category: string, message: string, data?: unknown) => void): Promise<void>;
/** Settings returned by loadSavedSettings */
export interface SavedSettings {
    serverUrl?: string;
    logLevel?: string;
    screenshotOnError?: boolean;
    sourceMapEnabled?: boolean;
    debugMode?: boolean;
}
/**
 * Load saved settings from chrome.storage.local
 */
export declare function loadSavedSettings(): Promise<SavedSettings>;
/**
 * Load AI Web Pilot enabled state from storage
 */
export declare function loadAiWebPilotState(logFn?: (message: string) => void): Promise<boolean>;
/**
 * Load debug mode state from storage
 */
export declare function loadDebugModeState(): Promise<boolean>;
/**
 * Save setting to chrome.storage.local
 */
export declare function saveSetting(key: string, value: unknown): void;
/** Tracked tab info type */
export interface TrackedTabInfo {
    trackedTabId: number | null;
    trackedTabUrl: string | null;
    trackedTabTitle: string | null;
    tabStatus: 'loading' | 'complete' | null;
    trackedTabActive: boolean | null;
}
export interface TerminalWorkspaceTarget {
    hostTabId: number;
    mainTabId: number;
    tabGroupId: number;
}
/**
 * Get tracked tab information, including Chrome tab status.
 */
export declare function getTrackedTabInfo(): Promise<TrackedTabInfo>;
export declare function resolveTerminalWorkspaceTarget(requestTabId?: number): Promise<TerminalWorkspaceTarget | null>;
/**
 * Get all extension config settings.
 */
export declare function getAllConfigSettings(): Promise<Record<string, boolean | string | undefined>>;
/**
 * Query for the currently active tab in the current window.
 * Returns null if no active tab or no tab id.
 */
export declare function getActiveTab(): Promise<chrome.tabs.Tab | null>;
/**
 * Capture a screenshot of a tab without permanently stealing focus.
 * chrome.tabs.captureVisibleTab() requires the tab to be active. If the target
 * tab isn't currently active, we briefly activate it, capture, then restore
 * the previously active tab so the user's workflow isn't interrupted.
 */
export declare function captureVisibleTabSafe(tabId: number, windowId: number, options: {
    format: 'jpeg' | 'png';
    quality?: number;
}): Promise<string>;
/**
 * Toggles visibility of all Kaboom UI overlays (hover launcher, draw mode)
 * in the target tab. Uses executeScript for speed — no message round-trip needed.
 */
export declare function setKaboomOverlayVisibility(tabId: number, visible: boolean): Promise<void>;
/**
 * Send a kaboom_action_toast message to a tab.
 * Silently ignores errors (content script may not be loaded).
 */
export declare function sendTabToast(tabId: number, text: string, detail?: string, state?: 'trying' | 'success' | 'warning' | 'error' | 'audio', duration_ms?: number): void;
//# sourceMappingURL=tab-state.d.ts.map