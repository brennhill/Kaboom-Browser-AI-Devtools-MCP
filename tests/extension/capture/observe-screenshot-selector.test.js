// @ts-nocheck
/**
 * @fileoverview observe-screenshot-selector.test.js — Regression tests for issue #597.
 *
 * `observe(what:"screenshot", selector:"…")` used to silently ignore `selector`,
 * returning the un-scrolled viewport (so an element inside a nested overflow:auto
 * container appeared at the container's top). The fix scrolls the element into
 * view and crops the DPR-scaled viewport capture to it. This covers the pure
 * DPR/clamp crop math; the OffscreenCanvas crop + scrollIntoView are validated in
 * live/UAT browser testing (they need a real DOM + compositor).
 */

import { test, describe, beforeEach } from 'node:test'
import assert from 'node:assert'

describe('computeElementCropRect (#597)', () => {
  let computeElementCropRect

  beforeEach(async () => {
    ;({ computeElementCropRect } = await import('../../../extension/background/commands/observe.js'))
  })

  test('dpr=1: crop rect equals the CSS rect', () => {
    const r = computeElementCropRect({ x: 100, y: 200, width: 80, height: 32 }, 1, 1280, 800)
    assert.deepStrictEqual(r, { sx: 100, sy: 200, sw: 80, sh: 32 })
  })

  test('dpr=2: capture image is DPR-scaled, so the crop scales by DPR', () => {
    const r = computeElementCropRect({ x: 100, y: 200, width: 80, height: 32 }, 2, 2560, 1600)
    assert.deepStrictEqual(r, { sx: 200, sy: 400, sw: 160, sh: 64 })
  })

  test('negative offsets (element crossing the top-left edge) clamp to 0', () => {
    const r = computeElementCropRect({ x: -10, y: -5, width: 50, height: 40 }, 1, 1280, 800)
    assert.strictEqual(r.sx, 0)
    assert.strictEqual(r.sy, 0)
  })

  test('an element wider/taller than the remaining image is clamped to image bounds', () => {
    const r = computeElementCropRect({ x: 1200, y: 760, width: 400, height: 400 }, 1, 1280, 800)
    assert.strictEqual(r.sx, 1200)
    assert.strictEqual(r.sy, 760)
    assert.strictEqual(r.sw, 80) // 1280 - 1200
    assert.strictEqual(r.sh, 40) // 800 - 760
  })

  test('zero or negative size returns null (nothing to crop)', () => {
    assert.strictEqual(computeElementCropRect({ x: 0, y: 0, width: 0, height: 10 }, 1, 100, 100), null)
    assert.strictEqual(computeElementCropRect({ x: 0, y: 0, width: 10, height: 0 }, 1, 100, 100), null)
  })

  test('an element fully outside the captured image returns null', () => {
    assert.strictEqual(computeElementCropRect({ x: 2000, y: 0, width: 50, height: 50 }, 1, 1280, 800), null)
  })

  test('dpr<=0 is treated as 1 (defensive)', () => {
    const r = computeElementCropRect({ x: 10, y: 10, width: 20, height: 20 }, 0, 100, 100)
    assert.deepStrictEqual(r, { sx: 10, sy: 10, sw: 20, sh: 20 })
  })
})

// The crop path originally decoded the capture with fetch('data:...'), which MV3
// service workers restrict — the throw was swallowed and every selector screenshot
// silently returned the uncropped viewport. Decoding is now fetch-free.
describe('dataUrlToBlob (#597 crop decode)', () => {
  let dataUrlToBlob

  beforeEach(async () => {
    ;({ dataUrlToBlob } = await import('../../../extension/lib/screenshot/image-size.js'))
  })

  test('decodes a base64 data URL into a Blob with the right type and bytes', async () => {
    // "hi" -> aGk=
    const blob = dataUrlToBlob('data:image/png;base64,aGk=')
    assert.strictEqual(blob.type, 'image/png')
    assert.strictEqual(blob.size, 2)
    assert.strictEqual(await blob.text(), 'hi')
  })

  test('preserves binary bytes that are not valid UTF-8 text', async () => {
    // 0x89 0x50 0x4E 0x47 = PNG magic
    const blob = dataUrlToBlob('data:image/png;base64,iVBORw==')
    const bytes = new Uint8Array(await blob.arrayBuffer())
    assert.deepStrictEqual(Array.from(bytes.slice(0, 4)), [0x89, 0x50, 0x4e, 0x47])
  })

  test('rejects a non-data URL', () => {
    assert.throws(() => dataUrlToBlob('https://example.com/x.png'), /not a data URL/)
  })

  test('rejects a non-base64 data URL', () => {
    assert.throws(() => dataUrlToBlob('data:image/png,rawbytes'), /not base64/)
  })
})
