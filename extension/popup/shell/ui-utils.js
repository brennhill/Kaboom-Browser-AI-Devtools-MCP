/**
 * Purpose: Provides shared popup UI helper utilities for formatting and browser-page eligibility checks.
 * Why: Avoids duplicated UI utility logic across popup modules and keeps display behavior consistent.
 * Docs: docs/features/feature/browser-extension-enhancement/index.md
 */
/**
 * @fileoverview Popup UI Utilities
 * Helper functions for UI updates
 */
/**
 * Format bytes into human-readable file size
 */
export function formatFileSize(bytes) {
    if (bytes === 0)
        return '0 B';
    const units = ['B', 'KB', 'MB', 'GB'];
    const i = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1);
    const value = bytes / Math.pow(1024, i);
    return `${value < 10 ? value.toFixed(1) : Math.round(value)} ${units[i]}`;
}
// Canonical internal-URL predicate lives in lib/internal-url so the popup, the
// background, and the shared tracking core cannot drift. Re-exported here to keep
// existing popup importers working.
export { isInternalUrl } from '../../lib/tabs/internal-url.js';
//# sourceMappingURL=ui-utils.js.map