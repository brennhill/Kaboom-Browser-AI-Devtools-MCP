// @ts-nocheck
/**
 * @fileoverview telemetry-beacon-optout.test.js — Regression tests for the telemetry
 * opt-out startup race: beacon() must wait for the opt-out flag to hydrate from
 * chrome.storage.local before sending, so opted-out users never emit beacons fired
 * during service-worker startup (e.g. extension_start).
 */

import { describe, mock, test } from 'node:test'
import assert from 'node:assert'

import { createMockChrome } from './helpers.js'

let importCounter = 1000

function setupChrome({ storedOptOut, hydrateDelayMs }) {
  const sendBeacon = mock.fn(() => true)
  const storageGet = mock.fn((key, callback) => {
    const respond = () => callback({ kaboom_telemetry_off: storedOptOut })
    if (hydrateDelayMs > 0) {
      setTimeout(respond, hydrateDelayMs)
    } else {
      respond()
    }
  })

  globalThis.chrome = createMockChrome({
    runtime: {
      getManifest: () => ({ version: '1.2.3' })
    },
    storage: {
      local: {
        get: storageGet,
        set: mock.fn(),
        remove: mock.fn()
      },
      onChanged: {
        addListener: mock.fn()
      }
    }
  })

  Object.defineProperty(globalThis, 'navigator', {
    configurable: true,
    writable: true,
    value: { sendBeacon }
  })

  return { sendBeacon }
}

describe('telemetry beacon opt-out startup race', () => {
  test('beacon fired before hydration completes is suppressed for opted-out users', async () => {
    // Storage hydration is delayed: the opt-out flag is NOT yet known when beacon() fires.
    const { sendBeacon } = setupChrome({ storedOptOut: true, hydrateDelayMs: 20 })
    const mod = await import(`../../extension/lib/telemetry-beacon.js?v=${++importCounter}`)

    // Fire immediately — simulates beacon('extension_start') during init.
    mod.beacon('extension_start')

    // Before hydration resolves, nothing must have been sent.
    assert.strictEqual(sendBeacon.mock.calls.length, 0)

    // After hydration resolves with opt-out=true, still nothing must be sent.
    await new Promise((r) => setTimeout(r, 50))
    assert.strictEqual(sendBeacon.mock.calls.length, 0, 'opted-out user must not emit a startup beacon')
  })

  test('beacon fired before hydration completes is delivered for opted-in users', async () => {
    const { sendBeacon } = setupChrome({ storedOptOut: false, hydrateDelayMs: 20 })
    const mod = await import(`../../extension/lib/telemetry-beacon.js?v=${++importCounter}`)

    mod.beacon('extension_start')
    assert.strictEqual(sendBeacon.mock.calls.length, 0, 'send waits for hydration')

    await new Promise((r) => setTimeout(r, 50))
    assert.strictEqual(sendBeacon.mock.calls.length, 1, 'beacon is sent once the flag is hydrated')
  })
})
