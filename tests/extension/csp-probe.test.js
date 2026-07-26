// @ts-nocheck
/**
 * @fileoverview csp-probe.test.js — Tests for probeCSPStatus CSP detection.
 * Verifies the three CSP levels: none, script_exec, page_blocked.
 */

import { test, describe, beforeEach, mock } from 'node:test'
import assert from 'node:assert'

describe('probeCSPStatus', () => {
  let probeCSPStatus

  beforeEach(async () => {
    mock.reset()

    // Set up chrome.scripting mock (default: succeeds with 'ok')
    globalThis.chrome = {
      scripting: {
        executeScript: mock.fn(async () => [{ result: 'ok' }])
      }
    }
    ;({ probeCSPStatus } = await import('../../extension/background/exec/query-execution.js'))
  })

  test('returns none when execute_js succeeds', async () => {
    chrome.scripting.executeScript = mock.fn(async () => [{ result: 'ok' }])

    const result = await probeCSPStatus(123)
    assert.deepStrictEqual(result, { csp_restricted: false, csp_level: 'none' })
  })

  test('returns script_exec when CSP blocks new Function', async () => {
    chrome.scripting.executeScript = mock.fn(async () => [{ result: 'csp_blocked' }])

    const result = await probeCSPStatus(123)
    assert.deepStrictEqual(result, { csp_restricted: true, csp_level: 'script_exec' })
  })

  test('returns page_blocked when chrome.scripting throws', async () => {
    chrome.scripting.executeScript = mock.fn(async () => {
      throw new Error('Cannot access chrome:// URL')
    })

    const result = await probeCSPStatus(123)
    assert.deepStrictEqual(result, { csp_restricted: true, csp_level: 'page_blocked' })
  })

  test('returns page_blocked when result is null', async () => {
    chrome.scripting.executeScript = mock.fn(async () => null)

    const result = await probeCSPStatus(123)
    assert.deepStrictEqual(result, { csp_restricted: true, csp_level: 'page_blocked' })
  })

  test('returns page_blocked when result array is empty', async () => {
    chrome.scripting.executeScript = mock.fn(async () => [])

    const result = await probeCSPStatus(123)
    assert.deepStrictEqual(result, { csp_restricted: true, csp_level: 'page_blocked' })
  })

  test('passes correct tabId and MAIN world to executeScript', async () => {
    await probeCSPStatus(456)

    const calls = chrome.scripting.executeScript.mock.calls
    assert.strictEqual(calls.length, 1)
    const arg = calls[0].arguments[0]
    assert.strictEqual(arg.target.tabId, 456)
    assert.strictEqual(arg.world, 'MAIN')
    assert.strictEqual(typeof arg.func, 'function')
  })
})
