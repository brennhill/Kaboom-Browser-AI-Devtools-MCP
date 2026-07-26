// @ts-nocheck
/**
 * @fileoverview query-execution-routing.test.js — world routing fallback behavior.
 */

import { beforeEach, describe, test, mock } from 'node:test'
import assert from 'node:assert'

const mockDebugLog = mock.fn()

mock.module('../../extension/background/index.js', {
  namedExports: {
    debugLog: mockDebugLog
  }
})

const { executeWithWorldRouting } = await import('../../extension/background/exec/query-execution.js')

describe('query execution world routing', () => {
  beforeEach(() => {
    mock.reset()
    globalThis.chrome = {
      tabs: {
        sendMessage: mock.fn(() =>
          Promise.resolve({ success: false, error: 'inject_not_responding', message: 'inject timed out' })
        )
      },
      scripting: {
        executeScript: mock.fn(() =>
          Promise.resolve([
            {
              result: {
                success: true,
                result: 2
              }
            }
          ])
        )
      }
    }
  })

  test('auto mode falls back to scripting API when MAIN world inject stops responding', async () => {
    const result = await executeWithWorldRouting(42, { script: '1+1', timeout_ms: 500 }, 'auto')

    assert.strictEqual(globalThis.chrome.tabs.sendMessage.mock.calls.length, 1)
    assert.strictEqual(globalThis.chrome.scripting.executeScript.mock.calls.length, 1)
    assert.strictEqual(result.success, true)
    assert.strictEqual(result.result, 2)
  })

  test('main mode returns inject_not_responding without fallback', async () => {
    const result = await executeWithWorldRouting(42, { script: '1+1', timeout_ms: 500 }, 'main')

    assert.strictEqual(globalThis.chrome.tabs.sendMessage.mock.calls.length, 1)
    assert.strictEqual(globalThis.chrome.scripting.executeScript.mock.calls.length, 0)
    assert.strictEqual(result.success, false)
    assert.strictEqual(result.error, 'inject_not_responding')
  })
})
