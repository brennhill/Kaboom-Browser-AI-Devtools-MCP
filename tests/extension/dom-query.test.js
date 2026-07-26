// @ts-nocheck
import { test, describe, mock, beforeEach, afterEach } from 'node:test'
import assert from 'node:assert'
import { createMockChrome } from './helpers.js'

import { handleDomQuery } from '../../extension/content/message-handlers.js'
import { executeDOMQuery } from '../../extension/lib/analysis/dom-queries.js'

describe('DOM Query Production Paths', () => {
  let originalSetTimeout

  beforeEach(() => {
    globalThis.chrome = createMockChrome()

    globalThis.window = {
      location: { origin: 'http://localhost:3000', href: 'http://localhost:3000/dashboard' },
      addEventListener: mock.fn(),
      removeEventListener: mock.fn(),
      postMessage: mock.fn(),
      getComputedStyle: mock.fn(() => ({
        display: 'block',
        color: 'rgb(0, 0, 0)',
        position: 'static',
        getPropertyValue: (_name) => 'mock-value'
      }))
    }

    globalThis.document = {
      title: 'Dashboard',
      querySelector: mock.fn(() => null),
      querySelectorAll: mock.fn(() => []),
      addEventListener: mock.fn(),
      removeEventListener: mock.fn(),
      readyState: 'complete',
      head: { appendChild: mock.fn() },
      documentElement: { appendChild: mock.fn() },
      createElement: mock.fn(() => ({ remove: mock.fn() }))
    }

    originalSetTimeout = globalThis.setTimeout
    globalThis.setTimeout = mock.fn(() => 0)
  })

  afterEach(() => {
    globalThis.setTimeout = originalSetTimeout
  })

  test('handleDomQuery forwards parsed params via kaboom_dom_query', () => {
    const sendResponse = mock.fn()

    const keepOpen = handleDomQuery('{"selector":"#submit"}', sendResponse)

    assert.strictEqual(keepOpen, true)
    assert.strictEqual(globalThis.window.postMessage.mock.calls.length, 1)

    const [posted, origin] = globalThis.window.postMessage.mock.calls[0].arguments
    assert.strictEqual(posted.type, 'kaboom_dom_query')
    assert.strictEqual(posted.params.selector, '#submit')
    assert.ok(typeof posted.requestId === 'number')
    assert.strictEqual(origin, 'http://localhost:3000')
  })

  test('handleDomQuery gracefully handles malformed JSON params', () => {
    const sendResponse = mock.fn()

    handleDomQuery('{invalid-json', sendResponse)

    const [posted] = globalThis.window.postMessage.mock.calls[0].arguments
    assert.deepStrictEqual(posted.params, {})
  })

  test('executeDOMQuery uses real selector lookup and serializes DOM matches', async () => {
    const childEl = {
      tagName: 'SPAN',
      textContent: 'child',
      attributes: [],
      children: [],
      offsetParent: {},
      getBoundingClientRect: () => ({ x: 15, y: 30, width: 20, height: 10 })
    }

    const el = {
      tagName: 'BUTTON',
      textContent: 'Submit',
      attributes: [
        { name: 'id', value: 'submit-btn' },
        { name: 'data-testid', value: 'submit' }
      ],
      children: [childEl],
      offsetParent: {},
      getBoundingClientRect: () => ({ x: 10, y: 20, width: 120, height: 40 })
    }

    globalThis.document.querySelectorAll = mock.fn((selector) => {
      if (selector === '#submit') return [el]
      return []
    })

    const result = await executeDOMQuery({
      selector: '#submit',
      include_styles: true,
      properties: ['display'],
      include_children: true,
      max_depth: 2
    })

    assert.strictEqual(globalThis.document.querySelectorAll.mock.calls[0].arguments[0], '#submit')
    assert.strictEqual(result.url, 'http://localhost:3000/dashboard')
    assert.strictEqual(result.title, 'Dashboard')
    assert.strictEqual(result.matchCount, 1)
    assert.strictEqual(result.returnedCount, 1)
    assert.strictEqual(result.matches[0].tag, 'button')
    assert.strictEqual(result.matches[0].attributes.id, 'submit-btn')
    assert.strictEqual(result.matches[0].styles.display, 'mock-value')
    assert.strictEqual(result.matches[0].children.length, 1)
    assert.strictEqual(result.matches[0].children[0].tag, 'span')
  })
})
