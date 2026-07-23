// @ts-nocheck
/**
 * @fileoverview dom-primitives-click-enhancements.test.js — Tests for click UX improvements.
 *
 * Covers:
 *   #336 — auto-scroll off-screen elements into view before clicking
 *   #332 — bubble up to nearest interactive ancestor for click
 *   #333 — scroll_to with nested scrollable containers
 *   #316 — ambiguous text selector warning metadata
 *
 * Run: node --test extension/background/dom-primitives-click-enhancements.test.js
 */

import { test, describe, beforeEach } from 'node:test'
import assert from 'node:assert'

// ---------------------------------------------------------------------------
// Minimal DOM mocks
// ---------------------------------------------------------------------------
class MockHTMLElement {
  constructor(tag, props = {}) {
    this.tagName = tag
    this.id = props.id || ''
    this.textContent = props.textContent || ''
    this.innerText = props.textContent || ''
    this.offsetParent = {} // non-null = visible
    this.style = { position: '' }
    this.className = props.className || ''
    this.parentElement = props.parentElement || null
    this._rect = props.rect || { width: 100, height: 30, top: 10, left: 10, right: 110, bottom: 40, x: 10, y: 10 }
    this._scrollIntoViewCalls = []
    this._clickCalls = []
    this.children = { length: 0 }
    this.isConnected = true
  }
  click() { this._clickCalls.push(true) }
  focus() {}
  getAttribute(name) {
    if (name === 'role') return this._role || null
    if (name === 'aria-label') return this._ariaLabel || null
    return null
  }
  closest(sel) {
    let node = this
    while (node) {
      if (node._matchesSelector && node._matchesSelector(sel)) return node
      node = node.parentElement
    }
    return null
  }
  matches(sel) {
    return this._matchesSelector ? this._matchesSelector(sel) : false
  }
  querySelector() { return null }
  querySelectorAll() { return [] }
  scrollIntoView(opts) { this._scrollIntoViewCalls.push(opts || {}) }
  setAttribute() {}
  dispatchEvent() {}
  getBoundingClientRect() { return this._rect }
  getRootNode() { return globalThis.document }
  contains(el) {
    let node = el
    while (node) {
      if (node === this) return true
      node = node.parentElement
    }
    return false
  }
}

globalThis.HTMLElement = MockHTMLElement
globalThis.HTMLInputElement = class extends MockHTMLElement {}
globalThis.HTMLTextAreaElement = class extends MockHTMLElement {}
globalThis.HTMLSelectElement = class extends MockHTMLElement {}
globalThis.HTMLAnchorElement = class extends MockHTMLElement {}
globalThis.CSS = { escape: (s) => s }
globalThis.NodeFilter = { SHOW_TEXT: 4 }
globalThis.ShadowRoot = class ShadowRoot {}
globalThis.InputEvent = class extends Event {}
globalThis.KeyboardEvent = class extends Event {}
globalThis.ClipboardEvent = class extends Event {
  constructor(type, init) {
    super(type, init)
    this.clipboardData = init?.clipboardData
  }
}
globalThis.DataTransfer = class {
  constructor() { this._data = {} }
  setData(type, val) { this._data[type] = val }
}
globalThis.getComputedStyle = (el) => ({
  visibility: 'visible',
  display: 'block',
  position: el?.style?.position || '',
  overflow: el?._overflow || 'visible',
  overflowY: el?._overflowY || 'visible',
  overflowX: el?._overflowX || 'visible',
  zIndex: '',
  backgroundColor: ''
})
globalThis.MutationObserver = class {
  constructor(cb) { this._cb = cb }
  observe() {}
  disconnect() {}
}
let perfNowValue = 0
globalThis.performance = { now: () => perfNowValue++ }
globalThis.requestAnimationFrame = (cb) => cb()

const { domPrimitive } = await import('./dom-primitives.js')

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------
function makeElement(tag, props = {}) {
  const el = new MockHTMLElement(tag, props)
  Object.setPrototypeOf(el, MockHTMLElement.prototype)
  return el
}

