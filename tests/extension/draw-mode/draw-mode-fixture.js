// @ts-nocheck
/**
 * @fileoverview Shared DOM, Chrome, timer, and module fixtures for draw-mode tests.
 */

import { mock, afterEach } from 'node:test'
import { MANIFEST_VERSION } from '../shared/helpers.js'

/** Minimal mock element that supports style, events, and child management. */
export function createMockElement(tag = 'div') {
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
    // Attribute API. Real elements have it and production code uses it — notably the
    // `data-kaboom-overlay` marker that lets screenshot capture strip our own overlays.
    // A mock missing these methods makes any legitimate setAttribute call look like a bug.
    _attrs: {},
    setAttribute(name, value) {
      this._attrs[name] = String(value)
      if (name === 'id') this.id = String(value)
    },
    getAttribute(name) {
      return Object.prototype.hasOwnProperty.call(this._attrs, name) ? this._attrs[name] : null
    },
    hasAttribute(name) {
      return Object.prototype.hasOwnProperty.call(this._attrs, name)
    },
    removeAttribute(name) {
      delete this._attrs[name]
    },
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

export let documentBody
export let createdElements
let styleEl
export let storageData

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

export function setupGlobals() {
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
export function nextModuleVersion() {
  importCounter++
  return importCounter
}

export async function importDrawMode() {
  const mod = await import(`../../../extension/content/draw-mode.js?v=${nextModuleVersion()}`)
  return mod
}
