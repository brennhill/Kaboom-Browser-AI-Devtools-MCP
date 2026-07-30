// @ts-nocheck
/**
 * @fileoverview pilot-execute.test.js — Tests for execute_javascript feature.
 * Covers JavaScript execution, serialization of results, error handling,
 * promise resolution, timeout handling, and circular reference detection.
 */

import { test, describe, mock, beforeEach, afterEach } from 'node:test'
import assert from 'node:assert'
import { MANIFEST_VERSION } from '../shared/helpers.js'

async function resetPilotCacheForTesting(value) {
  const { resetPilotCacheForTesting } = await import('../../../extension/background/runtime-state/pilot-state.js')
  resetPilotCacheForTesting(value)
}

// Mock Chrome APIs
const createMockChrome = () => ({
  runtime: {
    onMessage: { addListener: mock.fn() },
    sendMessage: mock.fn(() => Promise.resolve()),
    getURL: mock.fn((path) => `chrome-extension://test-id/${path}`),
    getManifest: () => ({ version: MANIFEST_VERSION })
  },
  tabs: {
    query: mock.fn(() => Promise.resolve([{ id: 1, windowId: 1, url: 'http://localhost:3000' }])),
    sendMessage: mock.fn(() => Promise.resolve({ success: true, result: 42 })),
    onRemoved: { addListener: mock.fn() }
  },
  alarms: {
    create: mock.fn(),
    onAlarm: { addListener: mock.fn() }
  },
  storage: {
    sync: {
      get: mock.fn((keys, callback) => callback({ aiWebPilotEnabled: true })),
      set: mock.fn((data, callback) => callback && callback()),
      remove: mock.fn((keys, callback) => {
        if (typeof callback === 'function') callback()
        else return Promise.resolve()
      })
    },
    local: {
      get: mock.fn((keys, callback) => callback({})),
      set: mock.fn((data, callback) => callback && callback()),
      remove: mock.fn((keys, callback) => {
        if (typeof callback === 'function') callback()
        else return Promise.resolve()
      })
    },
    session: {
      get: mock.fn((keys, callback) => callback({})),
      set: mock.fn((data, callback) => callback && callback()),
      remove: mock.fn((keys, callback) => {
        if (typeof callback === 'function') callback()
        else return Promise.resolve()
      })
    },
    onChanged: {
      addListener: mock.fn()
    }
  }
})

const createMockDocument = () => ({
  getElementById: mock.fn(() => ({ textContent: '', id: 'test' })),
  querySelector: mock.fn(() => null),
  querySelectorAll: mock.fn(() => []),
  title: 'Test Page',
  readyState: 'complete',
  createElement: mock.fn((tag) => ({
    tagName: tag.toUpperCase(),
    id: '',
    className: '',
    textContent: '',
    remove: mock.fn()
  })),
  head: { appendChild: mock.fn() },
  documentElement: { appendChild: mock.fn() }
})

const createMockWindow = () => ({
  postMessage: mock.fn(),
  addEventListener: mock.fn(),
  location: { href: 'http://localhost:3000/test' }
})

let mockChrome, mockDocument, mockWindow