function setupDocumentForCSS(elements, querySelector) {
  globalThis.document = {
    querySelector: querySelector || ((sel) => {
      for (const el of elements) {
        if (el.id && sel === `#${el.id}`) return el
      }
      return null
    }),
    querySelectorAll: (sel) => {
      return elements.filter((el) => {
        if (el.id && sel === `#${el.id}`) return true
        const tag = el.tagName.toLowerCase()
        if (sel === tag) return true
        if (sel === 'label') return tag === 'label'
        if (sel.startsWith('[role=')) {
          const m = sel.match(/\[role="(.+?)"\]/)
          return m && el.getAttribute && el.getAttribute('role') === m[1]
        }
        if (sel.startsWith('[aria-label=')) {
          const m = sel.match(/\[aria-label="(.+?)"\]/)
          return m && el.getAttribute && el.getAttribute('aria-label') === m[1]
        }
        if (sel.startsWith('[contenteditable=')) return false
        if (sel.startsWith('[onclick]')) return false
        if (sel.startsWith('[tabindex]')) return false
        return false
      })
    },
    getElementById: (id) => elements.find((el) => el.id === id) || null,
    body: {
      querySelectorAll: (sel) => {
        return elements.filter((el) => {
          const tag = el.tagName.toLowerCase()
          if (sel === tag) return true
          if (sel === 'a[href]') return tag === 'a'
          if (sel.startsWith('[role=')) {
            const m = sel.match(/\[role="(.+?)"\]/)
            return m && el.getAttribute && el.getAttribute('role') === m[1]
          }
          if (sel.startsWith('[contenteditable=')) return false
          if (sel.startsWith('[onclick]')) return false
          if (sel.startsWith('[tabindex]')) return false
          return false
        })
      },
      appendChild: () => {},
      children: { length: 0 }
    },
    documentElement: { children: { length: 0 } },
    createTreeWalker: (root, filter) => {
      const textNodes = []
      for (const el of elements) {
        if (el.textContent) {
          textNodes.push({ textContent: el.textContent, parentElement: el })
        }
      }
      let idx = -1
      return {
        currentNode: null,
        nextNode() {
          idx++
          if (idx < textNodes.length) {
            this.currentNode = textNodes[idx]
            return textNodes[idx]
          }
          return null
        }
      }
    },
    getSelection: () => null,
    execCommand: () => {}
  }
}

// ---------------------------------------------------------------------------
// #336: click auto-scrolls off-screen elements into view
// ---------------------------------------------------------------------------
describe('#336: click auto-scrolls off-screen elements into view', () => {
  beforeEach(() => {
    perfNowValue = 0
  })

  test('click calls scrollIntoView on element outside viewport', async () => {
    const btn = makeElement('BUTTON', {
      id: 'below-fold',
      textContent: 'Settings',
      rect: { width: 100, height: 30, top: 2000, left: 10, right: 110, bottom: 2030, x: 10, y: 2000 }
    })
    globalThis.window = { innerHeight: 800, innerWidth: 1200 }
    setupDocumentForCSS([btn])

    const result = await domPrimitive('click', '#below-fold', {})
    assert.strictEqual(result.success, true, 'click should succeed')
    assert.ok(
      btn._scrollIntoViewCalls.length > 0,
      'scrollIntoView should be called for off-screen elements'
    )
    assert.deepStrictEqual(
      btn._scrollIntoViewCalls[0],
      { behavior: 'instant', block: 'center' },
      'scrollIntoView should use instant behavior and center block'
    )
  })

  test('click does NOT call scrollIntoView when element is in viewport', async () => {
    const btn = makeElement('BUTTON', {
      id: 'in-view',
      textContent: 'Save',
      rect: { width: 100, height: 30, top: 100, left: 10, right: 110, bottom: 130, x: 10, y: 100 }
    })
    globalThis.window = { innerHeight: 800, innerWidth: 1200 }
    setupDocumentForCSS([btn])

    const result = await domPrimitive('click', '#in-view', {})
    assert.strictEqual(result.success, true, 'click should succeed')
    assert.strictEqual(
      btn._scrollIntoViewCalls.length,
      0,
      'scrollIntoView should NOT be called for visible elements'
    )
  })

  test('click scrolls element that is above viewport (negative top)', async () => {
    const btn = makeElement('BUTTON', {
      id: 'above-fold',
      textContent: 'Back',
      rect: { width: 100, height: 30, top: -200, left: 10, right: 110, bottom: -170, x: 10, y: -200 }
    })
    globalThis.window = { innerHeight: 800, innerWidth: 1200 }
    setupDocumentForCSS([btn])

    const result = await domPrimitive('click', '#above-fold', {})
    assert.strictEqual(result.success, true)
    assert.ok(
      btn._scrollIntoViewCalls.length > 0,
      'scrollIntoView should be called for elements above viewport'
    )
  })

  test('auto_scrolled flag is set in result when scroll happened', async () => {
    const btn = makeElement('BUTTON', {
      id: 'off-screen',
      textContent: 'Action',
      rect: { width: 100, height: 30, top: 2000, left: 10, right: 110, bottom: 2030, x: 10, y: 2000 }
    })
    globalThis.window = { innerHeight: 800, innerWidth: 1200 }
    setupDocumentForCSS([btn])

    const result = await domPrimitive('click', '#off-screen', {})
    assert.strictEqual(result.success, true)
    assert.strictEqual(result.auto_scrolled, true, 'result should indicate that auto-scroll occurred')
  })
})

