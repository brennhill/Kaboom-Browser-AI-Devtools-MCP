// browser-state-driver.test.js — Verifies deterministic QA fixture browser mutations and restoration.

import assert from 'node:assert/strict'
import { test } from 'node:test'

import {
  createEnvironmentStateDriver,
  unsupportedEnvironmentCapabilities
} from '../../../extension/background/environment-transaction/browser-state-driver.js'

test('unsupported capabilities are reported before snapshot or mutation', async () => {
  const calls = []
  const driver = createEnvironmentStateDriver(fakeDeps(calls))
  const fixture = { version: 1, locale: 'de-DE', network: { profile: 'offline' } }
  assert.deepEqual(unsupportedEnvironmentCapabilities(fixture), ['locale', 'network'])
  await assert.rejects(driver.snapshot(7, fixture), /unsupported_fixture_capabilities/)
  assert.deepEqual(calls, [])
})

test('snapshot captures exact change-coupled state before mutation', async () => {
  const calls = []
  const driver = createEnvironmentStateDriver(fakeDeps(calls))
  const fixture = {
    version: 1,
    target: { url: 'https://example.test/checkout' },
    viewport: { width: 1280, height: 720 },
    cookies: [{ name: 'session', value: 'new-private-value' }],
    local_storage: { token: 'new-private-token' },
    session_storage: { journey: 'checkout' }
  }
  const snapshot = await driver.snapshot(7, fixture)
  assert.deepEqual(calls, ['get_tab', 'get_window', 'capture_page_state', 'get_cookie:session'])
  assert.equal(snapshot.tab_url, 'https://example.test/original')
  assert.equal(snapshot.page_state.local_storage.token, 'old-private-token')
  assert.equal(snapshot.cookies[0].value, 'old-private-value')
})

test('apply uses bounded navigation before page-scoped mutations', async () => {
  const calls = []
  const driver = createEnvironmentStateDriver(fakeDeps(calls))
  const fixture = {
    version: 1,
    target: { url: 'https://example.test/checkout' },
    viewport: { width: 1280, height: 720 },
    cookies: [{ name: 'session', value: 'new-private-value' }],
    local_storage: { flag: 'on' },
    feature_flags: { new_checkout: true },
    seed_data: { cart: { items: 2 } }
  }
  const counts = await driver.apply(7, fixture)
  assert.deepEqual(calls, [
    'navigate:https://example.test/checkout',
    'get_tab',
    'resize:1280x720',
    'set_cookie:session',
    'apply_page_state'
  ])
  assert.deepEqual(counts, {
    cookies: 1,
    local_storage: 1,
    session_storage: 0,
    feature_flags: 1,
    seed_data: 1
  })
})

test('empty viewport is a no-op', async () => {
  const calls = []
  const driver = createEnvironmentStateDriver(fakeDeps(calls))
  await driver.apply(7, { version: 1, viewport: {} })
  assert.deepEqual(calls, [])
})

test('cross-origin page state is rejected before mutation', async () => {
  const calls = []
  const driver = createEnvironmentStateDriver(fakeDeps(calls))
  await assert.rejects(
    driver.snapshot(7, {
      version: 1,
      target: { url: 'https://other.test/' },
      local_storage: { private: 'value' }
    }),
    /cross_origin_page_state_unsupported/
  )
  assert.deepEqual(calls, ['get_tab'])
})

test('restore reverses page, cookie, viewport, and navigation state', async () => {
  const calls = []
  const deps = fakeDeps(calls)
  const driver = createEnvironmentStateDriver(deps)
  const fixture = {
    version: 1,
    target: { url: 'https://example.test/checkout' },
    viewport: { width: 1280, height: 720 },
    cookies: [{ name: 'session', value: 'new-private-value' }],
    local_storage: { token: 'new-private-token' }
  }
  const snapshot = await driver.snapshot(7, fixture)
  calls.length = 0
  await driver.restore(7, fixture, snapshot)
  assert.deepEqual(calls, [
    'navigate:https://example.test/original',
    'resize:1440x900',
    'remove_cookie:session',
    'set_cookie:session',
    'restore_page_state'
  ])
})

test('restore attempts every independent recovery step after a failure', async () => {
  const calls = []
  const deps = fakeDeps(calls)
  deps.navigate = async (_tabId, url) => {
    calls.push(`navigate:${url}`)
    throw new Error('private navigation detail')
  }
  const driver = createEnvironmentStateDriver(deps)
  const fixture = {
    version: 1,
    target: { url: 'https://example.test/checkout' },
    viewport: { width: 1280, height: 720 },
    cookies: [{ name: 'session', value: 'new-private-value' }],
    local_storage: { token: 'new-private-token' }
  }
  const snapshot = {
    tab_url: 'https://example.test/original',
    window_id: 2,
    window_bounds: { width: 1440, height: 900 },
    page_state: {
      local_storage: { token: 'old-private-token' },
      session_storage: {},
      feature_flags: {},
      seed_data: {}
    },
    cookies: [{ name: 'session', value: 'old-private-value' }]
  }
  await assert.rejects(driver.restore(7, fixture, snapshot), (error) => {
    assert.equal(error.message, 'fixture_restore_failed')
    return true
  })
  assert.deepEqual(calls, [
    'navigate:https://example.test/original',
    'resize:1440x900',
    'remove_cookie:session',
    'set_cookie:session',
    'restore_page_state'
  ])
})

function fakeDeps(calls) {
  return {
    async getTab() {
      calls.push('get_tab')
      return { url: 'https://example.test/original', windowId: 2 }
    },
    async getWindow() {
      calls.push('get_window')
      return { width: 1440, height: 900 }
    },
    async capturePageState() {
      calls.push('capture_page_state')
      return {
        local_storage: { token: 'old-private-token' },
        session_storage: { journey: 'old' },
        feature_flags: { new_checkout: 'false' },
        seed_data: { cart: '{"items":1}' }
      }
    },
    async getCookie(_url, name) {
      calls.push(`get_cookie:${name}`)
      return { name, value: 'old-private-value', path: '/' }
    },
    async navigate(_tabId, url) {
      calls.push(`navigate:${url}`)
    },
    async resizeViewport(_tabId, _windowId, width, height) {
      calls.push(`resize:${width}x${height}`)
    },
    async restoreWindow(_windowId, width, height) {
      calls.push(`resize:${width}x${height}`)
    },
    async setCookie(cookie) {
      calls.push(`set_cookie:${cookie.name}`)
    },
    async removeCookie(_url, name) {
      calls.push(`remove_cookie:${name}`)
    },
    async applyPageState() {
      calls.push('apply_page_state')
    },
    async restorePageState() {
      calls.push('restore_page_state')
    }
  }
}
