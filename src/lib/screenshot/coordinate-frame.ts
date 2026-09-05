/**
 * Purpose: Build the coordinate frame that ties a captured image's pixels to the CSS coordinates an action accepts.
 * Docs: docs/features/feature/observe/index.md
 */

import type { WireCoordinateFrame, WireImageRect } from '../../types/wire/wire-screenshot.js'

/**
 * What the page reports about itself at capture time. Every field is a CSS-pixel
 * measurement read from the page, never inferred.
 */
export interface PageViewportMetrics {
  readonly viewport_width: number
  readonly viewport_height: number
  readonly scroll_x: number
  readonly scroll_y: number
  readonly document_width: number
  readonly document_height: number
  readonly device_pixel_ratio: number
}

/** The image the caller is actually holding, in real pixels. */
export interface CapturedImageSize {
  readonly width: number
  readonly height: number
}

/**
 * The CSS-pixel rectangle the image covers, in VIEWPORT coordinates.
 *
 * This one input is what makes every capture kind the same arithmetic. A viewport
 * capture covers {0, 0, innerWidth, innerHeight}. An element crop covers the
 * element's bounding rect. A zoom region covers the requested rectangle. A
 * full-page image covers the document, whose origin sits `scrollY` above the top
 * of the screen, so its y is negative whenever the page is scrolled.
 */
export interface CoveredCssRegion {
  readonly x: number
  readonly y: number
  readonly width: number
  readonly height: number
}

/**
 * Runs in the page's MAIN world via chrome.scripting.executeScript.
 *
 * Self-contained by necessity — an injected function cannot close over anything in
 * this module.
 *
 * `innerWidth`/`innerHeight` rather than `visualViewport`: these are the CSS
 * coordinate space that CDP input events are expressed in, and they are the extent
 * `chrome.tabs.captureVisibleTab` photographs (scrollbars included). visualViewport
 * shrinks under pinch-zoom and would make the frame describe a region the image
 * does not cover, which moves every click by the zoom offset.
 */
export function readPageViewportMetrics(): PageViewportMetrics {
  const doc = document.documentElement
  return {
    viewport_width: window.innerWidth || doc?.clientWidth || 0,
    viewport_height: window.innerHeight || doc?.clientHeight || 0,
    scroll_x: window.scrollX || window.pageXOffset || 0,
    scroll_y: window.scrollY || window.pageYOffset || 0,
    document_width: Math.max(doc?.scrollWidth || 0, document.body?.scrollWidth || 0),
    document_height: Math.max(doc?.scrollHeight || 0, document.body?.scrollHeight || 0),
    device_pixel_ratio: window.devicePixelRatio || 1
  }
}

/** Capture kinds, mirroring internal/screenshotframe.CaptureKinds(). */
export type CaptureKind = 'viewport' | 'full_page' | 'element' | 'region'

/**
 * The CSS region a capture covers, in viewport coordinates.
 *
 * `covered` is what the capture path itself reports it photographed, and it always
 * wins: only that path knows whether it clipped to CDP's cssVisualViewport, to a
 * clamped full-page box, or to an element's rect, and the difference between those
 * and the page's own idea of its viewport is tens of pixels at the far edge of the
 * image. The kind-derived fallbacks below are for the paths that cannot report one
 * — `captureVisibleTab` photographs the visible viewport and nothing else.
 *
 * Kept in one place rather than at the four call sites so a new capture path cannot
 * quietly ship a frame with the wrong origin: the difference between a viewport
 * origin and a document origin is the scroll offset, and getting it wrong misplaces
 * every click on a scrolled page by exactly the amount the user scrolled.
 */
export function coveredRegionFor(
  kind: CaptureKind,
  metrics: PageViewportMetrics,
  covered?: CoveredCssRegion | null
): CoveredCssRegion {
  if (covered) return covered
  if (kind === 'full_page') {
    return {
      x: negate(metrics.scroll_x),
      y: negate(metrics.scroll_y),
      width: metrics.document_width,
      height: metrics.document_height
    }
  }
  return { x: 0, y: 0, width: metrics.viewport_width, height: metrics.viewport_height }
}

