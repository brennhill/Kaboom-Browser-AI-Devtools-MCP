// @ts-nocheck
/**
 * @fileoverview On-demand DOM query and page-information tests.
 */

import { test, describe, mock, beforeEach, afterEach } from 'node:test'
import assert from 'node:assert'
import { createMockDocument, createMockWindow } from './on-demand-fixture.js'

let originalDocument
let originalWindow


describe('DOM Query Execution', () => {
  beforeEach(() => {
    originalDocument = globalThis.document
    originalWindow = globalThis.window
    globalThis.document = createMockDocument()
    globalThis.window = createMockWindow()
  })

  afterEach(() => {
    globalThis.document = originalDocument
    globalThis.window = originalWindow
  })

  test('should execute querySelectorAll with given selector', async () => {
    const { executeDOMQuery } = await import('../../extension/lib/analysis/dom-queries.js')

    globalThis.document.querySelectorAll = mock.fn(() => [
      {
        tagName: 'H1',
        textContent: 'Hello World',
        getAttribute: (_name) => null,
        attributes: [],
        getBoundingClientRect: () => ({ x: 0, y: 0, width: 100, height: 30 }),
        children: [],
        offsetParent: {}
      }
    ])

    const result = await executeDOMQuery({ selector: 'h1' })

    assert.strictEqual(result.matchCount, 1)
    assert.strictEqual(result.matches[0].tag, 'h1')
    assert.strictEqual(result.matches[0].text, 'Hello World')
  })

  test('should include attributes', async () => {
    const { executeDOMQuery } = await import('../../extension/lib/analysis/dom-queries.js')

    globalThis.document.querySelectorAll = mock.fn(() => [
      {
        tagName: 'DIV',
        textContent: '',
        attributes: [
          { name: 'class', value: 'user-card active' },
          { name: 'data-id', value: '42' }
        ],
        getAttribute: (name) => {
          if (name === 'class') return 'user-card active'
          if (name === 'data-id') return '42'
          return null
        },
        getBoundingClientRect: () => ({ x: 0, y: 0, width: 300, height: 100 }),
        children: [],
        offsetParent: {}
      }
    ])

    const result = await executeDOMQuery({ selector: '.user-card' })

    assert.strictEqual(result.matches[0].attributes.class, 'user-card active')
    assert.strictEqual(result.matches[0].attributes['data-id'], '42')
  })

  test('should include bounding box', async () => {
    const { executeDOMQuery } = await import('../../extension/lib/analysis/dom-queries.js')

    globalThis.document.querySelectorAll = mock.fn(() => [
      {
        tagName: 'DIV',
        textContent: '',
        attributes: [],
        getAttribute: () => null,
        getBoundingClientRect: () => ({ x: 20, y: 140, width: 300, height: 48 }),
        children: [],
        offsetParent: {}
      }
    ])

    const result = await executeDOMQuery({ selector: 'div' })

    assert.deepStrictEqual(result.matches[0].boundingBox, { x: 20, y: 140, width: 300, height: 48 })
  })

  test('should detect visibility', async () => {
    const { executeDOMQuery } = await import('../../extension/lib/analysis/dom-queries.js')

    globalThis.document.querySelectorAll = mock.fn(() => [
      {
        tagName: 'DIV',
        textContent: 'visible',
        attributes: [],
        getAttribute: () => null,
        getBoundingClientRect: () => ({ x: 0, y: 0, width: 100, height: 50 }),
        children: [],
        offsetParent: {} // non-null means visible
      },
      {
        tagName: 'DIV',
        textContent: 'hidden',
        attributes: [],
        getAttribute: () => null,
        getBoundingClientRect: () => ({ x: 0, y: 0, width: 0, height: 0 }),
        children: [],
        offsetParent: null // null means hidden
      }
    ])

    const result = await executeDOMQuery({ selector: 'div' })

    assert.strictEqual(result.matches[0].visible, true)
    assert.strictEqual(result.matches[1].visible, false)
  })

  test('should include computed styles when requested', async () => {
    const { executeDOMQuery } = await import('../../extension/lib/analysis/dom-queries.js')

    globalThis.window.getComputedStyle = mock.fn(() => ({
      display: 'flex',
      color: 'rgb(0, 0, 0)',
      position: 'relative'
    }))

    globalThis.document.querySelectorAll = mock.fn(() => [
      {
        tagName: 'DIV',
        textContent: '',
        attributes: [],
        getAttribute: () => null,
        getBoundingClientRect: () => ({ x: 0, y: 0, width: 100, height: 50 }),
        children: [],
        offsetParent: {}
      }
    ])

    const result = await executeDOMQuery({ selector: 'div', include_styles: true })

    assert.ok(result.matches[0].styles, 'Expected styles in result')
    assert.strictEqual(result.matches[0].styles.display, 'flex')
  })

  test('should include only specified style properties', async () => {
    const { executeDOMQuery } = await import('../../extension/lib/analysis/dom-queries.js')

    const styles = {
      display: 'flex',
      color: 'rgb(0, 0, 0)',
      position: 'relative',
      margin: '10px',
      padding: '5px'
    }
    globalThis.window.getComputedStyle = mock.fn(() => ({
      ...styles,
      getPropertyValue: (prop) => styles[prop] || ''
    }))

    globalThis.document.querySelectorAll = mock.fn(() => [
      {
        tagName: 'DIV',
        textContent: '',
        attributes: [],
        getAttribute: () => null,
        getBoundingClientRect: () => ({ x: 0, y: 0, width: 100, height: 50 }),
        children: [],
        offsetParent: {}
      }
    ])

    const result = await executeDOMQuery({
      selector: 'div',
      include_styles: true,
      properties: ['display', 'color']
    })

    assert.strictEqual(Object.keys(result.matches[0].styles).length, 2)
    assert.strictEqual(result.matches[0].styles.display, 'flex')
    assert.strictEqual(result.matches[0].styles.color, 'rgb(0, 0, 0)')
  })

  test('should include children when requested', async () => {
    const { executeDOMQuery } = await import('../../extension/lib/analysis/dom-queries.js')

    const childElement = {
      tagName: 'SPAN',
      textContent: 'child text',
      attributes: [{ name: 'class', value: 'name' }],
      getAttribute: (name) => (name === 'class' ? 'name' : null),
      children: []
    }

    globalThis.document.querySelectorAll = mock.fn(() => [
      {
        tagName: 'LI',
        textContent: 'child text',
        attributes: [],
        getAttribute: () => null,
        getBoundingClientRect: () => ({ x: 0, y: 0, width: 300, height: 48 }),
        children: [childElement],
        offsetParent: {}
      }
    ])

    const result = await executeDOMQuery({ selector: 'li', include_children: true })

    assert.ok(result.matches[0].children, 'Expected children array')
    assert.strictEqual(result.matches[0].children.length, 1)
    assert.strictEqual(result.matches[0].children[0].tag, 'span')
    assert.strictEqual(result.matches[0].children[0].text, 'child text')
  })

  test('should limit child depth to max_depth', async () => {
    const { executeDOMQuery } = await import('../../extension/lib/analysis/dom-queries.js')

    // Create deeply nested structure
    const makeNested = (depth) => ({
      tagName: 'DIV',
      textContent: `depth-${depth}`,
      attributes: [],
      getAttribute: () => null,
      children: depth > 0 ? [makeNested(depth - 1)] : []
    })

    globalThis.document.querySelectorAll = mock.fn(() => [
      {
        ...makeNested(10),
        getBoundingClientRect: () => ({ x: 0, y: 0, width: 100, height: 100 }),
        offsetParent: {}
      }
    ])

    const result = await executeDOMQuery({ selector: 'div', include_children: true, max_depth: 3 })

    // Should not go deeper than 3 levels
    let depth = 0
    let current = result.matches[0]
    while (current.children && current.children.length > 0) {
      depth++
      current = current.children[0]
    }

    assert.ok(depth <= 3, `Expected max depth 3, got ${depth}`)
  })

  test('should limit max_depth to 5 even if higher requested', async () => {
    const { executeDOMQuery } = await import('../../extension/lib/analysis/dom-queries.js')

    const makeNested = (depth) => ({
      tagName: 'DIV',
      textContent: `depth-${depth}`,
      attributes: [],
      getAttribute: () => null,
      children: depth > 0 ? [makeNested(depth - 1)] : []
    })

    globalThis.document.querySelectorAll = mock.fn(() => [
      {
        ...makeNested(10),
        getBoundingClientRect: () => ({ x: 0, y: 0, width: 100, height: 100 }),
        offsetParent: {}
      }
    ])

    const result = await executeDOMQuery({ selector: 'div', include_children: true, max_depth: 20 })

    let depth = 0
    let current = result.matches[0]
    while (current.children && current.children.length > 0) {
      depth++
      current = current.children[0]
    }

    assert.ok(depth <= 5, `Expected max depth capped at 5, got ${depth}`)
  })

  test('should limit to 50 elements max', async () => {
    const { executeDOMQuery } = await import('../../extension/lib/analysis/dom-queries.js')

    const elements = Array.from({ length: 100 }, (_, i) => ({
      tagName: 'LI',
      textContent: `Item ${i}`,
      attributes: [],
      getAttribute: () => null,
      getBoundingClientRect: () => ({ x: 0, y: i * 20, width: 200, height: 20 }),
      children: [],
      offsetParent: {}
    }))

    globalThis.document.querySelectorAll = mock.fn(() => elements)

    const result = await executeDOMQuery({ selector: 'li' })

    assert.strictEqual(result.returnedCount, 50)
    assert.strictEqual(result.matchCount, 100)
  })

  test('should truncate text content at 500 chars', async () => {
    const { executeDOMQuery } = await import('../../extension/lib/analysis/dom-queries.js')

    const longText = 'x'.repeat(1000)
    globalThis.document.querySelectorAll = mock.fn(() => [
      {
        tagName: 'P',
        textContent: longText,
        attributes: [],
        getAttribute: () => null,
        getBoundingClientRect: () => ({ x: 0, y: 0, width: 300, height: 100 }),
        children: [],
        offsetParent: {}
      }
    ])

    const result = await executeDOMQuery({ selector: 'p' })

    assert.ok(result.matches[0].text.length <= 500)
  })

  test('should include page URL and title in response', async () => {
    const { executeDOMQuery } = await import('../../extension/lib/analysis/dom-queries.js')

    globalThis.document.title = 'My App - Dashboard'
    globalThis.document.querySelectorAll = mock.fn(() => [])

    const result = await executeDOMQuery({ selector: 'nonexistent' })

    assert.strictEqual(result.url, 'http://localhost:3000/dashboard')
    assert.strictEqual(result.title, 'My App - Dashboard')
  })
})

