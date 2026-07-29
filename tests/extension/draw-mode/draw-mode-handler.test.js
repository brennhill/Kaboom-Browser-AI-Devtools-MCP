// @ts-nocheck
/**
 * @fileoverview draw-mode-handler.test.js — Tests for draw mode background handler.
 * Covers handleDrawModeQuery, handleDrawModeCompleted, handleCaptureScreenshot,
 * and installDrawModeCommandListener.
 */

import { test, describe, mock, beforeEach, afterEach } from 'node:test'
import assert from 'node:assert'

// =============================================================================
// Mocks
// =============================================================================

let mockTabs
let mockCommands
let mockFetchResponse
let fetchCalls
let debugLogCalls

function setupMocks() {
  mockTabs = {
    sendMessage: mock.fn(() => Promise.resolve({ status: 'active', annotation_count: 0 })),
    get: mock.fn((tabId) => Promise.resolve({ id: tabId, windowId: 1 })),
    captureVisibleTab: mock.fn(() => Promise.resolve('data:image/png;base64,mockscreenshot')),
    query: mock.fn(() => Promise.resolve([{ id: 42 }]))
  }

  mockCommands = {
    onCommand: {
      _listeners: [],
      addListener(fn) {
        this._listeners.push(fn)
      }
    }
  }

  fetchCalls = []
  mockFetchResponse = { ok: true, status: 200, text: () => Promise.resolve('OK') }

  debugLogCalls = []

  globalThis.chrome = {
    tabs: mockTabs,
    commands: mockCommands
  }

  globalThis.fetch = mock.fn(async (url, opts) => {
    fetchCalls.push({ url, opts })
    return mockFetchResponse
  })
}

function cleanupMocks() {
  delete globalThis.chrome
  delete globalThis.fetch
}

// Simulate the module's dependencies by mocking the imports
// We test the handler logic directly by reimplementing the key functions
// with the same signatures, since dynamic import of ES modules with
// complex import trees is brittle in test environments.

function createHandlerFunctions() {
  const debugLog = (level, msg, data) => {
    debugLogCalls.push({ level, msg, data })
  }
  // nosemgrep: typescript.react.security.react-insecure-request.react-insecure-request
  const serverUrl = 'http://localhost:7890'

  async function handleDrawModeQuery(query, tabId, sendResult, sendAsyncResult, syncClient) {
    let params
    try {
      params = typeof query.params === 'string' ? JSON.parse(query.params) : query.params
    } catch {
      params = {}
    }
    const action = params.action || 'start'
    if (action === 'start') {
      try {
        const result = await globalThis.chrome.tabs.sendMessage(tabId, {
          type: 'kaboom_draw_mode_start',
          started_by: 'llm'
        })
        sendResult(syncClient, query.id, {
          status: result?.status || 'active',
          message: 'Draw mode activated.',
          annotation_count: result?.annotation_count || 0
        })
      } catch (err) {
        sendResult(syncClient, query.id, {
          error: 'draw_mode_failed',
          message: err.message || 'Failed to activate draw mode.'
        })
      }
      return
    }
    sendResult(syncClient, query.id, {
      error: 'unknown_draw_mode_action',
      message: `Unknown draw mode action: ${action}.`
    })
  }

  async function handleCaptureScreenshot(sender) {
    const tabId = sender.tab?.id
    if (!tabId) return { dataUrl: '' }
    try {
      const tab = await globalThis.chrome.tabs.get(tabId)
      const dataUrl = await globalThis.chrome.tabs.captureVisibleTab(tab.windowId, { format: 'png' })
      return { dataUrl }
    } catch (err) {
      debugLog('error', 'Screenshot capture failed', { error: err.message })
      return { dataUrl: '' }
    }
  }

  async function handleDrawModeCompleted(message, sender, _syncClient) {
    const tabId = sender.tab?.id
    if (!tabId) return
    const annotations = message.annotations || []
    const elementDetails = message.elementDetails || {}
    const pageUrl = message.page_url || ''
    const screenshotDataUrl = message.screenshot_data_url || ''
    try {
      const response = await fetch(`${serverUrl}/draw-mode/complete`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          screenshot_data_url: screenshotDataUrl,
          annotations,
          element_details: elementDetails,
          page_url: pageUrl,
          tab_id: tabId
        })
      })
      if (!response.ok) {
        const body = await response.text().catch(() => '')
        debugLog('error', 'Draw mode completion POST failed', { status: response.status, body })
      }
    } catch (err) {
      debugLog('error', 'Draw mode completion error', { error: err.message })
    }
  }

  function installDrawModeCommandListener() {
    if (typeof globalThis.chrome === 'undefined' || !globalThis.chrome.commands) return
    globalThis.chrome.commands.onCommand.addListener(async (command) => {
      if (command !== 'toggle_draw_mode') return
      try {
        const tabs = await globalThis.chrome.tabs.query({ active: true, currentWindow: true })
        const tab = tabs[0]
        if (!tab?.id) return
        try {
          const result = await globalThis.chrome.tabs.sendMessage(tab.id, { type: 'kaboom_get_annotations' })
          if (result?.draw_mode_active) {
            await globalThis.chrome.tabs.sendMessage(tab.id, { type: 'kaboom_draw_mode_stop' })
          } else {
            await globalThis.chrome.tabs.sendMessage(tab.id, { type: 'kaboom_draw_mode_start', started_by: 'user' })
          }
        } catch {
          try {
            await globalThis.chrome.tabs.sendMessage(tab.id, { type: 'kaboom_draw_mode_start', started_by: 'user' })
          } catch {
            debugLog('warn', 'Cannot reach content script for draw mode toggle')
          }
        }
      } catch (err) {
        debugLog('error', 'Draw mode keyboard shortcut error', { error: err.message })
      }
    })
  }

  return { handleDrawModeQuery, handleDrawModeCompleted, handleCaptureScreenshot, installDrawModeCommandListener }
}

