/**
 * Purpose: Extracts all links from the current page and checks their health via HEAD/GET/no-cors fallback chain, categorizing results by status.
 * Docs: docs/features/feature/link-health/index.md
 */
/**
 * Check all links on the current page for health issues.
 * Extracts links, deduplicates, and checks max 20 concurrently.
 */
function extractUniqueLinks(domainFilter) {
    const linkElements = document.querySelectorAll('a[href]');
    const urls = new Set();
    for (const elem of linkElements) {
        const href = elem.href;
        if (!href || isIgnoredLink(href))
            continue;
        if (domainFilter !== '' && !matchesDomain(href, domainFilter))
            continue;
        urls.add(href);
    }
    return Array.from(urls);
}
function aggregateResults(results) {
    const summary = {
        totalLinks: results.length,
        ok: 0,
        redirect: 0,
        requiresAuth: 0,
        broken: 0,
        timeout: 0,
        corsBlocked: 0,
        needsServerVerification: 0
    };
    const codeToField = {
        ok: 'ok',
        redirect: 'redirect',
        requires_auth: 'requiresAuth',
        broken: 'broken',
        timeout: 'timeout',
        cors_blocked: 'corsBlocked'
    };
    for (const result of results) {
        const field = codeToField[result.code];
        if (field)
            summary[field]++;
        if (result.code === 'cors_blocked' && result.needsServerVerification)
            summary.needsServerVerification++;
    }
    return summary;
}
export async function checkLinkHealth(params) {
    const timeout_ms = params.timeout_ms || 15000;
    const max_workers = params.max_workers || 20;
    const domainFilter = normalizeDomainFilter(params.domain);
    const uniqueLinks = extractUniqueLinks(domainFilter);
    const results = [];
    const chunks = chunkArray(uniqueLinks, max_workers);
    for (const chunk of chunks) {
        const batchResults = await Promise.allSettled(chunk.map((url) => checkLink(url, timeout_ms)));
        for (const result of batchResults) {
            if (result.status === 'fulfilled' && result.value)
                results.push(result.value);
        }
    }
    return { summary: aggregateResults(results), results };
}
/**
 * Classify an HTTP status code into a link health category.
 */
function classifyStatus(status) {
    if (status >= 200 && status < 300)
        return 'ok';
    if (status >= 300 && status < 400)
        return 'redirect';
    if (status === 401 || status === 403)
        return 'requires_auth';
    return 'broken';
}
/**
 * Returns true if the status indicates we should retry with GET.
 * 405 = Method Not Allowed (server rejects HEAD).
 * 0   = opaque/CORS-blocked response (no readable status).
 */
function shouldFallbackToGet(status) {
    return status === 405 || status === 0;
}
/**
 * Returns true if the error looks like a CORS or network failure (not a timeout).
 */
function isCorsOrNetworkError(error) {
    return error.name === 'TypeError';
}
/**
 * Try a no-cors GET as a last resort to check if the server is reachable.
 * An opaque response (status 0, type "opaque") proves the server is up,
 * even though we cannot read the actual status code.
 */
async function tryNoCors(url, signal) {
    try {
        await fetch(url, { method: 'GET', mode: 'no-cors', signal, redirect: 'follow' });
        // Any response (even opaque) means the server is reachable
        return true;
    }
    catch {
        return false;
    }
}
/**
 * Check a single link for health issues.
 *
 * Fallback chain:
 *   1. HEAD — fast, minimal bandwidth
 *   2. GET  — when HEAD returns 405 or 0, or throws (CORS)
 *   3. no-cors GET — for cross-origin links, proves reachability
 */