/**
 * Build the frame, or return null when it cannot be measured.
 *
 * Null is a real answer and the only honest one when an input is missing: a frame
 * assembled from a zero image width would report a scale of Infinity, and a caller
 * would act on it because it looks like a number. `observe` reports the absence
 * (see screenshot_frame_unavailable) instead of shipping a frame that misplaces
 * every click.
 */
export function buildCoordinateFrame(
  kind: CaptureKind,
  metrics: PageViewportMetrics,
  image: CapturedImageSize,
  covered?: CoveredCssRegion | null
): WireCoordinateFrame | null {
  const region = coveredRegionFor(kind, metrics, covered)
  if (!(image.width > 0) || !(image.height > 0)) return null
  if (!(region.width > 0) || !(region.height > 0)) return null
  if (!(metrics.viewport_width > 0) || !(metrics.viewport_height > 0)) return null

  const scaleX = region.width / image.width
  const scaleY = region.height / image.height
  return {
    capture: kind,
    image_width: image.width,
    image_height: image.height,
    viewport_width: metrics.viewport_width,
    viewport_height: metrics.viewport_height,
    scroll_x: metrics.scroll_x,
    scroll_y: metrics.scroll_y,
    document_width: metrics.document_width,
    document_height: metrics.document_height,
    device_pixel_ratio: metrics.device_pixel_ratio,
    clipped: isClipped(region, metrics),
    image_to_viewport: { scale_x: scaleX, scale_y: scaleY, offset_x: region.x, offset_y: region.y },
    viewport_bounds_in_image: viewportBoundsInImage(region, metrics, image),
    note: ''
  }
}

/**
 * Whether the image leaves part of the document out.
 *
 * Answered against the DOCUMENT, not the viewport: an image that shows the whole
 * viewport of a page twice as tall as the screen is still clipped, and a caller
 * told otherwise concludes a target it cannot see is absent rather than below the
 * fold. One CSS pixel of slack absorbs the sub-pixel difference between
 * scrollHeight and a fractional viewport height.
 */
function isClipped(region: CoveredCssRegion, metrics: PageViewportMetrics): boolean {
  const documentLeft = -metrics.scroll_x
  const documentTop = -metrics.scroll_y
  return (
    region.x > documentLeft + 1 ||
    region.y > documentTop + 1 ||
    region.x + region.width < documentLeft + metrics.document_width - 1 ||
    region.y + region.height < documentTop + metrics.document_height - 1
  )
}

/**
 * The part of the image that is on screen, in image pixels, clamped to the image.
 *
 * The viewport occupies CSS (0,0)-(viewport_width, viewport_height); converting
 * that back through the same affine map the frame publishes keeps this field and
 * `image_to_viewport` from ever disagreeing.
 */
function viewportBoundsInImage(
  region: CoveredCssRegion,
  metrics: PageViewportMetrics,
  image: CapturedImageSize
): WireImageRect {
  const scaleX = region.width / image.width
  const scaleY = region.height / image.height
  const left = clamp((0 - region.x) / scaleX, 0, image.width)
  const top = clamp((0 - region.y) / scaleY, 0, image.height)
  const right = clamp((metrics.viewport_width - region.x) / scaleX, 0, image.width)
  const bottom = clamp((metrics.viewport_height - region.y) / scaleY, 0, image.height)
  return { x: left, y: top, width: Math.max(0, right - left), height: Math.max(0, bottom - top) }
}

/**
 * Negate without producing -0.
 *
 * `-0` compares equal to `0` under `===` but not under `Object.is`, so an offset of
 * -0 makes a frame that is arithmetically identical to another one compare as
 * different — in a test, in a cache key, or in any dedupe that uses Object.is.
 */
function negate(value: number): number {
  return value === 0 ? 0 : -value
}

function clamp(value: number, low: number, high: number): number {
  if (!Number.isFinite(value)) return low
  return Math.min(high, Math.max(low, value))
}
