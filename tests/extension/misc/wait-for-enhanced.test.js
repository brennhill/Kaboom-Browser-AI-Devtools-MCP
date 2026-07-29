// @ts-nocheck
/**
 * @fileoverview Tests for enhanced wait_for conditions (#371, #362).
 *
 * Validates dispatch routing for:
 *   - wait_for with selector (existing behavior)
 *   - wait_for with absent:true (element disappearance)
 *   - wait_for with text (text matching)
 *   - wait_for with url_contains (URL substring matching)
 *   - Timeout behavior for each condition type
 *   - Validation: mutual exclusivity, absent requires selector
 *   - Polling retry behavior (transition from failure to success)
 *   - Error handling: chrome.tabs.get failure during URL polling
 */

import { test, describe, mock, beforeEach } from 'node:test'
import assert from 'node:assert'

function createMockSyncClient() {
  return { id: 'test-client' }
}

function createMockSendAsyncResult() {
  return mock.fn()
}

function createMockActionToast() {
  return mock.fn()
}

function createPendingQuery(params) {
  return {
    id: 'q-1',
    correlation_id: 'corr-1',
    type: 'dom_action',
    params: JSON.stringify(params),
    created_at: Date.now()
  }
}

describe('Enhanced wait_for dispatch routing (#371, #362)', () => {
  let sendAsyncResult
  let actionToast
  let syncClient
  let executeScriptCalls

  beforeEach(() => {
    executeScriptCalls = []
    sendAsyncResult = createMockSendAsyncResult()
    actionToast = createMockActionToast()
    syncClient = createMockSyncClient()

    globalThis.chrome = {
      runtime: {
        onMessage: { addListener: mock.fn() },
        onInstalled: { addListener: mock.fn() },
        sendMessage: mock.fn(() => Promise.resolve()),
        getManifest: () => ({ version: '0.7.12' })
      },
      scripting: {
        executeScript: mock.fn((opts) => {
          executeScriptCalls.push(opts)
          const action = opts.args?.[0] || 'wait_for'
          const selector = opts.args?.[1] || ''
          return Promise.resolve([{
            frameId: 0,
            result: { success: true, action, selector, value: 'div' }
          }])
        })
      },
      tabs: {
        get: mock.fn((tabId) => Promise.resolve({
          id: tabId,
          windowId: 1,
          url: 'http://localhost:3000/dashboard'
        }))
      },
      storage: {
        local: {
          get: mock.fn(() => Promise.resolve({})),
          set: mock.fn(() => Promise.resolve()),
          remove: mock.fn(() => Promise.resolve())
        }
      }
    }
  })

  // ── Selector (existing behavior) ──

  test('wait_for with selector dispatches wait_for action', async () => {
    const { executeDOMAction } = await import('../../../extension/background/dom/dom-dispatch.js')

    const query = createPendingQuery({
      action: 'wait_for',
      selector: '#my-element',
      timeout_ms: 100
    })

    await executeDOMAction(query, 1, syncClient, sendAsyncResult, actionToast)

    assert.strictEqual(sendAsyncResult.mock.calls.length, 1)
    const [, , , status] = sendAsyncResult.mock.calls[0].arguments
    assert.strictEqual(status, 'complete')

    assert.ok(executeScriptCalls.length >= 1)
    assert.strictEqual(executeScriptCalls[0].args[0], 'wait_for')
    assert.strictEqual(executeScriptCalls[0].args[1], '#my-element')
  })

  // ── Absent condition ──

  test('wait_for with absent:true dispatches wait_for_absent action', async () => {
    const { executeDOMAction } = await import('../../../extension/background/dom/dom-dispatch.js')

    globalThis.chrome.scripting.executeScript = mock.fn((opts) => {
      executeScriptCalls.push(opts)
      const action = opts.args?.[0] || 'wait_for_absent'
      const selector = opts.args?.[1] || ''
      return Promise.resolve([{
        frameId: 0,
        result: { success: true, action, selector, absent: true }
      }])
    })

    const query = createPendingQuery({
      action: 'wait_for',
      selector: '#loading-spinner',
      absent: true,
      timeout_ms: 100
    })

    await executeDOMAction(query, 1, syncClient, sendAsyncResult, actionToast)

    assert.strictEqual(sendAsyncResult.mock.calls.length, 1)
    const [, , , status] = sendAsyncResult.mock.calls[0].arguments
    assert.strictEqual(status, 'complete')

    assert.ok(executeScriptCalls.length >= 1)
    assert.strictEqual(executeScriptCalls[0].args[0], 'wait_for_absent')
    assert.strictEqual(executeScriptCalls[0].args[1], '#loading-spinner')
  })

  // ── Text condition ──

  test('wait_for with text dispatches wait_for_text action', async () => {
    const { executeDOMAction } = await import('../../../extension/background/dom/dom-dispatch.js')

    globalThis.chrome.scripting.executeScript = mock.fn((opts) => {
      executeScriptCalls.push(opts)
      const action = opts.args?.[0] || 'wait_for_text'
      return Promise.resolve([{
        frameId: 0,
        result: { success: true, action, selector: '', matched_text: 'Welcome back' }
      }])
    })

    const query = createPendingQuery({
      action: 'wait_for',
      text: 'Welcome back',
      timeout_ms: 100
    })

    await executeDOMAction(query, 1, syncClient, sendAsyncResult, actionToast)

    assert.strictEqual(sendAsyncResult.mock.calls.length, 1)
    const [, , , status] = sendAsyncResult.mock.calls[0].arguments
    assert.strictEqual(status, 'complete')

    assert.ok(executeScriptCalls.length >= 1)
    assert.strictEqual(executeScriptCalls[0].args[0], 'wait_for_text')
  })

  // ── URL condition ──

  test('wait_for with url_contains polls tab URL without page injection', async () => {
    const { executeDOMAction } = await import('../../../extension/background/dom/dom-dispatch.js')

    const query = createPendingQuery({
      action: 'wait_for',
      url_contains: '/dashboard',
      timeout_ms: 100
    })

    await executeDOMAction(query, 1, syncClient, sendAsyncResult, actionToast)

    assert.strictEqual(sendAsyncResult.mock.calls.length, 1)
    const [, , , status] = sendAsyncResult.mock.calls[0].arguments
    assert.strictEqual(status, 'complete')

    assert.ok(globalThis.chrome.tabs.get.mock.calls.length >= 1)
    assert.strictEqual(executeScriptCalls.length, 0)
  })

  test('wait_for with url_contains times out when URL does not match', async () => {
    const { executeDOMAction } = await import('../../../extension/background/dom/dom-dispatch.js')

    globalThis.chrome.tabs.get = mock.fn((tabId) => Promise.resolve({
      id: tabId, windowId: 1, url: 'http://localhost:3000/login'
    }))

    const query = createPendingQuery({
      action: 'wait_for',
      url_contains: '/dashboard',
      timeout_ms: 200
    })

    await executeDOMAction(query, 1, syncClient, sendAsyncResult, actionToast)

    assert.strictEqual(sendAsyncResult.mock.calls.length, 1)
    const [, , , status, result] = sendAsyncResult.mock.calls[0].arguments
    assert.strictEqual(status, 'error')
    assert.strictEqual(result.error, 'timeout')
  })

  // ── Validation: missing conditions ──

  test('wait_for with no selector, text, or url_contains returns error', async () => {
    const { executeDOMAction } = await import('../../../extension/background/dom/dom-dispatch.js')

    const query = createPendingQuery({ action: 'wait_for', timeout_ms: 100 })

    await executeDOMAction(query, 1, syncClient, sendAsyncResult, actionToast)

    assert.strictEqual(sendAsyncResult.mock.calls.length, 1)
    const [, , , status, , error] = sendAsyncResult.mock.calls[0].arguments
    assert.strictEqual(status, 'error')
    assert.ok(error.includes('requires'))
  })

  // ── Validation: mutual exclusivity ──

  test('wait_for with both selector and text returns mutual exclusivity error', async () => {
    const { executeDOMAction } = await import('../../../extension/background/dom/dom-dispatch.js')

    const query = createPendingQuery({
      action: 'wait_for',
      selector: '#foo',
      text: 'bar',
      timeout_ms: 100
    })

    await executeDOMAction(query, 1, syncClient, sendAsyncResult, actionToast)

    assert.strictEqual(sendAsyncResult.mock.calls.length, 1)
    const [, , , status, , error] = sendAsyncResult.mock.calls[0].arguments
    assert.strictEqual(status, 'error')
    assert.ok(error.includes('mutually exclusive'))
  })

  test('wait_for with both url_contains and selector returns mutual exclusivity error', async () => {
    const { executeDOMAction } = await import('../../../extension/background/dom/dom-dispatch.js')

    const query = createPendingQuery({
      action: 'wait_for',
      selector: '#foo',
      url_contains: '/bar',
      timeout_ms: 100
    })

    await executeDOMAction(query, 1, syncClient, sendAsyncResult, actionToast)

    assert.strictEqual(sendAsyncResult.mock.calls.length, 1)
    const [, , , status, , error] = sendAsyncResult.mock.calls[0].arguments
    assert.strictEqual(status, 'error')
    assert.ok(error.includes('mutually exclusive'))
  })

  test('wait_for with both url_contains and text returns mutual exclusivity error', async () => {
    const { executeDOMAction } = await import('../../../extension/background/dom/dom-dispatch.js')

    const query = createPendingQuery({
      action: 'wait_for',
      text: 'hello',
      url_contains: '/bar',
      timeout_ms: 100
    })

    await executeDOMAction(query, 1, syncClient, sendAsyncResult, actionToast)

    assert.strictEqual(sendAsyncResult.mock.calls.length, 1)
    const [, , , status, , error] = sendAsyncResult.mock.calls[0].arguments
    assert.strictEqual(status, 'error')
    assert.ok(error.includes('mutually exclusive'))
  })

  // ── Validation: absent requires selector ──

  test('wait_for with absent:true but no selector returns error', async () => {
    const { executeDOMAction } = await import('../../../extension/background/dom/dom-dispatch.js')

    const query = createPendingQuery({
      action: 'wait_for',
      absent: true,
      timeout_ms: 100
    })

    await executeDOMAction(query, 1, syncClient, sendAsyncResult, actionToast)

    assert.strictEqual(sendAsyncResult.mock.calls.length, 1)
    const [, , , status, , error] = sendAsyncResult.mock.calls[0].arguments
    assert.strictEqual(status, 'error')
    assert.ok(error.includes('selector'))
  })

  // ── Timeout for selector ──

  test('wait_for with selector times out when element not found', async () => {
    const { executeDOMAction } = await import('../../../extension/background/dom/dom-dispatch.js')

    globalThis.chrome.scripting.executeScript = mock.fn((opts) => {
      executeScriptCalls.push(opts)
      return Promise.resolve([{
        frameId: 0,
        result: { success: false, action: 'wait_for', selector: '#never-exists', error: 'not_found' }
      }])
    })

    const query = createPendingQuery({
      action: 'wait_for',
      selector: '#never-exists',
      timeout_ms: 200
    })

    await executeDOMAction(query, 1, syncClient, sendAsyncResult, actionToast)

    assert.strictEqual(sendAsyncResult.mock.calls.length, 1)
    const [, , , status] = sendAsyncResult.mock.calls[0].arguments
    assert.strictEqual(status, 'error')
  })

  // ── Timeout for absent ──

  test('wait_for with absent:true times out when element persists', async () => {
    const { executeDOMAction } = await import('../../../extension/background/dom/dom-dispatch.js')

    globalThis.chrome.scripting.executeScript = mock.fn((opts) => {
      executeScriptCalls.push(opts)
      return Promise.resolve([{
        frameId: 0,
        result: { success: false, action: 'wait_for_absent', selector: '#persistent', error: 'element_still_present' }
      }])
    })

    const query = createPendingQuery({
      action: 'wait_for',
      selector: '#persistent',
      absent: true,
      timeout_ms: 200
    })

    await executeDOMAction(query, 1, syncClient, sendAsyncResult, actionToast)

    assert.strictEqual(sendAsyncResult.mock.calls.length, 1)
    const [, , , status] = sendAsyncResult.mock.calls[0].arguments
    assert.strictEqual(status, 'error')
  })

  // ── Timeout for text ──

  test('wait_for with text times out when text not found', async () => {
    const { executeDOMAction } = await import('../../../extension/background/dom/dom-dispatch.js')

    globalThis.chrome.scripting.executeScript = mock.fn((opts) => {
      executeScriptCalls.push(opts)
      return Promise.resolve([{
        frameId: 0,
        result: { success: false, action: 'wait_for_text', selector: '', error: 'text_not_found' }
      }])
    })

    const query = createPendingQuery({
      action: 'wait_for',
      text: 'Never appears',
      timeout_ms: 200
    })

    await executeDOMAction(query, 1, syncClient, sendAsyncResult, actionToast)

    assert.strictEqual(sendAsyncResult.mock.calls.length, 1)
    const [, , , status] = sendAsyncResult.mock.calls[0].arguments
    assert.strictEqual(status, 'error')
  })

  // ── Polling retry: condition becomes true after initial failure ──

  test('wait_for with selector retries and succeeds after initial failure', async () => {
    const { executeDOMAction } = await import('../../../extension/background/dom/dom-dispatch.js')

    let callCount = 0
    globalThis.chrome.scripting.executeScript = mock.fn((opts) => {
      executeScriptCalls.push(opts)
      callCount++
      // First 2 calls fail, 3rd succeeds
      if (callCount < 3) {
        return Promise.resolve([{
          frameId: 0,
          result: { success: false, action: 'wait_for', selector: '#delayed', error: 'not_found' }
        }])
      }
      return Promise.resolve([{
        frameId: 0,
        result: { success: true, action: 'wait_for', selector: '#delayed', value: 'div' }
      }])
    })

    const query = createPendingQuery({
      action: 'wait_for',
      selector: '#delayed',
      timeout_ms: 2000
    })

    await executeDOMAction(query, 1, syncClient, sendAsyncResult, actionToast)

    assert.strictEqual(sendAsyncResult.mock.calls.length, 1)
    const [, , , status] = sendAsyncResult.mock.calls[0].arguments
    assert.strictEqual(status, 'complete')
    // Verify it polled multiple times
    assert.ok(callCount >= 3, `expected at least 3 polls, got ${callCount}`)
  })

  test('wait_for with url_contains retries and succeeds on URL change', async () => {
    const { executeDOMAction } = await import('../../../extension/background/dom/dom-dispatch.js')

    let tabGetCount = 0
    globalThis.chrome.tabs.get = mock.fn((tabId) => {
      tabGetCount++
      // First 2 calls return /login, 3rd returns /dashboard
      const url = tabGetCount < 3
        ? 'http://localhost:3000/login'
        : 'http://localhost:3000/dashboard'
      return Promise.resolve({ id: tabId, windowId: 1, url })
    })

    const query = createPendingQuery({
      action: 'wait_for',
      url_contains: '/dashboard',
      timeout_ms: 2000
    })

    await executeDOMAction(query, 1, syncClient, sendAsyncResult, actionToast)

    assert.strictEqual(sendAsyncResult.mock.calls.length, 1)
    const [, , , status, result] = sendAsyncResult.mock.calls[0].arguments
    assert.strictEqual(status, 'complete')
    assert.ok(result.value.includes('/dashboard'))
    assert.ok(tabGetCount >= 3, `expected at least 3 tab.get calls, got ${tabGetCount}`)
  })

  // ── Error handling: chrome.tabs.get throws ──

  test('wait_for with url_contains handles chrome.tabs.get rejection', async () => {
    const { executeDOMAction } = await import('../../../extension/background/dom/dom-dispatch.js')

    globalThis.chrome.tabs.get = mock.fn(() => {
      return Promise.reject(new Error('No tab with id: 999'))
    })

    const query = createPendingQuery({
      action: 'wait_for',
      url_contains: '/dashboard',
      timeout_ms: 100
    })

    await executeDOMAction(query, 1, syncClient, sendAsyncResult, actionToast)

    assert.strictEqual(sendAsyncResult.mock.calls.length, 1)
    const [, , , status, , error] = sendAsyncResult.mock.calls[0].arguments
    assert.strictEqual(status, 'error')
    assert.ok(error)
  })
})
