// @ts-nocheck
/**
 * @fileoverview safe-global-patch.test.js — Patching globals must survive pages
 * that make them read-only.
 *
 * Regression: `window.fetch = wrapped` sat unguarded. Sites that define fetch as
 * non-writable (anti-tampering shims, frozen globals) made it throw
 * "Cannot assign to read only property 'fetch' of object '#<Window>'", and the
 * uncaught error aborted every patch after it.
 */

import { describe, test } from 'node:test'
import assert from 'node:assert'
import { safeAssignGlobal } from '../../extension/lib/page/safe-global-patch.js'

describe('safeAssignGlobal', () => {
  test('assigns a normal writable property', () => {
    const target = { fetch: () => 'original' }
    const next = () => 'patched'
    assert.strictEqual(safeAssignGlobal(target, 'fetch', next), true)
    assert.strictEqual(target.fetch, next)
  })

  test('recovers via defineProperty when the property is non-writable', () => {
    const target = {}
    Object.defineProperty(target, 'fetch', { value: () => 'original', writable: false, configurable: true })
    const next = () => 'patched'

    assert.strictEqual(safeAssignGlobal(target, 'fetch', next), true,
      'a non-writable but configurable property can still be redefined')
    assert.strictEqual(target.fetch, next)
  })

  test('reports failure instead of throwing when the property is locked down', () => {
    const target = {}
    const original = () => 'original'
    Object.defineProperty(target, 'fetch', { value: original, writable: false, configurable: false })

    // The whole point: no throw, and the caller learns capture is unavailable.
    assert.strictEqual(safeAssignGlobal(target, 'fetch', () => 'patched'), false)
    assert.strictEqual(target.fetch, original, "the page's own value is left intact")
  })

  test('reports failure on a frozen target without throwing', () => {
    const target = Object.freeze({ fetch: () => 'original' })
    assert.strictEqual(safeAssignGlobal(target, 'fetch', () => 'patched'), false)
  })
})
