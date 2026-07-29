// @ts-nocheck
/**
 * @fileoverview transient-capture.test.js — Tests for transient UI element capture.
 * Verifies classification logic (ARIA, class fingerprints, computed style),
 * skip tags, and subtree walking (classifyCandidates).
 *
 * Note: These tests exercise stateless classification functions only. Dedup logic
 * (isDuplicate, recordDedup) and lifecycle (install/uninstall) are internal to the
 * module and would require full browser API mocks (MutationObserver, postMessage)
 * to test. Node.js module caching means all tests share the same module instance —
 * acceptable here since classification functions are pure.
 */

import { test, describe, beforeEach, afterEach, mock } from 'node:test'
import assert from 'node:assert'

// Mock window and document for Node.js environment
let originalWindow
let originalDocument
let mockPostMessage

function setupBrowserEnv() {
  originalWindow = globalThis.window
  originalDocument = globalThis.document
  mockPostMessage = mock.fn()

  globalThis.window = {
    location: { href: 'https://example.com', origin: 'https://example.com' },
    postMessage: mockPostMessage,
    getComputedStyle: () => ({
      position: 'static',
      zIndex: 'auto'
    })
  }

  globalThis.document = {
    body: {
      tagName: 'BODY'
    },
    readyState: 'complete'
  }

  // Mock requestIdleCallback
  globalThis.requestIdleCallback = (fn) => setTimeout(fn, 0)
}

function teardownBrowserEnv() {
  if (originalWindow !== undefined) {
    globalThis.window = originalWindow
  } else {
    delete globalThis.window
  }
  if (originalDocument !== undefined) {
    globalThis.document = originalDocument
  } else {
    delete globalThis.document
  }
  delete globalThis.requestIdleCallback
}

// Helper to create mock DOM elements
function createElement(tag, attrs = {}, opts = {}) {
  const children = opts.children || []
  return {
    tagName: tag.toUpperCase(),
    nodeType: 1,
    id: attrs.id || '',
    className: attrs.class || '',
    textContent: opts.textContent || '',
    innerText: opts.textContent || '',
    parentElement: opts.parent || null,
    children,
    getAttribute: (name) => {
      if (name === 'role') return attrs.role || null
      if (name === 'aria-live') return attrs['aria-live'] || null
      if (name === 'aria-label') return attrs['aria-label'] || null
      if (name === 'data-testid') return attrs['data-testid'] || null
      if (name === 'data-test-id') return attrs['data-test-id'] || null
      if (name === 'data-cy') return attrs['data-cy'] || null
      if (name === 'type') return attrs.type || null
      if (name === 'href') return attrs.href || null
      return attrs[name] || null
    },
    hasAttribute: (name) => name in attrs,
    getBoundingClientRect: () => ({
      height: opts.height ?? 50,
      width: opts.width ?? 200,
      top: 0,
      left: 0,
      bottom: opts.height ?? 50,
      right: opts.width ?? 200
    }),
    querySelectorAll: mock.fn(() => [])
  }
}

// --- Classification Tests ---

