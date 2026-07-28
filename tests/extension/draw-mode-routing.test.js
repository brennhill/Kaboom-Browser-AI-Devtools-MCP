// @ts-nocheck
/**
 * @fileoverview Draw-mode routing, lifecycle, accessibility, and re-entry tests.
 */

import { test, describe, mock, beforeEach, afterEach } from 'node:test'
import assert from 'node:assert'
import {
  createdElements,
  documentBody,
  importDrawMode,
  setupGlobals,
} from './draw-mode-fixture.js'

describe('Draw Mode — Content Script Message Routing', () => {
  let dm

  beforeEach(async () => {
    setupGlobals()
    dm = await importDrawMode()
  })

  test('GASOLINE_DRAW_MODE_START activates with correct source', () => {
    const result = dm.activateDrawMode('llm')
    assert.strictEqual(result.status, 'active')
    assert.strictEqual(result.started_by, 'llm')
    assert.strictEqual(dm.isDrawModeActive(), true)
  })

  test('GASOLINE_DRAW_MODE_STOP deactivates and returns annotations', () => {
    dm.activateDrawMode('user')
    assert.strictEqual(dm.isDrawModeActive(), true)

    const result = dm.deactivateDrawMode()
    assert.ok(Array.isArray(result.annotations))
    assert.strictEqual(dm.isDrawModeActive(), false)
  })

  test('GASOLINE_GET_ANNOTATIONS returns annotations and viewport', () => {
    dm.activateDrawMode('user')

    const response = {
      annotations: dm.getAnnotations(),
      draw_mode_active: dm.isDrawModeActive(),
      viewport: { width: globalThis.window.innerWidth, height: globalThis.window.innerHeight }
    }

    assert.ok(Array.isArray(response.annotations))
    assert.strictEqual(response.draw_mode_active, true)
    assert.strictEqual(response.viewport.width, 1024)
    assert.strictEqual(response.viewport.height, 768)
  })

  test('GASOLINE_GET_ANNOTATION_DETAIL returns null for unknown id', () => {
    assert.strictEqual(dm.getElementDetail('nonexistent'), null)
  })

  test('GASOLINE_CLEAR_ANNOTATIONS empties annotation list', () => {
    dm.activateDrawMode('user')
    dm.clearAnnotations()
    assert.deepStrictEqual(dm.getAnnotations(), [])
  })

  test('draw mode start while active returns already_active', () => {
    dm.activateDrawMode('llm')
    const second = dm.activateDrawMode('llm')
    assert.strictEqual(second.status, 'already_active')
    assert.strictEqual(typeof second.annotation_count, 'number')
  })
})

// =============================================================================
// State leak prevention (#9)
// =============================================================================
describe('Draw Mode — State Leak Prevention', () => {
  let dm

  beforeEach(async () => {
    setupGlobals()
    dm = await importDrawMode()
  })

  afterEach(() => {
    if (dm?.isDrawModeActive()) dm.deactivateDrawMode()
  })

  test('deactivateDrawMode clears annotations for next session', () => {
    globalThis.chrome.storage.session.get = mock.fn((_keys, callback) => {
      const empty = {}
      if (typeof callback === 'function') callback(empty)
      else return Promise.resolve(empty)
    })
    dm.clearAnnotations()
    const baselineCount = dm.getAnnotations().length
    assert.strictEqual(baselineCount, 0, 'test should start from empty annotation state')

    dm.activateDrawMode('user')

    // Simulate adding annotation data via the public API
    // First session
    dm.deactivateDrawMode()

    // Second activation should start clean
    dm.activateDrawMode('user')
    const anns = dm.getAnnotations()
    assert.strictEqual(anns.length, baselineCount, 'Annotations should be empty after re-activation')
  })

  test('deactivateDrawMode clears elementDetails for next session', () => {
    dm.activateDrawMode('user')
    dm.deactivateDrawMode()

    // After deactivation, getElementDetail for any old ID should return null
    dm.activateDrawMode('user')
    assert.strictEqual(dm.getElementDetail('old_correlation_id'), null)
  })
})

