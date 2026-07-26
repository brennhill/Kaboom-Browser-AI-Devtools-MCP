// @ts-nocheck
/**
 * @fileoverview network-body-e2e.test.js — E2E tests for network body capture.
 * Tests actual fetch behavior with a real HTTP server for edge cases:
 * large body truncation, binary content handling, header sanitization,
 * streaming responses, and error response capture.
 *
 * Requires test server: node extension-tests/fixtures/test-server.mjs
 * Tests skip gracefully if the server is not running.
 */

import { test, describe, beforeEach, afterEach, before } from 'node:test'
import assert from 'node:assert'
import http from 'node:http'

const TEST_SERVER_PORT = 19891
const TEST_SERVER_URL = `http://localhost:${TEST_SERVER_PORT}`

// Track captured network body events
let capturedEvents = []
let mockWindow

/**
 * Check if test server is running
 * @returns {Promise<boolean>}
 */
async function isServerRunning() {
  return new Promise((resolve) => {
    const req = http.get(`${TEST_SERVER_URL}/health`, (res) => {
      let data = ''
      res.on('data', (chunk) => (data += chunk))
      res.on('end', () => {
        try {
          const json = JSON.parse(data)
          resolve(json.status === 'ok')
        } catch {
          resolve(false)
        }
      })
    })
    req.on('error', () => resolve(false))
    req.setTimeout(1000, () => {
      req.destroy()
      resolve(false)
    })
  })
}

/**
 * Make an HTTP request using Node's http module
 * @param {string} path - URL path
 * @param {Object} options - Request options
 * @returns {Promise<{status: number, headers: Object, body: string|Buffer}>}
 */
function makeRequest(path, options = {}) {
  return new Promise((resolve, reject) => {
    const url = new URL(path, TEST_SERVER_URL)
    const reqOptions = {
      hostname: url.hostname,
      port: url.port,
      path: url.pathname,
      method: options.method || 'GET',
      headers: options.headers || {}
    }

    const req = http.request(reqOptions, (res) => {
      const chunks = []
      res.on('data', (chunk) => chunks.push(chunk))
      res.on('end', () => {
        const body = Buffer.concat(chunks)
        resolve({
          status: res.statusCode,
          headers: res.headers,
          body: options.binary ? body : body.toString()
        })
      })
    })

    req.on('error', reject)

    if (options.body) {
      req.write(options.body)
    }

    req.end()
  })
}

/**
 * Create a mock window with postMessage capture
 */
function createTestWindow() {
  capturedEvents = []
  return {
    postMessage: (data) => {
      if (data && data.type === 'kaboom_network_body') {
        capturedEvents.push(data.payload)
      }
    }
  }
}

