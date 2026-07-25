// @ts-nocheck
/**
 * @fileoverview csp-safe-integration.test.js — End-to-end integration tests for CSP-safe execution.
 * Tests the parse → execute pipeline and the fallback chain in executeWithWorldRouting.
 */

import { describe, test, beforeEach, mock } from 'node:test'
import assert from 'node:assert'

const { parseExpression } = await import('../../extension/background/csp-safe/parser.js')
const { cspSafeExecutor } = await import('../../extension/background/csp-safe/executor.js')

// =============================================================================
// End-to-end: parse + execute
// =============================================================================

describe('CSP-safe integration: parse + execute', () => {
  beforeEach(() => {
    // Set up mock globals for test
    globalThis.document = {
      title: 'Test Page',
      querySelector: function (_sel) { return { tagName: 'DIV', textContent: 'found: ' + _sel } },
      querySelectorAll: function (_sel) { return [{ tagName: 'DIV' }] }
    }
    globalThis.window = {
      location: { href: 'https://example.com/page', hostname: 'example.com' }
    }
    globalThis.localStorage = {
      getItem: function (key) { return 'stored_' + key },
      setItem: function () {}
    }
  })

  function parseAndExecute(input) {
    const parsed = parseExpression(input)
    if (!parsed.ok) return { success: false, error: 'parse_failed', reason: parsed.reason }
    return cspSafeExecutor(parsed.command)
  }

  test('document.title', () => {
    const result = parseAndExecute('document.title')
    assert.strictEqual(result.success, true)
    assert.strictEqual(result.result, 'Test Page')
  })

  test('window.location.href', () => {
    const result = parseAndExecute('window.location.href')
    assert.strictEqual(result.success, true)
    assert.strictEqual(result.result, 'https://example.com/page')
  })

  test("document.querySelector('#app')", () => {
    const result = parseAndExecute("document.querySelector('#app')")
    assert.strictEqual(result.success, true)
    assert.strictEqual(result.result.textContent, 'found: #app')
  })

  test("localStorage.getItem('token')", () => {
    const result = parseAndExecute("localStorage.getItem('token')")
    assert.strictEqual(result.success, true)
    assert.strictEqual(result.result, 'stored_token')
  })

  test("new URL('https://example.com/path').pathname", () => {
    const result = parseAndExecute("new URL('https://example.com/path').pathname")
    assert.strictEqual(result.success, true)
    assert.strictEqual(result.result, '/path')
  })

  test("({title: document.title, url: window.location.href})", () => {
    const result = parseAndExecute("({title: document.title, url: window.location.href})")
    assert.strictEqual(result.success, true)
    assert.strictEqual(result.result.title, 'Test Page')
    assert.strictEqual(result.result.url, 'https://example.com/page')
  })

  test("document.title = 'New Title'", () => {
    const result = parseAndExecute("document.title = 'New Title'")
    assert.strictEqual(result.success, true)
    assert.strictEqual(globalThis.document.title, 'New Title')
  })
})

// =============================================================================
// Parse failure returns useful error
// =============================================================================

describe('CSP-safe integration: parse failure', () => {
  test('complex JS returns parse failure', () => {
    const parsed = parseExpression('() => document.title')
    assert.strictEqual(parsed.ok, false)
    assert.ok(parsed.reason.length > 0)
  })

  test('control flow returns parse failure', () => {
    const parsed = parseExpression('if (true) { document.title }')
    assert.strictEqual(parsed.ok, false)
  })
})

// =============================================================================
// Result tagging
// =============================================================================

describe('CSP-safe integration: result tagging', () => {
  test('all results include execution_mode: csp_safe_structured', () => {
    const parsed = parseExpression('42')
    assert.ok(parsed.ok)
    const result = cspSafeExecutor(parsed.command)
    assert.strictEqual(result.execution_mode, 'csp_safe_structured')
  })

  test('error results include execution_mode', () => {
    const result = cspSafeExecutor({
      expr: {
        type: 'chain',
        root: { type: 'literal', value: null },
        steps: [{ op: 'access', key: 'x' }]
      }
    })
    assert.strictEqual(result.success, false)
    assert.strictEqual(result.execution_mode, 'csp_safe_structured')
  })
})

// =============================================================================
// Fallback chain: executeWithWorldRouting
// =============================================================================

const mockDebugLog = mock.fn()

mock.module('../../extension/background/index.js', {
  namedExports: {
    debugLog: mockDebugLog
  }
})

const { executeWithWorldRouting } = await import('../../extension/background/query-execution.js')

