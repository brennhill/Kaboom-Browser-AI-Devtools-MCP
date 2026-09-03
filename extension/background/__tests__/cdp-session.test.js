// cdp-session.test.js — Contract for the single-owner CDP session manager (kaboom-05ue.1).
//
// Chrome permits exactly one chrome.debugger attachment per (extension, tab). These tests
// pin the behaviour that lets input dispatch, performance tracing and full-page capture
// share one attachment instead of fighting over four independent ones.

import { describe, test, beforeEach } from 'node:test'
import assert from 'node:assert'

const { CDPSessionManager, CDP_SESSION_ERRORS } = await import('../dom/cdp/cdp-session.js')

/** Deterministic clock + timer queue so no test sleeps (repo rule 9). */
function makeClock() {
  let now = 0
  const timers = new Map()
  let nextId = 1
  return {
    now: () => now,
    setTimeout: (fn, ms) => {
      const id = nextId++
      timers.set(id, { fn, at: now + ms })
      return id
    },
    clearTimeout: (id) => timers.delete(id),
    /** Advance the clock and fire every timer whose deadline has passed. */
    advance(ms) {
      now += ms
      const due = [...timers.entries()].filter(([, t]) => t.at <= now)
      for (const [id, t] of due) {
        timers.delete(id)
        t.fn()
      }
    },
    pending: () => timers.size
  }
}

/**
 * Fake chrome.debugger. `probeAttached` models whether Chrome already has us attached
 * (the MV3 service-worker-restart case): a read-only probe resolves instead of rejecting.
 */
function makeDebuggerApi(opts = {}) {
  const calls = { attach: [], detach: [], send: [] }
  // Attachment is tracked PER TAB: Chrome's one-attachment rule is per (extension, tab),
  // so a fake with a single global flag would hide cross-tab bugs.
  const attached = new Set(opts.probeAttached === true ? [1] : [])
  const detachListeners = []
  return {
    calls,
    fireDetach(tabId, reason) {
      attached.delete(tabId)
      for (const l of detachListeners) l({ tabId }, reason)
    },
    isAttached: (tabId) => attached.has(tabId),
    api: {
      attach: async (target, version) => {
        calls.attach.push({ target, version })
        if (opts.attachError) throw new Error(opts.attachError)
        attached.add(target.tabId)
      },
      detach: async (target) => {
        calls.detach.push({ target })
        attached.delete(target.tabId)
      },
      sendCommand: async (target, method, params) => {
        calls.send.push({ target, method, params })
        if (!attached.has(target.tabId)) {
          throw new Error(`Debugger is not attached to the tab with id: ${target.tabId}`)
        }
        if (opts.sendError && opts.sendError.method === method) throw new Error(opts.sendError.message)
        return { ok: true, method }
      },
      onDetach: { addListener: (cb) => detachListeners.push(cb) }
    }
  }
}

function makeManager(dbgOpts = {}, mgrOpts = {}) {
  const clock = makeClock()
  const dbg = makeDebuggerApi(dbgOpts)
  const mgr = new CDPSessionManager({
    debuggerApi: dbg.api,
    setTimeout: clock.setTimeout,
    clearTimeout: clock.clearTimeout,
    idleGraceMs: 30_000,
    ...mgrOpts
  })
  return { mgr, dbg, clock }
}

describe('CDPSessionManager — warm reuse', () => {
  test('attaches once across many sequential actions', async () => {
    const { mgr, dbg } = makeManager()
    for (let i = 0; i < 5; i++) {
      const lease = await mgr.acquire(1)
      await lease.send('Input.dispatchMouseEvent', { type: 'mousePressed' })
      lease.release()
    }
    assert.strictEqual(dbg.calls.attach.length, 1, 'expected exactly one attach across 5 actions')
  })

  test('does not detach between actions inside the idle grace', async () => {
    const { mgr, dbg, clock } = makeManager()
    const a = await mgr.acquire(1)
    a.release()
    clock.advance(5_000)
    const b = await mgr.acquire(1)
    b.release()
    assert.strictEqual(dbg.calls.detach.length, 0, 'must not detach while inside the grace window')
    assert.strictEqual(dbg.calls.attach.length, 1, 'must reuse the warm session')
  })

  test('detaches once the idle grace elapses at zero refs', async () => {
    const { mgr, dbg, clock } = makeManager()
    const lease = await mgr.acquire(1)
    lease.release()
    assert.strictEqual(dbg.calls.detach.length, 0)
    clock.advance(30_000)
    assert.strictEqual(dbg.calls.detach.length, 1, 'expected detach after the grace window')
  })

  test('nested leases keep the session alive until the last release', async () => {
    const { mgr, dbg, clock } = makeManager()
    const outer = await mgr.acquire(1)
    const inner = await mgr.acquire(1)
    inner.release()
    clock.advance(30_000)
    assert.strictEqual(dbg.calls.detach.length, 0, 'outer lease must hold the session open')
    outer.release()
    clock.advance(30_000)
    assert.strictEqual(dbg.calls.detach.length, 1)
  })
})

