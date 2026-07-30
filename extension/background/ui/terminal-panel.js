/**
 * Purpose: The one place that opens, closes, and tracks the terminal side panel.
 * Why: `chrome.sidePanel.open()` is gesture-restricted, and the rules for keeping
 * a gesture alive are subtle enough that every entry point must share one
 * implementation (repo rule 19).
 * Docs: docs/features/feature/terminal/index.md
 */
import { resolveTerminalWorkspaceTarget } from './terminal-workspace.js';
import { enableTerminalPanelForTab, SIDE_PANEL_PATH } from './side-panel-availability.js';
import { TERMINAL_PANEL_PORT, StorageKey } from '../../lib/constants.js';
import { persist } from '../../lib/storage/io.js';
import { setSession } from '../../lib/storage/session.js';
import { errorMessage } from '../../lib/error-utils.js';
/**
 * The live panel document's port, or null when no panel is open.
 *
 * Presence, not a mirrored flag. The previous version mirrored
 * TERMINAL_UI_STATE from chrome.storage, which goes stale the moment the panel
 * is dismissed by Chrome's own X — that path has no chance to flush a storage
 * write — so the toggle kept trying to close a panel that was already gone and
 * the user could never open one again. A port is torn down by Chrome whenever
 * the document dies, however it died.
 *
 * Read synchronously on purpose: the toggle must decide open-vs-close *before*
 * chrome.sidePanel.open(), and any await first expires the user gesture.
 */
// Every live panel document, not one flag. A user can have a panel open in more
// than one window, and closing one must not report "no panel" while another is
// still up (that flipped TERMINAL_UI_STATE to 'closed', so the flame reappeared
// and the surviving panel's own storage listener then hid it).
const livePanelPorts = new Set();
export function isTerminalPanelOpenSync() {
    return livePanelPorts.size > 0;
}
/** The most-recently-connected live panel port (Set keeps insertion order), or null. */
function currentPanelPort() {
    let last = null;
    for (const port of livePanelPorts)
        last = port;
    return last;
}
/**
 * Track whether a panel document is alive. Call once during background init.
 *
 * The panel connects on load and reconnects if the service worker restarts, so
 * this stays accurate across worker teardown.
 */
export function watchTerminalPanelState() {
    if (typeof chrome === 'undefined' || !chrome.runtime?.onConnect)
        return;
    chrome.runtime.onConnect.addListener((port) => {
        if (port.name !== TERMINAL_PANEL_PORT)
            return;
        livePanelPorts.add(port);
        // Mirror authoritative panel presence into TERMINAL_UI_STATE. The in-page
        // flame launcher suppresses itself while the terminal is visible, reading
        // this key — but a Chrome-native panel close (the panel's own "X") destroys
        // the document with no chance to flush its 'closed' write, which used to
        // leave the key stuck at 'open' and hide the flame forever. The port drop is
        // the reliable "panel is gone" signal, so reset the mirror here.
        persist(setSession(StorageKey.TERMINAL_UI_STATE, 'open'), 'terminal-ui-state-open');
        port.onDisconnect.addListener(() => {
            livePanelPorts.delete(port);
            // Only mark closed once the LAST panel document is gone — another window's
            // panel may still be open, and an older port's drop must not hide the flame
            // (or flip the mirror) while a newer panel is live.
            if (livePanelPorts.size === 0) {
                persist(setSession(StorageKey.TERMINAL_UI_STATE, 'closed'), 'terminal-ui-state-closed');
            }
        });
    });
}
/**
 * Ask the panel to close itself.
 *
 * The background cannot close a side panel document directly on every Chrome
 * version, but the panel can (`window.close()`), so we message it. Closing this
 * way keeps the shell running — reopening reconnects to the same session.
 *
 * With no panel connected there is nothing to close, and reporting that as a
 * failure would surface an error toast for a no-op.
 */
export async function closeTerminalSidePanel() {
    const port = currentPanelPort();
    if (!port)
        return { success: true };
    try {
        port.postMessage({ type: 'close_terminal_panel' });
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
 * NOT async on the open path: it reads panel presence synchronously and calls
 * openTerminalSidePanel() with zero awaits before chrome.sidePanel.open(), so
 * the caller's user gesture survives.
 */
export function toggleTerminalSidePanel(tabId) {
    if (isTerminalPanelOpenSync()) {
        return closeTerminalSidePanel();
    }
    return openTerminalSidePanel(tabId);
}
/**
 * Tell an existing panel to put the terminal back on screen.
 *
 * Chrome answers `sidePanel.open()` on a panel that already exists by focusing
 * it — no code runs in the panel document. So a panel sitting minimized, or
 * blank because its close was refused, would stay that way and "open" would look
 * like it did nothing. Best-effort: if there is no panel, open() is doing the
 * work and this is a no-op.
 */
function requestPanelRestore() {
    const port = currentPanelPort();
    if (!port)
        return;
    try {
        port.postMessage({ type: 'restore_terminal_panel' });
    }
    catch {
        // The document died between the presence check and the post; open() covers it.
    }
}
/**
 * Group the tracked tab into the Kaboom workspace and make sure the panel is
 * offered on the tab that ends up hosting it. Best-effort, and it runs AFTER the
 * panel is already open — it must never gate open(), because its awaits would
 * expire the user gesture.
 *
 * It deliberately does not touch the panel path: a path change reloads the side
 * panel document, which would tear down the xterm that just booted.
 */
async function refineTerminalWorkspace(tabId) {
    try {
        const workspace = await resolveTerminalWorkspaceTarget(tabId);
        if (!workspace || !chrome.sidePanel?.setOptions)
            return;
        if (workspace.hostTabId === tabId)
            return;
        await chrome.sidePanel
            .setOptions({ tabId: workspace.hostTabId, path: SIDE_PANEL_PATH, enabled: true })
            .catch(() => {
            // EXPECTED_ABSENCE: optional refinement failure is normal after the panel
            // opens on its safe host; logging it would misleadingly imply open failed.
        });
    }
    catch {
        // EXPECTED_ABSENCE: optional refinement failure is normal after the panel
        // opens; logging it would misleadingly misdiagnose a successful user action.
    }
}
/**
 * Open the terminal side panel on `tabId`.
 *
 * GESTURE CONTRACT — nothing may be awaited before `chrome.sidePanel.open()`.
 * Chrome requires an active user gesture, and any await first expires it. The
 * availability enable below is dispatched, never awaited, for exactly that
 * reason; Chrome preserves the ordering of the two calls on its own.
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
        // An explicit request outranks tracked-tab scoping. Without this, opening on
        // any tab the scoping disabled fails with "No active side panel for tabId".
        enableTerminalPanelForTab(tabId);
        requestPanelRestore();
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
        // gesture-safety: allow — there is no tab id to open on, so resolving one is
        // the only option; this path is best-effort and callers avoid it.
        const workspace = await resolveTerminalWorkspaceTarget(tabId);
        if (!workspace) {
            return { success: false, error: 'missing workspace tab' };
        }
        enableTerminalPanelForTab(workspace.hostTabId);
        requestPanelRestore();
        await chrome.sidePanel.open({ tabId: workspace.hostTabId });
        return { success: true };
    }
    catch (error) {
        return { success: false, error: errorMessage(error) };
    }
}
//# sourceMappingURL=terminal-panel.js.map