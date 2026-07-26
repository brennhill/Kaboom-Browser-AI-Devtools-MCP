/**
 * Purpose: Barrel re-export for the AI error context enrichment pipeline (parsing and enrichment sub-modules).
 * Docs: docs/features/feature/error-bundling/index.md
 */
/**
 * @fileoverview AI-preprocessed error enrichment pipeline.
 * Parses stack traces, resolves source maps, extracts code snippets,
 * detects UI frameworks (React/Vue/Svelte), captures state snapshots,
 * and generates AI-friendly error summaries. All within a timeout guard.
 *
 * This file re-exports from ai-context-parsing.ts and ai-context-enrichment.ts
 * so existing importers are unaffected.
 */
export { parseStackFrames, parseSourceMap, extractSnippet, extractSourceSnippets, setSourceMapCache, getSourceMapCache, getSourceMapCacheSize, } from './ai-context-parsing.js';
export { detectFramework, getReactComponentAncestry, captureStateSnapshot, generateAiSummary, enrichErrorWithAiContext, setAiContextEnabled, setAiContextStateSnapshot, } from './ai-context-enrichment.js';
//# sourceMappingURL=ai-context.d.ts.map