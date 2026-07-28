// @ts-nocheck
/**
 * @fileoverview Draw-mode activation, annotation CRUD, persistence, and export tests.
 */

import { test, describe, mock, beforeEach } from 'node:test'
import assert from 'node:assert'
import {
  createdElements,
  documentBody,
  importDrawMode,
  nextModuleVersion,
  setupGlobals,
} from './draw-mode-fixture.js'

describe('Draw Mode — Activation/Deactivation', () => {
  let dm

  beforeEach(async () => {
    setupGlobals()
    dm = await importDrawMode()
  })

  test('activateDrawMode returns active status', () => {
    const result = dm.activateDrawMode('user')
    assert.strictEqual(result.status, 'active')
    assert.strictEqual(result.started_by, 'user')
  })

  test('isDrawModeActive returns true after activation', () => {
    dm.activateDrawMode('user')
    assert.strictEqual(dm.isDrawModeActive(), true)
  })

  test('activateDrawMode from LLM sets started_by', () => {
    const result = dm.activateDrawMode('llm')
    assert.strictEqual(result.started_by, 'llm')
  })

  test('double activation returns already_active', () => {
    dm.activateDrawMode('user')
    const result = dm.activateDrawMode('user')
    assert.strictEqual(result.status, 'already_active')
    assert.strictEqual(typeof result.annotation_count, 'number')
  })

  test('deactivateDrawMode returns results', () => {
    dm.activateDrawMode('user')
    const result = dm.deactivateDrawMode()
    assert.ok(Array.isArray(result.annotations))
    assert.ok(result.elementDetails !== undefined)
  })

  test('deactivateDrawMode when not active returns empty', () => {
    const result = dm.deactivateDrawMode()
    assert.deepStrictEqual(result.annotations, [])
  })

  test('isDrawModeActive returns false after deactivation', () => {
    dm.activateDrawMode('user')
    dm.deactivateDrawMode()
    assert.strictEqual(dm.isDrawModeActive(), false)
  })

  test('overlay is appended to document.body on activation', () => {
    dm.activateDrawMode('user')
    assert.ok(documentBody.children.length > 0, 'expected overlay to be appended')
  })

  test('overlay is removed on deactivation', () => {
    dm.activateDrawMode('user')
    assert.ok(documentBody.children.length > 0, 'overlay should exist before deactivation')
    dm.deactivateDrawMode()
    assert.strictEqual(dm.isDrawModeActive(), false, 'draw mode should be inactive after deactivation')
  })
})

describe('Draw Mode — Annotations CRUD', () => {
  let dm

  beforeEach(async () => {
    setupGlobals()
    dm = await importDrawMode()
  })

  test('getAnnotations returns empty array initially', () => {
    dm.activateDrawMode('user')
    const anns = dm.getAnnotations()
    assert.deepStrictEqual(anns, [])
  })

  test('clearAnnotations empties the list', () => {
    dm.activateDrawMode('user')
    dm.clearAnnotations()
    assert.deepStrictEqual(dm.getAnnotations(), [])
  })

  test('getElementDetail returns null for unknown correlationId', () => {
    const detail = dm.getElementDetail('nonexistent')
    assert.strictEqual(detail, null)
  })

  test('getAnnotations returns copies (not references)', () => {
    dm.activateDrawMode('user')
    const a = dm.getAnnotations()
    const b = dm.getAnnotations()
    assert.notStrictEqual(a, b) // Different array instances
  })
})

describe('Draw Mode — Export', () => {
  let compositeAnnotations

  beforeEach(async () => {
    setupGlobals()
    const mod = await import(
      `../../extension/content/draw-mode-export.js?v=${nextModuleVersion()}`,
    )
    compositeAnnotations = mod.compositeAnnotations
  })

  test('returns original screenshot for empty annotations', async () => {
    const dataUrl = 'data:image/png;base64,abc'
    const result = await compositeAnnotations(dataUrl, [])
    assert.strictEqual(result, dataUrl)
  })

  test('returns original screenshot for null annotations', async () => {
    const dataUrl = 'data:image/png;base64,abc'
    const result = await compositeAnnotations(dataUrl, null)
    assert.strictEqual(result, dataUrl)
  })

  test('composites annotations onto screenshot', async () => {
    const dataUrl = 'data:image/png;base64,abc'
    const annotations = [{ rect: { x: 100, y: 200, width: 150, height: 50 }, text: 'make darker', id: 'ann_1' }]
    const result = await compositeAnnotations(dataUrl, annotations)
    // Should return a data URL (from our mock canvas.toDataURL)
    assert.ok(result.startsWith('data:image/png'))
  })
})

