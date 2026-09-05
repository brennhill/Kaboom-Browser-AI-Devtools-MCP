// @ts-nocheck
/**
 * @fileoverview early-patch-seeded-random.test.js — The seeded generator early-patch installs
 * for a pinned session (kaboom-x0li.2).
 *
 * A recorded session that calls Math.random cannot be replayed: the ids, the shuffles and the
 * sampled arms differ on every run, so a generated test asserts on values that never come back.
 * These run the shipped bundle, not the source, because the bundle is what the browser loads.
 */

import { test, describe } from 'node:test'
import assert from 'node:assert'
import { readFileSync } from 'node:fs'
import vm from 'node:vm'

const BUNDLE = readFileSync('extension/early-patch.bundled.js', 'utf8')

/**
 * Run the early-patch IIFE against a fake page.
 *
 * `presetSeed` covers the ordering the extension cannot control: the CDP snippet and this file
 * both run before page scripts, and either may land first.
 */
function runEarlyPatch({ presetSeed, writableRandom = true } = {}) {
  const fakeCrypto = { getRandomValues: (array) => array }
  const fakeMath = Object.create(Math)
  if (!writableRandom) {
    Object.defineProperty(fakeMath, 'random', { value: Math.random, writable: false, configurable: false })
  } else {
    fakeMath.random = Math.random
  }

  const fakeWindow = { crypto: fakeCrypto }
  if (presetSeed !== undefined) fakeWindow.__KABOOM_RANDOM_SEED__ = presetSeed

  const sandbox = {
    window: fakeWindow,
    location: { hostname: 'app.example.com' },
    Math: fakeMath,
    Uint8Array,
    ArrayBuffer,
    Number,
    String,
    Object,
    Date: { now: () => 1000 },
    setTimeout: () => 1,
    clearTimeout: () => undefined
  }
  sandbox.globalThis = sandbox
  vm.runInNewContext(BUNDLE, sandbox)
  return { window: fakeWindow, math: fakeMath, crypto: fakeCrypto }
}

function draw(math, count) {
  return Array.from({ length: count }, () => math.random())
}

describe('seeded randomness', () => {
  test('the installer is published but installs nothing until a seed arrives', () => {
    const { window, math } = runEarlyPatch()
    assert.equal(typeof window.__KABOOM_SEED_RANDOM__, 'function')
    // Opt-in: an unpinned page must keep the browser's own generator.
    assert.equal(window.__KABOOM_RANDOM_SEED_ACTIVE__, undefined)
    assert.equal(math.random, Math.random)
  })

  test('the same seed draws the same sequence in two independent pages', () => {
    const first = runEarlyPatch()
    const second = runEarlyPatch()
    first.window.__KABOOM_SEED_RANDOM__('run-1')
    second.window.__KABOOM_SEED_RANDOM__('run-1')
    assert.deepEqual(draw(first.math, 8), draw(second.math, 8))
  })

  test('different seeds draw different sequences', () => {
    const first = runEarlyPatch()
    const second = runEarlyPatch()
    first.window.__KABOOM_SEED_RANDOM__('run-1')
    second.window.__KABOOM_SEED_RANDOM__('run-2')
    assert.notDeepEqual(draw(first.math, 8), draw(second.math, 8))
  })

  test('every drawn value stays inside [0, 1)', () => {
    const { window, math } = runEarlyPatch()
    window.__KABOOM_SEED_RANDOM__('run-1')
    for (const value of draw(math, 500)) {
      assert.ok(value >= 0 && value < 1, `drew ${value}, which Math.random can never return`)
    }
  })

  test('a zero hash lane cannot degenerate the generator to all zeros', () => {
    // xorshift128 with a zero lane emits zeros forever, so a "seeded" run would be silently
    // degenerate: every id identical, every shuffle a no-op.
    const { window, math } = runEarlyPatch()
    window.__KABOOM_SEED_RANDOM__('')
    const values = draw(math, 32)
    assert.ok(
      values.some((value) => value !== 0),
      'the generator returned only zeros'
    )
  })

  test('crypto.getRandomValues is seeded too, and returns the array it was given', () => {
    const first = runEarlyPatch()
    const second = runEarlyPatch()
    first.window.__KABOOM_SEED_RANDOM__('run-1')
    second.window.__KABOOM_SEED_RANDOM__('run-1')

    const a = new Uint8Array(16)
    const b = new Uint8Array(16)
    assert.equal(first.crypto.getRandomValues(a), a, 'must return the same array object')
    second.crypto.getRandomValues(b)
    assert.deepEqual(Array.from(a), Array.from(b))
    assert.ok(
      Array.from(a).some((byte) => byte !== 0),
      'a buffer of zeros is not randomness, seeded or otherwise'
    )
  })

  test('a seed set before early-patch loads is applied on load', () => {
    // The CDP snippet and early-patch have no guaranteed order, so both orders must work or
    // pinning would land on some page loads and not others.
    const { window, math } = runEarlyPatch({ presetSeed: 'run-1' })
    assert.equal(window.__KABOOM_RANDOM_SEED_ACTIVE__, true)
    const preset = draw(math, 4)

    const later = runEarlyPatch()
    later.window.__KABOOM_SEED_RANDOM__('run-1')
    assert.deepEqual(preset, draw(later.math, 4))
  })

  test('re-seeding with the same seed is a no-op rather than a restart', () => {
    const { window, math } = runEarlyPatch()
    window.__KABOOM_SEED_RANDOM__('run-1')
    const first = draw(math, 4)
    window.__KABOOM_SEED_RANDOM__('run-1')
    const second = draw(math, 4)
    assert.notDeepEqual(first, second, 'an idempotent install must not rewind the stream')
  })

  test('a page that refuses to let Math.random be replaced reports the seed as inactive', () => {
    // Reporting a seed that never took effect would put a determinism claim in the emitted
    // test that the recording never honoured.
    const { window } = runEarlyPatch({ writableRandom: false })
    assert.equal(window.__KABOOM_SEED_RANDOM__('run-1'), false)
    assert.equal(window.__KABOOM_RANDOM_SEED_ACTIVE__, false)
  })
})
