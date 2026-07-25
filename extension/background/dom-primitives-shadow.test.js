// @ts-nocheck
/**
 * @fileoverview dom-primitives-shadow.test.js — Tests for shadow DOM deep traversal.
 *
 * Tests cover: getShadowRoot, querySelectorDeep, querySelectorAllDeep,
 * resolveByTextDeep, >>> combinator, buildShadowSelector, list_interactive
 * shadow upgrade, and isVisible shadow fix.
 *
 * Run: node --test extension/background/dom-primitives-shadow.test.js
 */

import { test, describe, beforeEach } from 'node:test'
import assert from 'node:assert'

// ---------------------------------------------------------------------------
// Shadow DOM mock infrastructure
// ---------------------------------------------------------------------------

let nextId = 0

class MockShadowRoot {
  constructor(host, mode) {
    this.host = host
    this.mode = mode
    this._children = []
    this._allElements = new Map() // selector -> element
  }

  get children() {
    return this._children
  }

  get childElementCount() {
    return this._children.length
  }

  querySelector(sel) {
    return this._allElements.get(sel) || null
  }

  querySelectorAll(sel) {
    // Return all elements matching the selector pattern
    const results = []
    for (const [key, el] of this._allElements) {
      if (key === sel || matchesSelector(el, sel)) {
        results.push(el)
      }
    }
    return results
  }

  getRootNode() {
    return this
  }
}

// Simple selector matching for test mocks
function matchesSelector(el, sel) {
  if (sel === '*') return true
  if (sel.startsWith('#') && el.id === sel.slice(1)) return true
  if (sel.startsWith('.') && el.className === sel.slice(1)) return true
  if (sel === el.tagName?.toLowerCase()) return true
  if (sel.startsWith('[') && sel.includes('=')) {
    const m = sel.match(/\[([^=]+)="([^"]+)"\]/)
    if (m && el.getAttribute && el.getAttribute(m[1]) === m[2]) return true
  }
  // Interactive selectors
  const interactiveSelectors = [
    'a[href]', 'button', 'input', 'select', 'textarea',
    '[role="button"]', '[role="link"]', '[role="tab"]', '[role="menuitem"]',
    '[contenteditable="true"]', '[onclick]', '[tabindex]'
  ]
  if (interactiveSelectors.includes(sel)) {
    const tag = el.tagName?.toLowerCase()
    if (sel === 'button' && tag === 'button') return true
    if (sel === 'input' && tag === 'input') return true
    if (sel === 'a[href]' && tag === 'a' && el.getAttribute('href')) return true
    if (sel.startsWith('[role=') && el.getAttribute) {
      const roleMatch = sel.match(/\[role="([^"]+)"\]/)
      if (roleMatch && el.getAttribute('role') === roleMatch[1]) return true
    }
  }
  return false
}

class MockHTMLElement {
  constructor(tag, props = {}) {
    this.tagName = tag.toUpperCase()
    this.id = props.id || ''
    this.className = props.className || ''
    this.textContent = props.textContent || ''
    this.innerText = props.innerText || props.textContent || ''
    this.isContentEditable = props.isContentEditable || false
    this.offsetParent = props.offsetParent !== undefined ? props.offsetParent : {}
    this.style = { position: props.position || '' }
    this.shadowRoot = null
    this._children = []
    this._parent = null
    this._rootNode = null
    this._attributes = props.attributes || {}
    this._uniqueId = nextId++
  }

  get children() {
    return this._children
  }

  get childElementCount() {
    return this._children.length
  }

  appendChild(child) {
    this._children.push(child)
    child._parent = this
    return child
  }

  getAttribute(name) {
    return this._attributes[name] || null
  }

  closest(sel) {
    // Walk up parents checking tag matches
    let node = this
    while (node) {
      const tag = node.tagName?.toLowerCase()
      if (sel.includes(tag)) return node
      if (sel.includes('[role="button"]') && node.getAttribute && node.getAttribute('role') === 'button') return node
      node = node._parent
    }
    return null
  }

  querySelector(sel) {
    return findInChildren(this._children, sel)
  }

  querySelectorAll(sel) {
    return findAllInChildren(this._children, sel)
  }

  matches(sel) {
    return matchesSelector(this, sel)
  }

  getRootNode() {
    if (this._rootNode) return this._rootNode
    let node = this
    while (node._parent) {
      node = node._parent
    }
    return node
  }

  getBoundingClientRect() {
    return { width: 100, height: 20, top: 0, left: 0 }
  }

