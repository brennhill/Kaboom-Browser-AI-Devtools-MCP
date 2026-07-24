// @ts-nocheck
/**
 * @fileoverview server-edge-cases.test.js — Edge case and negative path tests
 * for extension/background/server.js functions.
 */

import { test, describe, mock, beforeEach } from 'node:test'
import assert from 'node:assert'
import { MANIFEST_VERSION as _MANIFEST_VERSION } from './helpers.js'

let mockFetch

beforeEach(() => {
  mock.restoreAll()
  mockFetch = mock.fn()
  globalThis.fetch = mockFetch
})

const {
  sendLogsToServer,
  sendNetworkBodiesToServer,
  sendWSEventsToServer: _sendWSEventsToServer,
  sendEnhancedActionsToServer: _sendEnhancedActionsToServer,
  sendPerformanceSnapshotsToServer
} = await import('../../extension/background/server.js')

// ============================================
// sendLogsToServer edge cases
// ============================================

describe('sendLogsToServer edge cases', () => {
  test('calls debugLogFn on success', async () => {
    mockFetch.mock.mockImplementation(() =>
      Promise.resolve({ ok: true, json: () => Promise.resolve({ entries: 3 }) })
    )
    const debugLog = mock.fn()
    await sendLogsToServer('http://localhost:9222', [{}, {}, {}], debugLog)
    assert.ok(debugLog.mock.calls.length >= 2, 'debugLog should be called for send + accept')
  })

  test('calls debugLogFn on error', async () => {
    mockFetch.mock.mockImplementation(() =>
      Promise.resolve({ ok: false, status: 502, statusText: 'Bad Gateway' })
    )
    const debugLog = mock.fn()
    await assert.rejects(
      () => sendLogsToServer('http://localhost:9222', [{}], debugLog)
    )
    assert.ok(debugLog.mock.calls.some(c => c.arguments[0] === 'error'))
  })

  test('empty entries array succeeds', async () => {
    mockFetch.mock.mockImplementation(() =>
      Promise.resolve({ ok: true, json: () => Promise.resolve({ entries: 0 }) })
    )
    const result = await sendLogsToServer('http://localhost:9222', [])
    assert.strictEqual(result.entries, 0)
  })
})

// ============================================
// sendNetworkBodiesToServer edge cases
// ============================================

describe('sendNetworkBodiesToServer edge cases', () => {
  test('handles body with all optional fields missing', async () => {
    mockFetch.mock.mockImplementation(() => Promise.resolve({ ok: true }))

    await sendNetworkBodiesToServer('http://localhost:9222', [{
      url: 'https://api.example.com',
      method: 'GET',
      status: 200
    }])

    const call = mockFetch.mock.calls[0]
    const body = JSON.parse(call.arguments[1].body)
    const entry = body.bodies[0]
    assert.strictEqual(entry.url, 'https://api.example.com')
    assert.strictEqual(entry.content_type, undefined)
    assert.strictEqual('tab_id' in entry, false)
    assert.strictEqual('response_truncated' in entry, false)
  })

  test('multiple bodies in single batch', async () => {
    mockFetch.mock.mockImplementation(() => Promise.resolve({ ok: true }))

    await sendNetworkBodiesToServer('http://localhost:9222', [
      { url: 'https://a.com', method: 'GET', status: 200, content_type: 'text/html' },
      { url: 'https://b.com', method: 'POST', status: 201, content_type: 'application/json' },
      { url: 'https://c.com', method: 'DELETE', status: 204 }
    ])

    const body = JSON.parse(mockFetch.mock.calls[0].arguments[1].body)
    assert.strictEqual(body.bodies.length, 3)
  })

  test('throws on server error', async () => {
    mockFetch.mock.mockImplementation(() =>
      Promise.resolve({ ok: false, status: 500, statusText: 'Internal Server Error' })
    )
    await assert.rejects(
      () => sendNetworkBodiesToServer('http://localhost:9222', [{ url: 'x', method: 'GET', status: 200 }])
    )
  })
})

// ============================================
// sendPerformanceSnapshotsToServer
// ============================================

describe('sendPerformanceSnapshotsToServer edge cases', () => {
  test('sends to correct endpoint', async () => {
    mockFetch.mock.mockImplementation(() => Promise.resolve({ ok: true }))
    await sendPerformanceSnapshotsToServer('http://localhost:9222', [{ metrics: {} }])
    assert.strictEqual(mockFetch.mock.calls[0].arguments[0], 'http://localhost:9222/performance-snapshots')
  })

  test('throws on error response', async () => {
    mockFetch.mock.mockImplementation(() =>
      Promise.resolve({ ok: false, status: 503, statusText: 'Unavailable' })
    )
    await assert.rejects(
      () => sendPerformanceSnapshotsToServer('http://localhost:9222', [{}])
    )
  })
})
