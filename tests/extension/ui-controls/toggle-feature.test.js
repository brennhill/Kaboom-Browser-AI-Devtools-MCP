// @ts-nocheck
/**
 * @fileoverview toggle-feature.test.js — Comprehensive feature toggle tests.
 * Covers: toggle persistence, default states, capture gate ON/OFF,
 * source maps toggle, content→inject forwarding, popup↔options sync,
 * and MCP feedback when capture is disabled.
 *
 * Architecture under test:
 *   Popup UI → Background (single writer) → Content → Inject (capture gates)
 */

import { test, describe, mock, beforeEach } from 'node:test'
import assert from 'node:assert'
import { applyFeatureTogglesFromMockStorage, MANIFEST_VERSION } from '../shared/helpers.js'
import { composeBackgroundHandlers } from '../shared/background-message-router.js'

// =============================================================================
// MOCK SETUP
// =============================================================================

let mockChrome
let mockDocument
let mockElements

function resetMocks() {
  mockElements = {}

  mockChrome = {
    runtime: {
      id: 'test-extension-id',
      sendMessage: mock.fn((msg, callback) => {
        if (typeof callback === 'function') callback({ success: true })
        return Promise.resolve()
      }),
      onMessage: { addListener: mock.fn() },
      getManifest: () => ({ version: MANIFEST_VERSION }),
      getURL: mock.fn((path) => `chrome-extension://test-id/${path}`)
    },
    storage: {
      local: {
        get: mock.fn((keys, callback) => callback({})),
        set: mock.fn((data, callback) => callback && callback()),
        remove: mock.fn((keys, callback) => {
          if (typeof callback === 'function') callback()
          else return Promise.resolve()
        })
      },
      sync: {
        get: mock.fn((keys, callback) => callback({})),
        set: mock.fn((data, callback) => callback && callback())
      },
      session: {
        get: mock.fn((keys, callback) => callback({})),
        set: mock.fn((data, callback) => callback && callback())
      },
      onChanged: { addListener: mock.fn() }
    },
    tabs: {
      query: mock.fn((queryInfo, callback) => callback([{ id: 1, url: 'http://localhost:3000' }])),
      sendMessage: mock.fn(() => Promise.resolve()),
      onRemoved: { addListener: mock.fn() }
    }
  }

  mockDocument = {
    getElementById: mock.fn((id) => {
      if (!mockElements[id]) {
        mockElements[id] = createMockElement(id)
      }
      return mockElements[id]
    }),
    querySelector: mock.fn(() => null),
    querySelectorAll: mock.fn(() => []),
    addEventListener: mock.fn(),
    createElement: mock.fn((tag) => createMockElement(tag)),
    head: { appendChild: mock.fn() },
    body: { appendChild: mock.fn() },
    documentElement: { appendChild: mock.fn() }
  }

  globalThis.chrome = mockChrome
  globalThis.document = mockDocument
}

function createMockElement(id) {
  return {
    id,
    textContent: '',
    innerHTML: '',
    className: '',
    classList: { add: mock.fn(), remove: mock.fn(), toggle: mock.fn() },
    style: {},
    addEventListener: mock.fn(),
    setAttribute: mock.fn(),
    getAttribute: mock.fn(),
    appendChild: mock.fn(),
    remove: mock.fn(),
    checked: false,
    disabled: false,
    value: ''
  }
}

// =============================================================================
// ALL FEATURE TOGGLES — DEFAULT STATE & PERSISTENCE
// =============================================================================