  click() {}
  focus() {}
  scrollIntoView() {}
  setAttribute() {}
  dispatchEvent() { return true }
}

function findInChildren(children, sel) {
  for (const child of children) {
    if (matchesSelector(child, sel)) return child
    const found = findInChildren(child._children || [], sel)
    if (found) return found
  }
  return null
}

function findAllInChildren(children, sel) {
  const results = []
  for (const child of children) {
    if (matchesSelector(child, sel)) results.push(child)
    results.push(...findAllInChildren(child._children || [], sel))
  }
  return results
}

// Attach a mock shadow root to an element
function attachShadow(host, mode = 'open') {
  const shadow = new MockShadowRoot(host, mode)
  if (mode === 'open') {
    host.shadowRoot = shadow
  }
  return shadow
}

// Add element to a shadow root (registers it for querySelector)
function addToShadow(shadow, el, selectors = []) {
  shadow._children.push(el)
  el._parent = null // shadow children don't have a light DOM parent
  el._rootNode = shadow
  // Register for querySelector
  if (el.id) shadow._allElements.set(`#${el.id}`, el)
  if (el.tagName) shadow._allElements.set(el.tagName.toLowerCase(), el)
  for (const sel of selectors) {
    shadow._allElements.set(sel, el)
  }
  return el
}

// ---------------------------------------------------------------------------
// Global mocks
// ---------------------------------------------------------------------------

globalThis.HTMLElement = MockHTMLElement
globalThis.HTMLInputElement = class extends MockHTMLElement {}
globalThis.HTMLTextAreaElement = class extends MockHTMLElement {}
globalThis.HTMLSelectElement = class extends MockHTMLElement {}
globalThis.CSS = { escape: (s) => s }
globalThis.NodeFilter = { SHOW_TEXT: 4 }
globalThis.InputEvent = class extends Event {}
globalThis.KeyboardEvent = class extends Event {}
globalThis.ClipboardEvent = class extends Event {
  constructor(type, init = {}) { super(type, init); this.clipboardData = init.clipboardData || null }
}
globalThis.DataTransfer = class {
  constructor() { this._data = {} }
  setData(t, v) { this._data[t] = v }
  getData(t) { return this._data[t] || '' }
}
globalThis.ShadowRoot = MockShadowRoot
globalThis.getComputedStyle = () => ({ visibility: 'visible', display: 'block' })

class MockMutationObserver {
  constructor(cb) { this._cb = cb }
  observe() {}
  disconnect() {}
}
globalThis.MutationObserver = MockMutationObserver

let perfNowValue = 0
globalThis.performance = { now: () => perfNowValue++ }

// ---------------------------------------------------------------------------
// Import domPrimitive AFTER globals are set up
// ---------------------------------------------------------------------------
const { domPrimitive, domWaitFor, domPrimitiveListInteractive } = await import('./dom/dom-primitives.js')

// ---------------------------------------------------------------------------
// Helper: build a mock document with shadow DOM structure
//
// document
// ├── <div id="light-btn"><button id="light-button">Light</button></div>
// ├── <my-component id="comp1">
// │   └── #shadow-root (open)
// │       ├── <button id="shadow-btn">Shadow Button</button>
// │       └── <nested-comp id="nested1">
// │           └── #shadow-root (open)
// │               └── <input id="deep-input" placeholder="Deep Input">
// └── <another-component id="comp2">
//     └── #shadow-root (open)
//         └── <a id="shadow-link" href="/link">Shadow Link</a>
// ---------------------------------------------------------------------------

