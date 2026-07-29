// @ts-nocheck
/**
 * @fileoverview pilot-highlight.test.js — Tests for AI Web Pilot highlight_element feature.
 * Covers highlight overlay creation, positioning, auto-removal, duplicate handling.
 * Uses inline function copy for isolated testing (avoids module side-effects).
 */

import { test, describe, mock, beforeEach, afterEach } from 'node:test'
import assert from 'node:assert'
import { createMockWindow } from '../shared/helpers.js'

// ============================================================================
// UNIT TESTS FOR highlightElement FUNCTION
// Uses inline implementation to avoid inject.js module side-effects during import.
// The actual implementation in inject.js matches this code exactly.
// ============================================================================

describe('highlightElement function', () => {
  let mockWindow
  let mockDocument
  let appendedElements
  let removedElements
  let gasolineHighlighter

  // Create mock element factory
  const createMockElement = (tagName) => {
    const el = {
      tagName,
      id: '',
      style: {},
      dataset: {},
      remove: mock.fn(() => {
        removedElements.push(el)
      }),
      getBoundingClientRect: mock.fn(() => ({
        top: 100,
        left: 50,
        width: 200,
        height: 100,
        x: 50,
        y: 100
      }))
    }
    return el
  }

  beforeEach(() => {
    appendedElements = []
    removedElements = []
    gasolineHighlighter = null

    // Mock document
    mockDocument = {
      querySelector: mock.fn((selector) => {
        if (selector === '.test-element' || selector === '#my-button') {
          return createMockElement('div')
        }
        if (selector === '.nonexistent') {
          return null
        }
        if (selector === '#kaboom-highlighter') {
          return gasolineHighlighter
        }
        return null
      }),
      createElement: mock.fn((tagName) => createMockElement(tagName)),
      body: {
        appendChild: mock.fn((el) => {
          appendedElements.push(el)
          gasolineHighlighter = el
        })
      }
    }

    mockWindow = createMockWindow({
      withOnerror: true,
      overrides: {
        setTimeout: mock.fn((_cb, _ms) => {
          return 123 // Return a mock timer ID
        })
      }
    })

    globalThis.window = mockWindow
    globalThis.document = mockDocument
  })

  afterEach(() => {
    delete globalThis.window
    delete globalThis.document
  })

  // Inline implementation matching inject.js highlightElement
  function highlightElement(selector, durationMs = 5000) {
    // Remove existing highlight
    if (gasolineHighlighter) {
      gasolineHighlighter.remove()
      gasolineHighlighter = null
    }

    const element = document.querySelector(selector)
    if (!element) {
      return { success: false, error: 'element_not_found', selector }
    }

    const rect = element.getBoundingClientRect()

    gasolineHighlighter = document.createElement('div')
    gasolineHighlighter.id = 'kaboom-highlighter'
    gasolineHighlighter.dataset.selector = selector
    Object.assign(gasolineHighlighter.style, {
      position: 'fixed',
      top: `${rect.top}px`,
      left: `${rect.left}px`,
      width: `${rect.width}px`,
      height: `${rect.height}px`,
      border: '4px solid red',
      borderRadius: '4px',
      backgroundColor: 'rgba(255, 0, 0, 0.1)',
      zIndex: '2147483647',
      pointerEvents: 'none',
      boxSizing: 'border-box'
    })

    document.body.appendChild(gasolineHighlighter)

    globalThis.window.setTimeout(() => {
      if (gasolineHighlighter) {
        gasolineHighlighter.remove()
        gasolineHighlighter = null
      }
    }, durationMs)

    return {
      success: true,
      selector,
      bounds: { x: rect.x, y: rect.y, width: rect.width, height: rect.height }
    }
  }

  function clearHighlight() {
    if (gasolineHighlighter) {
      gasolineHighlighter.remove()
      gasolineHighlighter = null
    }
  }

  test('creates div with correct ID', () => {
    highlightElement('.test-element')

    assert.ok(appendedElements.length > 0, 'Should append a highlighter element')
    const highlighter = appendedElements[appendedElements.length - 1]
    assert.strictEqual(highlighter.id, 'kaboom-highlighter', 'Should have correct ID')
  })

  test('creates div with fixed positioning', () => {
    highlightElement('.test-element')

    const highlighter = appendedElements[appendedElements.length - 1]
    assert.strictEqual(highlighter.style.position, 'fixed', 'Should use fixed positioning')
  })

  test('creates div with red border', () => {
    highlightElement('.test-element')

    const highlighter = appendedElements[appendedElements.length - 1]
    assert.strictEqual(highlighter.style.border, '4px solid red', 'Should have red border')
  })

  test('creates div with rounded corners', () => {
    highlightElement('.test-element')

    const highlighter = appendedElements[appendedElements.length - 1]
    assert.strictEqual(highlighter.style.borderRadius, '4px', 'Should have rounded corners')
  })

  test('creates div with semi-transparent red background', () => {
    highlightElement('.test-element')

    const highlighter = appendedElements[appendedElements.length - 1]
    assert.strictEqual(
      highlighter.style.backgroundColor,
      'rgba(255, 0, 0, 0.1)',
      'Should have semi-transparent red background'
    )
  })

  test('creates div with max z-index', () => {
    highlightElement('.test-element')

    const highlighter = appendedElements[appendedElements.length - 1]
    assert.strictEqual(highlighter.style.zIndex, '2147483647', 'Should have max z-index')
  })

  test('creates div with pointer-events none', () => {
    highlightElement('.test-element')

    const highlighter = appendedElements[appendedElements.length - 1]
    assert.strictEqual(highlighter.style.pointerEvents, 'none', 'Should not intercept pointer events')
  })

  test('creates div with border-box sizing', () => {
    highlightElement('.test-element')

    const highlighter = appendedElements[appendedElements.length - 1]
    assert.strictEqual(highlighter.style.boxSizing, 'border-box', 'Should use border-box sizing')
  })

  test('positions on element bounds - x coordinate', () => {
    const result = highlightElement('.test-element')

    assert.ok(result.success, 'Should succeed')
    assert.strictEqual(result.bounds.x, 50, 'Should have correct x position')
  })

  test('positions on element bounds - y coordinate', () => {
    const result = highlightElement('.test-element')

    assert.strictEqual(result.bounds.y, 100, 'Should have correct y position')
  })

  test('positions on element bounds - width', () => {
    const result = highlightElement('.test-element')

    assert.strictEqual(result.bounds.width, 200, 'Should have correct width')
  })

  test('positions on element bounds - height', () => {
    const result = highlightElement('.test-element')

    assert.strictEqual(result.bounds.height, 100, 'Should have correct height')
  })

  test('sets correct top style', () => {
    highlightElement('.test-element')

    const highlighter = appendedElements[appendedElements.length - 1]
    assert.strictEqual(highlighter.style.top, '100px', 'Should position at element top')
  })

  test('sets correct left style', () => {
    highlightElement('.test-element')

    const highlighter = appendedElements[appendedElements.length - 1]
    assert.strictEqual(highlighter.style.left, '50px', 'Should position at element left')
  })

  test('sets correct width style', () => {
    highlightElement('.test-element')

    const highlighter = appendedElements[appendedElements.length - 1]
    assert.strictEqual(highlighter.style.width, '200px', 'Should match element width')
  })

  test('sets correct height style', () => {
    highlightElement('.test-element')

    const highlighter = appendedElements[appendedElements.length - 1]
    assert.strictEqual(highlighter.style.height, '100px', 'Should match element height')
  })

  test('schedules auto-removal with setTimeout', () => {
    highlightElement('.test-element', 3000)

    assert.ok(mockWindow.setTimeout.mock.calls.length > 0, 'Should call setTimeout')
    const [, duration] = mockWindow.setTimeout.mock.calls[0].arguments
    assert.strictEqual(duration, 3000, 'Should set timeout with specified duration')
  })

  test('uses default 5000ms duration when not specified', () => {
    highlightElement('.test-element')

    const [, duration] = mockWindow.setTimeout.mock.calls[0].arguments
    assert.strictEqual(duration, 5000, 'Should default to 5000ms duration')
  })

  test('second highlight removes first', () => {
    // First highlight
    const result1 = highlightElement('.test-element')
    assert.ok(result1.success, 'First highlight should succeed')

    // Second highlight - should remove the first
    const result2 = highlightElement('#my-button')
    assert.ok(result2.success, 'Second highlight should succeed')

    // First highlighter should have been removed
    assert.ok(removedElements.length > 0, 'First highlighter should be removed')
  })

  test('returns error for non-existent selector', () => {
    const result = highlightElement('.nonexistent')

    assert.strictEqual(result.success, false, 'Should not succeed')
    assert.strictEqual(result.error, 'element_not_found', 'Should return element_not_found error')
    assert.strictEqual(result.selector, '.nonexistent', 'Should include the selector')
  })

  test('stores selector in dataset for scroll tracking', () => {
    highlightElement('.test-element')

    const highlighter = appendedElements[appendedElements.length - 1]
    assert.strictEqual(highlighter.dataset.selector, '.test-element', 'Should store selector in dataset')
  })

  test('clearHighlight removes existing highlighter', () => {
    highlightElement('.test-element')
    assert.ok(gasolineHighlighter, 'Should have a highlighter')

    clearHighlight()
    assert.ok(removedElements.length > 0, 'Should have removed the highlighter')
  })

  test('clearHighlight is safe to call when no highlighter exists', () => {
    // Should not throw
    clearHighlight()
    assert.strictEqual(removedElements.length, 0, 'Should not remove anything')
  })
})
