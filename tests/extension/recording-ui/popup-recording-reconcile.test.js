// @ts-nocheck
/**
 * @fileoverview popup-recording-reconcile.test.js — The popup reconciles the
 * StorageKey.RECORDING mirror against the background's live recorder (rule 18,
 * kaboom-jdm5.7): a stale "active" mirror must not show a phantom recording, a
 * live recording must show even if the mirror lags, and an unreachable
 * background falls back to the mirror rather than guessing.
 */

import { test } from 'node:test'
import assert from 'node:assert'

globalThis.chrome = globalThis.chrome || { runtime: { id: 'test', sendMessage: async () => ({}) } }

const { reconcileRecordingState } = await import('../../../extension/popup/recording/recording-io.js')

const mirrorActive = { active: true, name: 'old', startTime: 1000 }

test('live idle state overrides a stale active mirror', () => {
  const live = { active: false, name: '', startTime: 0 }
  assert.deepStrictEqual(reconcileRecordingState(mirrorActive, live), { active: false, name: '', startTime: 0 })
})

test('live active recording wins over an idle mirror', () => {
  const live = { active: true, name: 'now', startTime: 2000 }
  assert.deepStrictEqual(reconcileRecordingState(null, live), live)
})

test('unreachable background falls back to the mirror', () => {
  assert.deepStrictEqual(reconcileRecordingState(mirrorActive, null), mirrorActive)
  assert.strictEqual(reconcileRecordingState(null, null), null)
})
