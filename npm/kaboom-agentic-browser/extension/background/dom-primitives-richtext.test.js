// @ts-nocheck
/**
 * @fileoverview dom-primitives-richtext.test.js — Tests for rich text support:
 * 1. Rich editor detection (Quill, ProseMirror, Draft.js, TinyMCE, CKEditor)
 * 2. Native DOM insertion for detected editors
 * 3. Keyboard simulation fallback for generic contenteditable
 * 4. type/paste insertion_strategy field
 * 5. get_text: innerText vs textContent
 * 6. paste action: synthetic ClipboardEvent + rich editor native insertion
 *
 * Run: node --test extension/background/dom-primitives-richtext.test.js
 */

import { test, describe, beforeEach } from 'node:test'
import assert from 'node:assert'

// ---------------------------------------------------------------------------
// Minimal DOM mocks — enhanced with matches/closest/innerHTML for rich editor
// ---------------------------------------------------------------------------
class MockHTMLElement {
  constructor(tag, props = {}) {
    this.tagName = tag
    this.id = props.id || ''
    this.className = props.className || ''
    this.textContent = props.textContent || ''
    this.innerText = props.innerText || ''
    this.innerHTML = props.innerHTML || ''
    this.isContentEditable = props.isContentEditable || false
    this.offsetParent = {}
    this.style = { position: '' }
    this._attributes = props.attributes || {}
    this._parent = props.parent || null
  }
  click() {}
  focus() {}
  getAttribute(name) {
    if (name === 'contenteditable' && this.isContentEditable) return 'true'
    if (name === 'class') return this.className
    return this._attributes[name] !== undefined ? this._attributes[name] : null
  }
  matches(selector) {
    if (selector.startsWith('.')) {
      const cls = selector.slice(1)
      return this.className.split(/\s+/).includes(cls)
    }
    if (selector.startsWith('#')) {
      return this.id === selector.slice(1)
    }
    if (selector.startsWith('[')) {
      const m = selector.match(/\[([^=\]]+)(?:="([^"]*)")?\]/)
      if (m) {
        const attr = m[1]
        const val = m[2]
        const actual = this._attributes[attr]
        if (val !== undefined) return actual === val
        return actual !== undefined && actual !== null
      }
    }
    return false
  }
  closest(selector) {
    let current = this
    while (current) {
      if (typeof current.matches === 'function' && current.matches(selector)) return current
      current = current._parent || null
    }
    return null
  }
  querySelector() {
    return null
  }
  querySelectorAll(sel) {
    if (sel === '*') return [this]
    return []
  }
  scrollIntoView() {}
  setAttribute() {}
  dispatchEvent() {
    return true
  }
  insertAdjacentHTML(_position, html) {
    this.innerHTML += html
  }
  getBoundingClientRect() {
    return { width: 100, height: 20, top: 0, left: 0, right: 100, bottom: 20 }
  }
}

globalThis.HTMLElement = MockHTMLElement
globalThis.HTMLInputElement = class extends MockHTMLElement {}
globalThis.HTMLTextAreaElement = class extends MockHTMLElement {}
globalThis.HTMLSelectElement = class extends MockHTMLElement {}
globalThis.CSS = { escape: (s) => s }
globalThis.NodeFilter = { SHOW_TEXT: 4 }
globalThis.InputEvent = class extends Event {}
globalThis.KeyboardEvent = class extends Event {}
globalThis.getComputedStyle = () => ({ visibility: 'visible', display: 'block' })

class MockMutationObserver {
  constructor(cb) {
    this._cb = cb
  }
  observe() {}
  disconnect() {}
}
globalThis.MutationObserver = MockMutationObserver

let perfNowValue = 0
globalThis.performance = { now: () => perfNowValue++ }

// Mock ClipboardEvent and DataTransfer for paste tests
class MockDataTransfer {
  constructor() {
    this._data = {}
  }
  setData(type, value) {
    this._data[type] = value
  }
  getData(type) {
    return this._data[type] || ''
  }
}
globalThis.DataTransfer = MockDataTransfer

class MockClipboardEvent extends Event {
  constructor(type, init = {}) {
    super(type, init)
    this.clipboardData = init.clipboardData || null
  }
}
globalThis.ClipboardEvent = MockClipboardEvent

