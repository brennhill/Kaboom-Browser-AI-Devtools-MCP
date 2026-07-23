/**
 * Result shape returned by extractMarkdown.
 */
export interface MarkdownResult {
    title: string;
    markdown: string;
    word_count: number;
    url: string;
    truncated?: boolean;
}
/**
 * Extract page content and convert to Markdown.
 * Returns structured data with title, markdown content, word count, and URL.
 * Output is capped at MAX_OUTPUT_CHARS to prevent memory pressure on large pages.
 */
export declare function extractMarkdown(): MarkdownResult;
//# sourceMappingURL=markdown.d.ts.map