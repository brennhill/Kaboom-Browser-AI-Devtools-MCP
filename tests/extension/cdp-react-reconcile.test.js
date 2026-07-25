// @ts-nocheck
/**
 * @fileoverview cdp-react-reconcile.test.js — Regression tests for issue #599.
 *
 * CDP-based `type`/`click` don't reliably drive controlled-input frameworks (React):
 *   1. `type` sets the DOM value but leaves React's value tracker stale, so onChange
 *      never fires and React resets the field. The CDP path must reconcile the value
 *      through the native setter + dispatch input/change so onChange fires.
 *   2. Users need an escape hatch (`dispatch: "dom"`) that skips CDP escalation and
 *      routes through the DOM-primitives path (native setter + real element.click()).
 *
 * These cover the two pure, deterministic pieces of the fix. The in-page reconciliation
 * expression's runtime effect on real React is validated by manual/UAT browser testing.
 */

import { test, describe, beforeEach } from 'node:test'
import assert from 'node:assert'

describe('buildReactValueReconcileExpression (#599 type)', () => {
  let buildReactValueReconcileExpression

  beforeEach(async () => {
    ;({ buildReactValueReconcileExpression } = await import('../../extension/background/dom/cdp/cdp-dispatch.js'))
  })

  test('JSON-encodes the selector to survive quotes/injection', () => {
    const expr = buildReactValueReconcileExpression('input[name="q"]')
    assert.ok(expr.includes(JSON.stringify('input[name="q"]')))
  })

  test('reads the value back through the native prototype setter', () => {
    const expr = buildReactValueReconcileExpression('#field')
    // Must use the prototype descriptor's setter, not the (React-overridden) instance setter.
    assert.match(expr, /getOwnPropertyDescriptor\(/)
    assert.match(expr, /HTMLInputElement\.prototype/)
    assert.match(expr, /HTMLTextAreaElement\.prototype/)
    assert.match(expr, /\.set\b/)
  })

  test('dispatches bubbling input and change events', () => {
    const expr = buildReactValueReconcileExpression('#field')
    assert.match(expr, /dispatchEvent\(/)
    assert.match(expr, /['"]input['"]/)
    assert.match(expr, /['"]change['"]/)
    assert.match(expr, /bubbles:\s*true/)
  })

  test('only reconciles when a React value tracker is present (no double-fire on plain inputs)', () => {
    const expr = buildReactValueReconcileExpression('#field')
    assert.match(expr, /_valueTracker|__reactFiber\$|__reactProps\$/)
  })

  test('guards against non-input elements', () => {
    const expr = buildReactValueReconcileExpression('#field')
    assert.match(expr, /HTMLInputElement/)
    assert.match(expr, /HTMLTextAreaElement/)
  })
})

describe('shouldEscalateToCDP (#599 dispatch escape hatch)', () => {
  let shouldEscalateToCDP

  beforeEach(async () => {
    ;({ shouldEscalateToCDP } = await import('../../extension/background/dom/cdp/cdp-dispatch.js'))
  })

  test('escalates click/type/key_press by default', () => {
    for (const action of ['click', 'type', 'key_press']) {
      assert.strictEqual(shouldEscalateToCDP(action, {}), true, `${action} should escalate`)
    }
  })

  test('does not escalate non-escalatable actions', () => {
    assert.strictEqual(shouldEscalateToCDP('scroll', {}), false)
    assert.strictEqual(shouldEscalateToCDP('get_text', {}), false)
  })

  test('dispatch:"dom" disables CDP escalation (routes to DOM primitives)', () => {
    assert.strictEqual(shouldEscalateToCDP('type', { dispatch: 'dom' }), false)
    assert.strictEqual(shouldEscalateToCDP('click', { dispatch: 'dom' }), false)
  })

  test('dispatch:"auto" (explicit default) still escalates', () => {
    assert.strictEqual(shouldEscalateToCDP('type', { dispatch: 'auto' }), true)
  })

  test('frame-scoped and nth-scoped actions never escalate to CDP', () => {
    assert.strictEqual(shouldEscalateToCDP('click', { frame: 'iframe#a' }), false)
    assert.strictEqual(shouldEscalateToCDP('click', { nth: 2 }), false)
  })
})
