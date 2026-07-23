/**
 * Purpose: The one place that opens the terminal side panel.
 * Why: `chrome.sidePanel.open()` is gesture-restricted, and the rules for keeping
 * a gesture alive are subtle enough that every entry point must share one
 * implementation (repo rule 19).
 * Docs: docs/features/feature/terminal/index.md
 */
export { syncTerminalPanelAvailability } from './side-panel-availability.js';
export declare function isTerminalPanelOpenSync(): boolean;
/**
 * Hydrate the cache and keep it current. Call once during background init.
 *
 * A service worker restart resets the flag, so it is re-read on boot; that read
 * happens long before any click, so it never sits inside a gesture.
 */
export declare function watchTerminalPanelState(): void;
/**
 * Ask the panel to close itself.
 *
 * The background cannot close a side panel document directly on every Chrome
 * version, but the panel can (`window.close()`), so we message it. Closing this
 * way keeps the shell running — reopening reconnects to the same session.
 */
export declare function closeTerminalSidePanel(): Promise<{
    success: boolean;
    error?: string;
}>;
/**
 * Toggle the panel. One helper so the context menu, keyboard command, and any
 * future entry point cannot drift apart (repo rule 19).
 *
 * NOT async on the open path: it reads the cached state synchronously and calls
 * openTerminalSidePanel() with zero awaits before chrome.sidePanel.open(), so
 * the caller's user gesture survives.
 */
export declare function toggleTerminalSidePanel(tabId: number | undefined): Promise<{
    success: boolean;
    error?: string;
}>;
/**
 * Open the terminal side panel on `tabId`.
 *
 * GESTURE CONTRACT — nothing may be awaited before `chrome.sidePanel.open()`.
 * Chrome requires an active user gesture, and any await first expires it.
 *
 * Caller matters as much as the code here. Chrome grants a *restricted* gesture
 * to `runtime.onMessage` listeners, and `sidePanel.open()` rejects it on some
 * Chrome/Brave builds (crbug 355266358), so the in-page launcher button cannot
 * be relied on alone. `chrome.commands.onCommand` and `chrome.contextMenus.onClicked`
 * get a full gesture *and* hand us the tab synchronously, so those paths are the
 * dependable ones — see installTerminalPanelCommandListener.
 */
export declare function openTerminalSidePanel(tabId: number | undefined): Promise<{
    success: boolean;
    error?: string;
}>;
//# sourceMappingURL=terminal-panel.d.ts.map