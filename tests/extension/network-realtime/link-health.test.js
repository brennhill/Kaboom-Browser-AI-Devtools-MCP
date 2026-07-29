// @ts-nocheck
import { test, describe, mock, beforeEach, afterEach } from 'node:test'
import assert from 'node:assert'

/**
 * Link Health Checker tests for CORS/405 false positive fixes (#71).
 *
 * These tests validate:
 * - HEAD -> GET fallback on 405 (Method Not Allowed)
 * - HEAD -> GET fallback on status 0 (CORS opaque)
 * - CORS TypeError classified as cors_blocked (not broken)
 * - no-cors fallback for cross-origin requests proves server reachability
 * - Summary counts distinguish cors_blocked from broken
 */

// We need to set up globals before importing the module
let fetchMock
let originalFetch
let originalWindow
let originalDocument
let originalPerformance
let originalURL

beforeEach(() => {
  fetchMock = mock.fn()
  originalFetch = globalThis.fetch
  originalWindow = globalThis.window
  originalDocument = globalThis.document
  originalPerformance = globalThis.performance
  originalURL = globalThis.URL

  globalThis.fetch = fetchMock

  globalThis.window = {
    location: {
      origin: 'http://localhost:3000',
      href: 'http://localhost:3000/'
    }
  }

  globalThis.document = {
    querySelectorAll: mock.fn(() => [])
  }

  // performance.now() for timing
  let now = 0
  globalThis.performance = {
    now: mock.fn(() => {
      now += 10
      return now
    })
  }
})

afterEach(() => {
  globalThis.fetch = originalFetch
  globalThis.window = originalWindow
  globalThis.document = originalDocument
  globalThis.performance = originalPerformance
  globalThis.URL = originalURL
})

