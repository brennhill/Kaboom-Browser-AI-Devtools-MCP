/**
 * Purpose: Self-contained DOM primitives for overlay dismiss actions (dismiss_top_overlay, auto_dismiss_overlays).
 * Why: Extracted from dom-primitives.ts to keep file sizes under the 800 LOC limit (#502).
 *      These actions find and dismiss overlays/modals/consent banners using multi-strategy resolution.
 * Docs: docs/features/feature/interact-explore/index.md
 */
/**
 * Self-contained function that finds and dismisses overlays, modals, and consent banners.
 * Handles both dismiss_top_overlay and auto_dismiss_overlays actions.
 * Passed to chrome.scripting.executeScript({ func: domPrimitiveOverlay }).
 * MUST NOT reference any module-level variables.
 */
export declare function domPrimitiveOverlay(action: 'dismiss_top_overlay' | 'auto_dismiss_overlays', options?: {
    scope_selector?: string;
    timeout_ms?: number;
}): {
    success: boolean;
    action: string;
    selector: string;
    error?: string;
    message?: string;
    strategy?: string;
    selector_used?: string;
    overlay_type?: string;
    overlay_selector?: string;
    overlay_text_preview?: string;
    overlay_source?: 'extension' | 'page';
    dismissed_count?: number;
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
    };
    match_count?: number;
    match_strategy?: string;
    viewport?: {
        scroll_x: number;
        scroll_y: number;
        viewport_width: number;
        viewport_height: number;
        page_height: number;
    };
};
//# sourceMappingURL=dom-primitives-overlay.d.ts.map