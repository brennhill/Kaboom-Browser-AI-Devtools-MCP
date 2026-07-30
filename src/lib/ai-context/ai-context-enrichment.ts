/**
 * Purpose: Detects UI frameworks (React/Vue/Svelte), captures state snapshots, and generates AI-friendly error summaries with timeout guards.
 * Docs: docs/features/feature/error-bundling/index.md
 */

// ai-context-enrichment.ts — Framework detection, state capture, and AI error enrichment pipeline.

/**
 * @fileoverview Runtime introspection for AI error context.
 * Detects UI frameworks (React/Vue/Svelte), walks React fiber trees,
 * captures Redux state snapshots, generates AI summaries, and orchestrates
 * the full error enrichment pipeline with timeout guards.
 */

import type { LogEntry } from '../../types/capture/telemetry.js'
import type { AiContextData } from '../../types/capture/ai-context.js'

import {
  AI_CONTEXT_MAX_ANCESTRY_DEPTH,
  AI_CONTEXT_MAX_PROP_KEYS,
  AI_CONTEXT_MAX_STATE_KEYS,
  AI_CONTEXT_MAX_RELEVANT_SLICE,
  AI_CONTEXT_MAX_VALUE_LENGTH,
  AI_CONTEXT_PIPELINE_TIMEOUT_MS
} from '../constants.js'

import { parseStackFrames, extractSourceSnippets, getSourceMapCache } from './ai-context-parsing.js'

// =============================================================================
// TYPE DEFINITIONS
// =============================================================================

/**
 * Framework detection result
 */
export interface FrameworkDetection {
  framework: 'react' | 'vue' | 'svelte'
  key?: string
}

/**
 * React component ancestry entry
 */
export interface ReactComponentEntry {
  name: string
  propKeys?: string[]
  hasState?: boolean
  stateKeys?: string[]
}

/**
 * React fiber node (partial typing for what we access)
 */
interface ReactFiber {
  type?:
    | {
        displayName?: string
        name?: string
      }
    | string
  memoizedProps?: Record<string, unknown>
  memoizedState?: Record<string, unknown> | unknown[] | null
  return?: ReactFiber | null
}

/**
 * Component ancestry result
 */
export interface ComponentAncestryResult {
  framework: 'react'
  components: ReactComponentEntry[]
}

/**
 * Redux store interface
 */
interface ReduxStore {
  getState: () => Record<string, unknown>
}

/**
 * State snapshot result
 */
export interface StateSnapshotResult {
  source: 'redux'
  keys: Record<string, { type: string }>
  relevantSlice: Record<string, unknown>
}

/**
 * AI summary generation data
 */
export interface AiSummaryData {
  errorType: string
  message: string
  file: string | null
  line: number | null
  componentAncestry: ComponentAncestryResult | null
  stateSnapshot: StateSnapshotResult | null
}

/**
 * Enriched error entry with AI context
 */
type EnrichedErrorEntry = LogEntry & {
  _aiContext?: AiContextData
  _enrichments?: string[]
}

/**
 * Internal AI context result
 */
interface InternalAiContext {
  sourceSnippets?: InternalSourceSnippet[]
  componentAncestry?: ComponentAncestryResult
  stateSnapshot?: StateSnapshotResult
  summary: string
}

/**
 * Code snippet line entry
 */
interface SnippetLine {
  line: number
  text: string
  isError?: boolean
}

/**
 * Source snippet with file and line info
 */
interface InternalSourceSnippet {
  file: string
  line: number
  snippet: SnippetLine[]
}

/**
 * Element with framework markers
 */
interface FrameworkElement {
  __vueParentComponent?: unknown
  __vue_app__?: unknown
  __svelte_meta?: unknown
  [key: string]: unknown
}

/**
 * Error entry for enrichment (partial typing for what we access)
 */
interface ErrorEntryForEnrichment {
  stack?: string
  message?: string
}

// Extend Window interface for Redux store
declare global {
  interface Window {
    __REDUX_STORE__?: ReduxStore
  }
}

// =============================================================================
// MODULE STATE
// =============================================================================

let aiContextEnabled = true
let aiContextStateSnapshotEnabled = false

// =============================================================================
// FRAMEWORK DETECTION
// =============================================================================

/**
 * Detect which UI framework an element belongs to
 * @param element - The DOM element (or element-like object)
 * @returns { framework, key? } or null
 */
export function detectFramework(element: FrameworkElement | null | undefined): FrameworkDetection | null {
  if (!element || typeof element !== 'object') return null

  // React: __reactFiber$ or __reactInternalInstance$
  const keys = Object.keys(element)
  const reactKey = keys.find((k) => k.startsWith('__reactFiber$') || k.startsWith('__reactInternalInstance$'))
  if (reactKey) return { framework: 'react', key: reactKey }

  // Vue 3: __vueParentComponent or __vue_app__
  if (element.__vueParentComponent || element.__vue_app__) {
    return { framework: 'vue' }
  }

  // Svelte: __svelte_meta
  if (element.__svelte_meta) {
    return { framework: 'svelte' }
  }

  return null
}

