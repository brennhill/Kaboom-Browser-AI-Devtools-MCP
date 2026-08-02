// @ts-nocheck
/**
 * @fileoverview server.test.js — Tests for extension/background/server.js.
 * Covers request headers, server communication, health checks, and polling.
 */

import { test, describe, mock, beforeEach } from 'node:test'
import assert from 'node:assert'
import { MANIFEST_VERSION } from '../shared/helpers.js'

// Store original fetch
const _originalFetch = globalThis.fetch

// Mock fetch for server tests
let mockFetch

beforeEach(() => {
  mock.restoreAll()
  mockFetch = mock.fn()
  globalThis.fetch = mockFetch
})

// Import after chrome mock is set up by helpers.js
const {
  getRequestHeaders,
  sendLogsToServer,
  sendNetworkBodiesToServer,
  checkServerHealth,
  sendWSEventsToServer,
  sendEnhancedActionsToServer,
  updateBadge
} = await import('../../../extension/background/sync/server.js')

// ============================================
// getRequestHeaders
// ============================================

describe('getRequestHeaders', () => {
  test('returns standard headers with version', () => {
    const headers = getRequestHeaders()
    assert.strictEqual(headers['Content-Type'], 'application/json')
    assert.ok(headers['X-Kaboom-Client'].startsWith('kaboom-extension/'))
    assert.strictEqual(headers['X-Kaboom-Extension-Version'], MANIFEST_VERSION)
  })

  test('merges additional headers without overwriting', () => {
    const headers = getRequestHeaders({ 'X-Custom': 'value' })
    assert.strictEqual(headers['Content-Type'], 'application/json')
    assert.strictEqual(headers['X-Custom'], 'value')
  })

  test('additional headers can override defaults', () => {
    const headers = getRequestHeaders({ 'Content-Type': 'text/plain' })
    assert.strictEqual(headers['Content-Type'], 'text/plain')
  })
})

// ============================================
// sendLogsToServer
// ============================================

describe('sendLogsToServer', () => {
  test('calls fetch with correct URL, method, headers, and body', async () => {
    mockFetch.mock.mockImplementation(() =>
      Promise.resolve({ ok: true, json: () => Promise.resolve({ entries: 5 }) })
    )

    const result = await sendLogsToServer('http://localhost:9222', [{ level: 'error', message: 'test' }])
    assert.strictEqual(result.entries, 5)

    const call = mockFetch.mock.calls[0]
    assert.strictEqual(call.arguments[0], 'http://localhost:9222/logs')
    assert.strictEqual(JSON.parse(call.arguments[1].body).entries.length, 1)
    assert.strictEqual(call.arguments[1].method, 'POST')
  })

  test('throws on non-ok response', async () => {
    mockFetch.mock.mockImplementation(() =>
      Promise.resolve({ ok: false, status: 500, statusText: 'Internal Server Error' })
    )

    await assert.rejects(
      () => sendLogsToServer('http://localhost:9222', [{ level: 'error' }]),
      (err) => err.message.includes('500')
    )
  })
})

// ============================================
// sendNetworkBodiesToServer
// ============================================

describe('sendNetworkBodiesToServer', () => {
  test('sends snake_case keys as-is', async () => {
    mockFetch.mock.mockImplementation(() =>
      Promise.resolve({ ok: true })
    )

    await sendNetworkBodiesToServer('http://localhost:9222', [{
      url: 'https://api.example.com',
      method: 'GET',
      status: 200,
      content_type: 'application/json',
      request_body: '{}',
      response_body: '{"ok":true}',
      duration: 100,
      tab_id: 1
    }])

    const call = mockFetch.mock.calls[0]
    const body = JSON.parse(call.arguments[1].body)
    const entry = body.bodies[0]

    assert.strictEqual(entry.content_type, 'application/json')
    assert.strictEqual(entry.request_body, '{}')
    assert.strictEqual(entry.response_body, '{"ok":true}')
    assert.strictEqual(entry.tab_id, 1)
    // Verify camelCase keys are NOT present
    assert.strictEqual(entry.contentType, undefined)
    assert.strictEqual(entry.requestBody, undefined)
  })

  test('omits tab_id when tabId is null', async () => {
    mockFetch.mock.mockImplementation(() =>
      Promise.resolve({ ok: true })
    )

    await sendNetworkBodiesToServer('http://localhost:9222', [{
      url: 'https://api.example.com',
      method: 'GET',
      status: 200,
      content_type: 'application/json'
    }])

    const call = mockFetch.mock.calls[0]
    const body = JSON.parse(call.arguments[1].body)
    assert.strictEqual('tab_id' in body.bodies[0], false)
  })

  test('includes response_truncated when set', async () => {
    mockFetch.mock.mockImplementation(() =>
      Promise.resolve({ ok: true })
    )

    await sendNetworkBodiesToServer('http://localhost:9222', [{
      url: 'https://api.example.com',
      method: 'GET',
      status: 200,
      content_type: 'text/html',
      response_truncated: true
    }])

    const call = mockFetch.mock.calls[0]
    const body = JSON.parse(call.arguments[1].body)
    assert.strictEqual(body.bodies[0].response_truncated, true)
  })
})

// ============================================
// sendWSEventsToServer
// ============================================

