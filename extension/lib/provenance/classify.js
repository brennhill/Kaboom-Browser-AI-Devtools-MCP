/**
 * Purpose: Turn frame and timing facts into a named provenance classification, and build the
 *          envelope that carries those classifications back with the content.
 * Why: The classification is evidence, not a verdict. It never scores a region and never decides
 *      what to do with one — that stays with the agent and the person whose browser it is.
 * Docs: docs/features/feature/content-provenance/index.md
 */
// classify.ts — Region classification and the ContentProvenance envelope.
import { sameOrigin } from './origins.js';
import { PROVENANCE_CLASSIFICATIONS } from './provenance-types.js';
/**
 * Classify one region.
 *
 * Precedence, most specific first:
 *  1. Bytes that were not in the document Chrome parsed are `post_load_injected`, whoever served
 *     them. Timing is the fact an agent cannot recover any other way, and an ad-network injection
 *     after load is the shape the bead exists to name.
 *  2. Content at an origin that is not the first party's is `third_party_frame`. An opaque origin
 *     (a `data:` or sandboxed frame) is not the first party either.
 *  3. Anything else that is not the top-level document itself is a `same_origin_subresource`.
 *  4. What remains is the `first_party_document`.
 *
 * The region keeps its raw facts alongside the classification, so a cross-origin frame injected
 * after load still reports `origin` and `is_frame` — nothing is lost to the headline.
 */
export function classifyRegion(facts) {
    if (facts.delivered_in_initial_document === false)
        return 'post_load_injected';
    if (!sameOrigin(facts.origin, facts.document_origin))
        return 'third_party_frame';
    if (facts.is_frame || !facts.is_top_level_document)
        return 'same_origin_subresource';
    return 'first_party_document';
}
/** Count regions per classification, including the classifications with no regions. */
export function countByClassification(regions) {
    const counts = {
        first_party_document: 0,
        same_origin_subresource: 0,
        third_party_frame: 0,
        post_load_injected: 0
    };
    for (const region of regions) {
        if (PROVENANCE_CLASSIFICATIONS.includes(region.classification))
            counts[region.classification] += 1;
    }
    return counts;
}
/**
 * One region attributed by frame identity alone.
 *
 * Used where the evidence comes from outside the page — a frame origin probe, or Chrome's frame
 * tree — so delivery timing is unknown rather than assumed. The in-page collector is the surface
 * that can tell initial-document content from a post-load injection.
 */
export function frameRegion(regionId, origin, documentOrigin, isTopLevelDocument) {
    const isFrame = !isTopLevelDocument;
    return {
        region_id: regionId,
        classification: classifyRegion({
            origin,
            document_origin: documentOrigin,
            is_top_level_document: isTopLevelDocument,
            is_frame: isFrame,
            delivered_in_initial_document: null
        }),
        origin,
        is_top_level_document: isTopLevelDocument,
        is_frame: isFrame,
        delivered_in_initial_document: null,
        initiator_origin: isFrame ? documentOrigin || null : null,
        text_length: 0,
        imperative_text: null
    };
}
/** The envelope for provenance derived from frame identity alone. */
export function frameProvenance(documentOrigin, regions) {
    return {
        attribution_available: true,
        document_origin: documentOrigin,
        is_top_level_document: true,
        injection_tracking_active: false,
        regions,
        region_counts: countByClassification(regions),
        post_load_script_origins: [],
        imperative_text_from_non_first_party: [],
        notes: ['Frame delivery timing is not observable outside the page, so it is reported as unknown.']
    };
}
/**
 * The envelope for a response that could not be attributed.
 *
 * Reporting "no regions, attribution unavailable" is the point: an empty provenance block that
 * looked like a clean first-party page would be a false assurance, which is worse than a gap.
 */
export function unavailableProvenance(reason, extraNotes = []) {
    return {
        attribution_available: false,
        document_origin: '',
        is_top_level_document: false,
        injection_tracking_active: false,
        regions: [],
        region_counts: countByClassification([]),
        post_load_script_origins: [],
        imperative_text_from_non_first_party: [],
        notes: [`Content provenance is unavailable for this response (${reason}).`, ...extraNotes]
    };
}
//# sourceMappingURL=classify.js.map