// @ts-nocheck
/**
 * @fileoverview network-body-install.test.js — TDD tests for Bug #4 fix.
 * Tests that installFetchCapture() uses wrapFetchWithBodies() to capture
 * request/response bodies for ALL requests (not just errors).
 *
 * Bug #4: network_bodies returns empty arrays because installFetchCapture()
 * used wrapFetch() which only captures 4xx/5xx errors, not body data.
 *
 * Expected behavior after fix:
 * - Large POST body is captured and returned
 * - Large response body is captured
 * - Binary bodies are detected and marked
 * - Auth headers are stripped but body is preserved
 * - Error response body (500) is captured
 * - Bodies are truncated at limit (8KB request / 16KB response)
 * - Multiple requests return bodies (not empty arrays)
 */

import { test, describe, beforeEach, afterEach, mock } from 'node:test'
import assert from 'node:assert'
import { createMockWindow } from './helpers.js'

// Minimal Headers polyfill for testing
class MockHeaders {
  constructor(init) {
    this._map = new Map()
    if (init) {
      if (typeof init === 'object' && !(init instanceof MockHeaders)) {
        Object.entries(init).forEach(([k, v]) => this._map.set(k.toLowerCase(), v))
      }
    }
  }
  get(name) {
    return this._map.get(name.toLowerCase()) || null
  }
  set(name, value) {
    this._map.set(name.toLowerCase(), value)
  }
  entries() {
    return this._map.entries()
  }
  forEach(fn) {
    this._map.forEach((v, k) => fn(v, k))
  }
}

/**
 * Create a mock Response object for testing
 */
function createMockResponse(options = {}) {
  const headers = new MockHeaders()
  headers.set('content-type', options.contentType || 'application/json')
  if (options.headers) {
    Object.entries(options.headers).forEach(([k, v]) => headers.set(k, v))
  }

  return {
    ok: options.ok !== undefined ? options.ok : (options.status || 200) < 400,
    status: options.status || 200,
    statusText: options.statusText || 'OK',
    headers,
    clone: function () {
      return {
        ok: this.ok,
        status: this.status,
        statusText: this.statusText,
        headers: this.headers,
        text: () => Promise.resolve(options.body || '{}'),
        blob: () =>
          Promise.resolve({
            size: (options.body || '{}').length,
            type: options.contentType || 'application/json'
          })
      }
    }
  }
}

let originalWindow
let originalHeaders
let capturedBodyEvents

