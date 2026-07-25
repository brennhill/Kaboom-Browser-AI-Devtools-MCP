// @ts-nocheck
/**
 * terminal-html-reconnect.test.js — behavioral coverage for terminal.html's
 * reconnect state machine. terminal.html is hand-authored plain JS (not compiled),
 * so this harness extracts its IIFE and runs it under vm with stubbed globals
 * (including a manual clock + manual timers), driving the real event flow.
 *
 * Finding D: input typed during scrollback replay must be buffered FIFO, not sent
 * ahead of gap-buffered input.
 * Finding E-ii: onopen resets the CONSECUTIVE reconnect counter, so an
 * open→immediately-drop flap would never hit MAX_RECONNECT_ATTEMPTS and never
 * exhaust. A total-within-window cap must make a persistent flap eventually give up.
 */
import { readFileSync } from 'node:fs'
import vm from 'node:vm'
import { describe, test } from 'node:test'
import assert from 'node:assert'

const html = readFileSync(new URL('../../cmd/browser-agent/internal/terminal/terminal_assets/terminal.html', import.meta.url), 'utf8')
// The only inline <script> (the xterm one has a src attribute) is the IIFE.
const iife = [...html.matchAll(/<script>([\s\S]*?)<\/script>/g)].map((m) => m[1]).find((s) => s.includes('term.onData'))
assert.ok(iife && iife.includes('replay_end'), 'could not extract terminal.html reconnect IIFE')

/** Build a stubbed environment, run the IIFE, and expose the captured seams. */
function runTerminalPage() {
  const sends = [] // every ws.send arg, in order
  const parentMsgs = [] // every notifyParent postMessage payload
  const timers = [] // pending manual timers {id, fn}
  let timerSeq = 1
  let nowMs = 1_000_000
  const stateRef = { ws: null, onData: null }

  const makeEl = () => ({
    className: '',
    textContent: '',
    title: '',
    addEventListener() {},
    clientWidth: 0,
    clientHeight: 0
  })

  const term = {
    _core: { _renderService: { dimensions: { css: { cell: { width: 0, height: 0 } } } } },
    cols: 80,
    rows: 24,
    textarea: { addEventListener() {} },
    open() {},
    reset() {},
    focus() {},
    refresh() {},
    resize() {},
    write() {},
    onData(cb) { stateRef.onData = cb }
  }

  class FakeWebSocket {
    constructor(url) {
      this.url = url
      this.binaryType = ''
      this.readyState = 0 // CONNECTING
      this.onopen = null
      this.onmessage = null
      this.onclose = null
      this.onerror = null
      stateRef.ws = this
    }
    send(data) { sends.push(data) }
    close() {}
  }
  FakeWebSocket.CONNECTING = 0
  FakeWebSocket.OPEN = 1
  FakeWebSocket.CLOSING = 2
  FakeWebSocket.CLOSED = 3

  const win = {
    location: { search: '?token=abc', protocol: 'http:', host: 'localhost:7891' },
    addEventListener() {},
    removeEventListener() {}
  }
  win.parent = { postMessage(msg) { parentMsgs.push(msg) } } // distinct from win

  const sandbox = {
    window: win,
    document: { getElementById: () => makeEl(), addEventListener() {}, activeElement: null },
    Terminal: function () { return term },
    WebSocket: FakeWebSocket,
    URLSearchParams,
    TextEncoder,
    TextDecoder,
    setTimeout: (fn) => { const id = timerSeq++; timers.push({ id, fn }); return id },
    clearTimeout: (id) => { const i = timers.findIndex((t) => t.id === id); if (i >= 0) timers.splice(i, 1) },
    Date: { now: () => nowMs },
    console
  }
  vm.createContext(sandbox)
  vm.runInContext(iife, sandbox)

  const decode = (d) => (typeof d === 'string' ? d : new TextDecoder().decode(d))
  return {
    get ws() { return stateRef.ws },
    typeKey: (s) => stateRef.onData(s),
    binarySends: () => sends.filter((d) => typeof d !== 'string').map(decode),
    parentEvents: () => parentMsgs.map((m) => m && m.event),
    clock: { advance: (ms) => { nowMs += ms } },
    fireTimers: () => { const pending = timers.splice(0); for (const t of pending) t.fn() }
  }
}

describe('terminal.html reconnect input ordering (finding D)', () => {
  test('input typed during scrollback replay is buffered FIFO, not sent ahead', () => {
    const h = runTerminalPage()
    assert.ok(h.ws, 'connect() should have opened a WebSocket')

    // Reconnect gap: a keystroke before the socket is OPEN is buffered.
    h.typeKey('G')
    assert.deepStrictEqual(h.binarySends(), [], 'gap keystroke must be buffered, not sent')

    // Socket opens but the server is still replaying scrollback (no replay_end yet).
    h.ws.readyState = 1 // OPEN
    h.ws.onopen()

    // A keystroke typed DURING replay must NOT be sent immediately — it would jump
    // ahead of the earlier gap byte. On the buggy code onData sees readyState OPEN
    // and ws.send()s it right away.
    h.typeKey('L')
    assert.deepStrictEqual(
      h.binarySends(),
      [],
      'a keystroke typed during replay must be buffered behind the gap input, not sent ahead'
    )

    // replay_end: now the buffered input flushes in FIFO order (gap, then live).
    h.ws.onmessage({ data: JSON.stringify({ type: 'replay_end' }) })
    assert.deepStrictEqual(
      h.binarySends(),
      ['GL'],
      'at replay_end the buffered input flushes once, in order: gap ("G") then live ("L")'
    )

    // After replay_end, live input flows straight through again.
    h.typeKey('Z')
    assert.deepStrictEqual(h.binarySends(), ['GL', 'Z'], 'post-replay input is sent live again')
  })
})

describe('terminal.html reconnect exhaustion (finding E-ii)', () => {
  test('a persistent open→drop flap eventually exhausts despite onopen resetting the consecutive counter', () => {
    const h = runTerminalPage()

    // One flap: open (resets the consecutive counter), immediately drop, then run
    // the scheduled reconnect timer (which opens the next socket).
    const flap = () => {
      h.clock.advance(200) // stay well within the reconnect window
      h.ws.readyState = 1
      h.ws.onopen()
      h.ws.onclose()
      h.fireTimers()
    }

    flap()
    assert.ok(
      !h.parentEvents().includes('reconnect_exhausted'),
      'a single flap must not exhaust — exhaustion is about a persistent flap, not one drop'
    )

    let exhausted = false
    for (let i = 0; i < 30 && !exhausted; i++) {
      flap()
      exhausted = h.parentEvents().includes('reconnect_exhausted')
    }
    assert.ok(
      exhausted,
      'a persistent open→drop flap must eventually signal reconnect_exhausted (total-within-window cap), not loop forever'
    )
  })
})
