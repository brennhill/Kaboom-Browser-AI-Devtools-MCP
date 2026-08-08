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
export function isInternalUrl(url) {
    if (!url)
        return true;
    const internalPrefixes = ['chrome://', 'chrome-extension://', 'about:', 'edge://', 'brave://', 'devtools://'];
    return internalPrefixes.some((prefix) => url.startsWith(prefix));
}
/**
 * Check if a tab is internal, accounting for navigations that have not committed yet.
 * Chrome keeps reporting the outgoing document in `url` while an uncommitted
 * navigation's destination is visible only through `pendingUrl`, so a tab racing
 * toward a restricted page still looks scriptable through `url` alone. Fails
 * closed: either URL being internal makes the tab internal.
 */
export function isInternalTab(tab) {
    if (!tab)
        return true;
    if (isInternalUrl(tab.url))
        return true;
    return typeof tab.pendingUrl === 'string' && tab.pendingUrl.length > 0 && isInternalUrl(tab.pendingUrl);
}
//# sourceMappingURL=internal-url.js.map