// ---------------------------------------------------------------------------
// #332: click bubbles up to nearest interactive ancestor
// ---------------------------------------------------------------------------
describe('#332: click bubbles up to nearest interactive ancestor', () => {
  beforeEach(() => {
    perfNowValue = 0
  })

  test('text= match on span inside button clicks the button', async () => {
    const button = makeElement('BUTTON', {
      id: 'save-btn',
      textContent: 'Save',
      rect: { width: 100, height: 30, top: 10, left: 10, right: 110, bottom: 40, x: 10, y: 10 }
    })
    button._matchesSelector = (sel) => {
      const tags = sel.split(',').map(s => s.trim())
      return tags.some(t => t === 'button' || t.includes('button'))
    }
    const span = makeElement('SPAN', {
      textContent: 'Save',
      rect: { width: 80, height: 20, top: 15, left: 15, right: 95, bottom: 35, x: 15, y: 15 }
    })
    span.parentElement = button
    span._matchesSelector = () => false
    span.closest = (sel) => {
      if (sel.includes('button')) return button
      if (sel.includes('a')) return null
      const interactivePatterns = ['a', 'button', '[role="button"]', 'input', 'select', 'textarea']
      for (const pattern of interactivePatterns) {
        if (sel.includes(pattern)) return button
      }
      return null
    }

    globalThis.window = { innerHeight: 800, innerWidth: 1200 }
    const textElements = [span]
    globalThis.document = {
      querySelector: (sel) => {
        if (sel === '#save-btn') return button
        return null
      },
      querySelectorAll: (sel) => {
        if (sel === 'button') return [button]
        return []
      },
      getElementById: () => null,
      body: {
        querySelectorAll: (sel) => {
          if (sel === 'button') return [button]
          return []
        },
        appendChild: () => {},
        children: { length: 0 }
      },
      documentElement: { children: { length: 0 } },
      createTreeWalker: () => {
        let idx = -1
        return {
          currentNode: null,
          nextNode() {
            idx++
            if (idx < textElements.length) {
              this.currentNode = { textContent: textElements[idx].textContent, parentElement: textElements[idx] }
              return this.currentNode
            }
            return null
          }
        }
      },
      getSelection: () => null,
      execCommand: () => {}
    }

    const result = await domPrimitive('click', 'text=Save', {})
    assert.strictEqual(result.success, true, 'click should succeed')
    assert.strictEqual(
      result.matched.tag,
      'button',
      'matched target should be the interactive ancestor (button), not the wrapper span'
    )
  })

  test('CSS match on wrapper div inside anchor clicks the anchor', async () => {
    const anchor = makeElement('A', {
      id: 'nav-link',
      textContent: 'Dashboard',
      rect: { width: 120, height: 30, top: 10, left: 10, right: 130, bottom: 40, x: 10, y: 10 }
    })
    anchor._matchesSelector = (sel) => sel.includes('a')

    const wrapper = makeElement('DIV', {
      id: 'nav-wrapper',
      textContent: 'Dashboard',
      rect: { width: 120, height: 30, top: 10, left: 10, right: 130, bottom: 40, x: 10, y: 10 }
    })
    wrapper.parentElement = anchor
    wrapper._matchesSelector = () => false
    wrapper.closest = (sel) => {
      if (sel.includes('a') || sel.includes('button') || sel.includes('[role="button"]')) return anchor
      return null
    }

    globalThis.window = { innerHeight: 800, innerWidth: 1200 }
    setupDocumentForCSS([wrapper], (sel) => {
      if (sel === '#nav-wrapper') return wrapper
      return null
    })

    const result = await domPrimitive('click', '#nav-wrapper', {})
    assert.strictEqual(result.success, true)
    assert.strictEqual(
      result.matched.tag,
      'a',
      'click target should bubble up from wrapper div to nearest interactive ancestor (anchor)'
    )
  })

  test('does NOT bubble up when matched element is already interactive', async () => {
    const button = makeElement('BUTTON', {
      id: 'action-btn',
      textContent: 'Submit',
      rect: { width: 100, height: 30, top: 10, left: 10, right: 110, bottom: 40, x: 10, y: 10 }
    })
    button._matchesSelector = (sel) => sel.includes('button')

    globalThis.window = { innerHeight: 800, innerWidth: 1200 }
    setupDocumentForCSS([button])

    const result = await domPrimitive('click', '#action-btn', {})
    assert.strictEqual(result.success, true)
    assert.strictEqual(result.matched.tag, 'button', 'already-interactive element should not be changed')
  })

  test('bubbles up to [role=button] parent', async () => {
    const roleBtn = makeElement('DIV', {
      id: 'role-btn',
      textContent: 'Click Me',
      rect: { width: 100, height: 30, top: 10, left: 10, right: 110, bottom: 40, x: 10, y: 10 }
    })
    roleBtn._role = 'button'
    roleBtn._matchesSelector = (sel) => sel.includes('[role="button"]')
    roleBtn.getAttribute = (name) => {
      if (name === 'role') return 'button'
      return null
    }

    const inner = makeElement('SPAN', {
      id: 'inner-text',
      textContent: 'Click Me',
      rect: { width: 80, height: 20, top: 15, left: 15, right: 95, bottom: 35, x: 15, y: 15 }
    })
    inner.parentElement = roleBtn
    inner._matchesSelector = () => false
    inner.closest = (sel) => {
      if (sel.includes('[role="button"]')) return roleBtn
      return null
    }

    globalThis.window = { innerHeight: 800, innerWidth: 1200 }
    setupDocumentForCSS([inner], (sel) => {
      if (sel === '#inner-text') return inner
      return null
    })

    const result = await domPrimitive('click', '#inner-text', {})
    assert.strictEqual(result.success, true)
    assert.strictEqual(
      result.matched.tag,
      'div',
      'should bubble up to [role=button] ancestor'
    )
    assert.strictEqual(
      result.matched.role,
      'button',
      'matched element should have role=button'
    )
  })
})

