// @ts-nocheck
/**
 * @fileoverview early-patch-hardened-restore.test.js — The 30-second self-cleanup
 * must not throw on hardened pages.
 *
 * early-patch installs its shims through safeAssignGlobal because some pages make
 * `fetch`/`WebSocket` non-writable. The *restore* path (run when Phase 2 never
 * adopts the shims) has to be just as careful: a bare `window.WebSocket = …`
 * throws on a non-configurable read-only global, and the throw — inside the
 * cleanup timer — aborts the rest of the cleanup and leaks the WS buffer. This
 * exercises the bundled IIFE against exactly that page shape.
 */

import { test, describe } from 'node:test'
import assert from 'node:assert'
import { readFileSync } from 'node:fs'
import vm from 'node:vm'

const BUNDLE = readFileSync('extension/early-patch.bundled.js', 'utf8')

/**
 * Run the early-patch IIFE against a fake page whose WebSocket is read-only and
 * (optionally) non-configurable — the frozen-globals shape safeAssignGlobal
 * exists for. Returns the fake window plus the captured self-cleanup callback.
 *
 * Only one timer is scheduled during import (the self-cleanup); the fetch
 * body-read timer arms only when a fetch is actually made, and none is here.
 */
function runEarlyPatchOnHardenedPage({ configurable }) {
  const timers = []
  const fakeWindow = {}
  Object.defineProperty(fakeWindow, 'WebSocket', {
    value: function OriginalWebSocket() {},
    writable: false,
    configurable
  })
  fakeWindow.fetch = function fetch() {
    return { then: () => ({ catch: () => undefined }) }
  }

  function FakeXHR() {}
  FakeXHR.prototype.open = function () {}
  FakeXHR.prototype.send = function () {}

  const sandbox = {
    window: fakeWindow,
    location: { hostname: 'hardened.example.com' },
    XMLHttpRequest: FakeXHR,
    Date: { now: () => 1000 },
    setTimeout: (fn) => {
      timers.push(fn)
      return timers.length
    },
    clearTimeout: () => undefined
  }
  sandbox.globalThis = sandbox
  vm.runInNewContext(BUNDLE, sandbox)

  return { fakeWindow, cleanup: timers[0] }
}

describe('early-patch self-cleanup on hardened pages', () => {
  test('restoring a non-configurable read-only WebSocket does not throw', () => {
    const { cleanup } = runEarlyPatchOnHardenedPage({ configurable: false })
    assert.strictEqual(typeof cleanup, 'function', 'the 30s self-cleanup must be scheduled')

    // Before the fix this restored WebSocket with a plain assignment, which throws
    // on a non-configurable read-only global and aborts the rest of the cleanup.
    assert.doesNotThrow(() => cleanup(), 'the cleanup timer must not throw on a frozen WebSocket global')
  })

  test('the WebSocket early buffer is freed on a page that froze the global', () => {
    // Whether the shim is abandoned at install (it could not be assigned) or torn
    // down by the cleanup timer, the buffer must not leak. Before the fix the
    // cleanup threw at the plain assignment before it reached the `delete`.
    const { fakeWindow, cleanup } = runEarlyPatchOnHardenedPage({ configurable: false })

    cleanup()

    assert.ok(!('__KABOOM_EARLY_WS__' in fakeWindow),
      'the WebSocket buffer must not leak on a page that froze the global')
  })

  test('a configurable read-only WebSocket still round-trips cleanly', () => {
    const { fakeWindow, cleanup } = runEarlyPatchOnHardenedPage({ configurable: true })
    assert.doesNotThrow(() => cleanup())
    assert.ok(!('__KABOOM_EARLY_WS__' in fakeWindow))
  })
})