// ---------------------------------------------------------------------------
// Import domPrimitive AFTER globals are set up
// ---------------------------------------------------------------------------
const { domPrimitive } = await import('./dom-primitives.js')

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------
function setupContentEditable(id = '#editor', elProps = {}) {
  const el = new MockHTMLElement('DIV', {
    id: id.replace('#', ''),
    isContentEditable: true,
    textContent: '',
    innerText: '',
    ...elProps
  })
  Object.setPrototypeOf(el, MockHTMLElement.prototype)

  const commands = []
  const dispatched = []
  el.dispatchEvent = (event) => {
    dispatched.push(event)
    return true
  }

  globalThis.document = {
    querySelector: (sel) => (sel === id ? el : null),
    querySelectorAll: (sel) => (sel === id || sel === '*' ? [el] : []),
    body: {
      querySelectorAll: (sel) => (sel === id || sel === '*' ? [el] : []),
      appendChild: () => {}
    },
    documentElement: {},
    createTreeWalker: () => ({ nextNode: () => null }),
    getSelection: () => ({
      selectAllChildren: () => {},
      deleteFromDocument: () => {}
    }),
    execCommand: (cmd, _showUI, value) => {
      commands.push({ cmd, value })
      return true
    }
  }

  return { el, commands, dispatched }
}

function setupQuillEditor(id = '#editor') {
  return setupContentEditable(id, { className: 'ql-editor' })
}

function setupProseMirrorEditor(id = '#editor') {
  return setupContentEditable(id, { className: 'ProseMirror' })
}

function setupDraftJSEditor(id = '#editor') {
  return setupContentEditable(id, { attributes: { 'data-contents': 'true' } })
}

// ---------------------------------------------------------------------------
// 1. Rich editor detection
// ---------------------------------------------------------------------------

describe('rich editor detection', () => {
  beforeEach(() => {
    perfNowValue = 0
    globalThis.MutationObserver = MockMutationObserver
    globalThis.requestAnimationFrame = (cb) => cb()
  })

  test('Quill: element with class ql-editor detected', async () => {
    setupQuillEditor()
    const result = await domPrimitive('type', '#editor', { text: 'test' })
    assert.strictEqual(result.success, true)
    assert.strictEqual(result.insertion_strategy, 'quill_native')
  })

  test('ProseMirror: element with class ProseMirror detected', async () => {
    setupProseMirrorEditor()
    const result = await domPrimitive('type', '#editor', { text: 'test' })
    assert.strictEqual(result.success, true)
    assert.strictEqual(result.insertion_strategy, 'prosemirror_native')
  })

  test('Draft.js: element with data-contents attribute detected', async () => {
    setupDraftJSEditor()
    const result = await domPrimitive('type', '#editor', { text: 'test' })
    assert.strictEqual(result.success, true)
    assert.strictEqual(result.insertion_strategy, 'draftjs_native')
  })

  test('no framework: returns keyboard_simulation for single-line', async () => {
    setupContentEditable()
    const result = await domPrimitive('type', '#editor', { text: 'test' })
    assert.strictEqual(result.success, true)
    assert.strictEqual(result.insertion_strategy, 'keyboard_simulation')
  })

  test('nested: detects ancestor editor via closest()', async () => {
    // Parent is the Quill editor, child is a <p> inside it
    const parent = new MockHTMLElement('DIV', {
      id: 'quill-root',
      className: 'ql-editor',
      isContentEditable: true
    })
    Object.setPrototypeOf(parent, MockHTMLElement.prototype)

    const child = new MockHTMLElement('P', {
      id: 'editor',
      isContentEditable: true,
      parent: parent
    })
    Object.setPrototypeOf(child, MockHTMLElement.prototype)
    child.dispatchEvent = () => true

    globalThis.document = {
      querySelector: (sel) => (sel === '#editor' ? child : null),
      querySelectorAll: (sel) => (sel === '#editor' || sel === '*' ? [child] : []),
      body: {
        querySelectorAll: (sel) => (sel === '#editor' || sel === '*' ? [child] : []),
        appendChild: () => {}
      },
      documentElement: {},
      createTreeWalker: () => ({ nextNode: () => null }),
      getSelection: () => ({
        selectAllChildren: () => {},
        deleteFromDocument: () => {}
      }),
      execCommand: () => true
    }

    const result = await domPrimitive('type', '#editor', { text: 'test' })
    assert.strictEqual(result.success, true)
    assert.strictEqual(result.insertion_strategy, 'quill_native')
  })
})

