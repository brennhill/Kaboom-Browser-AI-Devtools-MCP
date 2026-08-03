// @ts-nocheck
/**
 * @fileoverview Sync-client command dispatch, result queue, and version negotiation tests.
 */

import { test, describe, mock, beforeEach, afterEach } from 'node:test'
import assert from 'node:assert'
import {
  SyncClient,
  createMockCallbacks,
  installFetchMock,
  makeSyncResponse,
  tick,
} from './sync-client-fixture.js'

describe('SyncClient — Command dispatch', () => {
  let client
  let callbacks

  beforeEach(() => {
    mock.reset()
    callbacks = createMockCallbacks()
  })

  afterEach(() => {
    client.stop()
  })

  test('should dispatch commands received from server', async () => {
    const commands = [
      { id: 'cmd-1', type: 'dom_query', params: { selector: '#app' } },
      { id: 'cmd-2', type: 'screenshot', params: {} }
    ]
    installFetchMock(makeSyncResponse({ commands, next_poll_ms: 60000 }))

    client = new SyncClient('http://localhost:7777', 'sess-1', callbacks)
    client.start()
    await tick(50)

    assert.strictEqual(callbacks.onCommand.mock.calls.length, 2)
    assert.deepStrictEqual(callbacks.onCommand.mock.calls[0].arguments[0], {
      connection_generation: 1,
      ...commands[0]
    })
    assert.deepStrictEqual(callbacks.onCommand.mock.calls[1].arguments[0], {
      connection_generation: 1,
      ...commands[1]
    })
  })

  test('rejects commands from a superseded connection generation', async () => {
    const commands = [
      { id: 'cmd-stale', type: 'dom_query', params: {}, connection_generation: 6 },
      { id: 'cmd-current', type: 'screenshot', params: {}, connection_generation: 7 }
    ]
    installFetchMock(makeSyncResponse({ connection_generation: 7, commands, next_poll_ms: 60000 }))

    client = new SyncClient('http://localhost:7777', 'sess-1', callbacks)
    client.start()
    await tick(50)

    assert.strictEqual(callbacks.onCommand.mock.calls.length, 1)
    assert.strictEqual(callbacks.onCommand.mock.calls[0].arguments[0].id, 'cmd-current')
    const staleLog = callbacks.debugLog.mock.calls.find(
      (call) => call.arguments[1] === 'Rejected stale connection generation'
    )
    assert.ok(staleLog, 'stale command rejection should remain visible to Doctor diagnostics')
  })

  test('should set lastCommandAck to the last successfully dispatched command id', async () => {
    const commands = [
      { id: 'cmd-1', type: 'dom_query', params: {} },
      { id: 'cmd-2', type: 'screenshot', params: {} }
    ]
    installFetchMock(makeSyncResponse({ commands, next_poll_ms: 60000 }))

    client = new SyncClient('http://localhost:7777', 'sess-1', callbacks)
    client.start()
    await tick(50) // wait for async dispatch to complete (ack set in finally block)

    assert.strictEqual(client.getState().lastCommandAck, 'cmd-2')
  })

  test('should continue dispatching if one command handler throws', async () => {
    let callNum = 0
    callbacks.onCommand = mock.fn(() => {
      callNum++
      if (callNum === 1) throw new Error('handler crash')
      return Promise.resolve()
    })

    const commands = [
      { id: 'cmd-1', type: 'will_fail', params: {} },
      { id: 'cmd-2', type: 'will_succeed', params: {} }
    ]
    installFetchMock(makeSyncResponse({ commands, next_poll_ms: 60000 }))

    client = new SyncClient('http://localhost:7777', 'sess-1', callbacks)
    client.start()
    await tick(50) // wait for async dispatch to complete (ack set in finally block)

    // Both commands attempted
    assert.strictEqual(callbacks.onCommand.mock.calls.length, 2)
    // lastCommandAck should be the second command — ack advances through failed dispatches too
    assert.strictEqual(client.getState().lastCommandAck, 'cmd-2')
  })

  test('should not ack commands before dispatch completes', async () => {
    let resolveSlowHandler
    callbacks.onCommand = mock.fn(
      () =>
        new Promise((resolve) => {
          resolveSlowHandler = resolve
        })
    )

    const commands = [{ id: 'cmd-slow', type: 'browser_action', params: {} }]
    installFetchMock(makeSyncResponse({ commands, next_poll_ms: 60000 }))

    client = new SyncClient('http://localhost:7777', 'sess-1', callbacks)
    client.start()
    await tick(30) // sync fires, command dispatched but handler not resolved yet

    // Ack should NOT be set while dispatch is still in progress
    assert.strictEqual(client.getState().lastCommandAck, null, 'should not ack before dispatch completes')

    // Now resolve the handler
    resolveSlowHandler()
    await tick(30)

    // Ack should now be set
    assert.strictEqual(client.getState().lastCommandAck, 'cmd-slow', 'should ack after dispatch completes')
  })

  test('should report running commands in in_progress heartbeat payload', async () => {
    let callCount = 0
    globalThis.fetch = mock.fn(() => {
      callCount++
      if (callCount === 1) {
        return Promise.resolve({
          ok: true,
          json: () =>
            Promise.resolve(
              makeSyncResponse({
                commands: [{ id: 'cmd-running', type: 'browser_action', correlation_id: 'corr-running', params: {} }],
                next_poll_ms: 10
              })
            )
        })
      }
      return Promise.resolve({
        ok: true,
        json: () => Promise.resolve(makeSyncResponse({ commands: [], next_poll_ms: 60000 }))
      })
    })

    callbacks.onCommand = mock.fn(
      () =>
        new Promise((resolve) => {
          setTimeout(resolve, 120)
        })
    )

    client = new SyncClient('http://localhost:7777', 'sess-1', callbacks)
    client.start()
    await tick(70)

    assert.ok(globalThis.fetch.mock.calls.length >= 2, 'expected at least two sync cycles')
    const secondBody = JSON.parse(globalThis.fetch.mock.calls[1].arguments[1].body)
    assert.ok(Array.isArray(secondBody.in_progress))
    const active = secondBody.in_progress.find((cmd) => cmd.id === 'cmd-running')
    assert.ok(active, `expected cmd-running in in_progress, got ${JSON.stringify(secondBody.in_progress)}`)
    assert.strictEqual(active.correlation_id, 'corr-running')
    assert.strictEqual(active.type, 'browser_action')
    assert.ok(active.status === 'running' || active.status === 'pending')
  })

  test('retains the originating generation on late command progress and results', async () => {
    let resolveCommand
    let callCount = 0
    globalThis.fetch = mock.fn(() => {
      callCount++
      const response =
        callCount === 1
          ? makeSyncResponse({
              connection_generation: 1,
              commands: [{ id: 'cmd-late', type: 'browser_action', params: {} }],
              next_poll_ms: 10
            })
          : makeSyncResponse({ connection_generation: 2, commands: [], next_poll_ms: 10 })
      return Promise.resolve({ ok: true, json: () => Promise.resolve(response) })
    })
    callbacks.onCommand = mock.fn(
      () =>
        new Promise((resolve) => {
          resolveCommand = () => {
            client.queueCommandResult({ id: 'cmd-late', status: 'complete', result: { success: true } })
            resolve()
          }
        })
    )

    client = new SyncClient('http://localhost:7777', 'sess-1', callbacks)
    client.start()
    await tick(70)
    resolveCommand()
    await tick(70)

    const bodies = globalThis.fetch.mock.calls.map((call) => JSON.parse(call.arguments[1].body))
    const progress = bodies.flatMap((body) => body.in_progress || []).find((entry) => entry.id === 'cmd-late')
    assert.strictEqual(progress.connection_generation, 1)
    const result = bodies.flatMap((body) => body.command_results || []).find((entry) => entry.id === 'cmd-late')
    assert.strictEqual(result.connection_generation, 1)
  })

  test('should not dispatch commands when response has empty commands array', async () => {
    installFetchMock(makeSyncResponse({ commands: [], next_poll_ms: 60000 }))

    client = new SyncClient('http://localhost:7777', 'sess-1', callbacks)
    client.start()
    await tick(50)

    assert.strictEqual(callbacks.onCommand.mock.calls.length, 0)
  })

  test('should not redispatch a command ID that was already acknowledged', async () => {
    let pollCount = 0
    globalThis.fetch = mock.fn(() => {
      pollCount++
      if (pollCount === 1) {
        return Promise.resolve({
          ok: true,
          json: () =>
            Promise.resolve(
              makeSyncResponse({
                commands: [{ id: 'cmd-dup', type: 'dom_query', params: {} }],
                next_poll_ms: 10
              })
            )
        })
      }
      return Promise.resolve({
        ok: true,
        json: () =>
          Promise.resolve(
            makeSyncResponse({
              commands: [{ id: 'cmd-dup', type: 'dom_query', params: {} }],
              next_poll_ms: 60000
            })
          )
      })
    })

    client = new SyncClient('http://localhost:7777', 'sess-1', callbacks)
    client.start()
    await tick(120)

    assert.strictEqual(callbacks.onCommand.mock.calls.length, 1)
  })

  test('should retain only five acknowledged command signatures across daemon ID reuse', async () => {
    let pollCount = 0
    const firstBatch = Array.from({ length: 6 }, (_, index) => ({
      id: `cmd-${index + 1}`,
      type: 'screenshot',
      params: {}
    }))
    globalThis.fetch = mock.fn(() => {
      pollCount++
      return Promise.resolve({
        ok: true,
        json: () =>
          Promise.resolve(
            makeSyncResponse({
              commands:
                pollCount === 1
                  ? firstBatch
                  : pollCount === 2
                    ? [{ id: 'cmd-1', type: 'screenshot', params: {} }]
                    : [],
              next_poll_ms: pollCount < 2 ? 10 : 60000
            })
          )
      })
    })

    client = new SyncClient('http://localhost:7777', 'sess-1', callbacks)
    client.start()
    await tick(120)

    assert.strictEqual(callbacks.onCommand.mock.calls.length, 7)
  })

  test('should keep syncing while async command handlers are still running', async () => {
    let fetchCalls = 0
    globalThis.fetch = mock.fn(() => {
      fetchCalls++
      if (fetchCalls === 1) {
        return Promise.resolve({
          ok: true,
          json: () =>
            Promise.resolve(
              makeSyncResponse({
                commands: [{ id: 'cmd-slow', type: 'browser_action', params: {} }],
                next_poll_ms: 10
              })
            )
        })
      }
      return Promise.resolve({
        ok: true,
        json: () => Promise.resolve(makeSyncResponse({ commands: [], next_poll_ms: 10 }))
      })
    })

    callbacks.onCommand = mock.fn(
      () =>
        new Promise((resolve) => {
          setTimeout(resolve, 80)
        })
    )

    client = new SyncClient('http://localhost:7777', 'sess-1', callbacks)
    client.start()

    await tick(40)
    assert.ok(fetchCalls >= 2, `expected heartbeat sync to continue while command is running, got ${fetchCalls}`)

    await tick(120)
    assert.strictEqual(callbacks.onCommand.mock.calls.length, 1)
    assert.ok(fetchCalls >= 2, `expected at least 2 sync cycles, got ${fetchCalls}`)
  })

  test('should timeout hanging command handlers and continue syncing', async () => {
    let fetchCalls = 0
    globalThis.fetch = mock.fn(() => {
      fetchCalls++
      if (fetchCalls === 1) {
        return Promise.resolve({
          ok: true,
          json: () =>
            Promise.resolve(
              makeSyncResponse({
                commands: [{ id: 'cmd-hang', type: 'browser_action', params: {} }],
                next_poll_ms: 10
              })
            )
        })
      }
      return Promise.resolve({
        ok: true,
        json: () => Promise.resolve(makeSyncResponse({ commands: [], next_poll_ms: 10 }))
      })
    })

    callbacks.onCommand = mock.fn(async () => {
      await new Promise(() => {})
    })
    callbacks.commandTimeoutMs = 50

    client = new SyncClient('http://localhost:7777', 'sess-1', callbacks)
    const queued = []
    const originalQueue = client.queueCommandResult.bind(client)
    client.queueCommandResult = (result) => {
      queued.push(result)
      originalQueue(result)
    }
    client.start()

    for (let i = 0; i < 40 && queued.length === 0; i++) {
      await tick(10)
    }

    assert.ok(queued.length > 0, 'expected a timeout error result to be queued')
    assert.strictEqual(queued[0].id, 'cmd-hang')
    assert.strictEqual(queued[0].status, 'error')
    assert.ok(String(queued[0].error).includes('timed out'))
    for (let i = 0; i < 20 && fetchCalls < 2; i++) {
      await tick(10)
    }
    assert.ok(fetchCalls >= 2, `expected sync loop to continue after timeout, got ${fetchCalls} fetch call(s)`)
  })

  test('should not let long command timeout block heartbeat polling', async () => {
    let fetchCalls = 0
    globalThis.fetch = mock.fn(() => {
      fetchCalls++
      if (fetchCalls === 1) {
        return Promise.resolve({
          ok: true,
          json: () =>
            Promise.resolve(
              makeSyncResponse({
                commands: [{ id: 'cmd-long', type: 'browser_action', params: {} }],
                next_poll_ms: 10
              })
            )
        })
      }
      return Promise.resolve({
        ok: true,
        json: () => Promise.resolve(makeSyncResponse({ commands: [], next_poll_ms: 10 }))
      })
    })

    callbacks.onCommand = mock.fn(async () => {
      await new Promise(() => {})
    })
    callbacks.commandTimeoutMs = 10_000

    client = new SyncClient('http://localhost:7777', 'sess-1', callbacks)
    client.start()

    await tick(60)
    assert.ok(fetchCalls >= 2, `expected follow-up heartbeat polls while command is still pending, got ${fetchCalls}`)
  })
})

