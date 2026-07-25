/**
 * Purpose: Core DOM primitives for selector-based actions (click, type, wait_for, etc.).
 * #502: Intent/overlay/stability actions extracted to separate self-contained modules:
 *   - dom-primitives-intent.ts (open_composer, submit_active_composer, confirm_top_dialog)
 *   - dom-primitives-overlay.ts (dismiss_top_overlay, auto_dismiss_overlays)
 *   - dom-primitives-stability.ts (wait_for_stable, action_diff)
 * Docs: docs/features/feature/interact-explore/index.md
 */
import type { DOMPrimitiveOptions, DOMResult } from './dom-types.js';
export { domPrimitiveListInteractive } from './dom-primitives-list-interactive.js';
/**
 * Single self-contained function for all DOM primitives.
 * Passed to chrome.scripting.executeScript({ func: domPrimitive, args: [...] }).
 * MUST NOT reference any module-level variables — Chrome serializes the function source only.
 */
export declare function domPrimitive(action: string, selector: string, options: DOMPrimitiveOptions): DOMResult | Promise<DOMResult> | {
    success: boolean;
    elements: unknown[];
    candidate_count?: number;
    scope_rect_used?: {
        x: number;
        y: number;
        width: number;
        height: number;
    };
    error?: string;
    message?: string;
};
/**
 * Backward-compatible wait helper used by unit tests and legacy call sites.
 * Polls wait_for and listens for DOM mutations for fast resolution.
 */
export declare function domWaitFor(selector: string, timeoutMs?: number): Promise<DOMResult>;
//# sourceMappingURL=dom-primitives.d.ts.map