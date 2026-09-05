// @ts-nocheck
/**
 * @fileoverview background-tab-capture.test.js — Capturing a tab the user is not looking at.
 *
 * Every screenshot used to steal the foreground: `chrome.tabs.captureVisibleTab` photographs
 * whatever is visible in the window, so the old path activated the target tab, captured, then
 * put the user's tab back. A person working alongside the agent had their window yanked away
 * once per capture, and anything they were typing lost focus mid-word.
 *
 * These tests pin the replacement: `Page.captureScreenshot` over the tab's persistent CDP
 * lease, clipped to the visual viewport at the page's device pixel ratio, with no tab
 * activation at all.
 */

import { describe, test, mock, beforeEach, after } from 'node:test'
import assert from 'node:assert'

const IMAGE_B64 = 'Q0RQSU1BR0U='

let sends
let attachCount
let detachCount
let captureShouldFail

function installChrome() {
  sends = []
  attachCount = 0
  detachCount = 0
  captureShouldFail = false
  const attached = new Set()

  globalThis.chrome = {
    tabs: {
      // Tab 99 is the one the human is looking at. Nothing may move it.
      query: mock.fn(async () => [{ id: 99, windowId: 11 }]),
      get: mock.fn(async (tabId) => ({ id: tabId, windowId: 11, url: 'https://example.test/' })),
      update: mock.fn(async () => ({})),
      captureVisibleTab: mock.fn(async () => 'data:image/jpeg;base64,VklTSUJMRQ==')
    },
    scripting: {
      executeScript: mock.fn(async () => [])
    },
    debugger: {
      attach: async (target) => {
        attachCount += 1
        attached.add(target.tabId)
      },
      detach: async (target) => {
        detachCount += 1
        attached.delete(target.tabId)
      },
      // Chrome rejects every command on an unattached target; the adoption probe depends on it.
      sendCommand: async (target, method, params) => {
        sends.push({ tabId: target.tabId, method, params })
        if (!attached.has(target.tabId)) {
          throw new Error(`Debugger is not attached to the tab with id: ${target.tabId}`)
        }
        if (method === 'Page.getLayoutMetrics') {
          return {
            cssVisualViewport: { pageX: 0, pageY: 240, clientWidth: 1280, clientHeight: 720 }
          }
        }
        if (method === 'Runtime.evaluate') return { result: { value: 2 } }
        if (method === 'Page.captureScreenshot') {
          if (captureShouldFail) throw new Error('Unable to capture screenshot')
          return { data: IMAGE_B64 }
        }
        return {}
      },
      onDetach: { addListener: () => {} }
    }
  }
}

installChrome()

const { captureTabImage } = await import('../../../../extension/background/ui/tracked-tab-state.js')
const { cdpSessions } = await import('../../../../extension/background/dom/cdp/cdp-session.js')

// A warm session holds a 30s idle-detach timer. Leaving it armed would keep the test process
// alive for that long after the last assertion; aborting is the same path the Stop button uses.
after(() => {
  for (const tabId of [7, 21, 22, 23]) cdpSessions()?.abort(tabId, 'test_teardown')
})

const captureCalls = () => sends.filter((s) => s.method === 'Page.captureScreenshot')
const overlayArgs = () => globalThis.chrome.scripting.executeScript.mock.calls.map((c) => c.arguments[0].args[0])

