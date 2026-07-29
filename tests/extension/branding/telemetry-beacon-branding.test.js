// @ts-nocheck
/**
 * @fileoverview telemetry-beacon-branding.test.js — Verifies Kaboom telemetry host and opt-out keys.
 */

import { beforeEach, describe, mock, test } from 'node:test'
import assert from 'node:assert'

import { createMockChrome } from '../shared/helpers.js'

let importCounter = 0

describe('telemetry beacon branding', () => {
  let sendBeacon
  let storageGet
  let onChangedListener

  beforeEach(() => {
    sendBeacon = mock.fn(() => true)
    storageGet = mock.fn((key, callback) => callback({ kaboom_telemetry_off: false }))
    onChangedListener = undefined

    globalThis.chrome = createMockChrome({
      runtime: {
        getManifest: () => ({ version: '9.9.9' })
      },
      storage: {
        local: {
          get: storageGet,
          set: mock.fn(),
          remove: mock.fn()
        },
        onChanged: {
          addListener: mock.fn((listener) => {
            onChangedListener = listener
          })
        }
      }
    })

    Object.defineProperty(globalThis, 'navigator', {
      configurable: true,
      writable: true,
      value: {
        sendBeacon
      }
    })
  })

  test('uses Kaboom telemetry endpoint and kaboom storage opt-out key', async () => {
    const mod = await import(`../../../extension/lib/telemetry-beacon.js?v=${++importCounter}`)

    mod.beacon('extension_start', { source: 'test' })
    // beacon() awaits opt-out flag hydration before sending
    await new Promise((r) => setTimeout(r, 0))

    assert.strictEqual(storageGet.mock.calls[0].arguments[0], 'kaboom_telemetry_off')
    assert.strictEqual(sendBeacon.mock.calls.length, 1)
    assert.strictEqual(sendBeacon.mock.calls[0].arguments[0], 'https://t.gokaboom.dev/v1/event')

    const payload = JSON.parse(await sendBeacon.mock.calls[0].arguments[1].text())
    assert.deepStrictEqual(payload, {
      event: 'extension_start',
      v: '9.9.9',
      props: { source: 'test' }
    })
  })

  test('respects kaboom runtime opt-out updates', async () => {
    const mod = await import(`../../../extension/lib/telemetry-beacon.js?v=${++importCounter}`)

    onChangedListener({ kaboom_telemetry_off: { newValue: true } }, 'local')
    mod.beacon('extension_start')
    await new Promise((r) => setTimeout(r, 0))

    assert.strictEqual(sendBeacon.mock.calls.length, 0)
  })
})
