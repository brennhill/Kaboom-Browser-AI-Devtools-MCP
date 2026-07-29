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
 * Persist tracked tab state.
 */
export declare function setTrackedTab(tab: Pick<chrome.tabs.Tab, 'id' | 'url' | 'title'>): Promise<void>;
/**
 * Clear tracked tab state (all keys, including title).
 */
export declare function clearTrackedTab(): Promise<void>;
//# sourceMappingURL=tracked-tab-storage.d.ts.map