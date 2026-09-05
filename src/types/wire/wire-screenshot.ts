// THIS FILE IS GENERATED — do not edit by hand.
// Source: internal/screenshotframe/wire_screenshot.go
// Generator: scripts/build/generate-wire-types.js

/**
 * @fileoverview Wire types for the screenshot coordinate frame — matches internal/screenshotframe/wire_screenshot.go
 *
 * Canonical TypeScript definitions for the frame that maps a screenshot pixel to a clickable coordinate.
 * Changes here MUST be mirrored in the Go counterpart. Run `make check-wire-drift`.
 */

/**
 * WireImageToViewport maps an image pixel to the CSS-pixel coordinate an action accepts:
 * viewport_x = image_x*scale_x + offset_x, viewport_y = image_y*scale_y + offset_y.
 * Both axes are reported because a clamped capture is squashed on one axis only.
 */
export interface WireImageToViewport {
  readonly scale_x: number
  readonly scale_y: number
  readonly offset_x: number
  readonly offset_y: number
}

/**
 * WireImageRect is a rectangle in image pixels.
 */
export interface WireImageRect {
  readonly x: number
  readonly y: number
  readonly width: number
  readonly height: number
}

/**
 * WireCoordinateFrame is everything needed to read a coordinate off a screenshot and act on it.
 * `note` is filled in by the daemon, not by the page.
 */
export interface WireCoordinateFrame {
  readonly capture: string
  readonly image_width: number
  readonly image_height: number
  readonly viewport_width: number
  readonly viewport_height: number
  readonly scroll_x: number
  readonly scroll_y: number
  readonly document_width: number
  readonly document_height: number
  readonly device_pixel_ratio: number
  readonly clipped: boolean
  readonly image_to_viewport: WireImageToViewport
  readonly viewport_bounds_in_image: WireImageRect
  readonly note?: string
}
