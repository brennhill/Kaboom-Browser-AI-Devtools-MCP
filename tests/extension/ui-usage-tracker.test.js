// @ts-nocheck
/**
 * @fileoverview ui-usage-tracker.test.js -- Tests for UI-originated feature usage tracking.
 * Covers trackUIFeature, drainUIFeatures (swap-and-replace), restoreUIFeatures.
 */

import { test, describe, beforeEach } from 'node:test'
import assert from 'node:assert'

// The module uses no chrome APIs, so no mocks needed.
let mod

async function loadModule() {
  // Fresh import each test suite — but module state persists.
  // We drain before each test to reset state.
  if (!mod) {
    mod = await import('../../extension/background/ui/ui-usage-tracker.js')
  }
  return mod
}

describe('trackUIFeature', () => {
  beforeEach(async () => {
    const m = await loadModule()
    m.drainUIFeatures() // reset state
  })

  test('tracking a feature makes it appear in drain', async () => {
    const m = await loadModule()
    m.trackUIFeature('screenshot')
    const result = m.drainUIFeatures()
    assert.deepStrictEqual(result, { screenshot: true })
  })

  test('tracking multiple features returns all', async () => {
    const m = await loadModule()
    m.trackUIFeature('screenshot')
    m.trackUIFeature('video')
    m.trackUIFeature('annotations')
    const result = m.drainUIFeatures()
    assert.deepStrictEqual(result, {
      screenshot: true,
      video: true,
      annotations: true
    })
  })

  test('tracking same feature twice is idempotent', async () => {
    const m = await loadModule()
    m.trackUIFeature('screenshot')
    m.trackUIFeature('screenshot')
    const result = m.drainUIFeatures()
    assert.deepStrictEqual(result, { screenshot: true })
  })
})

describe('drainUIFeatures', () => {
  beforeEach(async () => {
    const m = await loadModule()
    m.drainUIFeatures() // reset state
  })

  test('returns undefined when empty', async () => {
    const m = await loadModule()
    assert.strictEqual(m.drainUIFeatures(), undefined)
  })

  test('clears state after drain', async () => {
    const m = await loadModule()
    m.trackUIFeature('video')
    const first = m.drainUIFeatures()
    assert.ok(first)
    const second = m.drainUIFeatures()
    assert.strictEqual(second, undefined)
  })

  test('features tracked during drain iteration are not lost', async () => {
    const m = await loadModule()
    m.trackUIFeature('screenshot')
    // Drain swaps the map — any trackUIFeature after drain goes to the new map
    const drained = m.drainUIFeatures()
    assert.deepStrictEqual(drained, { screenshot: true })

    // Track a new feature after drain
    m.trackUIFeature('video')
    const next = m.drainUIFeatures()
    assert.deepStrictEqual(next, { video: true })
  })
})

describe('restoreUIFeatures', () => {
  beforeEach(async () => {
    const m = await loadModule()
    m.drainUIFeatures() // reset state
  })

  test('restores features back into pending', async () => {
    const m = await loadModule()
    m.restoreUIFeatures({ screenshot: true, video: true })
    const result = m.drainUIFeatures()
    assert.deepStrictEqual(result, { screenshot: true, video: true })
  })

  test('skips false values', async () => {
    const m = await loadModule()
    m.restoreUIFeatures({ screenshot: true, video: false })
    const result = m.drainUIFeatures()
    assert.deepStrictEqual(result, { screenshot: true })
  })

  test('preserves features tracked between drain and restore', async () => {
    const m = await loadModule()
    m.trackUIFeature('screenshot')
    const drained = m.drainUIFeatures()

    // New feature tracked after drain, before restore
    m.trackUIFeature('annotations')

    // Restore the drained features (simulating failed sync)
    m.restoreUIFeatures(drained)

    const result = m.drainUIFeatures()
    assert.strictEqual(result.screenshot, true)
    assert.strictEqual(result.annotations, true)
  })

  test('restore with empty object is a no-op', async () => {
    const m = await loadModule()
    m.restoreUIFeatures({})
    assert.strictEqual(m.drainUIFeatures(), undefined)
  })
})