describe('captureTabImage — background capture over CDP', () => {
  beforeEach(() => {
    captureShouldFail = false
    globalThis.chrome.tabs.update.mock.resetCalls()
    globalThis.chrome.tabs.captureVisibleTab.mock.resetCalls()
    globalThis.chrome.scripting.executeScript.mock.resetCalls()
    sends.length = 0
  })

  test('never activates the tab it is capturing', async () => {
    await captureTabImage(7, 11, { format: 'jpeg', quality: 80 })
    assert.strictEqual(
      globalThis.chrome.tabs.update.mock.calls.length,
      0,
      'capturing must not steal the foreground from the person using the browser'
    )
    assert.strictEqual(
      globalThis.chrome.tabs.captureVisibleTab.mock.calls.length,
      0,
      'captureVisibleTab is the no-debugger fallback only'
    )
  })

  test('a tab the user is already looking at is captured without attaching the debugger', async () => {
    // Tab 99 is the active tab in window 11. captureVisibleTab already has exactly these
    // pixels, so taking a CDP lease buys nothing and costs a lot: Chrome raises the
    // "Kaboom is debugging this browser" infobar over the user's own browsing for the
    // lease's idle grace. screenshot_on_error fires on any page error, so that banner
    // would appear unprompted, repeatedly, while someone is just using their browser.
    const before = attachCount

    const capture = await captureTabImage(99, 11, { format: 'jpeg', quality: 80 })

    assert.strictEqual(capture.data_url, 'data:image/jpeg;base64,VklTSUJMRQ==')
    assert.strictEqual(capture.source, 'visible_tab')
    assert.strictEqual(
      capture.covered_css_region,
      null,
      'captureVisibleTab reports no bounds of its own; the frame falls back to the page'
    )
    assert.strictEqual(attachCount, before, 'no debugger attach for a tab that is already visible')
    assert.strictEqual(
      globalThis.chrome.tabs.update.mock.calls.length,
      0,
      'and no activation either — it is already the active tab'
    )
    assert.strictEqual(captureCalls().length, 0, 'Page.captureScreenshot must not be reached')
  })

  test('returns the CDP image as a data URL in the requested format', async () => {
    const png = await captureTabImage(7, 11, { format: 'png' })
    assert.strictEqual(png.data_url, `data:image/png;base64,${IMAGE_B64}`)
    const jpeg = await captureTabImage(7, 11, { format: 'jpeg', quality: 80 })
    assert.strictEqual(jpeg.data_url, `data:image/jpeg;base64,${IMAGE_B64}`)
    assert.strictEqual(jpeg.source, 'cdp')
  })

  test('the CDP capture reports the clip it photographed, not the page\'s idea of its viewport', async () => {
    // The coordinate frame's scale is the CSS extent divided by the image size, so the
    // extent has to be the one that was actually captured. cssVisualViewport excludes a
    // classic scrollbar; reporting innerWidth instead would misplace a click at the right
    // edge of the image by the scrollbar width.
    const capture = await captureTabImage(7, 11, { format: 'jpeg', quality: 80 })
    const clip = captureCalls()[0]?.params?.clip
    assert.ok(clip, 'the capture must send a clip, or there is no region to report')
    assert.deepStrictEqual(capture.covered_css_region, {
      x: 0,
      y: 0,
      width: clip.width,
      height: clip.height
    })
  })

  test('clips to the visual viewport at the page device pixel ratio', async () => {
    // Without the clip the capture would be the whole document, and without the dpr scale a
    // HiDPI screenshot would come back at half the resolution captureVisibleTab produced —
    // the element crop in observe(screenshot, selector) reads that scale to place its crop.
    await captureTabImage(7, 11, { format: 'jpeg', quality: 80 })
    const params = captureCalls()[0].params
    assert.deepStrictEqual(params.clip, { x: 0, y: 240, width: 1280, height: 720, scale: 2 })
    assert.strictEqual(params.format, 'jpeg')
    assert.strictEqual(params.quality, 80)
  })

  test('omits quality for png, which Chrome rejects it on', async () => {
    await captureTabImage(7, 11, { format: 'png', quality: 80 })
    assert.strictEqual(captureCalls()[0].params.quality, undefined)
  })

  test('strips Kaboom overlays before the capture and restores them after', async () => {
    // Without this the agent screenshots its own supervision overlay and reports Kaboom's UI
    // back as the page under test.
    await captureTabImage(7, 11, { format: 'jpeg', quality: 80 })
    assert.deepStrictEqual(overlayArgs(), [false, true])
  })

  test('restores overlays exactly once even when the capture falls back', async () => {
    captureShouldFail = true
    await captureTabImage(7, 11, { format: 'jpeg', quality: 80 })
    assert.deepStrictEqual(overlayArgs(), [false, true], 'a failed capture must not leave the page blanked')
  })

  test('falls back to the visible-tab path when the CDP capture fails', async () => {
    captureShouldFail = true
    const capture = await captureTabImage(7, 11, { format: 'jpeg', quality: 80 })
    assert.strictEqual(capture.data_url, 'data:image/jpeg;base64,VklTSUJMRQ==')
    assert.strictEqual(capture.source, 'visible_tab', 'the fallback must not claim the CDP clip it never took')
    assert.strictEqual(globalThis.chrome.tabs.captureVisibleTab.mock.calls.length, 1)
    // The fallback API can only photograph the visible tab, so it has no choice but to
    // borrow the foreground — and it must hand it straight back.
    assert.deepStrictEqual(
      globalThis.chrome.tabs.update.mock.calls.map((c) => c.arguments),
      [
        [7, { active: true }],
        [99, { active: true }]
      ]
    )
  })

  test('reuses one debugger attachment across a burst of captures', async () => {
    const before = attachCount
    await captureTabImage(21, 11, { format: 'jpeg', quality: 80 })
    await captureTabImage(21, 11, { format: 'jpeg', quality: 80 })
    await captureTabImage(21, 11, { format: 'jpeg', quality: 80 })
    assert.strictEqual(attachCount - before, 1, 'three captures on one tab must share one attachment')
    assert.strictEqual(detachCount, 0, 'a capture burst must not tear the session down between shots')
  })

  test('shouts when the capture actually failed on an attached tab', async () => {
    // Rule 25: the fallback exists for "no lease available", not for "capture broke". A real
    // failure that took the user's foreground and returned a different image must be loud
    // enough to see with extension debug logging off.
    const warnings = []
    const realWarn = console.warn
    console.warn = (...args) => warnings.push(args.join(' '))
    try {
      captureShouldFail = true
      await captureTabImage(7, 11, { format: 'jpeg', quality: 80 })
    } finally {
      console.warn = realWarn
    }
    assert.strictEqual(warnings.length, 1, 'a genuine capture failure must be reported once')
    assert.match(warnings[0], /CDP capture failed, activating the tab instead/)
  })

  test('stays quiet when another owner holds the tab exclusively', async () => {
    // A performance trace owning the session is an expected, recoverable state. Reporting it
    // the same way as a broken capture teaches the reader to ignore both.
    const warnings = []
    const realWarn = console.warn
    console.warn = (...args) => warnings.push(args.join(' '))
    let exclusive
    try {
      exclusive = await cdpSessions().acquire(23, { exclusive: true })
      await captureTabImage(23, 11, { format: 'jpeg', quality: 80 })
    } finally {
      console.warn = realWarn
      exclusive?.release()
    }
    assert.strictEqual(warnings.length, 0, 'an expected busy session is not a defect to shout about')
    assert.strictEqual(
      globalThis.chrome.tabs.captureVisibleTab.mock.calls.length,
      1,
      'it still produces an image via the foreground fallback'
    )
  })

  test('makes the captured tab behave as focused', async () => {
    // A capture takes the same lease the input path uses, so the page it photographs is the
    // page the agent is driving — focus rings and focus-gated widgets included.
    await captureTabImage(22, 11, { format: 'jpeg', quality: 80 })
    const focus = sends.filter((s) => s.method === 'Emulation.setFocusEmulationEnabled')
    assert.deepStrictEqual(
      focus.map((f) => f.params.enabled),
      [true]
    )
  })
})
