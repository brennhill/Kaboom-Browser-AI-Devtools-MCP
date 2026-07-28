// @ts-nocheck
/**
 * @fileoverview inject-console-network-exceptions.test.js — Tests for console capture,
 * network capture, exception capture, and safe serialization in inject.js.
 */

import { test, describe, mock, beforeEach, afterEach } from 'node:test'
import assert from 'node:assert'
import { createMockWindow, createMockConsole } from './helpers.js'

// Define esbuild constant not available in Node test env
globalThis.__KABOOM_VERSION__ = 'test'

// Store original
let originalWindow
let originalConsole

describe('Console Capture', () => {
  beforeEach(() => {
    originalWindow = globalThis.window
    originalConsole = globalThis.console
    globalThis.window = createMockWindow({ href: 'http://localhost:3000/test', withOnerror: true })
    globalThis.console = createMockConsole()
  })

  afterEach(() => {
    globalThis.window = originalWindow
    globalThis.console = originalConsole
  })

  test('should intercept console.log', async () => {
    // Import the module to install interceptors
    const { installConsoleCapture, uninstallConsoleCapture } = await import('../../extension/lib/page/console.js')

    const originalLog = globalThis.console.log
    installConsoleCapture()

    // Call console.log
    globalThis.console.log('test message', { data: 123 })

    // Should have posted message
    assert.strictEqual(globalThis.window.postMessage.mock.calls.length, 1)

    const [message] = globalThis.window.postMessage.mock.calls[0].arguments
    assert.strictEqual(message.type, 'kaboom_log')
    assert.strictEqual(message.payload.level, 'log')
    assert.deepStrictEqual(message.payload.args, ['test message', { data: 123 }])

    // Should have called original
    assert.strictEqual(originalLog.mock.calls.length, 1)

    uninstallConsoleCapture()
  })

  test('should intercept console.error', async () => {
    const { installConsoleCapture, uninstallConsoleCapture } = await import('../../extension/lib/page/console.js')

    installConsoleCapture()
    globalThis.console.error('error message')

    const [message] = globalThis.window.postMessage.mock.calls[0].arguments
    assert.strictEqual(message.payload.level, 'error')

    uninstallConsoleCapture()
  })

  test('should intercept console.warn', async () => {
    const { installConsoleCapture, uninstallConsoleCapture } = await import('../../extension/lib/page/console.js')

    installConsoleCapture()
    globalThis.console.warn('warning message')

    const [message] = globalThis.window.postMessage.mock.calls[0].arguments
    assert.strictEqual(message.payload.level, 'warn')

    uninstallConsoleCapture()
  })

  test('should intercept console.info', async () => {
    const { installConsoleCapture, uninstallConsoleCapture } = await import('../../extension/lib/page/console.js')

    installConsoleCapture()
    globalThis.console.info('info message')

    const [message] = globalThis.window.postMessage.mock.calls[0].arguments
    assert.strictEqual(message.payload.level, 'info')

    uninstallConsoleCapture()
  })

  test('should intercept console.debug', async () => {
    const { installConsoleCapture, uninstallConsoleCapture } = await import('../../extension/lib/page/console.js')

    installConsoleCapture()
    globalThis.console.debug('debug message')

    const [message] = globalThis.window.postMessage.mock.calls[0].arguments
    assert.strictEqual(message.payload.level, 'debug')

    uninstallConsoleCapture()
  })

  test('should include page URL', async () => {
    const { installConsoleCapture, uninstallConsoleCapture } = await import('../../extension/lib/page/console.js')

    installConsoleCapture()
    globalThis.console.log('test')

    const [message] = globalThis.window.postMessage.mock.calls[0].arguments
    assert.strictEqual(message.payload.url, 'http://localhost:3000/test')

    uninstallConsoleCapture()
  })

  test('should restore original console on uninstall', async () => {
    const { installConsoleCapture, uninstallConsoleCapture } = await import('../../extension/lib/page/console.js')

    const originalLog = globalThis.console.log
    installConsoleCapture()

    // Console should be wrapped
    assert.notStrictEqual(globalThis.console.log, originalLog)

    uninstallConsoleCapture()

    // Console should be restored
    assert.strictEqual(globalThis.console.log, originalLog)
  })
})