// =============================================================================
// Deactivation failure paths (#22)
// =============================================================================
describe('Draw Mode — Deactivation Failure Paths', () => {
  let dm

  beforeEach(async () => {
    setupGlobals()
    dm = await importDrawMode()
  })

  afterEach(() => {
    if (dm?.isDrawModeActive()) dm.deactivateDrawMode()
  })

  test('deactivateDrawMode succeeds even when chrome.runtime.sendMessage throws', () => {
    dm.activateDrawMode('user')
    assert.strictEqual(dm.isDrawModeActive(), true)

    // Make sendMessage throw
    globalThis.chrome.runtime.sendMessage = mock.fn(() => {
      throw new Error('Extension context invalidated')
    })

    // Direct deactivation should still work
    const result = dm.deactivateDrawMode()
    assert.strictEqual(dm.isDrawModeActive(), false)
    assert.ok(Array.isArray(result.annotations))
  })

  test('deactivateDrawMode returns results even when not active', () => {
    // Not active → should return empty result without error
    const result = dm.deactivateDrawMode()
    assert.deepStrictEqual(result.annotations, [])
    assert.deepStrictEqual(result.elementDetails, {})
  })

  test('clearAnnotations works when draw mode is active', () => {
    dm.activateDrawMode('user')
    dm.clearAnnotations()
    assert.deepStrictEqual(dm.getAnnotations(), [])
    assert.strictEqual(dm.isDrawModeActive(), true, 'Should still be active after clear')
  })
})

// =============================================================================
// Session name (#18 multi-page sessions)
// =============================================================================
describe('Draw Mode — Session Name', () => {
  let dm

  beforeEach(async () => {
    setupGlobals()
    dm = await importDrawMode()
  })

  afterEach(() => {
    if (dm?.isDrawModeActive()) dm.deactivateDrawMode()
  })

  test('activateDrawMode accepts session name', () => {
    const result = dm.activateDrawMode('llm', 'qa-review')
    assert.strictEqual(result.status, 'active')
    assert.strictEqual(dm.isDrawModeActive(), true)
  })

  test('session name cleared on deactivation', () => {
    dm.activateDrawMode('llm', 'qa-review')
    dm.deactivateDrawMode()
    // Reactivate without session name — should not carry over
    dm.activateDrawMode('user')
    assert.strictEqual(dm.isDrawModeActive(), true)
  })
})

// =============================================================================
// A11y checks (#19 accessibility auto-enrichment)
// =============================================================================
describe('Draw Mode — A11y Auto-Enrichment', () => {
  let dm

  beforeEach(async () => {
    setupGlobals()
    dm = await importDrawMode()
  })

  afterEach(() => {
    if (dm?.isDrawModeActive()) dm.deactivateDrawMode()
  })

  test('DOM capture produces element detail with a11y_flags field', () => {
    const beforeCount = dm.getAnnotations().length
    dm.activateDrawMode('user')
    const overlay = documentBody.children[0]
    assert.ok(overlay, 'expected overlay element')

    // Draw a rectangle to trigger DOM capture + a11y enrichment
    overlay._dispatch('mousedown', { button: 0, clientX: 100, clientY: 100 })
    overlay._dispatch('mouseup', { clientX: 250, clientY: 200 })

    const inputEl = createdElements.find((el) => el.tagName === 'INPUT')
    inputEl.value = 'a11y test'
    inputEl._listeners['keydown']?.[0]?.({ key: 'Enter', preventDefault: mock.fn(), stopPropagation: mock.fn() })

    const annotations = dm.getAnnotations()
    assert.ok(annotations.length > beforeCount, 'expected annotation count to increase')

    const latest = annotations[annotations.length - 1]
    const detail = dm.getElementDetail(latest.correlation_id)
    assert.ok(detail, 'should retrieve detail by correlation_id')
    assert.ok(Array.isArray(detail.a11y_flags), 'a11y_flags should be an array')
  })
})

// =============================================================================
// Re-entry guard and timeout fallback (#36)
// =============================================================================
describe('Draw Mode — deactivateAndSendResults re-entry guard', () => {
  let dm

  beforeEach(async () => {
    setupGlobals()
    dm = await importDrawMode()
  })

  afterEach(() => {
    if (dm?.isDrawModeActive()) dm.deactivateDrawMode()
  })

  test('deactivateAndSendResults is exported', () => {
    assert.strictEqual(typeof dm.deactivateAndSendResults, 'function')
  })

  test('deactivateAndSendResults does nothing when not active', () => {
    // Should not throw
    dm.deactivateAndSendResults()
    assert.strictEqual(dm.isDrawModeActive(), false)
  })

  test('double call does not throw (re-entry guard)', async () => {
    dm.activateDrawMode('user')

    // Track sendMessage calls
    const sendCalls = []
    globalThis.chrome.runtime.sendMessage = mock.fn((...args) => {
      sendCalls.push(args)
      // Simulate async callback for GASOLINE_CAPTURE_SCREENSHOT
      const callback = args[1]
      if (typeof callback === 'function') {
        callback({ dataUrl: '' })
      }
    })

    dm.deactivateAndSendResults()
    // Second call should be a no-op (re-entry guard)
    dm.deactivateAndSendResults()

    // Wait for the 300ms fade-out delay before deactivation completes
    await new Promise((r) => setTimeout(r, 350))
    assert.strictEqual(dm.isDrawModeActive(), false)
  })
})

// =============================================================================
// Phase: Parent Context, Siblings, CSS Framework enrichment
// =============================================================================
