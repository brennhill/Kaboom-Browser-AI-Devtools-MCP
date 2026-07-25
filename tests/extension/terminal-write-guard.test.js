// @ts-nocheck
/**
 * terminal-write-guard.test.js — the write-guard must never wedge the terminal
 * forever. Every "in-flight" / "deferred" state has a TERMINAL_GUARD_MAX_WAIT_MS
 * escape hatch: a permanently-down socket (or a stuck focus/typing flag) makes
 * the guard give up LOUDLY (toast + reset) instead of polling in silence.
 *
 * Uses a hand-rolled fake clock (portable across Node 20 CI / Node 26 local —
 * node:test mock.timers changed its enable() signature between those versions)
 * so the 30s deadline and the self-rescheduling pollers are driven
 * deterministically without real waiting.
 */
import { beforeEach, afterEach, describe, test } from 'node:test'
import assert from 'node:assert'

import {
  state,
  resetAllState,
  TERMINAL_GUARD_MAX_WAIT_MS,
  TERMINAL_TYPING_IDLE_MS
} from '../../extension/content/ui/terminal-widget-types.js'
import {
  flushQueuedWrites,
  resetWriteGuardState
} from '../../extension/content/ui/terminal-write-guard.js'

// --- portable fake timers (setTimeout/clearTimeout/Date.now) -----------------
let fakeNow = 0
let timers = []
let nextTimerId = 1
const realSetTimeout = globalThis.setTimeout
const realClearTimeout = globalThis.clearTimeout
const realDateNow = Date.now

function installFakeClock() {
  fakeNow = 1_000_000
  timers = []
  nextTimerId = 1
  globalThis.setTimeout = (cb, delay = 0) => {
    const id = nextTimerId++
    timers.push({ id, cb, due: fakeNow + delay })
    return id
  }
  globalThis.clearTimeout = (id) => {
    timers = timers.filter((t) => t.id !== id)
  }
  Date.now = () => fakeNow
}

function restoreClock() {
  globalThis.setTimeout = realSetTimeout
  globalThis.clearTimeout = realClearTimeout
  Date.now = realDateNow
}

// Advance the fake clock, firing due timers in chronological order and honoring
// timers rescheduled during the advance (the guard's self-polling loop).
function advance(ms) {
  const target = fakeNow + ms
  for (;;) {
    const due = timers.filter((t) => t.due <= target).sort((a, b) => a.due - b.due)
    if (due.length === 0) break
    const next = due[0]
    timers = timers.filter((t) => t.id !== next.id)
    fakeNow = next.due
    next.cb()
  }
  fakeNow = target
}

// Minimal DOM stub so showActionToast (called on give-up) runs headless.
function installDomStub() {
  const makeEl = () => ({ id: '', className: '', textContent: '', style: {}, appendChild() {}, remove() {} })
  globalThis.document = {
    body: makeEl(),
    documentElement: makeEl(),
    head: makeEl(),
    getElementById: () => null,
    createElement: () => makeEl()
  }
  globalThis.requestAnimationFrame = () => {}
  globalThis.chrome = { runtime: { getURL: () => '' } }
}

function fakeIframe() {
  return { contentWindow: { postMessage() {} } }
}

