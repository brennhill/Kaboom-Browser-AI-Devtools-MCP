// @ts-nocheck
/**
 * @fileoverview agent-indicator.test.js — Supervision surface behaviour (kaboom-05ue.3).
 *
 * Covers the parts that decide what the human sees and whether a stop is honoured. The
 * state machine takes an injected clock, so heartbeat expiry is asserted without waiting.
 */

import { describe, test } from 'node:test'
import assert from 'node:assert'

const {
  AgentIndicatorCore,
  isHonouredStop,
  drivingLabel,
  HEARTBEAT_TTL_MS,
  AGENT_INDICATOR_IDS,
  OVERLAY_Z_INDEX
} = await import('../../../extension/content/ui/agent-indicator.js')

/** Injected clock so nothing sleeps (repo rule 9). */
function makeClock(start = 1_000) {
  let t = start
  return { now: () => t, advance: (ms) => (t += ms) }
}

describe('AgentIndicatorCore — driving state', () => {
  test('starts not driving, so a fresh page shows nothing', () => {
    const core = new AgentIndicatorCore(makeClock().now)
    assert.strictEqual(core.driving, false)
    assert.strictEqual(core.snapshot().action, null)
  })

  test('driving spans the lease, and relabelling does not restart it', () => {
    const clock = makeClock()
    const core = new AgentIndicatorCore(clock.now)
    core.startDriving('click')
    assert.strictEqual(core.driving, true)
    core.startDriving('type')
    assert.strictEqual(core.snapshot().action, 'type', 'a second action relabels the same session')
    assert.strictEqual(core.driving, true)
  })

  test('stopDriving clears the cursor so no ghost pointer is left behind', () => {
    const core = new AgentIndicatorCore(makeClock().now)
    core.startDriving('click')
    core.moveCursor(120, 240)
    core.stopDriving()
    const snap = core.snapshot()
    assert.strictEqual(snap.driving, false)
    assert.strictEqual(snap.cursor, null)
    assert.strictEqual(snap.action, null)
  })

  test('snapshot is a copy the caller cannot mutate back into the core', () => {
    const core = new AgentIndicatorCore(makeClock().now)
    core.startDriving('click')
    core.moveCursor(5, 6)
    const snap = core.snapshot()
    snap.cursor.x = 999
    snap.driving = false
    assert.strictEqual(core.snapshot().cursor.x, 5)
    assert.strictEqual(core.driving, true)
  })
})

describe('AgentIndicatorCore — phantom cursor', () => {
  test('tracks the target the agent is about to act on', () => {
    const core = new AgentIndicatorCore(makeClock().now)
    core.startDriving('click')
    assert.strictEqual(core.moveCursor(300, 400), true)
    assert.deepStrictEqual(core.snapshot().cursor, { x: 300, y: 400 })
  })

  // A cursor drawn while nothing is driving would assert activity that is not happening.
  test('refuses to move while not driving', () => {
    const core = new AgentIndicatorCore(makeClock().now)
    assert.strictEqual(core.moveCursor(10, 10), false)
    assert.strictEqual(core.snapshot().cursor, null)
  })
})

describe('AgentIndicatorCore — heartbeat', () => {
  test('holds the overlay while heartbeats keep arriving', () => {
    const clock = makeClock()
    const core = new AgentIndicatorCore(clock.now)
    core.startDriving('click')
    for (let i = 0; i < 5; i++) {
      clock.advance(HEARTBEAT_TTL_MS - 1)
      core.heartbeat()
      assert.strictEqual(core.tick(), null, 'a live worker must keep the overlay up')
    }
    assert.strictEqual(core.driving, true)
  })

  // MV3 kills the service worker without warning. Without self-teardown the user keeps a
  // permanent "an agent is driving this tab" badge on a tab nothing is driving.
  test('tears itself down when the worker stops heartbeating', () => {
    const clock = makeClock()
    const core = new AgentIndicatorCore(clock.now)
    core.startDriving('click')
    clock.advance(HEARTBEAT_TTL_MS + 1)
    assert.strictEqual(core.tick(), 'heartbeat_expired')
    assert.strictEqual(core.driving, false, 'the overlay must not survive its own worker')
  })

  test('expiry is reported once, not on every later tick', () => {
    const clock = makeClock()
    const core = new AgentIndicatorCore(clock.now)
    core.startDriving('click')
    clock.advance(HEARTBEAT_TTL_MS + 1)
    assert.strictEqual(core.tick(), 'heartbeat_expired')
    assert.strictEqual(core.tick(), null)
  })

  test('ticks are inert while not driving', () => {
    const clock = makeClock()
    const core = new AgentIndicatorCore(clock.now)
    clock.advance(HEARTBEAT_TTL_MS * 10)
    assert.strictEqual(core.tick(), null)
  })

  test('exactly at the TTL boundary the overlay is still held', () => {
    const clock = makeClock()
    const core = new AgentIndicatorCore(clock.now)
    core.startDriving('click')
    clock.advance(HEARTBEAT_TTL_MS)
    assert.strictEqual(core.tick(), null, 'the boundary must not tear down early')
  })
})

describe('stop control is gated on a real user gesture', () => {
  // A page can dispatch a synthetic click on any element in its own document. Without the
  // isTrusted gate a hostile page could abort the agent at will, or fire it constantly and
  // strip the stop control of meaning.
  test('a synthetic click is refused', () => {
    assert.strictEqual(isHonouredStop({ isTrusted: false }), false)
  })

  test('a real gesture is honoured', () => {
    assert.strictEqual(isHonouredStop({ isTrusted: true }), true)
  })

  test('a missing or malformed event is refused, not defaulted', () => {
    assert.strictEqual(isHonouredStop(null), false)
    assert.strictEqual(isHonouredStop(undefined), false)
    assert.strictEqual(isHonouredStop({}), false)
  })
})

describe('driving label', () => {
  test('names the action so the user knows what is happening', () => {
    assert.match(drivingLabel('click'), /driving this tab/)
    assert.match(drivingLabel('click'), /click/)
  })

  test('reads as prose for multi-word action names', () => {
    assert.match(drivingLabel('fill_form_and_submit'), /fill form and submit/)
  })

  test('falls back to a bare statement rather than an empty dash', () => {
    for (const value of [null, '', '   ']) {
      const label = drivingLabel(value)
      assert.strictEqual(label, 'Kaboom is driving this tab')
      assert.ok(!label.includes('—'), 'no dangling separator with nothing after it')
    }
  })

  test('truncates a hostile action name instead of overflowing the pill', () => {
    const label = drivingLabel('x'.repeat(500))
    assert.ok(label.length < 90, `label must stay bounded, got ${label.length}`)
    assert.ok(label.endsWith('…'))
  })
})

describe('overlay identity', () => {
  test('exposes stable ids and a z-index above page content', () => {
    assert.strictEqual(AGENT_INDICATOR_IDS.root, 'kaboom-agent-indicator')
    for (const key of ['root', 'cursor', 'glow', 'pill', 'stop']) {
      assert.match(AGENT_INDICATOR_IDS[key], /^kaboom-/, `${key} id must be namespaced`)
    }
    assert.ok(OVERLAY_Z_INDEX >= 2147483000, 'must sit above realistic page z-indexes')
  })
})
