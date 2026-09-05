/**
 * Purpose: Reduce a URL to an origin, and compare origins the way the platform does.
 * Why: Provenance has to name where content came from without ever recording where the user went.
 *      Rule 13 forbids logging full URLs; an origin carries the attribution and drops the path,
 *      query string, and fragment that carry session tokens and search terms.
 * Docs: docs/features/feature/content-provenance/index.md
 */
// origins.ts — Origin reduction and same-origin comparison for content provenance.
/** What the platform calls an origin that is same-origin with nothing: data:, about:blank, sandboxed frames. */
export const OPAQUE_ORIGIN = 'null';
/**
 * Reduce a URL to `scheme://host[:port]`.
 *
 * Returns `OPAQUE_ORIGIN` for sources that have no tuple origin, and `''` when the origin cannot
 * be determined at all — an unknown origin stays unknown rather than defaulting to the document's.
 */
export function toOrigin(url, base) {
    const raw = (url ?? '').trim();
    if (raw === '')
        return '';
    // A blob: URL carries its creator's origin after the scheme; URL.origin does not always expose it.
    const target = raw.startsWith('blob:') ? raw.slice('blob:'.length) : raw;
    try {
        const parsed = base ? new URL(target, base) : new URL(target);
        // URL.origin is already scheme + host + port: the path, query, and fragment never appear.
        return parsed.origin && parsed.origin !== OPAQUE_ORIGIN ? parsed.origin : OPAQUE_ORIGIN;
    }
    catch (err) {
        // EXPECTED_ABSENCE: a relative or malformed src with no usable base is normal page markup.
        // Returning '' reports the origin as unknown, and logging every such attribute would file
        // ordinary markup as a failure.
        void err;
        return '';
    }
}
/**
 * Whether two origins are the same origin.
 *
 * An opaque origin is same-origin with nothing, not even another opaque origin — treating them as
 * equal would let a `data:` iframe pass as first-party content. An unknown origin never matches.
 */
export function sameOrigin(a, b) {
    if (!a || !b)
        return false;
    if (a === OPAQUE_ORIGIN || b === OPAQUE_ORIGIN)
        return false;
    return a === b;
}
//# sourceMappingURL=origins.js.map