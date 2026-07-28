// @ts-nocheck
/**
 * @fileoverview Canonical Chrome, DOM, window, and timer fixtures for on-demand tests.
 */

import { mock, after } from 'node:test'
import { MANIFEST_VERSION } from './helpers.js'

// Mock Chrome APIs
export const createMockChrome = () => ({
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

export const createMockDocument = () => ({
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

export const createMockWindow = () => ({
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
