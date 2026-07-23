/**
 * Purpose: Parses Chrome/Firefox stack traces into structured frames, resolves inline source maps, and extracts code snippets around error lines.
 * Docs: docs/features/feature/error-bundling/index.md
 */
/**
 * @fileoverview AI context foundation layer.
 * Parses Chrome/Firefox stack traces into structured frames, resolves inline
 * base64 source maps, extracts code snippets around error lines, and manages
 * an LRU source map cache.
 */
import type { ParsedSourceMap } from '../types/index.js';
/**
 * Parsed stack frame (internal representation with nullable functionName)
 */
export interface InternalStackFrame {
    functionName: string | null;
    filename: string;
    lineno: number;
    colno: number;
}
/**
 * Code snippet line entry
 */
export interface SnippetLine {
    line: number;
    text: string;
    isError?: boolean;
}
/**
 * Source snippet with file and line info
 */
export interface InternalSourceSnippet {
    file: string;
    line: number;
    snippet: SnippetLine[];
}
/**
 * Source map lookup for extractSourceSnippets
 */
export type SourceMapLookup = Record<string, ParsedSourceMap>;
export declare function parseStackFrames(stack: string | undefined): InternalStackFrame[];
/**
 * Parse an inline base64 source map data URL
 * @param dataUrl - The data: URL containing the source map
 * @returns Parsed source map or null
 */
export declare function parseSourceMap(dataUrl: string | undefined | null): ParsedSourceMap | null;
/**
 * Extract a code snippet around a given line number
 * @param sourceContent - The full source file content
 * @param line - The 1-based line number of the error
 * @returns Array of { line, text, isError? } or null
 */
export declare function extractSnippet(sourceContent: string | undefined | null, line: number): SnippetLine[] | null;
/**
 * Extract source snippets for multiple stack frames
 * @param frames - Parsed stack frames
 * @param mockSourceMaps - Map of filename to parsed source map
 * @returns Array of snippet objects
 */
export declare function extractSourceSnippets(frames: InternalStackFrame[], mockSourceMaps: SourceMapLookup): Promise<InternalSourceSnippet[]>;
/**
 * Cache a parsed source map for a URL
 * @param url - The script URL
 * @param map - The parsed source map
 */
export declare function setSourceMapCache(url: string, map: ParsedSourceMap): void;
/**
 * Get a cached source map
 * @param url - The script URL
 * @returns The cached source map or null
 */
export declare function getSourceMapCache(url: string): ParsedSourceMap | null;
/**
 * Get the number of cached source maps
 * @returns
 */
export declare function getSourceMapCacheSize(): number;
/**
 * Reset parsing module state for testing purposes.
 * Clears source map cache.
 */
export declare function resetParsingForTesting(): void;
//# sourceMappingURL=ai-context-parsing.d.ts.map