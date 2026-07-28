// @ts-nocheck
/**
 * @fileoverview Canonical DOM, Chrome, storage, fetch, and port fixtures for terminal-panel tests.
 */

import { mock } from 'node:test'
import { StorageKey } from '../../extension/lib/constants.js'

export const sidepanelState = {
  importCounter: 0,
  localStorageData: {},
  sessionStorageData: {},
  fetchHandler: null,
  roots: [],
  windowListeners: {},
  runtimeMessageListeners: [],
  storageChangeListener: null,
  activeTabId: 1,
  connectedPorts: [],
}

/** Stand-in for a chrome.runtime.Port, with the two listener lists exposed. */
function createFakePort(name) {
  const port = {
    name,
    messageListeners: [],
    disconnectListeners: [],
    postMessage: mock.fn(),
    disconnect: mock.fn(),
    onMessage: { addListener: mock.fn((fn) => port.messageListeners.push(fn)) },
    onDisconnect: { addListener: mock.fn((fn) => port.disconnectListeners.push(fn)) }
  }
  return port
}

export function makeResponse(status, body) {
  return {
    ok: status >= 200 && status < 300,
    status,
    json: async () => body
  }
}

export function walkTree(node, visit) {
  for (const child of node.children || []) {
    if (visit(child)) return child
    const found = walkTree(child, visit)
    if (found) return found
  }
  return null
}

function matchSelector(el, selector) {
  if (selector.startsWith('#')) return el.id === selector.slice(1)
  if (selector.startsWith('.')) {
    const cls = selector.slice(1)
    return String(el.className || '')
      .split(/\s+/)
      .filter(Boolean)
      .includes(cls)
  }
  return String(el.tagName || '').toLowerCase() === selector.toLowerCase()
}

function querySelectorWithin(node, selector) {
  return walkTree(node, (child) => matchSelector(child, selector))
}

export function getElementById(id) {
  for (const root of sidepanelState.roots) {
    if (root.id === id) return root
    const found = walkTree(root, (child) => child.id === id)
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
    attributes: {},
    dataset: {},
    offsetWidth: 800,
    offsetHeight: 400,
    appendChild: mock.fn((child) => {
      child.parentElement = el
      el.children.push(child)
      return child
    }),
    replaceChildren: mock.fn((...children) => {
      el.children = []
      for (const child of children) {
        child.parentElement = el
        el.children.push(child)
      }
    }),
    remove: mock.fn(() => {
      if (!el.parentElement) return
      const siblings = el.parentElement.children || []
      const idx = siblings.indexOf(el)
      if (idx >= 0) siblings.splice(idx, 1)
      el.parentElement = null
    }),
    addEventListener: mock.fn((type, handler) => {
      listeners[type] = handler
    }),
    setAttribute: mock.fn((name, value) => {
      el.attributes[name] = value
    }),
    querySelector: mock.fn((selector) => querySelectorWithin(el, selector)),
    dispatch: (type, event = {}) => {
      const handler = listeners[type]
      if (!handler) return
      handler({
        preventDefault() {},
        stopPropagation() {},
        clientX: 0,
        clientY: 0,
        ...event
      })
    }
  }

  if (tag === 'iframe') {
    el.contentWindow = { postMessage: mock.fn() }
  }

  return el
}

export function dispatchWindowEvent(type, event = {}) {
  const handlers = sidepanelState.windowListeners[type] || []
  for (const handler of handlers) handler(event)
}

export function getPostMessagePayloads(iframe, startAt = 0) {
  const calls = iframe?.contentWindow?.postMessage?.mock?.calls || []
  return calls.slice(startAt).map((call) => call.arguments[0])
}

export function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

function emitStorageChange(areaName, key, oldValue, newValue) {
  if (!sidepanelState.storageChangeListener) return
  sidepanelState.storageChangeListener({ [key]: { oldValue, newValue } }, areaName)
}