describe('Page Info', () => {
  beforeEach(() => {
    originalDocument = globalThis.document
    originalWindow = globalThis.window
    globalThis.document = createMockDocument()
    globalThis.window = createMockWindow()
  })

  afterEach(() => {
    globalThis.document = originalDocument
    globalThis.window = originalWindow
  })

  test('should return basic page info', async () => {
    const { getPageInfo } = await import('../../extension/lib/analysis/dom-queries.js')

    globalThis.document.title = 'My App - Dashboard'
    globalThis.document.querySelectorAll = mock.fn((selector) => {
      if (selector === 'h1,h2,h3,h4,h5,h6') return [{ textContent: 'Dashboard' }, { textContent: 'Settings' }]
      if (selector === 'a') return Array(24).fill({})
      if (selector === 'img') return Array(8).fill({})
      if (selector === 'button,input,select,textarea,a[href]') return Array(15).fill({})
      if (selector === 'form') return []
      return []
    })

    const info = await getPageInfo()

    assert.strictEqual(info.url, 'http://localhost:3000/dashboard')
    assert.strictEqual(info.title, 'My App - Dashboard')
    assert.deepStrictEqual(info.viewport, { width: 1440, height: 900 })
    assert.deepStrictEqual(info.scroll, { x: 0, y: 320 })
    assert.strictEqual(info.documentHeight, 2400)
  })

  test('should list headings', async () => {
    const { getPageInfo } = await import('../../extension/lib/analysis/dom-queries.js')

    globalThis.document.querySelectorAll = mock.fn((selector) => {
      if (selector === 'h1,h2,h3,h4,h5,h6') {
        return [{ textContent: 'Dashboard' }, { textContent: 'Recent Activity' }, { textContent: 'Settings' }]
      }
      return []
    })

    const info = await getPageInfo()

    assert.deepStrictEqual(info.headings, ['Dashboard', 'Recent Activity', 'Settings'])
  })

  test('should list forms with fields', async () => {
    const { getPageInfo } = await import('../../extension/lib/analysis/dom-queries.js')

    globalThis.document.querySelectorAll = mock.fn((selector) => {
      if (selector === 'form') {
        return [
          {
            id: 'login-form',
            action: '/api/login',
            querySelectorAll: () => [
              { name: 'email', tagName: 'INPUT' },
              { name: 'password', tagName: 'INPUT' }
            ]
          }
        ]
      }
      return []
    })

    const info = await getPageInfo()

    assert.strictEqual(info.forms.length, 1)
    assert.strictEqual(info.forms[0].id, 'login-form')
    assert.strictEqual(info.forms[0].action, '/api/login')
    assert.deepStrictEqual(info.forms[0].fields, ['email', 'password'])
  })

  test('should count links, images, and interactive elements', async () => {
    const { getPageInfo } = await import('../../extension/lib/analysis/dom-queries.js')

    globalThis.document.querySelectorAll = mock.fn((selector) => {
      if (selector === 'a') return Array(24).fill({})
      if (selector === 'img') return Array(8).fill({})
      if (selector === 'button,input,select,textarea,a[href]') return Array(15).fill({})
      return []
    })

    const info = await getPageInfo()

    assert.strictEqual(info.links, 24)
    assert.strictEqual(info.images, 8)
    assert.strictEqual(info.interactiveElements, 15)
  })
})