// ---------------------------------------------------------------------------
// #333: scroll_to with nested scrollable containers
// ---------------------------------------------------------------------------
describe('#333: scroll_to finds nested scrollable containers', () => {
  beforeEach(() => {
    perfNowValue = 0
  })

  test('scroll_to calls scrollIntoView on element for reliable nested-container support', () => {
    const target = makeElement('DIV', {
      id: 'target',
      textContent: 'Target',
      rect: { width: 100, height: 30, top: 1500, left: 10, right: 110, bottom: 1530, x: 10, y: 1500 }
    })

    globalThis.window = { innerHeight: 800, innerWidth: 1200 }
    setupDocumentForCSS([target])

    const result = domPrimitive('scroll_to', '#target', {})
    assert.strictEqual(result.success, true, 'scroll_to should succeed')
    assert.ok(
      target._scrollIntoViewCalls.length > 0,
      'scroll_to should call scrollIntoView on the target element'
    )
  })

  test('scroll_to uses smooth behavior', () => {
    const target = makeElement('DIV', {
      id: 'content',
      textContent: 'Content',
      rect: { width: 200, height: 50, top: 2000, left: 0, right: 200, bottom: 2050, x: 0, y: 2000 }
    })

    globalThis.window = { innerHeight: 800, innerWidth: 1200 }
    setupDocumentForCSS([target])

    const result = domPrimitive('scroll_to', '#content', {})
    assert.strictEqual(result.success, true)
    assert.deepStrictEqual(
      target._scrollIntoViewCalls[0],
      { behavior: 'smooth', block: 'center' },
      'scroll_to should use smooth behavior and center block'
    )
  })
})

