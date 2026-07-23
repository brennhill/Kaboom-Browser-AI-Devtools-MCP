/**
 * Purpose: Detects UI frameworks (React/Vue/Svelte), captures state snapshots, and generates AI-friendly error summaries with timeout guards.
 * Docs: docs/features/feature/error-bundling/index.md
 */
/**
 * @fileoverview Runtime introspection for AI error context.
 * Detects UI frameworks (React/Vue/Svelte), walks React fiber trees,
 * captures Redux state snapshots, generates AI summaries, and orchestrates
 * the full error enrichment pipeline with timeout guards.
 */
import type { LogEntry, AiContextData } from '../types/index.js';
/**
 * Framework detection result
 */
export interface FrameworkDetection {
    framework: 'react' | 'vue' | 'svelte';
    key?: string;
}
/**
 * React component ancestry entry
 */
export interface ReactComponentEntry {
    name: string;
    propKeys?: string[];
    hasState?: boolean;
    stateKeys?: string[];
}
/**
 * React fiber node (partial typing for what we access)
 */
interface ReactFiber {
    type?: {
        displayName?: string;
        name?: string;
    } | string;
    memoizedProps?: Record<string, unknown>;
    memoizedState?: Record<string, unknown> | unknown[] | null;
    return?: ReactFiber | null;
}
/**
 * Component ancestry result
 */
export interface ComponentAncestryResult {
    framework: 'react';
    components: ReactComponentEntry[];
}
/**
 * Redux store interface
 */
interface ReduxStore {
    getState: () => Record<string, unknown>;
}
/**
 * State snapshot result
 */
export interface StateSnapshotResult {
    source: 'redux';
    keys: Record<string, {
        type: string;
    }>;
    relevantSlice: Record<string, unknown>;
}
/**
 * AI summary generation data
 */
export interface AiSummaryData {
    errorType: string;
    message: string;
    file: string | null;
    line: number | null;
    componentAncestry: ComponentAncestryResult | null;
    stateSnapshot: StateSnapshotResult | null;
}
/**
 * Enriched error entry with AI context
 */
type EnrichedErrorEntry = LogEntry & {
    _aiContext?: AiContextData;
    _enrichments?: string[];
};
/**
 * Element with framework markers
 */
interface FrameworkElement {
    __vueParentComponent?: unknown;
    __vue_app__?: unknown;
    __svelte_meta?: unknown;
    [key: string]: unknown;
}
/**
 * Error entry for enrichment (partial typing for what we access)
 */
interface ErrorEntryForEnrichment {
    stack?: string;
    message?: string;
}
declare global {
    interface Window {
        __REDUX_STORE__?: ReduxStore;
    }
}
/**
 * Detect which UI framework an element belongs to
 * @param element - The DOM element (or element-like object)
 * @returns { framework, key? } or null
 */
export declare function detectFramework(element: FrameworkElement | null | undefined): FrameworkDetection | null;
/**
 * Walk a React fiber tree to extract component ancestry
 * @param fiber - The React fiber node
 * @returns Array of { name, propKeys?, hasState?, stateKeys? } in root-first order
 */
export declare function getReactComponentAncestry(fiber: ReactFiber | null | undefined): ReactComponentEntry[] | null;
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
export declare function captureStateSnapshot(errorMessage: string): StateSnapshotResult | null;
/**
 * Generate a template-based AI summary from enrichment data
 * @param data - { errorType, message, file, line, componentAncestry, stateSnapshot }
 * @returns Summary string
 */
export declare function generateAiSummary(data: AiSummaryData): string;
export declare function enrichErrorWithAiContext(error: ErrorEntryForEnrichment): Promise<EnrichedErrorEntry>;
/**
 * Enable or disable AI context enrichment
 * @param enabled
 */
export declare function setAiContextEnabled(enabled: boolean): void;
/**
 * Enable or disable state snapshot in AI context
 * @param enabled
 */
export declare function setAiContextStateSnapshot(enabled: boolean): void;
/**
 * Reset enrichment module state for testing purposes.
 * Call this in beforeEach/afterEach test hooks to prevent test pollution.
 */
export declare function resetEnrichmentForTesting(): void;
export {};
//# sourceMappingURL=ai-context-enrichment.d.ts.map