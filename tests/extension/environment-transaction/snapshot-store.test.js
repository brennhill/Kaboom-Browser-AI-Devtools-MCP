// snapshot-store.test.js — Verifies durable, bounded, redacted environment snapshot persistence.

import assert from 'node:assert/strict'
import test from 'node:test'

import { createPersistentEnvironmentSnapshotStore } from '../../../extension/background/environment-transaction/snapshot-store.js'

test('persistent snapshot store survives reconstruction and evicts the oldest record', async () => {
  const storage = memoryStorage()
  let now = 1
  let id = 0
  const deps = { storage, limit: 2, now: () => now++, newID: () => `opaque_${++id}`, onNotice: () => {} }
  const first = createPersistentEnvironmentSnapshotStore(deps)
  await first.save(snapshot('one'))
  await first.save(snapshot('two'))
  const newestID = await first.save(snapshot('three'))

  const reconstructed = createPersistentEnvironmentSnapshotStore(deps)
  assert.equal(await reconstructed.get('opaque_1'), undefined)
  assert.equal((await reconstructed.get('opaque_2')).tab_url, 'https://two.test/')
  assert.equal((await reconstructed.get(newestID)).tab_url, 'https://three.test/')
})

test('persistent snapshot store clears corrupt state and emits a stable notice', async () => {
  const notices = []
  const storage = memoryStorage({ environment_transaction_snapshots_v1: { version: 1, records: 'private-secret' } })
  const store = createPersistentEnvironmentSnapshotStore({
    storage,
    limit: 2,
    now: () => 1,
    newID: () => 'opaque_1',
    onNotice: (notice) => notices.push(notice)
  })

  assert.equal(await store.get('opaque_1'), undefined)
  assert.deepEqual(notices, ['environment_snapshot_store_corrupt'])
  assert.equal(JSON.stringify(notices).includes('private-secret'), false)
  assert.equal(storage.values.has('environment_transaction_snapshots_v1'), false)
})

test('persistent snapshot store reports storage failures without leaking values', async () => {
  const notices = []
  const storage = memoryStorage()
  storage.set = async () => {
    throw new Error('private-cookie-value')
  }
  const store = createPersistentEnvironmentSnapshotStore({
    storage,
    limit: 2,
    now: () => 1,
    newID: () => 'opaque_1',
    onNotice: (notice) => notices.push(notice)
  })

  await assert.rejects(store.save(snapshot('private')), { message: 'environment_snapshot_store_write_failed' })
  assert.deepEqual(notices, ['environment_snapshot_store_write_failed'])
  assert.equal(JSON.stringify(notices).includes('private'), false)
})

function snapshot(name) {
  return {
    tab_url: `https://${name}.test/`,
    window_id: 1,
    page_state: { local_storage: {}, session_storage: {}, feature_flags: {}, seed_data: {} },
    cookies: []
  }
}

function memoryStorage(initial = {}) {
  const values = new Map(Object.entries(initial))
  return {
    values,
    async get(key) {
      return { [key]: values.get(key) }
    },
    async set(items) {
      for (const [key, value] of Object.entries(items)) values.set(key, value)
    },
    async remove(key) {
      values.delete(key)
    }
  }
}
