// @ts-nocheck
import { beforeEach, describe, test } from 'node:test'
import assert from 'node:assert'

class MockHTMLElement {
  constructor(tag, options = {}) {
    this.tagName = tag.toUpperCase()
    this.style = {
      overflow: options.style?.overflow || '',
      overflowY: options.style?.overflowY || '',
      overflowX: options.style?.overflowX || '',
      height: options.style?.height || '',
      minHeight: options.style?.minHeight || '',
      maxHeight: options.style?.maxHeight || '',
      flex: options.style?.flex || '',
      contain: options.style?.contain || ''
    }
    this._computed = {
      overflow: options.computed?.overflow || '',
      overflowY: options.computed?.overflowY || '',
      overflowX: options.computed?.overflowX || ''
    }
    this._attrs = {}
    this.scrollHeight = options.scrollHeight || 0
    this.clientHeight = options.clientHeight || 0
    this._top = options.top || 0
    this._left = options.left || 0
    this._width = options.width || 100
  }

  setAttribute(name, value) {
    this._attrs[name] = value
  }

  getAttribute(name) {
    return this._attrs[name] || null
  }

  removeAttribute(name) {
    delete this._attrs[name]
  }

  getBoundingClientRect() {
    return {
      x: this._left,
      y: this._top,
      left: this._left,
      top: this._top,
      width: this._width,
      height: this.clientHeight,
      right: this._left + this._width,
      bottom: this._top + this.clientHeight
    }
  }
}

globalThis.HTMLElement = MockHTMLElement

const { screenshotExpandContainers, screenshotRestoreContainers, computeFullPageCaptureDimensions } =
  await import('./observe.js')

describe('observe full-page helpers', () => {
  let htmlEl
  let bodyEl
  let mainScrollable
  let allElements

  beforeEach(() => {
    htmlEl = new MockHTMLElement('html', {
      computed: { overflowY: 'visible' },
      scrollHeight: 1000,
      clientHeight: 1000
    })
    bodyEl = new MockHTMLElement('body', {
      computed: { overflowY: 'visible' },
      scrollHeight: 1000,
      clientHeight: 1000
    })
    mainScrollable = new MockHTMLElement('main', {
      style: { overflow: 'auto', height: '100vh', maxHeight: '100vh', flex: '1 1 auto', contain: 'layout' },
      computed: { overflowY: 'auto', overflow: 'auto' },
      scrollHeight: 2400,
      clientHeight: 800,
      top: 80
    })

    allElements = [mainScrollable]

    bodyEl.querySelectorAll = (selector) => (selector === '*' ? allElements : [])

    globalThis.window = { scrollY: 0 }
    globalThis.getComputedStyle = (el) => el._computed
    globalThis.document = {
      documentElement: htmlEl,
      body: bodyEl,
      querySelectorAll: (selector) => {
        if (selector !== '[data-kaboom-fpx]') return []
        return [htmlEl, bodyEl, ...allElements].filter((el) => !!el.getAttribute('data-kaboom-fpx'))
      }
    }
  })

  test('expands nested scroll container and reports content height hint', () => {
    const result = screenshotExpandContainers()
    assert.strictEqual(result.expanded, 1)
    assert.ok(result.content_height_hint >= 2480, 'content height hint should include nested scroll content')
    assert.strictEqual(mainScrollable.style.overflow, 'visible')
    assert.strictEqual(mainScrollable.style.overflowY, 'visible')
    assert.strictEqual(mainScrollable.style.height, '2400px')
    assert.strictEqual(mainScrollable.style.minHeight, '2400px')
    assert.strictEqual(mainScrollable.style.maxHeight, 'none')
    assert.strictEqual(mainScrollable.style.flex, 'none')
    assert.strictEqual(mainScrollable.style.contain, 'none')
    assert.ok(mainScrollable.getAttribute('data-kaboom-fpx'))
  })

  test('restores original styles after expansion', () => {
    screenshotExpandContainers()
    screenshotRestoreContainers()

    assert.strictEqual(mainScrollable.style.overflow, 'auto')
    assert.strictEqual(mainScrollable.style.overflowY, '')
    assert.strictEqual(mainScrollable.style.height, '100vh')
    assert.strictEqual(mainScrollable.style.minHeight, '')
    assert.strictEqual(mainScrollable.style.maxHeight, '100vh')
    assert.strictEqual(mainScrollable.style.flex, '1 1 auto')
    assert.strictEqual(mainScrollable.style.contain, 'layout')
    assert.strictEqual(mainScrollable.getAttribute('data-kaboom-fpx'), null)
  })

  test('restores gracefully when stored expansion data is malformed', () => {
    mainScrollable.setAttribute('data-kaboom-fpx', '{invalid-json')
    screenshotRestoreContainers()
    assert.strictEqual(mainScrollable.getAttribute('data-kaboom-fpx'), null)
  })
})

describe('computeFullPageCaptureDimensions', () => {
  test('uses expanded content hint and clamps to Chrome limits', () => {
    const result = computeFullPageCaptureDimensions(20000, 1200, 18000)
    assert.deepStrictEqual(result, { width: 16384, height: 16384 })
  })

  test('falls back to defaults for invalid metrics', () => {
    const result = computeFullPageCaptureDimensions(NaN, 0, -10)
    assert.deepStrictEqual(result, { width: 1280, height: 720 })
  })

  test('prefers hinted height when larger than content height', () => {
    const result = computeFullPageCaptureDimensions(1400, 900, 2400)
    assert.deepStrictEqual(result, { width: 1400, height: 2400 })
  })
})
