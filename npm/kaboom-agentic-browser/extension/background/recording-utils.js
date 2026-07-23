/**
 * Purpose: Shared recording helpers used by context menus, keyboard shortcuts, and runtime listeners.
 * Why: Keep recording slug generation consistent across all recording entry points.
 * Docs: docs/features/feature/flow-recording/index.md
 */
/**
 * Build a filesystem-safe recording slug from the current tab URL.
 */
export function buildScreenRecordingSlug(url) {
    try {
        const hostname = new URL(url ?? '').hostname.replace(/^www\./, '');
        return (hostname
            .replace(/[^a-z0-9]/gi, '-')
            .replace(/-+/g, '-')
            .replace(/^-|-$/g, '') || 'recording');
    }
    catch {
        return 'recording';
    }
}
/**
 * Build a short human-readable recording toast label from a tab URL.
 */
export function buildRecordingToastLabel(url) {
    try {
        const parsed = new URL(url ?? '');
        const host = parsed.hostname.replace(/^www\./, '');
        const path = parsed.pathname === '/' ? '' : parsed.pathname;
        const base = `${host}${path}`;
        const clipped = base.length > 42 ? `${base.slice(0, 39)}...` : base;
        return `Recording ${clipped}`;
    }
    catch {
        return 'Recording started';
    }
}
//# sourceMappingURL=recording-utils.js.map