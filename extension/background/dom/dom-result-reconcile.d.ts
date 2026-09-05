/**
 * Purpose: DOM action result validation, lifecycle reconciliation, and frame result picking.
 * Why: Separates result shape validation and status derivation from chrome.scripting execution.
 * Docs: docs/features/feature/interact-explore/index.md
 */
import type { DOMResult } from './dom-types.js';
import type { ActionToastFn } from '../commands/helpers.js';
import type { ContentProvenance } from '../../lib/provenance/provenance-types.js';
/** What a frame reported about itself, from the self-contained origin probe. */
export interface FrameOriginInfo {
    origin: string;
    is_top_level_document: boolean;
}
export type FrameOriginMap = ReadonlyMap<number, FrameOriginInfo>;
export declare function toDOMResult(value: unknown): DOMResult | null;
/** Pick the best result from multi-frame executeScript. Prefers main frame, falls back to first success. */
export declare function pickFrameResult(results: chrome.scripting.InjectionResult[]): {
    result: unknown;
    frameId: number;
} | null;
/**
 * Merge list_interactive results from all frames (up to 100 elements).
 *
 * Every element carries the frame it came from and, when the origin probe succeeded, that frame's
 * origin: a merged list that hides which frame drew a control makes an ad iframe's button
 * indistinguishable from the site's own.
 */
export declare function mergeListInteractive(results: chrome.scripting.InjectionResult[], frames?: FrameOriginMap): {
    success: boolean;
    elements: unknown[];
    candidate_count?: number;
    scope_rect_used?: unknown;
    provenance: ContentProvenance;
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