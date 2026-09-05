// @ts-nocheck
/**
 * @fileoverview cdp-env-pin.test.js — What the environment-pinning surface actually tells Chrome,
 * and what it reports back (kaboom-x0li.2).
 *
 * Every assertion here decides whether a generated regression test is honest. A clock sent in
 * milliseconds where CDP expects seconds puts the page ~55,000 years into the future and every
 * date on it reads as invalid. A refused override reported as pinned puts a claim in the emitted
 * test that the recording never honoured. Neither needs a browser to catch.
 */

import { test, describe, before, beforeEach } from 'node:test'
import assert from 'node:assert'

const PIN = '../../../extension/background/dom/cdp/cdp-env-pin.js'

/** A fake CDP lease that records every command and can be told to refuse specific methods. */
function recordingLease({ refuse = [], seedActive = true } = {}) {
  const sent = []
  const domains = []
  return {
    sent,
    domains,
    lease: {
      tabId: 7,
      valid: true,
      async ensureDomain(domain) {
        domains.push(domain)
      },
      async send(method, params) {
        sent.push({ method, params })
        if (refuse.includes(method)) throw new Error('refused: ' + method)
        if (method === 'Runtime.evaluate' && String(params.expression).includes('SEED_ACTIVE')) {
          return { result: { value: seedActive } }
        }
        if (method === 'Runtime.evaluate' && String(params.expression).includes('__KABOOM_RANDOM_SEED_ACTIVE__')) {
          return { result: { value: seedActive } }
        }
        return {}
      },
      release() {}
    }
  }
}

function sentParams(sent, method) {
  const entry = sent.find((item) => item.method === method)
  return entry ? entry.params : undefined
}

