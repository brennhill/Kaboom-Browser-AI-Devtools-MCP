/**
 * Purpose: Records user interactions with multi-strategy selectors (testId, role, aria, text, CSS path) and generates Playwright reproduction scripts.
 * Docs: docs/features/feature/reproduction-scripts/index.md
 */
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