// =============================================================================
// REACT COMPONENT ANCESTRY
// =============================================================================

/**
 * Walk a React fiber tree to extract component ancestry
 * @param fiber - The React fiber node
 * @returns Array of { name, propKeys?, hasState?, stateKeys? } in root-first order
 */
// #lizard forgives
export function getReactComponentAncestry(fiber: ReactFiber | null | undefined): ReactComponentEntry[] | null {
  if (!fiber) return null

  const ancestry: ReactComponentEntry[] = []
  let current: ReactFiber | null | undefined = fiber
  let depth = 0

  while (current && depth < AI_CONTEXT_MAX_ANCESTRY_DEPTH) {
    depth++

    // Only include component fibers (type is function/object), skip host elements (type is string)
    if (current.type && typeof current.type !== 'string') {
      const typeObj = current.type as { displayName?: string; name?: string }
      const name = typeObj.displayName || typeObj.name || 'Anonymous'
      const entry: ReactComponentEntry = { name }

      // Extract prop keys (excluding children)
      if (current.memoizedProps && typeof current.memoizedProps === 'object') {
        entry.propKeys = Object.keys(current.memoizedProps)
          .filter((k) => k !== 'children')
          .slice(0, AI_CONTEXT_MAX_PROP_KEYS)
      }

      // Extract state keys
      if (current.memoizedState && typeof current.memoizedState === 'object' && !Array.isArray(current.memoizedState)) {
        entry.hasState = true
        entry.stateKeys = Object.keys(current.memoizedState as Record<string, unknown>).slice(
          0,
          AI_CONTEXT_MAX_STATE_KEYS
        )
      }

      ancestry.push(entry)
    }

    current = current.return
  }

  return ancestry.reverse() // Root-first order
}

// =============================================================================
// STATE SNAPSHOT
// =============================================================================

function classifyValueType(value: unknown): string {
  if (Array.isArray(value)) return 'array'
  if (value === null) return 'null'
  return typeof value
}

const RELEVANT_STATE_KEYS = ['error', 'loading', 'status', 'failed']

// #lizard forgives
function buildRelevantSlice(state: Record<string, unknown>, errorWords: string[]): Record<string, unknown> {
  const relevantSlice: Record<string, unknown> = {}
  let sliceCount = 0

  for (const [key, value] of Object.entries(state)) {
    if (sliceCount >= AI_CONTEXT_MAX_RELEVANT_SLICE) break
    if (typeof value !== 'object' || value === null || Array.isArray(value)) continue

    for (const [subKey, subValue] of Object.entries(value as Record<string, unknown>)) {
      if (sliceCount >= AI_CONTEXT_MAX_RELEVANT_SLICE) break
      const isRelevantKey = RELEVANT_STATE_KEYS.some((k) => subKey.toLowerCase().includes(k))
      const isKeywordMatch = errorWords.some((w) => key.toLowerCase().includes(w))
      if (!isRelevantKey && !isKeywordMatch) continue

      let val: unknown = subValue
      if (typeof val === 'string' && val.length > AI_CONTEXT_MAX_VALUE_LENGTH) {
        val = val.slice(0, AI_CONTEXT_MAX_VALUE_LENGTH)
      }
      relevantSlice[`${key}.${subKey}`] = val
      sliceCount++
    }
  }

  return relevantSlice
}

/**
 * Capture application state snapshot from known store patterns.
 *
 * STATE RELEVANCE MATCHING STRATEGY:
 * 1. Extract error keywords from the error message (words > 2 chars).
 * 2. Build a "relevant slice" by matching nested state keys against common error state
 *    keys ('error', 'loading', 'status', 'failed') and error message keywords.
 * 3. Caps at AI_CONTEXT_MAX_RELEVANT_SLICE entries; values truncated at MAX_VALUE_LENGTH.
 *
 * NOTE: Only supports Redux. Other state management would need additional window.__* patterns.
 */
export function captureStateSnapshot(errorMessage: string): StateSnapshotResult | null {
  if (typeof window === 'undefined') return null

  try {
    const store = window.__REDUX_STORE__
    if (!store || typeof store.getState !== 'function') return null

    const state = store.getState()
    if (!state || typeof state !== 'object') return null

    const keys: Record<string, { type: string }> = {}
    for (const [key, value] of Object.entries(state)) {
      keys[key] = { type: classifyValueType(value) }
    }

    const errorWords = (errorMessage || '')
      .toLowerCase()
      .split(/\W+/)
      .filter((w) => w.length > 2)
    const relevantSlice = buildRelevantSlice(state, errorWords)

    return { source: 'redux', keys, relevantSlice }
  } catch {
    return null
  }
}

// =============================================================================
// AI SUMMARY GENERATION
// =============================================================================

