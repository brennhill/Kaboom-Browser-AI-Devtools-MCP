// @ts-nocheck
/**
 * @fileoverview content-message-correlation.test.js — Tests for requestId-based
 * correlation in forwardInjectQuery and handleGetNetworkWaterfall, plus nonce
 * validation on response paths.
 *
 * Run: node --experimental-test-module-mocks --test tests/extension/content-message-correlation.test.js
 */

import { test, describe, mock, beforeEach } from 'node:test'
import assert from 'node:assert'
import { createMockWindow } from './helpers.js'

// =============================================================================
// TEST INFRASTRUCTURE
// =============================================================================

/** Captured window.addEventListener('message', ...) handlers */
let messageListeners = []
/** Captured postMessage calls */
let postedMessages = []
/** The mock window */
let mockWindow

/** Reset mock window and capture arrays */
function resetWindow() {
  messageListeners = []
  postedMessages = []

  mockWindow = createMockWindow({ href: 'http://localhost:3000/' })
  mockWindow.addEventListener = mock.fn((type, handler) => {
    if (type === 'message') messageListeners.push(handler)
  })
  mockWindow.removeEventListener = mock.fn((type, handler) => {
    if (type === 'message') {
      messageListeners = messageListeners.filter((h) => h !== handler)
    }
  })
  mockWindow.postMessage = mock.fn((data) => {
    postedMessages.push(data)
  })

  globalThis.window = mockWindow
}

/** Simulate a postMessage event from the page (inject.js response) */
function fireWindowMessage(data, { authenticate = true } = {}) {
  const nonce = postedMessages.findLast((message) => typeof message._nonce === 'string')?._nonce
  const event = {
    source: mockWindow,
    origin: mockWindow.location.origin,
    data: authenticate && data._nonce === undefined ? { ...data, _nonce: nonce } : data
  }
  // Copy listeners array to avoid mutation during iteration
  const listeners = [...messageListeners]
  for (const handler of listeners) {
    handler(event)
  }
}

async function importContentHandlers() {
  return import(`../../extension/content/message-handlers.js?t=${Date.now()}_${Math.random()}`)
}

async function importScriptInjection() {
  return import('../../extension/content/script-injection.js')
}

// =============================================================================
// Fix 1: requestId correlation — forwardInjectQuery
// =============================================================================

