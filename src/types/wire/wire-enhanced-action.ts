// THIS FILE IS GENERATED — do not edit by hand.
// Source: internal/types/wire_enhanced_action.go
// Generator: scripts/generate-wire-types.js

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
  readonly type: string
  readonly timestamp: number
  readonly url?: string
  readonly selectors?: Readonly<Record<string, unknown>>
  readonly value?: string
  readonly input_type?: string
  readonly key?: string
  readonly from_url?: string
  readonly to_url?: string
  readonly selected_value?: string
  readonly selected_text?: string
  readonly scroll_y?: number
  readonly tab_id?: number
  readonly classification?: string
  readonly duration_ms?: number
  readonly role?: string
  // server-only: test_ids — added by Go daemon for test boundary correlation
  // server-only: source — added by Go daemon ("human" or "ai")
}