describe('SyncClient — Command result queuing', () => {
  let client
  let callbacks

  beforeEach(() => {
    mock.reset()
    callbacks = createMockCallbacks()
  })

  afterEach(() => {
    client.stop()
  })

  test('should include pending results in request body', async () => {
    const mockFetch = installFetchMock(makeSyncResponse({ next_poll_ms: 60000 }))

    client = new SyncClient('http://localhost:7777', 'sess-1', callbacks)
    client.start()
    await tick(50) // first sync completes

    // Queue a result and it will flush immediately
    client.queueCommandResult({
      id: 'cmd-1',
      correlation_id: 'corr-1',
      status: 'complete',
      result: { html: '<div>test</div>' }
    })
    await tick(50)

    // Find the fetch call that included command_results
    const callsWithResults = mockFetch.mock.calls.filter((c) => {
      const body = JSON.parse(c.arguments[1].body)
      return body.command_results && body.command_results.length > 0
    })

    assert.ok(callsWithResults.length >= 1, 'Should have sent command_results')
    const body = JSON.parse(callsWithResults[0].arguments[1].body)
    assert.strictEqual(body.command_results[0].id, 'cmd-1')
    assert.strictEqual(body.command_results[0].status, 'complete')
    assert.strictEqual(body.connection_generation, 1)
    assert.strictEqual(body.command_results[0].connection_generation, 1)
  })

  test('should cap pending results queue at 200', () => {
    installFetchMock(makeSyncResponse({ next_poll_ms: 60000 }))
    // Don't start — we just want to test queuing without running syncs
    client = new SyncClient('http://localhost:7777', 'sess-1', callbacks)

    // Queue 250 results
    for (let i = 0; i < 250; i++) {
      // Manually push to avoid flush triggering (since client is not started, flush is a no-op)
      client.queueCommandResult({ id: `cmd-${i}`, status: 'complete' })
    }

    // The internal queue should be capped at 200
    // We can verify by checking that the oldest entries were dropped
    // (We test this indirectly — the code splices to keep last 200)
    // Start a sync to verify the body
    const mockFetch = installFetchMock(makeSyncResponse({ next_poll_ms: 60000 }))
    client.start()

    // Wait for sync
    return tick(50).then(() => {
      client.stop()
      const body = JSON.parse(mockFetch.mock.calls[0].arguments[1].body)
      assert.ok(body.command_results.length <= 200, `Expected <=200 results, got ${body.command_results.length}`)
    })
  })

  test('should clear pending results after successful sync', async () => {
    const mockFetch = installFetchMock(makeSyncResponse({ next_poll_ms: 60000 }))

    client = new SyncClient('http://localhost:7777', 'sess-1', callbacks)

    // Queue result before starting
    client.queueCommandResult({ id: 'cmd-1', status: 'complete' })
    client.start()
    await tick(50)

    // First sync includes results
    const firstBody = JSON.parse(mockFetch.mock.calls[0].arguments[1].body)
    assert.ok(firstBody.command_results)
    assert.strictEqual(firstBody.command_results.length, 1)

    // Force another sync via flush
    client.flush()
    await tick(50)

    // Second sync should not have command_results (they were cleared)
    const lastCall = mockFetch.mock.calls[mockFetch.mock.calls.length - 1]
    const lastBody = JSON.parse(lastCall.arguments[1].body)
    assert.strictEqual(lastBody.command_results, undefined)

    client.stop()
  })
})

