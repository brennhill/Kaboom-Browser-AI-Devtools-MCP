// @ts-nocheck
/**
 * @fileoverview XMLHttpRequest network-body capture, filtering, redaction, and restore tests.
 */

import { test, describe, beforeEach, afterEach } from 'node:test'
import assert from 'node:assert'
import { createMockWindow } from '../shared/helpers.js'
import { MockHeaders } from './network-bodies-fixture.js'

/**
 * Creates a mock XMLHttpRequest constructor for testing.
 * Each instance tracks open/send calls and supports firing load events.
 */
function createMockXMLHttpRequest() {
  const instances = []

  class MockXMLHttpRequest {
    constructor() {
      this.readyState = 0
      this.status = 200
      this.responseType = ''
      this._responseText = '{}'
      this._headers = new Map([['content-type', 'application/json']])
      this._listeners = {}
      instances.push(this)
    }
    open(method, url) {
      this._method = method
      this._url = url
    }
    send(body) {
      this._body = body
    }
    addEventListener(event, handler) {
      if (!this._listeners[event]) this._listeners[event] = []
      this._listeners[event].push(handler)
    }
    getResponseHeader(name) {
      return this._headers.get(name.toLowerCase()) || null
    }
    get responseText() {
      return this._responseText
    }
    // Test helper: fire the 'load' event
    _fireLoad() {
      if (this._listeners['load']) {
        this._listeners['load'].forEach((h) => h.call(this))
      }
    }
  }

  MockXMLHttpRequest.instances = instances
  return MockXMLHttpRequest
}