export function setupEnvironment() {
  sidepanelState.roots = []
  sidepanelState.fetchHandler = null
  sidepanelState.windowListeners = {}
  sidepanelState.runtimeMessageListeners = []
  sidepanelState.storageChangeListener = null
  sidepanelState.activeTabId = 1
  sidepanelState.connectedPorts = []

  const body = createElement('body')
  const head = createElement('head')
  const documentElement = createElement('html')
  sidepanelState.roots.push(body, head, documentElement)

  globalThis.document = {
    body,
    head,
    documentElement,
    readyState: 'complete',
    createElement: mock.fn((tag) => createElement(tag)),
    getElementById: mock.fn((id) => getElementById(id)),
    addEventListener: mock.fn(),
    removeEventListener: mock.fn()
  }

  globalThis.window = {
    addEventListener: mock.fn((type, handler) => {
      if (!sidepanelState.windowListeners[type]) sidepanelState.windowListeners[type] = []
      sidepanelState.windowListeners[type].push(handler)
    }),
    removeEventListener: mock.fn((type, handler) => {
      if (!sidepanelState.windowListeners[type]) return
      sidepanelState.windowListeners[type] = sidepanelState.windowListeners[type].filter((item) => item !== handler)
    }),
    innerWidth: 1600,
    innerHeight: 900
  }

  const clipboard = { writeText: mock.fn(() => Promise.resolve()) }
  if (!globalThis.navigator) {
    Object.defineProperty(globalThis, 'navigator', {
      value: { clipboard },
      configurable: true
    })
  } else {
    Object.defineProperty(globalThis.navigator, 'clipboard', {
      value: clipboard,
      configurable: true
    })
  }

  globalThis.requestAnimationFrame = (cb) => cb()

  globalThis.fetch = mock.fn(async (url, options = {}) => {
    const call = { url: String(url), options }
    if (!sidepanelState.fetchHandler) throw new Error('sidepanelState.fetchHandler is not configured')
    return sidepanelState.fetchHandler(call)
  })

  globalThis.chrome = {
    runtime: {
      id: 'test-extension-id',
      lastError: null,
      sendMessage: mock.fn((message, callback) => {
        if (message?.type === 'terminal_panel_write') {
          callback?.({ success: true })
          return Promise.resolve({ success: true })
        }
        if (message?.type === 'open_terminal_panel') {
          callback?.({ success: true })
          return Promise.resolve({ success: true })
        }
        callback?.({})
        return Promise.resolve({})
      }),
      connect: mock.fn(({ name }) => {
        const port = createFakePort(name)
        sidepanelState.connectedPorts.push(port)
        return port
      }),
      onMessage: {
        addListener: mock.fn((listener) => {
          sidepanelState.runtimeMessageListeners.push(listener)
        }),
        removeListener: mock.fn((listener) => {
          sidepanelState.runtimeMessageListeners = sidepanelState.runtimeMessageListeners.filter((item) => item !== listener)
        })
      }
    },
    sidePanel: {
      close: mock.fn(() => Promise.resolve())
    },
    tabs: {
      query: mock.fn((_queryInfo) => Promise.resolve([{ id: sidepanelState.activeTabId }]))
    },
    storage: {
      local: {
        get: mock.fn((keys, callback) => {
          const keyList = Array.isArray(keys) ? keys : [keys]
          const result = {}
          for (const key of keyList) result[key] = sidepanelState.localStorageData[key]
          callback(result)
        }),
        set: mock.fn((payload, callback) => {
          sidepanelState.localStorageData = { ...sidepanelState.localStorageData, ...(payload || {}) }
          callback?.()
        }),
        remove: mock.fn((keys, callback) => {
          const keyList = Array.isArray(keys) ? keys : [keys]
          for (const key of keyList) delete sidepanelState.localStorageData[key]
          callback?.()
        })
      },
      session: {
        get: mock.fn((keys, callback) => {
          const keyList = Array.isArray(keys) ? keys : [keys]
          const result = {}
          for (const key of keyList) result[key] = sidepanelState.sessionStorageData[key]
          callback(result)
        }),
        set: mock.fn((payload, callback) => {
          const prev = { ...sidepanelState.sessionStorageData }
          sidepanelState.sessionStorageData = { ...sidepanelState.sessionStorageData, ...(payload || {}) }
          for (const [key, value] of Object.entries(payload || {})) {
            emitStorageChange('session', key, prev[key], value)
          }
          callback?.()
        }),
        remove: mock.fn((keys, callback) => {
          const keyList = Array.isArray(keys) ? keys : [keys]
          const prev = { ...sidepanelState.sessionStorageData }
          for (const key of keyList) {
            delete sidepanelState.sessionStorageData[key]
            emitStorageChange('session', key, prev[key], undefined)
          }
          callback?.()
        })
      },
      onChanged: {
        addListener: mock.fn((listener) => {
          sidepanelState.storageChangeListener = listener
        }),
        removeListener: mock.fn((listener) => {
          if (sidepanelState.storageChangeListener === listener) sidepanelState.storageChangeListener = null
        })
      }
    }
  }
}

export function findButton(root, predicate) {
  if (!root) return null
  return walkTree(root, (node) => node.tagName === 'BUTTON' && predicate(node))
}