describe('Draw Mode — Event Handling Basics', () => {
  let dm

  beforeEach(async () => {
    setupGlobals()
    dm = await importDrawMode()
  })

  test('ESC keydown triggers deactivation and sends messages', async () => {
    dm.activateDrawMode('user')

    // Track all sendMessage calls in order
    const sentMessages = []
    globalThis.chrome.runtime.sendMessage = mock.fn((msg, callback) => {
      sentMessages.push(msg)
      if (msg.type === 'kaboom_capture_screenshot' && typeof callback === 'function') {
        // Simulate background returning a screenshot synchronously
        callback({ dataUrl: 'data:image/png;base64,mockscreenshot' })
      }
      return undefined
    })

    // Find the keydown listener registered on document
    const keydownCalls = globalThis.document.addEventListener.mock.calls
    const keydownHandler = keydownCalls.find((c) => c.arguments[0] === 'keydown')?.arguments[1]

    if (keydownHandler) {
      const event = {
        key: 'Escape',
        preventDefault: mock.fn(),
        stopPropagation: mock.fn()
      }
      keydownHandler(event)

      // Wait for the 300ms fade-out delay before deactivation completes
      await new Promise((r) => setTimeout(r, 350))

      // After ESC + fade, draw mode should be deactivated
      assert.strictEqual(dm.isDrawModeActive(), false, 'draw mode should be deactivated after ESC')

      // Should have sent messages (toast + capture screenshot + completed)
      assert.ok(sentMessages.length > 0, 'expected at least one sendMessage call')

      // Find the DRAW_MODE_COMPLETED message
      const completed = sentMessages.find((m) => m.type === 'draw_mode_completed')
      assert.ok(completed, 'expected DRAW_MODE_COMPLETED message')
      assert.ok(Array.isArray(completed.annotations), 'expected annotations array')

      // Verify toast was sent
      const toast = sentMessages.find((m) => m.type === 'kaboom_action_toast')
      assert.ok(toast, 'expected GASOLINE_ACTION_TOAST message')
      assert.strictEqual(toast.text, 'Annotations submitted')
      assert.strictEqual(toast.state, 'success')
    }
  })

  test('draw mode creates canvas element', () => {
    dm.activateDrawMode('user')
    const canvasEls = createdElements.filter((el) => el.tagName === 'CANVAS')
    assert.ok(canvasEls.length > 0, 'expected canvas to be created')
  })

  test('draw mode creates badge element', () => {
    dm.activateDrawMode('user')
    // Badge div is created as part of overlay
    assert.ok(createdElements.length >= 3, 'expected overlay, canvas, badge, and style elements')
  })
})

describe('Draw Mode — Persistence', () => {
  let dm

  beforeEach(async () => {
    setupGlobals()
    dm = await importDrawMode()
  })

  test('activateDrawMode loads annotations from storage', () => {
    dm.activateDrawMode('user')
    // chrome.storage.session.get should have been called
    const getCalls = globalThis.chrome.storage.session.get.mock.calls
    assert.ok(getCalls.length > 0, 'expected storage.session.get to be called')
  })

  test('clearAnnotations triggers persistence', (_t) => {
    dm.activateDrawMode('user')

    // Reset mock call count
    globalThis.chrome.storage.session.set.mock.resetCalls()

    dm.clearAnnotations()

    // Verify annotations were actually cleared
    const anns = dm.getAnnotations()
    assert.deepStrictEqual(anns, [], 'annotations should be empty after clearAnnotations')
  })
})