describe('classifyTransient', () => {
  beforeEach(setupBrowserEnv)
  afterEach(teardownBrowserEnv)

  test('should classify role="alert" as alert', async () => {
    const { classifyTransient } = await import('../../../extension/lib/page/transient-capture.js')

    const el = createElement('div', { role: 'alert' }, { textContent: 'Error occurred' })
    const result = classifyTransient(el)

    assert.ok(result)
    assert.strictEqual(result.classification, 'alert')
    assert.strictEqual(result.role, 'alert')
    assert.strictEqual(result.text, 'Error occurred')
  })

  test('should classify aria-live="assertive" as alert', async () => {
    const { classifyTransient } = await import('../../../extension/lib/page/transient-capture.js')

    const el = createElement('div', { 'aria-live': 'assertive' }, { textContent: 'Critical error' })
    const result = classifyTransient(el)

    assert.ok(result)
    assert.strictEqual(result.classification, 'alert')
  })

  test('should classify role="status" as toast', async () => {
    const { classifyTransient } = await import('../../../extension/lib/page/transient-capture.js')

    const el = createElement('div', { role: 'status' }, { textContent: 'Saved' })
    const result = classifyTransient(el)

    assert.ok(result)
    assert.strictEqual(result.classification, 'toast')
    assert.strictEqual(result.role, 'status')
  })

  test('should classify aria-live="polite" as toast', async () => {
    const { classifyTransient } = await import('../../../extension/lib/page/transient-capture.js')

    const el = createElement('div', { 'aria-live': 'polite' }, { textContent: 'Updated' })
    const result = classifyTransient(el)

    assert.ok(result)
    assert.strictEqual(result.classification, 'toast')
  })

  test('should classify toast class as toast', async () => {
    const { classifyTransient } = await import('../../../extension/lib/page/transient-capture.js')

    const el = createElement('div', { class: 'Toastify__toast' }, { textContent: 'Success!' })
    const result = classifyTransient(el)

    assert.ok(result)
    assert.strictEqual(result.classification, 'toast')
  })

  test('should classify snackbar class as snackbar', async () => {
    const { classifyTransient } = await import('../../../extension/lib/page/transient-capture.js')

    const el = createElement('div', { class: 'MuiSnackbar-root' }, { textContent: 'Item deleted' })
    const result = classifyTransient(el)

    assert.ok(result)
    assert.strictEqual(result.classification, 'snackbar')
  })

  test('should classify notification class as notification', async () => {
    const { classifyTransient } = await import('../../../extension/lib/page/transient-capture.js')

    const el = createElement('div', { class: 'notification-banner' }, { textContent: 'New message' })
    const result = classifyTransient(el)

    assert.ok(result)
    assert.strictEqual(result.classification, 'notification')
  })

  test('should classify tooltip class as tooltip', async () => {
    const { classifyTransient } = await import('../../../extension/lib/page/transient-capture.js')

    const el = createElement('div', { class: 'tooltip-content' }, { textContent: 'Help text' })
    const result = classifyTransient(el)

    assert.ok(result)
    assert.strictEqual(result.classification, 'tooltip')
  })

  test('should skip SCRIPT elements', async () => {
    const { classifyTransient } = await import('../../../extension/lib/page/transient-capture.js')

    const el = createElement('script', { role: 'alert' }, { textContent: 'alert("test")' })
    const result = classifyTransient(el)

    assert.strictEqual(result, null)
  })

  test('should skip STYLE elements', async () => {
    const { classifyTransient } = await import('../../../extension/lib/page/transient-capture.js')

    const el = createElement('style', {}, { textContent: 'body { color: red }' })
    const result = classifyTransient(el)

    assert.strictEqual(result, null)
  })

  test('should skip LINK elements', async () => {
    const { classifyTransient } = await import('../../../extension/lib/page/transient-capture.js')

    const el = createElement('link', {}, { textContent: 'stylesheet ref' })
    const result = classifyTransient(el)

    assert.strictEqual(result, null)
  })

  test('should skip META elements', async () => {
    const { classifyTransient } = await import('../../../extension/lib/page/transient-capture.js')

    const el = createElement('meta', {}, { textContent: 'charset info' })
    const result = classifyTransient(el)

    assert.strictEqual(result, null)
  })

  test('should skip elements with no text content', async () => {
    const { classifyTransient } = await import('../../../extension/lib/page/transient-capture.js')

    const el = createElement('div', { role: 'alert' }, { textContent: '' })
    const result = classifyTransient(el)

    assert.strictEqual(result, null)
  })

  test('should skip elements with only whitespace text', async () => {
    const { classifyTransient } = await import('../../../extension/lib/page/transient-capture.js')

    const el = createElement('div', { role: 'alert' }, { textContent: '   \n\t  ' })
    const result = classifyTransient(el)

    assert.strictEqual(result, null)
  })

  test('should return null for plain div with no transient signals', async () => {
    const { classifyTransient } = await import('../../../extension/lib/page/transient-capture.js')

    const el = createElement('div', {}, { textContent: 'Regular content' })
    // Override getComputedStyle to return non-matching styles
    globalThis.window.getComputedStyle = () => ({
      position: 'static',
      zIndex: 'auto'
    })

    const result = classifyTransient(el)
    assert.strictEqual(result, null)
  })

  test('should classify fixed/high-z-index/small element as flash', async () => {
    const { classifyTransient } = await import('../../../extension/lib/page/transient-capture.js')

    const el = createElement('div', {}, { textContent: 'Flash message', height: 80 })
    globalThis.window.getComputedStyle = () => ({
      position: 'fixed',
      zIndex: '9999'
    })

    const result = classifyTransient(el)

    assert.ok(result)
    assert.strictEqual(result.classification, 'flash')
  })

  test('should NOT classify fixed element with height > 200 as flash', async () => {
    const { classifyTransient } = await import('../../../extension/lib/page/transient-capture.js')

    const el = createElement('div', {}, { textContent: 'Large fixed element', height: 300 })
    globalThis.window.getComputedStyle = () => ({
      position: 'fixed',
      zIndex: '9999'
    })

    const result = classifyTransient(el)
    assert.strictEqual(result, null)
  })

  test('should NOT classify fixed element with low z-index as flash', async () => {
    const { classifyTransient } = await import('../../../extension/lib/page/transient-capture.js')

    const el = createElement('div', {}, { textContent: 'Low z fixed', height: 80 })
    globalThis.window.getComputedStyle = () => ({
      position: 'fixed',
      zIndex: '10'
    })

    const result = classifyTransient(el)
    assert.strictEqual(result, null)
  })

  test('should prioritize ARIA over class fingerprints', async () => {
    const { classifyTransient } = await import('../../../extension/lib/page/transient-capture.js')

    // Element has both role="alert" AND toast class
    const el = createElement('div', { role: 'alert', class: 'toast-container' }, { textContent: 'Error!' })
    const result = classifyTransient(el)

    assert.ok(result)
    // ARIA takes priority — should be "alert", not "toast"
    assert.strictEqual(result.classification, 'alert')
  })

  test('should truncate text to 500 chars', async () => {
    const { classifyTransient } = await import('../../../extension/lib/page/transient-capture.js')

    const longText = 'A'.repeat(600)
    const el = createElement('div', { role: 'alert' }, { textContent: longText })
    const result = classifyTransient(el)

    assert.ok(result)
    assert.strictEqual(result.text.length, 500)
  })
})