describe('Feature Toggle Defaults and Persistence', () => {
  beforeEach(() => {
    mock.reset()
    resetMocks()
  })

  test('FEATURE_TOGGLES contains all 9 expected toggles', async () => {
    const { FEATURE_TOGGLES } = await import('../../../extension/popup/feature-toggles.js')

    const expectedIds = [
      'toggle-websocket',
      'toggle-network-waterfall',
      'toggle-performance-marks',
      'toggle-action-replay',
      'toggle-screenshot',
      'toggle-source-maps',
      'toggle-network-body-capture',
      'toggle-action-toasts',
      'toggle-subtitles'
    ]

    assert.strictEqual(FEATURE_TOGGLES.length, expectedIds.length, 'Should have exactly 9 toggles')
    for (const expectedId of expectedIds) {
      assert.ok(
        FEATURE_TOGGLES.some((t) => t.id === expectedId),
        `Missing toggle: ${expectedId}`
      )
    }
  })

  test('all toggles default to true', async () => {
    const { FEATURE_TOGGLES } = await import('../../../extension/popup/feature-toggles.js')

    for (const toggle of FEATURE_TOGGLES) {
      assert.strictEqual(toggle.default, true, `Toggle ${toggle.id} should default to true`)
    }
  })

  test('all toggles load saved state from storage on init', async () => {
    // Save all as disabled
    const savedState = {
      webSocketCaptureEnabled: false,
      networkWaterfallEnabled: false,
      performanceMarksEnabled: false,
      actionReplayEnabled: false,
      screenshotOnError: false,
      sourceMapEnabled: false,
      networkBodyCaptureEnabled: false,
      actionToastsEnabled: false,
      subtitlesEnabled: false
    }
    mockChrome.storage.local.get.mock.mockImplementation((keys, callback) => callback(savedState))

    await applyFeatureTogglesFromMockStorage()

    // Each checkbox should be set to false
    const { FEATURE_TOGGLES } = await import('../../../extension/popup/feature-toggles.js')
    for (const toggle of FEATURE_TOGGLES) {
      const el = mockDocument.getElementById(toggle.id)
      assert.strictEqual(el.checked, false, `Toggle ${toggle.id} should load saved disabled state`)
    }
  })

  test('all toggles default to checked when storage is empty', async () => {
    mockChrome.storage.local.get.mock.mockImplementation((keys, callback) => callback({}))

    await applyFeatureTogglesFromMockStorage()

    const { FEATURE_TOGGLES } = await import('../../../extension/popup/feature-toggles.js')
    for (const toggle of FEATURE_TOGGLES) {
      const el = mockDocument.getElementById(toggle.id)
      assert.strictEqual(el.checked, true, `Toggle ${toggle.id} should default to checked when no saved state`)
    }
  })

  test('each toggle registers a change event handler', async () => {
    mockChrome.storage.local.get.mock.mockImplementation((keys, callback) => callback({}))

    await applyFeatureTogglesFromMockStorage()

    const { FEATURE_TOGGLES } = await import('../../../extension/popup/feature-toggles.js')
    for (const toggle of FEATURE_TOGGLES) {
      const el = mockDocument.getElementById(toggle.id)
      const changeListeners = el.addEventListener.mock.calls.filter((c) => c.arguments[0] === 'change')
      assert.ok(changeListeners.length > 0, `Toggle ${toggle.id} should have a 'change' event listener`)
    }
  })

  test('each toggle sends message via runtime.sendMessage (not storage)', async () => {
    const { handleFeatureToggle, FEATURE_TOGGLES } = await import('../../../extension/popup/feature-toggles.js')

    for (const toggle of FEATURE_TOGGLES) {
      mockChrome.runtime.sendMessage.mock.resetCalls()
      handleFeatureToggle(toggle.storageKey, toggle.messageType, false)

      const messageSent = mockChrome.runtime.sendMessage.mock.calls.some(
        (c) => c.arguments[0]?.type === toggle.messageType && c.arguments[0]?.enabled === false
      )
      assert.ok(messageSent, `Toggle ${toggle.id} should send ${toggle.messageType} message`)
    }

    // CRITICAL: Popup should NEVER call storage.local.set directly for toggles
    const storageSets = mockChrome.storage.local.set.mock.calls
    assert.strictEqual(
      storageSets.length,
      0,
      'Popup must NEVER write toggle state to storage directly (single source of truth)'
    )
  })
})

// =============================================================================
// SOURCE MAPS TOGGLE
// =============================================================================

