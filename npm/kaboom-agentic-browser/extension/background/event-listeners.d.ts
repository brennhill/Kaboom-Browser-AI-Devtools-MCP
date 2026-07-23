/**
 * Purpose: Installs Chrome extension event listeners (alarms, tab lifecycle, storage changes, runtime startup) and re-exports keyboard shortcuts, context menus, and tab-state accessors.
 * Docs: docs/features/feature/tab-tracking-ux/index.md
 */
export { installDrawModeCommandListener, installRecordingShortcutCommandListener, installScreenRecordingCommandListener } from './keyboard-shortcuts.js';
export type { RecordingShortcutHandlers, ScreenRecordingHandlers } from './keyboard-shortcuts.js';
export { installContextMenus } from './context-menus.js';
export { pingContentScript, waitForTabLoad, forwardToAllContentScripts, loadSavedSettings, loadAiWebPilotState, loadDebugModeState, saveSetting, getTrackedTabInfo, clearTrackedTab, getActiveTab, sendTabToast } from './tab-state.js';
export type { SavedSettings, TrackedTabInfo } from './tab-state.js';
declare const ALARM_NAMES: {
    readonly RECONNECT: "reconnect";
    readonly ERROR_GROUP_FLUSH: "errorGroupFlush";
    readonly MEMORY_CHECK: "memoryCheck";
    readonly ERROR_GROUP_CLEANUP: "errorGroupCleanup";
};
export type AlarmName = (typeof ALARM_NAMES)[keyof typeof ALARM_NAMES];
/**
 * Setup Chrome alarms for periodic tasks
 *
 * RATE LIMITING & DoS PROTECTION:
 * 1. RECONNECT (5s): Maintains MCP connection with exponential backoff
 * 2. ERROR_GROUP_FLUSH (30s): Deduplicates errors, reduces server load
 * 3. MEMORY_CHECK (30s): Monitors buffer memory, prevents exhaustion
 * 4. ERROR_GROUP_CLEANUP (10min): Removes stale deduplication state
 *
 * Note: Alarms are re-created on service worker startup (not persistent)
 * If service worker restarts, alarms must be recreated by this function
 */
export declare function setupChromeAlarms(): void;
/**
 * Install Chrome alarm listener.
 * Handlers may be async -- the listener awaits them to keep the SW alive
 * until the work completes (prevents badge updates from being lost).
 */
export declare function installAlarmListener(handlers: {
    onReconnect: () => void | Promise<void>;
    onErrorGroupFlush: () => void;
    onMemoryCheck: () => void;
    onErrorGroupCleanup: () => void;
}): void;
/**
 * Install tab removed listener
 */
export declare function installTabRemovedListener(onTabRemoved: (tabId: number) => void): void;
/**
 * Install tab updated listener to track URL changes
 */
export declare function installTabUpdatedListener(onTabUpdated: (tabId: number, newUrl: string) => void): void;
/**
 * Handle tracked tab URL change
 * Updates the stored URL and title when the tracked tab navigates
 */
export declare function handleTrackedTabUrlChange(updatedTabId: number, newUrl: string, logFn?: (message: string) => void): Promise<void>;
/**
 * Handle tracked tab being closed
 * SECURITY: Clears ephemeral tracking state when tab closes
 * Uses session storage for ephemeral tab tracking data
 */
export declare function handleTrackedTabClosed(closedTabId: number, logFn?: (message: string, data?: unknown) => void): Promise<void>;
/**
 * Install storage change listener
 */
export declare function installStorageChangeListener(handlers: {
    onAiWebPilotChanged?: (newValue: boolean) => void;
    onTrackedTabChanged?: (newTabId: number | null, oldTabId: number | null) => void;
}): void;
/**
 * Install browser startup listener (clears tracking state)
 */
export declare function installStartupListener(logFn?: (message: string) => void): void;
//# sourceMappingURL=event-listeners.d.ts.map