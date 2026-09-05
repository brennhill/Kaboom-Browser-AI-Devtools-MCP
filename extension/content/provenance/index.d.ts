/**
 * Purpose: The content script's entry point to content provenance — one tracker per document,
 *          and one call that attributes whatever a extractor is about to return.
 * Why: Provenance the agent has to make a second call to fetch is provenance it will not fetch,
 *      so every extractor reaches this from the same place and ships attribution in its payload.
 * Docs: docs/features/feature/content-provenance/index.md
 */
import type { ContentProvenance } from '../../lib/provenance/provenance-types.js';
/** Start watching this document for post-load insertions. Idempotent. */
export declare function initContentProvenance(): void;
/** Stop watching, so later queries report timing as unknown rather than as initial content. */
export declare function stopContentProvenance(): void;
/** Attribute the regions of `root`, for inclusion in the extraction response that carries its text. */
export declare function provenanceForExtraction(root: Element | null): ContentProvenance;
//# sourceMappingURL=index.d.ts.map