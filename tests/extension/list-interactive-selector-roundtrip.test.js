// @ts-nocheck
/**
 * @fileoverview list-interactive-selector-roundtrip.test.js — Regression guard for
 * list_interactive selector round-tripping into domPrimitive click.
 */

import { describe, test } from 'node:test'
import assert from 'node:assert'

class FakeHTMLElement {
  constructor(tag, attrs = {}, rect = { x: 0, y: 0, width: 120, height: 30 }) {
    this.tagName = String(tag || 'div').toUpperCase()
    this._attrs = { ...attrs }
    this._rect = { ...rect }
    this.children = []
    this.parentElement = null
    this.shadowRoot = null
    this.id = attrs.id || ''
    this.name = attrs.name || ''
    this.type = attrs.type || ''
    this.href = attrs.href || ''
    this.textContent = attrs.textContent || ''
    this.offsetParent = rect.width > 0 && rect.height > 0 ? {} : null
    this._clicked = 0
    this.isConnected = true
  }

  getAttribute(name) {
    return Object.prototype.hasOwnProperty.call(this._attrs, name) ? this._attrs[name] : null
  }

  setAttribute(name, value) {
    this._attrs[name] = String(value)
  }

  removeAttribute(name) {
    delete this._attrs[name]
  }

  getBoundingClientRect() {
    const { x, y, width, height } = this._rect
    return {
      x, y, width, height,
      left: x, top: y,
      right: x + width,
      bottom: y + height
    }
  }

  getRootNode() {
    return globalThis.document
  }

  querySelectorAll() {
    return []
  }

  querySelector() {
    return null
  }

  closest(selector) {
    if (this.tagName === 'BUTTON' && selector.includes('button')) return this
    if ((this.getAttribute('role') || '') === 'button' && selector.includes('[role="button"]')) return this
    return null
  }

  contains(node) {
    return node === this
  }

  scrollIntoView() {}

  click() {
    this._clicked++
  }
}

class FakeInputElement extends FakeHTMLElement {}

function parseEqualsAttrSelector(selector) {
  const m = selector.match(/^\[([a-zA-Z0-9_-]+)="([^"]*)"\]$/)
  if (!m) return null
  return { attr: m[1], value: m[2] }
}

function matchesSelector(el, selector) {
  const sel = String(selector || '').trim()
  if (!sel) return false
  if (sel === 'button') return el.tagName === 'BUTTON'
  if (sel === 'input') return el.tagName === 'INPUT'
  if (sel === 'select') return el.tagName === 'SELECT'
  if (sel === 'textarea') return el.tagName === 'TEXTAREA'
  if (sel === 'a[href]') return el.tagName === 'A' && !!el.getAttribute('href')
  if (sel === '[role="button"]') return el.getAttribute('role') === 'button'
  if (sel === '[role="link"]') return el.getAttribute('role') === 'link'
  if (sel === '[role="tab"]') return el.getAttribute('role') === 'tab'
  if (sel === '[role="menuitem"]') return el.getAttribute('role') === 'menuitem'
  if (sel === '[contenteditable="true"]') return el.getAttribute('contenteditable') === 'true'
  if (sel === '[onclick]') return el.getAttribute('onclick') != null
  if (sel === '[tabindex]') return el.getAttribute('tabindex') != null
  const attrEq = parseEqualsAttrSelector(sel)
  if (attrEq) return (el.getAttribute(attrEq.attr) || '') === attrEq.value
  return false
}

