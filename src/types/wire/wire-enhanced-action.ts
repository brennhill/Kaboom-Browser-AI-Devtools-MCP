// THIS FILE IS GENERATED — do not edit by hand.
// Source: internal/types/wire_enhanced_action.go
// Generator: scripts/build/generate-wire-types.js

/**
 * @fileoverview Wire type for enhanced actions — matches internal/types/wire_enhanced_action.go
 *
 * This is the canonical TypeScript definition for the EnhancedAction HTTP payload.
 * Changes here MUST be mirrored in the Go counterpart. Run `make check-wire-drift`.
 */

/**
 * WireAXLocator re-finds a target by accessibility semantics when its selector stops matching.
 */
export interface WireAXLocator {
  readonly ref?: string
  readonly role?: string
  readonly name?: string
}

/**
 * WireViewportLocator re-finds a target by the point it occupied, in the frame that point belongs to.
 */
export interface WireViewportLocator {
  readonly x: number
  readonly y: number
  readonly width?: number
  readonly height?: number
  readonly frame_url?: string
  readonly viewport_width?: number
  readonly viewport_height?: number
  readonly device_pixel_ratio?: number
}

/**
 * WireClockPin records the clock and timezone a session held still.
 */
export interface WireClockPin {
  readonly epoch_ms?: number
  readonly timezone_id?: string
  readonly virtual_time_policy?: string
}

/**
 * WireGeoPin records the geolocation a session held still.
 */
export interface WireGeoPin {
  readonly latitude: number
  readonly longitude: number
  readonly accuracy_m?: number
}

/**
 * WireViewportPin records the device metrics a session held still.
 */
export interface WireViewportPin {
  readonly width: number
  readonly height: number
  readonly device_scale_factor?: number
  readonly mobile?: boolean
}

/**
 * WireEnvironmentPin reports what a session pinned, so the emitted test states its dependencies.
 */
export interface WireEnvironmentPin {
  readonly clock?: WireClockPin
  readonly geolocation?: WireGeoPin
  readonly viewport?: WireViewportPin
  readonly random_seed?: string
  readonly unpinned?: readonly string[]
}

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
  readonly ax?: WireAXLocator
  readonly viewport?: WireViewportLocator
  readonly environment?: WireEnvironmentPin
  // server-only: test_ids — added by Go daemon for test boundary correlation
  // server-only: source — added by Go daemon ("human" or "ai")
}
