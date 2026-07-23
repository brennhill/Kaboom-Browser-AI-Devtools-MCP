/**
 * Purpose: Chrome API and storage operations for tab tracking — track/untrack lifecycle, tab switching.
 * Why: Separates browser API side-effects from DOM UI state rendering in tab-tracking.
 * Docs: docs/features/feature/tab-tracking-ux/index.md
 */
export type ShowStateFn = (btn: HTMLButtonElement) => void;
export type ShowTrackingStateFn = (btn: HTMLButtonElement, url: string | undefined, tabId: number | undefined) => void;
/**
 * Handle launching the tracked-site audit workflow from popup controls.
 */
export declare function handleAuditClick(pageUrl: string | undefined): Promise<void>;
/**
 * Handle stop tracking from the compact tracking bar stop button.
 */
export declare function handleStopTracking(showIdleState: ShowStateFn): Promise<void>;
/**
 * Handle clicking on the tracked URL.
 * Switches to the tracked tab.
 */
export declare function handleUrlClick(tabId: number | undefined): Promise<void>;
/**
 * Handle Track This Tab button click.
 * Toggles tracking on/off for the current tab.
 * Blocks tracking on internal Chrome pages.
 */
export declare function handleTrackPageClick(showInternalPageState: ShowStateFn, showCloakedState: ShowStateFn, showTrackingState: ShowTrackingStateFn, showIdleState: ShowStateFn): Promise<void>;
//# sourceMappingURL=tab-tracking-api.d.ts.map