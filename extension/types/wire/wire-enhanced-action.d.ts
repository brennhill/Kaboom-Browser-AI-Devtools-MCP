/**
 * @fileoverview Wire type for enhanced actions — matches internal/types/wire_enhanced_action.go
 *
 * This is the canonical TypeScript definition for the EnhancedAction HTTP payload.
 * Changes here MUST be mirrored in the Go counterpart. Run `make check-wire-drift`.
 */
/**
 * WireEnhancedAction is the JSON shape sent over HTTP between extension and Go daemon.
 * All fields use snake_case to match the Go json tags.
 */
export interface WireEnhancedAction {
    readonly type: string;
    readonly timestamp: number;
    readonly url?: string;
    readonly selectors?: Readonly<Record<string, unknown>>;
    readonly value?: string;
    readonly input_type?: string;
    readonly key?: string;
    readonly from_url?: string;
    readonly to_url?: string;
    readonly selected_value?: string;
    readonly selected_text?: string;
    readonly scroll_y?: number;
    readonly tab_id?: number;
    readonly classification?: string;
    readonly duration_ms?: number;
    readonly role?: string;
}
//# sourceMappingURL=wire-enhanced-action.d.ts.map