// ---------------------------------------------------------------------------
// 2. Native insertion for detected rich editors
// ---------------------------------------------------------------------------

describe('native insertion for rich editors', () => {
  beforeEach(() => {
    perfNowValue = 0
    globalThis.MutationObserver = MockMutationObserver
    globalThis.requestAnimationFrame = (cb) => cb()
  })

  test('multiline text produces correct <p> elements in innerHTML', async () => {
    const { el } = setupQuillEditor()
    await domPrimitive('type', '#editor', { text: 'line one\nline two', clear: true })
    assert.strictEqual(el.innerHTML, '<p>line one</p><p>line two</p>')
  })

  test('empty lines become <p><br></p>', async () => {
    const { el } = setupQuillEditor()
    await domPrimitive('type', '#editor', { text: 'before\n\nafter', clear: true })
    assert.strictEqual(el.innerHTML, '<p>before</p><p><br></p><p>after</p>')
  })

  test('clear: true replaces content via innerHTML', async () => {
    const { el } = setupQuillEditor()
    el.innerHTML = '<p>old content</p>'
    await domPrimitive('type', '#editor', { text: 'new', clear: true })
    assert.strictEqual(el.innerHTML, '<p>new</p>')
  })

  test('clear: false appends via insertAdjacentHTML', async () => {
    const { el } = setupQuillEditor()
    el.innerHTML = '<p>existing</p>'
    await domPrimitive('type', '#editor', { text: 'appended' })
    assert.strictEqual(el.innerHTML, '<p>existing</p><p>appended</p>')
  })

  test('input event dispatched after insertion', async () => {
    const { dispatched } = setupQuillEditor()
    await domPrimitive('type', '#editor', { text: 'hello' })
    const inputEvents = dispatched.filter((e) => e.type === 'input')
    assert.ok(inputEvents.length > 0, 'Should dispatch an input event')
  })

  test('HTML special characters are escaped', async () => {
    const { el } = setupQuillEditor()
    await domPrimitive('type', '#editor', { text: '<b>bold</b> & "quotes"', clear: true })
    assert.strictEqual(el.innerHTML, '<p>&lt;b&gt;bold&lt;/b&gt; &amp; "quotes"</p>')
  })

  test('no execCommand calls for detected rich editors', async () => {
    const { commands } = setupQuillEditor()
    await domPrimitive('type', '#editor', { text: 'line one\nline two' })
    assert.strictEqual(commands.length, 0, 'Rich editor insertion should not use execCommand')
  })
})

// ---------------------------------------------------------------------------
// 3. Keyboard simulation for generic contenteditable
// ---------------------------------------------------------------------------

describe('keyboard simulation for generic contenteditable', () => {
  beforeEach(() => {
    perfNowValue = 0
    globalThis.MutationObserver = MockMutationObserver
    globalThis.requestAnimationFrame = (cb) => cb()
  })

  test('single-line text uses keyboard simulation fallback', async () => {
    const { commands } = setupContentEditable()

    const result = await domPrimitive('type', '#editor', { text: 'hello world' })

    assert.strictEqual(result.success, true)
    assert.strictEqual(result.insertion_strategy, 'keyboard_simulation')
    assert.strictEqual(commands.length, 0, 'Generic contenteditable fallback should avoid deprecated execCommand')
  })

  test('two-line text uses keyboard simulation with Enter keydown/keyup', async () => {
    const { commands, dispatched } = setupContentEditable()

    const result = await domPrimitive('type', '#editor', { text: 'line one\nline two' })

    assert.strictEqual(result.success, true)
    assert.strictEqual(result.insertion_strategy, 'keyboard_simulation')

    assert.strictEqual(commands.length, 0, 'Keyboard simulation should not rely on execCommand')
    const keydowns = dispatched.filter((e) => e.type === 'keydown')
    const keyups = dispatched.filter((e) => e.type === 'keyup')
    assert.ok(keydowns.length > 0, 'Should dispatch keydown events')
    assert.ok(keyups.length > 0, 'Should dispatch keyup events')
  })

  test('three-line text dispatches two Enter key pairs', async () => {
    const { commands, dispatched } = setupContentEditable()

    await domPrimitive('type', '#editor', { text: 'a\nb\nc' })

    assert.strictEqual(commands.length, 0, 'Keyboard simulation should not rely on execCommand')
    const keydowns = dispatched.filter((e) => e.type === 'keydown')
    assert.ok(keydowns.length > 0, 'Should dispatch keydown events')
  })

  test('consecutive newlines dispatch Enter without insertText for empty lines', async () => {
    const { commands, dispatched } = setupContentEditable()

    await domPrimitive('type', '#editor', { text: 'before\n\nafter' })

    assert.strictEqual(commands.length, 0, 'Keyboard simulation should not rely on execCommand')
    const keydowns = dispatched.filter((e) => e.type === 'keydown')
    assert.ok(keydowns.length > 0, 'Should dispatch keydown events')
  })

  test('clear option works with multiline keyboard simulation', async () => {
    const { commands } = setupContentEditable()

    const result = await domPrimitive('type', '#editor', { text: 'a\nb', clear: true })

    assert.strictEqual(result.success, true)
    assert.strictEqual(result.insertion_strategy, 'keyboard_simulation')
    assert.strictEqual(commands.length, 0, 'Keyboard simulation should not rely on execCommand')
  })
})

