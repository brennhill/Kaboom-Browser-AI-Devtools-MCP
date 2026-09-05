// @ts-nocheck
/**
 * @fileoverview provenance-fixture.js — A hand-rolled page fixture for content provenance.
 *
 * The repo has no DOM implementation in test (no jsdom), and the provenance collector only
 * needs a narrow slice of the Element surface: tag, attributes, parentage, containment, text,
 * and a frame query. Building that slice here keeps the assertions deterministic and keeps the
 * production code free of test-only seams.
 */

/** Element node type, as the DOM spec numbers it. */
const ELEMENT_NODE = 1
const TEXT_NODE = 3

/** Minimal Element stand-in: enough surface for the provenance collector, nothing more. */
export function element(tag, options = {}) {
  const node = {
    nodeType: ELEMENT_NODE,
    tagName: String(tag).toUpperCase(),
    parentElement: null,
    isConnected: options.is_connected !== false,
    children: [],
    contentDocument: options.content_document ?? null,
    _attrs: { ...(options.attrs ?? {}) },
    _ownText: options.text ?? '',

    getAttribute(name) {
      return Object.prototype.hasOwnProperty.call(this._attrs, name) ? this._attrs[name] : null
    },
    hasAttribute(name) {
      return Object.prototype.hasOwnProperty.call(this._attrs, name)
    },
    setAttribute(name, value) {
      this._attrs[name] = String(value)
    },
    append(...kids) {
      for (const kid of kids) {
        this.children.push(kid)
        kid.parentElement = this
      }
      return this
    },
    contains(candidate) {
      let cursor = candidate
      while (cursor) {
        if (cursor === this) return true
        cursor = cursor.parentElement
      }
      return false
    },
    // Only the comma-separated tag lists the collector actually uses are supported. An
    // unsupported selector throws rather than silently matching nothing, so a production
    // change that widens the selector fails loudly here instead of quietly losing regions.
    querySelectorAll(selector) {
      const wanted = selector.split(',').map((part) => part.trim().toUpperCase())
      for (const part of wanted) {
        if (!/^[A-Z]+$/.test(part)) throw new Error(`fixture supports tag selectors only, got: ${selector}`)
      }
      const found = []
      const walk = (current) => {
        for (const kid of current.children) {
          if (wanted.includes(kid.tagName)) found.push(kid)
          walk(kid)
        }
      }
      walk(this)
      return found
    },
    get textContent() {
      return this.children.reduce((text, kid) => text + kid.textContent, this._ownText)
    },
    get innerText() {
      return this.textContent
    }
  }
  for (const kid of options.children ?? []) node.append(kid)
  return node
}

/** Text node stand-in, for asserting that a text insertion is attributed to its parent element. */
export function textNode(text) {
  return { nodeType: TEXT_NODE, textContent: text, parentElement: null }
}

/**
 * The fixture page the bead asks for: a same-origin block, a cross-origin iframe, and a slot
 * where a script injects imperative text after load.
 */
export function buildFixturePage() {
  const firstPartyBlock = element('article', {
    text: 'Nitrile gloves, box of 100. Free shipping on orders over $50.'
  })
  const sameOriginFrame = element('iframe', {
    attrs: { src: '/reviews.html' },
    content_document: { body: element('body', { text: 'Four stars from 812 reviewers.' }) }
  })
  const crossOriginFrame = element('iframe', {
    attrs: { src: 'https://ads.example/unit?slot=top&uid=9f2c' }
  })
  const main = element('main', { children: [firstPartyBlock, sameOriginFrame, crossOriginFrame] })
  const body = element('body', { children: [main] })
  return { body, main, firstPartyBlock, sameOriginFrame, crossOriginFrame }
}

/** MutationObserver stand-in: records the observe() call and lets a test deliver records. */
export function fakeObserverFactory() {
  const state = { callback: null, observed: [], disconnected: 0 }
  const factory = (callback) => {
    state.callback = callback
    return {
      observe(target, options) {
        state.observed.push({ target, options })
      },
      disconnect() {
        state.disconnected += 1
      }
    }
  }
  return { factory, state, deliver: (records) => state.callback(records) }
}
