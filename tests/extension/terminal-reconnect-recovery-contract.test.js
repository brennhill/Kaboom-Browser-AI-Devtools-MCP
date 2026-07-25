/**
 * Contract for daemon-restart recovery in the terminal iframe host
 * (cmd/browser-agent/internal/terminal/terminal_assets/terminal.html) and its
 * parent (src/sidepanel.ts).
 *
 * Why this exists: a full daemon restart drops every session/token, so the
 * iframe's captured token 401s on reconnect and onopen never fires. Without a cap
 * the iframe reconnects forever (status stuck) with no recovery, and keystrokes
 * typed in the gap were dropped. terminal.html's JS is not executed by the DOM
 * tests, so these static assertions pin the invariants so a refactor can't quietly
 * remove them. The behavioral parent-side handling is covered in
 * sidepanel-terminal.test.js ('reconnect_exhausted ... revalidates and rebuilds').
 */
import { test, describe } from 'node:test'
import assert from 'node:assert'
import { readFileSync } from 'node:fs'

const TERMINAL_HTML = 'cmd/browser-agent/internal/terminal/terminal_assets/terminal.html'
const SIDEPANEL = 'src/sidepanel.ts'

describe('terminal reconnect-recovery contract', () => {
  const html = readFileSync(TERMINAL_HTML, 'utf8')
  const sidepanel = readFileSync(SIDEPANEL, 'utf8')

  test('iframe caps reconnect attempts and signals reconnect_exhausted', () => {
    assert.match(html, /MAX_RECONNECT_ATTEMPTS/, 'terminal.html must bound reconnect attempts')
    assert.match(
      html,
      /reconnectAttempts\s*>\s*MAX_RECONNECT_ATTEMPTS/,
      'terminal.html must stop reconnecting once the cap is exceeded'
    )
    assert.match(
      html,
      /notifyParent\(\s*['"]reconnect_exhausted['"]/,
      'terminal.html must signal reconnect_exhausted to the parent instead of looping forever'
    )
  })

  test('parent handles reconnect_exhausted by revalidating and rebuilding', () => {
    // The parent must have a case for the exact event the iframe emits, and it must
    // route to redrawTerminal (which validates the token and rebuilds if dead).
    assert.match(
      sidepanel,
      /case\s*['"]reconnect_exhausted['"]\s*:/,
      'sidepanel.ts must handle the reconnect_exhausted event'
    )
    const idx = sidepanel.indexOf("case 'reconnect_exhausted'")
    // Slice the whole case body — from this case label to the next `case '` — instead
    // of a fixed-width window. The recovery path legitimately grows (e.g. the bounded
    // exhaustion-recovery ceiling guard for a flapping daemon sits BEFORE the
    // redrawTerminal() call), and a brittle fixed slice silently breaks when it does.
    const nextCase = sidepanel.indexOf("case '", idx + 'case '.length)
    const caseBody = sidepanel.slice(idx, nextCase === -1 ? idx + 2000 : nextCase)
    assert.match(
      caseBody,
      /redrawTerminal\(\)/,
      'reconnect_exhausted must trigger redrawTerminal (validate-then-rebuild recovery)'
    )
  })

  test('iframe buffers keystrokes during a reconnect gap and flushes on replay_end', () => {
    assert.match(html, /function bufferInput\(/, 'terminal.html must buffer input during a reconnect gap')
    assert.match(html, /function flushPendingInput\(/, 'terminal.html must flush buffered input on reconnect')
    // onData must route to the buffer when the socket is not OPEN (not drop).
    assert.match(
      html,
      /else if \(!processExited\) \{\s*\/\/[^\n]*\n\s*bufferInput\(data\)/,
      'onData must bufferInput when the socket is not OPEN instead of dropping the keystroke'
    )
    // The buffer is bounded (drops oldest) so a long outage cannot grow unbounded.
    assert.match(html, /MAX_PENDING_INPUT/, 'the input buffer must be bounded')
    // Flush happens after scrollback replay so input lands after restored state.
    assert.match(
      html,
      /replay_end[\s\S]{0,200}flushPendingInput\(\)/,
      'buffered input must be flushed on replay_end'
    )
  })

  test('fitTerminal guards the xterm private-internals access so boot cannot throw blank', () => {
    // The cell-size read reaches into term._core._renderService — guard it so a
    // not-yet-measured renderer cannot throw and abort boot before connect().
    assert.match(
      html,
      /try\s*\{\s*cellWidth\s*=\s*term\._core\._renderService/,
      'fitTerminal must wrap the xterm private-internals read in try/catch'
    )
  })
})
