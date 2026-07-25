/**
 * Purpose: Self-contained DOM primitives for wait_for_stable and action_diff actions.
 * Why: Extracted from dom-primitives.ts to keep file sizes under the 800 LOC limit (#502).
 *      These actions need no selector infrastructure — they observe DOM mutations and classify changes.
 * Docs: docs/features/feature/interact-explore/index.md
 */
/**
 * Self-contained function that waits for the DOM to become stable (no mutations for a configurable period).
 * Passed to chrome.scripting.executeScript({ func: domPrimitiveWaitForStable }).
 * MUST NOT reference any module-level variables.
 */
export declare function domPrimitiveWaitForStable(options?: {
    stability_ms?: number;
    timeout_ms?: number;
}): Promise<{
    success: boolean;
    action: string;
    selector: string;
    stable?: boolean;
    timed_out?: boolean;
    waited_ms?: number;
    mutations_observed?: number;
    stability_ms?: number;
}>;
/**
 * Self-contained function that instruments a MutationObserver, waits for DOM to settle,
 * then classifies mutations into categories (overlays, toasts, form errors, etc.).
 * Passed to chrome.scripting.executeScript({ func: domPrimitiveActionDiff }).
 * MUST NOT reference any module-level variables.
 */
export declare function domPrimitiveActionDiff(options?: {
    timeout_ms?: number;
}): Promise<{
    success: boolean;
    action: string;
    selector: string;
    action_diff?: {
        url_changed: boolean;
        title_changed: boolean;
        overlays_opened: Array<{
            selector: string;
            text: string;
        }>;
        overlays_closed: Array<{
            selector: string;
            text: string;
        }>;
        toasts: Array<{
            text: string;
            type: string;
        }>;
        form_errors: string[];
        loading_indicators: string[];
        elements_added: number;
        elements_removed: number;
        text_changes: Array<{
            selector: string;
            from: string;
            to: string;
        }>;
        network_requests: number;
    };
}>;
//# sourceMappingURL=dom-primitives-stability.d.ts.map