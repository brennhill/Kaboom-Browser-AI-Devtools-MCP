/**
 * Purpose: The one place that opens the terminal side panel.
 * Why: `chrome.sidePanel.open()` is gesture-restricted, and the rules for keeping
 * a gesture alive are subtle enough that every entry point must share one
 * implementation (repo rule 19).
 * Docs: docs/features/feature/terminal/index.md
 */
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