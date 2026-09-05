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
/** One tracker per document: the observer has to be running before anything is extracted. */
const tracker = new PostLoadInjectionTracker();
/** Start watching this document for post-load insertions. Idempotent. */
export function initContentProvenance() {
    tracker.start(document, window);
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