/**
 * Purpose: Parses Chrome/Firefox stack traces into structured frames, resolves inline source maps, and extracts code snippets around error lines.
 * Docs: docs/features/feature/error-bundling/index.md
 */

// ai-context-parsing.ts — Stack trace parsing, source map resolution, and code snippet extraction.

/**
 * @fileoverview AI context foundation layer.
 * Parses Chrome/Firefox stack traces into structured frames, resolves inline
 * base64 source maps, extracts code snippets around error lines, and manages
 * an LRU source map cache.
 */

import type { StackFrame, SourceSnippet } from '../../types/capture/ai-context.js'
import type { ParsedSourceMap } from '../../types/capture/sourcemap.js'

import {
  AI_CONTEXT_SNIPPET_LINES,
  AI_CONTEXT_MAX_LINE_LENGTH,
  AI_CONTEXT_MAX_SNIPPETS_SIZE,
  AI_CONTEXT_SOURCE_MAP_CACHE_SIZE
} from '../constants.js'

// =============================================================================
// TYPE DEFINITIONS
// =============================================================================

/**
 * Parsed stack frame (internal representation with nullable functionName)
 */
export interface InternalStackFrame {
  functionName: string | null
  filename: string
  lineno: number
  colno: number
}

/**
 * Code snippet line entry
 */
export interface SnippetLine {
  line: number
  text: string
  isError?: boolean
}

/**
 * Source snippet with file and line info
 */
export interface InternalSourceSnippet {
  file: string
  line: number
  snippet: SnippetLine[]
}

/**
 * Source map lookup for extractSourceSnippets
 */
export type SourceMapLookup = Record<string, ParsedSourceMap>

// =============================================================================
// MODULE STATE
// =============================================================================

const aiSourceMapCache = new Map<string, ParsedSourceMap>()

// =============================================================================
// STACK FRAME PARSING
// =============================================================================

/**
 * Parse stack trace into structured frames
 * Supports Chrome and Firefox formats
 * @param stack - The stack trace string
 * @returns Array of frame objects { functionName, filename, lineno, colno }
 */
type FrameParser = (line: string) => InternalStackFrame | null

const CHROME_FRAME_RE = /^at\s+(?:(.+?)\s+\()?(.+?):(\d+):(\d+)\)?$/
const FIREFOX_FRAME_RE = /^(.+?)@(.+?):(\d+):(\d+)$/

function parseMatchedFrame(line: string, pattern: RegExp): InternalStackFrame | null {
  const m = line.match(pattern)
  if (!m) return null
  const filename = m[2]
  if (!filename || filename.includes('<anonymous>')) return null
  if (!m[3] || !m[4]) return null
  return { functionName: m[1] || null, filename, lineno: parseInt(m[3], 10), colno: parseInt(m[4], 10) }
}

function parseChromeFrame(line: string): InternalStackFrame | null {
  return parseMatchedFrame(line, CHROME_FRAME_RE)
}

function parseFirefoxFrame(line: string): InternalStackFrame | null {
  return parseMatchedFrame(line, FIREFOX_FRAME_RE)
}

const FRAME_PARSERS: FrameParser[] = [parseChromeFrame, parseFirefoxFrame]

export function parseStackFrames(stack: string | undefined): InternalStackFrame[] {
  if (!stack) return []

  const frames: InternalStackFrame[] = []
  for (const line of stack.split('\n')) {
    const trimmed = line.trim()
    for (const parser of FRAME_PARSERS) {
      const frame = parser(trimmed)
      if (frame) {
        frames.push(frame)
        break
      }
    }
  }
  return frames
}

// =============================================================================
// SOURCE MAP PARSING
// =============================================================================

/**
 * Parse an inline base64 source map data URL
 * @param dataUrl - The data: URL containing the source map
 * @returns Parsed source map or null
 */
export function parseSourceMap(dataUrl: string | undefined | null): ParsedSourceMap | null {
  if (!dataUrl || typeof dataUrl !== 'string') return null
  if (!dataUrl.startsWith('data:')) return null

  try {
    // Extract base64 content after the last comma
    const base64Match = dataUrl.match(/;base64,(.+)$/)
    if (!base64Match || !base64Match[1]) return null

    const decoded = atob(base64Match[1])
    const parsed = JSON.parse(decoded) as ParsedSourceMap

    // Only useful if it has sourcesContent
    if (!parsed.sourcesContent || parsed.sourcesContent.length === 0) return null

    return parsed
  } catch {
    return null
  }
}

// =============================================================================
// CODE SNIPPET EXTRACTION
// =============================================================================

/**
 * Extract a code snippet around a given line number
 * @param sourceContent - The full source file content
 * @param line - The 1-based line number of the error
 * @returns Array of { line, text, isError? } or null
 */