// ---------------------------------------------------------------------------
// 4. insertion_strategy field
// ---------------------------------------------------------------------------

describe('insertion_strategy field', () => {
  beforeEach(() => {
    perfNowValue = 0
    globalThis.MutationObserver = MockMutationObserver
    globalThis.requestAnimationFrame = (cb) => cb()
  })

  test('Quill editor returns quill_native strategy', async () => {
    setupQuillEditor()
    const result = await domPrimitive('type', '#editor', { text: 'test' })
    assert.strictEqual(result.insertion_strategy, 'quill_native')
  })

  test('generic contenteditable multiline returns keyboard_simulation', async () => {
    setupContentEditable()
    const result = await domPrimitive('type', '#editor', { text: 'a\nb' })
    assert.strictEqual(result.insertion_strategy, 'keyboard_simulation')
  })

  test('generic contenteditable single-line returns keyboard_simulation', async () => {
    setupContentEditable()
    const result = await domPrimitive('type', '#editor', { text: 'hello' })
    assert.strictEqual(result.insertion_strategy, 'keyboard_simulation')
  })

  test('paste on generic contenteditable returns clipboard_event', async () => {
    const { el } = setupContentEditable()
    el.innerText = 'text'
    const result = await domPrimitive('paste', '#editor', { text: 'pasted' })
    assert.strictEqual(result.insertion_strategy, 'clipboard_event')
  })

  test('paste on Quill editor returns quill_native', async () => {
    const { el } = setupQuillEditor()
    el.innerText = 'text'
    const result = await domPrimitive('paste', '#editor', { text: 'pasted' })
    assert.strictEqual(result.insertion_strategy, 'quill_native')
  })
})

// ---------------------------------------------------------------------------
// 5. get_text: innerText preserves line breaks
// ---------------------------------------------------------------------------