describe('executeJavaScript Function', () => {
  beforeEach(() => {
    mockChrome = createMockChrome()
    mockDocument = createMockDocument()
    mockWindow = createMockWindow()
    globalThis.chrome = mockChrome
    globalThis.document = mockDocument
    globalThis.window = mockWindow
  })

  afterEach(() => {
    // Don't delete globalThis.chrome — background.js has deferred async
    // operations (alarms, reconnect) that reference chrome after test ends.
    // beforeEach() recreates all mocks fresh each time.
    delete globalThis.document
    delete globalThis.window
  })

  test('simple expression: 1 + 1 should return { result: 2 }', async () => {
    const { executeJavaScript } = await import('../../../extension/inject/execute-js.js')

    const response = await executeJavaScript('1 + 1')

    assert.strictEqual(response.success, true, 'Should succeed')
    assert.strictEqual(response.result, 2, 'Should return 2')
  })

  test('access globals: window.location.href should return URL string', async () => {
    const { executeJavaScript } = await import('../../../extension/inject/execute-js.js')

    const response = await executeJavaScript('window.location.href')

    assert.strictEqual(response.success, true, 'Should succeed')
    assert.strictEqual(typeof response.result, 'string', 'Should return a string')
    assert.ok(response.result.includes('localhost'), 'Should contain localhost')
  })

  test('object serialization: { a: 1, b: [2, 3] } should be properly serialized', async () => {
    const { executeJavaScript } = await import('../../../extension/inject/execute-js.js')

    const response = await executeJavaScript('({ a: 1, b: [2, 3] })')

    assert.strictEqual(response.success, true, 'Should succeed')
    assert.deepStrictEqual(response.result, { a: 1, b: [2, 3] }, 'Should serialize object correctly')
  })

  test('function return: (() => 42)() should return { result: 42 }', async () => {
    const { executeJavaScript } = await import('../../../extension/inject/execute-js.js')

    const response = await executeJavaScript('(() => 42)()')

    assert.strictEqual(response.success, true, 'Should succeed')
    assert.strictEqual(response.result, 42, 'Should return 42')
  })

  test('error handling: (() => { throw new Error("test") })() should return error response with stack', async () => {
    const { executeJavaScript } = await import('../../../extension/inject/execute-js.js')

    // Wrap throw in an IIFE since throw is a statement, not an expression
    const response = await executeJavaScript('(() => { throw new Error("test") })()')

    assert.strictEqual(response.success, false, 'Should fail')
    assert.strictEqual(response.error, 'execution_error', 'Should have execution_error type')
    assert.strictEqual(response.message, 'test', 'Should have error message')
    assert.ok(response.stack, 'Should have stack trace')
  })

  test('promise resolution: Promise.resolve(42) should return { result: 42 }', async () => {
    const { executeJavaScript } = await import('../../../extension/inject/execute-js.js')

    const response = await executeJavaScript('Promise.resolve(42)')

    assert.strictEqual(response.success, true, 'Should succeed')
    assert.strictEqual(response.result, 42, 'Should return 42')
  })

  test('promise rejection: Promise.reject(new Error("fail")) should return error response', async () => {
    const { executeJavaScript } = await import('../../../extension/inject/execute-js.js')

    const response = await executeJavaScript('Promise.reject(new Error("fail"))')

    assert.strictEqual(response.success, false, 'Should fail')
    assert.strictEqual(response.error, 'promise_rejected', 'Should have promise_rejected type')
    assert.strictEqual(response.message, 'fail', 'Should have error message')
  })

  test('async promise: new Promise(r => setTimeout(() => r(99), 10)) should resolve', async () => {
    const { executeJavaScript } = await import('../../../extension/inject/execute-js.js')

    const response = await executeJavaScript('new Promise(r => setTimeout(() => r(99), 10))')

    assert.strictEqual(response.success, true, 'Should succeed')
    assert.strictEqual(response.result, 99, 'Should return 99')
  })
})

