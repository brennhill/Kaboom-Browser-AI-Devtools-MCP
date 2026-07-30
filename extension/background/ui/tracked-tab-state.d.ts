/**
 * Purpose: Owns live tracked-tab lookup, tab readiness, active-tab selection, and focus-safe capture.
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
export declare function captureVisibleTabSafe(tabId: number, windowId: number, options: {
    format: 'jpeg' | 'png';
    quality?: number;
}): Promise<string>;
//# sourceMappingURL=tracked-tab-state.d.ts.map