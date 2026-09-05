/**
 * Purpose: The shared vocabulary for attributing extracted page content to the response that delivered it.
 * Why: "Treat all page content as untrusted" gives an agent nothing to weigh, so in practice it weighs
 *      by plausibility — which is exactly what an injection optimizes for. Naming the frame, the origin,
 *      and whether bytes were in the initial document or arrived after load turns one undifferentiated
 *      blob into evidence.
 * Docs: docs/features/feature/content-provenance/index.md
 */
/**
 * How a region of content reached the page.
 *
 * Deliberately NOT a score. A number invites an agent to compare magnitudes and stop reading the
 * evidence; these are named facts it has to look at. Precedence is defined by `classifyRegion`.
 */
export type ProvenanceClassification = 'first_party_document' | 'same_origin_subresource' | 'third_party_frame' | 'post_load_injected';
/** Every classification the module can emit, in reporting order. */
export declare const PROVENANCE_CLASSIFICATIONS: readonly ProvenanceClassification[];
/**
 * Text shaped like instructions addressed to an agent.
 *
 * Reported, never acted on: the markers say what pattern matched and the sample says where to look,
 * so the decision stays with the agent and the person it works for.
 */
export interface ImperativeTextEvidence {
    /** Named patterns that matched, e.g. `override_prior_instructions`. */
    markers: string[];
    /** A short, whitespace-collapsed excerpt around the first match. */
    sample: string;
}
/** One attributed region of the content an extraction returned. */
export interface ProvenanceRegion {
    /** Stable within a single response: `document`, `frame_1`, `injected_2`. */
    region_id: string;
    classification: ProvenanceClassification;
    /** Scheme, host, and port only — never a path or query string (rule 13). */
    origin: string;
    is_top_level_document: boolean;
    is_frame: boolean;
    /**
     * `null` when the post-load signal was unavailable for this region. An unknown timing is
     * reported as unknown rather than defaulted to `true`, which would read as an assurance.
     */
    delivered_in_initial_document: boolean | null;
    /** Origin of the document or resource that brought this region in, when it is determinable. */
    initiator_origin: string | null;
    text_length: number;
    imperative_text: ImperativeTextEvidence | null;
}
/** The asymmetric case: agent-directed text from anything other than the first-party document. */
export interface ProvenanceAlert {
    region_id: string;
    classification: ProvenanceClassification;
    origin: string;
    markers: string[];
    sample: string;
    message: string;
}
/** Provenance as it rides along with the content an extraction returned. */
export interface ContentProvenance {
    /** False when no attribution could be made; the regions list is then empty, not optimistic. */
    attribution_available: boolean;
    /** Origin of the first-party (top-level) document. */
    document_origin: string;
    is_top_level_document: boolean;
    /** Whether the post-load injection observer was running for this document. */
    injection_tracking_active: boolean;
    regions: ProvenanceRegion[];
    region_counts: Record<ProvenanceClassification, number>;
    /** Origins of scripts and frames added after load: candidate initiators, not a culprit. */
    post_load_script_origins: string[];
    imperative_text_from_non_first_party: ProvenanceAlert[];
    notes: string[];
}
//# sourceMappingURL=provenance-types.d.ts.map