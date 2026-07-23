/**
 * Purpose: The one place that opens the terminal side panel.
 * Why: `chrome.sidePanel.open()` is gesture-restricted, and the rules for keeping
 * a gesture alive are subtle enough that every entry point must share one
 * implementation (repo rule 19).
 * Docs: docs/features/feature/terminal/index.md
 */
import { resolveTerminalWorkspaceTarget } from './tab-state.js';
function errorMessage(error) {
    return error instanceof Error ? error.message : String(error);
}
function buildTerminalPanelPath(workspace) {
    return `sidepanel.html?tabId=${encodeURIComponent(workspace.hostTabId)}&tabGroupId=${encodeURIComponent(workspace.tabGroupId)}&mainTabId=${encodeURIComponent(workspace.mainTabId)}`;
}
/**
 * Group the tracked tab into the Kaboom workspace and point the panel at the
 * tab-scoped path. Best-effort, and it runs AFTER the panel is already open — it
 * must never gate open(), because its awaits would expire the user gesture. The
 * panel loads fine without it (default manifest path + active-tab fallback in
 * sidepanel.ts).
 */
async function refineTerminalWorkspace(tabId) {
    try {
        const workspace = await resolveTerminalWorkspaceTarget(tabId);
        if (!workspace || !chrome.sidePanel?.setOptions)
            return;
        await chrome.sidePanel
            .setOptions({ tabId: workspace.hostTabId, path: buildTerminalPanelPath(workspace), enabled: true })
            .catch(() => undefined);
    }
    catch {
        // Best-effort: the panel is already open with the default path.
    }
}
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
export async function openTerminalSidePanel(tabId) {
    if (typeof chrome === 'undefined' || !chrome.sidePanel?.open) {
        return { success: false, error: 'side panel unavailable' };
    }
    // Fast path: the tab is already known, so open() runs with zero awaits before it.
    if (typeof tabId === 'number') {
        try {
            await chrome.sidePanel.open({ tabId });
        }
        catch (error) {
            return { success: false, error: errorMessage(error) };
        }
        void refineTerminalWorkspace(tabId);
        return { success: true };
    }
    // Slow path: no tab id available synchronously (e.g. an extension page with no
    // sender tab). Resolving one costs awaits, so the gesture may not survive; this
    // is best-effort and callers should prefer supplying a tab id.
    try {
        const workspace = await resolveTerminalWorkspaceTarget(tabId);
        if (!workspace) {
            return { success: false, error: 'missing workspace tab' };
        }
        const setOptionsPromise = chrome.sidePanel.setOptions
            ? chrome.sidePanel
                .setOptions({ tabId: workspace.hostTabId, path: buildTerminalPanelPath(workspace), enabled: true })
                .catch(() => undefined)
            : null;
        await chrome.sidePanel.open({ tabId: workspace.hostTabId });
        void setOptionsPromise;
        return { success: true };
    }
    catch (error) {
        return { success: false, error: errorMessage(error) };
    }
}
//# sourceMappingURL=terminal-panel.js.map