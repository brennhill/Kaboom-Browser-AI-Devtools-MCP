// @ts-nocheck
/**
 * @fileoverview list-interactive-spatial-sort.test.js — Tests that list_interactive
 * returns elements in spatial reading order (top-to-bottom, left-to-right).
 */

import { describe, test } from 'node:test'
import assert from 'node:assert'

// Minimal DOM stubs for domPrimitiveListInteractive
class FakeElement {
  constructor(tag, attrs = {}, rect = { x: 0, y: 0, width: 100, height: 30 }) {
    this.tagName = tag.toUpperCase()
    this._attrs = attrs
    this._rect = rect
    this.textContent = attrs.textContent || ''
    this.children = []
    this.offsetParent = rect.width > 0 ? {} : null
    this.shadowRoot = null
    this.id = attrs.id || ''
    this.name = attrs.name || ''
    this.type = attrs.type || ''
  }
  getAttribute(name) { return this._attrs[name] || null }
  getBoundingClientRect() {
    return {
      x: this._rect.x, y: this._rect.y,
      left: this._rect.x, top: this._rect.y,
      right: this._rect.x + this._rect.width,
      bottom: this._rect.y + this._rect.height,
      width: this._rect.width, height: this._rect.height
    }
  }
  getRootNode() { return globalThis.document }
  querySelectorAll() { return [] }
}

// Build a fake document that returns elements in a specific DOM order
function setupDOM(elements) {
  const selectorMap = new Map()
  for (const el of elements) {
    const tag = el.tagName.toLowerCase()
    // Map elements to the selectors that match them
    const selectors = []
    if (tag === 'a' && el._attrs.href) selectors.push('a[href]')
    if (tag === 'button') selectors.push('button')
    if (tag === 'input') selectors.push('input')
    if (el._attrs.role === 'button') selectors.push('[role="button"]')
    if (el._attrs.tabindex) selectors.push('[tabindex]')
    for (const sel of selectors) {
      if (!selectorMap.has(sel)) selectorMap.set(sel, [])
      selectorMap.get(sel).push(el)
    }
  }

  globalThis.document = {
    querySelectorAll(selector) {
      return selectorMap.get(selector) || []
    },
    body: { children: [] },
    documentElement: { children: [] }
  }
  globalThis.ShadowRoot = class ShadowRoot {}
  globalThis.HTMLElement = class HTMLElement {}
  globalThis.HTMLInputElement = class HTMLInputElement {}
}

// Dynamic import to get the function after globals are set
async function getFunction() {
  // Force fresh import each test by using a cache-busting query
  const mod = await import(`../../../extension/background/dom/primitives/dom-primitives-list-interactive.js?t=${Date.now()}`)
  return mod.domPrimitiveListInteractive
}

describe('list_interactive spatial sort', () => {
  test('sorts visible elements in reading order (top-to-bottom, left-to-right)', async () => {
    // DOM order: bottom-right, top-left, top-right, middle-left
    // Expected spatial order: top-left, top-right, middle-left, bottom-right
    const bottomRight = new FakeElement('button', { textContent: 'Bottom Right', 'aria-label': 'Bottom Right' }, { x: 400, y: 300, width: 100, height: 30 })
    const topLeft = new FakeElement('button', { textContent: 'Top Left', 'aria-label': 'Top Left' }, { x: 10, y: 10, width: 100, height: 30 })
    const topRight = new FakeElement('button', { textContent: 'Top Right', 'aria-label': 'Top Right' }, { x: 400, y: 12, width: 100, height: 30 })
    const middleLeft = new FakeElement('button', { textContent: 'Middle Left', 'aria-label': 'Middle Left' }, { x: 10, y: 150, width: 100, height: 30 })

    setupDOM([bottomRight, topLeft, topRight, middleLeft])
    const fn = await getFunction()
    const result = fn()

    assert.strictEqual(result.success, true)
    assert.strictEqual(result.elements.length, 4)

    const labels = result.elements.map(e => e.label)
    assert.deepStrictEqual(labels, ['Top Left', 'Top Right', 'Middle Left', 'Bottom Right'])
  })

  test('elements within 10px vertically are treated as same row, sorted by x', async () => {
    // Two elements at y=100 and y=108 — within threshold, should sort by x
    const rightEl = new FakeElement('button', { textContent: 'Right', 'aria-label': 'Right' }, { x: 300, y: 100, width: 80, height: 30 })
    const leftEl = new FakeElement('button', { textContent: 'Left', 'aria-label': 'Left' }, { x: 50, y: 108, width: 80, height: 30 })

    setupDOM([rightEl, leftEl])
    const fn = await getFunction()
    const result = fn()

    assert.strictEqual(result.success, true)
    const labels = result.elements.map(e => e.label)
    assert.deepStrictEqual(labels, ['Left', 'Right'])
  })

  test('invisible elements are sorted after visible ones', async () => {
    const invisible = new FakeElement('button', { textContent: 'Hidden', 'aria-label': 'Hidden' }, { x: 0, y: 0, width: 0, height: 0 })
    const visible = new FakeElement('button', { textContent: 'Visible', 'aria-label': 'Visible' }, { x: 100, y: 200, width: 80, height: 30 })

    setupDOM([invisible, visible])
    const fn = await getFunction()
    const result = fn()

    assert.strictEqual(result.success, true)
    assert.strictEqual(result.elements.length, 2)
    assert.strictEqual(result.elements[0].label, 'Visible')
    assert.strictEqual(result.elements[1].label, 'Hidden')
  })

  test('indices are assigned after spatial sort', async () => {
    const bottom = new FakeElement('button', { textContent: 'Bottom', 'aria-label': 'Bottom' }, { x: 10, y: 200, width: 100, height: 30 })
    const top = new FakeElement('button', { textContent: 'Top', 'aria-label': 'Top' }, { x: 10, y: 10, width: 100, height: 30 })

    setupDOM([bottom, top])
    const fn = await getFunction()
    const result = fn()

    assert.strictEqual(result.elements[0].label, 'Top')
    assert.strictEqual(result.elements[0].index, 0)
    assert.strictEqual(result.elements[1].label, 'Bottom')
    assert.strictEqual(result.elements[1].index, 1)
  })
})