describe('CDPSessionManager — detach is authoritative', () => {
  test('external detach invalidates outstanding leases and send fails loud', async () => {
    const { mgr, dbg } = makeManager()
    const lease = await mgr.acquire(1)
    dbg.fireDetach(1, 'canceled_by_user')
    assert.strictEqual(lease.valid, false, 'lease must observe the detach')
    await assert.rejects(
      () => lease.send('Input.dispatchMouseEvent', {}),
      (err) => err.message.includes(CDP_SESSION_ERRORS.INVALIDATED),
      'a dead lease must fail loud, never silently no-op (rule 25)'
    )
  })

  test('a new acquire after external detach reattaches', async () => {
    const { mgr, dbg } = makeManager()
    const first = await mgr.acquire(1)
    dbg.fireDetach(1, 'target_closed')
    first.release()
    const second = await mgr.acquire(1)
    assert.strictEqual(dbg.calls.attach.length, 2, 'must reattach after an external detach')
    assert.strictEqual(second.valid, true)
  })
})

describe('CDPSessionManager — exclusivity', () => {
  test('shared acquire is refused while an exclusive lease is held', async () => {
    const { mgr } = makeManager()
    const trace = await mgr.acquire(1, { exclusive: true })
    await assert.rejects(
      () => mgr.acquire(1),
      (err) => err.message.includes(CDP_SESSION_ERRORS.EXCLUSIVE_HELD),
      'input must be refused by name, not silently corrupt the trace'
    )
    trace.release()
  })

  test('exclusive acquire waits for shared leases to drain', async () => {
    const { mgr, clock } = makeManager()
    const input = await mgr.acquire(1)
    let granted = false
    const pending = mgr.acquire(1, { exclusive: true, drainTimeoutMs: 5_000 }).then((l) => {
      granted = true
      return l
    })
    await Promise.resolve()
    assert.strictEqual(granted, false, 'exclusive must not be granted while a shared lease is open')
    input.release()
    const lease = await pending
    assert.strictEqual(granted, true, 'exclusive must be granted once shared leases drain')
    lease.release()
    clock.advance(30_000)
  })

  test('exclusive acquire fails by name when shared leases never drain', async () => {
    const { mgr, clock } = makeManager()
    const stuck = await mgr.acquire(1)
    const pending = mgr.acquire(1, { exclusive: true, drainTimeoutMs: 5_000 })
    clock.advance(5_000)
    await assert.rejects(
      () => pending,
      (err) => err.message.includes(CDP_SESSION_ERRORS.DRAIN_TIMEOUT)
    )
    stuck.release()
  })
})

