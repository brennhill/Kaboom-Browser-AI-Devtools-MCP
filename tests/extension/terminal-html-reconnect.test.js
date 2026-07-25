// @ts-nocheck
/**
 * terminal-html-reconnect.test.js — behavioral coverage for terminal.html's
 * reconnect input ordering. terminal.html is hand-authored plain JS (not compiled),
 * so this harness extracts its IIFE and runs it under vm with stubbed globals,
 * driving the real onopen → onData → replay_end sequence.
 *
 * Finding D: while the server is still replaying scrollback, the socket is already
 * OPEN, so a keystroke typed during replay used to be ws.send()'d immediately and
 * JUMP AHEAD of the earlier bytes buffered during the reconnect gap. Live input
 * typed during replay must instead be buffered (FIFO) behind the gap input and
 * flushed together at replay_end.
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
  const state = { ws: null, onData: null }

  const makeEl = () => ({
    className: '',
    textContent: '',
    title: '',
    addEventListener() {},
    // xterm renderer internals: cell size 0 makes fitTerminal() bail early.
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
    onData(cb) { state.onData = cb }
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
      state.ws = this
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
  win.parent = { postMessage() {} } // distinct from win, so notifyParent posts

  const sandbox = {
    window: win,
    document: { getElementById: () => makeEl(), addEventListener() {}, activeElement: null },
    Terminal: function () { return term },
    WebSocket: FakeWebSocket,
    URLSearchParams,
    TextEncoder,
    TextDecoder,
    setTimeout,
    clearTimeout,
    Date,
    console
  }
  vm.createContext(sandbox)
  vm.runInContext(iife, sandbox)

  const decode = (d) => (typeof d === 'string' ? d : new TextDecoder().decode(d))
  return {
    ws: state.ws,
    typeKey: (s) => state.onData(s),
    // Only the binary keystroke frames (control messages like resize are strings).
    binarySends: () => sends.filter((d) => typeof d !== 'string').map(decode)
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
