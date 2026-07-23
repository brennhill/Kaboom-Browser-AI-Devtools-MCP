/**
 * Purpose: Shared type definitions for DOM action parameters and results used by dispatch and injected primitives.
 * Docs: docs/features/feature/interact-explore/index.md
 */
export interface DOMMutationEntry {
    type: 'added' | 'removed' | 'attribute';
    tag?: string;
    id?: string;
    class?: string;
    text_preview?: string;
    attribute?: string;
    old_value?: string;
    new_value?: string;
}
export interface ScopeRect {
    x: number;
    y: number;
    width: number;
    height: number;
}
export interface BoundingBox {
    x: number;
    y: number;
    width: number;
    height: number;
}
export interface DOMResult {
    success: boolean;
    action: string;
    selector: string;
    value?: unknown;
    candidate_count?: number;
    scope_rect_used?: ScopeRect;
    match_count?: number;
    match_strategy?: string;
    matched?: {
        tag?: string;
        role?: string;
        aria_label?: string;
        text_preview?: string;
        classes?: string[];
        selector?: string;
        element_id?: string;
        bbox?: BoundingBox;
        scope_selector_used?: string;
        scope_rect_used?: ScopeRect;
        frame_id?: number;
    };
    candidates?: Array<{
        tag?: string;
        role?: string;
        aria_label?: string;
        text_preview?: string;
        selector?: string;
        element_id?: string;
        bbox?: BoundingBox;
        visible?: boolean;
    }>;
    auto_scrolled?: boolean;
    ambiguous_matches?: {
        total_count: number;
        warning: string;
        candidates: Array<{
            tag: string;
            element_id: string;
            text_preview?: string;
        }>;
    };
    reason?: string;
    error?: string;
    message?: string;
    dom_summary?: string;
    timing?: {
        total_ms: number;
    };
    dom_changes?: {
        added: number;
        removed: number;
        modified: number;
        summary: string;
    };
    dom_mutations?: DOMMutationEntry[];
    viewport?: {
        scroll_x: number;
        scroll_y: number;
        viewport_width: number;
        viewport_height: number;
        page_height: number;
    };
    analysis?: string;
    insertion_strategy?: string;
    ranked_candidates?: Array<{
        element_id: string;
        tag: string;
        text_preview?: string;
        score: number;
    }>;
    suggested_element_id?: string;
    strategy?: string;
    selector_used?: string;
    overlay_type?: string;
    overlay_selector?: string;
    overlay_text_preview?: string;
    overlay_warning?: string;
    overlay_source?: 'extension' | 'page';
    stable?: boolean;
    timed_out?: boolean;
    waited_ms?: number;
    mutations_observed?: number;
    stability_ms?: number;
    matched_text?: string;
    absent?: boolean;
    dismissed_count?: number;
    sections?: Array<{
        header?: string;
        content: string;
        expanded?: boolean;
        tag: string;
    }>;
    section_count?: number;
}
export interface DOMPrimitiveOptions {
    text?: string;
    key?: string;
    value?: string;
    direction?: string;
    clear?: boolean;
    checked?: boolean;
    name?: string;
    timeout_ms?: number;
    stability_ms?: number;
    analyze?: boolean;
    observe_mutations?: boolean;
    element_id?: string;
    scope_selector?: string;
    scope_rect?: ScopeRect;
    nth?: number;
    new_tab?: boolean;
    url_contains?: string;
    absent?: boolean;
    structured?: boolean;
}
export interface DOMActionParams extends DOMPrimitiveOptions {
    action?: string;
    selector?: string;
    reason?: string;
    frame?: string | number;
    text_contains?: string;
    role?: string;
    exclude_nav?: boolean;
    visible_only?: boolean;
    query_type?: string;
    attribute_names?: string[];
}
//# sourceMappingURL=dom-types.d.ts.map