// @ts-nocheck
/**
 * @fileoverview script-injection-ready.test.js — readiness queue + bridge probe behavior.
 */

import { beforeEach, describe, test } from 'node:test'
import assert from 'node:assert'

let appendedScripts = []
let attrStore = {}
let readyState = 'complete'
let domContentLoadedHandler = null
let windowMessageHandlers = []
let autoBridgePong = false
let pingPostCount = 0

function makeScriptElement() {
  return {
    id: '',
    src: '',
    type: '',
    dataset: {},
    onload: null,
    onerror: null,
    remove() {}
  }
}

function appendAndAutoLoad(node) {
  appendedScripts.push(node)
  setTimeout(() => {
    if (typeof node.onload === 'function') node.onload()
  }, 0)
  return node
}

function installWindowMocks() {
  globalThis.window = {
    location: { origin: 'http://localhost:3000' },
    addEventListener(event, handler) {
      if (event === 'message') windowMessageHandlers.push(handler)
    },
    removeEventListener(event, handler) {
      if (event !== 'message') return
      windowMessageHandlers = windowMessageHandlers.filter((h) => h !== handler)
    },
    postMessage(payload) {
      if (payload?.type === 'kaboom_inject_bridge_ping') {
        pingPostCount += 1
        if (!autoBridgePong) return
        const requestId = payload.requestId
        setTimeout(() => {
          for (const handler of windowMessageHandlers.slice()) {
            handler({
              source: globalThis.window,
              origin: globalThis.window.location.origin,
              data: {
                type: 'kaboom_inject_bridge_pong',
                requestId,
                _nonce: payload._nonce
              }
            })
          }
        }, 0)
      }
    }
  }
}

function installDomMocks() {
  globalThis.document = {
    get readyState() {
      return readyState
    },
    set readyState(v) {
      readyState = v
    },
    head: {
      appendChild: appendAndAutoLoad
    },
    documentElement: {
      appendChild: appendAndAutoLoad,
      setAttribute(name, value) {
        attrStore[name] = value
      }
    },
    createElement(tag) {
      if (tag !== 'script') {
        throw new Error(`unexpected tag: ${tag}`)
      }
      return makeScriptElement()
    },
    getElementById(id) {
      return appendedScripts.find((s) => s.id === id) || null
    },
    querySelectorAll() {
      return []
    },
    addEventListener(event, handler) {
      if (event === 'DOMContentLoaded') domContentLoadedHandler = handler
    }
  }
}

globalThis.chrome = {
  runtime: {
    getURL: (path) => `chrome-extension://gasoline/${path}`
  },
  storage: {
    local: {
      get: (_keys, callback) => callback({})
    }
  }
}

describe('script injection readiness', () => {
  async function importFreshModule() {
    return import(`../../../extension/content/script-injection.js?test=${Date.now()}_${Math.random()}`)
  }

  beforeEach(() => {
    appendedScripts = []
    attrStore = {}
    readyState = 'complete'
    domContentLoadedHandler = null
    windowMessageHandlers = []
    autoBridgePong = false
    pingPostCount = 0
    installDomMocks()
    installWindowMocks()
  })

  test('ensureInjectScriptReady queues concurrent callers and injects once', async () => {
    const mod = await importFreshModule()
    const p1 = mod.ensureInjectScriptReady(2000)
    const p2 = mod.ensureInjectScriptReady(2000)
    const [r1, r2] = await Promise.all([p1, p2])

    assert.strictEqual(r1, true)
    assert.strictEqual(r2, true)

    const injectLoads = appendedScripts.filter((s) => String(s.src).includes('inject.bundled.js'))
    assert.strictEqual(injectLoads.length, 1, 'inject script should be appended exactly once')
    assert.ok(attrStore['data-kaboom-nonce'], 'nonce should be set on documentElement for inject bridge')
  })

  test('ensureInjectScriptReady can force reinjection when requested', async () => {
    const mod = await importFreshModule()
    await mod.ensureInjectScriptReady(2000)
    await mod.ensureInjectScriptReady(2000, true)

    const injectLoads = appendedScripts.filter((s) => String(s.src).includes('inject.bundled.js'))
    assert.strictEqual(injectLoads.length, 2, 'force=true should trigger a fresh inject attempt')
  })

  test('initScriptInjection waits for DOMContentLoaded when document is loading', async () => {
    readyState = 'loading'
    const mod = await importFreshModule()
    mod.initScriptInjection()

    assert.strictEqual(typeof domContentLoadedHandler, 'function')
    assert.strictEqual(appendedScripts.length, 0, 'should not inject until DOMContentLoaded')

    domContentLoadedHandler()
    await new Promise((resolve) => setTimeout(resolve, 5))

    const injectLoads = appendedScripts.filter((s) => String(s.src).includes('inject.bundled.js'))
    assert.strictEqual(injectLoads.length, 1)
  })

  test('ensureInjectBridgeReady resolves true when inject responds to ping', async () => {
    const mod = await importFreshModule()
    await mod.ensureInjectScriptReady(2000)
    autoBridgePong = true

    const ready = await mod.ensureInjectBridgeReady(250)
    assert.strictEqual(ready, true)
    assert.strictEqual(pingPostCount, 1)
  })

  test('ensureInjectBridgeReady deduplicates concurrent pings', async () => {
    const mod = await importFreshModule()
    await mod.ensureInjectScriptReady(2000)
    autoBridgePong = true

    const [a, b] = await Promise.all([mod.ensureInjectBridgeReady(250), mod.ensureInjectBridgeReady(250)])
    assert.strictEqual(a, true)
    assert.strictEqual(b, true)
    assert.strictEqual(pingPostCount, 1, 'concurrent calls should share one bridge probe')
  })

  test('ensureInjectBridgeReady returns false on timeout when bridge is silent', async () => {
    const mod = await importFreshModule()
    await mod.ensureInjectScriptReady(2000)
    autoBridgePong = false

    const ready = await mod.ensureInjectBridgeReady(5)
    assert.strictEqual(ready, false)
    assert.strictEqual(pingPostCount, 1)
  })
})
