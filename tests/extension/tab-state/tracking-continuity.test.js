// tracking-continuity.test.js — Pins stable tracked-tab navigation transitions.
import assert from 'node:assert/strict'
import { test } from 'node:test'

import {
  TrackingContinuity
} from '../../../extension/background/runtime-state/tracking-continuity.js'

test('cross-origin navigation retains stable tracked tab through recovery phases', () => {
  const machine = new TrackingContinuity()

  machine.establish(42, 'https://one.example/')
  machine.navigationStarted(42)
  machine.observeProvisionalURL(42, 'https://two.example/')
  machine.injectionStarted(42)
  machine.extensionReconnectStarted(42)

  const recovering = machine.snapshot()
  assert.equal(recovering.tab_id, 42)
  assert.equal(recovering.is_tracked, true)
  assert.equal(recovering.phase, 'extension_reconnecting')
  assert.equal(recovering.provisional_url, 'https://two.example/')

  machine.confirm(42, 'https://two.example/')
  assert.deepEqual(machine.snapshot(), {
    tab_id: 42,
    phase: 'confirmed',
    is_tracked: true,
    confirmed_url: 'https://two.example/'
  })
})

test('stale tab events cannot hijack or clear the tracked identity', () => {
  const machine = new TrackingContinuity()
  machine.establish(42, 'https://one.example/')

  machine.navigationStarted(9)
  machine.observeProvisionalURL(9, 'https://attacker.example/')
  machine.close(9)

  assert.equal(machine.snapshot().tab_id, 42)
  assert.equal(machine.snapshot().phase, 'confirmed')
})

test('closing the stable tracked tab is the explicit transition to idle', () => {
  const machine = new TrackingContinuity()
  machine.establish(42, 'https://one.example/')
  machine.navigationStarted(42)
  machine.close(42)

  assert.deepEqual(machine.snapshot(), { phase: 'idle', is_tracked: false })
})

test('failed reinjection remains tracked and retains recovery context', () => {
  const machine = new TrackingContinuity()
  machine.establish(42, 'https://one.example/')
  machine.navigationStarted(42)
  machine.fail(42, 'content_script_unavailable')

  assert.deepEqual(machine.snapshot(), {
    tab_id: 42,
    phase: 'recovery_failed',
    is_tracked: true,
    confirmed_url: 'https://one.example/',
    failure: 'content_script_unavailable'
  })
})

test('content readiness cannot establish tracking for an untracked tab', () => {
  const machine = new TrackingContinuity()
  machine.confirm(42, 'https://untracked.example/')
  assert.deepEqual(machine.snapshot(), { phase: 'idle', is_tracked: false })
})

test('late tab-complete event cannot move a ready page back into injection', () => {
  const machine = new TrackingContinuity()
  machine.establish(42, 'https://one.example/')
  machine.navigationStarted(42)
  machine.observeProvisionalURL(42, 'https://two.example/')
  machine.confirm(42, 'https://two.example/')
  machine.injectionStarted(42)

  assert.equal(machine.snapshot().phase, 'confirmed')
})
