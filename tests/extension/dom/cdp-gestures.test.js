// @ts-nocheck
/**
 * @fileoverview cdp-gestures.test.js — Pure-function contracts for the pointer gesture surface
 * (kaboom-05ue.5).
 *
 * Everything covered here decides what Chrome is actually told to do. A wrong modifier bit is a
 * plain click where the agent asked for ctrl+click; a two-point drag path is a press-and-jump that
 * no drag library ever starts; a clickCount of 1 repeated twice is two clicks, never a dblclick.
 * None of it needs a browser, so none of it is left to UAT.
 */

import { test, describe, before } from 'node:test'
import assert from 'node:assert'

const CDP = '../../../extension/background/dom/cdp/cdp-gestures.js'
const KEYS = '../../../extension/background/dom/cdp/cdp-key-mappings.js'
const DOM = '../../../extension/background/dom/primitives/gestures/dom-primitives-gestures.js'

/** Records every CDP command a gesture dispatches, plus the cursor trail it drew. */
function recordingContext(overrides = {}) {
  const sent = []
  const cursor = []
  return {
    sent,
    cursor,
    ctx: {
      send: async (method, params) => {
        sent.push({ method, params })
        if (overrides.send) return overrides.send(method, params)
        return undefined
      },
      cursor: (x, y) => cursor.push({ x, y })
    }
  }
}

describe('modifier bitmask', () => {
  let modifierBitmask, gestureModifierMask

  before(async () => {
    ;({ modifierBitmask } = await import(KEYS))
    ;({ gestureModifierMask } = await import(DOM))
  })

  test('uses the bits Chrome defines: alt=1 ctrl=2 meta=4 shift=8', () => {
    assert.equal(modifierBitmask(['alt']), 1)
    assert.equal(modifierBitmask(['ctrl']), 2)
    assert.equal(modifierBitmask(['control']), 2)
    assert.equal(modifierBitmask(['cmd']), 4)
    assert.equal(modifierBitmask(['meta']), 4)
    assert.equal(modifierBitmask(['command']), 4)
    assert.equal(modifierBitmask(['shift']), 8)
  })

  test('combines modifiers and ignores case and padding', () => {
    assert.equal(modifierBitmask([' Ctrl ', 'SHIFT']), 10)
  })

  test('an unknown modifier is dropped, not fatal — the click still lands', () => {
    assert.equal(modifierBitmask(['hyper']), 0)
    assert.equal(modifierBitmask(['ctrl', 'hyper']), 2)
  })

  test('absent or empty modifiers are zero', () => {
    assert.equal(modifierBitmask(undefined), 0)
    assert.equal(modifierBitmask([]), 0)
  })

  test('the DOM fallback reports the same number as CDP', () => {
    for (const names of [['ctrl'], ['shift', 'alt'], ['cmd', 'ctrl', 'shift'], []]) {
      assert.equal(gestureModifierMask(names), modifierBitmask(names), names.join('+'))
    }
  })
})

describe('drag path normalization', () => {
  let normalizeDragPath, densifyDragPath, DRAG_STEPS_PER_SEGMENT

  before(async () => {
    ;({ normalizeDragPath, densifyDragPath, DRAG_STEPS_PER_SEGMENT } = await import(CDP))
  })

  test('rejects a path that cannot be dragged along', () => {
    assert.throws(() => normalizeDragPath(undefined), /drag_path with at least 2 points/)
    assert.throws(() => normalizeDragPath([]), /drag_path with at least 2 points/)
    assert.throws(() => normalizeDragPath([{ x: 1, y: 1 }]), /drag_path with at least 2 points/)
  })

  test('names the offending point when a coordinate is not a number', () => {
    assert.throws(() => normalizeDragPath([{ x: 1, y: 1 }, { x: '2', y: 3 }]), /drag_path point 1/)
  })

  test('copies the points so a later mutation cannot rewrite the dispatched path', () => {
    const source = [
      { x: 1, y: 2 },
      { x: 3, y: 4 }
    ]
    const normalized = normalizeDragPath(source)
    source[0].x = 999
    assert.deepEqual(normalized[0], { x: 1, y: 2 })
  })

  test('densify turns a two-point jump into a continuous route', () => {
    const dense = densifyDragPath([
      { x: 0, y: 0 },
      { x: 100, y: 0 }
    ])
    assert.equal(dense.length, 1 + DRAG_STEPS_PER_SEGMENT)
    assert.deepEqual(dense[0], { x: 0, y: 0 })
    assert.deepEqual(dense[dense.length - 1], { x: 100, y: 0 })
  })

  test('densify keeps every caller waypoint and adds steps per segment', () => {
    const dense = densifyDragPath(
      [
        { x: 0, y: 0 },
        { x: 10, y: 0 },
        { x: 10, y: 10 }
      ],
      2
    )
    assert.equal(dense.length, 1 + 2 * 2)
    assert.deepEqual(dense[2], { x: 10, y: 0 })
    assert.deepEqual(dense[4], { x: 10, y: 10 })
  })
})