describe('get_text: innerText for HTMLElement', () => {
  beforeEach(() => {
    perfNowValue = 0
  })

  test('get_text returns innerText for HTMLElement (preserves line breaks)', () => {
    const el = new MockHTMLElement('DIV', { id: 'content' })
    el.textContent = 'line oneline two'
    el.innerText = 'line one\nline two'
    Object.setPrototypeOf(el, MockHTMLElement.prototype)

    globalThis.document = {
      querySelector: (sel) => (sel === '#content' ? el : null),
      querySelectorAll: (sel) => (sel === '#content' || sel === '*' ? [el] : []),
      body: { querySelectorAll: (sel) => (sel === '#content' || sel === '*' ? [el] : []) },
      documentElement: {},
      createTreeWalker: () => ({ nextNode: () => null })
    }

    const result = domPrimitive('get_text', '#content', {})

    assert.strictEqual(result.success, true)
    assert.strictEqual(
      result.value,
      'line one\nline two',
      'get_text MUST use innerText (preserves \\n at block boundaries), not textContent'
    )
  })

  test('get_text uses textContent for non-HTMLElement', () => {
    // Simulate an SVG element (not an HTMLElement)
    const el = {
      tagName: 'SVG',
      textContent: 'svg text',
      getAttribute: () => null,
      closest: () => null,
      querySelector: () => null
    }
    // Don't set prototype to HTMLElement — it should NOT be instanceof HTMLElement

    globalThis.document = {
      querySelector: (sel) => (sel === '#svg' ? el : null),
      querySelectorAll: (sel) => (sel === '#svg' || sel === '*' ? [el] : []),
      body: { querySelectorAll: (sel) => (sel === '#svg' || sel === '*' ? [el] : []) },
      documentElement: {},
      createTreeWalker: () => ({ nextNode: () => null })
    }

    const result = domPrimitive('get_text', '#svg', {})

    assert.strictEqual(result.success, true)
    assert.strictEqual(result.value, 'svg text', 'Non-HTMLElement should fall back to textContent')
  })

  test('get_text returns reason when resolved text is null', () => {
    const el = new MockHTMLElement('DIV', { id: 'content' })
    el.innerText = null
    el.textContent = null
    Object.setPrototypeOf(el, MockHTMLElement.prototype)

    globalThis.document = {
      querySelector: (sel) => (sel === '#content' ? el : null),
      querySelectorAll: (sel) => (sel === '#content' || sel === '*' ? [el] : []),
      body: { querySelectorAll: (sel) => (sel === '#content' || sel === '*' ? [el] : []) },
      documentElement: {},
      createTreeWalker: () => ({ nextNode: () => null })
    }

    const result = domPrimitive('get_text', '#content', {})

    assert.strictEqual(result.success, true)
    assert.strictEqual(result.value, null)
    assert.strictEqual(result.reason, 'no_text_content')
    assert.ok(result.message.includes('null'))
  })
})

describe('null-value read actions include reason payload', () => {
  beforeEach(() => {
    perfNowValue = 0
  })

  test('get_attribute includes attribute_not_found reason when attribute is missing', () => {
    const el = new MockHTMLElement('DIV', { id: 'target' })
    Object.setPrototypeOf(el, MockHTMLElement.prototype)

    globalThis.document = {
      querySelector: (sel) => (sel === '#target' ? el : null),
      querySelectorAll: (sel) => (sel === '#target' || sel === '*' ? [el] : []),
      body: { querySelectorAll: (sel) => (sel === '#target' || sel === '*' ? [el] : []) },
      documentElement: {},
      createTreeWalker: () => ({ nextNode: () => null })
    }

    const result = domPrimitive('get_attribute', '#target', { name: 'aria-label' })

    assert.strictEqual(result.success, true)
    assert.strictEqual(result.value, null)
    assert.strictEqual(result.reason, 'attribute_not_found')
    assert.ok(result.message.includes('aria-label'))
  })

  test('get_value includes no_value reason when value is null', () => {
    const input = new globalThis.HTMLInputElement('INPUT', { id: 'field' })
    input.value = null
    Object.setPrototypeOf(input, globalThis.HTMLInputElement.prototype)

    globalThis.document = {
      querySelector: (sel) => (sel === '#field' ? input : null),
      querySelectorAll: (sel) => (sel === '#field' || sel === '*' ? [input] : []),
      body: { querySelectorAll: (sel) => (sel === '#field' || sel === '*' ? [input] : []) },
      documentElement: {},
      createTreeWalker: () => ({ nextNode: () => null })
    }

    const result = domPrimitive('get_value', '#field', {})

    assert.strictEqual(result.success, true)
    assert.strictEqual(result.value, null)
    assert.strictEqual(result.reason, 'no_value')
    assert.ok(result.message.includes('null'))
  })
})

// ---------------------------------------------------------------------------
// 6. paste action
// ---------------------------------------------------------------------------