/**
 * Generate a template-based AI summary from enrichment data
 * @param data - { errorType, message, file, line, componentAncestry, stateSnapshot }
 * @returns Summary string
 */
export function generateAiSummary(data: AiSummaryData): string {
  const parts: string[] = []

  // Error type and location
  if (data.file && data.line) {
    parts.push(`${data.errorType} in ${data.file}:${data.line} — ${data.message}`)
  } else {
    parts.push(`${data.errorType}: ${data.message}`)
  }

  // Component context
  if (data.componentAncestry && data.componentAncestry.components) {
    const path = data.componentAncestry.components.map((c) => c.name).join(' > ')
    parts.push(`Component tree: ${path}.`)
  }

  // State context
  if (data.stateSnapshot && data.stateSnapshot.relevantSlice) {
    const sliceKeys = Object.keys(data.stateSnapshot.relevantSlice)
    if (sliceKeys.length > 0) {
      const stateInfo = sliceKeys.map((k) => `${k}=${JSON.stringify(data.stateSnapshot!.relevantSlice[k])}`).join(', ') // nosemgrep: no-stringify-keys
      parts.push(`State: ${stateInfo}.`)
    }
  }

  return parts.join(' ')
}

// =============================================================================
// ERROR ENRICHMENT PIPELINE
// =============================================================================

/**
 * Full error enrichment pipeline
 * @param error - The error entry to enrich
 * @returns The enriched error entry
 */
// #lizard forgives
async function buildAiContext(error: ErrorEntryForEnrichment): Promise<InternalAiContext> {
  const result: Partial<InternalAiContext> = {}
  const frames = parseStackFrames(error.stack)

  if (frames.length === 0) return { summary: error.message || 'Unknown error' }
  const topFrame = frames[0]

  // Source snippets (from cache)
  if (topFrame) {
    const cached = getSourceMapCache(topFrame.filename)
    if (cached) {
      const snippets = await extractSourceSnippets(frames, { [topFrame.filename]: cached })
      if (snippets.length > 0) result.sourceSnippets = snippets
    }
  }

  // Component ancestry from activeElement
  result.componentAncestry = extractComponentAncestry() || undefined

  // State snapshot (if enabled)
  if (aiContextStateSnapshotEnabled) {
    const snapshot = captureStateSnapshot(error.message || '')
    if (snapshot) result.stateSnapshot = snapshot
  }

  result.summary = generateAiSummary({
    errorType: error.message?.split(':')[0] || 'Error',
    message: error.message || '',
    file: topFrame?.filename || null,
    line: topFrame?.lineno || null,
    componentAncestry: result.componentAncestry || null,
    stateSnapshot: result.stateSnapshot || null
  })

  return result as InternalAiContext
}

function extractComponentAncestry(): { framework: 'react'; components: ReactComponentEntry[] } | null {
  if (typeof document === 'undefined' || !document.activeElement) return null
  const framework = detectFramework(document.activeElement as unknown as FrameworkElement)
  if (!framework || framework.framework !== 'react' || !framework.key) return null
  const fiber = (document.activeElement as unknown as Record<string, ReactFiber>)[framework.key]
  const components = getReactComponentAncestry(fiber)
  if (!components || components.length === 0) return null
  return { framework: 'react', components }
}

function applyAiContext(enriched: EnrichedErrorEntry, context: InternalAiContext): void {
  enriched._aiContext = context as AiContextData
  if (!enriched._enrichments) enriched._enrichments = []
  enriched._enrichments.push('aiContext')
}

export async function enrichErrorWithAiContext(error: ErrorEntryForEnrichment): Promise<EnrichedErrorEntry> {
  if (!aiContextEnabled) return error as EnrichedErrorEntry

  const enriched: EnrichedErrorEntry = { ...error } as EnrichedErrorEntry

  try {
    const context = await Promise.race<InternalAiContext>([
      buildAiContext(error),
      new Promise<InternalAiContext>((resolve) => {
        setTimeout(() => resolve({ summary: `${error.message || 'Error'}` }), AI_CONTEXT_PIPELINE_TIMEOUT_MS)
      })
    ])
    applyAiContext(enriched, context)
  } catch {
    applyAiContext(enriched, { summary: error.message || 'Unknown error' })
  }

  return enriched
}

// =============================================================================
// CONFIGURATION
// =============================================================================

/**
 * Enable or disable AI context enrichment
 * @param enabled
 */
export function setAiContextEnabled(enabled: boolean): void {
  aiContextEnabled = enabled
}

/**
 * Enable or disable state snapshot in AI context
 * @param enabled
 */
export function setAiContextStateSnapshot(enabled: boolean): void {
  aiContextStateSnapshotEnabled = enabled
}

/**
 * Reset enrichment module state for testing purposes.
 * Call this in beforeEach/afterEach test hooks to prevent test pollution.
 */
export function resetEnrichmentForTesting(): void {
  aiContextEnabled = true
  aiContextStateSnapshotEnabled = false
}