describe('Network Capture', () => {
  beforeEach(() => {
    originalWindow = globalThis.window
    globalThis.window = createMockWindow({ href: 'http://localhost:3000/test', withOnerror: true })
  })

  afterEach(() => {
    globalThis.window = originalWindow
  })

  test('should capture fetch errors (status >= 400)', async () => {
    const { wrapFetch } = await import('../../extension/inject/observers.js')

    const mockResponse = {
      ok: false,
      status: 401,
      statusText: 'Unauthorized',
      clone: () => ({
        text: () => Promise.resolve(JSON.stringify({ error: 'Invalid credentials' }))
      })
    }

    const originalFetch = mock.fn(() => Promise.resolve(mockResponse))
    const wrappedFetch = wrapFetch(originalFetch)

    const _startTime = Date.now()
    await wrappedFetch('http://localhost:8789/auth/login', {
      method: 'POST',
      body: JSON.stringify({ email: 'test@test.com' })
    })

    // Should have posted network error
    assert.strictEqual(globalThis.window.postMessage.mock.calls.length, 1)

    const [message] = globalThis.window.postMessage.mock.calls[0].arguments
    assert.strictEqual(message.type, 'kaboom_log')
    assert.strictEqual(message.payload.type, 'network')
    assert.strictEqual(message.payload.level, 'error')
    assert.strictEqual(message.payload.status, 401)
    assert.strictEqual(message.payload.method, 'POST')
    assert.strictEqual(message.payload.url, 'http://localhost:8789/auth/login')
    assert.ok(message.payload.duration >= 0)
  })

  test('should not capture successful fetch requests', async () => {
    const { wrapFetch } = await import('../../extension/inject/observers.js')

    const mockResponse = {
      ok: true,
      status: 200
    }

    const originalFetch = mock.fn(() => Promise.resolve(mockResponse))
    const wrappedFetch = wrapFetch(originalFetch)

    await wrappedFetch('http://localhost:8789/api/data')

    // Should NOT have posted message
    assert.strictEqual(globalThis.window.postMessage.mock.calls.length, 0)
  })

  test('should capture 5xx server errors', async () => {
    const { wrapFetch } = await import('../../extension/inject/observers.js')

    const mockResponse = {
      ok: false,
      status: 500,
      statusText: 'Internal Server Error',
      clone: () => ({
        text: () => Promise.resolve('Server error')
      })
    }

    const originalFetch = mock.fn(() => Promise.resolve(mockResponse))
    const wrappedFetch = wrapFetch(originalFetch)

    await wrappedFetch('http://localhost:8789/api/data')

    const [message] = globalThis.window.postMessage.mock.calls[0].arguments
    assert.strictEqual(message.payload.status, 500)
  })

  test('should capture network failures', async () => {
    const { wrapFetch } = await import('../../extension/inject/observers.js')

    const originalFetch = mock.fn(() => Promise.reject(new Error('Failed to fetch')))
    const wrappedFetch = wrapFetch(originalFetch)

    try {
      await wrappedFetch('http://localhost:8789/api/data')
    } catch {
      // Expected
    }

    const [message] = globalThis.window.postMessage.mock.calls[0].arguments
    assert.strictEqual(message.payload.type, 'network')
    assert.strictEqual(message.payload.level, 'error')
    assert.ok(message.payload.error.includes('Failed to fetch'))
  })

  test('should exclude Authorization header from logs', async () => {
    const { wrapFetch } = await import('../../extension/inject/observers.js')

    const mockResponse = {
      ok: false,
      status: 401,
      statusText: 'Unauthorized',
      clone: () => ({
        text: () => Promise.resolve('{}')
      })
    }

    const originalFetch = mock.fn(() => Promise.resolve(mockResponse))
    const wrappedFetch = wrapFetch(originalFetch)

    await wrappedFetch('http://localhost:8789/api/data', {
      headers: {
        Authorization: 'Bearer secret-token',
        'Content-Type': 'application/json'
      }
    })

    const [message] = globalThis.window.postMessage.mock.calls[0].arguments

    // Should not include Authorization
    assert.ok(!JSON.stringify(message).includes('secret-token'))
    assert.ok(!JSON.stringify(message).includes('Authorization'))
  })

  test('should truncate large response bodies', async () => {
    const { wrapFetch } = await import('../../extension/inject/observers.js')

    const largeBody = 'x'.repeat(10000) // 10KB
    const mockResponse = {
      ok: false,
      status: 400,
      statusText: 'Bad Request',
      clone: () => ({
        text: () => Promise.resolve(largeBody)
      })
    }

    const originalFetch = mock.fn(() => Promise.resolve(mockResponse))
    const wrappedFetch = wrapFetch(originalFetch)

    await wrappedFetch('http://localhost:8789/api/data')

    const [message] = globalThis.window.postMessage.mock.calls[0].arguments
    assert.ok(message.payload.response.length < 6000) // 5KB limit + some buffer
  })
})