/* global XMLHttpRequest */
describe('Network Body Capture - XHR Wrapper', () => {
  let originalWindow
  let originalXMLHttpRequest

  beforeEach(() => {
    originalWindow = globalThis.window
    originalXMLHttpRequest = globalThis.XMLHttpRequest
    globalThis.window = createMockWindow({ withFetch: true })
    globalThis.Headers = MockHeaders
  })

  afterEach(async () => {
    // Clean up: unwrap XHR
    const { unwrapXHR } = await import('../../../extension/lib/net/network.js')
    unwrapXHR()
    globalThis.window = originalWindow
    globalThis.XMLHttpRequest = originalXMLHttpRequest
  })

  test('XHR body event has spec-compliant shape', async () => {
    const MockXHR = createMockXMLHttpRequest()
    globalThis.XMLHttpRequest = MockXHR

    const { wrapXHRWithBodies } = await import('../../../extension/lib/net/network.js')
    wrapXHRWithBodies()

    const xhr = new XMLHttpRequest()
    xhr.open('POST', '/api/users')
    xhr._responseText = '{"id":1}'
    xhr.status = 201
    xhr._headers = new Map([['content-type', 'application/json']])
    xhr.send('{"name":"Alice"}')
    xhr._fireLoad()

    await new Promise((r) => setTimeout(r, 10))

    const calls = globalThis.window.postMessage.mock.calls
    const bodyEvent = calls.find((c) => c.arguments[0].type === 'kaboom_network_body')
    assert.ok(bodyEvent, 'Expected GASOLINE_NETWORK_BODY event from XHR')
    const payload = bodyEvent.arguments[0].payload

    assert.ok('method' in payload, 'missing: method')
    assert.ok('url' in payload, 'missing: url')
    assert.ok('status' in payload, 'missing: status')
    assert.ok('content_type' in payload, 'missing: content_type')
    assert.ok('duration' in payload, 'missing: duration')

    assert.strictEqual(payload.method, 'POST')
    assert.strictEqual(payload.status, 201)
    assert.strictEqual(payload.content_type, 'application/json')
    assert.strictEqual(payload.response_body, '{"id":1}')
    assert.strictEqual(payload.request_body, '{"name":"Alice"}')
  })

  test('XHR captures GET request', async () => {
    const MockXHR = createMockXMLHttpRequest()
    globalThis.XMLHttpRequest = MockXHR

    const { wrapXHRWithBodies } = await import('../../../extension/lib/net/network.js')
    wrapXHRWithBodies()

    const xhr = new XMLHttpRequest()
    xhr.open('GET', '/api/data')
    xhr._responseText = '{"items":[1,2,3]}'
    xhr.status = 200
    xhr.send()
    xhr._fireLoad()

    await new Promise((r) => setTimeout(r, 10))

    const calls = globalThis.window.postMessage.mock.calls
    const bodyEvent = calls.find((c) => c.arguments[0].type === 'kaboom_network_body')
    assert.ok(bodyEvent, 'Expected GASOLINE_NETWORK_BODY event')
    assert.strictEqual(bodyEvent.arguments[0].payload.method, 'GET')
    assert.strictEqual(bodyEvent.arguments[0].payload.response_body, '{"items":[1,2,3]}')
  })

  test('XHR does not capture gasoline server URLs', async () => {
    const MockXHR = createMockXMLHttpRequest()
    globalThis.XMLHttpRequest = MockXHR

    const { wrapXHRWithBodies } = await import('../../../extension/lib/net/network.js')
    wrapXHRWithBodies()

    const xhr = new XMLHttpRequest()
    xhr.open('GET', 'http://localhost:7890/logs')
    xhr.send()
    xhr._fireLoad()

    await new Promise((r) => setTimeout(r, 10))

    const calls = globalThis.window.postMessage.mock.calls
    const bodyEvent = calls.find((c) => c.arguments[0].type === 'kaboom_network_body')
    assert.ok(!bodyEvent, 'Should not capture gasoline server XHR')
  })

  test('XHR does not capture binary content types', async () => {
    const MockXHR = createMockXMLHttpRequest()
    globalThis.XMLHttpRequest = MockXHR

    const { wrapXHRWithBodies } = await import('../../../extension/lib/net/network.js')
    wrapXHRWithBodies()

    const xhr = new XMLHttpRequest()
    xhr.open('GET', '/image.png')
    xhr._headers = new Map([['content-type', 'image/png']])
    xhr._responseText = 'binary data'
    xhr.send()
    xhr._fireLoad()

    await new Promise((r) => setTimeout(r, 10))

    const calls = globalThis.window.postMessage.mock.calls
    const bodyEvent = calls.find((c) => c.arguments[0].type === 'kaboom_network_body')
    assert.ok(!bodyEvent, 'Should not capture binary content type XHR')
  })

  test('XHR skips non-text responseType', async () => {
    const MockXHR = createMockXMLHttpRequest()
    globalThis.XMLHttpRequest = MockXHR

    const { wrapXHRWithBodies } = await import('../../../extension/lib/net/network.js')
    wrapXHRWithBodies()

    const xhr = new XMLHttpRequest()
    xhr.open('GET', '/api/binary')
    xhr.responseType = 'arraybuffer'
    xhr.send()
    xhr._fireLoad()

    await new Promise((r) => setTimeout(r, 10))

    const calls = globalThis.window.postMessage.mock.calls
    const bodyEvent = calls.find((c) => c.arguments[0].type === 'kaboom_network_body')
    assert.ok(!bodyEvent, 'Should not capture arraybuffer responseType')
  })

  test('XHR redacts auth endpoint bodies', async () => {
    const MockXHR = createMockXMLHttpRequest()
    globalThis.XMLHttpRequest = MockXHR

    const { wrapXHRWithBodies } = await import('../../../extension/lib/net/network.js')
    wrapXHRWithBodies()

    const xhr = new XMLHttpRequest()
    xhr.open('POST', '/auth/login')
    xhr._responseText = '{"token":"secret"}'
    xhr.status = 200
    xhr.send('{"password":"hunter2"}')
    xhr._fireLoad()

    await new Promise((r) => setTimeout(r, 10))

    const calls = globalThis.window.postMessage.mock.calls
    const bodyEvent = calls.find((c) => c.arguments[0].type === 'kaboom_network_body')
    assert.ok(bodyEvent, 'Expected network body event for auth endpoint')
    const payload = bodyEvent.arguments[0].payload
    assert.strictEqual(payload.request_body, '[REDACTED: auth endpoint]')
  })

  test('unwrapXHR restores originals', async () => {
    const MockXHR = createMockXMLHttpRequest()
    globalThis.XMLHttpRequest = MockXHR
    const originalOpen = MockXHR.prototype.open
    const originalSend = MockXHR.prototype.send

    const { wrapXHRWithBodies, unwrapXHR } = await import('../../../extension/lib/net/network.js')
    wrapXHRWithBodies()

    // Verify they're wrapped
    assert.notStrictEqual(MockXHR.prototype.open, originalOpen)
    assert.notStrictEqual(MockXHR.prototype.send, originalSend)

    // Unwrap
    unwrapXHR()

    // Verify they're restored
    assert.strictEqual(MockXHR.prototype.open, originalOpen)
    assert.strictEqual(MockXHR.prototype.send, originalSend)
  })
})
