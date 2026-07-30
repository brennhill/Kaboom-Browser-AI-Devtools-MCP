/**
 * Purpose: Control which tabs offer the terminal side panel.
 * Why: The manifest's `side_panel.default_path` makes the panel available on
 * every tab, so opening it anywhere untracked showed an empty Kaboom panel.
 * Chrome has no manifest-level "only this tab", so availability is managed here.
 * Docs: docs/features/feature/terminal/index.md
 *
 * Its own module rather than living in terminal-panel.ts or tab-state.ts: both
 * of those need it, and they already depend on each other in one direction.
 * Putting it in either would close the cycle.
 */
/**
 * Path the panel is served from; matches manifest `side_panel.default_path`.
 *
 * One constant path, never per-tab query parameters: Chrome reloads the side
 * panel document whenever the path changes, which would tear down an xterm that
 * had just booted and start a second session underneath it.
 */
export const SIDE_PANEL_PATH = 'sidepanel.html';
/**
 * Offer the side panel on `trackedTabId` and nowhere else.
 *
 * Pass `undefined` when nothing is tracked, which disables it everywhere.
 * Call whenever the tracked tab changes, and once at startup.
 *
 * This governs where the panel is offered *by default*. An explicit request to
 * open it enables the target tab on the spot (see enableTerminalPanelForTab), so
 * scoping never blocks a user who asked for the terminal on some other page.
 */
export async function syncTerminalPanelAvailability(trackedTabId) {
    if (typeof chrome === 'undefined' || !chrome.sidePanel?.setOptions)
        return;
    try {
        // Global default off, so no untracked tab offers an empty panel.
        await chrome.sidePanel.setOptions({ enabled: false });
    }
    catch {
        // EXPECTED_ABSENCE: UI recipients can normally disappear during navigation
        // or teardown; logging it would misleadingly report a normal lifecycle race as failure.
        // Older Chrome without a global default — the per-tab enable below still applies.
    }
    if (typeof trackedTabId !== 'number')
        return;
    try {
        await chrome.sidePanel.setOptions({ tabId: trackedTabId, path: SIDE_PANEL_PATH, enabled: true });
    }
    catch {
        // EXPECTED_ABSENCE: UI recipients can normally disappear during navigation
        // or teardown; logging it would misleadingly report a normal lifecycle race as failure.
        // The tab may have closed between read and write.
    }
}
/**
 * Make the panel openable on `tabId`, right now.
 *
 * DISPATCHES SYNCHRONOUSLY and does not await. `chrome.sidePanel.open()` needs a
 * live user gesture and any await before it expires the gesture, so the caller
 * fires this and then calls open() immediately. Chrome processes both on the same
 * channel in order, so the enable lands first.
 *
 * Without it, opening on a tab that availability scoping has disabled fails with
 * "No active side panel for tabId: N" — which is what made the Terminal button
 * dead on every page except the tracked one.
 */
export function enableTerminalPanelForTab(tabId) {
    if (typeof chrome === 'undefined' || !chrome.sidePanel?.setOptions)
        return;
    try {
        const pending = chrome.sidePanel.setOptions({ tabId, path: SIDE_PANEL_PATH, enabled: true });
        void Promise.resolve(pending).catch(() => {
            // EXPECTED_ABSENCE: a preparatory miss is normal because open() performs
            // the authoritative check; logging it would misleadingly duplicate failure evidence.
        });
    }
    catch {
        // EXPECTED_ABSENCE: this preparatory miss is normal because open() reports
        // the authoritative failure; logging it would misleadingly duplicate evidence.
    }
}
//# sourceMappingURL=side-panel-availability.js.map