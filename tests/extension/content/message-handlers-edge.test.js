// @ts-nocheck
/**
 * @fileoverview message-handlers-edge.test.js — Edge case tests for message routing:
 * forwarded settings, source map toggle, server URL change, draw mode.
 */

import { test, describe, mock, beforeEach as _beforeEach } from 'node:test'
import assert from 'node:assert'
import { MANIFEST_VERSION as _MANIFEST_VERSION } from '../shared/helpers.js'

const { installMessageListener } = await import('../../../extension/background/message-handlers.js')
const { composeBackgroundHandlers } = await import('../shared/background-message-router.js')

function getInstalledHandler(depsOverrides = {}) {
  const addListenerFn = mock.fn()
  const origOnMessage = chrome.runtime.onMessage
  chrome.runtime.onMessage = { addListener: addListenerFn }

  const defaultDeps = {
    debugLog: mock.fn(),
    addToWsBatcher: mock.fn(),
    addToEnhancedActionBatcher: mock.fn(),
    addToNetworkBodyBatcher: mock.fn(),
    addToPerfBatcher: mock.fn(),
    addToLogBatcher: mock.fn(),
    handleLogMessage: mock.fn(() => Promise.resolve()),
    handleClearLogs: mock.fn(() => Promise.resolve({ success: true })),
    isNetworkBodyCaptureDisabled: mock.fn(() => false),
    getConnectionStatus: mock.fn(() => ({ connected: true })),
    getServerUrl: mock.fn(() => 'http://localhost:9222'),
    getScreenshotOnError: mock.fn(() => false),
    getSourceMapEnabled: mock.fn(() => false),
    getDebugMode: mock.fn(() => false),
    getContextWarning: mock.fn(() => null),
    getCircuitBreakerState: mock.fn(() => 'closed'),
    getMemoryPressureState: mock.fn(() => 'normal'),
    setCurrentLogLevel: mock.fn(),
    setScreenshotOnError: mock.fn(),
    setSourceMapEnabled: mock.fn(),
    setDebugMode: mock.fn(),
    setServerUrl: mock.fn(),
    setAiWebPilotEnabled: mock.fn((val, cb) => cb && cb()),
    getAiWebPilotEnabled: mock.fn(() => false),
    saveSetting: mock.fn(),
    exportDebugLog: mock.fn(() => []),
    clearDebugLog: mock.fn(),
    clearSourceMapCache: mock.fn(),
    captureScreenshot: mock.fn(() => Promise.resolve({ success: true })),
    forwardToAllContentScripts: mock.fn(),
    checkConnectionAndUpdate: mock.fn(),
    ...depsOverrides
  }

  installMessageListener({ debugLog: defaultDeps.debugLog, handlers: composeBackgroundHandlers(defaultDeps) })
  chrome.runtime.onMessage = origOnMessage

  const handler = addListenerFn.mock.calls[0].arguments[0]
  return { handler, deps: defaultDeps }
}

const extensionSender = { id: chrome.runtime.id }
const contentScriptSender = { tab: { id: 1, url: 'http://localhost:3000' } }

// ============================================
// Forwarded settings
// ============================================

describe('forwarded settings', () => {
  const forwardedTypes = [
    'set_network_waterfall_enabled',
    'set_performance_marks_enabled',
    'set_action_replay_enabled',
    'set_web_socket_capture_enabled',
    'set_web_socket_capture_mode',
    'set_performance_snapshot_enabled',
    'set_deferral_enabled',
    'set_network_body_capture_enabled',
    'set_action_toasts_enabled',
    'set_subtitles_enabled'
  ]

  for (const msgType of forwardedTypes) {
    test(`${msgType} forwards to content scripts`, () => {
      const { handler, deps } = getInstalledHandler()
      const sendResponse = mock.fn()
      handler({ type: msgType, enabled: true }, extensionSender, sendResponse)
      assert.strictEqual(deps.forwardToAllContentScripts.mock.calls.length, 1)
      assert.strictEqual(sendResponse.mock.calls[0].arguments[0].success, true)
    })
  }
})

// ============================================
// setSourceMapEnabled
// ============================================

describe('set_source_map_enabled', () => {
  test('enabling does not clear cache', () => {
    const { handler, deps } = getInstalledHandler()
    const sendResponse = mock.fn()
    handler({ type: 'set_source_map_enabled', enabled: true }, extensionSender, sendResponse)
    assert.strictEqual(deps.setSourceMapEnabled.mock.calls[0].arguments[0], true)
    assert.strictEqual(deps.clearSourceMapCache.mock.calls.length, 0)
    assert.strictEqual(sendResponse.mock.calls[0].arguments[0].success, true)
  })

  test('disabling clears cache', () => {
    const { handler, deps } = getInstalledHandler()
    const sendResponse = mock.fn()
    handler({ type: 'set_source_map_enabled', enabled: false }, extensionSender, sendResponse)
    assert.strictEqual(deps.setSourceMapEnabled.mock.calls[0].arguments[0], false)
    assert.strictEqual(deps.clearSourceMapCache.mock.calls.length, 1)
  })
})

