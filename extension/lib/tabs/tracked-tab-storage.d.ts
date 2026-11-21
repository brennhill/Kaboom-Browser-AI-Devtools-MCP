/**
 * Purpose: Shared storage helpers for tracked-tab state (TRACKED_TAB_ID/URL/TITLE).
 * Why: Tracked-tab storage keys must be accessed through one helper module (CLAUDE.md rule 18)
 *      so background and popup never drift (e.g., leaving a stale TRACKED_TAB_TITLE behind).
 * Docs: docs/features/feature/tab-tracking-ux/index.md
 */
export interface TrackedTabState {
    id?: number;
    url?: string;
    title?: string;
}
/** All storage keys that make up tracked-tab state. Always read/cleared together. */
export declare const TRACKED_TAB_STORAGE_KEYS: ("trackedTabId" | "trackedTabUrl" | "trackedTabTitle")[];
/** Read the complete tracked-tab identity as one consistent snapshot. */
export declare function readTrackedTab(): Promise<TrackedTabState>;
/**
 * Persist tracked tab state (selection: popup, context menu, switch_tab, auto-track).
 */
export declare function setTrackedTab(tab: Pick<chrome.tabs.Tab, 'id' | 'url' | 'title'>): Promise<void>;
/**
 * Record navigation details for `tabId` only if it is still the tracked tab when the
 * write runs. Returns whether the details were written.
 */
export declare function updateTrackedTabDetails(tabId: number, details: {
    url: string;
    title?: string;
}): Promise<boolean>;
/**
 * Clear tracked tab state only if `tabId` is still the tracked tab (Chrome close
 * lifecycle). Returns whether tracking was cleared.
 */
export declare function clearTrackedTabIfCurrent(tabId: number): Promise<boolean>;
/**
 * Clear tracked tab state (all keys, including title).
 */
export declare function clearTrackedTab(): Promise<void>;
//# sourceMappingURL=tracked-tab-storage.d.ts.map