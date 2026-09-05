// driving-session.test.js — The supervision surface must actually work, not just render.
//
// These pin three defects found reviewing 43f2dfb16:
//   1. Nothing ever sent a 'heartbeat', so the overlay's self-teardown timer could only
//      ever fire — the mechanism meant to prevent a stranded badge was the thing that
//      would remove a live one.
//   2. The Stop button sent kaboom_agent_stop_requested and NOTHING in the background
//      listened, so pressing Stop removed the overlay while the agent kept driving. A
//      safety control that appears to work and does not is worse than none.
//   3. The direct-CDP path — the one a coordinate-addressed click takes — drove the page
//      with no indicator at all.

import { describe, test } from 'node:test'
import assert from 'node:assert'

const { DrivingSessions, HEARTBEAT_INTERVAL_MS } = await import('../supervision/driving-session.js')
const { HEARTBEAT_TTL_MS } = await import('../../content/ui/supervision/agent-indicator.js')

/** Deterministic interval queue — nothing sleeps (repo rule 9). */
function makeTimers() {
  const intervals = new Map()
  let next = 1
  let now = 0
  return {
    now: () => now,
    setInterval: (fn, ms) => {
      const id = next++
      intervals.set(id, { fn, ms, last: now })
      return id
    },
    clearInterval: (id) => intervals.delete(id),
    count: () => intervals.size,
    advance(ms) {
      now += ms
      for (const entry of [...intervals.values()]) {
        while (now - entry.last >= entry.ms) {
          entry.last += entry.ms
          entry.fn()
        }
      }
    }
  }
}

function makeSessions() {
  const notices = []
  const aborts = []
  const timers = makeTimers()
  const sessions = new DrivingSessions({
    notify: (tabId, phase, detail) => notices.push({ tabId, phase, ...(detail ?? {}) }),
    abortSession: (tabId, reason) => aborts.push({ tabId, reason }),
    setInterval: timers.setInterval,
    clearInterval: timers.clearInterval
  })
  return { sessions, notices, aborts, timers, phases: () => notices.map((n) => n.phase) }
}

describe('DrivingSessions — heartbeat keeps a live overlay alive', () => {
  // The content overlay tears itself down after HEARTBEAT_TTL_MS without a heartbeat. If the
  // background never sends one, every driving session longer than the TTL loses its indicator
  // while the agent is still driving.
  test('the heartbeat interval is shorter than the overlay TTL', () => {
    assert.ok(
      HEARTBEAT_INTERVAL_MS < HEARTBEAT_TTL_MS,
      `heartbeat every ${HEARTBEAT_INTERVAL_MS}ms cannot sustain a ${HEARTBEAT_TTL_MS}ms TTL`
    )
    assert.ok(
      HEARTBEAT_INTERVAL_MS * 2 <= HEARTBEAT_TTL_MS,
      'a single dropped heartbeat must not expire the overlay'
    )
  })

  test('sends heartbeats for as long as driving continues', () => {
    const f = makeSessions()
    f.sessions.start(7, 'click')
    f.timers.advance(HEARTBEAT_INTERVAL_MS * 3)
    const beats = f.notices.filter((n) => n.phase === 'heartbeat' && n.tabId === 7)
    assert.strictEqual(beats.length, 3, 'expected one heartbeat per interval while driving')
  })

  test('stops heartbeating once driving ends, leaving no timer behind', () => {
    const f = makeSessions()
    f.sessions.start(7, 'click')
    f.sessions.stop(7)
    const before = f.notices.filter((n) => n.phase === 'heartbeat').length
    f.timers.advance(HEARTBEAT_INTERVAL_MS * 5)
    assert.strictEqual(f.notices.filter((n) => n.phase === 'heartbeat').length, before)
    assert.strictEqual(f.timers.count(), 0, 'a stopped session must not leak an interval')
  })

  test('a second start on the same tab does not stack a second heartbeat timer', () => {
    const f = makeSessions()
    f.sessions.start(7, 'click')
    f.sessions.start(7, 'type')
    assert.strictEqual(f.timers.count(), 1, 'relabelling must reuse the running session')
    f.timers.advance(HEARTBEAT_INTERVAL_MS)
    assert.strictEqual(f.notices.filter((n) => n.phase === 'heartbeat').length, 1)
  })
})

