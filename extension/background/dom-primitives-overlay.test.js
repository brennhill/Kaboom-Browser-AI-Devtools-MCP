// @ts-nocheck
/**
 * @fileoverview dom-primitives-overlay.test.js — Tests for overlay dismiss loop detection (#444),
 * cross-extension overlay detection (#445), and text= selector auto-resolve (#443).
 *
 * Run: node --test extension/background/dom-primitives-overlay.test.js
 */

import { test, describe, beforeEach } from 'node:test'
import assert from 'node:assert'

// ---------------------------------------------------------------------------
// DOM mocks — overlay-focused
// ---------------------------------------------------------------------------
const attrStores = new WeakMap()

class MockHTMLElement {
  constructor(tag, props = {}) {
    this.tagName = tag.toUpperCase()
    this.id = props.id || ''
    this.textContent = props.textContent || ''
    this.className = props.className || ''
    this.offsetParent = props.hidden ? null : {} // non-null = visible
    this.style = { position: props.position || '', zIndex: props.zIndex || '' }
    this._children = props.children || []
    this._parent = null
    this._shadow = null
    attrStores.set(this, { ...(props.attrs || {}) })
    // Set parent refs on children
    for (const child of this._children) {
      if (child && child._parent !== undefined) child._parent = this
    }
  }

  get children() {
    return this._children
  }

  getAttribute(name) {
    const store = attrStores.get(this)
    if (!store) return null
    return store[name] ?? null
  }

  setAttribute(name, value) {
    const store = attrStores.get(this) || {}
    store[name] = value
    attrStores.set(this, store)
  }

  removeAttribute(name) {
    const store = attrStores.get(this)
    if (store) delete store[name]
  }

  closest(sel) {
    // Simple check: see if this element or parent matches
    if (matchesSelector(this, sel)) return this
    if (this._parent) return this._parent.closest(sel)
    return null
  }

  querySelector(sel) {
    for (const child of this._children) {
      if (matchesSelector(child, sel)) return child
      if (child.querySelector) {
        const found = child.querySelector(sel)
        if (found) return found
      }
    }
    return null
  }

  querySelectorAll(sel) {
    const results = []
    for (const child of this._children) {
      if (matchesSelector(child, sel)) results.push(child)
      if (child.querySelectorAll) {
        results.push(...child.querySelectorAll(sel))
      }
    }
    return results
  }

  contains(el) {
    if (el === this) return true
    for (const child of this._children) {
      if (child === el) return true
      if (child.contains && child.contains(el)) return true
    }
    return false
  }

  getBoundingClientRect() {
    return { x: 10, y: 10, left: 10, top: 10, right: 410, bottom: 310, width: 400, height: 300 }
  }

  click() {}
  focus() {}
  scrollIntoView() {}
  dispatchEvent() {}
}

function matchesSelector(el, sel) {
  if (!el || !el.tagName) return false
  const tag = el.tagName.toLowerCase()
  // Handle comma-separated selectors
  if (sel.includes(',')) {
    return sel.split(',').some(s => matchesSelector(el, s.trim()))
  }
  if (sel === tag) return true
  if (sel === '*') return true
  if (sel.startsWith('#') && el.id === sel.slice(1)) return true
  if (sel.startsWith('.') && el.className && el.className.includes(sel.slice(1))) return true
  if (sel === 'a[href]' && tag === 'a' && el.getAttribute('href')) return true
  if (sel.startsWith('[') && sel.includes('=')) {
    const m = sel.match(/\[([^=]+)="([^"]*)"?\]/)
    if (m) return el.getAttribute(m[1]) === m[2]
  }
  if (sel.startsWith('[') && !sel.includes('=')) {
    const attr = sel.slice(1, -1)
    return el.getAttribute(attr) !== null
  }
  // handle tag.class
  if (sel.includes('.') && !sel.startsWith('.') && !sel.startsWith('[')) {
    const [t, c] = sel.split('.')
    return tag === t && el.className && el.className.includes(c)
  }
  // handle tag[attr]
  if (sel.includes('[') && !sel.startsWith('[')) {
    const t = sel.split('[')[0]
    const attrPart = '[' + sel.split('[').slice(1).join('[')
    return tag === t && matchesSelector(el, attrPart)
  }
  return false
}

globalThis.HTMLElement = MockHTMLElement
globalThis.HTMLInputElement = class extends MockHTMLElement {}
globalThis.HTMLTextAreaElement = class extends MockHTMLElement {}
globalThis.HTMLSelectElement = class extends MockHTMLElement {}
globalThis.HTMLAnchorElement = class extends MockHTMLElement {}
globalThis.CSS = { escape: (s) => s }
globalThis.NodeFilter = { SHOW_TEXT: 4 }
globalThis.ShadowRoot = class ShadowRoot {}
globalThis.MouseEvent = class extends Event {
  constructor(type, init = {}) {
    super(type, init)
  }
}
globalThis.InputEvent = class extends Event {}
globalThis.KeyboardEvent = class extends Event {
  constructor(type, init = {}) {
    super(type, init)
    this.key = init.key || ''
    this.code = init.code || ''
    this.keyCode = init.keyCode || 0
  }
}
globalThis.MutationObserver = class {
  constructor() {}
  observe() {}
  disconnect() {}
}
let perfNowValue = 0
globalThis.performance = { now: () => perfNowValue++ }
globalThis.window = { innerWidth: 1024, innerHeight: 768 }
globalThis.requestAnimationFrame = (cb) => setTimeout(cb, 0)

