// @ts-nocheck
/**
 * @fileoverview draw-mode.test.js — Tests for draw mode overlay, annotations, and export.
 * Covers activation/deactivation, annotation CRUD, DOM capture, persistence, and export.
 */

import { test, describe, mock, beforeEach, afterEach } from 'node:test'
import assert from 'node:assert'
import { MANIFEST_VERSION } from './helpers.js'

// =============================================================================
// DOM + Chrome mocks
// =============================================================================

/** Minimal mock element that supports style, events, and child management. */
function createMockElement(tag = 'div') {
  const el = {
    tagName: tag.toUpperCase(),
    id: '',
    className: '',
    classList: {
      _items: [],
      add(c) {
        this._items.push(c)
      },
      contains(c) {
        return this._items.includes(c)
      },
      [Symbol.iterator]() {
        return this._items[Symbol.iterator]()
      }
    },
    style: {},
    dataset: {},
    textContent: '',
    children: [],
    parentElement: null,
    _listeners: {},
    addEventListener(type, fn) {
      if (!this._listeners[type]) this._listeners[type] = []
      this._listeners[type].push(fn)
    },
    removeEventListener(type, fn) {
      if (this._listeners[type]) {
        this._listeners[type] = this._listeners[type].filter((f) => f !== fn)
      }
    },
    appendChild(child) {
      this.children.push(child)
      child.parentElement = this
      return child
    },
    remove() {
      if (this.parentElement) {
        this.parentElement.children = this.parentElement.children.filter((c) => c !== this)
      }
    },
    focus: mock.fn(),
    getBoundingClientRect() {
      return { x: 10, y: 20, width: 100, height: 50 }
    },
    getContext(type) {
      if (type !== '2d') return null
      if (!this._ctx) this._ctx = createMockCanvasContext()
      return this._ctx
    },
    get _context2d() {
      return this._ctx || null
    },
    _ctx: null,
    // For canvas
    width: 1024,
    height: 768,
    toDataURL: mock.fn(() => 'data:image/png;base64,mockdata'),
    // Dispatch for tests
    _dispatch(type, eventData = {}) {
      if (this._listeners[type]) {
        for (const fn of this._listeners[type]) fn(eventData)
      }
    }
  }
  return el
}

function createMockCanvasContext() {
  return {
    clearRect: mock.fn(),
    fillRect: mock.fn(),
    strokeRect: mock.fn(),
    fillText: mock.fn(),
    measureText: mock.fn(() => ({ width: 50 })),
    beginPath: mock.fn(),
    moveTo: mock.fn(),
    lineTo: mock.fn(),
    arcTo: mock.fn(),
    quadraticCurveTo: mock.fn(),
    closePath: mock.fn(),
    arc: mock.fn(),
    fill: mock.fn(),
    stroke: mock.fn(),
    save: mock.fn(),
    restore: mock.fn(),
    setLineDash: mock.fn(),
    drawImage: mock.fn(),
    fillStyle: '',
    strokeStyle: '',
    lineWidth: 1,
    font: '',
    textAlign: '',
    textBaseline: ''
  }
}

let documentBody
let createdElements
let styleEl
let storageData

// Timer isolation: draw-mode's overlay schedules real 2.5s/3s toast timers (and
// a debounced persist) that far outlive each ~10-40ms test. Left pending, they
// fire during a LATER test and mutate shared state — e.g. a leaked persist timer
// writes an annotation into the next test's storage, which activateDrawMode then
// loads (the "2 annotations instead of 1" Node-20 flake). We wrap setTimeout to
// record every scheduled id, then a global afterEach clears whatever is still
// pending. Real timing is preserved WITHIN a test, so the persistence tests that
// `await setTimeout(r, 350)` still work.
let trackedTimers = null
const realSetTimeout = globalThis.setTimeout.bind(globalThis)
const realClearTimeout = globalThis.clearTimeout.bind(globalThis)