describe('Source Maps Toggle', () => {
  beforeEach(() => {
    mock.reset()
    resetMocks()
  })

  test('toggle in FEATURE_TOGGLES has correct config', async () => {
    const { FEATURE_TOGGLES } = await import('../../../extension/popup/feature-toggles.js')

    const sourceMapToggle = FEATURE_TOGGLES.find((t) => t.id === 'toggle-source-maps')
    assert.ok(sourceMapToggle, 'Source maps toggle should exist')
    assert.strictEqual(sourceMapToggle.storageKey, 'sourceMapEnabled')
    assert.strictEqual(sourceMapToggle.messageType, 'set_source_map_enabled')
    assert.strictEqual(sourceMapToggle.default, true)
  })

  test('background clears source map cache on disable', async () => {
    const mockDeps = {
      debugLog: mock.fn(),
      forwardToAllContentScripts: mock.fn(),
      setSourceMapEnabled: mock.fn(),
      saveSetting: mock.fn(),
      clearSourceMapCache: mock.fn()
    }

    const { installMessageListener } = await import('../../../extension/background/message-handlers.js')
    installMessageListener({ debugLog: mockDeps.debugLog, handlers: composeBackgroundHandlers(mockDeps) })

    const bgHandler = mockChrome.runtime.onMessage.addListener.mock.calls[0]?.arguments[0]
    const sendResponse = mock.fn()

    // Disable source maps
    bgHandler({ type: 'set_source_map_enabled', enabled: false }, { id: mockChrome.runtime.id }, sendResponse)

    assert.ok(
      mockDeps.setSourceMapEnabled.mock.calls.some((c) => c.arguments[0] === false),
      'Should call setSourceMapEnabled(false)'
    )
    assert.ok(
      mockDeps.saveSetting.mock.calls.some((c) => c.arguments[0] === 'sourceMapEnabled' && c.arguments[1] === false),
      'Should save source map setting to storage'
    )
    assert.ok(mockDeps.clearSourceMapCache.mock.calls.length > 0, 'Should clear source map cache when disabled')
  })

  test('background does NOT clear cache on enable', async () => {
    const mockDeps = {
      debugLog: mock.fn(),
      forwardToAllContentScripts: mock.fn(),
      setSourceMapEnabled: mock.fn(),
      saveSetting: mock.fn(),
      clearSourceMapCache: mock.fn()
    }

    const { installMessageListener } = await import('../../../extension/background/message-handlers.js')
    installMessageListener({ debugLog: mockDeps.debugLog, handlers: composeBackgroundHandlers(mockDeps) })

    const bgHandler = mockChrome.runtime.onMessage.addListener.mock.calls[0]?.arguments[0]
    const sendResponse = mock.fn()

    // Enable source maps
    bgHandler({ type: 'set_source_map_enabled', enabled: true }, { id: mockChrome.runtime.id }, sendResponse)

    assert.ok(
      mockDeps.setSourceMapEnabled.mock.calls.some((c) => c.arguments[0] === true),
      'Should call setSourceMapEnabled(true)'
    )
    assert.strictEqual(
      mockDeps.clearSourceMapCache.mock.calls.length,
      0,
      'Should NOT clear source map cache when enabling'
    )
  })
})

// =============================================================================
// CAPTURE GATE ON/OFF TESTS
// =============================================================================

