/**
 * Purpose: The one place that opens the terminal side panel.
 * Why: `chrome.sidePanel.open()` is gesture-restricted, and the rules for keeping
 * a gesture alive are subtle enough that every entry point must share one
 * implementation (repo rule 19).
 * Docs: docs/features/feature/terminal/index.md
 */
import { resolveTerminalWorkspaceTarget } from './tab-state.js';
export { syncTerminalPanelAvailability } from './side-panel-availability.js';
import { StorageKey } from '../lib/constants.js';
import { getSession } from '../lib/storage-utils.js';
/**
 * Whether the panel is showing, read synchronously.
 *
 * Synchronous on purpose. The toggle has to decide open-vs-close *before*
 * calling chrome.sidePanel.open(), and any await first expires the user gesture
 * — which is exactly what broke "Open Kaboom Terminal": the storage read cost
 * the gesture and Chrome refused the open. The service worker mirrors
 * TERMINAL_UI_STATE into this flag instead.
 */
let panelOpenCache = false;
export function isTerminalPanelOpenSync() {
    return panelOpenCache;
}
/**
 * Hydrate the cache and keep it current. Call once during background init.
 *
 * A service worker restart resets the flag, so it is re-read on boot; that read
 * happens long before any click, so it never sits inside a gesture.
 */
export function watchTerminalPanelState() {
    if (typeof chrome === 'undefined' || !chrome.storage?.session)
        return;
    void getSession(StorageKey.TERMINAL_UI_STATE)
        .then((value) => {
        panelOpenCache = value === 'open';
    })
        .catch(() => undefined);
    chrome.storage.onChanged.addListener((changes, areaName) => {
        if (areaName !== 'session')
            return;
        const change = changes[StorageKey.TERMINAL_UI_STATE];
        if (!change)
            return;
        panelOpenCache = change.newValue === 'open';
    });
}
/**
 * Ask the panel to close itself.
 *
 * The background cannot close a side panel document directly on every Chrome
 * version, but the panel can (`window.close()`), so we message it. Closing this
 * way keeps the shell running — reopening reconnects to the same session.
 */
export async function closeTerminalSidePanel() {
    try {
        await chrome.runtime.sendMessage({ type: 'close_terminal_panel' });
        return { success: true };
    }
    catch (error) {
        return { success: false, error: errorMessage(error) };
    }
}
/**
 * Toggle the panel. One helper so the context menu, keyboard command, and any
 * future entry point cannot drift apart (repo rule 19).
 *
 * NOT async on the open path: it reads the cached state synchronously and calls
 * openTerminalSidePanel() with zero awaits before chrome.sidePanel.open(), so
 * the caller's user gesture survives.
 */
export function toggleTerminalSidePanel(tabId) {
    if (isTerminalPanelOpenSync()) {
        return closeTerminalSidePanel();
    }
    return openTerminalSidePanel(tabId);
}
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