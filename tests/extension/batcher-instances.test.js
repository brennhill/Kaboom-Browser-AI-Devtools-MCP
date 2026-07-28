// @ts-nocheck
/**
 * @fileoverview batcher-instances.test.js — Tests for batcher-instances factory.
 * Covers createBatcherInstances return shape and withConnectionStatus wrapper
 * (success/error paths for connection status tracking).
 *
 * Run: node --experimental-test-module-mocks --test tests/extension/batcher-instances.test.js
 */

import { test, describe, mock, beforeEach } from 'node:test'
import assert from 'node:assert'

// ---------------------------------------------------------------------------
// Mock sibling modules before importing the unit under test
// ---------------------------------------------------------------------------

const mockUpdateBadge = mock.fn()
const mockCreateBatcherWithCircuitBreaker = mock.fn((sendFn, _opts) => {
  const batcher = { add: mock.fn(), flush: mock.fn() }
  return { batcher, sendFn }
})
const mockSendLogsToServer = mock.fn(() => Promise.resolve({ entries: 5 }))
const mockSendWSEventsToServer = mock.fn(() => Promise.resolve())
const mockSendEnhancedActionsToServer = mock.fn(() => Promise.resolve())
const mockSendNetworkBodiesToServer = mock.fn(() => Promise.resolve())
const mockSendPerformanceSnapshotsToServer = mock.fn(() => Promise.resolve())
const mockCheckContextAnnotations = mock.fn()

mock.module('../../extension/background/sync/batchers.js', {
  namedExports: {
    createBatcherWithCircuitBreaker: mockCreateBatcherWithCircuitBreaker
  }
})

mock.module('../../extension/background/sync/server.js', {
  namedExports: {
    updateBadge: mockUpdateBadge,
    sendLogsToServer: mockSendLogsToServer,
    sendWSEventsToServer: mockSendWSEventsToServer,
    sendEnhancedActionsToServer: mockSendEnhancedActionsToServer,
    sendNetworkBodiesToServer: mockSendNetworkBodiesToServer,
    sendPerformanceSnapshotsToServer: mockSendPerformanceSnapshotsToServer
  }
})

mock.module('../../extension/background/caches/snapshots.js', {
  namedExports: {
    checkContextAnnotations: mockCheckContextAnnotations
  }
})

const { createBatcherInstances } = await import(
  '../../extension/background/sync/batcher-instances.js'
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function createMockDeps(overrides = {}) {
  return {
    getServerUrl: mock.fn(() => 'http://localhost:7777'),
    getConnectionStatus: mock.fn(() => ({
      connected: true, entries: 10, maxEntries: 1000, errorCount: 0, logFile: 'test.log'
    })),
    setConnectionStatus: mock.fn(),
    debugLog: mock.fn(),
    ...overrides
  }
}

function createMockCircuitBreaker() {
  return { state: 'closed', fire: mock.fn() }
}

function resetMocks() {
  mockUpdateBadge.mock.resetCalls()
  mockCreateBatcherWithCircuitBreaker.mock.resetCalls()
  mockSendLogsToServer.mock.resetCalls()
  mockCheckContextAnnotations.mock.resetCalls()
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe('createBatcherInstances — factory shape', () => {
  beforeEach(() => resetMocks())

  test('returns all 10 batcher properties', () => {
    const deps = createMockDeps()
    const cb = createMockCircuitBreaker()
    const instances = createBatcherInstances(deps, cb)

    const expected = [
      'logBatcherWithCB', 'logBatcher',
      'wsBatcherWithCB', 'wsBatcher',
      'enhancedActionBatcherWithCB', 'enhancedActionBatcher',
      'networkBodyBatcherWithCB', 'networkBodyBatcher',
      'perfBatcherWithCB', 'perfBatcher'
    ]
    for (const key of expected) {
      assert.ok(instances[key] !== undefined, `Missing property: ${key}`)
    }
  })

  test('calls createBatcherWithCircuitBreaker 5 times (one per data type)', () => {
    const deps = createMockDeps()
    const cb = createMockCircuitBreaker()
    createBatcherInstances(deps, cb)

    assert.strictEqual(mockCreateBatcherWithCircuitBreaker.mock.calls.length, 5)
  })

  test('passes sharedCircuitBreaker to each batcher', () => {
    const deps = createMockDeps()
    const cb = createMockCircuitBreaker()
    createBatcherInstances(deps, cb)

    for (const call of mockCreateBatcherWithCircuitBreaker.mock.calls) {
      const opts = call.arguments[1]
      assert.strictEqual(opts.sharedCircuitBreaker, cb)
    }
  })
})

describe('withConnectionStatus — success path', () => {
  beforeEach(() => resetMocks())

  test('sets connected=true and calls updateBadge on success', async () => {
    const deps = createMockDeps()
    const cb = createMockCircuitBreaker()
    createBatcherInstances(deps, cb)

    // Extract the wrapped sendFn passed to createBatcherWithCircuitBreaker (first call = log batcher)
    const wrappedSendFn = mockCreateBatcherWithCircuitBreaker.mock.calls[0].arguments[0]
    await wrappedSendFn([{ level: 'error', msg: 'test' }])

    const connectedCall = deps.setConnectionStatus.mock.calls.find(
      c => c.arguments[0].connected === true
    )
    assert.ok(connectedCall, 'setConnectionStatus should be called with connected: true')
    assert.ok(mockUpdateBadge.mock.calls.length >= 1, 'updateBadge should be called')
  })

  test('log batcher onSuccess updates entries and errorCount', async () => {
    const deps = createMockDeps()
    const cb = createMockCircuitBreaker()
    createBatcherInstances(deps, cb)

    const wrappedSendFn = mockCreateBatcherWithCircuitBreaker.mock.calls[0].arguments[0]
    await wrappedSendFn([{ level: 'error', msg: 'err1' }, { level: 'log', msg: 'ok' }])

    // Should have a setConnectionStatus call that updates errorCount
    const entryCalls = deps.setConnectionStatus.mock.calls.filter(
      c => c.arguments[0].errorCount !== undefined
    )
    assert.ok(entryCalls.length >= 1, 'Should update errorCount')
    // 1 error out of 2 entries
    assert.strictEqual(entryCalls[0].arguments[0].errorCount, 1)
  })
})

describe('withConnectionStatus — error path', () => {
  beforeEach(() => resetMocks())

  test('sets connected=false and rethrows on send failure', async () => {
    mockSendLogsToServer.mock.mockImplementation(() =>
      Promise.reject(new Error('Network down'))
    )

    const deps = createMockDeps()
    const cb = createMockCircuitBreaker()
    createBatcherInstances(deps, cb)

    const wrappedSendFn = mockCreateBatcherWithCircuitBreaker.mock.calls[0].arguments[0]

    await assert.rejects(
      () => wrappedSendFn([{ level: 'error', msg: 'test' }]),
      (err) => err.message === 'Network down'
    )

    const disconnectedCall = deps.setConnectionStatus.mock.calls.find(
      c => c.arguments[0].connected === false
    )
    assert.ok(disconnectedCall, 'setConnectionStatus should be called with connected: false')
    assert.ok(mockUpdateBadge.mock.calls.length >= 1, 'updateBadge should be called on error')
  })
})