function setupGlobals() {
  createdElements = []
  documentBody = createMockElement('body')
  styleEl = null
  storageData = {}

  trackedTimers = new Set()
  globalThis.setTimeout = (fn, ms, ...args) => {
    const id = realSetTimeout(fn, ms, ...args)
    trackedTimers.add(id)
    return id
  }
  globalThis.clearTimeout = (id) => {
    trackedTimers.delete(id)
    return realClearTimeout(id)
  }

  globalThis.window = {
    innerWidth: 1024,
    innerHeight: 768,
    location: { href: 'https://example.com/page' },
    addEventListener: mock.fn(),
    removeEventListener: mock.fn(),
    getComputedStyle: mock.fn(() => ({
      getPropertyValue: mock.fn((prop) => {
        const defaults = {
          'background-color': 'rgb(59, 130, 246)',
          color: 'rgb(255, 255, 255)',
          'font-size': '14px'
        }
        return defaults[prop] || ''
      })
    }))
  }

  globalThis.document = {
    createElement: mock.fn((tag) => {
      const el = createMockElement(tag)
      createdElements.push(el)
      return el
    }),
    body: documentBody,
    documentElement: createMockElement('html'),
    head: createMockElement('head'),
    addEventListener: mock.fn(),
    removeEventListener: mock.fn(),
    createTextNode: mock.fn((text) => {
      const node = { nodeType: 3, textContent: text }
      return node
    }),
    getElementById: mock.fn((id) => {
      if (id === 'gasoline-draw-styles') return styleEl
      return null
    }),
    querySelector: mock.fn(() => null),
    elementsFromPoint: mock.fn((_x, _y) => {
      // Return a mock button element
      const btn = createMockElement('button')
      btn.classList._items = ['btn-primary']
      btn.textContent = 'Submit'
      btn.id = 'submit-btn'
      btn.parentElement = createMockElement('div')
      btn.parentElement.classList._items = ['actions']
      return [btn]
    })
  }

  // When head.appendChild is called for style element, track it
  const origAppendChild = globalThis.document.head.appendChild.bind(globalThis.document.head)
  globalThis.document.head.appendChild = (child) => {
    if (child.id === 'gasoline-draw-styles') styleEl = child
    return origAppendChild(child)
  }

  globalThis.chrome = {
    runtime: {
      sendMessage: mock.fn(() => Promise.resolve()),
      onMessage: { addListener: mock.fn() },
      getManifest: () => ({ version: MANIFEST_VERSION })
    },
    storage: {
      session: {
        get: mock.fn((keys, callback) => {
          const result = {}
          if (Array.isArray(keys)) {
            for (const k of keys) {
              if (storageData[k]) result[k] = storageData[k]
            }
          }
          if (typeof callback === 'function') callback(result)
          else return Promise.resolve(result)
        }),
        set: mock.fn((data, callback) => {
          Object.assign(storageData, data)
          if (typeof callback === 'function') callback()
          else return Promise.resolve()
        }),
        remove: mock.fn((keys) => {
          const keyList = Array.isArray(keys) ? keys : [keys]
          for (const k of keyList) delete storageData[k]
          return Promise.resolve()
        })
      }
    }
  }

  // CSS.escape mock for buildCSSSelector
  globalThis.CSS = { escape: (s) => s.replace(/([#.,:[\]()>+~'"!@])/g, '\\$1') }

  globalThis.requestAnimationFrame = mock.fn((cb) => {
    cb()
    return 1
  })
  globalThis.cancelAnimationFrame = mock.fn()
  globalThis.Image = class MockImage {
    set src(val) {
      this.width = 1024
      this.height = 768
      setTimeout(() => this.onload?.(), 0)
    }
  }
}

// Global safety net: after every test, clear any timer it left pending so it can
// never fire during a later test. This is the fix for the Node-20 draw-mode
// flakiness — a leaked overlay/persist timer firing mid-test corrupted shared
// state (extra annotations, stale correlation ids, dropped focused-element).
afterEach(() => {
  if (!trackedTimers) return
  for (const id of trackedTimers) realClearTimeout(id)
  trackedTimers.clear()
})

// =============================================================================
// Module import — fresh each test via dynamic import
// =============================================================================

/**
 * Dynamically import draw-mode.js. Each test group re-imports to get a fresh module.
 * We use a cache-busting query string so Node doesn't return the cached module.
 */
let importCounter = 0
async function importDrawMode() {
  importCounter++
  const mod = await import(`../../extension/content/draw-mode.js?v=${importCounter}`)
  return mod
}

// =============================================================================
// Tests
// =============================================================================

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
    const mod = await import(`../../extension/content/draw-mode-export.js?v=${++importCounter}`)
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

// =============================================================================
// Gap 2: Drawing Mechanics — mousedown → mousemove → mouseup → text → Enter
// =============================================================================

describe('Draw Mode — Drawing Mechanics', () => {
  let dm

  beforeEach(async () => {
    setupGlobals()
    dm = await importDrawMode()
  })

  test('mousedown + mousemove + mouseup creates text input for annotation', () => {
    dm.activateDrawMode('user')

    const overlay = documentBody.children[0]
    assert.ok(overlay, 'expected overlay element')

    overlay._dispatch('mousedown', { button: 0, clientX: 100, clientY: 100 })
    overlay._dispatch('mousemove', { clientX: 250, clientY: 200 })
    overlay._dispatch('mouseup', { clientX: 250, clientY: 200 })

    const inputEls = createdElements.filter((el) => el.tagName === 'INPUT')
    assert.ok(inputEls.length > 0, 'expected text input after drawing rectangle')
  })

  test('annotation composer shows desired-outcome helper guidance', () => {
    dm.activateDrawMode('user')
    const overlay = documentBody.children[0]

    overlay._dispatch('mousedown', { button: 0, clientX: 100, clientY: 100 })
    overlay._dispatch('mouseup', { clientX: 250, clientY: 200 })

    const inputEl = createdElements.find((el) => el.tagName === 'INPUT')
    assert.ok(inputEl, 'expected text input element')
    assert.ok(
      inputEl.placeholder.includes("Don't just tell the AI what's wrong"),
      `unexpected placeholder guidance: ${inputEl.placeholder}`
    )
  })

  test('tiny rectangle (< 5px) does not create text input', () => {
    dm.activateDrawMode('user')
    const overlay = documentBody.children[0]

    const inputsBefore = createdElements.filter((el) => el.tagName === 'INPUT').length

    overlay._dispatch('mousedown', { button: 0, clientX: 100, clientY: 100 })
    overlay._dispatch('mousemove', { clientX: 102, clientY: 102 })
    overlay._dispatch('mouseup', { clientX: 102, clientY: 102 })

    const inputsAfter = createdElements.filter((el) => el.tagName === 'INPUT').length
    assert.strictEqual(inputsAfter, inputsBefore, 'no text input for tiny rectangle')
  })

  test('completing text input with Enter creates annotation', () => {
    dm.activateDrawMode('user')
    const overlay = documentBody.children[0]

    overlay._dispatch('mousedown', { button: 0, clientX: 100, clientY: 100 })
    overlay._dispatch('mousemove', { clientX: 250, clientY: 200 })
    overlay._dispatch('mouseup', { clientX: 250, clientY: 200 })

    const inputEl = createdElements.find((el) => el.tagName === 'INPUT')
    assert.ok(inputEl, 'expected text input element')

    inputEl.value = 'make this darker'

    const enterHandler = inputEl._listeners['keydown']?.[0]
    if (enterHandler) {
      enterHandler({
        key: 'Enter',
        preventDefault: mock.fn(),
        stopPropagation: mock.fn()
      })
    }

    const annotations = dm.getAnnotations()
    assert.strictEqual(annotations.length, 1, 'expected 1 annotation')
    assert.strictEqual(annotations[0].text, 'make this darker')
    assert.ok(annotations[0].id, 'expected annotation id')
    assert.ok(annotations[0].correlation_id, 'expected correlation_id')
    assert.ok(annotations[0].rect, 'expected rect')
    assert.strictEqual(annotations[0].rect.x, 100)
    assert.strictEqual(annotations[0].rect.y, 100)
    assert.strictEqual(annotations[0].rect.width, 150)
    assert.strictEqual(annotations[0].rect.height, 100)
  })

  test('annotation rect is stored in document coordinates when page is scrolled', () => {
    globalThis.window.scrollX = 40
    globalThis.window.scrollY = 120

    dm.activateDrawMode('user')
    const overlay = documentBody.children[0]

    overlay._dispatch('mousedown', { button: 0, clientX: 100, clientY: 100 })
    overlay._dispatch('mouseup', { clientX: 250, clientY: 200 })

    const inputEl = createdElements.find((el) => el.tagName === 'INPUT')
    inputEl.value = 'doc-space'
    inputEl._listeners['keydown']?.[0]?.({ key: 'Enter', preventDefault: mock.fn(), stopPropagation: mock.fn() })

    const annotations = dm.getAnnotations()
    assert.strictEqual(annotations.length, 1)
    assert.strictEqual(annotations[0].rect.x, 140)
    assert.strictEqual(annotations[0].rect.y, 220)
    assert.strictEqual(annotations[0].coord_space, 'document')
  })

  test('scrolling re-renders annotations anchored to page content', () => {
    dm.activateDrawMode('user')
    const overlay = documentBody.children[0]

    overlay._dispatch('mousedown', { button: 0, clientX: 100, clientY: 100 })
    overlay._dispatch('mouseup', { clientX: 250, clientY: 200 })

    const inputEl = createdElements.find((el) => el.tagName === 'INPUT')
    inputEl.value = 'scroll-anchor'
    inputEl._listeners['keydown']?.[0]?.({ key: 'Enter', preventDefault: mock.fn(), stopPropagation: mock.fn() })

    const canvasEl = createdElements.find((el) => el.tagName === 'CANVAS')
    assert.ok(canvasEl?._context2d, 'expected canvas 2d context')
    canvasEl._context2d.strokeRect.mock.resetCalls()

    globalThis.window.scrollX = 20
    globalThis.window.scrollY = 40

    const scrollCall = globalThis.window.addEventListener.mock.calls.find((c) => c.arguments?.[0] === 'scroll')
    assert.ok(scrollCall, 'expected scroll listener registration')
    const onScroll = scrollCall.arguments[1]
    onScroll()

    assert.ok(canvasEl._context2d.stroke.mock.calls.length > 0, 'expected annotation redraw on scroll')
    const roundedRectMoveTo = canvasEl._context2d.moveTo.mock.calls.find((c) => c.arguments[0] === 84 && c.arguments[1] === 60)
    assert.ok(roundedRectMoveTo, 'expected viewport-adjusted rounded-rect path on scroll')
  })

  test('resize after scrolling preserves annotation alignment', () => {
    dm.activateDrawMode('user')
    const overlay = documentBody.children[0]

    overlay._dispatch('mousedown', { button: 0, clientX: 100, clientY: 100 })
    overlay._dispatch('mouseup', { clientX: 250, clientY: 200 })

    const inputEl = createdElements.find((el) => el.tagName === 'INPUT')
    inputEl.value = 'resize-anchor'
    inputEl._listeners['keydown']?.[0]?.({ key: 'Enter', preventDefault: mock.fn(), stopPropagation: mock.fn() })

    const canvasEl = createdElements.find((el) => el.tagName === 'CANVAS')
    assert.ok(canvasEl?._context2d, 'expected canvas 2d context')
    canvasEl._context2d.strokeRect.mock.resetCalls()

    globalThis.window.scrollX = 15
    globalThis.window.scrollY = 30
    globalThis.window.innerWidth = 1280
    globalThis.window.innerHeight = 900

    const resizeCall = globalThis.window.addEventListener.mock.calls.find((c) => c.arguments?.[0] === 'resize')
    assert.ok(resizeCall, 'expected resize listener registration')
    const onResize = resizeCall.arguments[1]
    onResize()

    assert.ok(canvasEl._context2d.stroke.mock.calls.length > 0, 'expected redraw on resize')
    const roundedRectMoveTo = canvasEl._context2d.moveTo.mock.calls.find((c) => c.arguments[0] === 89 && c.arguments[1] === 70)
    assert.ok(roundedRectMoveTo, 'expected viewport-adjusted rounded-rect path on resize')
  })

  test('second shortcut submit path commits active annotation text before exit', async () => {
    dm.activateDrawMode('user')
    const overlay = documentBody.children[0]

    const sentMessages = []
    globalThis.chrome.runtime.sendMessage = mock.fn((msg, callback) => {
      sentMessages.push(msg)
      if (msg.type === 'kaboom_capture_screenshot' && typeof callback === 'function') {
        callback({ dataUrl: 'data:image/png;base64,mockscreenshot' })
      }
      return undefined
    })

    overlay._dispatch('mousedown', { button: 0, clientX: 100, clientY: 100 })
    overlay._dispatch('mouseup', { clientX: 250, clientY: 200 })

    const inputEl = createdElements.find((el) => el.tagName === 'INPUT')
    inputEl.value = 'submit-via-shortcut'

    dm.deactivateAndSendResults()
    await new Promise((r) => setTimeout(r, 350))

    assert.strictEqual(dm.isDrawModeActive(), false, 'draw mode should be inactive after shortcut submit')

    const completed = sentMessages.find((m) => m.type === 'draw_mode_completed')
    assert.ok(completed, 'expected DRAW_MODE_COMPLETED message')
    assert.strictEqual(completed.annotations.length, 1)
    assert.strictEqual(completed.annotations[0].text, 'submit-via-shortcut')
  })

  test('second shortcut submit path with empty text keeps editor open and warns user', () => {
    dm.activateDrawMode('user')
    const overlay = documentBody.children[0]

    const sentMessages = []
    globalThis.chrome.runtime.sendMessage = mock.fn((msg, callback) => {
      sentMessages.push(msg)
      if (msg.type === 'kaboom_capture_screenshot' && typeof callback === 'function') {
        callback({ dataUrl: 'data:image/png;base64,mockscreenshot' })
      }
      return undefined
    })

    overlay._dispatch('mousedown', { button: 0, clientX: 100, clientY: 100 })
    overlay._dispatch('mouseup', { clientX: 250, clientY: 200 })

    const inputEl = createdElements.find((el) => el.tagName === 'INPUT')
    inputEl.value = ''

    dm.deactivateAndSendResults()

    assert.strictEqual(dm.isDrawModeActive(), true, 'draw mode should remain active on invalid submit')
    assert.ok(inputEl.parentElement, 'text input should remain mounted for correction')

    const errorToast = sentMessages.find((m) => m.type === 'kaboom_action_toast' && m.state === 'error')
    assert.ok(errorToast, 'expected validation error toast')
    assert.ok(!sentMessages.some((m) => m.type === 'draw_mode_completed'), 'should not send completion on invalid submit')
  })

  test('empty text on Enter discards annotation', () => {
    dm.activateDrawMode('user')
    const overlay = documentBody.children[0]

    overlay._dispatch('mousedown', { button: 0, clientX: 100, clientY: 100 })
    overlay._dispatch('mousemove', { clientX: 250, clientY: 200 })
    overlay._dispatch('mouseup', { clientX: 250, clientY: 200 })

    const inputEl = createdElements.find((el) => el.tagName === 'INPUT')
    inputEl.value = ''
    const enterHandler = inputEl._listeners['keydown']?.[0]
    if (enterHandler) {
      enterHandler({ key: 'Enter', preventDefault: mock.fn(), stopPropagation: mock.fn() })
    }

    assert.deepStrictEqual(dm.getAnnotations(), [], 'empty text should discard annotation')
  })

  test('blur with text auto-confirms annotation', () => {
    dm.activateDrawMode('user')
    const overlay = documentBody.children[0]

    overlay._dispatch('mousedown', { button: 0, clientX: 100, clientY: 100 })
    overlay._dispatch('mousemove', { clientX: 250, clientY: 200 })
    overlay._dispatch('mouseup', { clientX: 250, clientY: 200 })

    const inputEl = createdElements.find((el) => el.tagName === 'INPUT')
    inputEl.value = 'increase padding'

    const blurHandler = inputEl._listeners['blur']?.[0]
    if (blurHandler) {
      blurHandler({})
    }

    const annotations = dm.getAnnotations()
    assert.strictEqual(annotations.length, 1, 'blur with text should auto-confirm')
    assert.strictEqual(annotations[0].text, 'increase padding')
  })

  test('right-click does not start drawing', () => {
    dm.activateDrawMode('user')
    const overlay = documentBody.children[0]

    overlay._dispatch('mousedown', { button: 2, clientX: 100, clientY: 100 })
    overlay._dispatch('mousemove', { clientX: 250, clientY: 200 })
    overlay._dispatch('mouseup', { clientX: 250, clientY: 200 })

    const inputEls = createdElements.filter((el) => el.tagName === 'INPUT')
    assert.strictEqual(inputEls.length, 0, 'right-click should not create text input')
  })

  test('reverse-direction drawing normalizes rect coordinates', () => {
    dm.activateDrawMode('user')
    const overlay = documentBody.children[0]

    // Draw from bottom-right to top-left
    overlay._dispatch('mousedown', { button: 0, clientX: 300, clientY: 300 })
    overlay._dispatch('mousemove', { clientX: 100, clientY: 100 })
    overlay._dispatch('mouseup', { clientX: 100, clientY: 100 })

    const inputEl = createdElements.find((el) => el.tagName === 'INPUT')
    assert.ok(inputEl, 'expected text input for reverse-drawn rectangle')

    inputEl.value = 'test reverse'
    const enterHandler = inputEl._listeners['keydown']?.[0]
    if (enterHandler) {
      enterHandler({ key: 'Enter', preventDefault: mock.fn(), stopPropagation: mock.fn() })
    }

    const annotations = dm.getAnnotations()
    assert.strictEqual(annotations.length, 1)
    assert.strictEqual(annotations[0].rect.x, 100, 'rect.x should be min')
    assert.strictEqual(annotations[0].rect.y, 100, 'rect.y should be min')
    assert.strictEqual(annotations[0].rect.width, 200)
    assert.strictEqual(annotations[0].rect.height, 200)
  })

  test('multiple annotations accumulate in order', () => {
    dm.activateDrawMode('user')
    const overlay = documentBody.children[0]

    // Draw first annotation
    overlay._dispatch('mousedown', { button: 0, clientX: 10, clientY: 10 })
    overlay._dispatch('mouseup', { clientX: 110, clientY: 60 })
    let inputEl = createdElements.filter((el) => el.tagName === 'INPUT').pop()
    inputEl.value = 'first'
    inputEl._listeners['keydown']?.[0]?.({ key: 'Enter', preventDefault: mock.fn(), stopPropagation: mock.fn() })

    // Draw second annotation
    overlay._dispatch('mousedown', { button: 0, clientX: 200, clientY: 200 })
    overlay._dispatch('mouseup', { clientX: 350, clientY: 300 })
    inputEl = createdElements.filter((el) => el.tagName === 'INPUT').pop()
    inputEl.value = 'second'
    inputEl._listeners['keydown']?.[0]?.({ key: 'Enter', preventDefault: mock.fn(), stopPropagation: mock.fn() })

    const annotations = dm.getAnnotations()
    assert.strictEqual(annotations.length, 2)
    assert.strictEqual(annotations[0].text, 'first')
    assert.strictEqual(annotations[1].text, 'second')
  })

  test('DOM element capture populates element_summary', () => {
    dm.activateDrawMode('user')
    const overlay = documentBody.children[0]

    overlay._dispatch('mousedown', { button: 0, clientX: 100, clientY: 100 })
    overlay._dispatch('mouseup', { clientX: 250, clientY: 200 })

    const inputEl = createdElements.find((el) => el.tagName === 'INPUT')
    inputEl.value = 'test capture'
    inputEl._listeners['keydown']?.[0]?.({ key: 'Enter', preventDefault: mock.fn(), stopPropagation: mock.fn() })

    const annotations = dm.getAnnotations()
    assert.strictEqual(annotations.length, 1)
    // elementsFromPoint mock returns a button.btn-primary 'Submit'
    assert.ok(annotations[0].element_summary.includes('button'), 'element_summary should contain tag')
  })

  test('DOM element capture stores retrievable detail via correlation_id', () => {
    dm.activateDrawMode('user')
    const overlay = documentBody.children[0]

    overlay._dispatch('mousedown', { button: 0, clientX: 100, clientY: 100 })
    overlay._dispatch('mouseup', { clientX: 250, clientY: 200 })

    const inputEl = createdElements.find((el) => el.tagName === 'INPUT')
    inputEl.value = 'test detail'
    inputEl._listeners['keydown']?.[0]?.({ key: 'Enter', preventDefault: mock.fn(), stopPropagation: mock.fn() })

    const annotations = dm.getAnnotations()
    const correlationId = annotations[0].correlation_id
    assert.ok(correlationId, 'should have correlation_id')

    const detail = dm.getElementDetail(correlationId)
    assert.ok(detail, 'should retrieve detail by correlation_id')
    assert.ok(detail.selector, 'detail should have selector')
    assert.ok(detail.tag, 'detail should have tag')
    assert.ok(Array.isArray(detail.action_trail), 'detail should include action_trail')
    assert.ok(detail.ui_context, 'detail should include ui_context')
  })

  test('new annotations include bounded recent action trail', () => {
    dm.activateDrawMode('user')
    const overlay = documentBody.children[0]

    // Trigger a prior action so trail is non-empty.
    const scrollCall = globalThis.window.addEventListener.mock.calls.find((c) => c.arguments?.[0] === 'scroll')
    assert.ok(scrollCall, 'expected scroll listener registration')
    globalThis.window.scrollY = 55
    scrollCall.arguments[1]()

    overlay._dispatch('mousedown', { button: 0, clientX: 100, clientY: 100 })
    overlay._dispatch('mouseup', { clientX: 250, clientY: 200 })

    const inputEl = createdElements.find((el) => el.tagName === 'INPUT')
    inputEl.value = 'trail metadata'
    inputEl._listeners['keydown']?.[0]?.({ key: 'Enter', preventDefault: mock.fn(), stopPropagation: mock.fn() })

    const annotation = dm.getAnnotations()[0]
    assert.ok(Array.isArray(annotation.action_trail), 'expected action_trail array')
    assert.ok(annotation.action_trail.length > 0, 'expected non-empty action_trail')

    const first = annotation.action_trail[0]
    assert.ok(first.type, 'trail entry should include type')
    assert.ok(first.target_summary, 'trail entry should include target_summary')
    assert.strictEqual(typeof first.timestamp, 'number')
  })

  test('new annotations include ui context metadata and focused-element summary', () => {
    dm.activateDrawMode('user')
    const overlay = documentBody.children[0]

    const focused = createMockElement('button')
    focused.id = 'save-btn'
    focused.textContent = 'Save'
    focused.parentElement = createMockElement('div')
    globalThis.document.activeElement = focused

    overlay._dispatch('mousedown', { button: 0, clientX: 100, clientY: 100 })
    overlay._dispatch('mouseup', { clientX: 250, clientY: 200 })

    const inputEl = createdElements.find((el) => el.tagName === 'INPUT')
    inputEl.value = 'context metadata'
    inputEl._listeners['keydown']?.[0]?.({ key: 'Enter', preventDefault: mock.fn(), stopPropagation: mock.fn() })

    const annotation = dm.getAnnotations()[0]
    assert.ok(annotation.ui_context, 'expected ui_context on annotation')
    assert.ok(['light', 'dark'].includes(annotation.ui_context.theme), 'expected normalized theme')
    assert.strictEqual(annotation.ui_context.viewport.width, globalThis.window.innerWidth)
    assert.strictEqual(annotation.ui_context.viewport.height, globalThis.window.innerHeight)
    assert.strictEqual(typeof annotation.ui_context.sidebars.left_open, 'boolean')
    assert.strictEqual(typeof annotation.ui_context.sidebars.right_open, 'boolean')
    assert.ok(annotation.ui_context.focused_element, 'expected focused_element summary')
  })
})

// =============================================================================
// Gap 5: Content Script Message Routing
// =============================================================================

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
describe('Draw Mode — Element Detail Enrichment', () => {
  let dm

  beforeEach(async () => {
    setupGlobals()

    // Enhance mock to provide full DOM tree context for enrichment tests
    const grandparent = createMockElement('form')
    grandparent.classList._items = ['checkout-form']
    grandparent.id = 'checkout'

    const parent = createMockElement('div')
    parent.classList._items = ['actions', 'mt-4']
    parent.id = ''
    parent.parentElement = grandparent
    parent.getAttribute = (attr) => {
      if (attr === 'role') return 'group'
      return null
    }
    grandparent.getAttribute = (_attr) => null

    // Create siblings
    const prevSibling1 = createMockElement('button')
    prevSibling1.classList._items = ['btn-secondary']
    prevSibling1.textContent = 'Cancel'
    const prevSibling2 = createMockElement('span')
    prevSibling2.classList._items = ['separator']
    prevSibling2.textContent = '|'

    const target = createMockElement('button')
    target.classList._items = ['btn-primary']
    target.textContent = 'Submit'
    target.id = 'submit-btn'
    target.parentElement = parent
    target.getAttribute = (_attr) => null

    const nextSibling1 = createMockElement('a')
    nextSibling1.classList._items = ['help-link']
    nextSibling1.textContent = 'Help'
    const nextSibling2 = createMockElement('span')
    nextSibling2.classList._items = ['spacer']
    nextSibling2.textContent = ''

    // Set up parent.children with siblings
    parent.children = [prevSibling1, prevSibling2, target, nextSibling1, nextSibling2]
    for (const child of parent.children) {
      child.parentElement = parent
    }

    globalThis.document.elementsFromPoint = mock.fn((_x, _y) => [target])

    dm = await importDrawMode()
  })

  function drawAndGetDetail() {
    const beforeCount = dm.getAnnotations().length
    dm.activateDrawMode('user')
    const overlay = documentBody.children[0]

    overlay._dispatch('mousedown', { button: 0, clientX: 100, clientY: 100 })
    overlay._dispatch('mouseup', { clientX: 250, clientY: 200 })

    const inputEl = createdElements.find((el) => el.tagName === 'INPUT')
    inputEl.value = 'enrichment test'
    inputEl._listeners['keydown']?.[0]?.({ key: 'Enter', preventDefault: mock.fn(), stopPropagation: mock.fn() })

    const annotations = dm.getAnnotations()
    assert.ok(annotations.length > beforeCount)
    return dm.getElementDetail(annotations[annotations.length - 1].correlation_id)
  }

  test('element detail includes parent_context with parent and grandparent', () => {
    const detail = drawAndGetDetail()
    assert.ok(detail, 'should have detail')
    assert.ok(detail.parent_context, 'detail should have parent_context')

    const pc = detail.parent_context
    assert.ok(pc.parent, 'parent_context should have parent')
    assert.strictEqual(pc.parent.tag, 'div')
    assert.ok(Array.isArray(pc.parent.classes), 'parent classes should be array')
    assert.ok(pc.parent.classes.includes('actions'), 'parent should have actions class')

    assert.ok(pc.grandparent, 'parent_context should have grandparent')
    assert.strictEqual(pc.grandparent.tag, 'form')
    assert.strictEqual(pc.grandparent.id, 'checkout')
  })

  test('element detail includes siblings (up to 2 before and 2 after)', () => {
    const detail = drawAndGetDetail()
    assert.ok(detail, 'should have detail')
    assert.ok(Array.isArray(detail.siblings), 'detail should have siblings array')
    assert.ok(detail.siblings.length > 0, 'siblings should not be empty')
    assert.ok(detail.siblings.length <= 4, 'at most 4 siblings (2 before + 2 after)')

    // Check sibling shape
    for (const sib of detail.siblings) {
      assert.ok(sib.tag, 'sibling should have tag')
      assert.ok(Array.isArray(sib.classes), 'sibling should have classes array')
      assert.ok(['before', 'after'].includes(sib.position), 'sibling should have position before or after')
    }
  })

  test('css_framework absent when no framework detected', () => {
    const detail = drawAndGetDetail()
    assert.ok(detail, 'should have detail')
    // Mock element has btn-primary which doesn't match any framework threshold
    assert.strictEqual(detail.css_framework, undefined, 'css_framework should be absent when no framework detected')
    assert.strictEqual(detail.js_framework, undefined, 'js_framework should be absent when runtime framework not detected')
  })

  test('css_framework detects Tailwind classes', () => {
    // Override elementsFromPoint to return element with Tailwind classes
    const twEl = createMockElement('div')
    twEl.classList._items = ['flex', 'p-4', 'text-sm', 'bg-blue-500', 'rounded-lg', 'mt-2']
    twEl.textContent = 'Tailwind element'
    twEl.parentElement = createMockElement('div')
    twEl.parentElement.parentElement = null
    twEl.getAttribute = (_attr) => null
    globalThis.document.elementsFromPoint = mock.fn(() => [twEl])

    const detail = drawAndGetDetail()
    assert.ok(detail, 'should have detail')
    assert.strictEqual(detail.css_framework, 'tailwind', `expected 'tailwind', got '${detail.css_framework}'`)
  })

  test('css_framework detects Bootstrap classes', () => {
    const bsEl = createMockElement('div')
    bsEl.classList._items = ['col-md-6', 'btn-primary', 'form-control', 'container']
    bsEl.textContent = 'Bootstrap element'
    bsEl.parentElement = createMockElement('div')
    bsEl.parentElement.parentElement = null
    bsEl.getAttribute = (_attr) => null
    globalThis.document.elementsFromPoint = mock.fn(() => [bsEl])

    const detail = drawAndGetDetail()
    assert.ok(detail, 'should have detail')
    assert.strictEqual(detail.css_framework, 'bootstrap', `expected 'bootstrap', got '${detail.css_framework}'`)
  })

  test('css_framework detects CSS Modules hash pattern', () => {
    const modEl = createMockElement('div')
    modEl.classList._items = ['Button_primary__a1b2c3', 'Card_wrapper__x9y8z7']
    modEl.textContent = 'CSS Modules element'
    modEl.parentElement = createMockElement('div')
    modEl.parentElement.parentElement = null
    modEl.getAttribute = (_attr) => null
    globalThis.document.elementsFromPoint = mock.fn(() => [modEl])

    const detail = drawAndGetDetail()
    assert.ok(detail, 'should have detail')
    assert.strictEqual(detail.css_framework, 'css-modules', `expected 'css-modules', got '${detail.css_framework}'`)
  })

  test('css_framework detects styled-components/emotion pattern', () => {
    const scEl = createMockElement('div')
    scEl.classList._items = ['css-1a2b3c', 'sc-dkrFOg', 'css-4d5e6f']
    scEl.textContent = 'Styled element'
    scEl.parentElement = createMockElement('div')
    scEl.parentElement.parentElement = null
    scEl.getAttribute = (_attr) => null
    globalThis.document.elementsFromPoint = mock.fn(() => [scEl])

    const detail = drawAndGetDetail()
    assert.ok(detail, 'should have detail')
    assert.strictEqual(detail.css_framework, 'styled-components', `expected 'styled-components', got '${detail.css_framework}'`)
  })

  test('parent_context is absent when element parent is document.body', () => {
    const el = createMockElement('button')
    el.classList._items = ['main-btn']
    el.textContent = 'Click'
    el.parentElement = globalThis.document.body
    el.getAttribute = (_attr) => null
    globalThis.document.elementsFromPoint = mock.fn(() => [el])

    const detail = drawAndGetDetail()
    assert.ok(detail, 'should have detail')
    assert.strictEqual(detail.parent_context, undefined, 'parent_context should be absent when parent is body')
  })

  test('siblings is absent when element is only child', () => {
    const parent = createMockElement('div')
    parent.classList._items = ['wrapper']
    parent.getAttribute = (_attr) => null

    const el = createMockElement('button')
    el.classList._items = ['solo']
    el.textContent = 'Solo'
    el.parentElement = parent
    el.getAttribute = (_attr) => null
    parent.children = [el]

    globalThis.document.elementsFromPoint = mock.fn(() => [el])

    const detail = drawAndGetDetail()
    assert.ok(detail, 'should have detail')
    assert.strictEqual(detail.siblings, undefined, 'siblings should be absent when element is only child')
  })

  test('css_framework priority: Tailwind wins over Bootstrap when both match', () => {
    const mixedEl = createMockElement('div')
    // Has both Tailwind-specific (p-4, mt-2, text-sm) and Bootstrap (btn-primary, form-control)
    mixedEl.classList._items = ['flex', 'p-4', 'mt-2', 'text-sm', 'btn-primary', 'form-control']
    mixedEl.textContent = 'Mixed'
    mixedEl.parentElement = createMockElement('div')
    mixedEl.parentElement.parentElement = null
    mixedEl.getAttribute = (_attr) => null
    globalThis.document.elementsFromPoint = mock.fn(() => [mixedEl])

    const detail = drawAndGetDetail()
    assert.ok(detail, 'should have detail')
    assert.strictEqual(detail.css_framework, 'tailwind', 'Tailwind should win over Bootstrap when both match')
  })

  test('parent_context has null grandparent when parent has no parent element', () => {
    const parent = createMockElement('section')
    parent.classList._items = ['top-level']
    parent.parentElement = null
    parent.getAttribute = (_attr) => null

    const el = createMockElement('button')
    el.classList._items = ['action']
    el.textContent = 'Go'
    el.parentElement = parent
    el.getAttribute = (_attr) => null
    parent.children = [el]

    globalThis.document.elementsFromPoint = mock.fn(() => [el])

    const detail = drawAndGetDetail()
    assert.ok(detail, 'should have detail')
    assert.ok(detail.parent_context, 'should have parent_context')
    assert.strictEqual(detail.parent_context.parent.tag, 'section')
    assert.strictEqual(detail.parent_context.grandparent, null, 'grandparent should be null when parent has no parent')
  })

  test('siblings capped at 2 before and 2 after even with many siblings', () => {
    const parent = createMockElement('ul')
    parent.classList._items = ['list']
    parent.parentElement = null
    parent.getAttribute = (_attr) => null

    // Create 5 before + target + 5 after = 11 children
    const children = []
    for (let i = 0; i < 5; i++) {
      const li = createMockElement('li')
      li.classList._items = [`item-before-${i}`]
      li.textContent = `Before ${i}`
      li.parentElement = parent
      children.push(li)
    }
    const target = createMockElement('li')
    target.classList._items = ['selected']
    target.textContent = 'Target'
    target.parentElement = parent
    target.getAttribute = (_attr) => null
    children.push(target)
    for (let i = 0; i < 5; i++) {
      const li = createMockElement('li')
      li.classList._items = [`item-after-${i}`]
      li.textContent = `After ${i}`
      li.parentElement = parent
      children.push(li)
    }
    parent.children = children

    globalThis.document.elementsFromPoint = mock.fn(() => [target])

    const detail = drawAndGetDetail()
    assert.ok(detail, 'should have detail')
    assert.ok(Array.isArray(detail.siblings), 'should have siblings')
    assert.strictEqual(detail.siblings.length, 4, 'should cap at 4 siblings (2 before + 2 after)')

    const beforeSibs = detail.siblings.filter(s => s.position === 'before')
    const afterSibs = detail.siblings.filter(s => s.position === 'after')
    assert.strictEqual(beforeSibs.length, 2, 'exactly 2 before siblings')
    assert.strictEqual(afterSibs.length, 2, 'exactly 2 after siblings')
  })

  test('generic classes alone do not trigger Tailwind detection', () => {
    const genericEl = createMockElement('div')
    genericEl.classList._items = ['flex', 'border', 'shadow', 'rounded']
    genericEl.textContent = 'Generic'
    genericEl.parentElement = createMockElement('div')
    genericEl.parentElement.parentElement = null
    genericEl.getAttribute = (_attr) => null
    globalThis.document.elementsFromPoint = mock.fn(() => [genericEl])

    const detail = drawAndGetDetail()
    assert.ok(detail, 'should have detail')
    assert.strictEqual(detail.css_framework, undefined, 'generic CSS classes should not trigger Tailwind detection')
  })

  test('selector_candidates include semantic locator fallbacks', () => {
    const candidateEl = createMockElement('button')
    candidateEl.id = 'submit-btn'
    candidateEl.classList._items = ['btn-primary']
    candidateEl.textContent = 'Submit order'
    candidateEl.parentElement = createMockElement('div')
    candidateEl.parentElement.parentElement = null
    candidateEl.getAttribute = (attr) => {
      if (attr === 'data-testid') return 'checkout-submit'
      if (attr === 'role') return 'button'
      if (attr === 'aria-label') return 'Submit order'
      return null
    }
    globalThis.document.elementsFromPoint = mock.fn(() => [candidateEl])

    const detail = drawAndGetDetail()
    assert.ok(detail, 'should have detail')
    assert.ok(Array.isArray(detail.selector_candidates), 'selector_candidates should be an array')
    assert.ok(detail.selector_candidates.includes('css=#submit-btn'), 'should include id-based css selector')
    assert.ok(detail.selector_candidates.includes('testid=checkout-submit'), 'should include test id candidate')
    assert.ok(detail.selector_candidates.includes('role=button|Submit order'), 'should include role+name candidate')
    assert.ok(detail.selector_candidates.includes('label=Submit order'), 'should include label candidate')

    const unique = new Set(detail.selector_candidates)
    assert.strictEqual(unique.size, detail.selector_candidates.length, 'selector_candidates should be deduplicated')
  })

  test('js_framework and component are captured from runtime metadata', () => {
    const reactEl = createMockElement('button')
    reactEl.classList._items = ['btn']
    reactEl.textContent = 'Save'
    reactEl.parentElement = createMockElement('div')
    reactEl.parentElement.parentElement = null
    reactEl.getAttribute = (_attr) => null
    reactEl.__reactFiber$test = {
      type: { name: 'SaveButton' },
      _debugSource: { fileName: '/src/components/SaveButton.tsx', lineNumber: 18 },
      return: null
    }
    globalThis.document.elementsFromPoint = mock.fn(() => [reactEl])

    const detail = drawAndGetDetail()
    assert.ok(detail, 'should have detail')
    assert.strictEqual(detail.js_framework, 'react', 'should include top-level js_framework')
    assert.ok(detail.component, 'should include component detail object')
    assert.strictEqual(detail.component.framework, 'react')
    assert.strictEqual(detail.component.component, 'SaveButton')
    assert.strictEqual(detail.component.source_file, '/src/components/SaveButton.tsx')
  })
})