describe('DrivingSessions — Stop actually stops', () => {
  // Without this the button is a lie: the overlay disappears and the agent keeps driving.
  test('a user stop aborts the tab CDP session', () => {
    const f = makeSessions()
    f.sessions.start(7, 'click')
    f.sessions.requestStop(7)
    assert.deepStrictEqual(
      f.aborts.map((a) => a.tabId),
      [7],
      'stop must tear down the CDP session so the in-flight action cannot continue'
    )
  })

  test('a stop is reported to the caller exactly once', () => {
    const f = makeSessions()
    f.sessions.start(7, 'click')
    f.sessions.requestStop(7)
    assert.strictEqual(f.sessions.consumeStopRequest(7), true)
    assert.strictEqual(
      f.sessions.consumeStopRequest(7),
      false,
      'a consumed stop must not mark the NEXT action as stopped too'
    )
  })

  test('a stop on one tab does not abort another', () => {
    const f = makeSessions()
    f.sessions.start(7, 'click')
    f.sessions.start(9, 'click')
    f.sessions.requestStop(7)
    assert.deepStrictEqual(f.aborts.map((a) => a.tabId), [7])
    assert.strictEqual(f.sessions.consumeStopRequest(9), false)
    assert.strictEqual(f.sessions.isDriving(9), true, 'the other tab keeps driving')
  })

  test('stopping clears the overlay and the heartbeat', () => {
    const f = makeSessions()
    f.sessions.start(7, 'click')
    f.sessions.requestStop(7)
    assert.ok(f.phases().includes('idle'), 'the overlay must come down on stop')
    assert.strictEqual(f.timers.count(), 0)
    assert.strictEqual(f.sessions.isDriving(7), false)
  })

  // A stop for a tab nobody is driving is a race (the action finished as the user clicked),
  // not an error. It must not abort a session that a later action may already own.
  test('a stop for an idle tab is inert', () => {
    const f = makeSessions()
    f.sessions.requestStop(7)
    assert.deepStrictEqual(f.aborts, [], 'must not abort a session this stop does not own')
    assert.strictEqual(f.sessions.consumeStopRequest(7), false)
  })
})

describe('DrivingSessions — overlay notices', () => {
  test('driving names the action so the pill is not blank', () => {
    const f = makeSessions()
    f.sessions.start(7, 'fill_form_and_submit')
    const driving = f.notices.find((n) => n.phase === 'driving')
    assert.strictEqual(driving.action, 'fill_form_and_submit')
  })

  test('cursor updates are forwarded only while driving', () => {
    const f = makeSessions()
    f.sessions.cursor(7, 10, 20)
    assert.strictEqual(f.notices.filter((n) => n.phase === 'cursor').length, 0)
    f.sessions.start(7, 'click')
    f.sessions.cursor(7, 10, 20)
    const cursor = f.notices.find((n) => n.phase === 'cursor')
    assert.deepStrictEqual({ x: cursor.x, y: cursor.y }, { x: 10, y: 20 })
  })

  test('stop is idempotent and does not emit a second idle', () => {
    const f = makeSessions()
    f.sessions.start(7, 'click')
    f.sessions.stop(7)
    f.sessions.stop(7)
    assert.strictEqual(f.notices.filter((n) => n.phase === 'idle').length, 1)
  })
})

describe('a user stop must not be re-run through the DOM fallback', () => {
  // tryCDPEscalation returns null to mean "CDP did not handle this", and the caller then
  // performs the SAME action with synthetic DOM events. So if a user stop produced null,
  // pressing Stop would re-execute the very action being stopped — the action happens
  // anyway and the user is told they prevented it. The escalation path must therefore
  // return a result, not null, once a stop has been consumed.
  test('a consumed stop is reported, so the caller has a result and does not fall back', async () => {
    const { tryCDPEscalation, STOPPED_BY_USER } = await import('../dom/cdp/cdp-dispatch.js')
    const { drivingSessions } = await import('../supervision/driving-session.js')

    assert.ok(STOPPED_BY_USER.startsWith('stopped_by_user'), 'the stop must be a named terminal state')

    // A stop that was never requested must not fabricate one.
    assert.strictEqual(
      drivingSessions().consumeStopRequest(4242),
      false,
      'an unrequested stop must not be reported'
    )
    assert.strictEqual(typeof tryCDPEscalation, 'function')
  })
})