describe('SyncClient — Version mismatch handling', () => {
  let client
  let callbacks

  beforeEach(() => {
    mock.reset()
    callbacks = createMockCallbacks()
  })

  afterEach(() => {
    client.stop()
  })

  test('should call onVersionMismatch when major.minor differs', async () => {
    installFetchMock(
      makeSyncResponse({
        server_version: '7.1.0',
        next_poll_ms: 60000
      })
    )

    client = new SyncClient('http://localhost:7777', 'sess-1', callbacks, '6.0.3')
    client.start()
    await tick(50)

    assert.strictEqual(callbacks.onVersionMismatch.mock.calls.length, 1)
    assert.strictEqual(callbacks.onVersionMismatch.mock.calls[0].arguments[0], '6.0.3')
    assert.strictEqual(callbacks.onVersionMismatch.mock.calls[0].arguments[1], '7.1.0')
  })

  test('should NOT call onVersionMismatch when major.minor matches', async () => {
    installFetchMock(
      makeSyncResponse({
        server_version: '6.0.9',
        next_poll_ms: 60000
      })
    )

    client = new SyncClient('http://localhost:7777', 'sess-1', callbacks, '6.0.3')
    client.start()
    await tick(50)

    assert.strictEqual(callbacks.onVersionMismatch.mock.calls.length, 0)
  })

  test('should NOT call onVersionMismatch when server_version is absent', async () => {
    installFetchMock(makeSyncResponse({ next_poll_ms: 60000 }))

    client = new SyncClient('http://localhost:7777', 'sess-1', callbacks, '6.0.3')
    client.start()
    await tick(50)

    assert.strictEqual(callbacks.onVersionMismatch.mock.calls.length, 0)
  })

  test('should NOT call onVersionMismatch when extension version is empty', async () => {
    installFetchMock(
      makeSyncResponse({
        server_version: '7.0.0',
        next_poll_ms: 60000
      })
    )

    client = new SyncClient('http://localhost:7777', 'sess-1', callbacks, '')
    client.start()
    await tick(50)

    assert.strictEqual(callbacks.onVersionMismatch.mock.calls.length, 0)
  })

  test('should NOT crash when onVersionMismatch callback is not provided', async () => {
    delete callbacks.onVersionMismatch
    installFetchMock(
      makeSyncResponse({
        server_version: '7.0.0',
        next_poll_ms: 60000
      })
    )

    client = new SyncClient('http://localhost:7777', 'sess-1', callbacks, '6.0.3')
    client.start()
    await tick(50)

    // Should not throw — just skip version check
    assert.strictEqual(client.isConnected(), true)
  })
})
