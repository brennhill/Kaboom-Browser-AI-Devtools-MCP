// @ts-nocheck
/**
 * @fileoverview helpers.js — Shared test infrastructure for all extension tests.
 * Provides mock factories for Chrome APIs (runtime, storage, tabs), browser globals
 * (window, document, crypto, navigator), fetch, WebSocket, and PerformanceObserver.
 * Design: Each mock factory returns a fresh instance to prevent cross-test leakage.
 *
 * Usage: Import only what each test file needs.
 *   import { createMockWindow, createMockCrypto } from '../shared/helpers.js'
 */

import { mock } from 'node:test'
import { readFileSync } from 'node:fs'

const manifest = JSON.parse(readFileSync(new URL('../../../extension/manifest.json', import.meta.url), 'utf8'))
export const MANIFEST_VERSION = manifest.version

/**
 * Configurable mock window factory.
 * Options allow each test file to get exactly the shape it needs.
 *
 * @param {Object} options
 * @param {string} [options.pathname] - window.location.pathname (default: '/')
 * @param {string} [options.hostname] - window.location.hostname (default: 'localhost')
 * @param {string} [options.href] - window.location.href (default: 'http://localhost/')
 * @param {boolean} [options.withWebSocket] - Include MockWebSocket class on the window
 * @param {boolean} [options.withFetch] - Include mock fetch function on the window
 * @param {boolean} [options.withOnerror] - Include onerror/onunhandledrejection fields
 * @param {Object} [options.overrides] - Additional properties to spread onto the window
 * @returns {Object} Mock window object
 */
export function createMockWindow(options = {}) {
  const href = options.href || 'http://localhost/'
  const url = new URL(href)
  const base = {
    postMessage: mock.fn(),
    addEventListener: mock.fn(),
    removeEventListener: mock.fn(),
    location: {
      pathname: options.pathname || '/',
      hostname: options.hostname || 'localhost',
      href,
      origin: url.origin
    }
  }

  if (options.withOnerror) {
    base.onerror = null
    base.onunhandledrejection = null
  }

  if (options.withWebSocket) {
    class MockWebSocket {
      constructor(url, protocols) {
        this.url = url
        this.protocols = protocols
        this.readyState = 0
        this._listeners = {}
      }
      addEventListener(event, handler) {
        if (!this._listeners[event]) this._listeners[event] = []
        this._listeners[event].push(handler)
      }
      send(_data) {}
      close(_code, _reason) {}
      // Helper for tests to simulate events
      _emit(event, data) {
        if (this._listeners[event]) {
          this._listeners[event].forEach((h) => h(data))
        }
      }
    }
    MockWebSocket.CONNECTING = 0
    MockWebSocket.OPEN = 1
    MockWebSocket.CLOSING = 2
    MockWebSocket.CLOSED = 3

    base.WebSocket = MockWebSocket
  }

  if (options.withFetch) {
    base.fetch = mock.fn()
  }

  return { ...base, ...options.overrides }
}

/**
 * Mock crypto for UUID generation.
 * Returns a consistent mock that generates predictable-format UUIDs for testing.
 *
 * @returns {Object} Mock crypto object with randomUUID
 */
export function createMockCrypto() {
  return {
    randomUUID: mock.fn(() => 'test-uuid-' + Math.random().toString(36).slice(2))
  }
}

/**
 * Mock console factory with all standard methods as mock functions.
 *
 * @returns {Object} Mock console object
 */
export function createMockConsole() {
  return {
    log: mock.fn(),
    warn: mock.fn(),
    error: mock.fn(),
    info: mock.fn(),
    debug: mock.fn()
  }
}

/**
 * Mock Chrome API factory for content script tests.
 *
 * @param {Object} [overrides] - Override specific Chrome API methods
 * @returns {Object} Mock chrome object
 */
export function createMockChrome(overrides = {}) {
  return {
    runtime: {
      getURL: mock.fn((path) => `chrome-extension://abc123/${path}`),
      sendMessage: mock.fn(() => Promise.resolve()),
      onMessage: {
        addListener: mock.fn()
      },
      onInstalled: {
        addListener: mock.fn()
      },
      getManifest: () => ({ version: MANIFEST_VERSION }),
      ...overrides.runtime
    },
    action: {
      setBadgeText: mock.fn(),
      setBadgeBackgroundColor: mock.fn()
    },
    storage: {
      local: {
        get: mock.fn((keys, callback) => {
          if (typeof callback === 'function') callback({})
          else return Promise.resolve({})
        }),
        set: mock.fn((data, callback) => {
          if (typeof callback === 'function') callback()
          else return Promise.resolve()
        }),
        remove: mock.fn((keys, callback) => {
          if (typeof callback === 'function') callback()
          else return Promise.resolve()
        })
      },
      sync: {
        get: mock.fn((keys, callback) => callback && callback({})),
        set: mock.fn((data, callback) => callback && callback())
      },
      session: {
        get: mock.fn((keys, callback) => callback && callback({})),
        set: mock.fn((data, callback) => callback && callback())
      },
      onChanged: {
        addListener: mock.fn()
      }
    },
    tabs: {
      get: mock.fn((tabId) => Promise.resolve({ id: tabId, windowId: 1, url: 'http://localhost:3000' })),
      query: mock.fn(() => Promise.resolve([{ id: 1, windowId: 1 }])),
      onRemoved: { addListener: mock.fn() },
      onUpdated: { addListener: mock.fn() }
    },
    ...overrides
  }
}

// Auto-set up chrome global for all tests
if (!globalThis.chrome) {
  globalThis.chrome = createMockChrome()
}

/**
 * Mock document factory for tests that need DOM interactions.
 *
 * @param {Object} [overrides] - Additional properties to merge
 * @returns {Object} Mock document object
 */
export function createMockDocument(overrides = {}) {
  return {
    addEventListener: mock.fn(),
    removeEventListener: mock.fn(),
    querySelector: mock.fn(() => null),
    querySelectorAll: mock.fn(() => []),
    ...overrides
  }
}

function readMockLocal(keys) {
  return new Promise((resolve) => {
    chrome.storage.local.get(keys, resolve)
  })
}

/**
 * Exercise the canonical feature-toggle apply function with values supplied by
 * a test's chrome.storage mock.
 */
export async function applyFeatureTogglesFromMockStorage() {
  const { FEATURE_TOGGLES, applyFeatureToggles } = await import('../../../extension/popup.js')
  const keys = FEATURE_TOGGLES.map((toggle) => toggle.storageKey)
  applyFeatureToggles(await readMockLocal(keys))
}

/**
 * Exercise the canonical WebSocket-mode apply function with the value supplied
 * by a test's chrome.storage mock.
 */
export async function applyWebSocketModeFromMockStorage() {
  const { applyWebSocketMode } = await import('../../../extension/popup.js')
  const values = await readMockLocal(['webSocketCaptureMode'])
  applyWebSocketMode(values.webSocketCaptureMode)
}
