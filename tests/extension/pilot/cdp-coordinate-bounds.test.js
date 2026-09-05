// @ts-nocheck
/**
 * @fileoverview cdp-coordinate-bounds.test.js — The direct CDP query path refuses a point that is
 * not on the screen (kaboom-05ue.8).
 *
 * `click` with x/y is dispatched as a `cdp_action`, not a `dom_action`, so it never passes through
 * dom-dispatch's bounds check. Without its own check, `Input.dispatchMouseEvent` clamps an
 * out-of-range point onto the nearest edge and answers success: the agent is told it clicked at
 * (1900, 400) for a click that landed on the right border of a 1280-wide viewport.
 *
 * The check runs BEFORE the debugger is attached, which is what makes this testable without
 * chrome.debugger: an in-viewport point falls through to `cdp_unavailable`, and that difference is
 * the control proving the refusal above is the bounds check and not the missing debugger.
 */

import { test, describe, beforeEach, mock } from 'node:test'
import assert from 'node:assert'

const DISPATCH = '../../../extension/background/dom/cdp/cdp-dispatch.js'

const VIEWPORT = {
  viewport_width: 1280,
  viewport_height: 800,
  scroll_x: 0,
  scroll_y: 0,
  document_width: 1280,
  document_height: 4000,
  device_pixel_ratio: 1
}

function cdpQuery(params) {
  return {
    id: 'query-cdp',
    correlation_id: 'correlation-cdp',
    type: 'cdp_action',
    params: JSON.stringify(params),
    created_at: Date.now()
  }
}

describe('a direct CDP coordinate action is held inside the viewport', () => {
  let sent

  beforeEach(() => {
    sent = []
    globalThis.chrome = {
      scripting: {
        executeScript: mock.fn(() => Promise.resolve([{ frameId: 0, result: VIEWPORT }]))
      },
      storage: {
        local: {
          get: mock.fn(() => Promise.resolve({})),
          set: mock.fn(() => Promise.resolve()),
          remove: mock.fn(() => Promise.resolve())
        }
      }
      // No `debugger`: cdpSessions() is unavailable, so anything that gets past the bounds
      // check fails with cdp_unavailable instead of driving a real page.
    }
  })

  async function run(params) {
    const { executeCDPAction } = await import(DISPATCH)
    await executeCDPAction(
      cdpQuery(params),
      1,
      { id: 'test-client' },
      mock.fn((_client, _id, _correlation, status, _result, error) => sent.push({ status, error })),
      mock.fn()
    )
  }

  test('a click past the right edge is refused by name, not clamped', async () => {
    await run({ action: 'click', x: 1900, y: 400 })
    assert.deepStrictEqual(
      sent.map((entry) => entry.status),
      ['error']
    )
    assert.match(sent[0].error, /outside the viewport/)
    assert.match(sent[0].error, /1280x800/)
  })

  test('a click inside the viewport gets past the bounds check', async () => {
    await run({ action: 'click', x: 640, y: 400 })
    assert.deepStrictEqual(
      sent.map((entry) => entry.status),
      ['error']
    )
    // Control: it reached the debugger, which this environment does not have. If the bounds
    // check were rejecting everything, this would read "outside the viewport" instead.
    assert.match(sent[0].error, /cdp_unavailable/)
  })

  test('a capture may name a rectangle the viewport does not contain', async () => {
    // zoom_region reads pixels and dispatches no input, so its x/y is a region origin rather
    // than a place to click. Refusing it here would break capturing below the fold.
    await run({ action: 'zoom_region', x: 0, y: 2000, width: 400, height: 300 })
    assert.equal(sent.length, 1)
    assert.doesNotMatch(sent[0].error ?? '', /outside the viewport/)
  })
})
