// @ts-nocheck
/**
 * @fileoverview action-recording-reconcile.test.js — Regression tests for F5:
 * a persisted "recording in progress" must be reconciled against the daemon on
 * popup open, so a daemon restart does not resurrect a phantom recording — while
 * NEVER deleting a genuinely live recording on uncertainty.
 *
 * Restart detection uses the daemon PID (restart-stable, no clock math): the PID
 * captured at record-start is compared against the daemon's current PID. The
 * reconcile is destructive (it deletes the mirror), so it fails OPEN: it only
 * reports "stale" when it is CONFIDENT (both PIDs known and different).
 */

import { beforeEach, describe, test, mock } from 'node:test'
import assert from 'node:assert'

let healthResponse = null
let shouldThrow = false

// Control the daemon /mcp reply the reconcile helper sees.
mock.module('../../../extension/lib/daemon-http.js', {
  namedExports: {
    postDaemonJSON: mock.fn(async () => {
      if (shouldThrow) throw new Error('daemon unreachable')
      return {
        ok: true,
        status: 200,
        json: async () => healthResponse
      }
    })
  }
})

function makeStorageArea() {
  return {
    get: mock.fn((_k, cb) => (cb ? cb({}) : Promise.resolve({}))),
    set: mock.fn((_d, cb) => (cb ? cb() : Promise.resolve())),
    remove: mock.fn((_k, cb) => (cb ? cb() : Promise.resolve()))
  }
}

globalThis.chrome = {
  runtime: { id: 'test-ext', lastError: null },
  storage: { local: makeStorageArea(), session: makeStorageArea() }
}

const { isActionRecordingStillLive } = await import('../../../extension/popup/recording/action-recording.js')

function healthWithPid(pid) {
  return { result: { content: [{ text: JSON.stringify({ server: { pid, uptime_seconds: 123 } }) }] } }
}

describe('action recording phantom reconciliation (F5, PID-based)', () => {
  beforeEach(() => {
    shouldThrow = false
    healthResponse = null
  })

  test('live when the daemon PID is unchanged since record-start', async () => {
    healthResponse = healthWithPid(4242)
    assert.strictEqual(await isActionRecordingStillLive(4242), true)
  })

  test('stale ONLY when the daemon PID changed (confident restart)', async () => {
    healthResponse = healthWithPid(9999)
    assert.strictEqual(await isActionRecordingStillLive(4242), false)
  })

  test('keeps the mirror when the daemon is unreachable (fail-open, no data loss)', async () => {
    shouldThrow = true
    assert.strictEqual(await isActionRecordingStillLive(4242), true)
  })

  test('keeps the mirror when the health response has no pid (fail-open)', async () => {
    healthResponse = { result: { content: [{ text: 'no pid here' }] } }
    assert.strictEqual(await isActionRecordingStillLive(4242), true)
  })

  test('keeps the mirror when no baseline PID was captured at start (fail-open)', async () => {
    // Never calls the daemon — with no baseline we cannot detect a restart.
    assert.strictEqual(await isActionRecordingStillLive(null), true)
    assert.strictEqual(await isActionRecordingStillLive(undefined), true)
  })

  test('does not falsely delete a live recording across a clock jump (the old bug)', async () => {
    // The daemon is the same process (same PID). No timestamp is consulted, so a
    // laptop sleep / NTP / DST jump between start and reopen cannot mark it stale.
    healthResponse = healthWithPid(4242)
    assert.strictEqual(await isActionRecordingStillLive(4242), true)
  })
})
