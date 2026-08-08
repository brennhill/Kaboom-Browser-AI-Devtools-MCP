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
/** The two URLs Chrome exposes for a tab: the committed one and any in-flight destination. */
export interface InternalUrlTarget {
    url?: string;
    pendingUrl?: string;
}
/**
 * Check if a tab is internal, accounting for navigations that have not committed yet.
 * Chrome keeps reporting the outgoing document in `url` while an uncommitted
 * navigation's destination is visible only through `pendingUrl`, so a tab racing
 * toward a restricted page still looks scriptable through `url` alone. Fails
 * closed: either URL being internal makes the tab internal.
 */
export declare function isInternalTab(tab: InternalUrlTarget | null | undefined): boolean;
//# sourceMappingURL=internal-url.d.ts.map