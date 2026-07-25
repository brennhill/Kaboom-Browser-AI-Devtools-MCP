// @ts-nocheck
/**
 * @fileoverview sidepanel-terminal.test.js — Regression coverage for the terminal side panel host.
 */

import { beforeEach, describe, mock, test } from 'node:test'
import assert from 'node:assert'
import { StorageKey } from '../../extension/lib/constants.js'

let importCounter = 0
let localStorageData = {}
let sessionStorageData = {}
let fetchHandler = null
let roots = []
let windowListeners = {}
let runtimeMessageListeners = []
let storageChangeListener = null
let activeTabId = 1
let connectedPorts = []

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

function makeResponse(status, body) {
  return {
    ok: status >= 200 && status < 300,
    status,
    json: async () => body
  }
}

function walkTree(node, visit) {
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

function getElementById(id) {
  for (const root of roots) {
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

function dispatchWindowEvent(type, event = {}) {
  const handlers = windowListeners[type] || []
  for (const handler of handlers) handler(event)
}

function getPostMessagePayloads(iframe, startAt = 0) {
  const calls = iframe?.contentWindow?.postMessage?.mock?.calls || []
  return calls.slice(startAt).map((call) => call.arguments[0])
}

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

function emitStorageChange(areaName, key, oldValue, newValue) {
  if (!storageChangeListener) return
  storageChangeListener({ [key]: { oldValue, newValue } }, areaName)
}

function setupEnvironment() {
  roots = []
  fetchHandler = null
  windowListeners = {}
  runtimeMessageListeners = []
  storageChangeListener = null
  activeTabId = 1
  connectedPorts = []

  const body = createElement('body')
  const head = createElement('head')
  const documentElement = createElement('html')
  roots.push(body, head, documentElement)

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
      if (!windowListeners[type]) windowListeners[type] = []
      windowListeners[type].push(handler)
    }),
    removeEventListener: mock.fn((type, handler) => {
      if (!windowListeners[type]) return
      windowListeners[type] = windowListeners[type].filter((item) => item !== handler)
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
    if (!fetchHandler) throw new Error('fetchHandler is not configured')
    return fetchHandler(call)
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
        connectedPorts.push(port)
        return port
      }),
      onMessage: {
        addListener: mock.fn((listener) => {
          runtimeMessageListeners.push(listener)
        }),
        removeListener: mock.fn((listener) => {
          runtimeMessageListeners = runtimeMessageListeners.filter((item) => item !== listener)
        })
      }
    },
    sidePanel: {
      close: mock.fn(() => Promise.resolve())
    },
    tabs: {
      query: mock.fn((_queryInfo) => Promise.resolve([{ id: activeTabId }]))
    },
    storage: {
      local: {
        get: mock.fn((keys, callback) => {
          const keyList = Array.isArray(keys) ? keys : [keys]
          const result = {}
          for (const key of keyList) result[key] = localStorageData[key]
          callback(result)
        }),
        set: mock.fn((payload, callback) => {
          localStorageData = { ...localStorageData, ...(payload || {}) }
          callback?.()
        }),
        remove: mock.fn((keys, callback) => {
          const keyList = Array.isArray(keys) ? keys : [keys]
          for (const key of keyList) delete localStorageData[key]
          callback?.()
        })
      },
      session: {
        get: mock.fn((keys, callback) => {
          const keyList = Array.isArray(keys) ? keys : [keys]
          const result = {}
          for (const key of keyList) result[key] = sessionStorageData[key]
          callback(result)
        }),
        set: mock.fn((payload, callback) => {
          const prev = { ...sessionStorageData }
          sessionStorageData = { ...sessionStorageData, ...(payload || {}) }
          for (const [key, value] of Object.entries(payload || {})) {
            emitStorageChange('session', key, prev[key], value)
          }
          callback?.()
        }),
        remove: mock.fn((keys, callback) => {
          const keyList = Array.isArray(keys) ? keys : [keys]
          const prev = { ...sessionStorageData }
          for (const key of keyList) {
            delete sessionStorageData[key]
            emitStorageChange('session', key, prev[key], undefined)
          }
          callback?.()
        })
      },
      onChanged: {
        addListener: mock.fn((listener) => {
          storageChangeListener = listener
        }),
        removeListener: mock.fn((listener) => {
          if (storageChangeListener === listener) storageChangeListener = null
        })
      }
    }
  }
}

