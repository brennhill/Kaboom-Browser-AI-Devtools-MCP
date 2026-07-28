// @ts-nocheck
/**
 * @fileoverview on-demand.test.js — Tests for on-demand DOM/a11y query system.
 * Covers CSS selector query execution, accessibility tree audits, pending query
 * polling from the server, response posting, and error handling for invalid selectors.
 */

import { test, describe, mock, beforeEach, afterEach, after } from 'node:test'
import assert from 'node:assert'
import { MANIFEST_VERSION } from './helpers.js'

// Mock Chrome APIs
const createMockChrome = () => ({
  runtime: {
    onMessage: { addListener: mock.fn() },
    sendMessage: mock.fn(() => Promise.resolve()),
    getURL: mock.fn((path) => `chrome-extension://test-id/${path}`),
    getManifest: () => ({ version: MANIFEST_VERSION })
  },
  tabs: {
    query: mock.fn((query, callback) => callback([{ id: 1, windowId: 1, url: 'http://localhost:3000' }])),
    get: mock.fn((tabId) => Promise.resolve({ id: tabId, windowId: 1, url: 'http://localhost:3000' })),
    sendMessage: mock.fn((_tabId, _message) => Promise.resolve())
  },
  scripting: {
    executeScript: mock.fn(() => Promise.resolve([{ result: {} }]))
  },
  storage: {
    local: {
      get: mock.fn((keys, callback) => {
        const data = {
          serverUrl: 'http://localhost:7890',
          captureWebSockets: true,
          captureNetworkBodies: false,
          trackedTabId: 1
        }
        if (callback) callback(data)
        return Promise.resolve(data)
      }),
      set: mock.fn((data, callback) => {
        if (callback) callback()
        return Promise.resolve()
      }),
      remove: mock.fn((keys, callback) => {
        if (callback) callback()
        return Promise.resolve()
      })
    },
    sync: {
      get: mock.fn((keys, callback) => {
        if (callback) callback({})
        return Promise.resolve({})
      }),
      set: mock.fn((data, callback) => {
        if (callback) callback()
        return Promise.resolve()
      })
    },
    session: {
      get: mock.fn((keys, callback) => {
        if (callback) callback({})
        return Promise.resolve({})
      }),
      set: mock.fn((data, callback) => {
        if (callback) callback()
        return Promise.resolve()
      })
    },
    onChanged: {
      addListener: mock.fn()
    }
  }
})

const createMockDocument = () => ({
  querySelectorAll: mock.fn((_selector) => []),
  querySelector: mock.fn((_selector) => null),
  title: 'Test Page',
  readyState: 'complete',
  documentElement: {
    scrollHeight: 2400,
    scrollWidth: 1440
  },
  head: {
    appendChild: mock.fn()
  },
  createElement: mock.fn((tag) => ({
    tagName: tag.toUpperCase(),
    onload: null,
    onerror: null,
    src: '',
    setAttribute: mock.fn()
  }))
})

const createMockWindow = () => ({
  postMessage: mock.fn(),
  addEventListener: mock.fn(),
  location: { href: 'http://localhost:3000/dashboard' },
  innerWidth: 1440,
  innerHeight: 900,
  scrollX: 0,
  scrollY: 320,
  axe: null
})

// Set a baseline chrome mock so background.js async activity doesn't crash
globalThis.chrome = createMockChrome()

let originalDocument, originalWindow

// Track all setInterval calls so we can clean up leaked timers from module init
const activeIntervals = new Set()
const _originalSetInterval = globalThis.setInterval
const _originalClearInterval = globalThis.clearInterval
globalThis.setInterval = (...args) => {
  const id = _originalSetInterval(...args)
  activeIntervals.add(id)
  return id
}
globalThis.clearInterval = (id) => {
  activeIntervals.delete(id)
  _originalClearInterval(id)
}

// Clean up all leaked intervals after all tests complete
after(() => {
  for (const id of activeIntervals) {
    _originalClearInterval(id)
  }
  globalThis.setInterval = _originalSetInterval
  globalThis.clearInterval = _originalClearInterval
})

// Suppress unhandledRejection errors from background module initialization
process.on('unhandledRejection', (reason, _promise) => {
  // Suppress initialization errors from background.js module loading
  if (
    reason instanceof ReferenceError &&
    (reason.message?.includes('_connectionCheckRunning') ||
      reason.message?.includes('DebugCategory') ||
      reason.message?.includes('Cannot access'))
  ) {
    // Expected during test - background.js tries to access globals before init
    return
  }
  // Re-throw other unhandled rejections
  throw reason
})

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