export function extractSnippet(sourceContent: string | undefined | null, line: number): SnippetLine[] | null {
  if (!sourceContent || typeof sourceContent !== 'string') return null
  if (!line || line < 1) return null

  const lines = sourceContent.split('\n')
  if (line > lines.length) return null

  const start = Math.max(0, line - 1 - AI_CONTEXT_SNIPPET_LINES)
  const end = Math.min(lines.length, line + AI_CONTEXT_SNIPPET_LINES)

  const snippet: SnippetLine[] = []
  for (let i = start; i < end; i++) {
    let text = lines[i]
    if (!text) continue
    if (text.length > AI_CONTEXT_MAX_LINE_LENGTH) {
      text = text.slice(0, AI_CONTEXT_MAX_LINE_LENGTH)
    }
    const entry: SnippetLine = { line: i + 1, text }
    if (i + 1 === line) entry.isError = true
    snippet.push(entry)
  }

  return snippet
}

/**
 * Extract source snippets for multiple stack frames
 * @param frames - Parsed stack frames
 * @param mockSourceMaps - Map of filename to parsed source map
 * @returns Array of snippet objects
 */
export async function extractSourceSnippets(
  frames: InternalStackFrame[],
  mockSourceMaps: SourceMapLookup
): Promise<InternalSourceSnippet[]> {
  // SOURCE MAP CACHING STRATEGY:
  // This function works with a mockSourceMaps lookup that is pre-populated by
  // resolveSourceMap(). The caching layer is managed separately via the module-level
  // aiSourceMapCache Map, which stores up to AI_CONTEXT_SOURCE_MAP_CACHE_SIZE entries
  // using LRU eviction. When a source map is needed here, it should already be cached
  // by the MCP observe handler that parsed the HTTP response headers.
  //
  // OPTIMIZATION: We only process the top 3 stack frames to limit computation and avoid
  // redundant snippets. Most stack traces have the root cause in the first 1-3 frames.
  //
  // PARSE ERROR HANDLING: If sourcesContent is missing, we skip the frame entirely
  // rather than erroring. This gracefully handles source maps generated without embedded
  // sources (which only contain mappings, not code). We never throw here.
  //
  // SIZE ENFORCEMENT: Total snippets are capped at AI_CONTEXT_MAX_SNIPPETS_SIZE to prevent
  // bloating the error entry. Each snippet's JSON serialized size is checked before adding.
  // This ensures the enriched error entry stays lightweight for AI processing.

  const snippets: InternalSourceSnippet[] = []
  let totalSize = 0

  for (const frame of frames.slice(0, 3)) {
    if (totalSize >= AI_CONTEXT_MAX_SNIPPETS_SIZE) break

    const sourceMap = mockSourceMaps[frame.filename]
    if (!sourceMap || !sourceMap.sourcesContent || !sourceMap.sourcesContent[0]) continue

    const snippet = extractSnippet(sourceMap.sourcesContent[0], frame.lineno)
    if (!snippet) continue

    const snippetObj: InternalSourceSnippet = { file: frame.filename, line: frame.lineno, snippet }
    const snippetSize = JSON.stringify(snippetObj).length

    if (totalSize + snippetSize > AI_CONTEXT_MAX_SNIPPETS_SIZE) break

    totalSize += snippetSize
    snippets.push(snippetObj)
  }

  return snippets
}

// =============================================================================
// SOURCE MAP CACHE
// =============================================================================

/**
 * Cache a parsed source map for a URL
 * @param url - The script URL
 * @param map - The parsed source map
 */
export function setSourceMapCache(url: string, map: ParsedSourceMap): void {
  // Evict oldest if adding new entry and at capacity
  if (!aiSourceMapCache.has(url) && aiSourceMapCache.size >= AI_CONTEXT_SOURCE_MAP_CACHE_SIZE) {
    const firstKey = aiSourceMapCache.keys().next().value
    if (firstKey) {
      aiSourceMapCache.delete(firstKey)
    }
  }
  // Move to end (LRU): delete first if exists, then add
  // This ensures recently accessed/updated entries are kept longest
  aiSourceMapCache.delete(url)
  aiSourceMapCache.set(url, map)
}

/**
 * Get a cached source map
 * @param url - The script URL
 * @returns The cached source map or null
 */
export function getSourceMapCache(url: string): ParsedSourceMap | null {
  return aiSourceMapCache.get(url) || null
}

/**
 * Get the number of cached source maps
 * @returns
 */
export function getSourceMapCacheSize(): number {
  return aiSourceMapCache.size
}

/**
 * Reset parsing module state for testing purposes.
 * Clears source map cache.
 */
export function resetParsingForTesting(): void {
  aiSourceMapCache.clear()
}