describe('Bug #4 Fix: installFetchCapture uses wrapFetchWithBodies', () => {
  beforeEach(() => {
    originalWindow = globalThis.window
    originalHeaders = globalThis.Headers
    globalThis.Headers = MockHeaders
    capturedBodyEvents = []

    // Create window with mock postMessage that tracks GASOLINE_NETWORK_BODY
    globalThis.window = {
      ...createMockWindow({ withFetch: true }),
      postMessage: (data) => {
        if (data && data.type === 'kaboom_network_body') {
          capturedBodyEvents.push(data.payload)
        }
      },
      addEventListener: mock.fn(),
      removeEventListener: mock.fn(),
      location: {
        pathname: '/',
        hostname: 'localhost',
        href: 'http://localhost/',
        origin: 'http://localhost'
      }
    }
  })

  afterEach(async () => {
    // Reset module cache to get fresh imports
    globalThis.window = originalWindow
    globalThis.Headers = originalHeaders
  })

  test('installFetchCapture should capture response body for successful 200 JSON response', async () => {
    // Import fresh module
    const { installFetchCapture, uninstallFetchCapture } = await import('../../extension/inject/observers.js')
    const { setNetworkBodyCaptureEnabled } = await import('../../extension/lib/net/network.js')

    // Enable body capture
    setNetworkBodyCaptureEnabled(true)

    // Set up mock fetch that returns 200 OK with JSON body
    const responseBody = '{"id":1,"name":"Alice","email":"alice@example.com"}'
    const mockResponse = createMockResponse({
      status: 200,
      body: responseBody,
      contentType: 'application/json'
    })
    globalThis.window.fetch = mock.fn(() => Promise.resolve(mockResponse))

    // Install fetch capture
    installFetchCapture()

    // Make a request
    await globalThis.window.fetch('/api/users/1')

    // Wait for async body capture
    await new Promise((r) => setTimeout(r, 50))

    // Verify body was captured (this is the key assertion - will FAIL before fix)
    assert.ok(
      capturedBodyEvents.length > 0,
      'Expected network body event to be captured for 200 response. ' +
        'Bug #4: installFetchCapture uses wrapFetch (error-only) instead of wrapFetchWithBodies.'
    )

    const event = capturedBodyEvents[0]
    assert.strictEqual(event.status, 200)
    assert.strictEqual(event.response_body, responseBody)

    uninstallFetchCapture()
  })

  test('large POST body is captured and returned', async () => {
    const { installFetchCapture, uninstallFetchCapture } = await import('../../extension/inject/observers.js')
    const { setNetworkBodyCaptureEnabled } = await import('../../extension/lib/net/network.js')

    setNetworkBodyCaptureEnabled(true)

    // Large request body (4KB)
    const largeRequestBody = JSON.stringify({ data: 'x'.repeat(4000) })
    const mockResponse = createMockResponse({ status: 201, body: '{"id":1}' })
    globalThis.window.fetch = mock.fn(() => Promise.resolve(mockResponse))

    installFetchCapture()

    await globalThis.window.fetch('/api/data', {
      method: 'POST',
      body: largeRequestBody,
      headers: { 'Content-Type': 'application/json' }
    })

    await new Promise((r) => setTimeout(r, 50))

    assert.ok(capturedBodyEvents.length > 0, 'Expected body capture for POST request')
    const event = capturedBodyEvents[0]
    assert.strictEqual(event.method, 'POST')
    assert.ok(event.request_body, 'Request body should be captured')
    assert.ok(event.request_body.includes('data'), 'Request body content should be present')

    uninstallFetchCapture()
  })

  test('large response body is captured', async () => {
    const { installFetchCapture, uninstallFetchCapture } = await import('../../extension/inject/observers.js')
    const { setNetworkBodyCaptureEnabled } = await import('../../extension/lib/net/network.js')

    setNetworkBodyCaptureEnabled(true)

    // Large response body (10KB)
    const largeBody = JSON.stringify({ items: Array(500).fill({ id: 1, name: 'test' }) })
    const mockResponse = createMockResponse({ status: 200, body: largeBody })
    globalThis.window.fetch = mock.fn(() => Promise.resolve(mockResponse))

    installFetchCapture()

    await globalThis.window.fetch('/api/items')

    await new Promise((r) => setTimeout(r, 50))

    assert.ok(capturedBodyEvents.length > 0, 'Expected body capture for large response')
    const event = capturedBodyEvents[0]
    assert.ok(event.response_body, 'Response body should be captured')
    assert.ok(event.response_body.length > 1000, 'Large body content should be present')

    uninstallFetchCapture()
  })

  test('binary bodies are detected and marked', async () => {
    const { installFetchCapture, uninstallFetchCapture } = await import('../../extension/inject/observers.js')
    const { setNetworkBodyCaptureEnabled } = await import('../../extension/lib/net/network.js')

    setNetworkBodyCaptureEnabled(true)

    const mockResponse = createMockResponse({
      status: 200,
      body: 'binary data here',
      contentType: 'image/png'
    })
    globalThis.window.fetch = mock.fn(() => Promise.resolve(mockResponse))

    installFetchCapture()

    await globalThis.window.fetch('/image.png')

    await new Promise((r) => setTimeout(r, 50))

    assert.ok(capturedBodyEvents.length > 0, 'Expected body capture for binary response')
    const event = capturedBodyEvents[0]
    assert.ok(event.response_body.includes('[Binary:'), 'Binary content should be marked as such')
    assert.ok(event.response_body.includes('image/png'), 'Binary content type should be included')

    uninstallFetchCapture()
  })

  test('error response body (500) is captured', async () => {
    const { installFetchCapture, uninstallFetchCapture } = await import('../../extension/inject/observers.js')
    const { setNetworkBodyCaptureEnabled } = await import('../../extension/lib/net/network.js')

    setNetworkBodyCaptureEnabled(true)

    const errorBody = '{"error":"Internal Server Error","details":"Database connection failed"}'
    const mockResponse = createMockResponse({
      status: 500,
      ok: false,
      body: errorBody,
      statusText: 'Internal Server Error'
    })
    globalThis.window.fetch = mock.fn(() => Promise.resolve(mockResponse))

    installFetchCapture()

    await globalThis.window.fetch('/api/crash')

    await new Promise((r) => setTimeout(r, 50))

    assert.ok(capturedBodyEvents.length > 0, 'Expected body capture for 500 response')
    const event = capturedBodyEvents[0]
    assert.strictEqual(event.status, 500)
    assert.ok(event.response_body.includes('Database connection failed'), 'Error response body should be captured')

    uninstallFetchCapture()
  })

  test('request bodies are truncated at 8KB limit', async () => {
    const { installFetchCapture, uninstallFetchCapture } = await import('../../extension/inject/observers.js')
    const { setNetworkBodyCaptureEnabled } = await import('../../extension/lib/net/network.js')

    setNetworkBodyCaptureEnabled(true)

    // Request body larger than 8KB
    const hugeRequestBody = 'x'.repeat(10000)
    const mockResponse = createMockResponse({ status: 201, body: '{"id":1}' })
    globalThis.window.fetch = mock.fn(() => Promise.resolve(mockResponse))

    installFetchCapture()

    await globalThis.window.fetch('/api/upload', {
      method: 'POST',
      body: hugeRequestBody
    })

    await new Promise((r) => setTimeout(r, 50))

    assert.ok(capturedBodyEvents.length > 0, 'Expected body capture')
    const event = capturedBodyEvents[0]
    // 8KB = 8192 bytes
    assert.ok(
      event.request_body.length <= 8192,
      `Request body should be truncated at 8KB, got ${event.request_body.length}`
    )

    uninstallFetchCapture()
  })

  test('response bodies are truncated at 16KB limit', async () => {
    const { installFetchCapture, uninstallFetchCapture } = await import('../../extension/inject/observers.js')
    const { setNetworkBodyCaptureEnabled } = await import('../../extension/lib/net/network.js')

    setNetworkBodyCaptureEnabled(true)

    // Response body larger than 16KB
    const hugeResponseBody = 'y'.repeat(20000)
    const mockResponse = createMockResponse({ status: 200, body: hugeResponseBody })
    globalThis.window.fetch = mock.fn(() => Promise.resolve(mockResponse))

    installFetchCapture()

    await globalThis.window.fetch('/api/huge')

    await new Promise((r) => setTimeout(r, 50))

    assert.ok(capturedBodyEvents.length > 0, 'Expected body capture')
    const event = capturedBodyEvents[0]
    // 16KB = 16384 bytes
    assert.ok(
      event.response_body.length <= 16384,
      `Response body should be truncated at 16KB, got ${event.response_body.length}`
    )

    uninstallFetchCapture()
  })

  test('multiple requests return bodies (not empty arrays)', async () => {
    const { installFetchCapture, uninstallFetchCapture } = await import('../../extension/inject/observers.js')
    const { setNetworkBodyCaptureEnabled } = await import('../../extension/lib/net/network.js')

    setNetworkBodyCaptureEnabled(true)

    // Make multiple requests - set up mock BEFORE installFetchCapture
    const responses = [
      createMockResponse({ status: 200, body: '{"user":"alice"}' }),
      createMockResponse({ status: 200, body: '{"user":"bob"}' }),
      createMockResponse({ status: 201, body: '{"created":true}' })
    ]

    let callCount = 0
    globalThis.window.fetch = mock.fn(() => Promise.resolve(responses[callCount++]))

    // Install AFTER mock is set up so wrapping works correctly
    installFetchCapture()

    await globalThis.window.fetch('/api/users/1')
    await globalThis.window.fetch('/api/users/2')
    await globalThis.window.fetch('/api/users', { method: 'POST', body: '{"name":"carol"}' })

    await new Promise((r) => setTimeout(r, 100))

    // All 3 requests should have captured bodies
    assert.strictEqual(
      capturedBodyEvents.length,
      3,
      `Expected 3 body events, got ${capturedBodyEvents.length}. ` + 'Bug #4: network_bodies returns empty arrays.'
    )

    // Verify each has actual body content
    assert.ok(capturedBodyEvents[0].response_body.includes('alice'))
    assert.ok(capturedBodyEvents[1].response_body.includes('bob'))
    assert.ok(capturedBodyEvents[2].response_body.includes('created'))

    uninstallFetchCapture()
  })

  test('bodies NOT captured when networkBodyCaptureEnabled is false', async () => {
    const { installFetchCapture, uninstallFetchCapture } = await import('../../extension/inject/observers.js')
    const { setNetworkBodyCaptureEnabled } = await import('../../extension/lib/net/network.js')

    // Disable body capture
    setNetworkBodyCaptureEnabled(false)

    const mockResponse = createMockResponse({ status: 200, body: '{"secret":"data"}' })
    globalThis.window.fetch = mock.fn(() => Promise.resolve(mockResponse))

    installFetchCapture()

    await globalThis.window.fetch('/api/private')

    await new Promise((r) => setTimeout(r, 50))

    // Should NOT capture body when disabled
    assert.strictEqual(
      capturedBodyEvents.length,
      0,
      'Body should not be captured when networkBodyCaptureEnabled is false'
    )

    uninstallFetchCapture()
  })

  test('gasoline server requests are not captured', async () => {
    const { installFetchCapture, uninstallFetchCapture } = await import('../../extension/inject/observers.js')
    const { setNetworkBodyCaptureEnabled } = await import('../../extension/lib/net/network.js')

    setNetworkBodyCaptureEnabled(true)

    const mockResponse = createMockResponse({ status: 200, body: '{}' })
    globalThis.window.fetch = mock.fn(() => Promise.resolve(mockResponse))

    installFetchCapture()

    // These are gasoline server URLs - should NOT be captured
    await globalThis.window.fetch('http://localhost:7890/logs')
    await globalThis.window.fetch('http://127.0.0.1:7890/health')

    await new Promise((r) => setTimeout(r, 50))

    assert.strictEqual(
      capturedBodyEvents.length,
      0,
      'Kaboom server requests should not be captured to prevent infinite loops'
    )

    uninstallFetchCapture()
  })
})