describe('sendWSEventsToServer', () => {
  test('sends events to correct endpoint', async () => {
    mockFetch.mock.mockImplementation(() =>
      Promise.resolve({ ok: true })
    )

    await sendWSEventsToServer('http://localhost:9222', [
      { event: 'message', data: 'hello', id: 'ws1' }
    ])

    const call = mockFetch.mock.calls[0]
    assert.strictEqual(call.arguments[0], 'http://localhost:9222/websocket-events')
    assert.strictEqual(call.arguments[1].method, 'POST')
  })

  test('throws on non-ok response', async () => {
    mockFetch.mock.mockImplementation(() =>
      Promise.resolve({ ok: false, status: 503, statusText: 'Service Unavailable' })
    )

    await assert.rejects(
      () => sendWSEventsToServer('http://localhost:9222', [{ event: 'open' }]),
      (err) => err.message.includes('503')
    )
  })
})

// ============================================
// sendEnhancedActionsToServer
// ============================================

describe('sendEnhancedActionsToServer', () => {
  test('sends actions to correct endpoint', async () => {
    mockFetch.mock.mockImplementation(() =>
      Promise.resolve({ ok: true })
    )

    await sendEnhancedActionsToServer('http://localhost:9222', [
      { type: 'click', timestamp: 1000 }
    ])

    const call = mockFetch.mock.calls[0]
    assert.strictEqual(call.arguments[0], 'http://localhost:9222/enhanced-actions')
    const body = JSON.parse(call.arguments[1].body)
    assert.strictEqual(body.actions.length, 1)
  })
})

// ============================================
// checkServerHealth
// ============================================

describe('checkServerHealth', () => {
  test('returns connected true on success', async () => {
    mockFetch.mock.mockImplementation(() =>
      Promise.resolve({
        ok: true,
        json: () => Promise.resolve({
          status: 'ok',
          uptime: 120,
          capture: { extension_connected: true }
        })
      })
    )

    const result = await checkServerHealth('http://localhost:9222')
    assert.strictEqual(result.connected, true)
    assert.strictEqual(result.status, 'ok')
    assert.strictEqual(result.uptime, 120)
  })

  test('keeps daemon connected when extension heartbeat is missing', async () => {
    // Assert against the same literal the fixture serves. This test is about
    // heartbeat handling, not about any particular release, and hardcoding the
    // version in two places desyncs whenever bump-version.js rewrites only one.
    const servedVersion = '0.8.5'
    mockFetch.mock.mockImplementation(() =>
      Promise.resolve({
        ok: true,
        json: () =>
          Promise.resolve({
            status: 'ok',
            version: servedVersion,
            capture: {
              extension_connected: false,
              extension_last_seen: ''
            }
          })
      })
    )

    const result = await checkServerHealth('http://localhost:9222')
    assert.strictEqual(result.connected, true)
    assert.strictEqual(result.extensionConnected, false)
    assert.strictEqual(result.status, 'ok')
    assert.strictEqual(result.version, servedVersion)
    assert.ok(result.extensionError.includes('heartbeat'))
  })

  test('keeps daemon connected when heartbeat status is unavailable', async () => {
    mockFetch.mock.mockImplementation(() =>
      Promise.resolve({
        ok: true,
        json: () => Promise.resolve({ status: 'ok', version: '0.9.0' })
      })
    )

    const result = await checkServerHealth('http://localhost:9222')
    assert.strictEqual(result.connected, true)
    assert.strictEqual(result.extensionConnected, undefined)
    assert.ok(result.extensionError.includes('unavailable'))
  })

  test('returns connected false on fetch error', async () => {
    mockFetch.mock.mockImplementation(() =>
      Promise.reject(new Error('Connection refused'))
    )

    const result = await checkServerHealth('http://localhost:9222')
    assert.strictEqual(result.connected, false)
    assert.strictEqual(result.error, 'Connection refused')
  })

  test('returns connected false on non-ok response', async () => {
    mockFetch.mock.mockImplementation(() =>
      Promise.resolve({ ok: false, status: 503 })
    )

    const result = await checkServerHealth('http://localhost:9222')
    assert.strictEqual(result.connected, false)
    assert.ok(result.error.includes('503'))
  })

  test('returns connected false on invalid JSON response', async () => {
    mockFetch.mock.mockImplementation(() =>
      Promise.resolve({
        ok: true,
        json: () => Promise.reject(new SyntaxError('Unexpected token'))
      })
    )

    const result = await checkServerHealth('http://localhost:9222')
    assert.strictEqual(result.connected, false)
    assert.ok(result.error.includes('invalid response'))
  })
})

// ============================================
// updateBadge
// ============================================

describe('updateBadge', () => {
  test('sets green badge with empty text when connected with no errors', () => {
    updateBadge({ connected: true, errorCount: 0 })
    const textCalls = chrome.action.setBadgeText.mock.calls
    const lastText = textCalls[textCalls.length - 1].arguments[0]
    assert.strictEqual(lastText.text, '')
  })

  test('sets error count on badge when connected with errors', () => {
    updateBadge({ connected: true, errorCount: 5 })
    const textCalls = chrome.action.setBadgeText.mock.calls
    const lastText = textCalls[textCalls.length - 1].arguments[0]
    assert.strictEqual(lastText.text, '5')
  })

  test('caps badge at 99+', () => {
    updateBadge({ connected: true, errorCount: 150 })
    const textCalls = chrome.action.setBadgeText.mock.calls
    const lastText = textCalls[textCalls.length - 1].arguments[0]
    assert.strictEqual(lastText.text, '99+')
  })

  test('shows ! when disconnected', () => {
    updateBadge({ connected: false })
    const textCalls = chrome.action.setBadgeText.mock.calls
    const lastText = textCalls[textCalls.length - 1].arguments[0]
    assert.strictEqual(lastText.text, '!')
  })
})
