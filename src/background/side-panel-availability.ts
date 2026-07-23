/**
 * Purpose: Scope the terminal side panel to the tracked tab.
 * Why: The manifest's `side_panel.default_path` makes the panel available on
 * every tab, so opening it anywhere untracked showed an empty Kaboom panel.
 * Chrome has no manifest-level "only this tab", so availability is managed here.
 * Docs: docs/features/feature/terminal/index.md
 *
 * Its own module rather than living in terminal-panel.ts or tab-state.ts: both
 * of those need it, and they already depend on each other in one direction.
 * Putting it in either would close the cycle.
 */

/** Path the panel is served from; matches manifest `side_panel.default_path`. */
export const SIDE_PANEL_PATH = 'sidepanel.html'

/**
 * Offer the side panel on `trackedTabId` and nowhere else.
 *
 * Pass `undefined` when nothing is tracked, which disables it everywhere.
 * Call whenever the tracked tab changes, and once at startup.
 */
export async function syncTerminalPanelAvailability(trackedTabId?: number): Promise<void> {
  if (typeof chrome === 'undefined' || !chrome.sidePanel?.setOptions) return
  try {
    // Global default off, so no untracked tab offers an empty panel.
    await chrome.sidePanel.setOptions({ enabled: false })
  } catch {
    // Older Chrome without a global default — the per-tab enable below still applies.
  }
  if (typeof trackedTabId !== 'number') return
  try {
    await chrome.sidePanel.setOptions({ tabId: trackedTabId, path: SIDE_PANEL_PATH, enabled: true })
  } catch {
    // The tab may have closed between read and write.
  }
}
