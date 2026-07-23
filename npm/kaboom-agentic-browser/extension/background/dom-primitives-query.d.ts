/**
 * Purpose: Self-contained DOM query primitive for interact(what='query').
 * Why: Enables non-destructive element queries (exists, count, text_all, attributes)
 *      without erroring on missing elements. Complements get_text/get_attribute.
 * Docs: docs/features/feature/interact-explore/index.md
 */
/**
 * Self-contained function that queries the DOM for element existence, count, text, or attributes.
 * Unlike get_text/get_attribute which error on missing elements, this returns structured results
 * with exists=false or count=0 when no elements match.
 *
 * Passed to chrome.scripting.executeScript({ func: domPrimitiveQuery }).
 * MUST NOT reference any module-level variables.
 */
export declare function domPrimitiveQuery(selector: string, options?: {
    query_type?: string;
    attribute_names?: string[];
    scope_selector?: string;
}): {
    success: boolean;
    query_type: string;
    selector: string;
    exists?: boolean;
    count?: number;
    text?: string | null;
    texts?: string[];
    attributes?: Record<string, string | null>;
    error?: string;
    message?: string;
};
//# sourceMappingURL=dom-primitives-query.d.ts.map