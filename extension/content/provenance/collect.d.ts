/**
 * Purpose: Attribute the regions of an extracted page to the document, frame, or post-load
 *          insertion that delivered them.
 * Why: An extraction hands the agent one block of text. This says which parts of it came from the
 *      page the user asked for, which came from somebody else's frame, and which were written into
 *      the page after it finished loading — reported as evidence, never filtered or rewritten.
 * Docs: docs/features/feature/content-provenance/index.md
 */
import type { ContentProvenance } from '../../lib/provenance/provenance-types.js';
import type { InjectionQuery } from './post-load-tracker.js';
/** Where the collector is running and what it can ask about delivery timing. */
export interface ProvenanceEnvironment {
    /** Origin of the first-party (top-level) document. */
    document_origin: string;
    /** Origin of the document this collector is running in. */
    frame_origin: string;
    /** Absolute URL of that document, used only to resolve relative frame sources. */
    frame_href: string;
    is_top_level_document: boolean;
    tracker: InjectionQuery;
}
/**
 * Attribute an extraction root, reporting one region per document, frame, and post-load insertion.
 *
 * This reports. It does not filter, block, or rewrite content: what to do with the evidence stays
 * with the agent and the person whose browser it is.
 */
export declare function collectContentProvenance(root: Element | null, env: ProvenanceEnvironment): ContentProvenance;
//# sourceMappingURL=collect.d.ts.map