describe('applyEnvironmentPin', () => {
  let applyEnvironmentPin, clearEnvironmentPin, seedInstallSnippet

  before(async () => {
    ;({ applyEnvironmentPin, clearEnvironmentPin, seedInstallSnippet } = await import(PIN))
  })

  test('sends the clock origin in SECONDS, not milliseconds', async () => {
    const { lease, sent } = recordingLease()
    await applyEnvironmentPin(lease, { clock_epoch_ms: 1788480000000 })
    assert.equal(sentParams(sent, 'Emulation.setVirtualTimePolicy').initialVirtualTime, 1788480000)
  })

  test('defaults the virtual-time policy to advance, which does not freeze the page', async () => {
    const { lease, sent } = recordingLease()
    await applyEnvironmentPin(lease, { clock_epoch_ms: 1000 })
    assert.equal(sentParams(sent, 'Emulation.setVirtualTimePolicy').policy, 'advance')
  })

  test('honours an explicit pause policy for replay', async () => {
    const { lease, sent } = recordingLease()
    await applyEnvironmentPin(lease, { clock_epoch_ms: 1000, virtual_time_policy: 'pause' })
    assert.equal(sentParams(sent, 'Emulation.setVirtualTimePolicy').policy, 'pause')
  })

  test('pins timezone, geolocation and viewport through the documented CDP calls', async () => {
    const { lease, sent } = recordingLease()
    const pin = await applyEnvironmentPin(lease, {
      timezone_id: 'America/New_York',
      latitude: 37.7749,
      longitude: -122.4194,
      accuracy_m: 10,
      viewport_width: 1280,
      viewport_height: 720,
      device_scale_factor: 2
    })

    assert.equal(sentParams(sent, 'Emulation.setTimezoneOverride').timezoneId, 'America/New_York')
    assert.equal(sentParams(sent, 'Emulation.setGeolocationOverride').accuracy, 10)
    assert.equal(sentParams(sent, 'Emulation.setDeviceMetricsOverride').deviceScaleFactor, 2)
    assert.equal(pin.clock.timezone_id, 'America/New_York')
    assert.equal(pin.geolocation.latitude, 37.7749)
    assert.equal(pin.viewport.width, 1280)
    assert.ok(!pin.unpinned, 'nothing was refused, so nothing should be listed as unpinned')
  })

  test('never sends an override the caller did not ask for', async () => {
    const { lease, sent } = recordingLease()
    const pin = await applyEnvironmentPin(lease, { timezone_id: 'UTC' })
    const methods = sent.map((item) => item.method)
    assert.ok(!methods.includes('Emulation.setGeolocationOverride'))
    assert.ok(!methods.includes('Emulation.setDeviceMetricsOverride'))
    assert.ok(!methods.includes('Emulation.setVirtualTimePolicy'))
    assert.equal(pin.geolocation, undefined)
    assert.equal(pin.viewport, undefined)
  })

  test('a refused knob is named in unpinned and the remaining knobs still apply', async () => {
    // The knobs a session asked for and did not get are exactly the ones a replay diverges
    // on. Dropping the refusal silently would report a pin the browser never installed.
    const { lease, sent } = recordingLease({ refuse: ['Emulation.setTimezoneOverride'] })
    const pin = await applyEnvironmentPin(lease, { timezone_id: 'Mars/Olympus', viewport_width: 800, viewport_height: 600 })

    assert.equal(pin.clock, undefined, 'a refused timezone must not be reported as pinned')
    assert.equal(pin.viewport.width, 800, 'one refusal must not abandon the rest of the spec')
    assert.equal(pin.unpinned.length, 1)
    assert.ok(pin.unpinned[0].startsWith('timezone ('), pin.unpinned[0])
    assert.ok(sentParams(sent, 'Emulation.setDeviceMetricsOverride'), 'viewport was still attempted')
  })

  test('a seed that no page-side generator picked up is reported unpinned, not pinned', async () => {
    // early-patch is absent on cloaked domains and wherever CSP blocked it. Setting a seed
    // nothing reads is not pinning, and claiming it would make the test look deterministic.
    const { lease } = recordingLease({ seedActive: false })
    const pin = await applyEnvironmentPin(lease, { random_seed: 'run-1' })

    assert.equal(pin.random_seed, undefined)
    assert.equal(pin.unpinned.length, 1)
    assert.ok(pin.unpinned[0].startsWith('random seed ('), pin.unpinned[0])
  })

  test('a seed installs for the current document and for every document after it', async () => {
    const { lease, sent, domains } = recordingLease()
    const pin = await applyEnvironmentPin(lease, { random_seed: 'run-1' })

    assert.ok(domains.includes('Page'), 'addScriptToEvaluateOnNewDocument needs the Page domain')
    const persisted = sentParams(sent, 'Page.addScriptToEvaluateOnNewDocument')
    assert.ok(persisted.source.includes('run-1'), 'the seed must survive a reload')
    assert.ok(
      sent.some((item) => item.method === 'Runtime.evaluate' && item.params.expression.includes('run-1')),
      'the document already open must be seeded too'
    )
    assert.equal(pin.random_seed, 'run-1')
  })

  test('the injected snippet carries no generator of its own', () => {
    // Two independent Math.random replacements in one page disagree with each other, so the
    // generator lives in early-patch and the snippet only hands it the seed.
    const snippet = seedInstallSnippet('abc')
    assert.ok(snippet.includes('__KABOOM_SEED_RANDOM__'))
    assert.ok(!/Math\.imul|xorshift|Math\.random\s*=/.test(snippet), snippet)
  })

  test('clearEnvironmentPin releases every override and names the ones it could not', async () => {
    const { lease, sent } = recordingLease({ refuse: ['Emulation.clearDeviceMetricsOverride'] })
    const failures = await clearEnvironmentPin(lease)

    const methods = sent.map((item) => item.method)
    assert.ok(methods.includes('Emulation.clearGeolocationOverride'))
    assert.ok(methods.includes('Emulation.setTimezoneOverride'))
    assert.equal(failures.length, 1)
    assert.ok(failures[0].startsWith('viewport ('), failures[0])
  })
})

describe('per-session registry', () => {
  let environmentPinFor, recordEnvironmentPin, forgetEnvironmentPin

  before(async () => {
    ;({ environmentPinFor, recordEnvironmentPin, forgetEnvironmentPin } = await import(PIN))
  })

  beforeEach(() => {
    forgetEnvironmentPin(42)
  })

  test('a tab nobody pinned reports no pin, so the artifact can say so outright', () => {
    assert.equal(environmentPinFor(42), undefined)
    assert.equal(environmentPinFor(undefined), undefined)
  })

  test('a pinned tab reports its pin until it is released', () => {
    recordEnvironmentPin(42, { random_seed: 'run-1' })
    assert.deepEqual(environmentPinFor(42), { random_seed: 'run-1' })
    forgetEnvironmentPin(42)
    assert.equal(environmentPinFor(42), undefined)
  })
})