// --- Subtree walking tests (classifyCandidates) ---

describe('classifyCandidates', () => {
  beforeEach(setupBrowserEnv)
  afterEach(teardownBrowserEnv)

  test('should classify element itself when it has ARIA role', async () => {
    const { classifyCandidates } = await import('../../../extension/lib/page/transient-capture.js')

    const el = createElement('div', { role: 'alert' }, { textContent: 'Error!' })
    const result = classifyCandidates(el)

    assert.ok(result)
    assert.strictEqual(result.classification, 'alert')
  })

  test('should walk direct children to find transient signals', async () => {
    const { classifyCandidates } = await import('../../../extension/lib/page/transient-capture.js')

    // Framework wrapper pattern: outer div has no ARIA, inner child has role="alert"
    const child = createElement('div', { role: 'alert' }, { textContent: 'Error inside wrapper' })
    const wrapper = createElement('div', {}, { textContent: 'Error inside wrapper', children: [child] })
    const result = classifyCandidates(wrapper)

    assert.ok(result)
    assert.strictEqual(result.classification, 'alert')
  })

  test('should prioritize parent classification over child', async () => {
    const { classifyCandidates } = await import('../../../extension/lib/page/transient-capture.js')

    // Parent has role="status" (toast), child has role="alert" — parent should win
    const child = createElement('div', { role: 'alert' }, { textContent: 'Error in child' })
    const wrapper = createElement('div', { role: 'status' }, { textContent: 'Status wrapper', children: [child] })
    const result = classifyCandidates(wrapper)

    assert.ok(result)
    assert.strictEqual(result.classification, 'toast')
  })

  test('should return null when neither element nor children are transient', async () => {
    const { classifyCandidates } = await import('../../../extension/lib/page/transient-capture.js')

    const child = createElement('div', {}, { textContent: 'Plain child' })
    const wrapper = createElement('div', {}, { textContent: 'Plain wrapper', children: [child] })
    globalThis.window.getComputedStyle = () => ({
      position: 'static',
      zIndex: 'auto'
    })
    const result = classifyCandidates(wrapper)

    assert.strictEqual(result, null)
  })
})