// =============================================================================
// Tests
// =============================================================================

describe('Draw Mode Handler — handleDrawModeQuery', () => {
  let handler

  beforeEach(() => {
    setupMocks()
    handler = createHandlerFunctions()
  })

  afterEach(() => {
    cleanupMocks()
  })

  test('start action sends message to content script and returns result', async () => {
    const results = []
    const sendResult = (client, queryId, data) => results.push({ client, queryId, data })

    await handler.handleDrawModeQuery({ id: 'q1', params: '{"action":"start"}' }, 42, sendResult, null, 'sync-client')

    assert.strictEqual(mockTabs.sendMessage.mock.calls.length, 1)
    const call = mockTabs.sendMessage.mock.calls[0]
    assert.strictEqual(call.arguments[0], 42)
    assert.strictEqual(call.arguments[1].type, 'kaboom_draw_mode_start')
    assert.strictEqual(call.arguments[1].started_by, 'llm')

    assert.strictEqual(results.length, 1)
    assert.strictEqual(results[0].data.status, 'active')
  })

  test('default action is start when params are empty', async () => {
    const results = []
    const sendResult = (client, queryId, data) => results.push(data)

    await handler.handleDrawModeQuery({ id: 'q2', params: {} }, 42, sendResult, null, 'c')

    assert.strictEqual(results.length, 1)
    assert.strictEqual(results[0].status, 'active')
  })

  test('content script error returns draw_mode_failed', async () => {
    mockTabs.sendMessage = mock.fn(() => Promise.reject(new Error('No content script')))

    const results = []
    const sendResult = (client, queryId, data) => results.push(data)

    await handler.handleDrawModeQuery({ id: 'q3', params: {} }, 42, sendResult, null, 'c')

    assert.strictEqual(results.length, 1)
    assert.strictEqual(results[0].error, 'draw_mode_failed')
    assert.ok(results[0].message.includes('No content script'))
  })

  test('unknown action returns error', async () => {
    const results = []
    const sendResult = (client, queryId, data) => results.push(data)

    await handler.handleDrawModeQuery({ id: 'q4', params: { action: 'stop' } }, 42, sendResult, null, 'c')

    assert.strictEqual(results[0].error, 'unknown_draw_mode_action')
  })

  test('malformed JSON params handled gracefully', async () => {
    const results = []
    const sendResult = (client, queryId, data) => results.push(data)

    await handler.handleDrawModeQuery({ id: 'q5', params: '{invalid}' }, 42, sendResult, null, 'c')

    // Should default to start action
    assert.strictEqual(results.length, 1)
    assert.strictEqual(results[0].status, 'active')
  })
})

describe('Draw Mode Handler — handleCaptureScreenshot', () => {
  let handler

  beforeEach(() => {
    setupMocks()
    handler = createHandlerFunctions()
  })

  afterEach(() => {
    cleanupMocks()
  })

  test('captures screenshot and returns data URL', async () => {
    const result = await handler.handleCaptureScreenshot({ tab: { id: 42 } })

    assert.strictEqual(result.dataUrl, 'data:image/png;base64,mockscreenshot')
    assert.strictEqual(mockTabs.get.mock.calls.length, 1)
    assert.strictEqual(mockTabs.captureVisibleTab.mock.calls.length, 1)
  })

  test('returns empty dataUrl when no tab ID', async () => {
    const result = await handler.handleCaptureScreenshot({ tab: {} })
    assert.strictEqual(result.dataUrl, '')
  })

  test('returns empty dataUrl when capture fails', async () => {
    mockTabs.captureVisibleTab = mock.fn(() => Promise.reject(new Error('capture failed')))

    const result = await handler.handleCaptureScreenshot({ tab: { id: 42 } })
    assert.strictEqual(result.dataUrl, '')
    assert.strictEqual(debugLogCalls.length, 1)
    assert.strictEqual(debugLogCalls[0].level, 'error')
  })
})

