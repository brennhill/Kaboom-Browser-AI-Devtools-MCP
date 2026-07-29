// @ts-nocheck
/**
 * @fileoverview command-lifecycle.test.js — Contract tests for command dispatch lifecycle.
 * Verifies normalization across sync/async handlers and enforces one terminal result.
 */

import { beforeEach, describe, mock, test } from 'node:test'
import assert from 'node:assert'
import { createMockChrome } from '../shared/helpers.js'

function makeType(label) {
  return `__lifecycle_${label}_${Date.now()}_${Math.random().toString(16).slice(2)}`
}

async function setupHarness() {
  globalThis.chrome = createMockChrome({
    tabs: {
      query: mock.fn(() =>
        Promise.resolve([
          {
            id: 1,
            url: 'https://example.com',
            status: 'complete',
            title: 'Example'
          }
        ])
      ),
      get: mock.fn((tabId) =>
        Promise.resolve({
          id: tabId,
          url: 'https://example.com',
          status: 'complete',
          title: 'Example'
        })
      ),
      sendMessage: mock.fn(() => Promise.resolve({ success: true })),
      update: mock.fn((tabId, patch) => Promise.resolve({ id: tabId, ...patch, url: 'https://example.com' })),
      create: mock.fn(() => Promise.resolve({ id: 2, url: 'https://example.com', status: 'complete', title: 'Example' })),
      onRemoved: { addListener: mock.fn() },
      onUpdated: { addListener: mock.fn() }
    }
  })
  globalThis.fetch = mock.fn(() => Promise.resolve({ ok: true, json: async () => ({}) }))

  const state = await import('../../../extension/background/state.js')
  state.markInitComplete()

  const registry = await import('../../../extension/background/commands/registry.js')
  const pending = await import('../../../extension/background/pending-queries.js')
  return { registry, pending }
}

function createSyncClientSink() {
  const queued = []
  return {
    queued,
    syncClient: {
      queueCommandResult(result) {
        queued.push(result)
      }
    }
  }
}

describe('Command lifecycle contracts', () => {
  beforeEach(() => {
    mock.reset()
  })

  test('async query calling ctx.sendResult is normalized to async completion', async () => {
    const { registry, pending } = await setupHarness()
    const queryType = makeType('async_send_result')
    registry.registerCommand(queryType, async (ctx) => {
      ctx.sendResult({ ok: true, mode: 'normalized' })
    })

    const { queued, syncClient } = createSyncClientSink()
    await pending.handlePendingQuery(
      {
        id: 'q-async-result',
        type: queryType,
        correlation_id: 'corr-async-result',
        params: {}
      },
      syncClient
    )

    assert.strictEqual(queued.length, 1)
    assert.strictEqual(queued[0].correlation_id, 'corr-async-result')
    assert.strictEqual(queued[0].status, 'complete')
    assert.strictEqual(queued[0].result.ok, true)
    assert.strictEqual(queued[0].result.mode, 'normalized')
  })

  test('duplicate terminal sends from a handler are ignored after first completion', async () => {
    const { registry, pending } = await setupHarness()
    const queryType = makeType('duplicate_terminal')
    registry.registerCommand(queryType, async (ctx) => {
      ctx.sendResult({ step: 1 })
      ctx.sendResult({ step: 2 })
      ctx.sendAsyncResult(ctx.syncClient, ctx.query.id, 'ignored', 'error', { step: 3 }, 'unexpected')
    })

    const { queued, syncClient } = createSyncClientSink()
    await pending.handlePendingQuery(
      {
        id: 'q-duplicate',
        type: queryType,
        params: {}
      },
      syncClient
    )

    assert.strictEqual(queued.length, 1)
    assert.strictEqual(queued[0].status, 'complete')
    assert.strictEqual(queued[0].result.step, 1)
  })

  test('handler returning without terminal result gets explicit no_result error', async () => {
    const { registry, pending } = await setupHarness()
    const queryType = makeType('missing_terminal')
    registry.registerCommand(queryType, async () => {})

    const { queued, syncClient } = createSyncClientSink()
    await pending.handlePendingQuery(
      {
        id: 'q-no-result',
        type: queryType,
        correlation_id: 'corr-no-result',
        params: {}
      },
      syncClient
    )

    assert.strictEqual(queued.length, 1)
    assert.strictEqual(queued[0].status, 'error')
    assert.strictEqual(queued[0].error, 'no_result')
    assert.strictEqual(queued[0].result.error, 'no_result')
  })

  test('sync query calling ctx.sendAsyncResult(error) is normalized to sync error payload', async () => {
    const { registry, pending } = await setupHarness()
    const queryType = makeType('sync_from_async_error')
    registry.registerCommand(queryType, async (ctx) => {
      ctx.sendAsyncResult(ctx.syncClient, ctx.query.id, 'ignored', 'error', { source: 'async' }, 'boom')
    })

    const { queued, syncClient } = createSyncClientSink()
    await pending.handlePendingQuery(
      {
        id: 'q-sync-async-error',
        type: queryType,
        params: {}
      },
      syncClient
    )

    assert.strictEqual(queued.length, 1)
    assert.strictEqual(queued[0].status, 'complete')
    assert.strictEqual(queued[0].result.success, false)
    assert.strictEqual(queued[0].result.status, 'error')
    assert.strictEqual(queued[0].result.error, 'boom')
    assert.deepStrictEqual(queued[0].result.result.source, 'async')
  })
})