// ---------------------------------------------------------------------------
// Helper: set up document with overlay elements
// ---------------------------------------------------------------------------
function setupOverlayDocument(overlayElements, otherElements = []) {
  const allElements = [...overlayElements, ...otherElements]

  const mockBody = {
    querySelectorAll: (sel) => allElements.filter(el => matchesSelector(el, sel)),
    contains: (el) => allElements.includes(el),
    appendChild: () => {},
    children: { length: 0 },
    _children: allElements
  }

  globalThis.document = {
    querySelector: (sel) => allElements.find(el => matchesSelector(el, sel)) || null,
    querySelectorAll: (sel) => {
      if (sel === '*') return allElements
      return allElements.filter(el => matchesSelector(el, sel))
    },
    getElementById: (id) => allElements.find(el => el.id === id) || null,
    body: mockBody,
    documentElement: {
      clientHeight: 768,
      clientWidth: 1024,
      children: { length: 0 }
    },
    // Make document walkable by TreeWalker via _children pointing to body
    _children: [mockBody],
    createTreeWalker: (root, filter) => {
      const textNodes = []
      function walk(el) {
        if (el.textContent) {
          textNodes.push({ textContent: el.textContent, parentElement: el._parent || el })
        }
        if (el._children) {
          for (const child of el._children) walk(child)
        }
      }
      walk(root)
      let idx = -1
      return {
        currentNode: null,
        nextNode() {
          idx++
          if (idx < textNodes.length) {
            this.currentNode = textNodes[idx]
            return this.currentNode
          }
          return null
        }
      }
    },
    getSelection: () => null,
    execCommand: () => {},
    dispatchEvent: () => {}
  }
}

function makeOverlay(props = {}) {
  return new MockHTMLElement('DIV', {
    id: props.id || 'overlay-1',
    textContent: props.textContent || 'Cookie consent dialog',
    className: props.className || '',
    position: 'fixed',
    zIndex: props.zIndex || '9999',
    attrs: {
      'role': props.role || 'dialog',
      ...(props.ariaModal ? { 'aria-modal': 'true' } : {}),
      ...(props.attrs || {})
    },
    children: props.children || []
  })
}

function makeCloseButton(props = {}) {
  return new MockHTMLElement('BUTTON', {
    id: props.id || 'close-btn',
    textContent: props.textContent || 'Close',
    className: props.className || 'close',
    attrs: {
      'aria-label': props.ariaLabel || 'Close',
      ...(props.attrs || {})
    }
  })
}

// ---------------------------------------------------------------------------
// Import domPrimitive AFTER globals
// ---------------------------------------------------------------------------
const { domPrimitive } = await import('./dom-primitives.js')
// #502: overlay dismiss actions were extracted from domPrimitive into a self-contained module.
const { domPrimitiveOverlay } = await import('./dom-primitives-overlay.js')

// ===========================================================================
// #444: Overlay dismiss loop detection
// ===========================================================================
describe('#444: overlay dismiss loop detection', () => {
  beforeEach(() => {
    perfNowValue = 0
    globalThis.getComputedStyle = (el) => ({
      visibility: 'visible',
      display: 'block',
      opacity: '1',
      position: el.style?.position || '',
      zIndex: el.style?.zIndex || ''
    })
  })

  test('first dismiss attempt proceeds normally (no stamp)', async () => {
    const closeBtn = makeCloseButton()
    const overlay = makeOverlay({ children: [closeBtn] })
    setupOverlayDocument([overlay], [closeBtn])

    const result = await domPrimitiveOverlay('dismiss_top_overlay', {})
    assert.strictEqual(result.success, true, 'First dismiss should succeed')
  })

  test('overlay is stamped with data-kaboom-dismiss-ts after dismiss', async () => {
    const closeBtn = makeCloseButton()
    const overlay = makeOverlay({ children: [closeBtn] })
    setupOverlayDocument([overlay], [closeBtn])

    await domPrimitiveOverlay('dismiss_top_overlay', {})

    const stamp = overlay.getAttribute('data-kaboom-dismiss-ts')
    assert.ok(stamp, 'Overlay should be stamped after dismiss attempt')
    assert.ok(Number(stamp) > 0, 'Stamp should be a positive timestamp')
  })

  test('second dismiss on same overlay returns dismiss_loop_detected error', async () => {
    const closeBtn = makeCloseButton()
    const overlay = makeOverlay({
      children: [closeBtn],
      attrs: { 'data-kaboom-dismiss-ts': String(Date.now() - 1000) } // stamped 1s ago
    })
    setupOverlayDocument([overlay], [closeBtn])

    const result = await domPrimitiveOverlay('dismiss_top_overlay', {})
    assert.strictEqual(result.success, false, 'Second dismiss should fail')
    assert.strictEqual(result.error, 'dismiss_loop_detected', 'Should return dismiss_loop_detected')
  })

  test('stale stamp (>30s) is cleared and dismiss proceeds', async () => {
    const closeBtn = makeCloseButton()
    const overlay = makeOverlay({
      children: [closeBtn],
      attrs: { 'data-kaboom-dismiss-ts': String(Date.now() - 60000) } // stamped 60s ago
    })
    setupOverlayDocument([overlay], [closeBtn])

    const result = await domPrimitiveOverlay('dismiss_top_overlay', {})
    assert.strictEqual(result.success, true, 'Stale stamp should be ignored')
  })

  test('dismiss_loop_detected includes overlay info in message', async () => {
    const overlay = makeOverlay({
      id: 'cookie-modal',
      role: 'dialog',
      textContent: 'Accept cookies?',
      attrs: { 'data-kaboom-dismiss-ts': String(Date.now() - 2000) }
    })
    setupOverlayDocument([overlay])

    const result = await domPrimitiveOverlay('dismiss_top_overlay', {})
    assert.strictEqual(result.error, 'dismiss_loop_detected')
    assert.ok(result.message.includes('already attempted'), 'Message should mention prior attempt')
    assert.ok(result.overlay_selector, 'Should include overlay selector')
  })
})

