// chrome-state-adapter.test.js — Verifies exact browser storage capture and restoration.

import assert from 'node:assert/strict'
import { afterEach, test } from 'node:test'

import { chromeDriverDeps } from '../../../extension/background/environment-transaction/chrome-state-adapter.js'
import {
  applyEnvironment,
  restoreEnvironment,
  snapshotEnvironment
} from '../../../extension/background/environment-transaction/commands.js'

const originalChrome = globalThis.chrome
const originalLocalStorage = globalThis.localStorage
const originalSessionStorage = globalThis.sessionStorage

afterEach(() => {
  globalThis.chrome = originalChrome
  globalThis.localStorage = originalLocalStorage
  globalThis.sessionStorage = originalSessionStorage
})

test('adapter snapshots absent keys and restores exact prior raw values', async () => {
  const local = storage({ token: 'old-private-token', flag: 'not-json' })
  const session = storage({ journey: 'old' })
  globalThis.localStorage = local
  globalThis.sessionStorage = session
  globalThis.chrome = {
    scripting: {
      async executeScript({ func, args }) {
        return [{ result: func(...args) }]
      }
    }
  }

  const deps = chromeDriverDeps()
  const fixture = {
    version: 1,
    local_storage: { token: 'new-private-token', added: 'new' },
    session_storage: { journey: 'checkout' },
    feature_flags: { flag: true },
    seed_data: { cart: { items: 2 } }
  }
  const snapshot = await deps.capturePageState(7, fixture)
  assert.deepEqual(snapshot, {
    local_storage: { token: 'old-private-token', added: null },
    session_storage: { journey: 'old' },
    feature_flags: { flag: 'not-json' },
    seed_data: { cart: null }
  })

  await deps.applyPageState(7, fixture)
  assert.equal(local.getItem('token'), 'new-private-token')
  assert.equal(local.getItem('flag'), 'true')
  assert.equal(local.getItem('cart'), '{"items":2}')

  await deps.restorePageState(7, snapshot)
  assert.equal(local.getItem('token'), 'old-private-token')
  assert.equal(local.getItem('added'), null)
  assert.equal(local.getItem('flag'), 'not-json')
  assert.equal(local.getItem('cart'), null)
  assert.equal(session.getItem('journey'), 'old')
})

test('snapshot command exposes only generated opaque identifiers', async () => {
  const store = snapshotStore('fixture_snapshot_1')
  const snapshot = {
    tab_url: 'https://example.test/',
    window_id: 2,
    page_state: {
      local_storage: { token: 'private-token' },
      session_storage: {},
      feature_flags: {},
      seed_data: {}
    },
    cookies: [{ name: 'session', value: 'private-cookie' }]
  }
  const id = await store.save(snapshot)
  assert.equal(id, 'fixture_snapshot_1')
  assert.equal(JSON.stringify({ success: true, snapshot_id: id }).includes('private'), false)
  assert.equal(await store.get(id), snapshot)
  await store.delete(id)
  assert.equal(await store.get(id), undefined)
})

test('command boundary replaces private driver failures with stable errors and retains failed restores', async () => {
  const privateFailure = new Error('secret-cookie-value')
  const driver = {
    snapshot: async () => {
      throw privateFailure
    },
    apply: async () => {
      throw privateFailure
    },
    restore: async () => {
      throw privateFailure
    }
  }
  const store = snapshotStore('fixture_snapshot_1')
  const fixture = { version: 1 }
  const snapshot = {
    tab_url: 'https://example.test/',
    window_id: 2,
    page_state: { local_storage: {}, session_storage: {}, feature_flags: {}, seed_data: {} },
    cookies: []
  }
  await store.save(snapshot)

  await assert.rejects(snapshotEnvironment(driver, store, 7, fixture), { message: 'fixture_snapshot_failed' })
  await assert.rejects(applyEnvironment(driver, 7, fixture), { message: 'fixture_apply_failed' })
  await assert.rejects(restoreEnvironment(driver, store, 7, 'fixture_snapshot_1'), {
    message: 'fixture_restore_failed'
  })
  assert.equal(await store.get('fixture_snapshot_1'), snapshot)
})

test('restore is idempotent after the private snapshot was already consumed', async () => {
  const store = snapshotStore('fixture_snapshot_1')
  const driver = {
    snapshot: async () => {
      throw new Error('unexpected')
    },
    apply: async () => ({}),
    restore: async () => {
      throw new Error('unexpected')
    }
  }

  assert.deepEqual(await restoreEnvironment(driver, store, 7, 'missing_snapshot'), {
    success: true,
    restored: true,
    already_restored: true
  })
})

function snapshotStore(id) {
  const snapshots = new Map()
  return {
    async save(snapshot) {
      snapshots.set(id, snapshot)
      return id
    },
    async get(snapshotID) {
      return snapshots.get(snapshotID)
    },
    async delete(snapshotID) {
      snapshots.delete(snapshotID)
    }
  }
}

function storage(initial) {
  const values = new Map(Object.entries(initial))
  return {
    getItem(key) {
      return values.has(key) ? values.get(key) : null
    },
    setItem(key, value) {
      values.set(key, String(value))
    },
    removeItem(key) {
      values.delete(key)
    }
  }
}
