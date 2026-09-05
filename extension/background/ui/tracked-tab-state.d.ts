/**
 * Purpose: Owns live tracked-tab lookup, tab readiness, active-tab selection, and capture of a
 *          tab the user is not looking at.
 * Why: `chrome.tabs.captureVisibleTab` can only photograph the tab that is visible, so every
 *      screenshot used to activate the target tab and put the user's tab back afterwards.
 *      That stole the foreground once per capture and dropped whatever the person was typing.
 *      `Page.captureScreenshot` over the tab's persistent CDP lease has no such constraint.
 * Docs: docs/features/feature/tab-tracking-ux/index.md
 */
export interface TrackedTabInfo {
    trackedTabId: number | null;
    trackedTabUrl: string | null;
    trackedTabTitle: string | null;
    tabStatus: 'loading' | 'complete' | null;
    trackedTabActive: boolean | null;
}
export declare function waitForTabLoad(tabId: number, timeoutMs?: number): Promise<boolean>;
export declare function getTrackedTabInfo(): Promise<TrackedTabInfo>;
export declare function getActiveTab(): Promise<chrome.tabs.Tab | null>;
export interface TabCaptureOptions {
    format: 'jpeg' | 'png';
    /** JPEG only. Chrome rejects `quality` on a PNG capture. */
    quality?: number;
}
/**
 * Photograph a tab WITHOUT bringing it to the foreground.
 *
 * The CDP path is the ordinary one: it works on a tab the user is not looking at, so the
 * agent can drive and observe in the background while the person keeps working. The
 * visible-tab path is the fallback for a context with no `chrome.debugger`, and is the only
 * remaining place that touches the foreground.
 *
 * Kaboom's own overlays (`data-kaboom-overlay`) are hidden across BOTH paths, so a screenshot
 * never contains the supervision badge or phantom cursor that Kaboom itself drew.
 */
export declare function captureTabImage(tabId: number, windowId: number, options: TabCaptureOptions): Promise<string>;
//# sourceMappingURL=tracked-tab-state.d.ts.map