describe('Capture Gate Functions', () => {
  beforeEach(() => {
    mock.reset()
    resetMocks()
  })

  test('actions: recordAction is gated by actionCaptureEnabled', async () => {
    const { recordAction, getActionBuffer, clearActionBuffer, setActionCaptureEnabled } =
      await import('../../../extension/lib/page/actions.js')

    // Enabled by default
    clearActionBuffer()
    recordAction({ type: 'click', selector: '#btn' })
    assert.ok(getActionBuffer().length > 0, 'Action should be recorded when enabled')

    // Disable
    setActionCaptureEnabled(false)
    clearActionBuffer()
    recordAction({ type: 'click', selector: '#btn2' })
    assert.strictEqual(getActionBuffer().length, 0, 'Action should NOT be recorded when disabled')

    // Re-enable
    setActionCaptureEnabled(true)
    recordAction({ type: 'click', selector: '#btn3' })
    assert.ok(getActionBuffer().length > 0, 'Action should be recorded after re-enabling')
  })

  test('actions: disabling clears the action buffer', async () => {
    const { recordAction, getActionBuffer, clearActionBuffer, setActionCaptureEnabled } =
      await import('../../../extension/lib/page/actions.js')

    setActionCaptureEnabled(true)
    clearActionBuffer()
    recordAction({ type: 'click', selector: '#btn' })
    assert.ok(getActionBuffer().length > 0, 'Buffer should have entries')

    setActionCaptureEnabled(false)
    assert.strictEqual(getActionBuffer().length, 0, 'Buffer should be cleared on disable')
  })

  test('network: setNetworkWaterfallEnabled toggles capture gate', async () => {
    const { setNetworkWaterfallEnabled, isNetworkWaterfallEnabled } = await import('../../../extension/lib/net/network.js')

    setNetworkWaterfallEnabled(true)
    assert.strictEqual(isNetworkWaterfallEnabled(), true)

    setNetworkWaterfallEnabled(false)
    assert.strictEqual(isNetworkWaterfallEnabled(), false)

    setNetworkWaterfallEnabled(true)
    assert.strictEqual(isNetworkWaterfallEnabled(), true)
  })

  test('network: setNetworkBodyCaptureEnabled toggles body capture gate', async () => {
    const { setNetworkBodyCaptureEnabled, isNetworkBodyCaptureEnabled } = await import('../../../extension/lib/net/network.js')

    setNetworkBodyCaptureEnabled(true)
    assert.strictEqual(isNetworkBodyCaptureEnabled(), true)

    setNetworkBodyCaptureEnabled(false)
    assert.strictEqual(isNetworkBodyCaptureEnabled(), false)
  })

  test('performance: setPerformanceMarksEnabled toggles capture gate', async () => {
    const { setPerformanceMarksEnabled, isPerformanceMarksEnabled } = await import('../../../extension/lib/analysis/performance.js')

    setPerformanceMarksEnabled(true)
    assert.strictEqual(isPerformanceMarksEnabled(), true)

    setPerformanceMarksEnabled(false)
    assert.strictEqual(isPerformanceMarksEnabled(), false)

    setPerformanceMarksEnabled(true)
    assert.strictEqual(isPerformanceMarksEnabled(), true)
  })

  test('websocket: setWebSocketCaptureEnabled toggles capture gate', async () => {
    const { setWebSocketCaptureEnabled } = await import('../../../extension/lib/net/websocket.js')

    // Just verify no error on toggle — the internal state isn't exposed via getter
    assert.doesNotThrow(() => setWebSocketCaptureEnabled(false))
    assert.doesNotThrow(() => setWebSocketCaptureEnabled(true))
  })
})

// =============================================================================
// CONTENT SCRIPT TOGGLE FORWARDING
// =============================================================================

