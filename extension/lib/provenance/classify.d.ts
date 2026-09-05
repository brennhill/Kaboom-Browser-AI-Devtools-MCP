/**
 * Purpose: Turn frame and timing facts into a named provenance classification, and build the
 *          envelope that carries those classifications back with the content.
 * Why: The classification is evidence, not a verdict. It never scores a region and never decides
 *      what to do with one — that stays with the agent and the person whose browser it is.
 * Docs: docs/features/feature/content-provenance/index.md
 */
import type { ContentProvenance, ProvenanceClassification, ProvenanceRegion } from './provenance-types.js';
/** The facts a classification is derived from. Every field is observed, none is inferred. */
export interface RegionFacts {
    /** Origin of the region itself. */
    origin: string;
    /** Origin of the first-party (top-level) document. */
    document_origin: string;
    is_top_level_document: boolean;
    is_frame: boolean;
    /** `null` when the post-load signal was unavailable. */
    delivered_in_initial_document: boolean | null;
}
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
export declare function classifyRegion(facts: RegionFacts): ProvenanceClassification;
/** Count regions per classification, including the classifications with no regions. */
export declare function countByClassification(regions: readonly Pick<ProvenanceRegion, 'classification'>[]): Record<ProvenanceClassification, number>;
/**
 * One region attributed by frame identity alone.
 *
 * Used where the evidence comes from outside the page — a frame origin probe, or Chrome's frame
 * tree — so delivery timing is unknown rather than assumed. The in-page collector is the surface
 * that can tell initial-document content from a post-load injection.
 */
export declare function frameRegion(regionId: string, origin: string, documentOrigin: string, isTopLevelDocument: boolean): ProvenanceRegion;
/** The envelope for provenance derived from frame identity alone. */
export declare function frameProvenance(documentOrigin: string, regions: ProvenanceRegion[]): ContentProvenance;
/**
 * The envelope for a response that could not be attributed.
 *
 * Reporting "no regions, attribution unavailable" is the point: an empty provenance block that
 * looked like a clean first-party page would be a false assurance, which is worse than a gap.
 */
export declare function unavailableProvenance(reason: string, extraNotes?: readonly string[]): ContentProvenance;
//# sourceMappingURL=classify.d.ts.map