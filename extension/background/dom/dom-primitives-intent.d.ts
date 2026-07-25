/**
 * Purpose: Self-contained DOM primitives for intent-based actions (open_composer, submit_active_composer, confirm_top_dialog).
 * Why: Extracted from dom-primitives.ts to keep file sizes under the 800 LOC limit (#502).
 *      These actions use heuristic scoring to find the best target element for high-level intent.
 * Docs: docs/features/feature/interact-explore/index.md
 */
/**
 * Self-contained function that resolves intent-based targets (composer triggers, submit buttons, dialog confirms).
 * Passed to chrome.scripting.executeScript({ func: domPrimitiveIntent }).
 * MUST NOT reference any module-level variables.
 */
export declare function domPrimitiveIntent(action: 'open_composer' | 'submit_active_composer' | 'confirm_top_dialog', options?: {
    scope_selector?: string;
}): {
    success: boolean;
    action: string;
    selector: string;
    error?: string;
    message?: string;
    matched?: {
        tag?: string;
        role?: string;
        aria_label?: string;
        text_preview?: string;
        selector?: string;
        element_id?: string;
        bbox?: {
            x: number;
            y: number;
            width: number;
            height: number;
        };
        scope_selector_used?: string;
    };
    match_count?: number;
    match_strategy?: string;
    reason?: string;
    candidates?: Array<{
        tag?: string;
        role?: string;
        aria_label?: string;
        text_preview?: string;
        selector?: string;
        element_id?: string;
        bbox?: {
            x: number;
            y: number;
            width: number;
            height: number;
        };
        visible?: boolean;
    }>;
    viewport?: {
        scroll_x: number;
        scroll_y: number;
        viewport_width: number;
        viewport_height: number;
        page_height: number;
    };
};
//# sourceMappingURL=dom-primitives-intent.d.ts.map