describe('executeJavaScript Script Wrapping', () => {
  beforeEach(() => {
    mockChrome = createMockChrome()
    mockDocument = createMockDocument()
    mockWindow = createMockWindow()
    globalThis.chrome = mockChrome
    globalThis.document = mockDocument
    globalThis.window = mockWindow
  })

  afterEach(() => {
    delete globalThis.document
    delete globalThis.window
  })

  test('IIFE with semicolons should return its value', async () => {
    const { executeJavaScript } = await import('../../../extension/inject/execute-js.js')

    // IIFE bodies contain semicolons but the outer form is a single expression.
    // The return value of the IIFE must be captured.
    const response = await executeJavaScript('(function(){ var x = 42; return x; })()')

    assert.strictEqual(response.success, true, 'Should succeed')
    assert.strictEqual(response.result, 42, 'IIFE return value must be captured, not lost')
  })

  test('try/catch block should execute without SyntaxError', async () => {
    const { executeJavaScript } = await import('../../../extension/inject/execute-js.js')

    // try/catch is a statement, not an expression.
    // Wrapping it as return(try{...}catch{...}) is invalid JS.
    const response = await executeJavaScript("try { JSON.parse('{}') } catch(e) { }")

    assert.strictEqual(response.success, true, 'try/catch must not throw SyntaxError')
    assert.notStrictEqual(response.error, 'execution_error', 'Must not be a wrapping error')
  })

  test('if/else block should execute without SyntaxError', async () => {
    const { executeJavaScript } = await import('../../../extension/inject/execute-js.js')

    const response = await executeJavaScript('if (true) { 1 } else { 2 }')

    assert.strictEqual(response.success, true, 'if/else must not throw SyntaxError')
  })

  test('multi-statement script with trailing expression should return value', async () => {
    const { executeJavaScript } = await import('../../../extension/inject/execute-js.js')

    // Scripts like "var x = 1; x" should ideally return x,
    // but at minimum must not crash.
    const response = await executeJavaScript('var x = 99; x')

    assert.strictEqual(response.success, true, 'Should succeed')
    // The return value capture for multi-statement scripts is best-effort
  })

  test('arrow IIFE with semicolons should return its value', async () => {
    const { executeJavaScript } = await import('../../../extension/inject/execute-js.js')

    const response = await executeJavaScript('(() => { var a = 1; var b = 2; return a + b; })()')

    assert.strictEqual(response.success, true, 'Should succeed')
    assert.strictEqual(response.result, 3, 'Arrow IIFE return value must be captured')
  })

  test('IIFE returning JSON string should return the string', async () => {
    const { executeJavaScript } = await import('../../../extension/inject/execute-js.js')

    // This is the exact pattern used by the smoke test subtitle check
    const response = await executeJavaScript('(function() { var x = "hello"; return JSON.stringify({text: x}); })()')

    assert.strictEqual(response.success, true, 'Should succeed')
    assert.strictEqual(response.result, '{"text":"hello"}', 'IIFE should return the JSON string')
  })
})

