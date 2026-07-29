// @ts-nocheck
/**
 * @fileoverview recording-rehydration.test.js — Tests for the recording rehydration
 * decision logic (extension/background/recording/rehydration.js).
 * MV3 service workers restart routinely while the offscreen MediaRecorder keeps
 * recording; on startup the SW asks the offscreen document whether a recording is
 * still active and rehydrates in-memory state instead of unconditionally clearing it.
 * The "ask offscreen" dependency is injected so the decision is unit-testable.
 */

import { test, describe, mock } from 'node:test'
import assert from 'node:assert'

const { resolveRecordingRehydration } = await import('../../../extension/background/recording/rehydration.js')

const liveOffscreenState = {
  active: true,
  name: 'checkout-bug--2026-06-10-1010',
  startTime: 1765000000000,
  fps: 30,
  audioMode: 'mic',
  tabId: 42,
  url: 'http://example.com/checkout'
}

describe('resolveRecordingRehydration', () => {
  test('returns null when offscreen is unreachable (no offscreen document)', async () => {
    const getPersistedRecording = mock.fn(() => Promise.resolve({ name: 'stale', startTime: 1 }))
    const result = await resolveRecordingRehydration({
      queryOffscreenRecordingState: mock.fn(() => Promise.resolve(null)),
      getPersistedRecording
    })
    assert.strictEqual(result, null)
    assert.strictEqual(getPersistedRecording.mock.calls.length, 0, 'persisted state is not read when inactive')
  })

  test('returns null when offscreen reports no active recording', async () => {
    const result = await resolveRecordingRehydration({
      queryOffscreenRecordingState: mock.fn(() =>
        Promise.resolve({ active: false, name: '', startTime: 0, fps: 15, audioMode: '', tabId: 0, url: '' })
      ),
      getPersistedRecording: mock.fn(() => Promise.resolve(null))
    })
    assert.strictEqual(result, null)
  })

  test('rehydrates from live offscreen state when a recording survived the SW restart', async () => {
    const result = await resolveRecordingRehydration({
      queryOffscreenRecordingState: mock.fn(() => Promise.resolve(liveOffscreenState)),
      getPersistedRecording: mock.fn(() =>
        Promise.resolve({ active: true, name: 'persisted-name', startTime: 99, queryId: 'q-77' })
      )
    })
    assert.deepStrictEqual(result, {
      active: true,
      name: 'checkout-bug--2026-06-10-1010',
      startTime: 1765000000000,
      fps: 30,
      audioMode: 'mic',
      tabId: 42,
      url: 'http://example.com/checkout',
      queryId: 'q-77'
    })
  })

  test('falls back to persisted metadata for fields the offscreen response is missing', async () => {
    const result = await resolveRecordingRehydration({
      queryOffscreenRecordingState: mock.fn(() =>
        Promise.resolve({ active: true, name: '', startTime: 0, fps: 0, audioMode: '', tabId: 0, url: '' })
      ),
      getPersistedRecording: mock.fn(() =>
        Promise.resolve({
          active: true,
          name: 'persisted-rec',
          startTime: 1765000123456,
          fps: 24,
          audioMode: 'tab',
          tabId: 7,
          url: 'http://example.com/persisted',
          queryId: 'q-1'
        })
      )
    })
    assert.deepStrictEqual(result, {
      active: true,
      name: 'persisted-rec',
      startTime: 1765000123456,
      fps: 24,
      audioMode: 'tab',
      tabId: 7,
      url: 'http://example.com/persisted',
      queryId: 'q-1'
    })
  })

  test('still rehydrates with defaults when persisted metadata read fails', async () => {
    const before = Date.now()
    const result = await resolveRecordingRehydration({
      queryOffscreenRecordingState: mock.fn(() =>
        Promise.resolve({ active: true, name: 'live-rec', startTime: 0, fps: 0, audioMode: '', tabId: 3, url: '' })
      ),
      getPersistedRecording: mock.fn(() => Promise.reject(new Error('storage unavailable')))
    })
    assert.ok(result, 'active offscreen recording must still rehydrate')
    assert.strictEqual(result.active, true)
    assert.strictEqual(result.name, 'live-rec')
    assert.strictEqual(result.fps, 15, 'fps defaults to 15')
    assert.strictEqual(result.queryId, '')
    assert.ok(result.startTime >= before, 'startTime defaults to now when unknown')
  })
})
