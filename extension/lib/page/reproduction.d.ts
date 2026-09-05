/**
 * Purpose: Records user interactions with multi-strategy selectors (testId, role, aria, text, CSS path) and generates Playwright reproduction scripts.
 * Docs: docs/features/feature/reproduction-scripts/index.md
 */
import type { WireAXLocator, WireViewportLocator } from '../../types/wire/wire-enhanced-action.js';
type EnhancedActionType = 'click' | 'input' | 'keypress' | 'navigate' | 'select' | 'scroll' | 'transient';
interface RoleSelector {
    role: string;
    name?: string;
}
interface SelectorStrategies {
    testId?: string;
    ariaLabel?: string;
    role?: RoleSelector;
    id?: string;
    text?: string;
    cssPath: string;
}
interface EnhancedActionRecord {
    type: EnhancedActionType;
    timestamp: number;
    url: string;
    selectors?: SelectorStrategies;
    /**
     * Locators two and three, recorded alongside the selector rather than instead of it.
     *
     * A selector describes where an element sits in the markup, so any re-render can break
     * it. `ax` describes what the control MEANS, and `viewport` where it was on screen.
     * Emitting all three is what lets a generated test survive a change that breaks one.
     */
    ax?: WireAXLocator;
    viewport?: WireViewportLocator;
    input_type?: string;
    value?: string;
    key?: string;
    from_url?: string;
    to_url?: string;
    selected_value?: string;
    selected_text?: string;
    scroll_y?: number;
    classification?: string;
    duration_ms?: number;
    role?: string;
}
interface ScriptOptions {
    errorMessage?: string;
    baseUrl?: string;
    lastNActions?: number;
}
export declare function getImplicitRole(element: Element | null): string | null;
/**
 * Detect if a CSS class name is dynamically generated (CSS-in-JS)
 */
export declare function isDynamicClass(className: string | null): boolean;
/**
 * Compute a CSS path for an element
 */
export declare function computeCssPath(element: Element | null): string;
/**
 * Resolve the accessibility semantics of an element: its role and its accessible name.
 *
 * One resolver, two consumers — the `role` selector strategy and the `ax` locator — so the
 * two can never disagree about what the same element is called.
 */
export declare function resolveAccessibleRoleAndName(element: Element | null): {
    role: string;
    name: string;
};
/**
 * The second locator: what the control means, not where it sits.
 *
 * `ref` is deliberately absent. A CDP AX ref is a backend node id valid only inside the
 * snapshot that produced it, so recording one here would be stale by replay time. Role plus
 * accessible name is what `interact(what:'find')` resolves against, and it survives the DOM
 * restructuring that breaks a selector.
 */
export declare function computeAXLocator(element: Element | null): WireAXLocator | undefined;
/**
 * The third locator: the point the target occupied, with the frame and viewport it was
 * measured in.
 *
 * A bare x/y is unreplayable — at a different window size or device scale it lands
 * somewhere else, and in the wrong frame it lands on the wrong document — so the
 * measurement context travels with the point. A zero-area or non-finite box is dropped
 * rather than recorded: a point nothing occupies would send a replayed click into empty
 * space and report success.
 */
export declare function computeViewportLocator(element: Element | null): WireViewportLocator | undefined;
/**
 * Compute multi-strategy selectors for an element
 */
export declare function computeSelectors(element: Element | null): SelectorStrategies;
interface RecordActionOptions {
    value?: string;
    key?: string;
    from_url?: string;
    to_url?: string;
    selected_value?: string;
    selected_text?: string;
    scroll_y?: number;
    classification?: string;
    duration_ms?: number;
    role?: string;
}
/**
 * Record an enhanced action with multi-strategy selectors
 */
export declare function recordEnhancedAction(type: EnhancedActionType, element: Element | null, opts?: RecordActionOptions): EnhancedActionRecord;
/**
 * Get the enhanced action buffer
 */
export declare function getEnhancedActionBuffer(): EnhancedActionRecord[];
/**
 * Clear the enhanced action buffer
 */
export declare function clearEnhancedActionBuffer(): void;
/**
 * Generate a Playwright test script from captured actions
 */
export declare function generatePlaywrightScript(actions: EnhancedActionRecord[], opts?: ScriptOptions): string;
export {};
//# sourceMappingURL=reproduction.d.ts.map