describe('Exception Capture', () => {
  beforeEach(() => {
    originalWindow = globalThis.window
    globalThis.window = createMockWindow({ href: 'http://localhost:3000/test', withOnerror: true })
  })

  afterEach(() => {
    globalThis.window = originalWindow
  })

  test('should capture window.onerror events', async () => {
    const { installExceptionCapture, uninstallExceptionCapture } = await import('../../extension/lib/page/exceptions.js')

    installExceptionCapture()

    // Simulate error event
    globalThis.window.onerror("Cannot read property 'x' of undefined", 'app.js', 42, 15, new Error())

    // Wait for async enrichment
    await new Promise((resolve) => setTimeout(resolve, 50))

    const calls = globalThis.window.postMessage.mock.calls
    const message = calls[calls.length - 1].arguments[0]
    assert.strictEqual(message.type, 'kaboom_log')
    assert.strictEqual(message.payload.type, 'exception')
    assert.strictEqual(message.payload.level, 'error')
    assert.strictEqual(message.payload.message, "Cannot read property 'x' of undefined")
    assert.strictEqual(message.payload.filename, 'app.js')
    assert.strictEqual(message.payload.lineno, 42)
    assert.strictEqual(message.payload.colno, 15)

    uninstallExceptionCapture()
  })

  test('should capture unhandled promise rejections', async () => {
    const { installExceptionCapture, uninstallExceptionCapture } = await import('../../extension/lib/page/exceptions.js')

    installExceptionCapture()

    // Get the handler that was registered
    const addListenerCalls = globalThis.window.addEventListener.mock.calls
    const rejectionHandler = addListenerCalls.find((call) => call.arguments[0] === 'unhandledrejection')

    assert.ok(rejectionHandler, 'Should have registered unhandledrejection handler')

    // Simulate rejection event
    const handler = rejectionHandler.arguments[1]
    handler({
      reason: new Error('Promise rejection')
    })

    // Wait for async enrichment
    await new Promise((resolve) => setTimeout(resolve, 50))

    const calls = globalThis.window.postMessage.mock.calls
    const message = calls[calls.length - 1].arguments[0]
    assert.strictEqual(message.payload.type, 'exception')
    assert.strictEqual(message.payload.level, 'error')
    assert.ok(message.payload.message.includes('Promise rejection'))

    uninstallExceptionCapture()
  })

  test('should include stack trace', async () => {
    const { installExceptionCapture, uninstallExceptionCapture } = await import('../../extension/lib/page/exceptions.js')

    installExceptionCapture()

    const error = new Error('Test error')
    error.stack = 'Error: Test error\n    at foo (app.js:42)\n    at bar (app.js:100)'

    globalThis.window.onerror('Test error', 'app.js', 42, 1, error)

    // Wait for async enrichment
    await new Promise((resolve) => setTimeout(resolve, 50))

    const calls = globalThis.window.postMessage.mock.calls
    const message = calls[calls.length - 1].arguments[0]
    assert.ok(message.payload.stack.includes('app.js:42'))
    assert.ok(message.payload.stack.includes('app.js:100'))

    uninstallExceptionCapture()
  })

  test('should handle error event without error object', async () => {
    const { installExceptionCapture, uninstallExceptionCapture } = await import('../../extension/lib/page/exceptions.js')

    installExceptionCapture()

    // Some browsers don't provide error object
    globalThis.window.onerror('Script error', '', 0, 0, null)

    // Wait for async enrichment
    await new Promise((resolve) => setTimeout(resolve, 50))

    const calls = globalThis.window.postMessage.mock.calls
    const message = calls[calls.length - 1].arguments[0]
    assert.strictEqual(message.payload.message, 'Script error')
    assert.ok(message.payload.stack === '' || message.payload.stack === undefined)

    uninstallExceptionCapture()
  })
})