// Minimal Headers polyfill
class MockHeaders {
  constructor(init) {
    this._map = new Map()
    if (init) {
      if (init instanceof MockHeaders) {
        init._map.forEach((v, k) => this._map.set(k, v))
      } else if (typeof init === 'object') {
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
 * Create a mock Response object from http response
 */
function createMockResponse(httpRes) {
  const headers = new MockHeaders()
  Object.entries(httpRes.headers).forEach(([k, v]) => headers.set(k, v))

  return {
    ok: httpRes.status >= 200 && httpRes.status < 300,
    status: httpRes.status,
    statusText: http.STATUS_CODES[httpRes.status],
    headers,
    clone: function () {
      return {
        ...this,
        text: () => Promise.resolve(typeof httpRes.body === 'string' ? httpRes.body : httpRes.body.toString()),
        blob: () =>
          Promise.resolve({
            size: Buffer.byteLength(httpRes.body),
            type: headers.get('content-type') || ''
          }),
        headers: this.headers
      }
    }
  }
}

describe('Network Body E2E Tests', async () => {
  let serverAvailable = false
  let originalWindow
  let originalHeaders

  before(async () => {
    serverAvailable = await isServerRunning()
    if (!serverAvailable) {
      console.log('Test server not running. Skipping E2E tests.')
      console.log('Start server with: node extension-tests/fixtures/test-server.mjs')
    }
  })

  beforeEach(() => {
    originalWindow = globalThis.window
    originalHeaders = globalThis.Headers
    globalThis.Headers = MockHeaders
    mockWindow = createTestWindow()
    globalThis.window = mockWindow
    capturedEvents = []
  })

  afterEach(() => {
    globalThis.window = originalWindow
    globalThis.Headers = originalHeaders
  })

  describe('Large Body Truncation', () => {
    test('should truncate large JSON response to 16KB limit', async (t) => {
      if (!serverAvailable) {
        t.skip('Test server not running')
        return
      }

      const { truncateResponseBody } = await import('../../extension/lib/net/network.js')

      // Make actual request to get large JSON
      const res = await makeRequest('/large-json')
      assert.strictEqual(res.status, 200)

      // Verify the response is actually large
      assert.ok(res.body.length > 16384, `Expected large body, got ${res.body.length} bytes`)

      // Test truncation
      const result = truncateResponseBody(res.body)
      assert.strictEqual(result.body.length, 16384)
      assert.strictEqual(result.truncated, true)
    })

    test('should truncate large text response to 16KB limit', async (t) => {
      if (!serverAvailable) {
        t.skip('Test server not running')
        return
      }

      const { truncateResponseBody } = await import('../../extension/lib/net/network.js')

      const res = await makeRequest('/large-text')
      assert.strictEqual(res.status, 200)
      assert.ok(res.body.length > 16384, `Expected large body, got ${res.body.length} bytes`)

      const result = truncateResponseBody(res.body)
      assert.strictEqual(result.body.length, 16384)
      assert.strictEqual(result.truncated, true)
    })

    test('should not truncate small responses', async (t) => {
      if (!serverAvailable) {
        t.skip('Test server not running')
        return
      }

      const { truncateResponseBody } = await import('../../extension/lib/net/network.js')

      const res = await makeRequest('/json')
      assert.strictEqual(res.status, 200)
      assert.ok(res.body.length < 16384)

      const result = truncateResponseBody(res.body)
      assert.strictEqual(result.body, res.body)
      assert.strictEqual(result.truncated, false)
    })

    test('should truncate large request body to 8KB limit', async (t) => {
      if (!serverAvailable) {
        t.skip('Test server not running')
        return
      }

      const { truncateRequestBody } = await import('../../extension/lib/net/network.js')

      // Create large request body
      const largeBody = JSON.stringify({ data: 'x'.repeat(10000) })
      assert.ok(largeBody.length > 8192)

      const result = truncateRequestBody(largeBody)
      assert.strictEqual(result.body.length, 8192)
      assert.strictEqual(result.truncated, true)
    })
  })

  describe('Binary Content Handling', () => {
    test('should detect binary content type and return size placeholder', async (t) => {
      if (!serverAvailable) {
        t.skip('Test server not running')
        return
      }

      const { readResponseBody } = await import('../../extension/lib/net/network.js')

      const res = await makeRequest('/binary', { binary: true })
      assert.strictEqual(res.status, 200)
      assert.strictEqual(res.headers['content-type'], 'image/png')

      // Create mock response for readResponseBody
      const mockRes = createMockResponse({ ...res, body: res.body })
      const body = await readResponseBody(mockRes.clone())

      assert.ok(body.includes('[Binary:'), `Expected binary placeholder, got: ${body}`)
      assert.ok(body.includes('bytes'), 'Expected size in placeholder')
      assert.ok(body.includes('image/png'), 'Expected content type in placeholder')
    })

    test('should preserve binary data integrity', async (t) => {
      if (!serverAvailable) {
        t.skip('Test server not running')
        return
      }

      const res = await makeRequest('/binary', { binary: true })
      assert.strictEqual(res.status, 200)

      // Verify PNG magic bytes are intact
      const buffer = res.body
      assert.strictEqual(buffer[0], 0x89)
      assert.strictEqual(buffer[1], 0x50)
      assert.strictEqual(buffer[2], 0x4e)
      assert.strictEqual(buffer[3], 0x47)
    })

    test('should handle various binary content types', async (t) => {
      if (!serverAvailable) {
        t.skip('Test server not running')
        return
      }

      const { BINARY_CONTENT_TYPES } = await import('../../extension/lib/constants.js')

      const binaryTypes = [
        'image/png',
        'image/jpeg',
        'image/gif',
        'video/mp4',
        'audio/mpeg',
        'font/woff2',
        'application/wasm',
        'application/octet-stream',
        'application/zip',
        'application/pdf'
      ]

      for (const type of binaryTypes) {
        assert.ok(BINARY_CONTENT_TYPES.test(type), `Expected ${type} to match binary pattern`)
      }
    })

    test('should not treat text content as binary', async (t) => {
      if (!serverAvailable) {
        t.skip('Test server not running')
        return
      }

      const { BINARY_CONTENT_TYPES } = await import('../../extension/lib/constants.js')

      const textTypes = [
        'text/plain',
        'text/html',
        'application/json',
        'application/xml',
        'text/css',
        'text/javascript'
      ]

      for (const type of textTypes) {
        assert.ok(!BINARY_CONTENT_TYPES.test(type), `Expected ${type} to NOT match binary pattern`)
      }
    })
  })

  describe('Header Sanitization', () => {
    test('should strip Authorization header', async (t) => {
      if (!serverAvailable) {
        t.skip('Test server not running')
        return
      }

      const { sanitizeHeaders } = await import('../../extension/lib/net/network.js')

      const headers = {
        Authorization: 'Bearer secret-token-123',
        'Content-Type': 'application/json',
        Accept: 'application/json'
      }

      const sanitized = sanitizeHeaders(headers)

      assert.strictEqual(sanitized['Authorization'], undefined)
      assert.strictEqual(sanitized['Content-Type'], 'application/json')
      assert.strictEqual(sanitized['Accept'], 'application/json')
    })

    test('should strip Cookie and Set-Cookie headers', async (t) => {
      if (!serverAvailable) {
        t.skip('Test server not running')
        return
      }

      const { sanitizeHeaders } = await import('../../extension/lib/net/network.js')

      const headers = {
        Cookie: 'session=abc123; token=xyz',
        'Set-Cookie': 'session=new-value; Path=/',
        'Content-Type': 'text/html'
      }

      const sanitized = sanitizeHeaders(headers)

      assert.strictEqual(sanitized['Cookie'], undefined)
      assert.strictEqual(sanitized['Set-Cookie'], undefined)
      assert.strictEqual(sanitized['Content-Type'], 'text/html')
    })

    test('should strip X-API-Key and similar headers', async (t) => {
      if (!serverAvailable) {
        t.skip('Test server not running')
        return
      }

      const { sanitizeHeaders } = await import('../../extension/lib/net/network.js')

      const headers = {
        'X-API-Key': 'sk_live_abc123',
        'X-Auth-Token': 'token-value',
        'X-Secret-Key': 'secret-value',
        'X-Custom-Header': 'safe-value'
      }

      const sanitized = sanitizeHeaders(headers)

      assert.strictEqual(sanitized['X-API-Key'], undefined)
      assert.strictEqual(sanitized['X-Auth-Token'], undefined)
      assert.strictEqual(sanitized['X-Secret-Key'], undefined)
      assert.strictEqual(sanitized['X-Custom-Header'], 'safe-value')
    })

    test('should be case-insensitive for sensitive patterns', async (t) => {
      if (!serverAvailable) {
        t.skip('Test server not running')
        return
      }

      const { sanitizeHeaders } = await import('../../extension/lib/net/network.js')

      const headers = {
        authorization: 'Bearer xyz',
        AUTHORIZATION: 'Bearer abc',
        'x-auth-TOKEN': 'value',
        'X-PASSWORD': 'secret'
      }

      const sanitized = sanitizeHeaders(headers)

      assert.strictEqual(Object.keys(sanitized).length, 0, 'All headers should be stripped')
    })

    test('should handle Headers object', async (t) => {
      if (!serverAvailable) {
        t.skip('Test server not running')
        return
      }

      const { sanitizeHeaders } = await import('../../extension/lib/net/network.js')

      const headers = new MockHeaders({
        authorization: 'Bearer token',
        'content-type': 'application/json'
      })

      const sanitized = sanitizeHeaders(headers)

      assert.strictEqual(sanitized['authorization'], undefined)
      assert.strictEqual(sanitized['content-type'], 'application/json')
    })

    test('should handle null and undefined headers', async (t) => {
      if (!serverAvailable) {
        t.skip('Test server not running')
        return
      }

      const { sanitizeHeaders } = await import('../../extension/lib/net/network.js')

      assert.deepStrictEqual(sanitizeHeaders(null), {})
      assert.deepStrictEqual(sanitizeHeaders(undefined), {})
    })
  })

  describe('POST Body Capture', () => {
    test('should echo request body correctly', async (t) => {
      if (!serverAvailable) {
        t.skip('Test server not running')
        return
      }

      const requestBody = JSON.stringify({ name: 'Alice', email: 'alice@example.com' })

      const res = await makeRequest('/echo', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: requestBody
      })

      assert.strictEqual(res.status, 200)
      assert.strictEqual(res.body, requestBody)
    })

    test('should capture request body in wrapped fetch', async (t) => {
      if (!serverAvailable) {
        t.skip('Test server not running')
        return
      }

      const { wrapFetchWithBodies, setNetworkBodyCaptureEnabled } = await import('../../extension/lib/net/network.js')

      // Enable body capture
      setNetworkBodyCaptureEnabled(true)

      // Create a mock fetch that returns our test response
      const httpRes = await makeRequest('/echo', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: '{"test":"data"}'
      })

      const mockFetch = async () => createMockResponse(httpRes)
      const wrappedFetch = wrapFetchWithBodies(mockFetch)

      await wrappedFetch('/echo', {
        method: 'POST',
        body: '{"test":"data"}'
      })

      // Wait for async body capture
      await new Promise((r) => setTimeout(r, 50))

      const event = capturedEvents.find((e) => e.method === 'POST')
      assert.ok(event, 'Expected POST event to be captured')
      assert.strictEqual(event.request_body, '{"test":"data"}')
    })
  })

  describe('Content-Type Detection', () => {
    test('should detect JSON content type', async (t) => {
      if (!serverAvailable) {
        t.skip('Test server not running')
        return
      }

      const res = await makeRequest('/json')
      assert.strictEqual(res.headers['content-type'], 'application/json')

      // Verify JSON is parseable
      const data = JSON.parse(res.body)
      assert.ok(data.id)
      assert.ok(data.name)
    })

    test('should detect HTML content type', async (t) => {
      if (!serverAvailable) {
        t.skip('Test server not running')
        return
      }

      const res = await makeRequest('/html')
      assert.strictEqual(res.headers['content-type'], 'text/html')
      assert.ok(res.body.includes('<html>'))
    })

    test('should detect text content type', async (t) => {
      if (!serverAvailable) {
        t.skip('Test server not running')
        return
      }

      const res = await makeRequest('/large-text')
      assert.strictEqual(res.headers['content-type'], 'text/plain')
    })

    test('should detect binary content type', async (t) => {
      if (!serverAvailable) {
        t.skip('Test server not running')
        return
      }

      const res = await makeRequest('/binary', { binary: true })
      assert.strictEqual(res.headers['content-type'], 'image/png')
    })
  })

  describe('Streaming Response', () => {
    test('should receive all chunks from streaming response', async (t) => {
      if (!serverAvailable) {
        t.skip('Test server not running')
        return
      }

      const res = await makeRequest('/streaming')
      assert.strictEqual(res.status, 200)

      // Verify all chunks were received
      for (let i = 0; i < 5; i++) {
        assert.ok(res.body.includes(`Chunk ${i}`), `Missing chunk ${i}`)
      }
    })

    test('should have chunked transfer encoding', async (t) => {
      if (!serverAvailable) {
        t.skip('Test server not running')
        return
      }

      const res = await makeRequest('/streaming')
      assert.strictEqual(res.headers['transfer-encoding'], 'chunked')
    })
  })

  describe('Timeout Handling', () => {
    test('should timeout body read after configured limit', async (t) => {
      if (!serverAvailable) {
        t.skip('Test server not running')
        return
      }

      const { readResponseBodyWithTimeout } = await import('../../extension/lib/net/network.js')

      // Create a slow response that takes longer than timeout
      const slowResponse = {
        headers: new MockHeaders({ 'content-type': 'application/json' }),
        text: () => new Promise((resolve) => setTimeout(() => resolve('{"slow":true}'), 100))
      }

      // Use 10ms timeout (response takes 100ms)
      const body = await readResponseBodyWithTimeout(slowResponse, 10)

      assert.ok(body.includes('[Skipped:'), `Expected timeout message, got: ${body}`)
      assert.ok(body.includes('timeout'), 'Expected timeout in message')
    })

    test('should complete fast reads before timeout', async (t) => {
      if (!serverAvailable) {
        t.skip('Test server not running')
        return
      }

      const { readResponseBodyWithTimeout } = await import('../../extension/lib/net/network.js')

      const fastResponse = {
        headers: new MockHeaders({ 'content-type': 'application/json' }),
        text: () => Promise.resolve('{"fast":true}')
      }

      // Use 100ms timeout (response is instant)
      const body = await readResponseBodyWithTimeout(fastResponse, 100)

      assert.strictEqual(body, '{"fast":true}')
    })
  })

  describe('Error Response Capture', () => {
    test('should capture 400 Bad Request body', async (t) => {
      if (!serverAvailable) {
        t.skip('Test server not running')
        return
      }

      const res = await makeRequest('/error-400')
      assert.strictEqual(res.status, 400)

      const data = JSON.parse(res.body)
      assert.strictEqual(data.error, 'Bad Request')
      assert.ok(data.message)
    })

    test('should capture 404 Not Found body', async (t) => {
      if (!serverAvailable) {
        t.skip('Test server not running')
        return
      }

      const res = await makeRequest('/error-404')
      assert.strictEqual(res.status, 404)

      const data = JSON.parse(res.body)
      assert.strictEqual(data.error, 'Not Found')
    })

    test('should capture 500 Internal Server Error body', async (t) => {
      if (!serverAvailable) {
        t.skip('Test server not running')
        return
      }

      const res = await makeRequest('/error-500')
      assert.strictEqual(res.status, 500)

      const data = JSON.parse(res.body)
      assert.strictEqual(data.error, 'Internal Server Error')
    })

    test('should capture error responses in wrapped fetch', async (t) => {
      if (!serverAvailable) {
        t.skip('Test server not running')
        return
      }

      const { wrapFetchWithBodies, setNetworkBodyCaptureEnabled } = await import('../../extension/lib/net/network.js')

      setNetworkBodyCaptureEnabled(true)

      const httpRes = await makeRequest('/error-500')
      const mockFetch = async () => createMockResponse(httpRes)
      const wrappedFetch = wrapFetchWithBodies(mockFetch)

      await wrappedFetch('/error-500')
      await new Promise((r) => setTimeout(r, 50))

      const event = capturedEvents.find((e) => e.status === 500)
      assert.ok(event, 'Expected 500 error event')
      assert.ok(event.response_body.includes('Internal Server Error'))
    })
  })

  describe('URL Filtering', () => {
    test('should not capture requests to gasoline server', async (t) => {
      if (!serverAvailable) {
        t.skip('Test server not running')
        return
      }

      const { shouldCaptureUrl } = await import('../../extension/lib/net/network.js')

      assert.strictEqual(shouldCaptureUrl('http://localhost:7890/logs'), false)
      assert.strictEqual(shouldCaptureUrl('http://127.0.0.1:7890/health'), false)
    })

    test('should capture normal URLs', async (t) => {
      if (!serverAvailable) {
        t.skip('Test server not running')
        return
      }

      const { shouldCaptureUrl } = await import('../../extension/lib/net/network.js')

      assert.strictEqual(shouldCaptureUrl(`${TEST_SERVER_URL}/json`), true)
      assert.strictEqual(shouldCaptureUrl('/api/users'), true)
      assert.strictEqual(shouldCaptureUrl('https://api.example.com/data'), true)
    })

    test('should not capture chrome-extension URLs', async (t) => {
      if (!serverAvailable) {
        t.skip('Test server not running')
        return
      }

      const { shouldCaptureUrl } = await import('../../extension/lib/net/network.js')

      assert.strictEqual(shouldCaptureUrl('chrome-extension://abc123/lib/axe.min.js'), false)
    })
  })

  describe('wrapFetchWithBodies Integration', () => {
    test('should capture full request/response cycle', async (t) => {
      if (!serverAvailable) {
        t.skip('Test server not running')
        return
      }

      const { wrapFetchWithBodies, setNetworkBodyCaptureEnabled } = await import('../../extension/lib/net/network.js')

      setNetworkBodyCaptureEnabled(true)

      const httpRes = await makeRequest('/json')
      const mockFetch = async () => createMockResponse(httpRes)
      const wrappedFetch = wrapFetchWithBodies(mockFetch)

      const response = await wrappedFetch('/json')
      await new Promise((r) => setTimeout(r, 50))

      // Response should be returned immediately
      assert.ok(response.ok)
      assert.strictEqual(response.status, 200)

      // Event should be captured
      assert.ok(capturedEvents.length > 0, 'Expected captured events')
      const event = capturedEvents[0]
      assert.strictEqual(event.method, 'GET')
      assert.strictEqual(event.status, 200)
      assert.ok(event.response_body)
      assert.ok(event.duration >= 0)
    })

    test('should not block response on slow body read', async (t) => {
      if (!serverAvailable) {
        t.skip('Test server not running')
        return
      }

      const { wrapFetchWithBodies } = await import('../../extension/lib/net/network.js')

      // Create a response with a slow body read
      const slowMockResponse = {
        ok: true,
        status: 200,
        headers: new MockHeaders({ 'content-type': 'application/json' }),
        clone: () => ({
          text: () => new Promise((resolve) => setTimeout(() => resolve('{}'), 100)),
          headers: new MockHeaders({ 'content-type': 'application/json' })
        })
      }

      const mockFetch = async () => slowMockResponse
      const wrappedFetch = wrapFetchWithBodies(mockFetch)

      const start = Date.now()
      await wrappedFetch('/slow')
      const elapsed = Date.now() - start

      // Should return quickly, not wait for body read
      assert.ok(elapsed < 50, `Expected fast return, took ${elapsed}ms`)
    })

    test('should skip gasoline server URLs', async (t) => {
      if (!serverAvailable) {
        t.skip('Test server not running')
        return
      }

      const { wrapFetchWithBodies, setNetworkBodyCaptureEnabled } = await import('../../extension/lib/net/network.js')

      setNetworkBodyCaptureEnabled(true)
      capturedEvents = []

      const mockFetch = async () => createMockResponse({ status: 200, headers: {}, body: '{}' })
      const wrappedFetch = wrapFetchWithBodies(mockFetch)

      await wrappedFetch('http://localhost:7890/logs')
      await new Promise((r) => setTimeout(r, 50))

      assert.strictEqual(capturedEvents.length, 0, 'Should not capture gasoline server requests')
    })
  })
})
