// @ts-nocheck
/**
 * @fileoverview screenshot-coordinate-frame.test.js — The screenshot-to-coordinate loop.
 *
 * A screenshot with no frame of reference cannot be clicked: the image is in device
 * pixels, `click` takes CSS pixels relative to the viewport, and on a retina display
 * those differ by 2x. These tests hold the frame to the one promise it makes — a
 * pixel read off the returned image maps to the coordinate that hits the element —
 * for each capture kind, and prove that an unmeasurable capture reports no frame
 * rather than a wrong one.
 */

import { describe, test } from 'node:test'
import assert from 'node:assert'

const { buildCoordinateFrame, coveredRegionFor } = await import(
  '../../../extension/lib/screenshot/coordinate-frame.js'
)

/** A 1280x720 CSS viewport on a 2x display, on a page 4000 CSS px tall. */
function retinaMetrics(overrides = {}) {
  return {
    viewport_width: 1280,
    viewport_height: 720,
    scroll_x: 0,
    scroll_y: 0,
    document_width: 1280,
    document_height: 4000,
    device_pixel_ratio: 2,
    ...overrides
  }
}

/** Apply the frame the way an agent would: read a pixel, act at the coordinate. */
function act(frame, imageX, imageY) {
  const m = frame.image_to_viewport
  return { x: imageX * m.scale_x + m.offset_x, y: imageY * m.scale_y + m.offset_y }
}

describe('a coordinate read off the image lands on the element', () => {
  test('a viewport capture on a 2x display halves the image coordinate', () => {
    const frame = buildCoordinateFrame('viewport', retinaMetrics(), { width: 2560, height: 1440 })

    // A button drawn at CSS (400,300) is at image pixel (800,600) on this display.
    assert.deepStrictEqual(act(frame, 800, 600), { x: 400, y: 300 })
    assert.strictEqual(frame.image_to_viewport.scale_x, 0.5)

    // Discriminating control: the identity mapping an agent uses without a frame
    // gives 800,600 — a different element. Without this the assertion above would
    // hold for any frame that happened to report scale 1.
    assert.notDeepStrictEqual(act(frame, 800, 600), { x: 800, y: 600 })
  })

  test('the CDP clip wins over the page\'s own viewport size', () => {
    // The CDP path clips to cssVisualViewport, which excludes a classic scrollbar.
    // Reporting innerWidth (1280) for an image that actually covers 1265 CSS px
    // would misplace a click at the right edge by 15px — onto the scrollbar.
    const covered = { x: 0, y: 0, width: 1265, height: 720 }
    const frame = buildCoordinateFrame('viewport', retinaMetrics(), { width: 2530, height: 1440 }, covered)

    assert.deepStrictEqual(act(frame, 2530, 0), { x: 1265, y: 0 })

    const ignoringClip = buildCoordinateFrame('viewport', retinaMetrics(), { width: 2530, height: 1440 })
    assert.notStrictEqual(
      ignoringClip.image_to_viewport.scale_x,
      frame.image_to_viewport.scale_x,
      'control: the two regions must disagree, or this test proves nothing'
    )
  })

  test('an element crop maps back through the crop origin', () => {
    // The image is just the button: 80x32 CSS px at (100,200), captured at 2x.
    const rect = { x: 100, y: 200, width: 80, height: 32 }
    const frame = buildCoordinateFrame('element', retinaMetrics(), { width: 160, height: 64 }, rect)

    assert.strictEqual(frame.capture, 'element')
    // The centre of the cropped image is the centre of the button.
    assert.deepStrictEqual(act(frame, 80, 32), { x: 140, y: 216 })
    // Its top-left is the element's own origin, not the viewport's.
    assert.deepStrictEqual(act(frame, 0, 0), { x: 100, y: 200 })
  })

  test('a full-page image is in document coordinates and subtracts the scroll', () => {
    const metrics = retinaMetrics({ scroll_y: 900 })
    const covered = { x: 0, y: -900, width: 1280, height: 4000 }
    const frame = buildCoordinateFrame('full_page', metrics, { width: 1280, height: 4000 }, covered)

    // A heading at document y=1000 is 100px below the top of the screen.
    assert.deepStrictEqual(act(frame, 0, 1000), { x: 0, y: 100 })
    // One at document y=100 is 800px above it — a negative answer, which is the
    // honest one: acting on it without scrolling first clicks nothing.
    assert.deepStrictEqual(act(frame, 0, 100), { x: 0, y: -800 })
  })

  test('a full-page capture clamped by the texture limit reports the clamp', () => {
    // computeFullPageCaptureDimensions caps height at 16384. A 40000px page is
    // therefore captured short. A frame claiming to cover the whole document would
    // stretch every y by 40000/16384 and put every click 2.4x too far down.
    const metrics = retinaMetrics({ document_height: 40000 })
    const covered = { x: 0, y: 0, width: 1280, height: 16384 }
    const frame = buildCoordinateFrame('full_page', metrics, { width: 1280, height: 16384 }, covered)

    assert.strictEqual(frame.image_to_viewport.scale_y, 1)
    assert.strictEqual(frame.clipped, true, 'a clamped capture leaves most of the document out')
    assert.deepStrictEqual(act(frame, 0, 16384), { x: 0, y: 16384 })
  })
})

