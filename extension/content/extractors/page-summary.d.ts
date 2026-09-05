import type { ContentProvenance } from '../../lib/provenance/provenance-types.js';
/**
 * Result shape returned by extractPageSummary.
 */
export interface PageSummaryResult {
    url: string;
    title: string;
    type: string;
    headings: string[];
    nav_links: Array<{
        text: string;
        href: string;
    }>;
    forms: Array<{
        action: string;
        method: string;
        fields: string[];
    }>;
    interactive_element_count: number;
    main_content_preview: string;
    word_count: number;
    /** Where these bytes came from: frame, origin, and whether they were in the initial document. */
    provenance: ContentProvenance;
}
/**
 * Extract a structured page summary from the current page.
 * Returns headings, navigation links, forms, interactive count, content preview, and classification.
 */
export declare function extractPageSummary(): PageSummaryResult;
//# sourceMappingURL=page-summary.d.ts.map