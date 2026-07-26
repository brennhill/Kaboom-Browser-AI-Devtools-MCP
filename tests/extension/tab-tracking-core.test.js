// @ts-nocheck
/**
 * @fileoverview tab-tracking-core.test.js — Regression tests for the shared
 * tab-tracking core (F2). The context-menu "Control Tab" path used to persist
 * tracking directly, skipping the internal-page and cloaked-domain guards the
 * popup enforced — controlling a cloaked domain was a privacy leak. Both entry
 * points now go through this core, so the guards live in exactly one place.
 */

import { beforeEach, describe, test, mock } from 'node:test'
import assert from 'node:assert'

let storageState

function buildChrome() {
  storageState = {}
  return {
    runtime: { id: 'test-ext', lastError: null },
    storage: {
      local: {
        get: mock.fn((keys, cb) => {
          const out = {}
          const list = typeof keys === 'string' ? [keys] : keys
          for (const k of list) if (storageState[k] !== undefined) out[k] = storageState[k]
          if (typeof cb === 'function') { cb(out); return }
          return Promise.resolve(out)
        }),
        set: mock.fn((data, cb) => {
          Object.assign(storageState, data)
          if (typeof cb === 'function') { cb(); return }
          return Promise.resolve()
        }),
        remove: mock.fn((keys, cb) => {
          const list = typeof keys === 'string' ? [keys] : keys
          for (const k of list) delete storageState[k]
          if (typeof cb === 'function') { cb(); return }
          return Promise.resolve()
        })
      },
      onChanged: { addListener: mock.fn() }
    },
    tabs: {
      sendMessage: mock.fn((tabId, msg, cb) => {
        if (typeof cb === 'function') { cb({ status: 'alive' }); return }
        return Promise.resolve({ status: 'alive' })
      }),
      reload: mock.fn(() => Promise.resolve())
    }
  }
}

const { trackTab, untrackTab } = await import('../../extension/lib/tabs/tab-tracking-core.js')

describe('shared tab-tracking core (F2 guards)', () => {
  beforeEach(() => {
    globalThis.chrome = buildChrome()
  })

  test('trackTab refuses a cloaked domain and persists nothing (privacy guard)', async () => {
    const outcome = await trackTab({ id: 9, url: 'https://dash.cloudflare.com/login', title: 'CF' })
    assert.strictEqual(outcome, 'cloaked')
    assert.strictEqual(
      globalThis.chrome.storage.local.set.mock.calls.length,
      0,
      'a cloaked tab must never be persisted as tracked'
    )
  })

  test('trackTab refuses an internal browser page', async () => {
    const outcome = await trackTab({ id: 9, url: 'chrome://settings', title: 'Settings' })
    assert.strictEqual(outcome, 'internal_page')
    assert.strictEqual(globalThis.chrome.storage.local.set.mock.calls.length, 0)
  })

  test('trackTab persists a normal tab and pings the content script', async () => {
    const outcome = await trackTab({ id: 9, url: 'https://example.com/app', title: 'App' })
    assert.strictEqual(outcome, 'tracked')
    assert.ok(globalThis.chrome.storage.local.set.mock.calls.length >= 1, 'tracked tab must be persisted')
    const pinged = globalThis.chrome.tabs.sendMessage.mock.calls.some((c) => c.arguments[1]?.type === 'kaboom_ping')
    assert.ok(pinged, 'should ping the content script to ensure it is injected')
  })

  test('untrackTab clears storage, runs the stop callback, and notifies the content script', async () => {
    storageState = { trackedTabId: 9, trackedTabUrl: 'https://example.com', trackedTabTitle: 'App' }
    let stopped = false
    await untrackTab(9, () => { stopped = true })
    assert.ok(stopped, 'the context-specific stop callback must run')
    assert.strictEqual(storageState.trackedTabId, undefined, 'all tracked keys cleared')
    const notified = globalThis.chrome.tabs.sendMessage.mock.calls.some(
      (c) => c.arguments[1]?.type === 'tracking_state_changed' && c.arguments[1]?.state?.isTracked === false
    )
    assert.ok(notified, 'content script must be notified of the untrack')
  })
})