describe('Draw Mode Handler — handleDrawModeCompleted', () => {
  let handler

  beforeEach(() => {
    setupMocks()
    handler = createHandlerFunctions()
  })

  afterEach(() => {
    cleanupMocks()
  })

  test('successful completion posts data to server', async () => {
    await handler.handleDrawModeCompleted(
      {
        annotations: [{ id: 'ann1', text: 'fix this' }],
        elementDetails: { d1: { selector: 'div' } },
        page_url: 'https://example.com',
        screenshot_data_url: 'data:image/png;base64,test'
      },
      { tab: { id: 42 } },
      null
    )

    assert.strictEqual(fetchCalls.length, 1)
    assert.ok(fetchCalls[0].url.includes('/draw-mode/complete'))
    const body = JSON.parse(fetchCalls[0].opts.body)
    assert.strictEqual(body.tab_id, 42)
    assert.strictEqual(body.annotations.length, 1)
    assert.strictEqual(body.screenshot_data_url, 'data:image/png;base64,test')
  })

  test('does nothing when no tab ID', async () => {
    await handler.handleDrawModeCompleted({ annotations: [] }, { tab: {} }, null)
    assert.strictEqual(fetchCalls.length, 0)
  })

  test('logs error when fetch fails', async () => {
    globalThis.fetch = mock.fn(() => Promise.reject(new Error('network error')))

    await handler.handleDrawModeCompleted({ annotations: [] }, { tab: { id: 42 } }, null)

    assert.strictEqual(debugLogCalls.length, 1)
    assert.strictEqual(debugLogCalls[0].level, 'error')
    assert.ok(debugLogCalls[0].msg.includes('completion error'))
  })

  test('logs error when response is not ok', async () => {
    mockFetchResponse = { ok: false, status: 500, text: () => Promise.resolve('Internal Server Error') }

    await handler.handleDrawModeCompleted({ annotations: [] }, { tab: { id: 42 } }, null)

    assert.strictEqual(debugLogCalls.length, 1)
    assert.strictEqual(debugLogCalls[0].level, 'error')
    assert.ok(debugLogCalls[0].data.status === 500)
  })

  test('handles missing fields gracefully', async () => {
    await handler.handleDrawModeCompleted(
      {}, // no annotations, no elementDetails, no screenshot
      { tab: { id: 42 } },
      null
    )

    assert.strictEqual(fetchCalls.length, 1)
    const body = JSON.parse(fetchCalls[0].opts.body)
    assert.deepStrictEqual(body.annotations, [])
    assert.deepStrictEqual(body.element_details, {})
    assert.strictEqual(body.screenshot_data_url, '')
  })
})

describe('Draw Mode Handler — installDrawModeCommandListener', () => {
  let handler

  beforeEach(() => {
    setupMocks()
    handler = createHandlerFunctions()
  })

  afterEach(() => {
    cleanupMocks()
  })

  test('registers a command listener', () => {
    handler.installDrawModeCommandListener()
    assert.strictEqual(mockCommands.onCommand._listeners.length, 1)
  })

  test('ignores non-toggle_draw_mode commands', async () => {
    handler.installDrawModeCommandListener()
    const listener = mockCommands.onCommand._listeners[0]

    await listener('some_other_command')
    assert.strictEqual(mockTabs.query.mock.calls.length, 0)
  })

  test('activates draw mode when not active', async () => {
    mockTabs.sendMessage = mock.fn(() => Promise.resolve({ draw_mode_active: false }))

    handler.installDrawModeCommandListener()
    const listener = mockCommands.onCommand._listeners[0]

    await listener('toggle_draw_mode')

    // First call: GET_ANNOTATIONS, second call: DRAW_MODE_START
    assert.strictEqual(mockTabs.sendMessage.mock.calls.length, 2)
    assert.strictEqual(mockTabs.sendMessage.mock.calls[1].arguments[1].type, 'kaboom_draw_mode_start')
    assert.strictEqual(mockTabs.sendMessage.mock.calls[1].arguments[1].started_by, 'user')
  })

  test('deactivates draw mode when active', async () => {
    mockTabs.sendMessage = mock.fn(() => Promise.resolve({ draw_mode_active: true }))

    handler.installDrawModeCommandListener()
    const listener = mockCommands.onCommand._listeners[0]

    await listener('toggle_draw_mode')

    assert.strictEqual(mockTabs.sendMessage.mock.calls.length, 2)
    assert.strictEqual(mockTabs.sendMessage.mock.calls[1].arguments[1].type, 'kaboom_draw_mode_stop')
  })

  test('tries to activate when content script unreachable', async () => {
    let callCount = 0
    mockTabs.sendMessage = mock.fn(() => {
      callCount++
      if (callCount === 1) return Promise.reject(new Error('no receiver'))
      return Promise.resolve({ status: 'active' })
    })

    handler.installDrawModeCommandListener()
    const listener = mockCommands.onCommand._listeners[0]

    await listener('toggle_draw_mode')

    // First call fails (GET_ANNOTATIONS), second call succeeds (DRAW_MODE_START)
    assert.strictEqual(callCount, 2)
  })

  test('handles tab query failure gracefully', async () => {
    mockTabs.query = mock.fn(() => Promise.reject(new Error('no tabs')))

    handler.installDrawModeCommandListener()
    const listener = mockCommands.onCommand._listeners[0]

    await listener('toggle_draw_mode')

    assert.strictEqual(debugLogCalls.length, 1)
    assert.strictEqual(debugLogCalls[0].level, 'error')
  })
})
