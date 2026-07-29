// @ts-nocheck
/**
 * @fileoverview toggle-overlay.test.js — Tests for overlay toggle behavior.
 * Covers Action Toasts and Subtitles toggles (content script overlay gates).
 *
 * These toggles are unique: they are cached in the content script layer (not
 * forwarded to inject). The content script uses local boolean caches to gate
 * toast/subtitle display, updated via runtime messages from background.
 *
 * Architecture tested:
 *   Popup toggle → Background → Content script cache → show/suppress overlay
 */

import { test, describe, mock, beforeEach } from 'node:test'
import assert from 'node:assert'
import { MANIFEST_VERSION } from '../shared/helpers.js'

// =============================================================================
// MOCK SETUP
// =============================================================================

let mockChrome
let mockDocument
let mockElements
let runtimeMessageHandler
let nextTimerId = 1
let scheduledTimeouts

function resetMocks() {
  mockElements = {}
  runtimeMessageHandler = null
  nextTimerId = 1
  scheduledTimeouts = new Map()

  mockChrome = {
    runtime: {
      id: 'test-extension-id',
      sendMessage: mock.fn(() => Promise.resolve()),
      onMessage: {
        addListener: mock.fn((handler) => {
          runtimeMessageHandler = handler
        })
      },
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
      query: mock.fn(() => Promise.resolve([{ id: 1 }])),
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
    removeEventListener: mock.fn(),
    createElement: mock.fn((tag) => createMockElement(tag)),
    head: { appendChild: mock.fn() },
    body: {
      appendChild: mock.fn(),
      removeChild: mock.fn()
    },
    documentElement: { appendChild: mock.fn() }
  }

  globalThis.chrome = mockChrome
  globalThis.document = mockDocument
  globalThis.requestAnimationFrame = (cb) => cb()
  globalThis.setTimeout = mock.fn((_handler, _delay) => {
    const timerId = nextTimerId++
    scheduledTimeouts.set(timerId, { handler: _handler, delay: _delay })
    return timerId
  })
  globalThis.clearTimeout = mock.fn((timerId) => {
    scheduledTimeouts.delete(timerId)
  })
}

function createMockElement(id) {
  return {
    id,
    textContent: '',
    innerHTML: '',
    className: '',
    classList: {
      add: mock.fn(),
      remove: mock.fn(),
      toggle: mock.fn()
    },
    style: {},
    addEventListener: mock.fn(),
    setAttribute: mock.fn(),
    getAttribute: mock.fn(),
    appendChild: mock.fn(),
    remove: mock.fn(),
    checked: false,
    disabled: false,
    offsetHeight: 0
  }
}

/** Simulate a message from background to the content script runtime listener */
function sendRuntimeMessage(message) {
  if (!runtimeMessageHandler) {
    throw new Error('No runtime message handler registered — call initRuntimeMessageListener first')
  }
  const sender = { id: mockChrome.runtime.id }
  const sendResponse = mock.fn()
  return runtimeMessageHandler(message, sender, sendResponse)
}

// =============================================================================
// ACTION TOASTS TOGGLE
// =============================================================================

describe('Action Toasts Toggle', () => {
  beforeEach(() => {
    mock.reset()
    resetMocks()
  })

  test('defaults to enabled (toasts shown)', async () => {
    // Storage returns empty → default is true
    mockChrome.storage.local.get.mock.mockImplementation((keys, callback) => callback({}))

    const { initRuntimeMessageListener } = await import('../../../extension/content/runtime-message-listener.js')
    initRuntimeMessageListener()

    // Send a toast message — should NOT be suppressed
    sendRuntimeMessage({
      type: 'kaboom_action_toast',
      text: 'Clicking button',
      detail: 'Submit form',
      state: 'trying'
    })

    // When suppressed, handler returns false immediately without creating DOM
    // When enabled, it also returns false but after creating the toast element
    // Check that createElement was called (toast was created)
    assert.ok(
      mockDocument.createElement.mock.calls.length > 0,
      'Toast element should be created when toasts are enabled'
    )
  })

  test('suppresses toasts when disabled via storage', async () => {
    mockChrome.storage.local.get.mock.mockImplementation((keys, callback) => callback({ actionToastsEnabled: false }))

    const { initRuntimeMessageListener } = await import('../../../extension/content/runtime-message-listener.js')
    initRuntimeMessageListener()

    // Reset createElement calls from init
    mockDocument.createElement.mock.resetCalls()

    sendRuntimeMessage({
      type: 'kaboom_action_toast',
      text: 'Should be suppressed',
      state: 'trying'
    })

    // No DOM element should be created
    assert.strictEqual(
      mockDocument.createElement.mock.calls.length,
      0,
      'Toast element should NOT be created when toasts are disabled'
    )
  })

  test('respects runtime toggle update from background', async () => {
    // Start enabled
    mockChrome.storage.local.get.mock.mockImplementation((keys, callback) => callback({}))

    const { initRuntimeMessageListener } = await import('../../../extension/content/runtime-message-listener.js')
    initRuntimeMessageListener()

    // Disable via runtime message (simulating user toggling in popup)
    sendRuntimeMessage({ type: 'set_action_toasts_enabled', enabled: false })

    mockDocument.createElement.mock.resetCalls()

    // Now toast should be suppressed
    sendRuntimeMessage({
      type: 'kaboom_action_toast',
      text: 'Should be suppressed',
      state: 'success'
    })

    assert.strictEqual(
      mockDocument.createElement.mock.calls.length,
      0,
      'Toast should be suppressed after disabling via runtime message'
    )
  })

  test('re-enables toasts after toggle ON', async () => {
    // Start disabled
    mockChrome.storage.local.get.mock.mockImplementation((keys, callback) => callback({ actionToastsEnabled: false }))

    const { initRuntimeMessageListener } = await import('../../../extension/content/runtime-message-listener.js')
    initRuntimeMessageListener()

    // Re-enable
    sendRuntimeMessage({ type: 'set_action_toasts_enabled', enabled: true })

    mockDocument.createElement.mock.resetCalls()

    sendRuntimeMessage({
      type: 'kaboom_action_toast',
      text: 'Should appear',
      state: 'success'
    })

    assert.ok(mockDocument.createElement.mock.calls.length > 0, 'Toast should appear after re-enabling')
  })

  test('popup sends correct message type when toggling', async () => {
    // Test the popup side: handleFeatureToggle sends the right message
    const { handleFeatureToggle } = await import('../../../extension/popup.js')

    handleFeatureToggle('actionToastsEnabled', 'set_action_toasts_enabled', false)

    const calls = mockChrome.runtime.sendMessage.mock.calls
    const toggleCall = calls.find((c) => c.arguments[0]?.type === 'set_action_toasts_enabled')
    assert.ok(toggleCall, 'Should send setActionToastsEnabled message')
    assert.strictEqual(toggleCall.arguments[0].enabled, false)
  })

  test('background forwards toggle to all content scripts', async () => {
    // Simulate background message handler behavior
    const mockDeps = {
      debugLog: mock.fn(),
      forwardToAllContentScripts: mock.fn()
    }

    const { installMessageListener } = await import('../../../extension/background/message-handlers.js')
    installMessageListener(mockDeps)

    // Get the installed message handler
    const bgHandler = mockChrome.runtime.onMessage.addListener.mock.calls[0]?.arguments[0]
    assert.ok(bgHandler, 'Background should install message listener')

    const sendResponse = mock.fn()
    bgHandler({ type: 'set_action_toasts_enabled', enabled: false }, { id: mockChrome.runtime.id }, sendResponse)

    assert.ok(
      mockDeps.forwardToAllContentScripts.mock.calls.some((c) => c.arguments[0]?.type === 'set_action_toasts_enabled'),
      'Background should forward toggle to content scripts'
    )
    assert.ok(
      sendResponse.mock.calls.some((c) => c.arguments[0]?.success === true),
      'Background should acknowledge with success'
    )
  })
})

// =============================================================================
// SUBTITLES TOGGLE
// =============================================================================

describe('Subtitles Toggle', () => {
  beforeEach(() => {
    mock.reset()
    resetMocks()
  })

  test('defaults to enabled (subtitles shown)', async () => {
    mockChrome.storage.local.get.mock.mockImplementation((keys, callback) => callback({}))

    const { initRuntimeMessageListener } = await import('../../../extension/content/runtime-message-listener.js')
    initRuntimeMessageListener()

    sendRuntimeMessage({
      type: 'kaboom_subtitle',
      text: 'Opening settings page'
    })

    // Subtitle creates/updates a div element
    assert.ok(
      mockDocument.createElement.mock.calls.length > 0 ||
        mockDocument.getElementById.mock.calls.some((c) => c.arguments[0] === 'kaboom-subtitle'),
      'Subtitle element should be created/accessed when enabled'
    )
  })

  test('suppresses subtitles when disabled via storage', async () => {
    mockChrome.storage.local.get.mock.mockImplementation((keys, callback) => callback({ subtitlesEnabled: false }))

    const { initRuntimeMessageListener } = await import('../../../extension/content/runtime-message-listener.js')
    initRuntimeMessageListener()

    mockDocument.createElement.mock.resetCalls()
    const result = sendRuntimeMessage({
      type: 'kaboom_subtitle',
      text: 'Should be suppressed'
    })

    // When disabled, handler returns false immediately — no DOM access
    assert.strictEqual(result, false, 'Handler should return false when disabled')
  })

  test('respects runtime toggle update from background', async () => {
    mockChrome.storage.local.get.mock.mockImplementation((keys, callback) => callback({}))

    const { initRuntimeMessageListener } = await import('../../../extension/content/runtime-message-listener.js')
    initRuntimeMessageListener()

    // Disable subtitles
    sendRuntimeMessage({ type: 'set_subtitles_enabled', enabled: false })

    const result = sendRuntimeMessage({
      type: 'kaboom_subtitle',
      text: 'Should be suppressed'
    })

    assert.strictEqual(result, false, 'Should suppress subtitle after disabling')
  })

  test('re-enables subtitles after toggle ON', async () => {
    mockChrome.storage.local.get.mock.mockImplementation((keys, callback) => callback({ subtitlesEnabled: false }))

    const { initRuntimeMessageListener } = await import('../../../extension/content/runtime-message-listener.js')
    initRuntimeMessageListener()

    // Re-enable
    sendRuntimeMessage({ type: 'set_subtitles_enabled', enabled: true })

    sendRuntimeMessage({
      type: 'kaboom_subtitle',
      text: 'Should appear now'
    })

    // Should attempt to create/access the subtitle element
    assert.ok(
      mockDocument.getElementById.mock.calls.some((c) => c.arguments[0] === 'kaboom-subtitle') ||
        mockDocument.createElement.mock.calls.length > 0,
      'Subtitle should be shown after re-enabling'
    )
  })

  test('clearing subtitle text removes element', async () => {
    mockChrome.storage.local.get.mock.mockImplementation((keys, callback) => callback({}))

    const { initRuntimeMessageListener } = await import('../../../extension/content/runtime-message-listener.js')
    initRuntimeMessageListener()

    // Create a subtitle first
    const subtitleEl = createMockElement('kaboom-subtitle')
    subtitleEl.style = { opacity: '1' }
    mockElements['kaboom-subtitle'] = subtitleEl

    // Send empty text to clear
    sendRuntimeMessage({ type: 'kaboom_subtitle', text: '' })

    // Should set opacity to 0 (fade out)
    assert.strictEqual(subtitleEl.style.opacity, '0', 'Should fade out subtitle on clear')
  })

  test('popup sends correct message type for subtitles toggle', async () => {
    const { handleFeatureToggle } = await import('../../../extension/popup.js')

    handleFeatureToggle('subtitlesEnabled', 'set_subtitles_enabled', true)

    const calls = mockChrome.runtime.sendMessage.mock.calls
    const toggleCall = calls.find((c) => c.arguments[0]?.type === 'set_subtitles_enabled')
    assert.ok(toggleCall, 'Should send setSubtitlesEnabled message')
    assert.strictEqual(toggleCall.arguments[0].enabled, true)
  })

  test('background forwards subtitles toggle to content scripts', async () => {
    const mockDeps = {
      debugLog: mock.fn(),
      forwardToAllContentScripts: mock.fn()
    }

    const { installMessageListener } = await import('../../../extension/background/message-handlers.js')
    installMessageListener(mockDeps)

    const bgHandler = mockChrome.runtime.onMessage.addListener.mock.calls[0]?.arguments[0]
    const sendResponse = mock.fn()

    bgHandler({ type: 'set_subtitles_enabled', enabled: true }, { id: mockChrome.runtime.id }, sendResponse)

    assert.ok(
      mockDeps.forwardToAllContentScripts.mock.calls.some((c) => c.arguments[0]?.type === 'set_subtitles_enabled'),
      'Should forward subtitles toggle to content scripts'
    )
  })
})

// =============================================================================
// OVERLAY TOGGLE EDGE CASES
// =============================================================================

describe('Overlay Toggle Edge Cases', () => {
  beforeEach(() => {
    mock.reset()
    resetMocks()
  })

  test('both overlays can be independently controlled', async () => {
    mockChrome.storage.local.get.mock.mockImplementation((keys, callback) => callback({}))

    const { initRuntimeMessageListener } = await import('../../../extension/content/runtime-message-listener.js')
    initRuntimeMessageListener()

    // Disable toasts, keep subtitles enabled
    sendRuntimeMessage({ type: 'set_action_toasts_enabled', enabled: false })

    // Toast should be suppressed
    mockDocument.createElement.mock.resetCalls()
    sendRuntimeMessage({
      type: 'kaboom_action_toast',
      text: 'Suppressed',
      state: 'trying'
    })
    assert.strictEqual(mockDocument.createElement.mock.calls.length, 0, 'Toast suppressed')

    // Subtitle should still work
    sendRuntimeMessage({ type: 'kaboom_subtitle', text: 'Still showing' })
    // Subtitle accesses getElementById for 'kaboom-subtitle' — that's how we know it ran
    assert.ok(
      mockDocument.getElementById.mock.calls.some((c) => c.arguments[0] === 'kaboom-subtitle'),
      'Subtitle still functional when only toasts disabled'
    )
  })

  test('storage values override defaults on init', async () => {
    // Both disabled in storage
    mockChrome.storage.local.get.mock.mockImplementation((keys, callback) =>
      callback({ actionToastsEnabled: false, subtitlesEnabled: false })
    )

    const { initRuntimeMessageListener } = await import('../../../extension/content/runtime-message-listener.js')
    initRuntimeMessageListener()

    mockDocument.createElement.mock.resetCalls()

    // Both should be suppressed
    sendRuntimeMessage({ type: 'kaboom_action_toast', text: 'Test', state: 'trying' })
    assert.strictEqual(mockDocument.createElement.mock.calls.length, 0, 'Toast suppressed from storage')

    const result = sendRuntimeMessage({ type: 'kaboom_subtitle', text: 'Test' })
    assert.strictEqual(result, false, 'Subtitle suppressed from storage')
  })

  test('toggle messages from untrusted sender are rejected', async () => {
    mockChrome.storage.local.get.mock.mockImplementation((keys, callback) => callback({}))

    const { initRuntimeMessageListener } = await import('../../../extension/content/runtime-message-listener.js')
    initRuntimeMessageListener()

    // Ensure toasts are enabled (ESM module state may leak from prior tests)
    sendRuntimeMessage({ type: 'set_action_toasts_enabled', enabled: true })

    // Send from a different extension ID (untrusted) — should be rejected
    const sendResponse = mock.fn()
    runtimeMessageHandler(
      { type: 'set_action_toasts_enabled', enabled: false },
      { id: 'different-extension-id' },
      sendResponse
    )

    // Should have been rejected — the toggle should still be enabled
    mockDocument.createElement.mock.resetCalls()
    sendRuntimeMessage({ type: 'kaboom_action_toast', text: 'Still works', state: 'trying' })
    assert.ok(
      mockDocument.createElement.mock.calls.length > 0,
      'Toast should still work after rejecting untrusted toggle message'
    )
  })

  test('toast with all states creates correct theme elements', async () => {
    mockChrome.storage.local.get.mock.mockImplementation((keys, callback) => callback({}))

    const { initRuntimeMessageListener } = await import('../../../extension/content/runtime-message-listener.js')
    initRuntimeMessageListener()

    // Ensure toasts are enabled (ESM module state may leak from prior tests)
    sendRuntimeMessage({ type: 'set_action_toasts_enabled', enabled: true })

    const states = ['trying', 'success', 'warning', 'error', 'audio']
    for (const state of states) {
      mockDocument.createElement.mock.resetCalls()
      sendRuntimeMessage({
        type: 'kaboom_action_toast',
        text: `Test ${state}`,
        state
      })
      assert.ok(mockDocument.createElement.mock.calls.length > 0, `Toast should be created for state: ${state}`)
    }
  })

  test('rapid toggle ON/OFF does not corrupt state', async () => {
    mockChrome.storage.local.get.mock.mockImplementation((keys, callback) => callback({}))

    const { initRuntimeMessageListener } = await import('../../../extension/content/runtime-message-listener.js')
    initRuntimeMessageListener()

    // Rapidly toggle 20 times
    for (let i = 0; i < 20; i++) {
      sendRuntimeMessage({ type: 'set_action_toasts_enabled', enabled: i % 2 === 0 })
    }

    // After 20 toggles (last was i=19, enabled = false)
    mockDocument.createElement.mock.resetCalls()
    sendRuntimeMessage({ type: 'kaboom_action_toast', text: 'Test', state: 'trying' })
    assert.strictEqual(
      mockDocument.createElement.mock.calls.length,
      0,
      'Should be disabled after even number of rapid toggles ending on OFF'
    )

    // Toggle back ON
    sendRuntimeMessage({ type: 'set_action_toasts_enabled', enabled: true })
    mockDocument.createElement.mock.resetCalls()
    sendRuntimeMessage({ type: 'kaboom_action_toast', text: 'Test', state: 'trying' })
    assert.ok(mockDocument.createElement.mock.calls.length > 0, 'Should be enabled after explicit ON')
  })

  test('rapid subtitle toggle ON/OFF does not corrupt state', async () => {
    mockChrome.storage.local.get.mock.mockImplementation((keys, callback) => callback({}))

    const { initRuntimeMessageListener } = await import('../../../extension/content/runtime-message-listener.js')
    initRuntimeMessageListener()

    // Rapidly toggle 20 times
    for (let i = 0; i < 20; i++) {
      sendRuntimeMessage({ type: 'set_subtitles_enabled', enabled: i % 2 === 0 })
    }

    // After 20 toggles (last was i=19, enabled = false)
    const result = sendRuntimeMessage({ type: 'kaboom_subtitle', text: 'Test' })
    assert.strictEqual(result, false, 'Subtitle should be disabled after rapid toggles ending OFF')

    // Toggle back ON
    sendRuntimeMessage({ type: 'set_subtitles_enabled', enabled: true })
    sendRuntimeMessage({ type: 'kaboom_subtitle', text: 'Should show' })
    assert.ok(
      mockDocument.getElementById.mock.calls.some((c) => c.arguments[0] === 'kaboom-subtitle'),
      'Subtitle should work after re-enabling'
    )
  })

  test('recording watermark is not affected by toast/subtitle toggles', async () => {
    mockChrome.storage.local.get.mock.mockImplementation((keys, callback) =>
      callback({ actionToastsEnabled: false, subtitlesEnabled: false })
    )

    const { initRuntimeMessageListener } = await import('../../../extension/content/runtime-message-listener.js')
    initRuntimeMessageListener()

    mockDocument.createElement.mock.resetCalls()

    // Recording watermark should still work even with overlays disabled
    sendRuntimeMessage({ type: 'kaboom_recording_watermark', visible: true })
    assert.ok(
      mockDocument.createElement.mock.calls.length > 0 ||
        mockDocument.getElementById.mock.calls.some((c) => c.arguments[0] === 'kaboom-recording-watermark'),
      'Recording watermark should not be affected by overlay toggles'
    )
  })
})