describe('Accessibility Audit Execution', () => {
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

  test('should wait for axe-core to appear on window', async () => {
    const { runAxeAudit } = await import('../../extension/lib/analysis/dom-queries.js')

    globalThis.window.axe = null

    // Simulate content script injecting axe-core after 50ms
    // (loadAxeCore polls window.axe every 100ms with a 5s timeout)
    setTimeout(() => {
      globalThis.window.axe = {
        run: mock.fn(() =>
          Promise.resolve({
            violations: [],
            passes: [],
            incomplete: [],
            inapplicable: []
          })
        )
      }
    }, 50)

    const _result = await runAxeAudit({})

    assert.ok(globalThis.window.axe, 'axe-core should be loaded on window')
    assert.ok(globalThis.window.axe.run.mock.calls.length > 0, 'axe.run should have been called')
  })

  test('should reuse axe-core if already loaded', async () => {
    const { runAxeAudit } = await import('../../extension/lib/analysis/dom-queries.js')

    globalThis.window.axe = {
      run: mock.fn(() =>
        Promise.resolve({
          violations: [],
          passes: [],
          incomplete: [],
          inapplicable: []
        })
      )
    }

    await runAxeAudit({})

    // Should NOT have created a new script element
    assert.strictEqual(globalThis.document.createElement.mock.calls.length, 0)
    assert.strictEqual(globalThis.window.axe.run.mock.calls.length, 1)
  })

  test('should pass scope as context to axe.run', async () => {
    const { runAxeAudit } = await import('../../extension/lib/analysis/dom-queries.js')

    globalThis.window.axe = {
      run: mock.fn(() =>
        Promise.resolve({
          violations: [],
          passes: [],
          incomplete: [],
          inapplicable: []
        })
      )
    }

    await runAxeAudit({ scope: '#main-content' })

    const [context] = globalThis.window.axe.run.mock.calls[0].arguments
    assert.deepStrictEqual(context, { include: ['#main-content'] })
  })

  test('should pass tags as runOnly config', async () => {
    const { runAxeAudit } = await import('../../extension/lib/analysis/dom-queries.js')

    globalThis.window.axe = {
      run: mock.fn(() =>
        Promise.resolve({
          violations: [],
          passes: [],
          incomplete: [],
          inapplicable: []
        })
      )
    }

    await runAxeAudit({ tags: ['wcag2a', 'wcag2aa'] })

    const [, config] = globalThis.window.axe.run.mock.calls[0].arguments
    assert.deepStrictEqual(config.runOnly, ['wcag2a', 'wcag2aa'])
  })

  test('should include passes when include_passes is true', async () => {
    const { runAxeAudit } = await import('../../extension/lib/analysis/dom-queries.js')

    globalThis.window.axe = {
      run: mock.fn(() =>
        Promise.resolve({
          violations: [],
          passes: [{ id: 'button-name' }],
          incomplete: [],
          inapplicable: []
        })
      )
    }

    await runAxeAudit({ include_passes: true })

    const [, config] = globalThis.window.axe.run.mock.calls[0].arguments
    assert.ok(config.resultTypes.includes('passes'))
  })

  test('should format violations with selector, html, and fix suggestion', async () => {
    const { formatAxeResults } = await import('../../extension/lib/analysis/dom-queries.js')

    const axeResult = {
      violations: [
        {
          id: 'color-contrast',
          impact: 'serious',
          description: 'Elements must have sufficient color contrast',
          helpUrl: 'https://dequeuniversity.com/rules/axe/4.8/color-contrast',
          tags: ['wcag2aa', 'cat.color'],
          nodes: [
            {
              target: ['#signup-form > label:nth-child(2)'],
              html: '<label class="form-label subtle">Email address</label>',
              failureSummary: 'Element has insufficient color contrast of 2.8:1'
            }
          ]
        }
      ],
      passes: [],
      incomplete: [],
      inapplicable: []
    }

    const formatted = formatAxeResults(axeResult)

    assert.strictEqual(formatted.violations[0].id, 'color-contrast')
    assert.strictEqual(formatted.violations[0].impact, 'serious')
    assert.strictEqual(formatted.violations[0].nodes[0].selector, '#signup-form > label:nth-child(2)')
    assert.ok(formatted.violations[0].nodes[0].html.includes('form-label'))
  })

  test('should limit nodes per violation to 10', async () => {
    const { formatAxeResults } = await import('../../extension/lib/analysis/dom-queries.js')

    const nodes = Array.from({ length: 20 }, (_, i) => ({
      target: [`#node-${i}`],
      html: `<div id="node-${i}">Node ${i}</div>`,
      failureSummary: 'Failure'
    }))

    const axeResult = {
      violations: [
        {
          id: 'test-rule',
          impact: 'minor',
          description: 'Test',
          helpUrl: 'http://test.com',
          tags: [],
          nodes
        }
      ],
      passes: [],
      incomplete: [],
      inapplicable: []
    }

    const formatted = formatAxeResults(axeResult)

    assert.ok(formatted.violations[0].nodes.length <= 10)
    assert.strictEqual(formatted.violations[0].nodeCount, 20)
  })

  test('should truncate HTML snippets to 200 chars', async () => {
    const { formatAxeResults } = await import('../../extension/lib/analysis/dom-queries.js')

    const longHtml = '<div class="' + 'x'.repeat(300) + '">content</div>'
    const axeResult = {
      violations: [
        {
          id: 'test-rule',
          impact: 'minor',
          description: 'Test',
          helpUrl: 'http://test.com',
          tags: [],
          nodes: [{ target: ['div'], html: longHtml, failureSummary: 'Failure' }]
        }
      ],
      passes: [],
      incomplete: [],
      inapplicable: []
    }

    const formatted = formatAxeResults(axeResult)

    assert.ok(formatted.violations[0].nodes[0].html.length <= 200)
  })

  test('should include summary counts', async () => {
    const { formatAxeResults } = await import('../../extension/lib/analysis/dom-queries.js')

    const axeResult = {
      violations: [
        { id: 'v1', nodes: [] },
        { id: 'v2', nodes: [] }
      ],
      passes: Array(52).fill({ id: 'p', nodes: [] }),
      incomplete: [{ id: 'i1', nodes: [] }],
      inapplicable: Array(31).fill({ id: 'ia', nodes: [] })
    }

    const formatted = formatAxeResults(axeResult)

    assert.strictEqual(formatted.summary.violations, 2)
    assert.strictEqual(formatted.summary.passes, 52)
    assert.strictEqual(formatted.summary.incomplete, 1)
    assert.strictEqual(formatted.summary.inapplicable, 31)
  })

  test('should timeout after 30 seconds', async () => {
    const { runAxeAuditWithTimeout } = await import('../../extension/lib/analysis/dom-queries.js')

    globalThis.window.axe = {
      run: mock.fn(() => new Promise(() => {})) // Never resolves
    }

    const result = await runAxeAuditWithTimeout({}, 50) // 50ms timeout for testing

    assert.ok(result.error, 'Expected timeout error')
    assert.ok(result.error.includes('timeout'), 'Expected timeout message')
  })

  test('should extract WCAG tags', async () => {
    const { formatAxeResults } = await import('../../extension/lib/analysis/dom-queries.js')

    const axeResult = {
      violations: [
        {
          id: 'color-contrast',
          impact: 'serious',
          description: 'Test',
          helpUrl: 'http://test.com',
          tags: ['wcag2aa', 'cat.color', 'wcag143'],
          nodes: []
        }
      ],
      passes: [],
      incomplete: [],
      inapplicable: []
    }

    const formatted = formatAxeResults(axeResult)

    // Should extract WCAG-specific tags
    assert.ok(formatted.violations[0].wcag, 'Expected wcag field')
    assert.ok(formatted.violations[0].wcag.includes('wcag2aa'))
  })
})

