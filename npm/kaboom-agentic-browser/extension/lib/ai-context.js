/**
 * Purpose: Barrel re-export for the AI error context enrichment pipeline (parsing and enrichment sub-modules).
 * Docs: docs/features/feature/error-bundling/index.md
 */
// ai-context.ts — AI error context barrel. Re-exports parsing and enrichment sub-modules.
/**
 * @fileoverview AI-preprocessed error enrichment pipeline.
 * Parses stack traces, resolves source maps, extracts code snippets,
 * detects UI frameworks (React/Vue/Svelte), captures state snapshots,
 * and generates AI-friendly error summaries. All within a timeout guard.
 *
 * This file re-exports from ai-context-parsing.ts and ai-context-enrichment.ts
 * so existing importers are unaffected.
 */
// Re-export parsing layer
export { parseStackFrames, parseSourceMap, extractSnippet, extractSourceSnippets, setSourceMapCache, getSourceMapCache, getSourceMapCacheSize, } from './ai-context-parsing.js';
// Re-export enrichment layer
export { detectFramework, getReactComponentAncestry, captureStateSnapshot, generateAiSummary, enrichErrorWithAiContext, setAiContextEnabled, setAiContextStateSnapshot, } from './ai-context-enrichment.js';
import { resetParsingForTesting } from './ai-context-parsing.js';
import { resetEnrichmentForTesting } from './ai-context-enrichment.js';
/**
 * Reset all module state for testing purposes
 * Clears source map cache and restores default settings.
 * Call this in beforeEach/afterEach test hooks to prevent test pollution.
 */
function resetForTesting() {
    resetParsingForTesting();
    resetEnrichmentForTesting();
}
//# sourceMappingURL=ai-context.js.map