function setupShadowDocument() {
  perfNowValue = 0
  globalThis.MutationObserver = MockMutationObserver
  globalThis.requestAnimationFrame = (cb) => cb()

  // Light DOM elements
  const lightBtn = new MockHTMLElement('button', { id: 'light-button', textContent: 'Light' })
  const lightDiv = new MockHTMLElement('div', { id: 'light-btn' })
  lightDiv.appendChild(lightBtn)

  // Component 1 with nested shadow
  const comp1 = new MockHTMLElement('my-component', { id: 'comp1' })
  const shadow1 = attachShadow(comp1)
  const shadowBtn = new MockHTMLElement('button', { id: 'shadow-btn', textContent: 'Shadow Button' })
  addToShadow(shadow1, shadowBtn, ['button'])

  const nested1 = new MockHTMLElement('nested-comp', { id: 'nested1' })
  addToShadow(shadow1, nested1, ['nested-comp'])
  const shadow2 = attachShadow(nested1)
  const deepInput = new MockHTMLElement('input', {
    id: 'deep-input',
    attributes: { placeholder: 'Deep Input' }
  })
  addToShadow(shadow2, deepInput, ['input', '[placeholder="Deep Input"]'])

  // Component 2
  const comp2 = new MockHTMLElement('another-component', { id: 'comp2' })
  const shadow3 = attachShadow(comp2)
  const shadowLink = new MockHTMLElement('a', {
    id: 'shadow-link',
    textContent: 'Shadow Link',
    attributes: { href: '/link' }
  })
  addToShadow(shadow3, shadowLink, ['a', 'a[href]'])

  // TreeWalker mock — does NOT cross shadow boundaries (matches real browser behavior)
  function createTreeWalker(root) {
    const textNodes = []
    function collectText(node) {
      // Leaf elements with text become text nodes
      if (node.textContent && (node._children?.length === 0 || !node._children)) {
        textNodes.push({ textContent: node.textContent, parentElement: node })
      }
      // Walk light DOM children only — no shadow crossing
      for (const child of (node._children || [])) {
        collectText(child)
      }
    }
    if (root instanceof MockShadowRoot) {
      for (const child of root._children) collectText(child)
    } else if (root.children && !root._children) {
      // document.body mock — walk its children array
      for (const child of root.children) collectText(child)
    } else {
      collectText(root)
    }
    let idx = -1
    return {
      nextNode() {
        idx++
        if (idx < textNodes.length) {
          this.currentNode = textNodes[idx]
          return this.currentNode
        }
        return null
      },
      currentNode: null
    }
  }

  const allTopLevel = [lightDiv, comp1, comp2]

  globalThis.document = {
    children: allTopLevel,
    get childElementCount() { return allTopLevel.length },
    querySelector(sel) {
      // Only finds light DOM elements (simulates real browser behavior)
      if (sel === '#light-button') return lightBtn
      if (sel === '#light-btn') return lightDiv
      if (sel === '#comp1' || sel === 'my-component') return comp1
      if (sel === '#comp2' || sel === 'another-component') return comp2
      // Shadow elements are NOT found by document.querySelector
      if (sel === '#shadow-btn') return null
      if (sel === '#deep-input') return null
      if (sel === '#shadow-link') return null
      if (sel === '#nonexistent') return null
      return null
    },
    querySelectorAll(sel) {
      if (sel === '*') return allTopLevel
      if (sel === 'button') return [lightBtn]
      if (sel === 'label') return []
      if (sel === '[aria-label]') return []
      if (sel.startsWith('[aria-label="')) return []
      return []
    },
    body: {
      querySelectorAll(sel) { return globalThis.document.querySelectorAll(sel) },
      children: allTopLevel,
      get childElementCount() { return allTopLevel.length }
    },
    documentElement: {
      children: allTopLevel,
      get childElementCount() { return allTopLevel.length }
    },
    createTreeWalker(root) { return createTreeWalker(root) },
    getSelection: () => ({ selectAllChildren: () => {}, deleteFromDocument: () => {} }),
    execCommand: () => {},
    getElementById: (id) => globalThis.document.querySelector(`#${id}`)
  }

  return { lightBtn, lightDiv, comp1, comp2, shadowBtn, nested1, deepInput, shadowLink, shadow1, shadow2, shadow3 }
}

// ===========================================================================
// Phase 1: Core helpers
// ===========================================================================

describe('querySelectorDeep: fast path + shadow traversal', () => {
  beforeEach(() => setupShadowDocument())

  test('returns light DOM element on fast path (no shadow traversal)', () => {
    const result = domPrimitive('get_text', '#light-button', {})
    assert.strictEqual(result.success, true, 'Should find light DOM element')
  })

  test('finds element in first-level shadow root when fast path misses', () => {
    const result = domPrimitive('get_text', '#shadow-btn', {})
    assert.strictEqual(result.success, true, 'Should find element in shadow root')
    assert.ok(
      String(result.value).includes('Shadow Button'),
      `Should get shadow button text, got: ${result.value}`
    )
  })

  test('finds element in nested shadow root (2 levels deep)', () => {
    const result = domPrimitive('get_text', '#deep-input', {})
    assert.strictEqual(result.success, true, 'Should find element in nested shadow root')
  })

  test('returns element_not_found for non-existent selector', () => {
    const result = domPrimitive('get_text', '#nonexistent', {})
    assert.strictEqual(result.success, false)
    assert.strictEqual(result.error, 'element_not_found')
  })
})

