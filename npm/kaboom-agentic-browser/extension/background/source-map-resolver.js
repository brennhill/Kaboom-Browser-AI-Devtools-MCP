/**
 * Purpose: Handles extension background coordination and message routing.
 * Docs: docs/features/feature/analyze-tool/index.md
 * Docs: docs/features/feature/interact-explore/index.md
 * Docs: docs/features/feature/observe/index.md
 */
/**
 * @fileoverview Source Maps and Stack Trace Resolution
 * Handles source map fetching and caching, stack frame parsing,
 * VLQ decoding, and stack trace resolution for better error messages.
 */
import { getSourceMapCacheEntry, setSourceMapCacheEntry, isSourceMapEnabled } from './cache-limits.js';
// =============================================================================
// CONSTANTS
// =============================================================================
/** Source map fetch timeout */
const SOURCE_MAP_FETCH_TIMEOUT = 5000;
/** Context annotation thresholds */
const CONTEXT_SIZE_THRESHOLD = 20 * 1024;
const CONTEXT_WARNING_WINDOW_MS = 60000;
const CONTEXT_WARNING_COUNT = 3;
/** Debug log buffer size */
const DEBUG_LOG_MAX_ENTRIES = 200;
/** Processing query TTL */
const PROCESSING_QUERY_TTL_MS = 60000;
/** Stack frame regex patterns */
const STACK_FRAME_REGEX = /^\s*at\s+(?:(.+?)\s+\()?(?:(.+?):(\d+):(\d+)|(.+?):(\d+))\)?$/;
const ANONYMOUS_FRAME_REGEX = /^\s*at\s+(.+?):(\d+):(\d+)$/;
/** VLQ character mapping */
const VLQ_CHARS = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/';
const VLQ_CHAR_MAP = new Map(VLQ_CHARS.split('').map((c, i) => [c, i]));
// =============================================================================
// STATE
// =============================================================================
/** Context annotation monitoring state */
let contextExcessiveTimestamps = [];
let contextWarningState = null;
/** Processing queries tracking */
const processingQueries = new Map();
// =============================================================================
// CONTEXT ANNOTATION MONITORING
// =============================================================================
/**
 * Measure the serialized byte size of _context in a log entry
 */
export function measureContextSize(entry) {
    const context = entry._context;
    if (!context || typeof context !== 'object')
        return 0;
    const keys = Object.keys(context);
    if (keys.length === 0)
        return 0;
    return JSON.stringify(context).length;
}
/**
 * Check a batch of entries for excessive context annotation usage
 */
export function checkContextAnnotations(entries) {
    const now = Date.now();
    for (const entry of entries) {
        const size = measureContextSize(entry);
        if (size > CONTEXT_SIZE_THRESHOLD) {
            contextExcessiveTimestamps.push({ ts: now, size });
        }
    }
    contextExcessiveTimestamps = contextExcessiveTimestamps.filter((t) => now - t.ts < CONTEXT_WARNING_WINDOW_MS);
    if (contextExcessiveTimestamps.length >= CONTEXT_WARNING_COUNT) {
        const avgSize = contextExcessiveTimestamps.reduce((sum, t) => sum + t.size, 0) / contextExcessiveTimestamps.length;
        contextWarningState = {
            sizeKB: Math.round(avgSize / 1024),
            count: contextExcessiveTimestamps.length,
            triggeredAt: now
        };
    }
    else if (contextWarningState && contextExcessiveTimestamps.length === 0) {
        contextWarningState = null;
    }
}
/**
 * Get the current context annotation warning state
 */
export function getContextWarning() {
    return contextWarningState;
}
/**
 * Reset the context annotation warning (for testing)
 */
export function resetContextWarning() {
    contextExcessiveTimestamps = [];
    contextWarningState = null;
}
// =============================================================================
// VLQ DECODING AND SOURCE MAP PARSING
// =============================================================================
/**
 * Decode a VLQ-encoded string into an array of integers
 */
export function decodeVLQ(str) {
    const result = [];
    let shift = 0;
    let value = 0;
    for (const char of str) {
        const digit = VLQ_CHAR_MAP.get(char);
        if (digit === undefined) {
            throw new Error(`Invalid VLQ character: ${char}`);
        }
        const continued = digit & 32;
        value += (digit & 31) << shift;
        if (continued) {
            shift += 5;
        }
        else {
            const negate = value & 1;
            value = value >> 1;
            result.push(negate ? -value : value);
            value = 0;
            shift = 0;
        }
    }
    return result;
}
/**
 * Parse a source map's mappings string into a structured format
 */
export function parseMappings(mappingsStr) {
    const lines = mappingsStr.split(';');
    const parsed = [];
    for (const line of lines) {
        const segments = [];
        if (line.length > 0) {
            const segmentStrs = line.split(',');
            for (const segmentStr of segmentStrs) {
                if (segmentStr.length > 0) {
                    segments.push(decodeVLQ(segmentStr));
                }
            }
        }
        parsed.push(segments);
    }
    return parsed;
}
/**
 * Parse a stack trace line into components
 */
