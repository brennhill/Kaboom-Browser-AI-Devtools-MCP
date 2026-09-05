/**
 * Purpose: Reduce a URL to an origin, and compare origins the way the platform does.
 * Why: Provenance has to name where content came from without ever recording where the user went.
 *      Rule 13 forbids logging full URLs; an origin carries the attribution and drops the path,
 *      query string, and fragment that carry session tokens and search terms.
 * Docs: docs/features/feature/content-provenance/index.md
 */
/** What the platform calls an origin that is same-origin with nothing: data:, about:blank, sandboxed frames. */
export declare const OPAQUE_ORIGIN = "null";
/**
 * Reduce a URL to `scheme://host[:port]`.
 *
 * Returns `OPAQUE_ORIGIN` for sources that have no tuple origin, and `''` when the origin cannot
 * be determined at all — an unknown origin stays unknown rather than defaulting to the document's.
 */
export declare function toOrigin(url: string | null | undefined, base?: string | null): string;
/**
 * Whether two origins are the same origin.
 *
 * An opaque origin is same-origin with nothing, not even another opaque origin — treating them as
 * equal would let a `data:` iframe pass as first-party content. An unknown origin never matches.
 */
export declare function sameOrigin(a: string, b: string): boolean;
//# sourceMappingURL=origins.d.ts.map