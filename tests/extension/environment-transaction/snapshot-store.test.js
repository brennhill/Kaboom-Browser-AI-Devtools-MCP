// snapshot-store.test.js — Verifies durable, bounded, redacted environment snapshot persistence.

import assert from 'node:assert/strict'
import test from 'node:test'

import { createPersistentEnvironmentSnapshotStore } from '../../../extension/background/environment-transaction/snapshot-store.js'
import { createStorageFaultScenario } from '../state-recovery/storage-fault-fixture.js'

test('persistent snapshot store survives reconstruction and never evicts an active snapshot', async () => {
  const storage = memoryStorage()
  let now = 1
  let id = 0
  const deps = { storage, limit: 2, now: () => now++, newID: () => `opaque_${++id}`, onNotice: () => {} }
  const first = createPersistentEnvironmentSnapshotStore(deps)
  await first.save(snapshot('one'))
  await first.save(snapshot('two'))
  await assert.rejects(first.save(snapshot('three')), { message: 'environment_snapshot_store_full' })

  const reconstructed = createPersistentEnvironmentSnapshotStore(deps)
  assert.equal((await reconstructed.lookup('opaque_1')).snapshot.tab_url, 'https://one.test/')
  assert.equal((await reconstructed.lookup('opaque_2')).snapshot.tab_url, 'https://two.test/')
  assert.deepEqual(await reconstructed.lookup('opaque_3'), { status: 'missing' })
})

test('persistent snapshot store distinguishes bounded consumed tombstones from unknown identifiers', async () => {
  const storage = memoryStorage()
  let id = 0
  const deps = { storage, limit: 2, now: () => 7, newID: () => `opaque_${++id}`, onNotice: () => {} }
  const store = createPersistentEnvironmentSnapshotStore(deps)
  await store.save(snapshot('one'))
  await store.consume('opaque_1')
  assert.deepEqual(await createPersistentEnvironmentSnapshotStore(deps).lookup('opaque_1'), { status: 'consumed' })

  await store.save(snapshot('two'))
  await store.consume('opaque_2')
  await store.save(snapshot('three'))
  await store.consume('opaque_3')

  const reconstructed = createPersistentEnvironmentSnapshotStore(deps)
  assert.deepEqual(await reconstructed.lookup('opaque_1'), { status: 'missing' })
  assert.deepEqual(await reconstructed.lookup('opaque_2'), { status: 'consumed' })
  assert.deepEqual(await reconstructed.lookup('opaque_3'), { status: 'consumed' })
  assert.equal(JSON.stringify(storage.values.get('environment_transaction_snapshots_v1')).includes('one.test'), false)
})

test('consume rejects an unknown snapshot without creating a false recovery tombstone', async () => {
  const store = createPersistentEnvironmentSnapshotStore({
    storage: memoryStorage(),
    limit: 2,
    now: () => 1,
    newID: () => 'opaque_1',
    onNotice: () => {}
  })

  await assert.rejects(store.consume('unknown'), { message: 'environment_snapshot_store_consume_missing' })
  assert.deepEqual(await store.lookup('unknown'), { status: 'missing' })
})

test('startup reconciliation prunes only snapshots absent from the durable daemon registry', async () => {
  const storage = memoryStorage()
  let id = 0
  const deps = { storage, limit: 3, now: () => 7, newID: () => `opaque_${++id}`, onNotice: () => {} }
  const store = createPersistentEnvironmentSnapshotStore(deps)
  await store.save(snapshot('orphan'))
  await store.save(snapshot('owned'))

  assert.deepEqual(await store.reconcile(['opaque_2']), { pruned: 1, retained: 1 })
  assert.deepEqual(await store.lookup('opaque_1'), { status: 'missing' })
  assert.equal((await store.lookup('opaque_2')).snapshot.tab_url, 'https://owned.test/')
  assert.doesNotMatch(JSON.stringify(await storage.get('environment_transaction_snapshots_v1')), /orphan\.test/)
})

test('snapshot mutations serialize so concurrent saves cannot overwrite recovery obligations', async () => {
  const storage = memoryStorage()
  let releaseFirstSet
  let markFirstSetEntered
  const firstSetEntered = new Promise((resolve) => {
    markFirstSetEntered = resolve
  })
  const firstSetReleased = new Promise((resolve) => {
    releaseFirstSet = resolve
  })
  const write = storage.set
  let first = true
  storage.set = async (items) => {
    if (first) {
      first = false
      markFirstSetEntered()
      await firstSetReleased
    }
    await write(items)
  }
  let id = 0
  const deps = { storage, limit: 3, now: () => 7, newID: () => `opaque_${++id}`, onNotice: () => {} }
  const store = createPersistentEnvironmentSnapshotStore(deps)

  const one = store.save(snapshot('one'))
  await firstSetEntered
  const two = store.save(snapshot('two'))
  releaseFirstSet()
  await Promise.all([one, two])

  const reconstructed = createPersistentEnvironmentSnapshotStore(deps)
  assert.equal((await reconstructed.lookup('opaque_1')).status, 'active')
  assert.equal((await reconstructed.lookup('opaque_2')).status, 'active')
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

  assert.deepEqual(await store.lookup('opaque_1'), { status: 'missing' })
  assert.deepEqual(notices, [notice('environment_snapshot_store_corrupt', 'corruption')])
  assert.equal(JSON.stringify(notices).includes('private-secret'), false)
  assert.equal(storage.values.has('environment_transaction_snapshots_v1'), false)
})

test('persistent snapshot store reports storage failures without leaking values', async () => {
  const notices = []
  const storage = memoryStorage()
  const write = storage.set
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
  assert.deepEqual(notices, [notice('environment_snapshot_store_write_failed', 'write')])
  assert.equal(JSON.stringify(notices).includes('private'), false)

  storage.set = write
  assert.equal(await store.save(snapshot('recovered')), 'opaque_1')
  assert.equal((await store.lookup('opaque_1')).status, 'active')
})

test('snapshot store classifies quota and cancellation without retaining private values', async () => {
  for (const kind of ['quota', 'cancellation']) {
    const notices = []
    const scenario = createStorageFaultScenario(kind, 'private-environment-state')
    const storage = memoryStorage()
    storage.set = async () => {
      throw scenario.error
    }
    const store = createPersistentEnvironmentSnapshotStore({
      storage,
      limit: 2,
      now: () => 1,
      newID: () => 'opaque_1',
      onNotice: (entry) => notices.push(entry)
    })

    await assert.rejects(store.save(snapshot('private')))
    assert.deepEqual(notices, [notice('environment_snapshot_store_write_failed', kind)])
    assert.doesNotMatch(JSON.stringify(notices), /private-environment-state/)
  }
})

function notice(code, fault_kind) {
  return { code, fault_kind, lifecycle: 'active' }
}

function snapshot(name) {
  return {
    tab_url: `https://${name}.test/`,
    window_id: 1,
    page_state: { local_storage: {}, session_storage: {}, feature_flags: {}, seed_data: {} },
    cookies: [],
    restore_plan: {
      mutated_url: `https://${name}.test/`,
      setup_timeout_ms: 10_000,
      cookie_names: [],
      page_state_touched: false,
      navigation_changed: false
    }
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