export function parseStackFrame(line) {
    const match = line.match(STACK_FRAME_REGEX);
    if (match) {
        const [, functionName, file1, line1, col1, file2, line2] = match;
        return {
            functionName: functionName || '<anonymous>',
            fileName: file1 || file2 || '',
            lineNumber: parseInt(line1 || line2 || '0', 10),
            columnNumber: col1 ? parseInt(col1, 10) : 0,
            raw: line
        };
    }
    const anonMatch = line.match(ANONYMOUS_FRAME_REGEX);
    if (anonMatch) {
        return {
            functionName: '<anonymous>',
            fileName: anonMatch[1] || '',
            lineNumber: parseInt(anonMatch[2] || '0', 10),
            columnNumber: parseInt(anonMatch[3] || '0', 10),
            raw: line
        };
    }
    return null;
}
/**
 * Extract sourceMappingURL from script content
 */
export function extractSourceMapUrl(content) {
    const regex = /\/\/[#@]\s*sourceMappingURL=(.+?)(?:\s|$)/;
    const match = content.match(regex);
    return match && match[1] ? match[1].trim() : null;
}
/**
 * Parse source map data into a usable format
 */
export function parseSourceMapData(sourceMap) {
    const mappings = parseMappings(sourceMap.mappings || '');
    return {
        sources: sourceMap.sources || [],
        names: sourceMap.names || [],
        sourceRoot: sourceMap.sourceRoot || '',
        mappings,
        sourcesContent: sourceMap.sourcesContent || []
    };
}
/**
 * Find original location from source map
 */
// #lizard forgives
export function findOriginalLocation(sourceMap, line, column) {
    if (!sourceMap || !sourceMap.mappings)
        return null;
    const lineIndex = line - 1;
    if (lineIndex < 0 || lineIndex >= sourceMap.mappings.length)
        return null;
    const lineSegments = sourceMap.mappings[lineIndex];
    if (!lineSegments || lineSegments.length === 0)
        return null;
    let genCol = 0;
    let sourceIndex = 0;
    let origLine = 0;
    let origCol = 0;
    let nameIndex = 0;
    let bestMatch = null;
    for (let li = 0; li <= lineIndex; li++) {
        genCol = 0;
        const segments = sourceMap.mappings[li];
        if (!segments)
            continue;
        for (const segment of segments) {
            if (segment.length >= 1)
                genCol += segment[0];
            if (segment.length >= 2)
                sourceIndex += segment[1];
            if (segment.length >= 3)
                origLine += segment[2];
            if (segment.length >= 4)
                origCol += segment[3];
            if (segment.length >= 5)
                nameIndex += segment[4];
            if (li === lineIndex && genCol <= column) {
                bestMatch = {
                    source: sourceMap.sources[sourceIndex] || '',
                    line: origLine + 1,
                    column: origCol,
                    name: segment.length >= 5 ? sourceMap.names[nameIndex] || null : null
                };
            }
        }
    }
    return bestMatch;
}
function cacheNullAndReturn(scriptUrl) {
    setSourceMapCacheEntry(scriptUrl, null);
    return null;
}
async function fetchWithTimeout(url) {
    const controller = new AbortController();
    const timeoutId = setTimeout(() => controller.abort(), SOURCE_MAP_FETCH_TIMEOUT);
    try {
        const response = await fetch(url, { signal: controller.signal });
        return response;
    }
    finally {
        clearTimeout(timeoutId);
    }
}
function parseInlineSourceMap(dataUrl, scriptUrl, debugLogFn) {
    const base64Match = dataUrl.match(/^data:application\/json;base64,(.+)$/);
    if (!base64Match || !base64Match[1])
        return cacheNullAndReturn(scriptUrl);
    let jsonStr;
    try {
        jsonStr = atob(base64Match[1]);
    }
    catch {
        if (debugLogFn)
            debugLogFn('sourcemap', 'Invalid base64 in inline source map', { scriptUrl });
        return cacheNullAndReturn(scriptUrl);
    }
    let sourceMap;
    try {
        sourceMap = JSON.parse(jsonStr);
    }
    catch {
        if (debugLogFn)
            debugLogFn('sourcemap', 'Invalid JSON in inline source map', { scriptUrl });
        return cacheNullAndReturn(scriptUrl);
    }
    const parsed = parseSourceMapData(sourceMap);
    setSourceMapCacheEntry(scriptUrl, parsed);
    return parsed;
}
async function fetchExternalSourceMap(sourceMapUrl, scriptUrl, debugLogFn) {
    let resolvedUrl = sourceMapUrl;
    if (!resolvedUrl.startsWith('http')) {
        const base = scriptUrl.substring(0, scriptUrl.lastIndexOf('/') + 1);
        resolvedUrl = new URL(resolvedUrl, base).href;
    }
    const mapResponse = await fetchWithTimeout(resolvedUrl);
    if (!mapResponse.ok)
        return cacheNullAndReturn(scriptUrl);
    let sourceMap;
    try {
        sourceMap = await mapResponse.json();
    }
    catch {
        if (debugLogFn)
            debugLogFn('sourcemap', 'Invalid JSON in external source map', { scriptUrl, sourceMapUrl: resolvedUrl });
        return cacheNullAndReturn(scriptUrl);
    }
    const parsed = parseSourceMapData(sourceMap);
    setSourceMapCacheEntry(scriptUrl, parsed);
    return parsed;
}
export async function fetchSourceMap(scriptUrl, debugLogFn) {
    if (getSourceMapCacheEntry(scriptUrl)) {
        return getSourceMapCacheEntry(scriptUrl) || null;
    }
    try {
        const scriptResponse = await fetchWithTimeout(scriptUrl);
        if (!scriptResponse.ok)
            return cacheNullAndReturn(scriptUrl);
        const scriptContent = await scriptResponse.text();
        const sourceMapUrl = extractSourceMapUrl(scriptContent);
        if (!sourceMapUrl)
            return cacheNullAndReturn(scriptUrl);
        if (sourceMapUrl.startsWith('data:')) {
            return parseInlineSourceMap(sourceMapUrl, scriptUrl, debugLogFn);
        }
        return fetchExternalSourceMap(sourceMapUrl, scriptUrl, debugLogFn);
    }
    catch (err) {
        if (debugLogFn) {
            debugLogFn('sourcemap', 'Source map fetch failed', {
                scriptUrl,
                error: err.message
            });
        }
        return cacheNullAndReturn(scriptUrl);
    }
}
/**
 * Resolve a single stack frame to original location
 */
export async function resolveStackFrame(frame, debugLogFn) {
    if (!frame.fileName || !frame.fileName.startsWith('http')) {
        return frame;
    }
    const sourceMap = await fetchSourceMap(frame.fileName, debugLogFn);
    if (!sourceMap) {
        return frame;
    }
    const original = findOriginalLocation(sourceMap, frame.lineNumber, frame.columnNumber);
    if (!original) {
        return frame;
    }
    return {
        ...frame,
        originalFileName: original.source,
        originalLineNumber: original.line,
        originalColumnNumber: original.column,
        originalFunctionName: original.name || frame.functionName,
        resolved: true
    };
}
/**
 * Resolve an entire stack trace
 */
export async function resolveStackTrace(stack, debugLogFn) {
    if (!stack || !isSourceMapEnabled())
        return stack;
    const lines = stack.split('\n');
    const resolvedLines = [];
    for (const line of lines) {
        const frame = parseStackFrame(line);
        if (!frame) {
            resolvedLines.push(line);
            continue;
        }
        try {
            const resolved = await resolveStackFrame(frame, debugLogFn);
            if (resolved.resolved) {
                const funcName = resolved.originalFunctionName || resolved.functionName;
                const fileName = resolved.originalFileName;
                const lineNum = resolved.originalLineNumber;
                const colNum = resolved.originalColumnNumber;
                resolvedLines.push(`    at ${funcName} (${fileName}:${lineNum}:${colNum}) [resolved from ${resolved.fileName}:${resolved.lineNumber}:${resolved.columnNumber}]`);
            }
            else {
                resolvedLines.push(line);
            }
        }
        catch {
            resolvedLines.push(line);
        }
    }
    return resolvedLines.join('\n');
}
// =============================================================================
// PROCESSING QUERY TRACKING
// =============================================================================
/**
 * Get current state of processing queries (for testing)
 */
export function getProcessingQueriesState() {
    return processingQueries;
}
/**
 * Add a query to the processing set with timestamp
 */
export function addProcessingQuery(queryId, timestamp = Date.now()) {
    processingQueries.set(queryId, timestamp);
}
/**
 * Remove a query from the processing set
 */
export function removeProcessingQuery(queryId) {
    processingQueries.delete(queryId);
}
/**
 * Check if a query is currently being processed
 */
export function isQueryProcessing(queryId) {
    return processingQueries.has(queryId);
}
/**
 * Clean up stale processing queries that have exceeded the TTL
 */
export function cleanupStaleProcessingQueries(debugLogFn) {
    const now = Date.now();
    for (const [queryId, timestamp] of processingQueries) {
        if (now - timestamp > PROCESSING_QUERY_TTL_MS) {
            processingQueries.delete(queryId);
            if (debugLogFn) {
                debugLogFn('connection', 'Cleaned up stale processing query', {
                    queryId,
                    age: Math.round((now - timestamp) / 1000) + 's'
                });
            }
        }
    }
}
//# sourceMappingURL=source-map-resolver.js.map