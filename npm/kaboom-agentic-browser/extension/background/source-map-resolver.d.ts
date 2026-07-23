/**
 * Purpose: Handles extension background coordination and message routing.
 * Docs: docs/features/feature/analyze-tool/index.md
 * Docs: docs/features/feature/interact-explore/index.md
 * Docs: docs/features/feature/observe/index.md
 */
import type { LogEntry, ParsedSourceMap, ContextWarning } from '../types';
/** Parsed stack frame */
interface ParsedStackFrame {
    functionName: string;
    fileName: string;
    lineNumber: number;
    columnNumber: number;
    raw: string;
    originalFileName?: string;
    originalLineNumber?: number;
    originalColumnNumber?: number;
    originalFunctionName?: string;
    resolved?: boolean;
}
/** Original location from source map */
interface OriginalLocation {
    source: string;
    line: number;
    column: number;
    name: string | null;
}
/**
 * Measure the serialized byte size of _context in a log entry
 */
export declare function measureContextSize(entry: LogEntry): number;
/**
 * Check a batch of entries for excessive context annotation usage
 */
export declare function checkContextAnnotations(entries: LogEntry[]): void;
/**
 * Get the current context annotation warning state
 */
export declare function getContextWarning(): ContextWarning | null;
/**
 * Reset the context annotation warning (for testing)
 */
export declare function resetContextWarning(): void;
/**
 * Decode a VLQ-encoded string into an array of integers
 */
export declare function decodeVLQ(str: string): number[];
/**
 * Parse a source map's mappings string into a structured format
 */
export declare function parseMappings(mappingsStr: string): number[][][];
/**
 * Parse a stack trace line into components
 */
export declare function parseStackFrame(line: string): ParsedStackFrame | null;
/**
 * Extract sourceMappingURL from script content
 */
export declare function extractSourceMapUrl(content: string): string | null;
/**
 * Parse source map data into a usable format
 */
export declare function parseSourceMapData(sourceMap: {
    mappings?: string;
    sources?: string[];
    names?: string[];
    sourceRoot?: string;
    sourcesContent?: string[];
}): ParsedSourceMap;
/**
 * Find original location from source map
 */
export declare function findOriginalLocation(sourceMap: ParsedSourceMap, line: number, column: number): OriginalLocation | null;
type DebugLogFn = (category: string, message: string, data?: unknown) => void;
export declare function fetchSourceMap(scriptUrl: string, debugLogFn?: DebugLogFn): Promise<ParsedSourceMap | null>;
/**
 * Resolve a single stack frame to original location
 */
export declare function resolveStackFrame(frame: ParsedStackFrame, debugLogFn?: (category: string, message: string, data?: unknown) => void): Promise<ParsedStackFrame>;
/**
 * Resolve an entire stack trace
 */
export declare function resolveStackTrace(stack: string, debugLogFn?: (category: string, message: string, data?: unknown) => void): Promise<string>;
/**
 * Get current state of processing queries (for testing)
 */
export declare function getProcessingQueriesState(): Map<string, number>;
/**
 * Add a query to the processing set with timestamp
 */
export declare function addProcessingQuery(queryId: string, timestamp?: number): void;
/**
 * Remove a query from the processing set
 */
export declare function removeProcessingQuery(queryId: string): void;
/**
 * Check if a query is currently being processed
 */
export declare function isQueryProcessing(queryId: string): boolean;
/**
 * Clean up stale processing queries that have exceeded the TTL
 */
export declare function cleanupStaleProcessingQueries(debugLogFn?: (category: string, message: string, data?: unknown) => void): void;
export {};
//# sourceMappingURL=source-map-resolver.d.ts.map