describe('CSP-safe integration: fallback chain', () => {
  beforeEach(() => {
    mock.reset()
  })

  test('CSP error from content script falls back to MAIN structured executor', async () => {
    globalThis.chrome = {
      tabs: {
        sendMessage: mock.fn(() =>
          Promise.resolve({ success: false, error: 'csp_blocked', message: 'CSP blocks eval' })
        )
      },
      scripting: {
        executeScript: mock.fn((_opts) =>
          Promise.resolve([{
            result: { success: true, result: 'structured_works', execution_mode: 'csp_safe_structured' }
          }])
        )
      }
    }

    const result = await executeWithWorldRouting(42, { script: 'document.title', timeout_ms: 500 }, 'auto')
    assert.strictEqual(result.success, true)
    assert.strictEqual(result.execution_mode, 'csp_safe_structured')
    // Structured executor should be called with MAIN world and cspSafeExecutor func
    const scriptCall = globalThis.chrome.scripting.executeScript.mock.calls[0]
    assert.strictEqual(scriptCall.arguments[0].world, 'MAIN')
  })

  test('inject_not_loaded falls back to MAIN world scripting API', async () => {
    globalThis.chrome = {
      tabs: {
        sendMessage: mock.fn(() =>
          Promise.resolve({ success: false, error: 'inject_not_loaded', message: 'no inject' })
        )
      },
      scripting: {
        executeScript: mock.fn(() =>
          Promise.resolve([{ result: { success: true, result: 'main_works' } }])
        )
      }
    }

    const result = await executeWithWorldRouting(42, { script: '1+1', timeout_ms: 500 }, 'auto')
    assert.strictEqual(result.success, true)
    assert.strictEqual(result.result, 'main_works')
    // Should use MAIN world, not ISOLATED
    const scriptCall = globalThis.chrome.scripting.executeScript.mock.calls[0]
    assert.strictEqual(scriptCall.arguments[0].world, 'MAIN')
  })

  test('world=isolated routes directly to structured executor in ISOLATED world', async () => {
    globalThis.chrome = {
      tabs: { sendMessage: mock.fn() },
      scripting: {
        executeScript: mock.fn((_opts) =>
          Promise.resolve([{
            result: { success: true, result: 'isolated_structured', execution_mode: 'csp_safe_structured' }
          }])
        )
      }
    }

    const result = await executeWithWorldRouting(42, { script: 'document.title', timeout_ms: 500 }, 'isolated')
    assert.strictEqual(result.success, true)
    assert.strictEqual(result.execution_mode, 'isolated_structured')
    // Should not call sendMessage at all
    assert.strictEqual(globalThis.chrome.tabs.sendMessage.mock.calls.length, 0)
    // Should call scripting API with ISOLATED world and cspSafeExecutor func
    const scriptCall = globalThis.chrome.scripting.executeScript.mock.calls[0]
    assert.strictEqual(scriptCall.arguments[0].world, 'ISOLATED')
  })

  test('CSP error falls back to MAIN structured executor in single stage', async () => {
    globalThis.chrome = {
      tabs: {
        sendMessage: mock.fn(() =>
          Promise.resolve({ success: false, error: 'csp_blocked', message: 'CSP blocks eval' })
        )
      },
      scripting: {
        executeScript: mock.fn(() =>
          Promise.resolve([{
            result: { success: true, result: 'structured_works', execution_mode: 'csp_safe_structured' }
          }])
        )
      }
    }

    const result = await executeWithWorldRouting(42, { script: 'document.title', timeout_ms: 500 }, 'auto')
    assert.strictEqual(result.success, true)
    assert.strictEqual(result.execution_mode, 'csp_safe_structured')
    // Only one scripting API call (structured executor), no ISOLATED intermediate step
    assert.strictEqual(globalThis.chrome.scripting.executeScript.mock.calls.length, 1)
  })

  test('world=isolated returns parse error for unsupported expressions', async () => {
    globalThis.chrome = {
      tabs: { sendMessage: mock.fn() },
      scripting: { executeScript: mock.fn() }
    }

    const result = await executeWithWorldRouting(42, { script: '() => document.title', timeout_ms: 500 }, 'isolated')
    assert.strictEqual(result.success, false)
    assert.strictEqual(result.error, 'csp_blocked_unparseable')
    assert.strictEqual(result.execution_mode, 'isolated_structured')
    // Error message should never suggest world="isolated" — it uses the same structured executor
    assert.ok(!result.message.includes('world="isolated"'), 'should not suggest world=isolated')
    assert.ok(result.message.includes('DOM primitives'), 'should suggest DOM primitives')
    // Should not call sendMessage
    assert.strictEqual(globalThis.chrome.tabs.sendMessage.mock.calls.length, 0)
  })

  test('content script unreachable falls back to scripting API then structured', async () => {
    let callCount = 0
    globalThis.chrome = {
      tabs: {
        sendMessage: mock.fn(() => {
          throw new Error('Could not establish connection. Receiving end does not exist.')
        })
      },
      scripting: {
        executeScript: mock.fn(() => {
          callCount++
          if (callCount === 1) {
            // MAIN scripting API fails due to CSP
            return Promise.resolve([{
              result: { success: false, error: 'csp_blocked_all_worlds', message: 'CSP blocks all' }
            }])
          }
          // Second call is the structured executor
          return Promise.resolve([{
            result: { success: true, result: 'structured_works', execution_mode: 'csp_safe_structured' }
          }])
        })
      }
    }

    const result = await executeWithWorldRouting(42, { script: 'document.title', timeout_ms: 500 }, 'auto')
    assert.strictEqual(result.success, true)
    assert.strictEqual(result.execution_mode, 'csp_safe_structured')
    // Two scripting API calls: MAIN new Function (failed), then structured executor
    assert.strictEqual(globalThis.chrome.scripting.executeScript.mock.calls.length, 2)
    // First call should be MAIN world (new Function attempt)
    assert.strictEqual(globalThis.chrome.scripting.executeScript.mock.calls[0].arguments[0].world, 'MAIN')
    // Second call should be MAIN world (structured executor)
    assert.strictEqual(globalThis.chrome.scripting.executeScript.mock.calls[1].arguments[0].world, 'MAIN')
  })
})
