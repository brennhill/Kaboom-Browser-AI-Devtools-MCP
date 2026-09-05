/**
 * Purpose: The content script's entry point to content provenance — one tracker per document,
 *          and one call that attributes whatever a extractor is about to return.
 * Why: Provenance the agent has to make a second call to fetch is provenance it will not fetch,
 *      so every extractor reaches this from the same place and ships attribution in its payload.
 * Docs: docs/features/feature/content-provenance/index.md
 */
import type { ContentProvenance } from '../../lib/provenance/provenance-types.js';
/**
 * Start watching this document for post-load insertions. Idempotent.
 *
 * Provenance reports; it is never load-bearing. This runs first in the content script's bootstrap,
 * so a throw here — a context with no MutationObserver, a document with no documentElement — would
 * take the message listeners and request tracking down with it and leave the tab undrivable. The
 * failure is reported and the bootstrap continues; `wasInjectedAfterLoad` then answers `null`
 * (unknown timing) rather than claiming content was in the initial document.
 */
export declare function initContentProvenance(): void;
/** Stop watching, so later queries report timing as unknown rather than as initial content. */
export declare function stopContentProvenance(): void;
/** Attribute the regions of `root`, for inclusion in the extraction response that carries its text. */
export declare function provenanceForExtraction(root: Element | null): ContentProvenance;
//# sourceMappingURL=index.d.ts.map