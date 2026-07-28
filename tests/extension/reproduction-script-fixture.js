// @ts-nocheck
/**
 * @fileoverview Canonical DOM-element fixture for reproduction-script tests.
 */

import { mock } from 'node:test'

let originalWindow

// Mock DOM elements
export function createElement(tag, attrs = {}, opts = {}) {
  const el = {
    tagName: tag.toUpperCase(),
    id: attrs.id || '',
    className: attrs.class || '',
    classList: { from: [] },
    textContent: opts.textContent || '',
    innerText: opts.innerText || opts.textContent || '',
    parentElement: opts.parent || null,
    children: opts.children || [],
    childNodes: opts.children || [],
    getAttribute: (name) => {
      if (name === 'data-testid') return attrs['data-testid'] || null
      if (name === 'data-test-id') return attrs['data-test-id'] || null
      if (name === 'data-cy') return attrs['data-cy'] || null
      if (name === 'aria-label') return attrs['aria-label'] || null
      if (name === 'role') return attrs.role || null
      if (name === 'type') return attrs.type || null
      if (name === 'href') return attrs.href || null
      return attrs[name] || null
    },
    hasAttribute: (name) => name in attrs,
    querySelectorAll: mock.fn(() => [])
  }
  el.classList = Array.from(new Set((attrs.class || '').split(' ').filter(Boolean)))
  el.classList.from = el.classList
  return el
}