describe('viewport_bounds_in_image says what can be clicked right now', () => {
  test('a viewport capture is entirely on screen', () => {
    const frame = buildCoordinateFrame('viewport', retinaMetrics(), { width: 2560, height: 1440 })
    assert.deepStrictEqual(frame.viewport_bounds_in_image, { x: 0, y: 0, width: 2560, height: 1440 })
  })

  test('a scrolled full-page image reports only the scrolled window', () => {
    const metrics = retinaMetrics({ scroll_y: 900 })
    const covered = { x: 0, y: -900, width: 1280, height: 4000 }
    const frame = buildCoordinateFrame('full_page', metrics, { width: 1280, height: 4000 }, covered)

    assert.deepStrictEqual(frame.viewport_bounds_in_image, { x: 0, y: 900, width: 1280, height: 720 })

    // The bounds and the mapping are two answers to the same question and must
    // agree: the top of the reported window maps to viewport y=0.
    assert.strictEqual(act(frame, 0, frame.viewport_bounds_in_image.y).y, 0)
    assert.strictEqual(
      act(frame, 0, frame.viewport_bounds_in_image.y + frame.viewport_bounds_in_image.height).y,
      metrics.viewport_height
    )
  })
})

describe('clipped reports honestly whether the image left anything out', () => {
  test('a viewport capture of a page taller than the screen is clipped', () => {
    const frame = buildCoordinateFrame('viewport', retinaMetrics(), { width: 2560, height: 1440 })
    assert.strictEqual(frame.clipped, true, 'a target below the fold is absent from this image, not from the page')
  })

  test('a viewport capture of a page that fits is not clipped', () => {
    const metrics = retinaMetrics({ document_height: 720 })
    const frame = buildCoordinateFrame('viewport', metrics, { width: 2560, height: 1440 })
    assert.strictEqual(frame.clipped, false)
  })

  test('a full-page capture of the whole document is not clipped', () => {
    const covered = { x: 0, y: 0, width: 1280, height: 4000 }
    const frame = buildCoordinateFrame('full_page', retinaMetrics(), { width: 1280, height: 4000 }, covered)
    assert.strictEqual(frame.clipped, false)
  })
})

describe('an unmeasurable capture reports no frame instead of a wrong one', () => {
  test('a zero-width image yields null, not a scale of Infinity', () => {
    assert.strictEqual(buildCoordinateFrame('viewport', retinaMetrics(), { width: 0, height: 1440 }), null)
  })

  test('a page reporting no viewport yields null', () => {
    const metrics = retinaMetrics({ viewport_width: 0, viewport_height: 0 })
    assert.strictEqual(buildCoordinateFrame('viewport', metrics, { width: 2560, height: 1440 }), null)
  })

  test('an empty covered region yields null', () => {
    const covered = { x: 10, y: 10, width: 0, height: 0 }
    assert.strictEqual(buildCoordinateFrame('element', retinaMetrics(), { width: 160, height: 64 }, covered), null)
  })

  test('control: the same inputs with real dimensions do produce a frame', () => {
    // Without this the three assertions above would hold for a builder that
    // returned null unconditionally.
    assert.notStrictEqual(buildCoordinateFrame('viewport', retinaMetrics(), { width: 2560, height: 1440 }), null)
  })
})

describe('coveredRegionFor keeps each capture kind on its own origin', () => {
  test('a reported region always wins over the kind-derived fallback', () => {
    const explicit = { x: 5, y: 6, width: 7, height: 8 }
    assert.deepStrictEqual(coveredRegionFor('viewport', retinaMetrics(), explicit), explicit)
    assert.deepStrictEqual(coveredRegionFor('full_page', retinaMetrics(), explicit), explicit)
  })

  test('full_page falls back to the document, offset by the scroll', () => {
    const region = coveredRegionFor('full_page', retinaMetrics({ scroll_y: 900 }), null)
    assert.deepStrictEqual(region, { x: 0, y: -900, width: 1280, height: 4000 })
  })

  test('viewport falls back to the page\'s own viewport at the origin', () => {
    const region = coveredRegionFor('viewport', retinaMetrics({ scroll_y: 900 }), null)
    assert.deepStrictEqual(region, { x: 0, y: 0, width: 1280, height: 720 })
  })
})
