// @ts-nocheck
/**
 * @fileoverview three-locator-capture.test.js — The second and third locators recorded per step,
 * and the environment stamp applied to them (kaboom-x0li.2).
 *
 * A step described only by a CSS selector dies on the first re-render: the selector matches
 * nothing, the replay fails, and nothing in the artifact says where the element went. These cover
 * the two independent locators that survive that, and the pin stamp that says what the recording
 * depended on.
 */

import { test, describe, before } from 'node:test'
import assert from 'node:assert'

const REPRO = '../../../extension/lib/page/reproduction.js'
const TELEMETRY = '../../../extension/background/message-routing/telemetry-handler.js'

/** A DOM element stub with just enough surface for the locator functions. */
function fakeElement({ rect, role, ariaLabel, text, tag = 'BUTTON' } = {}) {
  const attributes = {}
  if (role) attributes.role = role
  if (ariaLabel) attributes['aria-label'] = ariaLabel
  return {
    tagName: tag,
    id: '',
    className: '',
    textContent: text ?? '',
    getAttribute: (name) => (name in attributes ? attributes[name] : null),
    getBoundingClientRect: () => rect
  }
}

const BOX = { left: 100, top: 200, width: 40, height: 20 }

describe('viewport locator', () => {
  let computeViewportLocator

  before(async () => {
    ;({ computeViewportLocator } = await import(REPRO))
    globalThis.window = {
      location: { href: 'https://app.example.com/checkout' },
      innerWidth: 1280,
      innerHeight: 720,
      devicePixelRatio: 2
    }
  })

  test('records the centre point, not a corner', () => {
    const locator = computeViewportLocator(fakeElement({ rect: BOX }))
    assert.equal(locator.x, 120)
    assert.equal(locator.y, 210)
  })

  test('carries the frame and viewport the point was measured in', () => {
    // A bare x/y is unreplayable: at a different window size or device scale it lands
    // somewhere else, and in the wrong frame it lands on the wrong document.
    const locator = computeViewportLocator(fakeElement({ rect: BOX }))
    assert.equal(locator.frame_url, 'https://app.example.com/checkout')
    assert.equal(locator.viewport_width, 1280)
    assert.equal(locator.viewport_height, 720)
    assert.equal(locator.device_pixel_ratio, 2)
  })

  test('drops a zero-area box rather than recording a point nothing occupies', () => {
    const collapsed = computeViewportLocator(fakeElement({ rect: { left: 5, top: 5, width: 0, height: 12 } }))
    assert.equal(collapsed, undefined)
  })

  test('drops a non-finite box rather than deriving a centre from NaN', () => {
    const broken = computeViewportLocator(fakeElement({ rect: { left: NaN, top: 5, width: 10, height: 10 } }))
    assert.equal(broken, undefined)
  })

  test('a missing element yields no locator at all', () => {
    assert.equal(computeViewportLocator(null), undefined)
  })
})

describe('accessibility locator', () => {
  let computeAXLocator, computeSelectors

  before(async () => {
    ;({ computeAXLocator, computeSelectors } = await import(REPRO))
  })

  test('records role and accessible name — the pair that survives a re-render', () => {
    const locator = computeAXLocator(fakeElement({ rect: BOX, text: 'Place order' }))
    assert.equal(locator.role, 'button')
    assert.equal(locator.name, 'Place order')
  })

  test('prefers aria-label over visible text, as the accessibility tree does', () => {
    const locator = computeAXLocator(fakeElement({ rect: BOX, ariaLabel: 'Confirm purchase', text: 'Place order' }))
    assert.equal(locator.name, 'Confirm purchase')
  })

  test('carries no ref: a CDP backend node id is stale outside its own snapshot', () => {
    const locator = computeAXLocator(fakeElement({ rect: BOX, text: 'Place order' }))
    assert.equal(locator.ref, undefined)
  })

  test('agrees with the role selector strategy about what the element is called', () => {
    // Two resolvers would let the emitted artifact contradict itself about one element.
    const element = fakeElement({ rect: BOX, ariaLabel: 'Confirm purchase', text: 'Place order' })
    const ax = computeAXLocator(element)
    const selectors = computeSelectors(element)
    assert.equal(selectors.role.role, ax.role)
    assert.equal(selectors.role.name, ax.name)
  })

  test('an element with neither role nor name yields no locator', () => {
    assert.equal(computeAXLocator(fakeElement({ rect: BOX, tag: 'SPAN' })), undefined)
  })
})

describe('environment stamp on recorded actions', () => {
  let createTelemetryMessageHandler

  before(async () => {
    ;({ createTelemetryMessageHandler } = await import(TELEMETRY))
  })

  function handlerWith(pin) {
    const recorded = []
    const handler = createTelemetryMessageHandler({
      addLog: () => {},
      addWebSocket: () => {},
      addEnhancedAction: (action) => recorded.push(action),
      addNetworkBody: () => {},
      addPerformance: () => {},
      handleLog: async () => {},
      isNetworkBodyCaptureDisabled: () => false,
      debugLog: () => {},
      addDiagnostic: () => {},
      environmentPinFor: () => pin
    })
    return { handler, recorded }
  }

  test('an unpinned tab records no environment block at all', () => {
    // That absence is what lets the artifact state "Environment not pinned" as a fact rather
    // than as a gap in reporting.
    const { handler, recorded } = handlerWith(undefined)
    handler.handle({ type: 'enhanced_action', payload: { type: 'click', timestamp: 1 }, tabId: 3 }, {})
    assert.equal(recorded.length, 1)
    assert.equal(recorded[0].environment, undefined)
  })

  test('a pinned tab stamps what was pinned onto every action it records', () => {
    const pin = { random_seed: 'run-1', clock: { timezone_id: 'UTC' } }
    const { handler, recorded } = handlerWith(pin)
    handler.handle({ type: 'enhanced_action', payload: { type: 'click', timestamp: 1 }, tabId: 3 }, {})
    assert.deepEqual(recorded[0].environment, pin)
    assert.equal(recorded[0].type, 'click', 'the original action must survive the stamp')
  })
})