// ===========================================================================
// #445: Cross-extension overlay detection
// ===========================================================================
describe('#445: cross-extension overlay detection', () => {
  beforeEach(() => {
    perfNowValue = 0
    globalThis.getComputedStyle = (el) => ({
      visibility: 'visible',
      display: 'block',
      opacity: '1',
      position: el.style?.position || '',
      zIndex: el.style?.zIndex || ''
    })
  })

  test('overlay with chrome-extension src is flagged as extension-sourced', async () => {
    const iframe = new MockHTMLElement('IFRAME', {
      attrs: { src: 'chrome-extension://abc123/popup.html' }
    })
    const closeBtn = makeCloseButton()
    const overlay = makeOverlay({
      id: 'ext-overlay',
      children: [iframe, closeBtn],
      attrs: {}
    })
    setupOverlayDocument([overlay], [iframe, closeBtn])

    const result = await domPrimitiveOverlay('dismiss_top_overlay', {})
    assert.strictEqual(result.success, true, 'Dismiss should succeed')
    assert.strictEqual(result.overlay_source, 'extension',
      'Extension overlay should be flagged as extension-sourced')
  })

  test('normal page overlay is flagged as page-sourced', async () => {
    const closeBtn = makeCloseButton()
    const overlay = makeOverlay({ children: [closeBtn] })
    setupOverlayDocument([overlay], [closeBtn])

    const result = await domPrimitiveOverlay('dismiss_top_overlay', {})
    assert.strictEqual(result.success, true)
    assert.strictEqual(result.overlay_source, 'page',
      'Page overlay should be flagged as page-sourced')
  })
})

// ===========================================================================
// #443: text= selector auto-resolve to clickable child
// ===========================================================================
describe('#443: text= selector resolves to interactive child', () => {
  beforeEach(() => {
    perfNowValue = 0
    globalThis.getComputedStyle = (el) => ({
      visibility: 'visible',
      display: 'block',
      opacity: '1',
      position: el.style?.position || '',
      zIndex: el.style?.zIndex || ''
    })
  })

  test('text= in non-interactive container resolves to interactive child', async () => {
    const link = new MockHTMLElement('A', {
      id: 'inner-link',
      textContent: 'Click here',
      attrs: { href: '/page' }
    })
    const td = new MockHTMLElement('TD', {
      id: 'cell',
      textContent: 'Click here',
      children: [link]
    })
    link._parent = td

    setupOverlayDocument([], [td, link])

    // Use text= selector to find "Click here"
    const result = await domPrimitive('click', 'text=Click here', {})

    assert.strictEqual(result.success, true, 'Click should succeed')
    assert.ok(result.matched, 'Should have matched element info')
    assert.strictEqual(result.matched.tag, 'a',
      'Should resolve to the <a> child, not the <td> container')
  })

  test('text= in interactive element resolves to that element directly', async () => {
    const button = new MockHTMLElement('BUTTON', {
      id: 'direct-btn',
      textContent: 'Submit'
    })

    setupOverlayDocument([], [button])

    const result = await domPrimitive('click', 'text=Submit', {})
    assert.strictEqual(result.success, true, 'Click should succeed')
    assert.ok(result.matched, 'Should have matched element info')
    assert.strictEqual(result.matched.tag, 'button',
      'Should resolve to the button directly')
  })

  test('text= with no interactive child falls back to container', async () => {
    const span = new MockHTMLElement('SPAN', {
      textContent: 'Static text'
    })
    const div = new MockHTMLElement('DIV', {
      id: 'container',
      textContent: 'Static text',
      children: [span]
    })
    span._parent = div

    setupOverlayDocument([], [div, span])

    const result = await domPrimitive('click', 'text=Static text', {})
    assert.strictEqual(result.success, true, 'Click should succeed even on non-interactive container')
  })
})
