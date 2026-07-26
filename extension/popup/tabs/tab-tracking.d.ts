/**
 * Purpose: Manages popup tab-tracking UI state and track/untrack transitions for the active browser tab.
 * Why: Keeps the tracked-tab lifecycle explicit so content-script injection and status UX stay synchronized.
 * Docs: docs/features/feature/tab-tracking-ux/index.md
 */
export declare function initTrackPageButton(): void;
export declare function handleTrackPageClick(): Promise<void>;
//# sourceMappingURL=tab-tracking.d.ts.map