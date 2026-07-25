/**
 * Purpose: Enumerates interactive elements on a page for AI-driven automation.
 * Why: Self-contained for chrome.scripting.executeScript (no closures allowed).
 * Docs: docs/features/feature/interact-explore/index.md
 */
/**
 * Self-contained function that scans a page for interactive elements.
 * Passed to chrome.scripting.executeScript({ func: domPrimitiveListInteractive }).
 * MUST NOT reference any module-level variables.
 */
export declare function domPrimitiveListInteractive(scopeSelector?: string, options?: {
    scope_rect?: {
        x?: unknown;
        y?: unknown;
        width?: unknown;
        height?: unknown;
    };
    text_contains?: string;
    role?: string;
    visible_only?: boolean;
    exclude_nav?: boolean;
}): {
    success: boolean;
    elements: unknown[];
    candidate_count?: number;
    scope_rect_used?: {
        x: number;
        y: number;
        width: number;
        height: number;
    };
    filters_applied?: Record<string, unknown>;
    error?: string;
    message?: string;
};
//# sourceMappingURL=dom-primitives-list-interactive.d.ts.map