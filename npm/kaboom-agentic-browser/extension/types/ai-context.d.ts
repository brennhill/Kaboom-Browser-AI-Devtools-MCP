/**
 * Purpose: Defines typed AI context payloads for stack parsing, snippets, and component ancestry enrichment.
 * Why: Preserves consistent error-enrichment structure across capture, processing, and downstream analysis.
 * Docs: docs/features/feature/ai-capture-control/index.md
 */
/**
 * @fileoverview AI Context Types
 * Stack frames, source snippets, and React component info for error analysis
 */
/**
 * Parsed stack frame
 */
export interface StackFrame {
    readonly functionName: string;
    readonly fileName: string;
    readonly lineNumber: number;
    readonly columnNumber: number;
    readonly raw: string;
    readonly originalFileName?: string;
    readonly originalLineNumber?: number;
    readonly originalColumnNumber?: number;
    readonly originalFunctionName?: string;
    readonly resolved?: boolean;
}
/**
 * Source code snippet
 */
export interface SourceSnippet {
    readonly file: string;
    readonly line: number;
    readonly lines: readonly string[];
    readonly highlightLine: number;
}
/**
 * React component ancestry info
 */
export interface ReactComponentAncestry {
    readonly component: string;
    readonly props?: Readonly<Record<string, unknown>>;
    readonly ancestors: readonly string[];
}
/**
 * AI context data attached to errors
 */
export interface AiContextData {
    readonly framework?: string;
    readonly snippets?: readonly SourceSnippet[];
    readonly componentAncestry?: ReactComponentAncestry;
    readonly stateSnapshot?: Readonly<Record<string, unknown>>;
    readonly summary?: string;
}
//# sourceMappingURL=ai-context.d.ts.map