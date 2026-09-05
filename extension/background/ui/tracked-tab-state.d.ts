/**
 * Purpose: Owns live tracked-tab lookup, tab readiness, active-tab selection, and capture of a
 *          tab the user is not looking at.
 * Why: `chrome.tabs.captureVisibleTab` can only photograph the tab that is visible, so every
 *      screenshot used to activate the target tab and put the user's tab back afterwards.
 *      That stole the foreground once per capture and dropped whatever the person was typing.
 *      `Page.captureScreenshot` over the tab's persistent CDP lease has no such constraint.
 * Docs: docs/features/feature/tab-tracking-ux/index.md
 */
import type { CoveredCssRegion } from '../../lib/screenshot/coordinate-frame.js';
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
export declare function captureTabImage(tabId: number, windowId: number, options: TabCaptureOptions): Promise<TabCapture>;
/**
 * One capture, plus the CSS region it actually photographed.
 *
 * The region travels with the image because only the path that took it knows what
 * it covers: the CDP path clips to `cssVisualViewport`, which excludes scrollbars,
 * while `captureVisibleTab` photographs the whole visible viewport. Those differ by
 * the scrollbar width, and a coordinate frame built from the wrong one misplaces
 * every click near the right or bottom edge of the image by that much.
 *
 * `covered_css_region` is null when the path cannot report one, which means "the
 * visible viewport as the page reports it" — see coveredRegionFor.
 */
export interface TabCapture {
    readonly data_url: string;
    readonly covered_css_region: CoveredCssRegion | null;
    readonly source: 'cdp' | 'visible_tab';
}
//# sourceMappingURL=tracked-tab-state.d.ts.map