describe('Content Script Toggle Forwarding', () => {
  beforeEach(() => {
    mock.reset()
    resetMocks()
    globalThis.window = {
      postMessage: mock.fn(),
      location: { origin: 'http://localhost:3000', href: 'http://localhost:3000/' },
      addEventListener: mock.fn(),
      removeEventListener: mock.fn()
    }
  })

  test('TOGGLE_MESSAGES contains all capture toggle types', async () => {
    const { TOGGLE_MESSAGES } = await import('../../../extension/content/message-handlers.js')

    const expected = [
      'set_network_waterfall_enabled',
      'set_performance_marks_enabled',
      'set_action_replay_enabled',
      'set_web_socket_capture_enabled',
      'set_web_socket_capture_mode',
      'set_performance_snapshot_enabled',
      'set_deferral_enabled',
      'set_network_body_capture_enabled',
      'set_server_url'
    ]

    for (const msgType of expected) {
      assert.ok(TOGGLE_MESSAGES.has(msgType), `TOGGLE_MESSAGES should include '${msgType}'`)
    }
  })

  test('handleToggleMessage forwards boolean setting via postMessage', async () => {
    const { handleToggleMessage } = await import('../../../extension/content/message-handlers.js')

    handleToggleMessage({ type: 'set_network_waterfall_enabled', enabled: false })

    const postCalls = globalThis.window.postMessage.mock.calls
    assert.ok(postCalls.length > 0, 'Should call window.postMessage')

    const payload = postCalls[0].arguments[0]
    assert.strictEqual(payload.type, 'kaboom_setting')
    assert.strictEqual(payload.setting, 'set_network_waterfall_enabled')
    assert.strictEqual(payload.enabled, false)

    // Verify targetOrigin is set (not wildcard)
    const targetOrigin = postCalls[0].arguments[1]
    assert.ok(targetOrigin && targetOrigin !== '*', 'Should use explicit targetOrigin, not "*"')
  })

  test('handleToggleMessage forwards mode for WebSocket capture mode', async () => {
    const { handleToggleMessage } = await import('../../../extension/content/message-handlers.js')

    handleToggleMessage({ type: 'set_web_socket_capture_mode', mode: 'high' })

    const postCalls = globalThis.window.postMessage.mock.calls
    const payload = postCalls[0].arguments[0]
    assert.strictEqual(payload.setting, 'set_web_socket_capture_mode')
    assert.strictEqual(payload.mode, 'high')
  })

  test('handleToggleMessage forwards URL for server URL setting', async () => {
    const { handleToggleMessage } = await import('../../../extension/content/message-handlers.js')

    handleToggleMessage({ type: 'set_server_url', url: 'http://localhost:9999' })

    const postCalls = globalThis.window.postMessage.mock.calls
    const payload = postCalls[0].arguments[0]
    assert.strictEqual(payload.setting, 'set_server_url')
    assert.strictEqual(payload.url, 'http://localhost:9999')
  })

  test('handleToggleMessage ignores unknown message types', async () => {
    const { handleToggleMessage } = await import('../../../extension/content/message-handlers.js')

    handleToggleMessage({ type: 'unknownMessage', enabled: true })

    assert.strictEqual(globalThis.window.postMessage.mock.calls.length, 0, 'Should not forward unknown message types')
  })

  test('action toasts and subtitles are NOT in TOGGLE_MESSAGES', async () => {
    const { TOGGLE_MESSAGES } = await import('../../../extension/content/message-handlers.js')

    assert.ok(
      !TOGGLE_MESSAGES.has('set_action_toasts_enabled'),
      'Action toasts should NOT be forwarded to inject (handled in content script)'
    )
    assert.ok(
      !TOGGLE_MESSAGES.has('set_subtitles_enabled'),
      'Subtitles should NOT be forwarded to inject (handled in content script)'
    )
  })
})

// =============================================================================
// BACKGROUND MESSAGE ROUTING
// =============================================================================

