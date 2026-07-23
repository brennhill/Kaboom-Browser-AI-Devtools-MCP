/**
 * Purpose: Page-injected element resolution for CDP escalation — finds elements by selector, gets bounding rects.
 * Why: Self-contained function injected via chrome.scripting.executeScript; must have no outer-scope closures.
 * Docs: docs/features/feature/interact-explore/index.md
 */
import type { DOMActionParams, DOMResult } from './dom-types.js';
/**
 * Injected into the page via chrome.scripting.executeScript to resolve an
 * element by selector, get its bounding rect, and optionally focus it.
 * Must be fully self-contained — no closures over outer scope.
 */
declare function cdpResolveAndPrepare(selectorStr: string, actionType: string, scopeSelectorStr: string | null, elementIdStr: string | null): {
    x: number;
    y: number;
    tag: string;
    text_preview: string;
    selector: string;
    element_id?: string;
    aria_label?: string;
    role?: string;
    bbox: {
        x: number;
        y: number;
        width: number;
        height: number;
    };
} | null;
export type ResolvedElement = NonNullable<ReturnType<typeof cdpResolveAndPrepare>>;
export declare function resolveElement(tabId: number, params: DOMActionParams): Promise<ResolvedElement | null>;
export declare function buildCDPResult(action: string, selector: string, resolved: ResolvedElement, elapsedMs: number, extra?: Record<string, unknown>): DOMResult;
export {};
//# sourceMappingURL=cdp-element-resolve.d.ts.map