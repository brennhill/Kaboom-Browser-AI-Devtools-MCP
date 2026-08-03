// storage-fault-contract.test.js — Guards canonical extension persistence fault fixtures.
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { test } from 'node:test'

import { createStorageFaultScenario, STORAGE_FAULT_KINDS } from './storage-fault-fixture.js'

function source(path) {
  return readFileSync(new URL(`../../../${path}`, import.meta.url), 'utf8')
}

test('extension and daemon share one persistence fault vocabulary', () => {
  const daemon = source('internal/statefault/fault.go')
  const extension = source('src/lib/storage/fault.ts')

  assert.deepEqual(STORAGE_FAULT_KINDS, [
    'read',
    'write',
    'sync',
    'rename',
    'directory_sync',
    'quota',
    'corruption',
    'partial_write',
    'cancellation',
    'restart'
  ])
  for (const kind of STORAGE_FAULT_KINDS) {
    assert.match(daemon, new RegExp(`"${kind}"`))
    assert.match(extension, new RegExp(`'${kind}'`))
  }
})

test('test-only scenarios are deterministic and redact private sentinels', () => {
  const sentinel = 'private-user-state'
  const valid = { enabled: true, label: sentinel }

  for (const kind of STORAGE_FAULT_KINDS) {
    const scenario = createStorageFaultScenario(kind, sentinel)
    assert.equal(scenario.kind, kind)
    assert.doesNotMatch(scenario.error.message, new RegExp(sentinel))
  }
  assert.deepEqual(createStorageFaultScenario('read', sentinel).storedValue(valid), valid)
  assert.notEqual(createStorageFaultScenario('read', sentinel).storedValue(valid), valid)
  assert.equal(createStorageFaultScenario('corruption', sentinel).storedValue(valid), '{"schema_version":')
  assert.deepEqual(createStorageFaultScenario('partial_write', sentinel).storedValue(valid), { enabled: true })
  assert.equal(createStorageFaultScenario('cancellation', sentinel).cancelled, true)
  assert.equal(createStorageFaultScenario('restart', sentinel).nextGeneration(4), 5)
})

test('production storage owns classification without exposing fault injection', () => {
  const fault = source('src/lib/storage/fault.ts')
  const io = source('src/lib/storage/io.ts')
  const validated = source('src/lib/storage/validated.ts')
  const production = `${fault}\n${io}\n${validated}`

  assert.match(io, /classifyStorageFailure/)
  assert.match(validated, /corruption/)
  assert.doesNotMatch(production, /inject|privateSentinel|storedValue|nextGeneration/)
})