describe('Page Load Deferral', () => {
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

  test('should defer intercepts while page is loading', async () => {
    const { shouldDeferIntercepts } = await import('../../extension/inject/observers.js')

    globalThis.document.readyState = 'loading'
    assert.strictEqual(shouldDeferIntercepts(), true)
  })

  test('should not defer if page already loaded', async () => {
    const { shouldDeferIntercepts } = await import('../../extension/inject/observers.js')

    globalThis.document.readyState = 'complete'
    assert.strictEqual(shouldDeferIntercepts(), false)
  })
})

describe('Memory Pressure Detection', () => {
  beforeEach(() => {
    originalWindow = globalThis.window
    globalThis.window = createMockWindow()
  })

  afterEach(() => {
    globalThis.window = originalWindow
  })

  test('should reduce buffers at soft limit (20MB)', async () => {
    const { checkMemoryPressure } = await import('../../extension/inject/observers.js')

    const state = {
      wsBufferCapacity: 500,
      networkBufferCapacity: 100,
      memoryUsageMB: 25 // Above 20MB soft limit
    }

    const result = checkMemoryPressure(state)

    assert.ok(result.wsBufferCapacity < 500, 'Expected WS buffer reduced')
    assert.ok(result.networkBufferCapacity < 100, 'Expected network buffer reduced')
  })

  test('should disable network bodies at hard limit (50MB)', async () => {
    const { checkMemoryPressure } = await import('../../extension/inject/observers.js')

    const state = {
      wsBufferCapacity: 500,
      networkBufferCapacity: 100,
      networkBodiesEnabled: true,
      memoryUsageMB: 55 // Above 50MB hard limit
    }

    const result = checkMemoryPressure(state)

    assert.strictEqual(result.networkBodiesEnabled, false)
  })

  test('should not modify state when under soft limit', async () => {
    const { checkMemoryPressure } = await import('../../extension/inject/observers.js')

    const state = {
      wsBufferCapacity: 500,
      networkBufferCapacity: 100,
      networkBodiesEnabled: true,
      memoryUsageMB: 15 // Under 20MB soft limit
    }

    const result = checkMemoryPressure(state)

    assert.strictEqual(result.wsBufferCapacity, 500)
    assert.strictEqual(result.networkBufferCapacity, 100)
    assert.strictEqual(result.networkBodiesEnabled, true)
  })
})
