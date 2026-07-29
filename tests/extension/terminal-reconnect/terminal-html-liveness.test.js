// @ts-nocheck
/**
 * terminal-html-liveness.test.js — the client must detect a HALF-OPEN socket.
 *
 * Liveness used to be entirely server-driven: the daemon pings every 30s and closes
 * after 60s of silence. But a WebSocket ping is a control frame the browser answers
 * automatically and never surfaces to JS, so page code could not distinguish a
 * live-but-idle terminal from a half-open one. After suspend/resume or a NAT rebind
 * there is no FIN/RST — readyState stays OPEN, onclose never fires, and both writers
 * take their non-queueing branch. Keystrokes and agent writes went into the void
 * behind a green status dot.
 *
 * The server now also emits an observable {"type":"keepalive"} text frame on its ping
 * tick; this asserts the client acts on its absence, and — just as important — that it
 * does NOT sever a healthy idle connection.
 */
import { readFileSync } from 'node:fs'
import vm from 'node:vm'
import { describe, test } from 'node:test'
import assert from 'node:assert'

const html = readFileSync(new URL('../../../cmd/browser-agent/internal/terminal/terminal_assets/terminal.html', import.meta.url), 'utf8')
const iife = [...html.matchAll(/<script>([\s\S]*?)<\/script>/g)].map((m) => m[1]).find((s) => s.includes('term.onData'))
assert.ok(iife && iife.includes('KEEPALIVE_DEAD_MS'), 'could not extract terminal.html IIFE with the liveness watchdog')

function runTerminalPage() {
  const parentMsgs = []
  const timers = []
  let timerSeq = 1
  let nowMs = 1_000_000
  const stateRef = { ws: null, onData: null }
  let closeCalls = 0

  const makeEl = () => ({
    className: '', textContent: '', title: '',
    addEventListener() {}, clientWidth: 0, clientHeight: 0
  })

  const term = {
    _core: { _renderService: { dimensions: { css: { cell: { width: 0, height: 0 } } } } },
    cols: 80, rows: 24,
    textarea: { addEventListener() {} },
    open() {}, reset() {}, focus() {}, refresh() {}, resize() {}, write() {},
    onData(cb) { stateRef.onData = cb }
  }

  class FakeWebSocket {
    constructor(url) {
      this.url = url
      this.binaryType = ''
      this.readyState = 0
      this.onopen = this.onmessage = this.onclose = this.onerror = null
      stateRef.ws = this
    }
    send() {}
    close() {
      closeCalls++
      this.readyState = 3
      if (this.onclose) this.onclose()
    }
  }
  FakeWebSocket.CONNECTING = 0
  FakeWebSocket.OPEN = 1
  FakeWebSocket.CLOSING = 2
  FakeWebSocket.CLOSED = 3

  const win = {
    location: { search: '?token=abc', protocol: 'http:', host: 'localhost:7891' },
    addEventListener() {}, removeEventListener() {}
  }
  win.parent = { postMessage(msg) { parentMsgs.push(msg) } }

  const sandbox = {
    window: win,
    document: { getElementById: () => makeEl(), addEventListener() {}, activeElement: null },
    Terminal: function () { return term },
    WebSocket: FakeWebSocket,
    URLSearchParams, TextEncoder, TextDecoder,
    setTimeout: (fn, ms) => { const id = timerSeq++; timers.push({ id, fn, ms }); return id },
    clearTimeout: (id) => { const i = timers.findIndex((t) => t.id === id); if (i >= 0) timers.splice(i, 1) },
    Date: { now: () => nowMs },
    console: { log() {}, warn() {}, error() {} }
  }
  vm.createContext(sandbox)
  vm.runInContext(iife, sandbox)

  return {
    get ws() { return stateRef.ws },
    closeCalls: () => closeCalls,
    parentEvents: () => parentMsgs.map((m) => m && m.event),
    advance: (ms) => { nowMs += ms },
    // Fire only the watchdog's pending tick (the shortest-delay timer), leaving
    // reconnect timers alone so the two paths stay distinguishable.
    fireWatchdog: () => {
      const idx = timers.reduce((best, t, i) => (best < 0 || t.ms < timers[best].ms ? i : best), -1)
      if (idx < 0) return false
      const [t] = timers.splice(idx, 1)
      t.fn()
      return true
    }
  }
}

function openSocket(h) {
  h.ws.readyState = 1
  h.ws.onopen()
}

describe('terminal.html half-open socket detection', () => {
  test('reports Claude API billing mode to the parent UI', () => {
    assert.match(html, /API Usage Billing/)
    assert.match(html, /notifyParent\('api_billing_detected'/)
  })

  test('reports the shell-classified execution provider to the parent UI', () => {
    assert.match(html, /KABOOM_EXECUTION_PROVIDER=/)
    assert.match(html, /notifyParent\('execution_provider_detected'/)
  })

  test('a socket silent past the threshold is force-closed and recovery starts', () => {
    const h = runTerminalPage()
    openSocket(h)
    assert.strictEqual(h.closeCalls(), 0, 'a freshly opened socket must not be closed')

    // No frames at all — the suspend/resume case. readyState stays OPEN, so nothing
    // in the old code would ever have noticed.
    h.advance(76_000) // > KEEPALIVE_DEAD_MS (30s * 2.5)
    assert.ok(h.fireWatchdog(), 'the watchdog should have a pending tick')

    assert.strictEqual(h.closeCalls(), 1, 'a half-open socket must be force-closed')
    assert.ok(
      h.parentEvents().includes('disconnected'),
      'the parent must learn the terminal is disconnected so the dot stops lying'
    )
  })

  test('a healthy idle connection is NEVER severed', () => {
    const h = runTerminalPage()
    openSocket(h)

    // An idle terminal produces no output for minutes, but the server keepalive
    // keeps arriving. Severing here would be a self-inflicted outage — the whole
    // reason the threshold is a multiple of the server tick.
    for (let i = 0; i < 20; i++) {
      h.advance(29_000)
      h.ws.onmessage({ data: JSON.stringify({ type: 'keepalive' }) })
      h.fireWatchdog()
    }
    assert.strictEqual(h.closeCalls(), 0, 'keepalives must keep a healthy idle socket alive')
  })

  test('ordinary terminal output also counts as liveness', () => {
    const h = runTerminalPage()
    openSocket(h)

    // A busy terminal may never idle long enough for a keepalive; its output frames
    // must refresh liveness too, or a chatty session would be killed mid-stream.
    for (let i = 0; i < 10; i++) {
      h.advance(40_000)
      h.ws.onmessage({ data: new TextEncoder().encode('output').buffer })
      h.fireWatchdog()
    }
    assert.strictEqual(h.closeCalls(), 0, 'binary output frames must refresh liveness')
  })

  test('the watchdog stops once the socket is closed (no reconnect storm)', () => {
    const h = runTerminalPage()
    openSocket(h)
    h.advance(76_000)
    h.fireWatchdog()
    const afterFirst = h.closeCalls()

    // Fire every remaining timer: the watchdog must not still be running and
    // re-closing a socket that onclose already handed to the reconnect path.
    for (let i = 0; i < 5; i++) h.fireWatchdog()
    assert.strictEqual(h.closeCalls(), afterFirst, 'the watchdog must not keep closing after it fired')
  })
})