describe('querySelectorAllDeep: find all across roots', () => {
  beforeEach(() => setupShadowDocument())

  test('list_interactive returns elements from both light DOM and shadow roots', () => {
    const result = domPrimitiveListInteractive('', {})
    assert.strictEqual(result.success, true)
    const elements = result.elements
    const tags = elements.map((e) => e.tag)

    assert.ok(tags.includes('button'), 'Should find light DOM button')
    // Shadow buttons should also be found
    assert.ok(
      elements.length > 1,
      `Should find elements across shadow roots, found ${elements.length}`
    )
  })
})

// ===========================================================================
// Phase 2: Semantic resolvers
// ===========================================================================

describe('resolveByTextDeep: text search across shadow roots', () => {
  beforeEach(() => setupShadowDocument())

  test('finds text in light DOM (unchanged behavior)', () => {
    const result = domPrimitive('get_text', 'text=Light', {})
    assert.strictEqual(result.success, true, 'Should find light DOM text')
  })

  test('finds text inside shadow root', () => {
    const result = domPrimitive('get_text', 'text=Shadow Button', {})
    assert.strictEqual(result.success, true, 'Should find text inside shadow root')
  })

  test('finds text inside another shadow root', () => {
    const result = domPrimitive('get_text', 'text=Shadow Link', {})
    assert.strictEqual(result.success, true, 'Should find text in second shadow root')
  })
})

// ===========================================================================
// Phase 3: Deep combinator >>>
// ===========================================================================

describe('deep combinator >>> syntax', () => {
  beforeEach(() => setupShadowDocument())

  test('resolves single-level: my-component >>> #shadow-btn', () => {
    const result = domPrimitive('get_text', 'my-component >>> #shadow-btn', {})
    assert.strictEqual(result.success, true, 'Should resolve >>> selector')
    assert.ok(
      String(result.value).includes('Shadow Button'),
      `Should get shadow button text, got: ${result.value}`
    )
  })

  test('resolves chained: my-component >>> nested-comp >>> #deep-input', () => {
    const result = domPrimitive('get_text', 'my-component >>> nested-comp >>> #deep-input', {})
    assert.strictEqual(result.success, true, 'Should resolve chained >>> selector')
  })

  test('returns error if host not found', () => {
    const result = domPrimitive('get_text', 'nonexistent-host >>> #shadow-btn', {})
    assert.strictEqual(result.success, false)
    assert.strictEqual(result.error, 'element_not_found')
  })

  test('returns error if host has no shadow root', () => {
    const result = domPrimitive('get_text', '#light-btn >>> button', {})
    assert.strictEqual(result.success, false)
    assert.strictEqual(result.error, 'element_not_found')
  })

  test('returns error if inner selector not found', () => {
    const result = domPrimitive('get_text', 'my-component >>> #nonexistent', {})
    assert.strictEqual(result.success, false)
    assert.strictEqual(result.error, 'element_not_found')
  })
})

// ===========================================================================
// Phase 4: Selector generation
// ===========================================================================

describe('selector generation for shadow elements', () => {
  beforeEach(() => setupShadowDocument())

  test('list_interactive generates >>> selectors for shadow elements', () => {
    const result = domPrimitiveListInteractive('', {})
    assert.strictEqual(result.success, true)

    const selectors = result.elements.map((e) => e.selector)
    const hasShadowSelector = selectors.some((s) => s.includes('>>>'))
    assert.ok(
      hasShadowSelector,
      `At least one selector should use >>>, got: ${JSON.stringify(selectors)}`
    )
  })

  test('light DOM element selector has no >>>', () => {
    const result = domPrimitiveListInteractive('', {})
    const lightEl = result.elements.find((e) => e.tag === 'button' && !e.selector.includes('>>>'))
    assert.ok(lightEl, 'Light DOM button should have selector without >>>')
  })
})

// ===========================================================================
// Phase 5: isVisible shadow fix
// ===========================================================================

describe('isVisible: shadow DOM fallback', () => {
  test('element with null offsetParent but non-zero rect is still visible', () => {
    const { shadowBtn } = setupShadowDocument()
    // Simulate shadow DOM offsetParent quirk
    shadowBtn.offsetParent = null
    shadowBtn.style = { position: '' }

    // The button should still be found and actionable
    const result = domPrimitive('get_text', '#shadow-btn', {})
    assert.strictEqual(result.success, true, 'Shadow element with null offsetParent should still be found')
  })
})

// ===========================================================================
// Phase 6: Actions work through shadow DOM
// ===========================================================================