function findButton(root, predicate) {
  if (!root) return null
  return walkTree(root, (node) => node.tagName === 'BUTTON' && predicate(node))
}

describe('terminal side panel host', () => {
  beforeEach(() => {
    mock.reset()
    localStorageData = { [StorageKey.SERVER_URL]: 'http://localhost:7890' }
    sessionStorageData = {}
    setupEnvironment()
  })

  test('boots a panel with terminal iframe and persists open state', async () => {
    fetchHandler = ({ url }) => {
      if (url.endsWith('/terminal/start')) {
        return Promise.resolve(makeResponse(200, {
          session_id: 'session-1',
          token: 'token-1',
          pid: 999
        }))
      }
      throw new Error(`Unexpected fetch call: ${url}`)
    }

    const module = await import(`../../extension/sidepanel.js?v=${++importCounter}`)
    await module._terminalPanelForTests.bootTerminalPanel(true)

    const header = getElementById('kaboom-terminal-header')
    const iframe = getElementById('kaboom-terminal-iframe')
    assert.ok(header, 'terminal header should be mounted')
    assert.ok(iframe, 'terminal iframe should be mounted')
    assert.strictEqual(sessionStorageData[StorageKey.TERMINAL_UI_STATE], 'open')

    const minimizeButton = findButton(header, (node) => node.title === 'Minimize terminal')
    assert.ok(minimizeButton, 'minimize button should exist')
    assert.strictEqual(minimizeButton.textContent, '\u2581')
  })

  test('re-booting with forceFresh unmounts the old panel and attaches the fresh shell (folder-reload fix)', async () => {
    let startCount = 0
    fetchHandler = ({ url }) => {
      if (url.endsWith('/terminal/start')) {
        startCount += 1
        return Promise.resolve(makeResponse(200, {
          session_id: `session-${startCount}`,
          token: `token-${startCount}`,
          pid: 999
        }))
      }
      throw new Error(`Unexpected fetch call: ${url}`)
    }

    const module = await import(`../../extension/sidepanel.js?v=${++importCounter}`)
    await module._terminalPanelForTests.bootTerminalPanel(true)
    assert.ok(getElementById('kaboom-terminal-iframe').src.includes('token-1'), 'first boot mounts the token-1 shell')

    // A second forceFresh boot models applyRootFolder()/the retry button rebuilding
    // the panel for a NEW session. Without unmounting first, mountPanel() early-
    // returns while rootEl is set, so the fresh token-2 shell is never attached and
    // the user is left on the old, just-stopped session \u2014 the "terminal won't start
    // after picking a folder" failure.
    await module._terminalPanelForTests.bootTerminalPanel(true)
    const iframe = getElementById('kaboom-terminal-iframe')
    assert.ok(iframe, 'a terminal iframe is mounted after the fresh re-boot')
    assert.ok(iframe.src.includes('token-2'), 'forceFresh re-boot attaches the fresh token-2 shell, not the orphaned old one')
  })

  test('applyRootFolder persists the root, stops the old session, and mounts a fresh shell (reported bug, end-to-end)', async () => {
    let startCount = 0
    let stoppedId = null
    fetchHandler = ({ url, options }) => {
      if (url.endsWith('/terminal/start')) {
        startCount += 1
        return Promise.resolve(makeResponse(200, {
          session_id: `session-${startCount}`,
          token: `token-${startCount}`,
          pid: 999
        }))
      }
      if (url.endsWith('/terminal/stop')) {
        try { stoppedId = JSON.parse(options.body || '{}').id } catch { stoppedId = 'parse-error' }
        return Promise.resolve(makeResponse(200, { ok: true }))
      }
      throw new Error(`Unexpected fetch call: ${url}`)
    }

    const module = await import(`../../extension/sidepanel.js?v=${++importCounter}`)
    await module._terminalPanelForTests.bootTerminalPanel(true)
    assert.ok(getElementById('kaboom-terminal-iframe').src.includes('token-1'), 'first boot mounts token-1')

    // The exact path that failed for the user: pick a folder and reload. A running
    // PTY can't change cwd, so the old session is stopped and a fresh one booted.
    await module._terminalPanelForTests.applyRootFolder('/Users/x/project')

    assert.strictEqual(stoppedId, 'session-1', 'the old session is stopped before rebuilding')
    assert.strictEqual(localStorageData[StorageKey.TERMINAL_DEV_ROOT], '/Users/x/project', 'the chosen root is persisted')
    const iframe = getElementById('kaboom-terminal-iframe')
    assert.ok(iframe && iframe.src.includes('token-2'), 'the fresh shell for the new folder is attached — not the orphaned, just-stopped session')
  })

  test('disconnect button ends the current session and closes the side panel', async () => {
    let startCount = 0
    const stopBodies = []

    fetchHandler = ({ url, options }) => {
      if (url.endsWith('/terminal/start')) {
        startCount += 1
        return Promise.resolve(makeResponse(200, {
          session_id: `session-${startCount}`,
          token: `token-${startCount}`,
          pid: 999
        }))
      }
      if (url.endsWith('/terminal/stop')) {
        stopBodies.push(JSON.parse(String(options.body || '{}')))
        return Promise.resolve(makeResponse(200, { ok: true }))
      }
      if (url.includes('/terminal/validate?token=')) {
        return Promise.resolve(makeResponse(200, { valid: false }))
      }
      throw new Error(`Unexpected fetch call: ${url}`)
    }

    const module = await import(`../../extension/sidepanel.js?v=${++importCounter}`)
    await module._terminalPanelForTests.bootTerminalPanel(true)

    const header = getElementById('kaboom-terminal-header')
    // Matched by id, not title: the title is user-facing copy and should be free
    // to change without breaking the behavioural contract below.
    const powerButton = findButton(header, (node) => node.id === 'kaboom-terminal-disconnect-button')
    assert.ok(powerButton, 'power button should be present')
    assert.match(powerButton.title, /end session/i, 'the power control must read as ending the session')
    assert.strictEqual(startCount, 1)

    powerButton.dispatch('click')
    await sleep(0)

    assert.strictEqual(stopBodies.length, 1, 'disconnect should stop the current session')
    assert.deepStrictEqual(stopBodies[0], { id: 'session-1' })
    assert.strictEqual(startCount, 1, 'disconnect should not boot a fresh session')
    assert.strictEqual(chrome.sidePanel.close.mock.calls.length, 1, 'disconnect should close the side panel')
    assert.strictEqual(chrome.sidePanel.close.mock.calls[0].arguments[0].tabId, 1)
    assert.strictEqual(sessionStorageData[StorageKey.TERMINAL_SESSION], undefined, 'disconnect should clear persisted session')
    assert.strictEqual(sessionStorageData[StorageKey.TERMINAL_UI_STATE], undefined, 'disconnect should clear persisted UI state')
    assert.strictEqual(getElementById('kaboom-terminal-widget'), null, 'disconnect should unmount the side panel shell')
  })

  test('minimize button hides the side panel and keeps the current session alive', async () => {
    let startCount = 0
    const stopBodies = []

    fetchHandler = ({ url, options }) => {
      if (url.endsWith('/terminal/start')) {
        startCount += 1
        return Promise.resolve(makeResponse(200, {
          session_id: `session-${startCount}`,
          token: `token-${startCount}`,
          pid: 999
        }))
      }
      if (url.endsWith('/terminal/stop')) {
        stopBodies.push(JSON.parse(String(options.body || '{}')))
        return Promise.resolve(makeResponse(200, { ok: true }))
      }
      throw new Error(`Unexpected fetch call: ${url}`)
    }

    const module = await import(`../../extension/sidepanel.js?v=${++importCounter}`)
    await module._terminalPanelForTests.bootTerminalPanel(true)

    const header = getElementById('kaboom-terminal-header')
    const minimizeButton = findButton(header, (node) => node.title === 'Minimize terminal')
    assert.ok(minimizeButton, 'minimize button should be present')
    assert.strictEqual(startCount, 1)

    minimizeButton.dispatch('click')
    await sleep(0)

    assert.strictEqual(stopBodies.length, 0, 'minimize should not stop the current session')
    assert.strictEqual(startCount, 1, 'minimize should not boot a fresh session')
    assert.strictEqual(chrome.sidePanel.close.mock.calls.length, 1, 'minimize should close the side panel')
    assert.strictEqual(chrome.sidePanel.close.mock.calls[0].arguments[0].tabId, 1)
    assert.deepStrictEqual(
      sessionStorageData[StorageKey.TERMINAL_SESSION],
      { sessionId: 'session-1', token: 'token-1' },
      'minimize should keep the persisted session'
    )
    assert.strictEqual(sessionStorageData[StorageKey.TERMINAL_UI_STATE], 'minimized', 'minimize should persist hidden-session state')
    assert.strictEqual(getElementById('kaboom-terminal-widget'), null, 'minimize should unmount the side panel shell')
  })

  test('redraw button reloads iframe without starting a new session', async () => {
    let startCount = 0

    fetchHandler = ({ url }) => {
      if (url.endsWith('/terminal/start')) {
        startCount += 1
        return Promise.resolve(makeResponse(200, {
          session_id: `session-${startCount}`,
          token: `token-${startCount}`,
          pid: 999
        }))
      }
      if (url.includes('/terminal/validate?token=')) {
        return Promise.resolve(makeResponse(200, { valid: true }))
      }
      throw new Error(`Unexpected fetch call: ${url}`)
    }

    const module = await import(`../../extension/sidepanel.js?v=${++importCounter}`)
    await module._terminalPanelForTests.bootTerminalPanel(true)

    const header = getElementById('kaboom-terminal-header')
    const iframe = getElementById('kaboom-terminal-iframe')
    const redrawButton = findButton(header, (node) => node.title === 'Redraw terminal graphics')
    assert.ok(iframe, 'terminal iframe should exist')
    assert.ok(redrawButton, 'redraw button should exist')

    const priorSrc = iframe.src
    redrawButton.dispatch('click')

    assert.strictEqual(iframe.src, priorSrc, 'redraw should keep the same token URL')
    assert.strictEqual(startCount, 1, 'redraw should not start a new session')
  })

  test('redraw revalidates the token and rebuilds when the daemon restarted (dead token, L1)', async () => {
    let startCount = 0
    let tokenAlive = true
    fetchHandler = ({ url }) => {
      if (url.endsWith('/terminal/start')) {
        startCount += 1
        return Promise.resolve(makeResponse(200, {
          session_id: `session-${startCount}`,
          token: `tok-${startCount}`,
          pid: 999
        }))
      }
      if (url.includes('/terminal/validate?token=')) {
        return Promise.resolve(makeResponse(200, { valid: tokenAlive }))
      }
      throw new Error(`Unexpected fetch call: ${url}`)
    }

    const module = await import(`../../extension/sidepanel.js?v=${++importCounter}`)
    await module._terminalPanelForTests.bootTerminalPanel(true)
    assert.ok(getElementById('kaboom-terminal-iframe').src.includes('tok-1'), 'first boot mounts tok-1')
    assert.strictEqual(startCount, 1)

    // Daemon restarted: the persisted token is now dead. A plain reload would
    // reconnect forever; redraw must revalidate and rebuild into a fresh session.
    tokenAlive = false
    await module._terminalPanelForTests.redrawTerminal()

    assert.strictEqual(startCount, 2, 'redraw on a dead token must boot a fresh session, not reload the dead one')
    assert.ok(getElementById('kaboom-terminal-iframe').src.includes('tok-2'), 'the rebuilt panel uses the fresh token')
  })

  test('write guard waits while user is typing and flushes after blur', async () => {
    fetchHandler = ({ url }) => {
      if (url.endsWith('/terminal/start')) {
        return Promise.resolve(makeResponse(200, {
          session_id: 'session-typing-guard',
          token: 'token-typing-guard',
          pid: 999
        }))
      }
      throw new Error(`Unexpected fetch call: ${url}`)
    }

    const module = await import(`../../extension/sidepanel.js?v=${++importCounter}`)
    await module._terminalPanelForTests.bootTerminalPanel(true)

    const iframe = getElementById('kaboom-terminal-iframe')
    assert.ok(iframe, 'terminal iframe should exist')

    dispatchWindowEvent('message', {
      origin: 'http://localhost:7891',
      data: { source: 'kaboom-terminal', event: 'connected' }
    })
    dispatchWindowEvent('message', {
      origin: 'http://localhost:7891',
      data: { source: 'kaboom-terminal', event: 'focus', data: { focused: true } }
    })
    dispatchWindowEvent('message', {
      origin: 'http://localhost:7891',
      data: { source: 'kaboom-terminal', event: 'typing', data: { at: Date.now() } }
    })

    const callStart = iframe.contentWindow.postMessage.mock.calls.length
    module._terminalPanelForTests.writeToTerminal('queued command')

    await sleep(80)
    const whileTypingPayloads = getPostMessagePayloads(iframe, callStart)
    assert.strictEqual(whileTypingPayloads.filter((payload) => payload?.command === 'write').length, 0)

    dispatchWindowEvent('message', {
      origin: 'http://localhost:7891',
      data: { source: 'kaboom-terminal', event: 'focus', data: { focused: false } }
    })

    await sleep(800)
    const flushedPayloads = getPostMessagePayloads(iframe, callStart)
    const flushedWrites = flushedPayloads
      .filter((payload) => payload?.command === 'write')
      .map((payload) => payload.text)

    assert.deepStrictEqual(flushedWrites, ['queued command', '\r'])
  })

  test('a write that arrives before the socket is connected is queued and replayed, not dropped', async () => {
    fetchHandler = ({ url }) => {
      if (url.endsWith('/terminal/start')) {
        return Promise.resolve(makeResponse(200, {
          session_id: 'session-preconnect',
          token: 'token-preconnect',
          pid: 999
        }))
      }
      throw new Error(`Unexpected fetch call: ${url}`)
    }

    const module = await import(`../../extension/sidepanel.js?v=${++importCounter}`)
    await module._terminalPanelForTests.bootTerminalPanel(true)

    const iframe = getElementById('kaboom-terminal-iframe')
    assert.ok(iframe, 'terminal iframe should exist')

    // No 'connected' event yet -> the WebSocket is not OPEN. A write here used to
    // be sent straight to the iframe (which drops it) while the submit fired a
    // bare Enter later -> the AI's text vanished. It must be QUEUED instead.
    const callStart = iframe.contentWindow.postMessage.mock.calls.length
    module._terminalPanelForTests.writeToTerminal('deploy the service')

    await sleep(20)
    const preConnectWrites = getPostMessagePayloads(iframe, callStart)
      .filter((p) => p?.command === 'write')
      .map((p) => p.text)
    assert.deepStrictEqual(preConnectWrites, [], 'nothing may be written to a not-yet-connected socket')

    // Socket comes up -> the queued write must be replayed (text first, then Enter).
    dispatchWindowEvent('message', {
      origin: 'http://localhost:7891',
      data: { source: 'kaboom-terminal', event: 'connected' }
    })

    await sleep(800)
    const afterConnectWrites = getPostMessagePayloads(iframe, callStart)
      .filter((p) => p?.command === 'write')
      .map((p) => p.text)
    assert.deepStrictEqual(afterConnectWrites, ['deploy the service', '\r'],
      'the queued write must be replayed in full once connected, not lost with a bare Enter')
  })

  test('two concurrent forceFresh boots keep writes on the on-screen iframe (boot race)', async () => {
    // Double-clicked "Start", or a rapid folder re-pick, fires bootTerminalPanel(true)
    // twice while the first is still awaiting the network. Without serialization,
    // createPanelShell() from the 2nd boot rebinds state.iframeEl to a fresh iframe
    // while mountPanel() early-returns (rootEl already set) — so the VISIBLE terminal
    // is the 1st iframe but state.iframeEl points at the detached 2nd, and every write
    // vanishes into the off-screen frame with no error. The write below must reach the
    // iframe that is actually mounted.
    let starts = 0
    fetchHandler = ({ url }) => {
      if (url.endsWith('/terminal/start')) {
        starts += 1
        return Promise.resolve(makeResponse(200, {
          session_id: `session-race-${starts}`,
          token: `token-race-${starts}`,
          pid: 900 + starts
        }))
      }
      if (url.endsWith('/terminal/validate')) {
        return Promise.resolve(makeResponse(200, { valid: true }))
      }
      throw new Error(`Unexpected fetch call: ${url}`)
    }

    const module = await import(`../../extension/sidepanel.js?v=${++importCounter}`)

    // Fire both boots without awaiting the first — this is the race.
    const boot1 = module._terminalPanelForTests.bootTerminalPanel(true)
    const boot2 = module._terminalPanelForTests.bootTerminalPanel(true)
    await Promise.all([boot1, boot2])

    const iframe = getElementById('kaboom-terminal-iframe')
    assert.ok(iframe, 'exactly one terminal iframe is mounted after the raced boots')

    dispatchWindowEvent('message', {
      origin: 'http://localhost:7891',
      data: { source: 'kaboom-terminal', event: 'connected' }
    })

    const callStart = iframe.contentWindow.postMessage.mock.calls.length
    module._terminalPanelForTests.writeToTerminal('reach the visible frame')

    const writes = getPostMessagePayloads(iframe, callStart)
      .filter((p) => p?.command === 'write')
      .map((p) => p.text)
    assert.ok(
      writes.includes('reach the visible frame'),
      'the write must land on the mounted iframe (state.iframeEl), not a detached one from a raced boot'
    )
  })

  test('terminal submit re-guards if focus returns before auto-enter', async () => {
    fetchHandler = ({ url }) => {
      if (url.endsWith('/terminal/start')) {
        return Promise.resolve(makeResponse(200, {
          session_id: 'session-re-guard',
          token: 'token-re-guard',
          pid: 999
        }))
      }
      throw new Error(`Unexpected fetch call: ${url}`)
    }

    const module = await import(`../../extension/sidepanel.js?v=${++importCounter}`)
    await module._terminalPanelForTests.bootTerminalPanel(true)

    const iframe = getElementById('kaboom-terminal-iframe')
    assert.ok(iframe, 'terminal iframe should exist')

    dispatchWindowEvent('message', {
      origin: 'http://localhost:7891',
      data: { source: 'kaboom-terminal', event: 'connected' }
    })
    dispatchWindowEvent('message', {
      origin: 'http://localhost:7891',
      data: { source: 'kaboom-terminal', event: 'focus', data: { focused: false } }
    })

    const callStart = iframe.contentWindow.postMessage.mock.calls.length
    module._terminalPanelForTests.writeToTerminal('submit guard command')

    await sleep(80)
    dispatchWindowEvent('message', {
      origin: 'http://localhost:7891',
      data: { source: 'kaboom-terminal', event: 'focus', data: { focused: true } }
    })
    dispatchWindowEvent('message', {
      origin: 'http://localhost:7891',
      data: { source: 'kaboom-terminal', event: 'typing', data: { at: Date.now() } }
    })

    await sleep(680)
    const blockedPayloads = getPostMessagePayloads(iframe, callStart)
    const blockedWrites = blockedPayloads
      .filter((payload) => payload?.command === 'write')
      .map((payload) => payload.text)
    assert.deepStrictEqual(blockedWrites, ['submit guard command'])

    dispatchWindowEvent('message', {
      origin: 'http://localhost:7891',
      data: { source: 'kaboom-terminal', event: 'focus', data: { focused: false } }
    })

    await sleep(320)
    const releasedPayloads = getPostMessagePayloads(iframe, callStart)
    const releasedWrites = releasedPayloads
      .filter((payload) => payload?.command === 'write')
      .map((payload) => payload.text)
    assert.deepStrictEqual(releasedWrites, ['submit guard command', '\r'])
  })

  test('reopening a minimized session restores the full panel without starting a new session', async () => {
    sessionStorageData[StorageKey.TERMINAL_SESSION] = { sessionId: 'session-min', token: 'token-min' }
    sessionStorageData[StorageKey.TERMINAL_UI_STATE] = 'minimized'

    fetchHandler = ({ url }) => {
      if (url.includes('/terminal/validate?token=')) {
        return Promise.resolve(makeResponse(200, { valid: true }))
      }
      throw new Error(`Unexpected fetch call: ${url}`)
    }

    const module = await import(`../../extension/sidepanel.js?v=${++importCounter}`)
    await module._terminalPanelForTests.bootTerminalPanel(true)

    const header = getElementById('kaboom-terminal-header')
    const minimizeButton = findButton(header, (node) => node.title === 'Minimize terminal')
    const terminalBody = getElementById('kaboom-terminal-body')

    assert.ok(minimizeButton, 'minimize button should be present after restore')
    assert.ok(terminalBody, 'terminal body should exist after restore')
    assert.strictEqual(terminalBody.style.display, 'block', 'reopened minimized session should restore the full panel')
    assert.strictEqual(sessionStorageData[StorageKey.TERMINAL_UI_STATE], 'open', 'reopen should promote minimized session back to open')
  })

  test('panel mounts only the terminal shell so xterm can use the full panel height', async () => {
    fetchHandler = ({ url }) => {
      if (url.endsWith('/terminal/start')) {
        return Promise.resolve(makeResponse(200, {
          session_id: 'session-full-height',
          token: 'token-full-height',
          pid: 999
        }))
      }
      throw new Error(`Unexpected fetch call: ${url}`)
    }

    const module = await import(`../../extension/sidepanel.js?v=${++importCounter}`)
    await module._terminalPanelForTests.bootTerminalPanel(true)

    const root = getElementById('kaboom-terminal-widget')
    const header = getElementById('kaboom-terminal-header')
    const iframe = getElementById('kaboom-terminal-iframe')
    const terminalShell = header?.parentElement || null
    const newProjectButton = findButton(root, (node) => node.textContent === 'New Project')
    const titleNode = walkTree(header, (child) => child.textContent === 'KaBOOM! Terminal')

    assert.ok(root, 'panel root should exist')
    assert.ok(header, 'terminal header should exist')
    assert.ok(iframe, 'terminal iframe should exist')
    assert.ok(terminalShell, 'terminal shell should wrap the header and iframe')
    assert.ok(titleNode, 'terminal header should show Kaboom Terminal')
    assert.strictEqual(newProjectButton, null, 'placeholder palette action should not be rendered')
    assert.strictEqual(root.children.length, 1, 'terminal shell should be the only top-level panel child')
  })

  test('daemon-unavailable fallback uses Kaboom copy', async () => {
    fetchHandler = ({ url }) => {
      if (url.endsWith('/terminal/start')) {
        return Promise.resolve(makeResponse(500, { error: 'daemon_unavailable' }))
      }
      throw new Error(`Unexpected fetch call: ${url}`)
    }

    const module = await import(`../../extension/sidepanel.js?v=${++importCounter}`)
    await module._terminalPanelForTests.bootTerminalPanel(true)

    const header = getElementById('kaboom-terminal-header')
    const terminalBody = getElementById('kaboom-terminal-body')
    const titleNode = walkTree(header, (child) => child.textContent === 'KaBOOM! Terminal')
    const fallbackNode = walkTree(terminalBody, (child) =>
      typeof child.textContent === 'string' && child.textContent.includes('KaBOOM! daemon')
    )
    // The dead-end sentence is replaced by a recoverable state: a session can be
    // started and the root folder changed without leaving the panel. Previously
    // an ended or failed session left nothing to click. The root folder moved
    // out of this state and into a bar that is always visible above the terminal.
    const startButton = walkTree(terminalBody, (child) => child.id === 'kaboom-terminal-start-button')
    const rootInput = getElementById('kaboom-terminal-root-folder-input')

    assert.ok(header, 'terminal header should exist')
    assert.ok(titleNode, 'terminal header should show Kaboom Terminal')
    assert.ok(terminalBody, 'terminal body should exist')
    assert.ok(fallbackNode, 'fallback should mention the Kaboom daemon')
    assert.ok(startButton, 'a session-less panel must offer a way to start one')
    assert.ok(rootInput, 'a session-less panel must let the user set the root folder')
    const rootBar = getElementById('kaboom-terminal-root-folder-bar')
    assert.ok(rootBar, 'the root folder bar must exist whether or not a session started')
  })

  test('close button closes the drawer and leaves the shell running', async () => {
    // Closing a drawer must not destroy the shell inside it. The old close
    // called exitTerminalSession(), so a user tidying the UI lost their session.
    const stopBodies = []
    globalThis.fetch = (url, init) => {
      if (url.includes('/terminal/start')) {
        return Promise.resolve(makeResponse(200, { session_id: 'session-1', token: 'tok-1', pid: 1 }))
      }
      if (url.includes('/terminal/stop')) {
        stopBodies.push(JSON.parse(init.body))
        return Promise.resolve(makeResponse(200, { status: 'stopped' }))
      }
      if (url.includes('/terminal/validate?token=')) {
        return Promise.resolve(makeResponse(200, { valid: false }))
      }
      throw new Error(`Unexpected fetch call: ${url}`)
    }

    const module = await import(`../../extension/sidepanel.js?v=${++importCounter}`)
    await module._terminalPanelForTests.bootTerminalPanel(true)

    const header = getElementById('kaboom-terminal-header')
    const closeButton = findButton(header, (node) => node.id === 'kaboom-terminal-close-button')
    assert.ok(closeButton, 'the panel needs an obvious close control')

    closeButton.dispatch('click')
    await sleep(0)

    assert.deepStrictEqual(stopBodies, [], 'closing the drawer must not stop the PTY')
    assert.notStrictEqual(
      sessionStorageData[StorageKey.TERMINAL_SESSION], undefined,
      'the session must survive so reopening reconnects to it'
    )
    assert.strictEqual(chrome.sidePanel.close.mock.calls.length, 1, 'close should close the side panel')
  })

  // The background decides open-vs-close from whether a panel document exists.
  // It learns that from this port — a stored flag cannot see Chrome's own X
  // dismiss the panel, and a stale "open" left the user unable to reopen at all.
  describe('presence port', () => {
    function bootWithSession() {
      globalThis.fetch = (url) => {
        if (url.includes('/terminal/start')) {
          return Promise.resolve(makeResponse(200, { session_id: 'session-1', token: 'tok-1', pid: 1 }))
        }
        if (url.includes('/terminal/validate?token=')) {
          return Promise.resolve(makeResponse(200, { valid: false }))
        }
        throw new Error(`Unexpected fetch call: ${url}`)
      }
      return import(`../../extension/sidepanel.js?v=${++importCounter}`)
    }

    test('the panel announces itself on the terminal panel port while it is alive', async () => {
      const module = await bootWithSession()
      await module._terminalPanelForTests.bootTerminalPanel(true)

      const port = connectedPorts.find((p) => p.name === 'kaboom_terminal_panel')
      assert.ok(port, 'the background has no other way to know a panel exists')
    })

    test('a close over the port closes the drawer without stopping the shell', async () => {
      const module = await bootWithSession()
      await module._terminalPanelForTests.bootTerminalPanel(true)
      const port = connectedPorts.find((p) => p.name === 'kaboom_terminal_panel')

      for (const listener of port.messageListeners) listener({ type: 'close_terminal_panel' })
      await sleep(0)

      assert.strictEqual(getElementById('kaboom-terminal-widget'), null, 'the drawer should be gone')
      assert.notStrictEqual(
        sessionStorageData[StorageKey.TERMINAL_SESSION], undefined,
        'closing the drawer must leave the shell running'
      )
    })

    test('a restore over the port rebuilds a panel that had been closed', async () => {
      // sidePanel.open() on an existing panel only focuses it — no code runs in
      // this document — so without this an already-open-but-blank panel stayed
      // blank and "Open Kaboom Terminal" looked broken.
      const module = await bootWithSession()
      await module._terminalPanelForTests.bootTerminalPanel(true)
      const port = connectedPorts.find((p) => p.name === 'kaboom_terminal_panel')

      for (const listener of port.messageListeners) listener({ type: 'close_terminal_panel' })
      await sleep(0)
      assert.strictEqual(getElementById('kaboom-terminal-widget'), null)

      for (const listener of port.messageListeners) listener({ type: 'restore_terminal_panel' })
      await sleep(0)

      assert.ok(getElementById('kaboom-terminal-widget'), 'restore must put the terminal back')
      assert.strictEqual(sessionStorageData[StorageKey.TERMINAL_UI_STATE], 'open')
    })

    test('a restore on a panel that has no terminal starts one', async () => {
      // Booting while the daemon was down leaves a mounted panel with no iframe.
      // Reopening has to try again, not just re-show the failure.
      let startCalls = 0
      globalThis.fetch = (url) => {
        if (url.includes('/terminal/start')) {
          startCalls += 1
          return startCalls === 1
            ? Promise.resolve(makeResponse(500, { error: 'daemon_unavailable' }))
            : Promise.resolve(makeResponse(200, { session_id: 's2', token: 'tok-2', pid: 2 }))
        }
        if (url.includes('/terminal/validate?token=')) {
          return Promise.resolve(makeResponse(200, { valid: false }))
        }
        throw new Error(`Unexpected fetch call: ${url}`)
      }
      const module = await import(`../../extension/sidepanel.js?v=${++importCounter}`)
      await module._terminalPanelForTests.bootTerminalPanel(true)
      const port = connectedPorts.find((p) => p.name === 'kaboom_terminal_panel')
      assert.strictEqual(getElementById('kaboom-terminal-iframe'), null, 'no session means no iframe')

      for (const listener of port.messageListeners) listener({ type: 'restore_terminal_panel' })
      await sleep(0)

      assert.strictEqual(startCalls, 2, 'restore must retry the session')
      assert.ok(getElementById('kaboom-terminal-iframe'), 'the retry should mount a terminal')
    })
  })
})
