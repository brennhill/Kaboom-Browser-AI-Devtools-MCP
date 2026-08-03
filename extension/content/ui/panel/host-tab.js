/**
 * Purpose: Side-panel host-tab resolution, and the two actions that depend on it —
 * starting page annotation on the tracked tab, and closing the browser side panel.
 * Why: None of this touches panel UI state; it is purely "which tab am I attached to
 * and how do I talk to it". Extracted so sidepanel.ts stays within the 800-line
 * limit, and because the folder limit pushes cohesive clusters into sub-modules
 * rather than more siblings.
 * Docs: docs/features/feature/terminal/index.md
 */
import { showActionToast } from '../toast.js';
export function getHostTabIdFromLocation() {
    try {
        const raw = new URLSearchParams(globalThis.location?.search ?? '').get('tabId');
        if (!raw)
            return undefined;
        const parsed = Number(raw);
        return Number.isFinite(parsed) ? parsed : undefined;
    }
    catch {
        // EXPECTED_ABSENCE: session storage loss during extension teardown is normal; logging would duplicate lifecycle recovery.
        return undefined;
    }
}
export async function getHostTabId() {
    const fromLocation = getHostTabIdFromLocation();
    if (fromLocation !== undefined)
        return fromLocation;
    if (!chrome.tabs?.query)
        return undefined;
    try {
        const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });
        return tab?.id;
    }
    catch {
        // EXPECTED_ABSENCE: active-tab loss while opening the panel is normal; logging would mislabel user navigation as failure.
        return undefined;
    }
}
/**
 * Start page annotation (draw mode) on the terminal's host/tracked tab.
 *
 * Draw mode is otherwise only reachable via the right-click menu / keyboard
 * shortcut, which is hard to discover — the header button surfaces it right next
 * to the terminal. The terminal lives in the side panel but draw mode runs in
 * the tracked tab's content script, so we send the same kaboom_draw_mode_start
 * the popup and shortcut use directly to that tab.
 */
export async function startPageAnnotation() {
    const tabId = await getHostTabId();
    if (typeof tabId !== 'number') {
        showActionToast('No page to annotate', 'Annotate', 'warning', 2000);
        return;
    }
    try {
        await chrome.tabs.sendMessage(tabId, { type: 'kaboom_draw_mode_start', started_by: 'user' });
    }
    catch {
        showActionToast('Refresh the page, then try Annotate again', 'Annotate', 'warning', 2600);
    }
}
/**
 * Close the browser side panel.
 *
 * `chrome.sidePanel.close()` only exists in very recent Chrome. The old code
 * bailed out silently when it was missing, so the close button did nothing at
 * all — combined with unmountPanel() that left a blank panel the user could not
 * close *or* recover. `window.close()` works from the panel document itself on
 * every version that has side panels, so it is the fallback and the last word.
 */
export async function closeBrowserSidePanel() {
    if (chrome.sidePanel?.close) {
        const tabId = await getHostTabId();
        if (tabId !== undefined) {
            try {
                await chrome.sidePanel.close({ tabId });
                return;
            }
            catch {
                // EXPECTED_ABSENCE: UI recipients can normally disappear during navigation
                // or teardown; logging it would misleadingly report a normal lifecycle race as failure.
                // Fall through to window.close().
            }
        }
    }
    try {
        window.close();
    }
    catch {
        // EXPECTED_ABSENCE: UI recipients can normally disappear during navigation
        // or teardown; logging it would misleadingly report a normal lifecycle race as failure.
        // Nothing else to try; the panel stays open but remains usable.
    }
}
//# sourceMappingURL=host-tab.js.map