function setupDOM(elements) {
  let all = [...elements]
  let textNodes = buildTextNodes(all)

  function buildTextNodes(items) {
    const out = []
    for (const el of items) {
      if ((el.textContent || '').trim()) {
        out.push({ textContent: el.textContent, parentElement: el })
      }
    }
    return out
  }

  function setElements(nextElements) {
    for (const el of all) {
      el.isConnected = false
    }
    all = [...nextElements]
    for (const el of all) {
      el.isConnected = true
    }
    textNodes = buildTextNodes(all)
  }

  globalThis.HTMLElement = FakeHTMLElement
  globalThis.HTMLInputElement = FakeInputElement
  globalThis.ShadowRoot = class ShadowRoot {}
  globalThis.NodeFilter = { SHOW_TEXT: 4 }
  globalThis.MutationObserver = class MutationObserver {
    constructor(cb) {
      this._cb = cb
    }
    observe() {}
    disconnect() {}
    takeRecords() { return [] }
  }
  globalThis.CSS = { escape: (v) => String(v).replace(/"/g, '\\"') }
  globalThis.getComputedStyle = () => ({ visibility: 'visible', display: 'block', position: 'static', zIndex: '0' })
  globalThis.window = {
    innerHeight: 1000,
    innerWidth: 1600,
    open: () => ({})
  }

  globalThis.document = {
    body: { children: [] },
    documentElement: { children: [] },
    querySelectorAll(selector) {
      return all.filter((el) => matchesSelector(el, selector))
    },
    querySelector(selector) {
      return this.querySelectorAll(selector)[0] || null
    },
    createTreeWalker(root, whatToShow) {
      let idx = -1
      return {
        currentNode: null,
        nextNode() {
          if (whatToShow !== globalThis.NodeFilter.SHOW_TEXT) return null
          idx++
          if (idx >= textNodes.length) return null
          this.currentNode = textNodes[idx]
          return this.currentNode
        }
      }
    }
  }

  return { setElements }
}

async function loadListInteractive() {
  const mod = await import(`../../extension/background/dom/primitives/dom-primitives-list-interactive.js?t=${Date.now()}`)
  return mod.domPrimitiveListInteractive
}

async function loadDomPrimitive() {
  const mod = await import(`../../extension/background/dom/primitives/dom-primitives.js?t=${Date.now()}`)
  return mod.domPrimitive
}

describe('list_interactive selector round-trip', () => {
  test('text=:nth-match selector from list_interactive is directly clickable', async () => {
    const first = new FakeHTMLElement('button', { textContent: 'Posts' }, { x: 20, y: 20, width: 100, height: 28 })
    const second = new FakeHTMLElement('button', { textContent: 'Posts' }, { x: 20, y: 70, width: 100, height: 28 })
    setupDOM([first, second])

    const listInteractive = await loadListInteractive()
    const domPrimitive = await loadDomPrimitive()

    const listed = listInteractive('', { visible_only: true })
    assert.strictEqual(listed.success, true)
    const posts = listed.elements.filter((e) => e.label === 'Posts')
    assert.strictEqual(posts.length, 2)
    assert.ok(String(posts[1].selector).startsWith('text=Posts:nth-match('))

    const clickResult = await domPrimitive('click', posts[1].selector, {})
    assert.strictEqual(clickResult.success, true, `clickResult=${JSON.stringify(clickResult)}`)
    assert.strictEqual(second._clicked, 1)
    assert.strictEqual(first._clicked, 0)
  })

  test('element_id from list_interactive is resilient across SPA-style remounts', async () => {
    const navA = new FakeHTMLElement('button', { textContent: 'Posts' }, { x: 20, y: 20, width: 100, height: 28 })
    const navB = new FakeHTMLElement('button', { textContent: 'Profiles' }, { x: 20, y: 70, width: 100, height: 28 })
    const dom = setupDOM([navA, navB])

    const listInteractive = await loadListInteractive()
    const domPrimitive = await loadDomPrimitive()

    const listed = listInteractive('', { visible_only: true })
    assert.strictEqual(listed.success, true)
    const profilesEntry = listed.elements.find((e) => e.label === 'Profiles')
    assert.ok(profilesEntry)
    assert.ok(profilesEntry.element_id)

    // Simulate SPA navigation/remount: old nodes detach, new nodes replace them.
    const nextNavA = new FakeHTMLElement('button', { textContent: 'Posts' }, { x: 20, y: 20, width: 100, height: 28 })
    const nextNavB = new FakeHTMLElement('button', { textContent: 'Profiles' }, { x: 20, y: 70, width: 100, height: 28 })
    dom.setElements([nextNavA, nextNavB])

    const clickResult = await domPrimitive('click', '', { element_id: profilesEntry.element_id })
    assert.strictEqual(clickResult.success, true, `clickResult=${JSON.stringify(clickResult)}`)
    assert.strictEqual(nextNavB._clicked, 1)
    assert.strictEqual(nextNavA._clicked, 0)
  })
})