describe('forwardInjectQuery — requestId correlation', () => {
  let handleComputedStylesQuery

  beforeEach(async () => {
    mock.reset()
    resetWindow()

    const mod = await importContentHandlers()
    handleComputedStylesQuery = mod.handleComputedStylesQuery
  })

  test('single query resolves correctly', async () => {
    const result = await new Promise((resolve) => {
      handleComputedStylesQuery({ selector: 'div' }, resolve)

      // Find the requestId from the posted message
      const posted = postedMessages.find((m) => m.type === 'kaboom_computed_styles_query')
      assert.ok(posted, 'should have posted a query message')
      const reqId = posted.requestId

      // Fire response with matching requestId
      fireWindowMessage({
        type: 'kaboom_computed_styles_response',
        requestId: reqId,
        result: { elements: ['div1'], count: 1 }
      })
    })

    assert.deepStrictEqual(result, { elements: ['div1'], count: 1 })
  })

  test('two concurrent queries for same type each resolve with own result', async () => {
    const results = []

    // Launch two concurrent queries
    const p1 = new Promise((resolve) => {
      handleComputedStylesQuery({ selector: '.first' }, resolve)
    })
    const p2 = new Promise((resolve) => {
      handleComputedStylesQuery({ selector: '.second' }, resolve)
    })

    // Find posted messages — there should be two with different requestIds
    const posted = postedMessages.filter((m) => m.type === 'kaboom_computed_styles_query')
    assert.strictEqual(posted.length, 2, 'should have posted two query messages')
    assert.notStrictEqual(posted[0].requestId, posted[1].requestId, 'requestIds should differ')

    // Fire responses in reverse order
    fireWindowMessage({
      type: 'kaboom_computed_styles_response',
      requestId: posted[1].requestId,
      result: { elements: ['second-result'], count: 1 }
    })
    fireWindowMessage({
      type: 'kaboom_computed_styles_response',
      requestId: posted[0].requestId,
      result: { elements: ['first-result'], count: 1 }
    })

    const [r1, r2] = await Promise.all([p1, p2])
    results.push(r1, r2)

    // Each should get its own result, not the other's
    assert.deepStrictEqual(r1, { elements: ['first-result'], count: 1 })
    assert.deepStrictEqual(r2, { elements: ['second-result'], count: 1 })
  })

  test('mismatched requestId is ignored — response for wrong query does not resolve listener', async () => {
    let resolved = false

    const p = new Promise((resolve) => {
      handleComputedStylesQuery({ selector: 'span' }, (result) => {
        resolved = true
        resolve(result)
      })
    })

    const posted = postedMessages.find((m) => m.type === 'kaboom_computed_styles_query')
    const correctId = posted.requestId

    // Fire response with wrong requestId
    fireWindowMessage({
      type: 'kaboom_computed_styles_response',
      requestId: correctId + 999,
      result: { elements: ['wrong'], count: 1 }
    })

    // Small delay to ensure handler had time to run
    await new Promise((r) => setTimeout(r, 50))
    assert.strictEqual(resolved, false, 'should not resolve with mismatched requestId')

    // Now fire correct one
    fireWindowMessage({
      type: 'kaboom_computed_styles_response',
      requestId: correctId,
      result: { elements: ['correct'], count: 1 }
    })

    const result = await p
    assert.deepStrictEqual(result, { elements: ['correct'], count: 1 })
  })
})

// =============================================================================
// Fix 1: requestId correlation — handleGetNetworkWaterfall
// =============================================================================

describe('handleGetNetworkWaterfall — requestId correlation', () => {
  let handleGetNetworkWaterfall

  beforeEach(async () => {
    mock.reset()
    resetWindow()

    const mod = await importContentHandlers()
    handleGetNetworkWaterfall = mod.handleGetNetworkWaterfall
  })

  test('single waterfall query resolves correctly', async () => {
    const result = await new Promise((resolve) => {
      handleGetNetworkWaterfall(resolve)

      const posted = postedMessages.find((m) => m.type === 'kaboom_get_waterfall')
      assert.ok(posted, 'should have posted a waterfall query')

      fireWindowMessage({
        type: 'kaboom_waterfall_response',
        requestId: posted.requestId,
        entries: [{ url: '/api/data', status: 200 }]
      })
    })

    assert.deepStrictEqual(result, { entries: [{ url: '/api/data', status: 200 }] })
  })

  test('two concurrent waterfall queries each get own entries', async () => {
    const p1 = new Promise((resolve) => {
      handleGetNetworkWaterfall(resolve)
    })
    const p2 = new Promise((resolve) => {
      handleGetNetworkWaterfall(resolve)
    })

    const posted = postedMessages.filter((m) => m.type === 'kaboom_get_waterfall')
    assert.strictEqual(posted.length, 2)
    assert.notStrictEqual(posted[0].requestId, posted[1].requestId)

    // Fire in reverse order
    fireWindowMessage({
      type: 'kaboom_waterfall_response',
      requestId: posted[1].requestId,
      entries: [{ url: '/second' }]
    })
    fireWindowMessage({
      type: 'kaboom_waterfall_response',
      requestId: posted[0].requestId,
      entries: [{ url: '/first' }]
    })

    const [r1, r2] = await Promise.all([p1, p2])
    assert.deepStrictEqual(r1, { entries: [{ url: '/first' }] })
    assert.deepStrictEqual(r2, { entries: [{ url: '/second' }] })
  })

  test('mismatched requestId is ignored for waterfall', async () => {
    let resolved = false

    const p = new Promise((resolve) => {
      handleGetNetworkWaterfall((result) => {
        resolved = true
        resolve(result)
      })
    })

    const posted = postedMessages.find((m) => m.type === 'kaboom_get_waterfall')

    // Wrong requestId
    fireWindowMessage({
      type: 'kaboom_waterfall_response',
      requestId: posted.requestId + 999,
      entries: [{ url: '/wrong' }]
    })

    await new Promise((r) => setTimeout(r, 50))
    assert.strictEqual(resolved, false)

    // Correct requestId
    fireWindowMessage({
      type: 'kaboom_waterfall_response',
      requestId: posted.requestId,
      entries: [{ url: '/correct' }]
    })

    const result = await p
    assert.deepStrictEqual(result, { entries: [{ url: '/correct' }] })
  })
})