// ============================================
// setScreenshotOnError
// ============================================

describe('set_screenshot_on_error', () => {
  test('persists setting', () => {
    const { handler, deps } = getInstalledHandler()
    const sendResponse = mock.fn()
    handler({ type: 'set_screenshot_on_error', enabled: true }, extensionSender, sendResponse)
    assert.strictEqual(deps.setScreenshotOnError.mock.calls[0].arguments[0], true)
    assert.ok(deps.saveSetting.mock.calls.some(
      c => c.arguments[0] === 'screenshotOnError' && c.arguments[1] === true
    ))
    assert.strictEqual(sendResponse.mock.calls[0].arguments[0].success, true)
  })
})

// ============================================
// setServerUrl
// ============================================

describe('set_server_url', () => {
  test('updates server URL and re-checks connection', () => {
    const { handler, deps } = getInstalledHandler({
      getServerUrl: mock.fn(() => 'http://localhost:8080')
    })
    const sendResponse = mock.fn()
    handler({ type: 'set_server_url', url: 'http://localhost:8080' }, extensionSender, sendResponse)
    assert.strictEqual(deps.setServerUrl.mock.calls[0].arguments[0], 'http://localhost:8080')
    assert.strictEqual(deps.checkConnectionAndUpdate.mock.calls.length, 1)
    assert.strictEqual(deps.forwardToAllContentScripts.mock.calls.length, 1)
    assert.strictEqual(sendResponse.mock.calls[0].arguments[0].success, true)
  })

  test('defaults to localhost:7890 for empty URL', () => {
    const { handler, deps } = getInstalledHandler({
      getServerUrl: mock.fn(() => 'http://localhost:7890')
    })
    const sendResponse = mock.fn()
    handler({ type: 'set_server_url', url: '' }, extensionSender, sendResponse)
    assert.strictEqual(deps.setServerUrl.mock.calls[0].arguments[0], 'http://localhost:7890')
  })
})

// ============================================
// log message async handling
// ============================================

describe('log message handling', () => {
  test('log message is handled asynchronously as fire-and-forget', async () => {
    const { handler, deps } = getInstalledHandler()
    const sendResponse = mock.fn()
    const result = handler(
      { type: 'log', payload: { level: 'error', message: 'test' }, tabId: 1 },
      contentScriptSender,
      sendResponse
    )
    // Regression: returning true without ever calling sendResponse rejects
    // awaited senders with "message port closed" — log is fire-and-forget.
    assert.strictEqual(result, false, 'should return false for fire-and-forget handling')
    await new Promise(r => setTimeout(r, 10))
    assert.strictEqual(deps.handleLogMessage.mock.calls.length, 1)
    assert.strictEqual(sendResponse.mock.calls.length, 0, 'no response is sent for log messages')
  })
})

describe('message handler diagnostics', () => {
  test('logs and responds when a feature owner throws unexpectedly', () => {
    const addListenerFn = mock.fn()
    const originalOnMessage = chrome.runtime.onMessage
    chrome.runtime.onMessage = { addListener: addListenerFn }
    const debugLog = mock.fn()
    installMessageListener({
      debugLog,
      handlers: [{ feature: 'broken', handle: () => { throw new Error('private detail') } }]
    })
    chrome.runtime.onMessage = originalOnMessage
    const handler = addListenerFn.mock.calls[0].arguments[0]
    const sendResponse = mock.fn()

    const result = handler({ type: 'get_status' }, extensionSender, sendResponse)

    assert.strictEqual(result, false)
    assert.deepStrictEqual(sendResponse.mock.calls[0].arguments[0], {
      success: false,
      error: 'message_handler_failed'
    })
    const diagnostic = debugLog.mock.calls.find((call) => call.arguments[1] === 'Runtime message handler failed')
    assert.ok(diagnostic)
    assert.match(diagnostic.arguments[2].correlation_id, /^runtime_message_/)
    assert.strictEqual(diagnostic.arguments[2].message_type, 'get_status')
    assert.strictEqual(diagnostic.arguments[2].error, 'private detail')
  })
})

// ============================================
// getAiWebPilotEnabled
// ============================================

describe('get_ai_web_pilot_enabled', () => {
  test('returns current state', () => {
    const { handler } = getInstalledHandler({
      getAiWebPilotEnabled: mock.fn(() => true)
    })
    const sendResponse = mock.fn()
    handler({ type: 'get_ai_web_pilot_enabled' }, extensionSender, sendResponse)
    assert.strictEqual(sendResponse.mock.calls[0].arguments[0].enabled, true)
  })
})

// ============================================
// clearLogs error handling
// ============================================

describe('clearLogs error handling', () => {
  test('returns error on handler failure', async () => {
    const { handler } = getInstalledHandler({
      handleClearLogs: mock.fn(() => Promise.reject(new Error('disk full')))
    })
    const sendResponse = mock.fn()
    handler({ type: 'clear_logs' }, extensionSender, sendResponse)
    await new Promise(r => setTimeout(r, 10))
    assert.ok(sendResponse.mock.calls[0].arguments[0].error.includes('disk full'))
  })
})
