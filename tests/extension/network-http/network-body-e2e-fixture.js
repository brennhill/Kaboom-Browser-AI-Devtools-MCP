// @ts-nocheck
/**
 * @fileoverview Canonical HTTP, Headers, response, and capture fixtures for network-body E2E tests.
 */

import http from 'node:http'

const TEST_SERVER_PORT = 19891
export const TEST_SERVER_URL = `http://localhost:${TEST_SERVER_PORT}`

// Track captured network body events
export const networkBodyE2EState = {
  capturedEvents: [],
  mockWindow: null,
}

/**
 * Check if test server is running
 * @returns {Promise<boolean>}
 */
export async function isServerRunning() {
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
export function makeRequest(path, options = {}) {
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
export function createTestWindow() {
  networkBodyE2EState.capturedEvents = []
  return {
    postMessage: (data) => {
      if (data && data.type === 'kaboom_network_body') {
        networkBodyE2EState.capturedEvents.push(data.payload)
      }
    }
  }
}

// Minimal Headers polyfill
export class MockHeaders {
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
export function createMockResponse(httpRes) {
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