describe('link-health: checkLink fallback behavior', () => {
  test('HEAD 405 falls back to GET and returns ok for 200', async () => {
    const { checkLinkHealth } = await import('../../../extension/lib/analysis/link-health.js')

    // Set up a single link
    globalThis.document.querySelectorAll = mock.fn(() => [
      { href: 'http://localhost:3000/api/data' }
    ])

    // First call (HEAD) returns 405, second call (GET) returns 200
    let callCount = 0
    fetchMock.mock.mockImplementation((url, opts) => {
      callCount++
      if (callCount === 1) {
        assert.strictEqual(opts.method, 'HEAD')
        return Promise.resolve({
          status: 405,
          ok: false,
          redirected: false,
          url,
          headers: new Map()
        })
      }
      // Second call should be GET
      assert.strictEqual(opts.method, 'GET')
      return Promise.resolve({
        status: 200,
        ok: true,
        redirected: false,
        url,
        headers: new Map()
      })
    })

    const result = await checkLinkHealth({})
    assert.strictEqual(result.results.length, 1)
    assert.strictEqual(result.results[0].code, 'ok')
    assert.strictEqual(result.results[0].status, 200)
    assert.strictEqual(result.summary.ok, 1)
    assert.strictEqual(result.summary.broken, 0)
  })

  test('HEAD status 0 (opaque CORS) falls back to GET', async () => {
    const { checkLinkHealth } = await import('../../../extension/lib/analysis/link-health.js')

    globalThis.document.querySelectorAll = mock.fn(() => [
      { href: 'http://localhost:3000/internal-link' }
    ])

    let callCount = 0
    fetchMock.mock.mockImplementation((_url, opts) => {
      callCount++
      if (callCount === 1) {
        assert.strictEqual(opts.method, 'HEAD')
        return Promise.resolve({
          status: 0,
          ok: false,
          redirected: false,
          url: _url,
          headers: new Map()
        })
      }
      assert.strictEqual(opts.method, 'GET')
      return Promise.resolve({
        status: 200,
        ok: true,
        redirected: false,
        url: _url,
        headers: new Map()
      })
    })

    const result = await checkLinkHealth({})
    assert.strictEqual(result.results.length, 1)
    assert.strictEqual(result.results[0].code, 'ok')
    assert.strictEqual(result.results[0].status, 200)
  })

  test('fetch TypeError on external link classified as cors_blocked', async () => {
    const { checkLinkHealth } = await import('../../../extension/lib/analysis/link-health.js')

    globalThis.document.querySelectorAll = mock.fn(() => [
      { href: 'https://external.example.com/page' }
    ])

    // First call (HEAD) throws TypeError (CORS)
    // Second call (no-cors GET) returns opaque response (status 0) = reachable
    let callCount = 0
    fetchMock.mock.mockImplementation((_url, opts) => {
      callCount++
      if (callCount === 1) {
        // HEAD throws TypeError (CORS)
        return Promise.reject(new TypeError('Failed to fetch'))
      }
      if (callCount === 2) {
        // GET also throws TypeError (CORS)
        return Promise.reject(new TypeError('Failed to fetch'))
      }
      // no-cors fallback returns opaque response
      assert.strictEqual(opts.mode, 'no-cors')
      return Promise.resolve({
        status: 0,
        ok: false,
        type: 'opaque',
        redirected: false,
        url: _url,
        headers: new Map()
      })
    })

    const result = await checkLinkHealth({})
    assert.strictEqual(result.results.length, 1)
    assert.strictEqual(result.results[0].code, 'cors_blocked')
    assert.notStrictEqual(result.results[0].code, 'broken')
    assert.strictEqual(result.results[0].isExternal, true)
    assert.strictEqual(result.results[0].needsServerVerification, true)
    assert.strictEqual(result.summary.corsBlocked, 1)
    assert.strictEqual(result.summary.broken, 0)
  })

  test('fetch TypeError on external link with no-cors also failing = unknown', async () => {
    const { checkLinkHealth } = await import('../../../extension/lib/analysis/link-health.js')

    globalThis.document.querySelectorAll = mock.fn(() => [
      { href: 'https://unreachable.example.com/page' }
    ])

    // All attempts fail
    fetchMock.mock.mockImplementation(() => {
      return Promise.reject(new TypeError('Failed to fetch'))
    })

    const result = await checkLinkHealth({})
    assert.strictEqual(result.results.length, 1)
    // When even no-cors fails, it's truly unreachable
    assert.strictEqual(result.results[0].code, 'cors_blocked')
    assert.strictEqual(result.results[0].isExternal, true)
  })

  test('genuine 404 on same-origin link classified as broken', async () => {
    const { checkLinkHealth } = await import('../../../extension/lib/analysis/link-health.js')

    globalThis.document.querySelectorAll = mock.fn(() => [
      { href: 'http://localhost:3000/missing-page' }
    ])

    fetchMock.mock.mockImplementation((_url, opts) => {
      if (opts.method === 'HEAD') {
        return Promise.resolve({
          status: 404,
          ok: false,
          redirected: false,
          url: _url,
          headers: new Map()
        })
      }
      // Should not reach here since 404 is not 405/0
      throw new Error('unexpected GET fallback')
    })

    const result = await checkLinkHealth({})
    assert.strictEqual(result.results.length, 1)
    assert.strictEqual(result.results[0].code, 'broken')
    assert.strictEqual(result.results[0].status, 404)
    assert.strictEqual(result.summary.broken, 1)
  })

  test('timeout is still classified as timeout', async () => {
    const { checkLinkHealth } = await import('../../../extension/lib/analysis/link-health.js')

    globalThis.document.querySelectorAll = mock.fn(() => [
      { href: 'http://localhost:3000/slow' }
    ])

    fetchMock.mock.mockImplementation(() => {
      const err = new DOMException('The operation was aborted.', 'AbortError')
      return Promise.reject(err)
    })

    const result = await checkLinkHealth({ timeout_ms: 100 })
    assert.strictEqual(result.results.length, 1)
    assert.strictEqual(result.results[0].code, 'timeout')
    assert.strictEqual(result.summary.timeout, 1)
  })

  test('summary correctly counts cors_blocked and needsServerVerification', async () => {
    const { checkLinkHealth } = await import('../../../extension/lib/analysis/link-health.js')

    globalThis.document.querySelectorAll = mock.fn(() => [
      { href: 'http://localhost:3000/ok-page' },
      { href: 'https://external1.com/page' },
      { href: 'https://external2.com/page' },
      { href: 'http://localhost:3000/broken-page' }
    ])

    fetchMock.mock.mockImplementation((url, opts) => {
      if (url === 'http://localhost:3000/ok-page') {
        return Promise.resolve({
          status: 200, ok: true, redirected: false, url, headers: new Map()
        })
      }
      if (url === 'http://localhost:3000/broken-page') {
        return Promise.resolve({
          status: 404, ok: false, redirected: false, url, headers: new Map()
        })
      }
      // External links fail with CORS
      if (opts.mode === 'no-cors') {
        return Promise.resolve({
          status: 0, ok: false, type: 'opaque', redirected: false, url, headers: new Map()
        })
      }
      return Promise.reject(new TypeError('Failed to fetch'))
    })

    const result = await checkLinkHealth({})
    assert.strictEqual(result.summary.totalLinks, 4)
    assert.strictEqual(result.summary.ok, 1)
    assert.strictEqual(result.summary.broken, 1)
    assert.strictEqual(result.summary.corsBlocked, 2)
    assert.strictEqual(result.summary.needsServerVerification, 2)
  })

  test('HEAD 405 on external link falls back to GET then to no-cors', async () => {
    const { checkLinkHealth } = await import('../../../extension/lib/analysis/link-health.js')

    globalThis.document.querySelectorAll = mock.fn(() => [
      { href: 'https://external.example.com/api' }
    ])

    let callCount = 0
    fetchMock.mock.mockImplementation((_url, opts) => {
      callCount++
      if (callCount === 1) {
        // HEAD returns 405
        return Promise.resolve({
          status: 405, ok: false, redirected: false, url: _url, headers: new Map()
        })
      }
      if (callCount === 2) {
        // GET throws CORS error
        return Promise.reject(new TypeError('Failed to fetch'))
      }
      // no-cors fallback succeeds (opaque response proves reachability)
      assert.strictEqual(opts.mode, 'no-cors')
      return Promise.resolve({
        status: 0, ok: false, type: 'opaque', redirected: false, url: _url, headers: new Map()
      })
    })

    const result = await checkLinkHealth({})
    assert.strictEqual(result.results.length, 1)
    assert.strictEqual(result.results[0].code, 'cors_blocked')
    assert.strictEqual(result.results[0].needsServerVerification, true)
  })

  test('domain filter limits checks to matching host and subdomains', async () => {
    const { checkLinkHealth } = await import('../../../extension/lib/analysis/link-health.js')

    globalThis.document.querySelectorAll = mock.fn(() => [
      { href: 'https://example.com/home' },
      { href: 'https://api.example.com/v1' },
      { href: 'https://other.example.org/' }
    ])

    fetchMock.mock.mockImplementation((url) =>
      Promise.resolve({
        status: 200,
        ok: true,
        redirected: false,
        url,
        headers: new Map()
      })
    )

    const result = await checkLinkHealth({ domain: 'example.com' })
    assert.strictEqual(result.summary.totalLinks, 2)
    assert.strictEqual(result.results.length, 2)
    assert.ok(result.results.every((r) => r.url.includes('example.com')))
  })

  test('domain filter accepts full URL values', async () => {
    const { checkLinkHealth } = await import('../../../extension/lib/analysis/link-health.js')

    globalThis.document.querySelectorAll = mock.fn(() => [
      { href: 'https://example.com/home' },
      { href: 'https://blog.example.com/post' },
      { href: 'https://other.test/path' }
    ])

    fetchMock.mock.mockImplementation((url) =>
      Promise.resolve({
        status: 200,
        ok: true,
        redirected: false,
        url,
        headers: new Map()
      })
    )

    const result = await checkLinkHealth({ domain: 'https://example.com/some/path' })
    assert.strictEqual(result.summary.totalLinks, 2)
    assert.strictEqual(result.results.length, 2)
  })
})