async function checkLink(url, timeout_ms) {
    const startTime = performance.now();
    const isExternal = new URL(url).origin !== window.location.origin;
    const controller = new AbortController();
    const timeoutId = setTimeout(() => controller.abort(), timeout_ms);
    try {
        // === Step 1: HEAD request ===
        const headResult = await tryFetch(url, 'HEAD', controller.signal);
        if (headResult.ok) {
            // HEAD succeeded, but check if we need to fall back
            if (!shouldFallbackToGet(headResult.response.status)) {
                clearTimeout(timeoutId);
                const timeMs = Math.round(performance.now() - startTime);
                return buildResult(url, headResult.response, timeMs, isExternal);
            }
            // 405 or 0 — fall through to GET
        }
        // === Step 2: GET request (fallback) ===
        const getResult = await tryFetch(url, 'GET', controller.signal);
        if (getResult.ok && getResult.response.status !== 0) {
            clearTimeout(timeoutId);
            const timeMs = Math.round(performance.now() - startTime);
            return buildResult(url, getResult.response, timeMs, isExternal);
        }
        // === Step 3: no-cors fallback for external links ===
        if (isExternal) {
            const reachable = await tryNoCors(url, controller.signal);
            clearTimeout(timeoutId);
            const timeMs = Math.round(performance.now() - startTime);
            return {
                url,
                status: null,
                code: 'cors_blocked',
                timeMs,
                isExternal,
                error: reachable
                    ? 'CORS policy blocked the request (server is reachable)'
                    : 'CORS policy blocked the request (server may be unreachable)',
                needsServerVerification: true
            };
        }
        // Internal link returned status 0 on both HEAD and GET — unusual
        clearTimeout(timeoutId);
        const timeMs = Math.round(performance.now() - startTime);
        return {
            url,
            status: null,
            code: 'broken',
            timeMs,
            isExternal,
            error: 'Request returned status 0'
        };
    }
    catch (error) {
        clearTimeout(timeoutId);
        const timeMs = Math.round(performance.now() - startTime);
        const err = error;
        const isTimeout = err.name === 'AbortError';
        if (isTimeout) {
            return { url, status: null, code: 'timeout', timeMs, isExternal, error: 'timeout' };
        }
        // CORS/network errors on external links — classify as cors_blocked, not broken
        if (isExternal && isCorsOrNetworkError(err)) {
            return {
                url,
                status: null,
                code: 'cors_blocked',
                timeMs,
                isExternal,
                error: 'CORS policy blocked the request',
                needsServerVerification: true
            };
        }
        return {
            url,
            status: null,
            code: 'broken',
            timeMs,
            isExternal,
            error: err.message
        };
    }
}
/** Sentinel response for failed fetch attempts. */
const FAILED_RESPONSE = {
    status: 0,
    ok: false,
    redirected: false,
    url: '',
    headers: new Headers()
};
/**
 * Attempt a fetch and return a normalized result.
 * Re-throws AbortError (timeout) so the caller can handle it.
 * All other errors are captured as { ok: false }.
 */
async function tryFetch(url, method, signal) {
    try {
        const response = await fetch(url, { method, signal, redirect: 'follow' });
        return { ok: true, response };
    }
    catch (error) {
        // Let AbortError (timeout) propagate to the outer catch
        if (error.name === 'AbortError')
            throw error;
        return { ok: false, response: FAILED_RESPONSE };
    }
}
/**
 * Build a LinkCheckResult from a successful HTTP response.
 */
function buildResult(url, response, timeMs, isExternal) {
    return {
        url,
        status: response.status,
        code: classifyStatus(response.status),
        timeMs,
        isExternal,
        redirectTo: response.redirected ? response.url : undefined
    };
}
/**
 * Determine if a link should be skipped (javascript:, mailto:, #anchor).
 */
function isIgnoredLink(href) {
    if (href.startsWith('javascript:'))
        return true;
    if (href.startsWith('mailto:'))
        return true;
    if (href.startsWith('tel:'))
        return true;
    if (href.startsWith('#'))
        return true;
    if (href === '')
        return true;
    return false;
}
/**
 * Normalize a domain filter into a lowercase hostname.
 */
function normalizeDomainFilter(domain) {
    const raw = (domain || '').trim().toLowerCase();
    if (raw === '')
        return '';
    const candidate = raw.includes('://') ? raw : `https://${raw}`;
    try {
        return new URL(candidate).hostname.toLowerCase();
    }
    catch {
        return raw;
    }
}
/**
 * Returns true when linkURL host matches domainFilter exactly or as subdomain.
 */
function matchesDomain(linkURL, domainFilter) {
    try {
        const hostname = new URL(linkURL).hostname.toLowerCase();
        return hostname === domainFilter || hostname.endsWith(`.${domainFilter}`);
    }
    catch {
        return false;
    }
}
/**
 * Split array into chunks of specified size.
 */
function chunkArray(arr, chunkSize) {
    const chunks = [];
    for (let i = 0; i < arr.length; i += chunkSize) {
        chunks.push(arr.slice(i, i + chunkSize));
    }
    return chunks;
}
//# sourceMappingURL=link-health.js.map