describe('CDPSessionManager — lifecycle hygiene', () => {
  test('release is idempotent and does not double-decrement', async () => {
    const { mgr, dbg, clock } = makeManager()
    const a = await mgr.acquire(1)
    const b = await mgr.acquire(1)
    a.release()
    a.release()
    a.release()
    clock.advance(30_000)
    assert.strictEqual(dbg.calls.detach.length, 0, 'repeat release must not drop another lease’s ref')
    b.release()
    clock.advance(30_000)
    assert.strictEqual(dbg.calls.detach.length, 1)
  })

  test('a failing send does not leak the reference count', async () => {
    const { mgr, dbg, clock } = makeManager({
      sendError: { method: 'Input.dispatchMouseEvent', message: 'boom' }
    })
    const lease = await mgr.acquire(1)
    await assert.rejects(() => lease.send('Input.dispatchMouseEvent', {}))
    lease.release()
    clock.advance(30_000)
    assert.strictEqual(dbg.calls.detach.length, 1, 'a thrown send must still allow the session to close')
  })

  test('adopts an attachment that survived a service-worker restart', async () => {
    const { mgr, dbg } = makeManager({ probeAttached: true })
    const lease = await mgr.acquire(1)
    assert.strictEqual(dbg.calls.attach.length, 0, 'must adopt the live attachment rather than attach twice')
    assert.strictEqual(lease.valid, true)
    const probes = dbg.calls.send.filter((c) => c.method === 'Page.getLayoutMetrics')
    assert.strictEqual(probes.length, 1, 'adoption is decided by a read-only probe against the live session')
  })

  test('enables a CDP domain once per session, not per action', async () => {
    const { mgr, dbg } = makeManager()
    for (let i = 0; i < 3; i++) {
      const lease = await mgr.acquire(1)
      await lease.ensureDomain('Page')
      await lease.ensureDomain('Page')
      lease.release()
    }
    const enables = dbg.calls.send.filter((c) => c.method === 'Page.enable')
    assert.strictEqual(enables.length, 1, 'Page.enable must be sent once per session')
  })

  test('domain enablement resets after the session closes', async () => {
    const { mgr, dbg, clock } = makeManager()
    const first = await mgr.acquire(1)
    await first.ensureDomain('Page')
    first.release()
    clock.advance(30_000)
    const second = await mgr.acquire(1)
    await second.ensureDomain('Page')
    const enables = dbg.calls.send.filter((c) => c.method === 'Page.enable')
    assert.strictEqual(enables.length, 2, 'a fresh session must re-enable its domains')
    second.release()
  })

  test('sessions are per tab and do not share reference counts', async () => {
    const { mgr, dbg, clock } = makeManager()
    const one = await mgr.acquire(1)
    const two = await mgr.acquire(2)
    one.release()
    clock.advance(30_000)
    assert.deepStrictEqual(
      dbg.calls.detach.map((c) => c.target.tabId),
      [1],
      'closing tab 1 must not close tab 2'
    )
    two.release()
    clock.advance(30_000)
    assert.strictEqual(dbg.calls.detach.length, 2)
  })
})

describe('CDPSessionManager — user abort', () => {
  // This is what makes the supervision Stop button real. Without an abort the action keeps
  // running behind a dismissed overlay: the user is told they stopped the agent and did not.
  test('abort invalidates every outstanding lease so the in-flight action cannot continue', async () => {
    const { mgr, dbg } = makeManager()
    const lease = await mgr.acquire(1)
    mgr.abort(1, 'stopped_by_user')
    assert.strictEqual(lease.valid, false, 'the running action must lose its session')
    await assert.rejects(
      () => lease.send('Input.dispatchMouseEvent', {}),
      (err) => err.message.includes(CDP_SESSION_ERRORS.INVALIDATED)
    )
    assert.strictEqual(dbg.calls.detach.length, 1, 'abort detaches immediately, not after the idle grace')
  })

  test('a later action reattaches cleanly after an abort', async () => {
    const { mgr, dbg } = makeManager()
    const first = await mgr.acquire(1)
    mgr.abort(1, 'stopped_by_user')
    first.release()
    const second = await mgr.acquire(1)
    assert.strictEqual(second.valid, true, 'an abort must not poison the tab for later work')
    assert.strictEqual(dbg.calls.attach.length, 2)
  })

  test('aborting a tab with no session is inert', async () => {
    const { mgr, dbg } = makeManager()
    mgr.abort(99, 'stopped_by_user')
    assert.strictEqual(dbg.calls.detach.length, 0)
  })

  test('abort does not touch other tabs', async () => {
    const { mgr, dbg } = makeManager()
    const one = await mgr.acquire(1)
    const two = await mgr.acquire(2)
    mgr.abort(1, 'stopped_by_user')
    assert.strictEqual(one.valid, false)
    assert.strictEqual(two.valid, true, 'a stop on one tab must not interrupt another')
    assert.deepStrictEqual(dbg.calls.detach.map((c) => c.target.tabId), [1])
  })
})
