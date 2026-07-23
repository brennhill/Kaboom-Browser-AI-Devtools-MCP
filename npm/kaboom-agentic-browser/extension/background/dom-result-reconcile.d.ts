/**
 * Purpose: DOM action result validation, lifecycle reconciliation, and frame result picking.
 * Why: Separates result shape validation and status derivation from chrome.scripting execution.
 * Docs: docs/features/feature/interact-explore/index.md
 */
import type { DOMResult } from './dom-types.js';
import type { ActionToastFn } from './commands/helpers.js';
export declare function toDOMResult(value: unknown): DOMResult | null;
/** Pick the best result from multi-frame executeScript. Prefers main frame, falls back to first success. */
export declare function pickFrameResult(results: chrome.scripting.InjectionResult[]): {
    result: unknown;
    frameId: number;
} | null;
/** Merge list_interactive results from all frames (up to 100 elements). */
export declare function mergeListInteractive(results: chrome.scripting.InjectionResult[]): {
    success: boolean;
    elements: unknown[];
    candidate_count?: number;
    scope_rect_used?: unknown;
    error?: string;
    message?: string;
};
export declare function deriveAsyncStatusFromDOMResult(action: string, selector: string, result: unknown): {
    result: unknown;
    status: 'complete' | 'error';
    error?: string;
};
export declare function enrichWithEffectiveContext(tabId: number, result: unknown): Promise<unknown>;
export declare function sendToastForResult(tabId: number, readOnly: boolean, result: {
    success?: boolean;
    error?: string;
}, actionToast: ActionToastFn, toastLabel: string, toastDetail: string | undefined): void;
//# sourceMappingURL=dom-result-reconcile.d.ts.map