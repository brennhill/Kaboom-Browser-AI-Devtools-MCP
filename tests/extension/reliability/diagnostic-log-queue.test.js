// @ts-nocheck
/**
 * Purpose: Verifies persisted, redacted extension diagnostics across worker and daemon restarts.
 */
import { beforeEach, describe, test } from 'node:test'
import assert from 'node:assert'
import { createStorageFaultScenario } from '../state-recovery/storage-fault-fixture.js'

const queue = await import('../../../extension/background/runtime-state/log-queue.js')

function memoryStorage(initial) {
  let value = initial
  return {
    read: async () => value,
    write: async (next) => {
      value = structuredClone(next)
    },
    value: () => value
  }
}

describe('persisted extension diagnostic queue', { concurrency: false }, () => {
  beforeEach(() => queue.clearExtensionLogsForTesting())

  test('redacts secrets and private URL query values before persistence', async () => {
    const storage = memoryStorage(undefined)
    await queue.initializeExtensionLogQueue(storage)

    queue.pushExtensionLog({
      timestamp: '2026-08-02T12:00:00Z',
      level: 'error',
      message: 'fetch failed',
      source: 'background',
      category: 'connection',
      data: {
        authorization: 'Bearer secret',
        nested: { api_key: 'private' },
        url: 'https://example.test/private?token=secret#account'
      }
    })
    await queue.flushExtensionLogPersistenceForTesting()

    const persisted = storage.value().entries[0]
    assert.strictEqual(persisted.data.authorization, '[REDACTED]')
    assert.strictEqual(persisted.data.nested.api_key, '[REDACTED]')
    assert.strictEqual(persisted.data.url, 'https://example.test/private')
  })

  test('redacts credentials embedded in diagnostic strings', async () => {
    const storage = memoryStorage(undefined)
    await queue.initializeExtensionLogQueue(storage)

    queue.pushExtensionLog({
      timestamp: '2026-08-02T12:00:00Z',
      level: 'error',
      message: 'request failed with Bearer message-private-token',
      source: 'background',
      category: 'connection',
      data: {
        error: 'fetch rejected Authorization: Bearer private-token at https://example.test/?api_key=private'
      }
    })
    await queue.flushExtensionLogPersistenceForTesting()

    const error = storage.value().entries[0].data.error
    const message = storage.value().entries[0].message
    assert.ok(!error.includes('private-token'))
    assert.ok(!error.includes('api_key=private'))
    assert.match(error, /\[REDACTED\]/)
    assert.ok(!message.includes('message-private-token'))
  })

  test('property: nested runtime diagnostics redact secrets and preserve canonical fields', async () => {
    const storage = memoryStorage(undefined)
    await queue.initializeExtensionLogQueue(storage)
    let state = 0x9e3779b9
    for (let index = 0; index < 100; index++) {
      state = (Math.imul(state, 1664525) + 1013904223) >>> 0
      const safeID = `correlation_${state.toString(16)}`
      const secret = `property-secret-${index}`
      queue.pushExtensionLog({
        timestamp: `2026-08-03T12:00:${String(index % 60).padStart(2, '0')}Z`,
        level: 'error',
        message: `request failed with Bearer ${secret}`,
        source: 'background',
        category: 'runtime_message',
        data: {
          correlation_id: safeID,
          nested: [{ password: secret }, { error: `Authorization: Bearer ${secret}` }]
        }
      })
      const entry = queue.getExtensionLogQueueSnapshot().at(-1)
      assert.strictEqual(entry.data.correlation_id, safeID)
      assert.ok(!JSON.stringify(entry).includes(secret))
    }
  })

  test('rehydrates valid entries and merges logs emitted during startup', async () => {
    const storage = memoryStorage({
      version: 1,
      dropped_count: 0,
      entries: [{ timestamp: 'old', level: 'warn', message: 'before restart', source: 'background', category: 'lifecycle' }]
    })
    queue.pushExtensionLog({
      timestamp: 'new', level: 'debug', message: 'worker waking', source: 'background', category: 'lifecycle'
    })

    const recovery = await queue.initializeExtensionLogQueue(storage)

    assert.strictEqual(recovery.status, 'restored')
    assert.deepStrictEqual(queue.getExtensionLogQueueSnapshot().map((entry) => entry.message), [
      'before restart',
      'worker waking'
    ])
  })

  test('falls back safely when persisted state is corrupt', async () => {
    const storage = memoryStorage({ version: 1, entries: [{ data: 'missing required fields' }] })

    const recovery = await queue.initializeExtensionLogQueue(storage)

    assert.strictEqual(recovery.status, 'recovered')
    const recovered = queue.getExtensionLogQueueSnapshot()
    assert.strictEqual(recovered.length, 1)
    assert.strictEqual(recovered[0].message, 'Diagnostic queue state recovered')
    assert.strictEqual(recovered[0].data.fault_kind, 'corruption')
  })

  test('bounds entries and records saturation with a dropped count', async () => {
    const storage = memoryStorage(undefined)
    await queue.initializeExtensionLogQueue(storage)

    for (let index = 0; index < 230; index++) {
      queue.pushExtensionLog({
        timestamp: String(index), level: 'debug', message: `entry ${index}`, source: 'background', category: 'test'
      })
    }

    const metrics = queue.getExtensionLogQueueMetrics()
    assert.strictEqual(metrics.entries, 200)
    assert.ok(metrics.droppedCount >= 30)
    assert.strictEqual(metrics.saturated, true)
    assert.ok(queue.getExtensionLogQueueSnapshot().some((entry) => entry.message === 'Diagnostic queue saturated'))
  })

  test('acknowledges only the entries included in a successful sync snapshot', async () => {
    const storage = memoryStorage(undefined)
    await queue.initializeExtensionLogQueue(storage)
    queue.pushExtensionLog({ timestamp: '1', level: 'debug', message: 'one', source: 'background', category: 'test' })
    queue.pushExtensionLog({ timestamp: '2', level: 'debug', message: 'two', source: 'background', category: 'test' })
    const sentCount = queue.getExtensionLogQueueSnapshot().length
    queue.pushExtensionLog({ timestamp: '3', level: 'debug', message: 'three', source: 'background', category: 'test' })

    queue.acknowledgeExtensionLogQueue(sentCount)
    await queue.flushExtensionLogPersistenceForTesting()

    assert.deepStrictEqual(queue.getExtensionLogQueueSnapshot().map((entry) => entry.message), ['three'])
    assert.deepStrictEqual(storage.value().entries.map((entry) => entry.message), ['three'])
  })

  test('records correlated worker lifecycle entries with snake_case data', async () => {
    const storage = memoryStorage(undefined)
    await queue.initializeExtensionLogQueue(storage)

    queue.recordExtensionDiagnosticLifecycle('worker_started', 'ext-session-1', {
      restored_entries: 3
    })

    const entry = queue.getExtensionLogQueueSnapshot()[0]
    assert.strictEqual(entry.category, 'diagnostic_lifecycle')
    assert.deepStrictEqual(entry.data, {
      event: 'worker_started',
      correlation_id: 'ext-session-1',
      lifecycle_sequence: ['worker_started'],
      restored_entries: 3
    })
  })

  test('does not allow caller data to replace canonical lifecycle fields', async () => {
    const storage = memoryStorage(undefined)
    await queue.initializeExtensionLogQueue(storage)

    queue.recordExtensionDiagnosticLifecycle('sync_connected', 'canonical-id', {
      event: 'spoofed',
      correlation_id: 'spoofed-id',
      lifecycle_sequence: ['spoofed']
    })

    const entry = queue.getExtensionLogQueueSnapshot()[0]
    assert.strictEqual(entry.data.event, 'sync_connected')
    assert.strictEqual(entry.data.correlation_id, 'canonical-id')
    assert.deepStrictEqual(entry.data.lifecycle_sequence, ['sync_connected'])
  })

  test('carries the recent lifecycle sequence after delivered entries are acknowledged', async () => {
    const storage = memoryStorage(undefined)
    await queue.initializeExtensionLogQueue(storage)
    queue.recordExtensionDiagnosticLifecycle('worker_started', 'ext-session-1')
    queue.acknowledgeExtensionLogQueue(queue.getExtensionLogQueueSnapshot().length)

    queue.recordExtensionDiagnosticLifecycle('sync_connected', 'ext-session-1')

    const entry = queue.getExtensionLogQueueSnapshot()[0]
    assert.deepStrictEqual(entry.data.lifecycle_sequence, ['worker_started', 'sync_connected'])
  })

  test('retains an in-memory diagnostic when session persistence fails', async () => {
    const storage = {
      read: async () => undefined,
      write: async () => { throw new Error('quota detail must not escape') }
    }
    await queue.initializeExtensionLogQueue(storage)
    await queue.flushExtensionLogPersistenceForTesting()

    assert.strictEqual(queue.getExtensionLogQueueMetrics().persistenceFailures, 1)
    const failure = queue.getExtensionLogQueueSnapshot().find(
      (entry) => entry.message === 'Diagnostic queue persistence failed'
    )
    assert.ok(failure)
    assert.deepStrictEqual(failure.data, {
      reason: 'session_storage_write_failed',
      fault_kind: 'quota',
      occurrences: 1
    })
  })

  test('classifies read, quota, and cancellation faults without retaining private details', async () => {
    for (const kind of ['read', 'quota', 'cancellation']) {
      queue.clearExtensionLogsForTesting()
      const scenario = createStorageFaultScenario(kind, 'private-diagnostic-value')
      const storage = memoryStorage(undefined)
      if (kind === 'read') storage.read = async () => { throw scenario.error }
      else storage.write = async () => { throw scenario.error }

      await queue.initializeExtensionLogQueue(storage)
      await queue.flushExtensionLogPersistenceForTesting()

      const failure = queue.getExtensionLogQueueSnapshot().find((entry) => entry.category === 'diagnostic_queue')
      assert.strictEqual(failure.data.fault_kind, kind)
      assert.doesNotMatch(JSON.stringify(failure), /private-diagnostic-value/)
      assert.ok(queue.getExtensionLogQueueMetrics().persistenceFailures >= 1)
    }
  })
})
