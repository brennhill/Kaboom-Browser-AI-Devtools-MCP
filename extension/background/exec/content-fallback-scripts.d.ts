/**
 * Purpose: Self-contained extraction fallbacks used when content scripts are unavailable.
 * Why: Keep fallback script implementations centralized and reusable across command handlers.
 * Docs: docs/features/feature/interact-explore/index.md
 *
 * Each exported script function is injected via chrome.scripting.executeScript({func}):
 * Chrome serializes the function alone, so every helper and selector list MUST be
 * nested inside the function body — module-scope references are lost on injection.
 */
export declare function readableFallbackScript(): Record<string, unknown>;
export type FallbackScript = () => Record<string, unknown>;
export declare const FALLBACK_SCRIPTS: Record<string, FallbackScript>;
//# sourceMappingURL=content-fallback-scripts.d.ts.map