// ---------------------------------------------------------------------------
// #316: ambiguous text= selector warning metadata
// ---------------------------------------------------------------------------
describe('#316: ambiguous text selector warning metadata', () => {
  beforeEach(() => {
    perfNowValue = 0
    delete globalThis.__gasolineElementHandles
  })

  test('text= selector with multiple matches includes warning metadata', async () => {
    const settings1 = makeElement('A', {
      id: 'settings-1',
      textContent: 'Settings',
      rect: { width: 80, height: 20, top: 100, left: 10, right: 90, bottom: 120, x: 10, y: 100 }
    })
    settings1._matchesSelector = (sel) => sel.includes('a')
    settings1.closest = (sel) => {
      if (sel.includes('a')) return settings1
      return null
    }

    const settings2 = makeElement('A', {
      id: 'settings-2',
      textContent: 'Settings',
      rect: { width: 80, height: 20, top: 300, left: 10, right: 90, bottom: 320, x: 10, y: 300 }
    })
    settings2._matchesSelector = (sel) => sel.includes('a')
    settings2.closest = (sel) => {
      if (sel.includes('a')) return settings2
      return null
    }

    const settings3 = makeElement('A', {
      id: 'settings-3',
      textContent: 'Settings',
      rect: { width: 80, height: 20, top: 500, left: 10, right: 90, bottom: 520, x: 10, y: 500 }
    })
    settings3._matchesSelector = (sel) => sel.includes('a')
    settings3.closest = (sel) => {
      if (sel.includes('a')) return settings3
      return null
    }

    const allSettings = [settings1, settings2, settings3]

    globalThis.window = { innerHeight: 800, innerWidth: 1200 }
    globalThis.document = {
      querySelector: (sel) => {
        for (const el of allSettings) {
          if (el.id && sel === `#${el.id}`) return el
        }
        return null
      },
      querySelectorAll: (sel) => {
        if (sel === 'a[href]') return allSettings
        return allSettings.filter(el => {
          const tag = el.tagName.toLowerCase()
          if (sel === tag) return true
          return false
        })
      },
      getElementById: (id) => allSettings.find(el => el.id === id) || null,
      body: {
        querySelectorAll: () => [],
        appendChild: () => {},
        children: { length: 0 }
      },
      documentElement: { children: { length: 0 } },
      createTreeWalker: () => {
        let idx = -1
        return {
          currentNode: null,
          nextNode() {
            idx++
            if (idx < allSettings.length) {
              this.currentNode = { textContent: allSettings[idx].textContent, parentElement: allSettings[idx] }
              return this.currentNode
            }
            return null
          }
        }
      },
      getSelection: () => null,
      execCommand: () => {}
    }

    const result = domPrimitive('get_text', 'text=Settings', {})
    assert.strictEqual(result.success, true, 'get_text with text= should still succeed')
    assert.ok(
      result.ambiguous_matches,
      'result should include ambiguous_matches metadata when text= matches multiple elements'
    )
    assert.strictEqual(
      result.ambiguous_matches.total_count,
      3,
      'ambiguous_matches should report the total number of matches'
    )
    assert.ok(
      result.ambiguous_matches.warning,
      'ambiguous_matches should include a warning message'
    )
  })

  test('text= selector with single match does NOT include warning', async () => {
    const uniqueEl = makeElement('BUTTON', {
      id: 'unique-btn',
      textContent: 'Unique Button Label',
      rect: { width: 100, height: 30, top: 10, left: 10, right: 110, bottom: 40, x: 10, y: 10 }
    })
    uniqueEl._matchesSelector = (sel) => sel.includes('button')
    uniqueEl.closest = (sel) => {
      if (sel.includes('button')) return uniqueEl
      return null
    }

    globalThis.window = { innerHeight: 800, innerWidth: 1200 }
    globalThis.document = {
      querySelector: (sel) => {
        if (sel === `#${uniqueEl.id}`) return uniqueEl
        return null
      },
      querySelectorAll: () => [],
      getElementById: (id) => (id === uniqueEl.id ? uniqueEl : null),
      body: {
        querySelectorAll: () => [],
        appendChild: () => {},
        children: { length: 0 }
      },
      documentElement: { children: { length: 0 } },
      createTreeWalker: () => {
        let done = false
        return {
          currentNode: null,
          nextNode() {
            if (!done) {
              done = true
              this.currentNode = { textContent: uniqueEl.textContent, parentElement: uniqueEl }
              return this.currentNode
            }
            return null
          }
        }
      },
      getSelection: () => null,
      execCommand: () => {}
    }

    const result = domPrimitive('get_text', 'text=Unique Button Label', {})
    assert.strictEqual(result.success, true)
    assert.strictEqual(
      result.ambiguous_matches,
      undefined,
      'should NOT include ambiguous_matches when only 1 element matches'
    )
  })

  test('ambiguous_matches includes candidate details for disambiguation', async () => {
    const btn1 = makeElement('BUTTON', {
      id: 'save-1',
      textContent: 'Save',
      rect: { width: 80, height: 30, top: 50, left: 10, right: 90, bottom: 80, x: 10, y: 50 }
    })
    btn1._matchesSelector = (sel) => sel.includes('button')
    btn1.closest = (sel) => {
      if (sel.includes('button')) return btn1
      return null
    }

    const btn2 = makeElement('BUTTON', {
      id: 'save-2',
      textContent: 'Save',
      rect: { width: 80, height: 30, top: 200, left: 10, right: 90, bottom: 230, x: 10, y: 200 }
    })
    btn2._matchesSelector = (sel) => sel.includes('button')
    btn2.closest = (sel) => {
      if (sel.includes('button')) return btn2
      return null
    }

    const allBtns = [btn1, btn2]

    globalThis.window = { innerHeight: 800, innerWidth: 1200 }
    globalThis.document = {
      querySelector: (sel) => {
        for (const el of allBtns) {
          if (el.id && sel === `#${el.id}`) return el
        }
        return null
      },
      querySelectorAll: () => [],
      getElementById: (id) => allBtns.find(el => el.id === id) || null,
      body: {
        querySelectorAll: () => [],
        appendChild: () => {},
        children: { length: 0 }
      },
      documentElement: { children: { length: 0 } },
      createTreeWalker: () => {
        let idx = -1
        return {
          currentNode: null,
          nextNode() {
            idx++
            if (idx < allBtns.length) {
              this.currentNode = { textContent: allBtns[idx].textContent, parentElement: allBtns[idx] }
              return this.currentNode
            }
            return null
          }
        }
      },
      getSelection: () => null,
      execCommand: () => {}
    }

    const result = domPrimitive('get_text', 'text=Save', {})
    assert.strictEqual(result.success, true)
    assert.ok(result.ambiguous_matches, 'should include ambiguous_matches for 2+ matches')
    assert.ok(
      Array.isArray(result.ambiguous_matches.candidates),
      'ambiguous_matches should include candidates array'
    )
    assert.ok(
      result.ambiguous_matches.candidates.length <= 5,
      'candidates should be capped at a reasonable number'
    )
    for (const c of result.ambiguous_matches.candidates) {
      assert.ok(c.tag, 'candidate should have tag')
    }
  })
})
