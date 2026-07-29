// @ts-nocheck
/**
 * @fileoverview terminal-session-stop.test.js — Regression tests for M1:
 * stopActiveSession must CONFIRM the session is gone, not fire-and-forget. The
 * session id is a fixed "default", so if a folder-change stop times out but the
 * old session survives, the following /terminal/start 409s and the client
 * silently reconnects to the OLD working directory. A confirmed teardown (200/404
 * or a validate poll) prevents that.
 */

import { beforeEach, describe, test, mock } from 'node:test'
import assert from 'node:assert'

let fetchCalls
let stopBehavior // 'ok' | '404' | 'reject' | '500'
let validateSequence // booleans returned by successive /terminal/validate calls

function installEnv() {
  fetchCalls = []
  globalThis.chrome = {
    runtime: { id: 'x', lastError: null },
    storage: {
      local: { get: mock.fn((_k, cb) => (cb ? cb({}) : Promise.resolve({}))) },
      session: {
        get: mock.fn((k, cb) => {
          const keys = Array.isArray(k) ? k : [k]
          const out = {}
          for (const key of keys) {
            if (String(key).includes('terminal_session')) {
              out[key] = { sessionId: 'default', token: 'tok-old' }
            }
          }
          return cb ? cb(out) : Promise.resolve(out)
        }),
        set: mock.fn((_d, cb) => (cb ? cb() : Promise.resolve())),
        remove: mock.fn((_k, cb) => (cb ? cb() : Promise.resolve()))
      }
    }
  }
  let validateIdx = 0
  globalThis.fetch = mock.fn(async (url) => {
    const u = String(url)
    fetchCalls.push(u)
    if (u.includes('/terminal/stop')) {
      if (stopBehavior === 'reject') throw new Error('timeout')
      const status = stopBehavior === '404' ? 404 : stopBehavior === 'ok' ? 200 : 500
      return { ok: status >= 200 && status < 300, status, json: async () => ({}) }
    }
    if (u.includes('/terminal/validate')) {
      const valid = validateSequence[Math.min(validateIdx++, validateSequence.length - 1)]
      return { ok: true, status: 200, json: async () => ({ valid }) }
    }
    throw new Error(`unexpected fetch: ${u}`)
  })
}

const { stopActiveSession } = await import('../../../extension/content/ui/terminal-widget-session.js')

describe('stopActiveSession confirms teardown (M1)', () => {
  beforeEach(() => { installEnv() })

  test('a 200 stop returns immediately without polling validate', async () => {
    stopBehavior = 'ok'
    validateSequence = [false]
    await stopActiveSession()
    assert.strictEqual(
      fetchCalls.some((u) => u.includes('/terminal/validate')),
      false,
      'a confirmed (200) stop needs no validation poll'
    )
  })

  test('a 404 stop (already gone) also returns without polling', async () => {
    stopBehavior = '404'
    validateSequence = [false]
    await stopActiveSession()
    assert.strictEqual(fetchCalls.some((u) => u.includes('/terminal/validate')), false)
  })

  test('a timed-out stop polls validate until the session is gone', async () => {
    stopBehavior = 'reject'
    validateSequence = [true, false] // still alive, then gone
    await stopActiveSession()
    const validateCount = fetchCalls.filter((u) => u.includes('/terminal/validate')).length
    assert.ok(validateCount >= 1, 'an unconfirmed stop must verify the session actually tore down')
  })

  test('a non-2xx stop also triggers confirmation polling', async () => {
    stopBehavior = '500'
    validateSequence = [false]
    await stopActiveSession()
    assert.ok(fetchCalls.some((u) => u.includes('/terminal/validate')))
  })
})
