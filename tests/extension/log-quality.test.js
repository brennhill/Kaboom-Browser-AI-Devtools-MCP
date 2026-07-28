// @ts-nocheck
/**
 * @fileoverview log-quality.test.js — Tests for W3 LogEntry data quality.
 * Verifies that bridge.js always enriches payloads with `message` and `source`
 * fields, and that exceptions.js includes the `source` field.
 */

import { test, describe, beforeEach, afterEach } from 'node:test'
import assert from 'node:assert'
import { createMockWindow, createMockConsole, createMockDocument } from './helpers.js'

let originalWindow
let originalConsole

// =============================================================================
// Bridge: message field enrichment
// =============================================================================

describe('W3 Bridge: message and source enrichment', () => {
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

  test('postLog always sends message and source fields', async () => {
    const { postLog } = await import('../../extension/lib/page/bridge.js')

    postLog({ level: 'error', args: ['test'] })

    const [posted] = globalThis.window.postMessage.mock.calls[0].arguments
    assert.strictEqual(posted.type, 'kaboom_log')
    assert.ok('message' in posted.payload, 'payload should have message field')
    assert.ok('source' in posted.payload, 'payload should have source field')
  })

  test('message extracted from args[0] when payload.message absent', async () => {
    const { postLog } = await import('../../extension/lib/page/bridge.js')

    postLog({ level: 'warn', args: ['something went wrong', { detail: 1 }] })

    const [posted] = globalThis.window.postMessage.mock.calls[0].arguments
    assert.strictEqual(posted.payload.message, 'something went wrong')
  })

  test('message preserves existing payload.message (not overwritten by args)', async () => {
    const { postLog } = await import('../../extension/lib/page/bridge.js')

    postLog({ level: 'error', message: 'custom msg', args: ['ignored arg'] })

    const [posted] = globalThis.window.postMessage.mock.calls[0].arguments
    assert.strictEqual(posted.payload.message, 'custom msg')
  })

  test('message is empty string when no message and no args', async () => {
    const { postLog } = await import('../../extension/lib/page/bridge.js')

    postLog({ level: 'info' })

    const [posted] = globalThis.window.postMessage.mock.calls[0].arguments
    assert.strictEqual(posted.payload.message, '')
  })

  test('message handles numeric args[0]', async () => {
    const { postLog } = await import('../../extension/lib/page/bridge.js')

    postLog({ level: 'log', args: [42] })

    const [posted] = globalThis.window.postMessage.mock.calls[0].arguments
    assert.strictEqual(posted.payload.message, '42')
  })

  test('source derived from filename and lineno', async () => {
    const { postLog } = await import('../../extension/lib/page/bridge.js')

    postLog({ level: 'error', filename: 'app.js', lineno: 42 })

    const [posted] = globalThis.window.postMessage.mock.calls[0].arguments
    assert.strictEqual(posted.payload.source, 'app.js:42')
  })

  test('source defaults lineno to 0 when lineno missing', async () => {
    const { postLog } = await import('../../extension/lib/page/bridge.js')

    postLog({ level: 'error', filename: 'app.js' })

    const [posted] = globalThis.window.postMessage.mock.calls[0].arguments
    assert.strictEqual(posted.payload.source, 'app.js:0')
  })

  test('source is empty string when no filename available', async () => {
    const { postLog } = await import('../../extension/lib/page/bridge.js')

    postLog({ level: 'log', args: ['hello'] })

    const [posted] = globalThis.window.postMessage.mock.calls[0].arguments
    assert.strictEqual(posted.payload.source, '')
  })

  test('payload source field overrides default when explicitly set', async () => {
    const { postLog } = await import('../../extension/lib/page/bridge.js')

    postLog({ level: 'error', source: 'custom-source:99', filename: 'app.js', lineno: 42 })

    const [posted] = globalThis.window.postMessage.mock.calls[0].arguments
    // The ...payload spread comes after, so payload.source should override the default
    assert.strictEqual(posted.payload.source, 'custom-source:99')
  })
})

// =============================================================================
// Bridge: console capture integration (message extracted from args)
// =============================================================================

describe('W3 Bridge: console capture populates message', () => {
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

  test('console.log populates message from first arg', async () => {
    const { installConsoleCapture, uninstallConsoleCapture } = await import('../../extension/lib/page/console.js')

    installConsoleCapture()
    globalThis.console.log('hello world')

    const [posted] = globalThis.window.postMessage.mock.calls[0].arguments
    assert.strictEqual(posted.payload.message, 'hello world')

    uninstallConsoleCapture()
  })

  test('console.error populates message from first arg', async () => {
    const { installConsoleCapture, uninstallConsoleCapture } = await import('../../extension/lib/page/console.js')

    installConsoleCapture()
    globalThis.console.error('failure occurred')

    const [posted] = globalThis.window.postMessage.mock.calls[0].arguments
    assert.strictEqual(posted.payload.message, 'failure occurred')

    uninstallConsoleCapture()
  })
})

// =============================================================================
// Exceptions: source field
// =============================================================================

describe('W3 Exceptions: source field', () => {
  beforeEach(() => {
    originalWindow = globalThis.window
    globalThis.window = createMockWindow({ href: 'http://localhost:3000/test', withOnerror: true })
    globalThis.document = createMockDocument({ activeElement: null })
  })

  afterEach(() => {
    globalThis.window = originalWindow
    delete globalThis.document
  })

  test('exception entries include source field with filename:lineno format', async () => {
    const { installExceptionCapture, uninstallExceptionCapture: _uninstallExceptionCapture } =
      await import('../../extension/lib/page/exceptions.js')

    installExceptionCapture()

    globalThis.window.onerror('TypeError: x is undefined', 'http://localhost:3000/app.js', 42, 15, new Error('x'))

    // Wait for async enrichment
    await new Promise((resolve) => setTimeout(resolve, 50))

    const calls = globalThis.window.postMessage.mock.calls
    const message = calls[calls.length - 1].arguments[0]
    assert.strictEqual(message.type, 'kaboom_log')
    assert.strictEqual(message.payload.source, 'http://localhost:3000/app.js:42')
  })

  test('exception source is empty when filename is empty', async () => {
    const { installExceptionCapture, uninstallExceptionCapture } = await import('../../extension/lib/page/exceptions.js')

    installExceptionCapture()

    globalThis.window.onerror('Script error', '', 0, 0, null)

    // Wait for async enrichment
    await new Promise((resolve) => setTimeout(resolve, 50))

    const calls = globalThis.window.postMessage.mock.calls
    const message = calls[calls.length - 1].arguments[0]
    assert.strictEqual(message.payload.source, '')

    uninstallExceptionCapture()
  })

  test('exception source defaults lineno to 0 when lineno not provided', async () => {
    const { installExceptionCapture, uninstallExceptionCapture } = await import('../../extension/lib/page/exceptions.js')

    installExceptionCapture()

    globalThis.window.onerror('Error', 'main.js', undefined, undefined, null)

    // Wait for async enrichment
    await new Promise((resolve) => setTimeout(resolve, 50))

    const calls = globalThis.window.postMessage.mock.calls
    const message = calls[calls.length - 1].arguments[0]
    assert.strictEqual(message.payload.source, 'main.js:0')

    uninstallExceptionCapture()
  })
})