// =============================================================================
// Fix 2: Nonce validation on response paths
// =============================================================================

describe('forwardInjectQuery — nonce validation', () => {
  let handleComputedStylesQuery
  let getPageNonce

  beforeEach(async () => {
    mock.reset()
    resetWindow()

    const mod = await importContentHandlers()
    handleComputedStylesQuery = mod.handleComputedStylesQuery
    ;({ getPageNonce } = await importScriptInjection())
  })

  test('response with correct nonce is accepted', async () => {
    const result = await new Promise((resolve) => {
      handleComputedStylesQuery({ selector: 'div' }, resolve)

      const posted = postedMessages.find((m) => m.type === 'kaboom_computed_styles_query')

      fireWindowMessage({
        type: 'kaboom_computed_styles_response',
        requestId: posted.requestId,
        _nonce: getPageNonce(),
        result: { elements: ['ok'], count: 1 }
      })
    })

    assert.deepStrictEqual(result, { elements: ['ok'], count: 1 })
  })

  test('response with wrong nonce is ignored', async () => {
    let resolved = false

    const p = new Promise((resolve) => {
      handleComputedStylesQuery({ selector: 'div' }, (result) => {
        resolved = true
        resolve(result)
      })
    })

    const posted = postedMessages.find((m) => m.type === 'kaboom_computed_styles_query')

    // Fire response with wrong nonce
    fireWindowMessage({
      type: 'kaboom_computed_styles_response',
      requestId: posted.requestId,
      _nonce: 'wrong-nonce',
      result: { elements: ['spoofed'], count: 1 }
    })

    await new Promise((r) => setTimeout(r, 50))
    assert.strictEqual(resolved, false, 'should not resolve with wrong nonce')

    // Now fire with correct nonce
    fireWindowMessage({
      type: 'kaboom_computed_styles_response',
      requestId: posted.requestId,
      _nonce: getPageNonce(),
      result: { elements: ['legit'], count: 1 }
    })

    const result = await p
    assert.deepStrictEqual(result, { elements: ['legit'], count: 1 })
  })

  test('response with no nonce is ignored', async () => {
    let resolved = false
    const pending = new Promise((resolve) => {
      handleComputedStylesQuery({ selector: 'div' }, (result) => {
        resolved = true
        resolve(result)
      })
    })

    const posted = postedMessages.find((m) => m.type === 'kaboom_computed_styles_query')
    fireWindowMessage(
      {
        type: 'kaboom_computed_styles_response',
        requestId: posted.requestId,
        result: { elements: ['spoofed'], count: 1 }
      },
      { authenticate: false }
    )

    await new Promise((resolve) => setTimeout(resolve, 50))
    assert.strictEqual(resolved, false, 'should not resolve without the page nonce')

    fireWindowMessage({
      type: 'kaboom_computed_styles_response',
      requestId: posted.requestId,
      _nonce: getPageNonce(),
      result: { elements: ['legit'], count: 1 }
    })
    assert.deepStrictEqual(await pending, { elements: ['legit'], count: 1 })
  })
})
