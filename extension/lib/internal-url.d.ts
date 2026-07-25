/**
 * Purpose: Canonical predicate for browser-internal URLs where content scripts cannot run.
 * Why: One source of truth so tracking guards, popup, and background never drift on
 *      which pages are trackable (previously duplicated as isInternalUrl + isRestrictedUrl).
 * Docs: docs/features/feature/tab-tracking-ux/index.md
 */
/**
 * Check if a URL is an internal browser page that cannot be tracked or scripted.
 * Chrome blocks content scripts from these pages, so tracking is impossible.
 * A missing URL is treated as internal (fail closed).
 */
export declare function isInternalUrl(url: string | undefined): boolean;
//# sourceMappingURL=internal-url.d.ts.map