describe('explicitGesturePoint', () => {
  let explicitGesturePoint

  before(async () => {
    ;({ explicitGesturePoint } = await import(CDP))
  })

  test('explicit x/y wins over a path', () => {
    assert.deepEqual(explicitGesturePoint({ x: 5, y: 6, drag_path: [{ x: 1, y: 1 }] }), { x: 5, y: 6 })
  })

  test('a drag anchors on its first path point', () => {
    assert.deepEqual(
      explicitGesturePoint({
        drag_path: [
          { x: 7, y: 8 },
          { x: 9, y: 9 }
        ]
      }),
      { x: 7, y: 8 }
    )
  })

  test('a half-supplied coordinate is not a point', () => {
    assert.equal(explicitGesturePoint({ x: 5 }), null)
    assert.equal(explicitGesturePoint({}), null)
  })
})

describe('isCDPGesture', () => {
  let isCDPGesture, CDP_GESTURE_ACTIONS

  before(async () => {
    ;({ isCDPGesture, CDP_GESTURE_ACTIONS } = await import(CDP))
  })

  test('covers exactly the six pointer gestures', () => {
    assert.deepEqual(
      [...CDP_GESTURE_ACTIONS].sort(),
      ['double_click', 'drag', 'hover_at', 'right_click', 'scroll_at', 'triple_click']
    )
  })

  test('click and type are not gestures — they keep their own CDP paths', () => {
    assert.equal(isCDPGesture('click'), false)
    assert.equal(isCDPGesture('type'), false)
    assert.equal(isCDPGesture('zoom_region'), false)
  })
})

describe('dispatchClickBurst', () => {
  let dispatchClickBurst

  before(async () => {
    ;({ dispatchClickBurst } = await import(CDP))
  })

  test('one press/release pair carries the whole clickCount', async () => {
    const { ctx, sent } = recordingContext()
    await dispatchClickBurst(ctx, { x: 10, y: 20 }, { button: 'left', clickCount: 2, modifiers: 0 })
    const types = sent.map((s) => s.params.type)
    assert.deepEqual(types, ['mouseMoved', 'mousePressed', 'mouseReleased'])
    assert.equal(sent[1].params.clickCount, 2)
    assert.equal(sent[2].params.clickCount, 2)
  })

  test('the right button sets Chrome buttons bit 2 while held and clears it on release', async () => {
    const { ctx, sent } = recordingContext()
    await dispatchClickBurst(ctx, { x: 1, y: 2 }, { button: 'right', clickCount: 1, modifiers: 0 })
    assert.equal(sent[1].params.buttons, 2)
    assert.equal(sent[2].params.buttons, 0)
  })

  test('modifiers ride on every event in the burst', async () => {
    const { ctx, sent } = recordingContext()
    await dispatchClickBurst(ctx, { x: 1, y: 2 }, { button: 'left', clickCount: 1, modifiers: 10 })
    for (const entry of sent) assert.equal(entry.params.modifiers, 10)
  })

  test('the supervision cursor moves before the press, not after', async () => {
    const { ctx, cursor } = recordingContext()
    await dispatchClickBurst(ctx, { x: 42, y: 43 }, { button: 'left', clickCount: 1, modifiers: 0 })
    assert.deepEqual(cursor, [{ x: 42, y: 43 }])
  })
})