describe('Background Toggle Routing', () => {
  let mockDeps
  let bgHandler

  beforeEach(() => {
    mock.reset()
    resetMocks()

    mockDeps = {
      debugLog: mock.fn(),
      forwardToAllContentScripts: mock.fn(),
      setScreenshotOnError: mock.fn(),
      saveSetting: mock.fn(),
      setSourceMapEnabled: mock.fn(),
      clearSourceMapCache: mock.fn(),
      setDebugMode: mock.fn(),
      setCurrentLogLevel: mock.fn(),
      setServerUrl: mock.fn(),
      getServerUrl: mock.fn(() => 'http://localhost:7890'),
      checkConnectionAndUpdate: mock.fn(),
      addToLogBatcher: mock.fn(),
      addToWsBatcher: mock.fn(),
      addToEnhancedActionBatcher: mock.fn(),
      addToNetworkBodyBatcher: mock.fn(),
      addToPerfBatcher: mock.fn(),
      isNetworkBodyCaptureDisabled: mock.fn(() => false),
      handleLogMessage: mock.fn(),
      getConnectionStatus: mock.fn(() => ({ connected: true })),
      getScreenshotOnError: mock.fn(() => true),
      getSourceMapEnabled: mock.fn(() => true),
      getDebugMode: mock.fn(() => false),
      getContextWarning: mock.fn(() => null),
      getCircuitBreakerState: mock.fn(() => 'closed'),
      getMemoryPressureState: mock.fn(() => ({ memoryPressureLevel: 'normal' })),
      setAiWebPilotEnabled: mock.fn(),
      getAiWebPilotEnabled: mock.fn(() => true),
      exportDebugLog: mock.fn(() => []),
      clearDebugLog: mock.fn(),
      captureScreenshot: mock.fn()
    }
  })

  async function installAndGetHandler() {
    const { installMessageListener } = await import('../../../extension/background/message-handlers.js')
    installMessageListener({ debugLog: mockDeps.debugLog, handlers: composeBackgroundHandlers(mockDeps) })
    return mockChrome.runtime.onMessage.addListener.mock.calls[0]?.arguments[0]
  }

  test('forwarded settings go through handleForwardedSetting', async () => {
    bgHandler = await installAndGetHandler()

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

    for (const type of forwardedTypes) {
      mockDeps.forwardToAllContentScripts.mock.resetCalls()
      const sendResponse = mock.fn()
      const msg = type === 'set_web_socket_capture_mode' ? { type, mode: 'high' } : { type, enabled: true }

      bgHandler(msg, { id: mockChrome.runtime.id }, sendResponse)

      assert.ok(
        mockDeps.forwardToAllContentScripts.mock.calls.length > 0,
        `${type} should be forwarded to content scripts`
      )
      assert.ok(
        sendResponse.mock.calls.some((c) => c.arguments[0]?.success === true),
        `${type} should respond with success`
      )
    }
  })

  test('screenshot toggle is handled specially (not forwarded)', async () => {
    bgHandler = await installAndGetHandler()
    const sendResponse = mock.fn()

    bgHandler({ type: 'set_screenshot_on_error', enabled: false }, { id: mockChrome.runtime.id }, sendResponse)

    assert.ok(
      mockDeps.setScreenshotOnError.mock.calls.some((c) => c.arguments[0] === false),
      'Should call setScreenshotOnError'
    )
    assert.ok(
      mockDeps.saveSetting.mock.calls.some((c) => c.arguments[0] === 'screenshotOnError'),
      'Should save screenshot setting'
    )
  })

  test('debug mode toggle is handled specially (not forwarded)', async () => {
    bgHandler = await installAndGetHandler()
    const sendResponse = mock.fn()

    bgHandler({ type: 'set_debug_mode', enabled: true }, { id: mockChrome.runtime.id }, sendResponse)

    assert.ok(
      mockDeps.setDebugMode.mock.calls.some((c) => c.arguments[0] === true),
      'Should call setDebugMode'
    )
    assert.ok(
      mockDeps.saveSetting.mock.calls.some((c) => c.arguments[0] === 'debugMode'),
      'Should save debug mode setting'
    )
  })

  test('rejects messages from untrusted senders', async () => {
    bgHandler = await installAndGetHandler()
    const sendResponse = mock.fn()

    bgHandler({ type: 'set_debug_mode', enabled: true }, { id: 'different-extension' }, sendResponse)

    assert.strictEqual(mockDeps.setDebugMode.mock.calls.length, 0, 'Should NOT process message from untrusted sender')
  })

  test('network body drop when capture disabled', async () => {
    mockDeps.isNetworkBodyCaptureDisabled.mock.mockImplementation(() => true)
    bgHandler = await installAndGetHandler()
    const sendResponse = mock.fn()

    bgHandler(
      { type: 'network_body', payload: { url: 'http://example.com/api', body: '{}' } },
      { tab: { id: 1, url: 'http://localhost:3000' } },
      sendResponse
    )

    assert.strictEqual(
      mockDeps.addToNetworkBodyBatcher.mock.calls.length,
      0,
      'Should drop network body when capture disabled'
    )
    assert.ok(
      mockDeps.debugLog.mock.calls.some((c) => c.arguments[0] === 'capture' && c.arguments[1]?.includes('dropped')),
      'Should log that network body was dropped'
    )
  })
})

// =============================================================================
// TOGGLE RAPID SWITCHING — NO RACE CONDITIONS
// =============================================================================

