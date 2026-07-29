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

const html = readFileSync(new URL('../../../cmd/browser-agent/internal/terminal/terminal_assets/terminal.html', import.meta.url), 'utf8')
// The only inline <script> (the xterm one has a src attribute) is the IIFE.
const iife = [...html.matchAll(/<script>([\s\S]*?)<\/script>/g)].map((m) => m[1]).find((s) => s.includes('term.onData'))
assert.ok(iife && iife.includes('replay_end'), 'could not extract terminal.html reconnect IIFE')

/** Build a stubbed environment, run the IIFE, and expose the captured seams. */
function runTerminalPage(random = Math.random) {
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
    setTimeout: (fn, delay) => { const id = timerSeq++; timers.push({ id, fn, delay }); return id },
    clearTimeout: (id) => { const i = timers.findIndex((t) => t.id === id); if (i >= 0) timers.splice(i, 1) },
    Date: { now: () => nowMs },
    // Own Math so the reconnect jitter is deterministic per page instance.
    Math: Object.assign(Object.create(Math), { random }),
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
    pendingDelays: () => timers.map((t) => t.delay),
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

describe('terminal.html reconnect jitter (finding S12)', () => {
  test('the reconnect delay is jittered, so simultaneous panels do not stampede', () => {
    // A daemon restart drops every open panel at once. With a fixed schedule they
    // all reconnect at the same instants and hit the 32-subscriber fanout cap
    // together; jitter spreads them out.
    const lowJitter = runTerminalPage(() => 0)
    const highJitter = runTerminalPage(() => 0.999)

    lowJitter.ws.onclose()
    highJitter.ws.onclose()

    const [low] = lowJitter.pendingDelays()
    const [high] = highJitter.pendingDelays()

    assert.ok(typeof low === 'number' && typeof high === 'number', 'a reconnect must be scheduled with a delay')
    assert.ok(
      high > low,
      `the reconnect delay must vary with Math.random (got ${low} and ${high}) — a fixed delay reconnects every panel in lockstep`
    )
  })

  test('jitter only ever delays, and never past the declared ratio', () => {
    const base = Number(iife.match(/var\s+reconnectDelay\s*=\s*(\d+)/)[1])
    const ratio = Number(iife.match(/RECONNECT_JITTER_RATIO\s*=\s*([\d.]+)/)[1])

    for (const r of [0, 0.25, 0.5, 0.75, 0.999]) {
      const h = runTerminalPage(() => r)
      h.ws.onclose()
      const [delay] = h.pendingDelays()
      assert.ok(
        delay >= base,
        `jitter must never fire EARLIER than the backoff (r=${r}: ${delay} < ${base}) — that would shorten the budget the write-guard derives from it`
      )
      assert.ok(
        delay <= Math.round(base * (1 + ratio)),
        `jitter must stay within the declared ratio (r=${r}: ${delay} > ${Math.round(base * (1 + ratio))})`
      )
    }
  })
})

describe('terminal.html reconnect-gap input buffer (finding S13)', () => {
  const MAX_PENDING_INPUT = Number(iife.match(/MAX_PENDING_INPUT\s*=\s*(\d+)/)[1])

  test('a single paste larger than the cap is evicted, not kept forever', () => {
    const h = runTerminalPage()
    const paste = 'x'.repeat(MAX_PENDING_INPUT * 2)

    h.typeKey(paste) // socket is still CONNECTING -> buffered

    h.ws.readyState = 1
    h.ws.onopen()
    h.ws.onmessage({ data: JSON.stringify({ type: 'replay_end' }) })

    assert.deepStrictEqual(
      h.binarySends(),
      [],
      'an oversized single chunk must be evicted like any other — the `length > 1` guard made it un-evictable, so it sat in the buffer forever and was replayed whole'
    )
  })

  test('the cap counts bytes actually sent, not UTF-16 code units', () => {
    const h = runTerminalPage()
    // '€' is ONE UTF-16 unit but THREE UTF-8 bytes: 4000 of them are 4000 by
    // .length (under the cap, so nothing was evicted) and 12000 on the wire.
    const chunks = 4000
    for (let i = 0; i < chunks; i++) h.typeKey('€')

    h.ws.readyState = 1
    h.ws.onopen()
    h.ws.onmessage({ data: JSON.stringify({ type: 'replay_end' }) })

    const sent = h.binarySends()
    assert.strictEqual(sent.length, 1, 'the buffered input should flush as one send')
    const bytes = new TextEncoder().encode(sent[0]).length
    assert.ok(
      bytes <= MAX_PENDING_INPUT,
      `the reconnect-gap buffer must be bounded by SENT bytes: flushed ${bytes} bytes with a ${MAX_PENDING_INPUT}-byte cap`
    )
    assert.ok(bytes > 0, 'eviction must not empty a buffer that still fits under the cap')
  })

  test('input that fits under the cap is preserved in full', () => {
    const h = runTerminalPage()
    h.typeKey('ls')
    h.typeKey(' -la')

    h.ws.readyState = 1
    h.ws.onopen()
    h.ws.onmessage({ data: JSON.stringify({ type: 'replay_end' }) })

    assert.deepStrictEqual(h.binarySends(), ['ls -la'], 'small gap input must flush intact, in order')
  })
})
