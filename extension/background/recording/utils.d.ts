/**
 * Purpose: Shared recording helpers used by context menus, keyboard shortcuts, and runtime listeners.
 * Why: Keep recording slug generation consistent across all recording entry points.
 * Docs: docs/features/feature/flow-recording/index.md
 */
/**
 * Request context for starting a recording: how it was initiated and where results resolve.
 */
export interface RecordingStartContext {
    /** PendingQuery ID for result resolution */
    queryId?: string;
    /** true when initiated from popup (activeTab already granted, skip reload) */
    fromPopup?: boolean;
    /** Explicit target tab (defaults to the active tab) */
    targetTabId?: number;
    /** Server connection generation guard */
    connectionGeneration?: number;
}
/**
 * Build a filesystem-safe recording slug from the current tab URL.
 */
export declare function buildScreenRecordingSlug(url: string | undefined): string;
/**
 * Build a short human-readable recording toast label from a tab URL.
 */
export declare function buildRecordingToastLabel(url: string | undefined): string;
//# sourceMappingURL=utils.d.ts.map