describe('Rapid Toggle Switching', () => {
  beforeEach(() => {
    mock.reset()
    resetMocks()
  })

  test('rapid capture gate toggling settles to final state', async () => {
    const { setNetworkWaterfallEnabled, isNetworkWaterfallEnabled } = await import('../../../extension/lib/net/network.js')

    // Toggle 100 times rapidly
    for (let i = 0; i < 100; i++) {
      setNetworkWaterfallEnabled(i % 2 === 0)
    }

    // i=99 → enabled = (99 % 2 === 0) → false
    assert.strictEqual(isNetworkWaterfallEnabled(), false, 'Should settle to last value (false)')

    setNetworkWaterfallEnabled(true)
    assert.strictEqual(isNetworkWaterfallEnabled(), true, 'Should accept final explicit true')
  })

  test('rapid action capture toggling preserves buffer integrity', async () => {
    const { recordAction, getActionBuffer, clearActionBuffer, setActionCaptureEnabled } =
      await import('../../../extension/lib/page/actions.js')

    clearActionBuffer()
    setActionCaptureEnabled(true)

    // Alternate enable/disable with records in between
    for (let i = 0; i < 50; i++) {
      if (i % 2 === 0) {
        setActionCaptureEnabled(true)
        recordAction({ type: 'click', selector: `#btn-${i}` })
      } else {
        setActionCaptureEnabled(false)
        recordAction({ type: 'click', selector: `#btn-${i}` })
      }
    }

    // Buffer should only contain entries from enabled phases
    const buffer = getActionBuffer()
    // Each disable clears buffer, so only the last enabled phase matters
    // Last iteration: i=49 (odd) → disabled, buffer cleared
    assert.strictEqual(buffer.length, 0, 'Buffer should be empty after ending disabled')
  })

  test('rapid popup message sends do not duplicate', async () => {
    const { handleFeatureToggle } = await import('../../../extension/popup/feature-toggles.js')

    // Rapidly toggle 20 times
    for (let i = 0; i < 20; i++) {
      handleFeatureToggle('networkWaterfallEnabled', 'set_network_waterfall_enabled', i % 2 === 0)
    }

    // Should have exactly 20 sends (one per toggle)
    const calls = mockChrome.runtime.sendMessage.mock.calls.filter(
      (c) => c.arguments[0]?.type === 'set_network_waterfall_enabled'
    )
    assert.strictEqual(calls.length, 20, 'Should send exactly one message per toggle action')
  })
})

// =============================================================================
// FEATURE TOGGLE CONFIG INTEGRITY
// =============================================================================

describe('Feature Toggle Config Integrity', () => {
  beforeEach(() => {
    mock.reset()
    resetMocks()
  })

  test('each toggle has unique id', async () => {
    const { FEATURE_TOGGLES } = await import('../../../extension/popup/feature-toggles.js')

    const ids = FEATURE_TOGGLES.map((t) => t.id)
    const uniqueIds = new Set(ids)
    assert.strictEqual(uniqueIds.size, ids.length, 'All toggle IDs must be unique')
  })

  test('each toggle has unique storageKey', async () => {
    const { FEATURE_TOGGLES } = await import('../../../extension/popup/feature-toggles.js')

    const keys = FEATURE_TOGGLES.map((t) => t.storageKey)
    const uniqueKeys = new Set(keys)
    assert.strictEqual(uniqueKeys.size, keys.length, 'All storage keys must be unique')
  })

  test('each toggle has unique messageType', async () => {
    const { FEATURE_TOGGLES } = await import('../../../extension/popup/feature-toggles.js')

    const types = FEATURE_TOGGLES.map((t) => t.messageType)
    const uniqueTypes = new Set(types)
    assert.strictEqual(uniqueTypes.size, types.length, 'All message types must be unique')
  })

  test('storageKey and messageType follow naming convention', async () => {
    const { FEATURE_TOGGLES } = await import('../../../extension/popup/feature-toggles.js')

    for (const toggle of FEATURE_TOGGLES) {
      // storageKey should be camelCase
      assert.ok(/^[a-z][a-zA-Z]+$/.test(toggle.storageKey), `storageKey '${toggle.storageKey}' should be camelCase`)
      // messageType should start with 'set'
      assert.ok(toggle.messageType.startsWith('set'), `messageType '${toggle.messageType}' should start with 'set'`)
    }
  })
})
