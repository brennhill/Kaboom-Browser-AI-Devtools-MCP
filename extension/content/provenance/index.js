/**
 * Purpose: The content script's entry point to content provenance — one tracker per document,
 *          and one call that attributes whatever a extractor is about to return.
 * Why: Provenance the agent has to make a second call to fetch is provenance it will not fetch,
 *      so every extractor reaches this from the same place and ships attribution in its payload.
 * Docs: docs/features/feature/content-provenance/index.md
 */
// index.ts — Content-provenance wiring for the content script.
import { collectContentProvenance } from './collect.js';
import { PostLoadInjectionTracker } from './post-load-tracker.js';
import { toOrigin } from '../../lib/provenance/origins.js';
import { KABOOM_LOG_PREFIX } from '../../lib/brand.js';
import { errorMessage } from '../../lib/error-utils.js';
/** One tracker per document: the observer has to be running before anything is extracted. */
const tracker = new PostLoadInjectionTracker();
/**
 * Start watching this document for post-load insertions. Idempotent.
 *
 * Provenance reports; it is never load-bearing. This runs first in the content script's bootstrap,
 * so a throw here — a context with no MutationObserver, a document with no documentElement — would
 * take the message listeners and request tracking down with it and leave the tab undrivable. The
 * failure is reported and the bootstrap continues; `wasInjectedAfterLoad` then answers `null`
 * (unknown timing) rather than claiming content was in the initial document.
 */
export function initContentProvenance() {
    try {
        tracker.start(document, window);
    }
    catch (err) {
        console.warn(`${KABOOM_LOG_PREFIX} content provenance did not start; delivery timing will be reported as unknown:`, errorMessage(err));
    }
}
/** Stop watching, so later queries report timing as unknown rather than as initial content. */
export function stopContentProvenance() {
    tracker.disconnect();
}
/**
 * Origin of the first-party document.
 *
 * The content script is registered without `all_frames`, so this is the page's own origin in
 * practice. The subframe branch stays because a wrong first-party origin would mislabel every
 * region, and `''` (unknown) is the honest answer where the ancestor chain is not exposed.
 */
function firstPartyOrigin(isTopLevel) {
    if (isTopLevel)
        return toOrigin(window.location.href);
    const ancestors = window.location.ancestorOrigins;
    const outermost = ancestors && ancestors.length > 0 ? ancestors.item(ancestors.length - 1) : null;
    return outermost ? toOrigin(outermost) : '';
}
/** Attribute the regions of `root`, for inclusion in the extraction response that carries its text. */
export function provenanceForExtraction(root) {
    const isTopLevel = window.top === window;
    const env = {
        document_origin: firstPartyOrigin(isTopLevel),
        frame_origin: toOrigin(window.location.href),
        frame_href: window.location.href,
        is_top_level_document: isTopLevel,
        tracker
    };
    return collectContentProvenance(root, env);
}
//# sourceMappingURL=index.js.map