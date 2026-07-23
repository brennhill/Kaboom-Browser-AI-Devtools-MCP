/**
 * Purpose: Shared recording helpers used by context menus, keyboard shortcuts, and runtime listeners.
 * Why: Keep recording slug generation consistent across all recording entry points.
 * Docs: docs/features/feature/flow-recording/index.md
 */
/**
 * Build a filesystem-safe recording slug from the current tab URL.
 */
export declare function buildScreenRecordingSlug(url: string | undefined): string;
/**
 * Build a short human-readable recording toast label from a tab URL.
 */
export declare function buildRecordingToastLabel(url: string | undefined): string;
//# sourceMappingURL=recording-utils.d.ts.map