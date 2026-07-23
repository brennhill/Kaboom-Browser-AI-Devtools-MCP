/**
 * Result shape returned by extractReadable.
 */
export interface ReadableResult {
    title: string;
    content: string;
    excerpt: string;
    byline: string;
    word_count: number;
    url: string;
}
/**
 * Extract readable content from the current page.
 * Returns structured data with title, content, excerpt, byline, word count, and URL.
 */
export declare function extractReadable(): ReadableResult;
//# sourceMappingURL=readable.d.ts.map