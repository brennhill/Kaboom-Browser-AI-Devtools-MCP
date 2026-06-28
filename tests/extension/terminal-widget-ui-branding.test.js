// @ts-nocheck
/**
 * @fileoverview terminal-widget-ui-branding.test.js — Verifies Kaboom title copy in the legacy terminal widget shell.
 */

import { beforeEach, describe, mock, test } from 'node:test'
import assert from 'node:assert'

function walkTree(node, visit) {
  for (const child of node.children || []) {
    if (visit(child)) return child
    const found = walkTree(child, visit)
    if (found) return found
  }
  return null
}

function createElement(tag) {
  const listeners = {}
  const el = {
    tagName: String(tag).toUpperCase(),
    id: '',
    className: '',
    textContent: '',
    title: '',
    type: '',
    src: '',
    style: {},
    children: [],
    parentElement: null,
    offsetWidth: 800,
    offsetHeight: 400,
    appendChild: mock.fn((child) => {
      child.parentElement = el
      el.children.push(child)
      return child
    }),
    setAttribute: mock.fn(),
    addEventListener: mock.fn((type, handler) => {
      listeners[type] = handler
    }),
    querySelector: mock.fn((selector) => walkTree(el, (child) => {
      if (!selector.startsWith('.')) return false
      return String(child.className || '')
        .split(/\s+/)
        .filter(Boolean)
        .includes(selector.slice(1))
    }))
  }

  if (tag === 'iframe') {
    el.contentWindow = { postMessage: mock.fn() }
  }

  return el
}

describe('terminal widget ui branding', () => {
  beforeEach(() => {
    mock.reset()
    const body = createElement('body')
    const documentElement = createElement('html')

    globalThis.document = {
      body,
      documentElement,
      createElement: mock.fn((tag) => createElement(tag)),
      addEventListener: mock.fn(),
      removeEventListener: mock.fn()
    }

    globalThis.window = {
      innerWidth: 1600,
      innerHeight: 900,
      addEventListener: mock.fn(),
      removeEventListener: mock.fn()
    }

    globalThis.requestAnimationFrame = (cb) => cb()
    Object.defineProperty(globalThis, 'navigator', {
      value: {
        clipboard: {
          writeText: mock.fn(async () => {})
        }
      },
      configurable: true
    })
  })

  test('createWidget labels the legacy shell as Kaboom Terminal', async () => {
    const { resetAllState } = await import('../../extension/content/ui/terminal-widget-types.js')
    const { createWidget } = await import('../../extension/content/ui/terminal-widget-ui.js')

    resetAllState()
    const widget = createWidget('token-123')

    const titleNode = walkTree(widget, (child) => child.textContent === 'KaBOOM! Terminal')
    assert.ok(titleNode, 'expected KaBOOM! Terminal title in widget header')
  })
})