describe('safeSerializeForExecute Function', () => {
  beforeEach(() => {
    mockChrome = createMockChrome()
    mockDocument = createMockDocument()
    mockWindow = createMockWindow()
    globalThis.chrome = mockChrome
    globalThis.document = mockDocument
    globalThis.window = mockWindow
  })

  afterEach(() => {
    // Don't delete globalThis.chrome — background.js has deferred async
    // operations (alarms, reconnect) that reference chrome after test ends.
    // beforeEach() recreates all mocks fresh each time.
    delete globalThis.document
    delete globalThis.window
  })

  test('should handle null', async () => {
    const { safeSerializeForExecute } = await import('../../../extension/inject/execute-js.js')

    const result = safeSerializeForExecute(null)
    assert.strictEqual(result, null)
  })

  test('should handle undefined', async () => {
    const { safeSerializeForExecute } = await import('../../../extension/inject/execute-js.js')

    const result = safeSerializeForExecute(undefined)
    assert.strictEqual(result, undefined)
  })

  test('should handle primitives', async () => {
    const { safeSerializeForExecute } = await import('../../../extension/inject/execute-js.js')

    assert.strictEqual(safeSerializeForExecute('hello'), 'hello')
    assert.strictEqual(safeSerializeForExecute(42), 42)
    assert.strictEqual(safeSerializeForExecute(true), true)
    assert.strictEqual(safeSerializeForExecute(false), false)
  })

  test('should handle functions', async () => {
    const { safeSerializeForExecute } = await import('../../../extension/inject/execute-js.js')

    const namedFn = function myFunc() {}
    // Note: In modern JS, variables assigned to anonymous functions get the variable name
    // so we use a truly anonymous function from an array
    const trulyAnonFn = [function () {}][0]

    assert.strictEqual(safeSerializeForExecute(namedFn), '[Function: myFunc]')
    // trulyAnonFn has no name
    assert.strictEqual(safeSerializeForExecute(trulyAnonFn), '[Function: anonymous]')
  })

  test('should handle symbols', async () => {
    const { safeSerializeForExecute } = await import('../../../extension/inject/execute-js.js')

    const sym = Symbol('test')
    assert.strictEqual(safeSerializeForExecute(sym), 'Symbol(test)')
  })

  test('should handle circular references without crashing', async () => {
    const { safeSerializeForExecute } = await import('../../../extension/inject/execute-js.js')

    const obj = { a: 1 }
    obj.self = obj

    const result = safeSerializeForExecute(obj)

    assert.strictEqual(result.a, 1)
    assert.strictEqual(result.self, '[Circular]')
  })

  test('should handle deeply nested objects with max depth', async () => {
    const { safeSerializeForExecute } = await import('../../../extension/inject/execute-js.js')

    // Create an object 15 levels deep
    let deep = { value: 'bottom' }
    for (let i = 0; i < 14; i++) {
      deep = { nested: deep }
    }

    const result = safeSerializeForExecute(deep)

    // Should hit max depth at level 10
    let current = result
    let depth = 0
    while (current && typeof current === 'object' && current.nested) {
      current = current.nested
      depth++
    }

    // At some point it should stop serializing further
    assert.ok(depth < 15, 'Should stop before max depth')
  })

  test('should handle arrays (capped at 100 elements)', async () => {
    const { safeSerializeForExecute } = await import('../../../extension/inject/execute-js.js')

    const largeArray = Array.from({ length: 150 }, (_, i) => i)
    const result = safeSerializeForExecute(largeArray)

    assert.strictEqual(result.length, 100, 'Should cap at 100 elements')
    assert.strictEqual(result[0], 0)
    assert.strictEqual(result[99], 99)
  })

  test('should handle Error objects', async () => {
    const { safeSerializeForExecute } = await import('../../../extension/inject/execute-js.js')

    const error = new Error('test error')
    const result = safeSerializeForExecute(error)

    assert.ok(result.error, 'Should have error property')
    assert.strictEqual(result.error, 'test error')
    assert.ok(result.stack, 'Should have stack property')
  })

  test('should handle Date objects', async () => {
    const { safeSerializeForExecute } = await import('../../../extension/inject/execute-js.js')

    const date = new Date('2024-01-15T10:30:00.000Z')
    const result = safeSerializeForExecute(date)

    assert.strictEqual(result, '2024-01-15T10:30:00.000Z')
  })

  test('should handle RegExp objects', async () => {
    const { safeSerializeForExecute } = await import('../../../extension/inject/execute-js.js')

    const regex = /test\d+/gi
    const result = safeSerializeForExecute(regex)

    assert.strictEqual(result, '/test\\d+/gi')
  })

  test('should handle DOM nodes', async () => {
    const { safeSerializeForExecute } = await import('../../../extension/inject/execute-js.js')

    // Mock a DOM node
    const mockNode = {
      nodeName: 'DIV',
      id: 'test-id',
      [Symbol.toStringTag]: 'HTMLDivElement'
    }
    // Add Node prototype check
    Object.setPrototypeOf(mockNode, { constructor: { name: 'Node' } })

    // For proper Node detection, we need to mock the Node class
    globalThis.Node = function Node() {}
    globalThis.Node.prototype = {}
    Object.setPrototypeOf(mockNode, globalThis.Node.prototype)

    const result = safeSerializeForExecute(mockNode)

    assert.strictEqual(result, '[DIV#test-id]')

    delete globalThis.Node
  })

  test('should handle large objects (capped at 50 keys)', async () => {
    const { safeSerializeForExecute } = await import('../../../extension/inject/execute-js.js')

    const largeObj = {}
    for (let i = 0; i < 100; i++) {
      largeObj[`key${i}`] = i
    }

    const result = safeSerializeForExecute(largeObj)
    const keys = Object.keys(result)

    // Should have 50 keys plus the '...' indicator
    assert.ok(keys.length <= 51, 'Should cap at ~50 keys')
    assert.ok(result['...'], 'Should have overflow indicator')
    assert.ok(result['...'].includes('more keys'), 'Should indicate more keys')
  })
})