describe('buildContextMenuExpression', () => {
  let buildContextMenuExpression

  before(async () => {
    ;({ buildContextMenuExpression } = await import(CDP))
  })

  test('targets the element under the coordinate', () => {
    const expr = buildContextMenuExpression({ x: 12, y: 34 }, 0)
    assert.match(expr, /elementFromPoint\(12, 34\)/)
    assert.match(expr, /new MouseEvent\('contextmenu'/)
  })

  test('translates the CDP bitmask back into the DOM event flags', () => {
    const expr = buildContextMenuExpression({ x: 0, y: 0 }, 10)
    assert.match(expr, /altKey: false/)
    assert.match(expr, /ctrlKey: true/)
    assert.match(expr, /metaKey: false/)
    assert.match(expr, /shiftKey: true/)
  })
})

describe('executeCDPGesture', () => {
  let executeCDPGesture

  before(async () => {
    ;({ executeCDPGesture } = await import(CDP))
  })

  test('right_click reports whether the contextmenu event reached the page', async () => {
    const { ctx } = recordingContext({
      send: (method) => (method === 'Runtime.evaluate' ? { result: { value: { dispatched: true } } } : undefined)
    })
    const evidence = await executeCDPGesture(ctx, 'right_click', {}, { x: 1, y: 2 })
    assert.equal(evidence.context_menu, true)
    assert.equal(evidence.button, 'right')
  })

  test('a page that has nothing under the point reports context_menu false, not success', async () => {
    const { ctx } = recordingContext({
      send: (method) => (method === 'Runtime.evaluate' ? { result: { value: { dispatched: false } } } : undefined)
    })
    const evidence = await executeCDPGesture(ctx, 'right_click', {}, { x: 1, y: 2 })
    assert.equal(evidence.context_menu, false)
  })

  test('double_click and triple_click send one burst with clickCount 2 and 3', async () => {
    for (const [action, count] of [
      ['double_click', 2],
      ['triple_click', 3]
    ]) {
      const { ctx, sent } = recordingContext()
      const evidence = await executeCDPGesture(ctx, action, {}, { x: 3, y: 4 })
      const presses = sent.filter((s) => s.params.type === 'mousePressed')
      assert.equal(presses.length, 1, `${action} must not send separate clicks`)
      assert.equal(presses[0].params.clickCount, count)
      assert.equal(evidence.click_count, count)
    }
  })

  test('scroll_at defaults a missing delta to zero rather than NaN', async () => {
    const { ctx, sent } = recordingContext()
    const evidence = await executeCDPGesture(ctx, 'scroll_at', { delta_y: 240 }, { x: 5, y: 6 })
    const wheel = sent.find((s) => s.params.type === 'mouseWheel')
    assert.equal(wheel.params.deltaX, 0)
    assert.equal(wheel.params.deltaY, 240)
    assert.deepEqual({ x: evidence.delta_x, y: evidence.delta_y }, { x: 0, y: 240 })
  })

  test('hover_at moves the pointer and presses nothing', async () => {
    const { ctx, sent } = recordingContext()
    await executeCDPGesture(ctx, 'hover_at', {}, { x: 8, y: 9 })
    assert.deepEqual(
      sent.map((s) => s.params.type),
      ['mouseMoved']
    )
  })

  test('drag holds the button down across every intermediate move', async () => {
    const { ctx, sent } = recordingContext()
    const evidence = await executeCDPGesture(
      ctx,
      'drag',
      {
        drag_path: [
          { x: 0, y: 0 },
          { x: 40, y: 0 }
        ]
      },
      { x: 0, y: 0 }
    )
    const mouse = sent.filter((s) => s.method === 'Input.dispatchMouseEvent')
    assert.equal(mouse[0].params.type, 'mouseMoved')
    assert.equal(mouse[1].params.type, 'mousePressed')
    assert.equal(mouse[mouse.length - 1].params.type, 'mouseReleased')
    const held = mouse.slice(2, mouse.length - 1)
    assert.ok(held.length >= 4, 'drag must dispatch intermediate moves')
    for (const move of held) assert.equal(move.params.buttons, 1)
    assert.equal(evidence.path_points, 2)
    assert.equal(evidence.html5_drag, true)
  })

  test('a drag whose HTML5 events Chrome refuses still completes and says so', async () => {
    const { ctx, sent } = recordingContext({
      send: (method) => {
        if (method === 'Input.dispatchDragEvent') throw new Error('not supported')
        return undefined
      }
    })
    const evidence = await executeCDPGesture(
      ctx,
      'drag',
      {
        drag_path: [
          { x: 0, y: 0 },
          { x: 10, y: 10 }
        ]
      },
      { x: 0, y: 0 }
    )
    assert.equal(evidence.html5_drag, false)
    const released = sent.filter((s) => s.params.type === 'mouseReleased')
    assert.equal(released.length, 1, 'the pointer must not be left held down')
  })

  test('an unrouted action throws instead of reporting a gesture nobody ran', async () => {
    const { ctx } = recordingContext()
    await assert.rejects(() => executeCDPGesture(ctx, 'click', {}, { x: 0, y: 0 }), /Unknown CDP gesture/)
  })
})

describe('normalizeZoomClip', () => {
  let normalizeZoomClip

  before(async () => {
    ;({ normalizeZoomClip } = await import(CDP))
  })

  test('rejects a clip Chrome would turn into an empty capture', () => {
    assert.throws(() => normalizeZoomClip({ width: 10, height: 10 }), /numeric x and y/)
    assert.throws(() => normalizeZoomClip({ x: 0, y: 0, width: 0, height: 10 }), /greater than 0/)
    assert.throws(() => normalizeZoomClip({ x: 0, y: 0, width: 10, height: -1 }), /greater than 0/)
  })

  test('defaults scale to 1 and caps it at 4', () => {
    assert.equal(normalizeZoomClip({ x: 0, y: 0, width: 10, height: 10 }).scale, 1)
    assert.equal(normalizeZoomClip({ x: 0, y: 0, width: 10, height: 10, scale: 0 }).scale, 1)
    assert.equal(normalizeZoomClip({ x: 0, y: 0, width: 10, height: 10, scale: 99 }).scale, 4)
    assert.equal(normalizeZoomClip({ x: 0, y: 0, width: 10, height: 10, scale: 2.5 }).scale, 2.5)
  })

  test('passes the rectangle through unchanged', () => {
    assert.deepEqual(normalizeZoomClip({ x: 12, y: 34, width: 56, height: 78, scale: 2 }), {
      x: 12,
      y: 34,
      width: 56,
      height: 78,
      scale: 2
    })
  })
})

describe('captureZoomRegion', () => {
  let captureZoomRegion

  before(async () => {
    ;({ captureZoomRegion } = await import(CDP))
  })

  test('returns a data URL Chrome can hand straight to the daemon', async () => {
    const clip = { x: 0, y: 0, width: 4, height: 4, scale: 1 }
    const seen = []
    const url = await captureZoomRegion(async (method, params) => {
      seen.push({ method, params })
      return { data: 'QUJD' }
    }, clip)
    assert.equal(url, 'data:image/png;base64,QUJD')
    assert.equal(seen[0].method, 'Page.captureScreenshot')
    assert.deepEqual(seen[0].params.clip, clip)
    assert.equal(seen[0].params.captureBeyondViewport, false)
  })

  test('an empty capture is an error, not a zero-byte image', async () => {
    await assert.rejects(
      () => captureZoomRegion(async () => ({}), { x: 0, y: 0, width: 1, height: 1, scale: 1 }),
      /no image data/
    )
  })
})

describe('deliverZoomRegion', () => {
  let deliverZoomRegion
  const originalChrome = globalThis.chrome
  const originalFetch = globalThis.fetch

  before(async () => {
    ;({ deliverZoomRegion } = await import(CDP))
  })

  /**
   * @param response what the daemon returns for the capture upload
   * @param metrics what the injected viewport probe reports, or null to make it fail
   */
  function stubBrowser(response, metrics = null) {
    globalThis.chrome = {
      tabs: { get: async () => ({ url: 'https://example.test/a' }) },
      scripting: {
        executeScript: async () => {
          if (!metrics) throw new Error('probe blocked')
          return [{ result: metrics }]
        }
      }
    }
    globalThis.fetch = async () => response
  }

  /** A page scrolled 900px down in a 1280x720 viewport. */
  function scrolledMetrics() {
    return {
      viewport_width: 1280,
      viewport_height: 720,
      scroll_x: 0,
      scroll_y: 900,
      document_width: 1280,
      document_height: 4000,
      device_pixel_ratio: 2
    }
  }

  function restoreBrowser() {
    globalThis.chrome = originalChrome
    globalThis.fetch = originalFetch
  }

  test('returns the terminal result the command handler must send', async () => {
    stubBrowser({
      ok: true,
      status: 200,
      json: async () => ({ filename: 'shot.png', path: '/tmp/shot.png' })
    })
    try {
      const result = await deliverZoomRegion(
        async () => ({ data: 'QUJD' }),
        { x: 10, y: 20, width: 100, height: 50, scale: 2 },
        { tabId: 7, queryId: 'q-1' }
      )
      // A handler that returns nothing is answered by the registry with `no_result`, which
      // overwrites the capture the daemon just saved.
      assert.equal(result.success, true)
      assert.equal(result.action, 'zoom_region')
      assert.equal(result.path, '/tmp/shot.png')
      assert.equal(result.data_url, 'data:image/png;base64,QUJD')
      assert.deepEqual(result.clip, { x: 10, y: 20, width: 100, height: 50, scale: 2 })
    } finally {
      restoreBrowser()
    }
  })

  test('a zoom on a scrolled page clips the region the caller pointed at', async () => {
    // zoom_region documents x and y as VIEWPORT pixels; Page.captureScreenshot clips
    // in DOCUMENT coordinates (this file's own viewport capture passes
    // cssVisualViewport.pageX/pageY as the clip origin to photograph the visible
    // area). Without the translation, zooming (320,180) on a page scrolled 900px
    // down captures a rectangle 900px above the one the caller pointed at and
    // reports success, so the agent inspects the wrong content and acts on it.
    let sentClip = null
    stubBrowser(
      { ok: true, status: 200, json: async () => ({ filename: 's.png', path: '/tmp/s.png' }) },
      scrolledMetrics()
    )
    try {
      const result = await deliverZoomRegion(
        async (_method, params) => {
          sentClip = params.clip
          return { data: 'QUJD' }
        },
        { x: 320, y: 180, width: 400, height: 220 },
        { tabId: 7, queryId: 'q-scroll' }
      )
      assert.deepEqual(sentClip, { x: 320, y: 1080, width: 400, height: 220, scale: 1 })
      // The caller's own coordinates come back unchanged, so the result still
      // describes the region that was asked for.
      assert.deepEqual(result.clip, { x: 320, y: 180, width: 400, height: 220, scale: 1 })
      assert.deepEqual(result.page_clip, sentClip)
    } finally {
      restoreBrowser()
    }
  })

  test('an unscrolled page sends the caller coordinates unchanged', async () => {
    // Discriminating control: the translation must be the scroll offset and not a
    // constant. Without this arm the assertion above would hold for any offset.
    let sentClip = null
    stubBrowser({ ok: true, status: 200, json: async () => ({ filename: 's.png', path: '/tmp/s.png' }) }, {
      ...scrolledMetrics(),
      scroll_y: 0
    })
    try {
      await deliverZoomRegion(
        async (_method, params) => {
          sentClip = params.clip
          return { data: 'QUJD' }
        },
        { x: 320, y: 180, width: 400, height: 220 },
        { tabId: 7, queryId: 'q-top' }
      )
      assert.deepEqual(sentClip, { x: 320, y: 180, width: 400, height: 220, scale: 1 })
    } finally {
      restoreBrowser()
    }
  })

  test('a blocked viewport probe reports no frame rather than a wrong one', async () => {
    stubBrowser({ ok: true, status: 200, json: async () => ({ filename: 's.png', path: '/tmp/s.png' }) }, null)
    try {
      const result = await deliverZoomRegion(
        async () => ({ data: 'QUJD' }),
        { x: 0, y: 0, width: 10, height: 10 },
        { tabId: 7, queryId: 'q-noprobe' }
      )
      assert.equal(result.coordinate_frame, undefined)
      assert.equal(result.coordinate_frame_error, 'viewport_metrics_unavailable')
    } finally {
      restoreBrowser()
    }
  })

  test('a daemon that rejects the capture is an error, not a silent success', async () => {
    stubBrowser({ ok: false, status: 507, json: async () => ({}) })
    try {
      await assert.rejects(
        () =>
          deliverZoomRegion(
            async () => ({ data: 'QUJD' }),
            { x: 0, y: 0, width: 10, height: 10 },
            { tabId: 7, queryId: 'q-2' }
          ),
        /HTTP 507/
      )
    } finally {
      restoreBrowser()
    }
  })
})

describe('DOM fallback evidence', () => {
  let buildGestureDOMResult, gesturePrimitiveFor, domGestureClick, domGestureScroll, domGestureDrag

  before(async () => {
    ;({ buildGestureDOMResult, gesturePrimitiveFor, domGestureClick, domGestureScroll, domGestureDrag } =
      await import(DOM))
  })

  test('each gesture is injected as the primitive that implements it', () => {
    assert.equal(gesturePrimitiveFor('drag'), domGestureDrag)
    assert.equal(gesturePrimitiveFor('hover_at'), domGestureScroll)
    assert.equal(gesturePrimitiveFor('scroll_at'), domGestureScroll)
    for (const action of ['click', 'right_click', 'double_click', 'triple_click']) {
      assert.equal(gesturePrimitiveFor(action), domGestureClick, action)
    }
  })

  test('a gesture the page could not dispatch is reported as a failure, not a success', () => {
    const result = buildGestureDOMResult(
      'right_click',
      '#row',
      { x: 5, y: 6 },
      null,
      { dispatched: false, error: 'no_element_at_point' },
      0
    )
    assert.equal(result.success, false)
    assert.equal(result.error, 'no_element_at_point')
    assert.match(result.message, /\(5, 6\)/)
  })

  test('a primitive that returned nothing at all is a failure, not an empty success', () => {
    const result = buildGestureDOMResult('drag', '', { x: 0, y: 0 }, null, undefined, 0)
    assert.equal(result.success, false)
    assert.equal(result.error, 'gesture_not_dispatched')
  })

  test('the resolved element becomes the matched evidence, and dispatched is not leaked', () => {
    const resolved = {
      x: 10,
      y: 20,
      tag: 'button',
      text_preview: 'Save',
      selector: '#save',
      element_id: 'e3',
      bbox: { x: 1, y: 2, width: 3, height: 4 }
    }
    const result = buildGestureDOMResult(
      'double_click',
      '#save',
      { x: 10, y: 20 },
      resolved,
      { dispatched: true, button: 'left', click_count: 2 },
      2
    )
    assert.equal(result.success, true)
    assert.equal(result.insertion_strategy, 'dom')
    assert.equal(result.modifiers, 2)
    assert.equal(result.click_count, 2)
    assert.equal(result.matched.element_id, 'e3')
    assert.deepEqual(result.matched.bbox, resolved.bbox)
    assert.equal('dispatched' in result, false)
    assert.equal('error' in result, false)
  })

  test('a coordinate gesture reports the coordinate and claims no matched element', () => {
    const result = buildGestureDOMResult(
      'scroll_at',
      '',
      { x: 120, y: 300 },
      null,
      { dispatched: true, delta_x: 0, delta_y: 400 },
      0
    )
    assert.deepEqual({ x: result.x, y: result.y }, { x: 120, y: 300 })
    assert.equal(result.matched, undefined)
    assert.equal(result.delta_y, 400)
  })
})

describe('needsGestureDispatch (DOM fallback routing)', () => {
  let needsGestureDispatch

  before(async () => {
    ;({ needsGestureDispatch } = await import(DOM))
  })

  test('a modifier-held click cannot go through element.click()', () => {
    assert.equal(needsGestureDispatch('click', ['ctrl']), true)
    assert.equal(needsGestureDispatch('click', []), false)
    assert.equal(needsGestureDispatch('click', undefined), false)
  })

  test('the six pointer gestures always route to the gesture primitive', () => {
    for (const action of ['drag', 'right_click', 'double_click', 'triple_click', 'hover_at', 'scroll_at']) {
      assert.equal(needsGestureDispatch(action, undefined), true, action)
    }
  })

  test('unrelated actions keep their own primitive', () => {
    assert.equal(needsGestureDispatch('type', ['ctrl']), false)
    assert.equal(needsGestureDispatch('select', undefined), false)
  })
})