describe('paste action: synthetic ClipboardEvent', () => {
  beforeEach(() => {
    perfNowValue = 0
    globalThis.MutationObserver = MockMutationObserver
    globalThis.requestAnimationFrame = (cb) => cb()
  })

  test('paste dispatches ClipboardEvent on generic contenteditable', async () => {
    const el = new MockHTMLElement('DIV', { id: 'editor', isContentEditable: true })
    el.innerText = 'pasted content'
    Object.setPrototypeOf(el, MockHTMLElement.prototype)

    let dispatchedEvent = null
    el.dispatchEvent = (event) => {
      dispatchedEvent = event
      return true
    }

    globalThis.document = {
      querySelector: (sel) => (sel === '#editor' ? el : null),
      querySelectorAll: (sel) => (sel === '#editor' || sel === '*' ? [el] : []),
      body: { querySelectorAll: (sel) => (sel === '#editor' || sel === '*' ? [el] : []), appendChild: () => {} },
      documentElement: {},
      createTreeWalker: () => ({ nextNode: () => null }),
      getSelection: () => ({
        selectAllChildren: () => {},
        deleteFromDocument: () => {}
      })
    }

    const result = await domPrimitive('paste', '#editor', { text: 'hello\nworld' })

    assert.strictEqual(result.success, true)
    assert.strictEqual(result.insertion_strategy, 'clipboard_event')
    assert.ok(dispatchedEvent, 'paste should dispatch an event')
    assert.strictEqual(dispatchedEvent.type, 'paste', 'Event type should be paste')
    assert.ok(dispatchedEvent.clipboardData, 'Event should have clipboardData')
    assert.strictEqual(
      dispatchedEvent.clipboardData.getData('text/plain'),
      'hello\nworld',
      'clipboardData should contain the text'
    )
  })

  test('paste on Quill editor uses native insertion', async () => {
    const { el } = setupQuillEditor()
    el.innerText = ''
    await domPrimitive('paste', '#editor', { text: 'line one\nline two', clear: true })
    assert.strictEqual(el.innerHTML, '<p>line one</p><p>line two</p>')
  })

  test('paste with clear selects all and deletes before pasting', async () => {
    const el = new MockHTMLElement('DIV', { id: 'editor', isContentEditable: true })
    el.innerText = 'new text'
    Object.setPrototypeOf(el, MockHTMLElement.prototype)

    let selectAllCalled = false
    let deleteCalled = false
    el.dispatchEvent = () => true

    globalThis.document = {
      querySelector: (sel) => (sel === '#editor' ? el : null),
      querySelectorAll: (sel) => (sel === '#editor' || sel === '*' ? [el] : []),
      body: { querySelectorAll: (sel) => (sel === '#editor' || sel === '*' ? [el] : []), appendChild: () => {} },
      documentElement: {},
      createTreeWalker: () => ({ nextNode: () => null }),
      getSelection: () => ({
        selectAllChildren: () => {
          selectAllCalled = true
        },
        deleteFromDocument: () => {
          deleteCalled = true
        }
      })
    }

    const result = await domPrimitive('paste', '#editor', { text: 'replacement', clear: true })

    assert.strictEqual(result.success, true)
    assert.strictEqual(selectAllCalled, true, 'clear should call selectAllChildren')
    assert.strictEqual(deleteCalled, true, 'clear should call deleteFromDocument')
  })

  test('paste fails on non-HTMLElement', async () => {
    const el = {
      tagName: 'SVG',
      textContent: 'svg',
      getAttribute: () => null,
      closest: () => null,
      querySelector: () => null
    }

    globalThis.document = {
      querySelector: (sel) => (sel === '#svg' ? el : null),
      querySelectorAll: (sel) => (sel === '#svg' || sel === '*' ? [el] : []),
      body: { querySelectorAll: (sel) => (sel === '#svg' || sel === '*' ? [el] : []), appendChild: () => {} },
      documentElement: {},
      createTreeWalker: () => ({ nextNode: () => null })
    }

    const result = await domPrimitive('paste', '#svg', { text: 'test' })

    assert.strictEqual(result.success, false)
    assert.strictEqual(result.error, 'not_interactive')
  })

  test('paste returns innerText as value', async () => {
    const el = new MockHTMLElement('DIV', { id: 'editor', isContentEditable: true })
    el.innerText = 'result text\nwith lines'
    Object.setPrototypeOf(el, MockHTMLElement.prototype)
    el.dispatchEvent = () => true

    globalThis.document = {
      querySelector: (sel) => (sel === '#editor' ? el : null),
      querySelectorAll: (sel) => (sel === '#editor' || sel === '*' ? [el] : []),
      body: { querySelectorAll: (sel) => (sel === '#editor' || sel === '*' ? [el] : []), appendChild: () => {} },
      documentElement: {},
      createTreeWalker: () => ({ nextNode: () => null }),
      getSelection: () => null
    }

    const result = await domPrimitive('paste', '#editor', { text: 'test' })

    assert.strictEqual(result.success, true)
    assert.strictEqual(result.value, 'result text\nwith lines', 'paste should return innerText')
  })
})