describe('Safe Serialization', () => {
  test('should handle circular references in objects', async () => {
    const { safeSerialize } = await import('../../extension/lib/page/serialize.js')

    const obj = { name: 'test' }
    obj.self = obj

    const result = safeSerialize(obj)

    // Should not throw
    assert.ok(result)
    JSON.stringify(result) // Should be serializable
  })

  test('should handle undefined and null', async () => {
    const { safeSerialize } = await import('../../extension/lib/page/serialize.js')

    assert.strictEqual(safeSerialize(undefined), null) // JSON has no undefined type
    assert.strictEqual(safeSerialize(null), null)
  })

  test('should preserve primitive types', async () => {
    const { safeSerialize } = await import('../../extension/lib/page/serialize.js')

    assert.strictEqual(safeSerialize('string'), 'string')
    assert.strictEqual(safeSerialize(123), 123)
    assert.strictEqual(safeSerialize(true), true)
  })

  test('should handle Error objects', async () => {
    const { safeSerialize } = await import('../../extension/lib/page/serialize.js')

    const error = new Error('test error')
    const result = safeSerialize(error)

    assert.ok(result.message === 'test error' || result.includes('test error'))
  })

  test('should truncate strings over 10KB', async () => {
    const { safeSerialize } = await import('../../extension/lib/page/serialize.js')

    const longString = 'x'.repeat(15000)
    const result = safeSerialize(longString)

    assert.ok(result.length < 12000) // 10KB + truncation message
    assert.ok(result.includes('[truncated]'))
  })

  test('should handle functions by converting to string', async () => {
    const { safeSerialize } = await import('../../extension/lib/page/serialize.js')

    const fn = function testFunc() {}
    const result = safeSerialize(fn)

    assert.ok(typeof result === 'string')
    assert.ok(result.includes('function') || result.includes('[Function'))
  })

  test('should handle DOM elements', async () => {
    const { safeSerialize } = await import('../../extension/lib/page/serialize.js')

    // Mock a DOM element
    const element = {
      nodeType: 1,
      tagName: 'DIV',
      id: 'test',
      className: 'foo bar'
    }

    const result = safeSerialize(element)

    // Should have some representation
    assert.ok(result)
  })

  test('should handle deeply nested objects', async () => {
    const { safeSerialize } = await import('../../extension/lib/page/serialize.js')

    let obj = { value: 'deep' }
    for (let i = 0; i < 50; i++) {
      obj = { nested: obj }
    }

    const result = safeSerialize(obj)

    // Should not throw, should truncate depth
    assert.ok(result)
  })
})
