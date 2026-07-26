/**
 * Purpose: Side-panel host-tab resolution, and the two actions that depend on it —
 * starting page annotation on the tracked tab, and closing the browser side panel.
 * Why: None of this touches panel UI state; it is purely "which tab am I attached to
 * and how do I talk to it". Extracted so sidepanel.ts stays within the 800-line
 * limit, and because the folder limit pushes cohesive clusters into sub-modules
 * rather than more siblings.
 * Docs: docs/features/feature/terminal/index.md
 */
export declare function getHostTabIdFromLocation(): number | undefined;
export declare function getHostTabId(): Promise<number | undefined>;
/**
 * Start page annotation (draw mode) on the terminal's host/tracked tab.
 *
 * Draw mode is otherwise only reachable via the right-click menu / keyboard
 * shortcut, which is hard to discover — the header button surfaces it right next
 * to the terminal. The terminal lives in the side panel but draw mode runs in
 * the tracked tab's content script, so we send the same kaboom_draw_mode_start
 * the popup and shortcut use directly to that tab.
 */
export declare function startPageAnnotation(): Promise<void>;
/**
 * Close the browser side panel.
 *
 * `chrome.sidePanel.close()` only exists in very recent Chrome. The old code
 * bailed out silently when it was missing, so the close button did nothing at
 * all — combined with unmountPanel() that left a blank panel the user could not
 * close *or* recover. `window.close()` works from the panel document itself on
 * every version that has side panels, so it is the fallback and the last word.
 */
export declare function closeBrowserSidePanel(): Promise<void>;
//# sourceMappingURL=host-tab.d.ts.map