describe('terminal write-guard escape hatch', () => {
  beforeEach(() => {
    installDomStub()
    installFakeClock()
    resetAllState()
  })

  afterEach(() => {
    resetWriteGuardState()
    restoreClock()
  })

  test('flush poller gives up loudly after MAX wait when socket never returns', () => {
    state.visible = true
    state.iframeEl = fakeIframe()
    state.serverUrl = 'http://127.0.0.1:7890'
    state.terminalConnected = false
    state.queuedWrites = ['echo hi\r']

    // First flush attempt: socket down -> marks blocked, schedules a poll.
    flushQueuedWrites()
    assert.ok(state.guardBlockedSince > 0, 'guard should record when it got blocked')
    assert.equal(state.queuedWrites.length, 1, 'write stays queued while momentarily blocked')
    assert.notEqual(state.queuedWriteFlushTimer, null, 'a re-poll must be scheduled, not a silent stop')

    // The poll reschedules itself each tick; past the deadline it must give up.
    advance(TERMINAL_GUARD_MAX_WAIT_MS + 1000)

    assert.equal(state.queuedWrites.length, 0, 'queued writes dropped on give-up')
    assert.equal(state.queuedWriteInFlight, false, 'in-flight cleared on give-up')
    assert.equal(state.guardBlockedSince, 0, 'blocked marker cleared on give-up')
    assert.equal(state.queuedWriteFlushTimer, null, 'no dangling flush poller after give-up')
  })

  test('in-flight submit cannot wedge forever if socket drops mid-write', () => {
    state.visible = true
    state.iframeEl = fakeIframe()
    state.serverUrl = 'http://127.0.0.1:7890'
    state.terminalConnected = true
    state.queuedWrites = ['run tests\r']

    // Dispatch the write -> queuedWriteInFlight becomes true, submit scheduled.
    flushQueuedWrites()
    assert.equal(state.queuedWriteInFlight, true, 'write goes in-flight')

    // Socket drops before the Enter submit fires.
    state.terminalConnected = false

    // Drive time forward well past the deadline: the submit poll must give up.
    advance(TERMINAL_GUARD_MAX_WAIT_MS + 2000)

    assert.equal(state.queuedWriteInFlight, false, 'stuck in-flight write must clear via escape hatch')
    assert.equal(state.queuedSubmitTimer, null, 'no dangling submit poller after give-up')
    assert.equal(state.guardBlockedSince, 0, 'blocked marker cleared on give-up')
  })

  test('momentary disconnect recovers and flushes without dropping (no premature give-up)', () => {
    state.visible = true
    state.iframeEl = fakeIframe()
    state.serverUrl = 'http://127.0.0.1:7890'
    state.terminalConnected = false
    state.queuedWrites = ['ls\r']

    flushQueuedWrites() // blocked (socket down)
    assert.ok(state.guardBlockedSince > 0)

    // Reconnect well before the deadline, then let the next poll run.
    advance(1000)
    state.terminalConnected = true
    advance(500)

    // The write should have dispatched (in-flight) — NOT been dropped.
    assert.equal(state.queuedWrites.length, 0, 'queued write dispatched, not dropped')
    assert.equal(state.queuedWriteInFlight, true, 'write is in flight after reconnect')
    assert.equal(state.guardBlockedSince, 0, 'blocked marker cleared once progress resumes')
  })

  // Regression (kaboom-089): the escape hatch must NOT fire while the terminal is
  // CONNECTED and the user is merely typing — that is self-limiting, not a wedge.
  // Before the fix, 30s of continuous typing dropped a healthy queued write and
  // falsely toasted "terminal not reachable".
  test('connected + continuous typing must NOT drop a healthy write', () => {
    state.visible = true
    state.iframeEl = fakeIframe()
    state.serverUrl = 'http://127.0.0.1:7890'
    state.terminalConnected = true // reachable the entire time
    state.terminalFocused = true
    state.lastTypingAt = Date.now() // user is actively typing
    state.queuedWrites = ['echo hi\r']

    flushQueuedWrites() // connected but typing -> defer, must not drop or accrue
    assert.equal(state.queuedWrites.length, 1, 'write stays queued while the user types')
    assert.equal(state.guardBlockedSince, 0, 'typing is reachable — no unreachable-time accrued')

    // Simulate continuous typing well past the escape-hatch deadline: advance in
    // steps shorter than the typing-idle window, refreshing lastTypingAt each step
    // so shouldDeferQueuedWrite stays true (a keystroke within the idle window).
    const step = Math.floor(TERMINAL_TYPING_IDLE_MS / 2)
    const steps = Math.ceil((TERMINAL_GUARD_MAX_WAIT_MS + 5000) / step)
    for (let i = 0; i < steps; i++) {
      state.lastTypingAt = Date.now()
      advance(step)
    }

    assert.equal(state.queuedWrites.length, 1, 'a healthy write must NOT be dropped during continuous typing')
    assert.equal(state.guardBlockedSince, 0, 'no unreachable-time accrues while connected')
    assert.notEqual(state.queuedWriteFlushTimer, null, 'the flush poller keeps deferring, never gives up')
  })
})