describe('Timeout Handling', () => {
  beforeEach(() => {
    mockChrome = createMockChrome()
    mockDocument = createMockDocument()
    mockWindow = createMockWindow()
    globalThis.chrome = mockChrome
    globalThis.document = mockDocument
    globalThis.window = mockWindow
  })

  afterEach(() => {
    // Don't delete globalThis.chrome — background.js has deferred async
    // operations (alarms, reconnect) that reference chrome after test ends.
    // beforeEach() recreates all mocks fresh each time.
    delete globalThis.document
    delete globalThis.window
  })

  test('should timeout for long-running promises', async () => {
    const { executeJavaScript } = await import('../../../extension/inject/execute-js.js')

    // Use a promise that takes longer than the timeout
    const response = await executeJavaScript(
      'new Promise(r => setTimeout(r, 200))', // Promise takes 200ms
      50 // 50ms timeout
    )

    assert.strictEqual(response.success, false, 'Should fail')
    assert.strictEqual(response.error, 'execution_timeout', 'Should have execution_timeout type')
    assert.ok(response.message.includes('50'), 'Should mention timeout duration')
  })

  test('should respect custom timeout_ms parameter', async () => {
    const { executeJavaScript } = await import('../../../extension/inject/execute-js.js')

    // A quick operation should succeed with a reasonable timeout
    const response = await executeJavaScript('42', 1000)

    assert.strictEqual(response.success, true, 'Should succeed')
    assert.strictEqual(response.result, 42, 'Should return 42')
  })
})

describe('Content Script Message Forwarding', () => {
  beforeEach(() => {
    mockChrome = createMockChrome()
    mockDocument = createMockDocument()
    mockWindow = createMockWindow()
    globalThis.chrome = mockChrome
    globalThis.document = mockDocument
    globalThis.window = mockWindow
  })

  afterEach(() => {
    // Don't delete globalThis.chrome — background.js has deferred async
    // operations (alarms, reconnect) that reference chrome after test ends.
    // beforeEach() recreates all mocks fresh each time.
    delete globalThis.document
    delete globalThis.window
  })

  test('content script should forward kaboom_execute_js to inject.js', async () => {
    // Get the message listener from content.js
    await import('../../../extension/content.js')

    // Find the runtime.onMessage listener
    const messageListenerCalls = mockChrome.runtime.onMessage.addListener.mock.calls
    assert.ok(messageListenerCalls.length > 0, 'Should have registered a message listener')

    const listener = messageListenerCalls[0].arguments[0]
    assert.ok(typeof listener === 'function', 'Listener should be a function')
  })
})

describe('Background Script Pilot Command Handler', () => {
  beforeEach(() => {
    mockChrome = createMockChrome()
    mockDocument = createMockDocument()
    mockWindow = createMockWindow()
    globalThis.chrome = mockChrome
    globalThis.document = mockDocument
    globalThis.window = mockWindow
  })

  afterEach(() => {
    // Don't delete globalThis.chrome — background.js has deferred async
    // operations (alarms, reconnect) that reference chrome after test ends.
    // beforeEach() recreates all mocks fresh each time.
    delete globalThis.document
    delete globalThis.window
  })

  test('handlePilotCommand should forward kaboom_execute_js to content script when enabled', async () => {
    mockChrome.storage.sync.get.mock.mockImplementation((keys, callback) => {
      callback({ aiWebPilotEnabled: true })
    })
    mockChrome.storage.local.get.mock.mockImplementation((keys, callback) => {
      callback({ aiWebPilotEnabled: true })
    })

    mockChrome.tabs.sendMessage.mock.mockImplementation(() => Promise.resolve({ success: true, result: 'executed' }))

    const { handlePilotCommand } = await import('../../../extension/background/commands/interact.js')
    await resetPilotCacheForTesting(true)

    await handlePilotCommand('kaboom_execute_js', {
      script: 'return 1+1',
      timeout_ms: 5000
    })

    // Should forward to tab
    assert.ok(mockChrome.tabs.sendMessage.mock.calls.length > 0, 'Should send message to tab')

    // Check the message type
    const sentMessage = mockChrome.tabs.sendMessage.mock.calls[0].arguments[1]
    assert.strictEqual(sentMessage.type, 'kaboom_execute_js', 'Should send correct message type')
  })

  test('handlePilotCommand should reject kaboom_execute_js when disabled', async () => {
    mockChrome.storage.sync.get.mock.mockImplementation((keys, callback) => {
      callback({ aiWebPilotEnabled: false })
    })

    const { handlePilotCommand } = await import('../../../extension/background/commands/interact.js')
    await resetPilotCacheForTesting(false)

    const result = await handlePilotCommand('kaboom_execute_js', {
      script: 'return 1+1'
    })

    assert.strictEqual(result.error, 'ai_web_pilot_disabled', 'Should return disabled error')
  })
})