describe('actions work on shadow DOM elements', () => {
  beforeEach(() => setupShadowDocument())

  test('click works on element inside shadow root', async () => {
    let clicked = false
    const { shadowBtn } = setupShadowDocument()
    shadowBtn.click = () => { clicked = true }

    const result = await domPrimitive('click', '#shadow-btn', {})
    assert.strictEqual(result.success, true)
    assert.strictEqual(clicked, true, 'Click should have been called on shadow element')
  })

  test('type works on element inside shadow root via >>> selector', async () => {
    const { deepInput } = setupShadowDocument()
    // Make it behave like an input
    Object.setPrototypeOf(deepInput, globalThis.HTMLInputElement.prototype)
    deepInput.value = ''
    deepInput.dispatchEvent = () => {}
    Object.defineProperty(globalThis.HTMLInputElement.prototype, 'value', {
      set(v) { this._value = v },
      get() { return this._value || '' },
      configurable: true
    })

    const result = await domPrimitive('type', 'my-component >>> nested-comp >>> #deep-input', { text: 'hello' })
    assert.strictEqual(result.success, true, `Type should work through >>> selector: ${JSON.stringify(result)}`)
  })
})

// ===========================================================================
// Regression: existing behavior preserved
// ===========================================================================

// ===========================================================================
// Phase 7: domWaitFor shadow DOM support
// ===========================================================================

describe('domWaitFor: shadow DOM support', () => {
  beforeEach(() => setupShadowDocument())

  test('resolves immediately for element inside shadow root', async () => {
    const result = await domWaitFor('#shadow-btn', 500)
    assert.strictEqual(result.success, true, 'Should find shadow element immediately')
    assert.strictEqual(result.value, 'button')
  })

  test('resolves immediately for element in nested shadow root', async () => {
    const result = await domWaitFor('#deep-input', 500)
    assert.strictEqual(result.success, true, 'Should find nested shadow element immediately')
    assert.strictEqual(result.value, 'input')
  })

  test('resolves immediately for >>> combinator', async () => {
    const result = await domWaitFor('my-component >>> #shadow-btn', 500)
    assert.strictEqual(result.success, true, 'Should resolve >>> in wait_for')
    assert.strictEqual(result.value, 'button')
  })

  test('resolves immediately for text= inside shadow root', async () => {
    const result = await domWaitFor('text=Shadow Button', 500)
    assert.strictEqual(result.success, true, 'Should find text inside shadow root')
  })

  test('resolves immediately for light DOM element (unchanged)', async () => {
    const result = await domWaitFor('#light-button', 500)
    assert.strictEqual(result.success, true, 'Should still find light DOM elements')
    assert.strictEqual(result.value, 'button')
  })

  test('times out for non-existent element', async () => {
    const result = await domWaitFor('#does-not-exist', 50)
    assert.strictEqual(result.success, false)
    assert.strictEqual(result.error, 'timeout')
  })

  test('detects element added to shadow root via MutationObserver', async () => {
    const { shadow1 } = setupShadowDocument()

    // Override MutationObserver AFTER setupShadowDocument so we capture the callback
    let observerCallback = null
    globalThis.MutationObserver = class {
      constructor(cb) { observerCallback = cb }
      observe() {}
      disconnect() {}
    }

    // Start waiting for an element that doesn't exist yet
    const waitPromise = domWaitFor('#new-shadow-el', 2000)

    // Simulate: element appears in shadow root after a tick
    await new Promise((r) => setTimeout(r, 10))
    const addedEl = new MockHTMLElement('button', { id: 'new-shadow-el', textContent: 'New' })
    addToShadow(shadow1, addedEl, ['button'])

    // Trigger the mutation observer callback
    assert.ok(observerCallback, 'MutationObserver callback should have been captured')
    observerCallback([{ type: 'childList' }])

    const result = await waitPromise
    assert.strictEqual(result.success, true, 'Should detect element added via mutation')
    assert.strictEqual(result.value, 'button')
  })
})

// ===========================================================================
// Regression: existing behavior preserved
// ===========================================================================

describe('regression: non-shadow pages unaffected', () => {
  test('standard selector resolves without deep traversal', () => {
    setupShadowDocument()
    const result = domPrimitive('get_text', '#light-button', {})
    assert.strictEqual(result.success, true)
  })

  test('element_not_found still returned for missing element', () => {
    setupShadowDocument()
    const result = domPrimitive('get_text', '#does-not-exist', {})
    assert.strictEqual(result.success, false)
    assert.strictEqual(result.error, 'element_not_found')
  })
})
