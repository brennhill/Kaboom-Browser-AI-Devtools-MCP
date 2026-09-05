// @ts-nocheck
/**
 * @fileoverview env-pin-command.test.js — What the env_pin command accepts from a caller
 * (kaboom-x0li.2).
 *
 * This parser decides what reaches CDP. A value of the wrong type coerced to NaN is sent to
 * Chrome, refused, and then reported as a knob the browser would not pin — blaming the browser
 * for a caller's typo, in an artifact a human reads to decide whether a test is trustworthy.
 */

import { test, describe, before } from 'node:test'
import assert from 'node:assert'

const COMMAND = '../../../extension/background/environment-transaction/env-pin.js'

describe('parseEnvironmentPinSpec', () => {
  let parseEnvironmentPinSpec

  before(async () => {
    ;({ parseEnvironmentPinSpec } = await import(COMMAND))
  })

  test('reads the knobs out of the environment object', () => {
    const spec = parseEnvironmentPinSpec({
      environment: {
        clock_epoch_ms: 1788480000000,
        timezone_id: 'America/New_York',
        latitude: 37.7749,
        longitude: -122.4194,
        viewport_width: 1280,
        viewport_height: 720,
        random_seed: 'run-1'
      }
    })
    assert.equal(spec.clock_epoch_ms, 1788480000000)
    assert.equal(spec.timezone_id, 'America/New_York')
    assert.equal(spec.viewport_width, 1280)
    assert.equal(spec.random_seed, 'run-1')
  })

  test('drops a numeric knob given as a string rather than coercing it to NaN', () => {
    const spec = parseEnvironmentPinSpec({ environment: { viewport_width: 'wide', viewport_height: 720 } })
    assert.equal(spec.viewport_width, undefined)
    assert.equal(spec.viewport_height, 720)
  })

  test('drops a non-finite number: Infinity is not a viewport', () => {
    const spec = parseEnvironmentPinSpec({ environment: { device_scale_factor: Infinity } })
    assert.equal(spec.device_scale_factor, undefined)
  })

  test('rejects a virtual-time policy Chrome does not define', () => {
    // An unknown policy string sent to CDP fails the whole clock pin, so a typo would cost
    // the timezone as well as the clock.
    const spec = parseEnvironmentPinSpec({ environment: { virtual_time_policy: 'freeze' } })
    assert.equal(spec.virtual_time_policy, undefined)
    assert.equal(parseEnvironmentPinSpec({ environment: { virtual_time_policy: 'pause' } }).virtual_time_policy, 'pause')
  })

  test('drops an empty seed: seeding on "" is not a pin anyone asked for', () => {
    assert.equal(parseEnvironmentPinSpec({ environment: { random_seed: '' } }).random_seed, undefined)
  })

  test('an empty spec stays empty, so the command can refuse rather than silently no-op', () => {
    assert.deepEqual(parseEnvironmentPinSpec({}), {})
    assert.deepEqual(parseEnvironmentPinSpec({ environment: {} }), {})
    assert.deepEqual(parseEnvironmentPinSpec({ environment: null }), {})
  })

  test('mobile is only set when explicitly true', () => {
    assert.equal(parseEnvironmentPinSpec({ environment: { mobile: 'yes' } }).mobile, undefined)
    assert.equal(parseEnvironmentPinSpec({ environment: { mobile: true } }).mobile, true)
  })
})
