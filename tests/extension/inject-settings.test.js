// @ts-nocheck
/**
 * @fileoverview inject-settings.test.js — Tests for inject/settings.js.
 * Covers: VALID_SETTINGS set, isValidSettingPayload validation,
 * handleSetting dispatch table, and handleStateCommand state operations.
 */

import { test, describe, mock, beforeEach, afterEach } from 'node:test'
import assert from 'node:assert'
import { createMockWindow } from './helpers.js'

// Provide esbuild constant
globalThis.__KABOOM_VERSION__ = 'test'

let originalWindow

// =============================================================================
// VALID_SETTINGS
// =============================================================================

describe('VALID_SETTINGS', () => {
  test('contains all expected inject-forwarded setting names', async () => {
    const { VALID_SETTINGS } = await import('../../extension/inject/settings.js')
    const { SettingName } = await import('../../extension/lib/constants.js')

    const expectedSettings = [
      SettingName.NETWORK_WATERFALL,
      SettingName.PERFORMANCE_MARKS,
      SettingName.ACTION_REPLAY,
      SettingName.WEBSOCKET_CAPTURE,
      SettingName.WEBSOCKET_CAPTURE_MODE,
      SettingName.PERFORMANCE_SNAPSHOT,
      SettingName.DEFERRAL,
      SettingName.NETWORK_BODY_CAPTURE,
      SettingName.SERVER_URL,
    ]

    for (const name of expectedSettings) {
      assert.ok(VALID_SETTINGS.has(name), `VALID_SETTINGS should contain '${name}'`)
    }
    assert.strictEqual(VALID_SETTINGS.size, expectedSettings.length, 'Should have exactly the expected number of settings')
  })

  test('does not contain content-only settings (ACTION_TOASTS, SUBTITLES)', async () => {
    const { VALID_SETTINGS } = await import('../../extension/inject/settings.js')
    const { SettingName } = await import('../../extension/lib/constants.js')

    assert.ok(!VALID_SETTINGS.has(SettingName.ACTION_TOASTS), 'ACTION_TOASTS should not be forwarded to inject')
    assert.ok(!VALID_SETTINGS.has(SettingName.SUBTITLES), 'SUBTITLES should not be forwarded to inject')
  })
})

// =============================================================================
// isValidSettingPayload
// =============================================================================

describe('isValidSettingPayload', () => {
  test('valid boolean setting payload returns true', async () => {
    const { isValidSettingPayload } = await import('../../extension/inject/settings.js')

    assert.strictEqual(
      isValidSettingPayload({ type: 'kaboom_setting', setting: 'set_network_waterfall_enabled', enabled: true }),
      true
    )
    assert.strictEqual(
      isValidSettingPayload({ type: 'kaboom_setting', setting: 'set_action_replay_enabled', enabled: false }),
      true
    )
  })

  test('invalid setting name returns false', async () => {
    const { isValidSettingPayload } = await import('../../extension/inject/settings.js')

    assert.strictEqual(
      isValidSettingPayload({ type: 'kaboom_setting', setting: 'setUnknownThing', enabled: true }),
      false
    )
  })

  test('missing enabled for boolean setting returns false', async () => {
    const { isValidSettingPayload } = await import('../../extension/inject/settings.js')

    assert.strictEqual(
      isValidSettingPayload({ type: 'kaboom_setting', setting: 'set_network_waterfall_enabled' }),
      false
    )
  })

  test('non-boolean enabled value returns false', async () => {
    const { isValidSettingPayload } = await import('../../extension/inject/settings.js')

    assert.strictEqual(
      isValidSettingPayload({ type: 'kaboom_setting', setting: 'set_network_waterfall_enabled', enabled: 'yes' }),
      false
    )
  })

  test('WebSocket capture mode requires string mode', async () => {
    const { isValidSettingPayload } = await import('../../extension/inject/settings.js')

    assert.strictEqual(
      isValidSettingPayload({ type: 'kaboom_setting', setting: 'set_web_socket_capture_mode', mode: 'high' }),
      true
    )
    assert.strictEqual(
      isValidSettingPayload({ type: 'kaboom_setting', setting: 'set_web_socket_capture_mode', mode: 123 }),
      false
    )
    assert.strictEqual(
      isValidSettingPayload({ type: 'kaboom_setting', setting: 'set_web_socket_capture_mode' }),
      false
    )
  })

  test('server URL requires string url', async () => {
    const { isValidSettingPayload } = await import('../../extension/inject/settings.js')

    assert.strictEqual(
      isValidSettingPayload({ type: 'kaboom_setting', setting: 'set_server_url', url: 'http://localhost:9999' }),
      true
    )
    assert.strictEqual(
      isValidSettingPayload({ type: 'kaboom_setting', setting: 'set_server_url', url: 123 }),
      false
    )
    assert.strictEqual(
      isValidSettingPayload({ type: 'kaboom_setting', setting: 'set_server_url' }),
      false
    )
  })
})

// =============================================================================
// handleSetting — dispatch table verification
// =============================================================================

describe('handleSetting', () => {
  test('dispatches network waterfall setting', async () => {
    const { handleSetting } = await import('../../extension/inject/settings.js')
    const { isNetworkWaterfallEnabled, setNetworkWaterfallEnabled } = await import('../../extension/lib/net/network.js')

    setNetworkWaterfallEnabled(false) // reset
    handleSetting({ setting: 'set_network_waterfall_enabled', enabled: true })
    assert.strictEqual(isNetworkWaterfallEnabled(), true, 'Should have enabled network waterfall')

    handleSetting({ setting: 'set_network_waterfall_enabled', enabled: false })
    assert.strictEqual(isNetworkWaterfallEnabled(), false, 'Should have disabled network waterfall')
  })

  test('dispatches network body capture setting', async () => {
    const { handleSetting } = await import('../../extension/inject/settings.js')
    const { isNetworkBodyCaptureEnabled, setNetworkBodyCaptureEnabled } = await import('../../extension/lib/net/network.js')

    setNetworkBodyCaptureEnabled(false) // reset
    handleSetting({ setting: 'set_network_body_capture_enabled', enabled: true })
    assert.strictEqual(isNetworkBodyCaptureEnabled(), true, 'Should have enabled network body capture')
  })

  test('dispatches performance marks setting', async () => {
    const { handleSetting } = await import('../../extension/inject/settings.js')
    const { isPerformanceMarksEnabled, setPerformanceMarksEnabled } = await import('../../extension/lib/performance.js')

    setPerformanceMarksEnabled(false) // reset
    handleSetting({ setting: 'set_performance_marks_enabled', enabled: true })
    assert.strictEqual(isPerformanceMarksEnabled(), true, 'Should have enabled performance marks')
  })

  test('dispatches performance snapshot setting', async () => {
    const { handleSetting } = await import('../../extension/inject/settings.js')
    const { isPerformanceSnapshotEnabled } = await import('../../extension/lib/perf-snapshot.js')

    handleSetting({ setting: 'set_performance_snapshot_enabled', enabled: true })
    assert.strictEqual(isPerformanceSnapshotEnabled(), true)

    handleSetting({ setting: 'set_performance_snapshot_enabled', enabled: false })
    assert.strictEqual(isPerformanceSnapshotEnabled(), false)
  })

  test('dispatches WebSocket capture mode setting', async () => {
    const { handleSetting } = await import('../../extension/inject/settings.js')
    const { getWebSocketCaptureMode } = await import('../../extension/lib/net/websocket.js')

    handleSetting({ setting: 'set_web_socket_capture_mode', mode: 'high' })
    assert.strictEqual(getWebSocketCaptureMode(), 'high')

    handleSetting({ setting: 'set_web_socket_capture_mode', mode: 'low' })
    assert.strictEqual(getWebSocketCaptureMode(), 'low')
  })

  test('defaults WebSocket capture mode to medium when mode is missing', async () => {
    const { handleSetting } = await import('../../extension/inject/settings.js')
    const { getWebSocketCaptureMode } = await import('../../extension/lib/net/websocket.js')

    handleSetting({ setting: 'set_web_socket_capture_mode' })
    assert.strictEqual(getWebSocketCaptureMode(), 'medium')
  })

  test('dispatches deferral setting', async () => {
    const { handleSetting } = await import('../../extension/inject/settings.js')
    const { getDeferralState } = await import('../../extension/inject/observers.js')

    handleSetting({ setting: 'set_deferral_enabled', enabled: true })
    assert.strictEqual(getDeferralState().deferralEnabled, true)

    handleSetting({ setting: 'set_deferral_enabled', enabled: false })
    assert.strictEqual(getDeferralState().deferralEnabled, false)
  })

  test('unknown setting name is silently ignored', async () => {
    const { handleSetting } = await import('../../extension/inject/settings.js')

    // Should not throw
    assert.doesNotThrow(() => {
      handleSetting({ setting: 'setNonExistentSetting', enabled: true })
    })
  })
})

// =============================================================================
// handleStateCommand
// =============================================================================

describe('handleStateCommand', () => {
  beforeEach(() => {
    originalWindow = globalThis.window
    globalThis.window = createMockWindow({ href: 'http://localhost:3000/test' })
  })

  afterEach(() => {
    globalThis.window = originalWindow
  })

  test('capture action calls captureStateFn and posts result', async () => {
    const { handleStateCommand } = await import('../../extension/inject/settings.js')

    const capturedState = { url: 'http://localhost:3000', cookies: [], localStorage: {} }
    const captureFn = mock.fn(() => capturedState)
    const restoreFn = mock.fn()

    handleStateCommand(
      { type: 'kaboom_state_command', messageId: 'msg-1', action: 'capture' },
      captureFn,
      restoreFn
    )

    assert.strictEqual(captureFn.mock.calls.length, 1, 'Should call captureStateFn')
    assert.strictEqual(restoreFn.mock.calls.length, 0, 'Should not call restoreStateFn')

    const postCalls = globalThis.window.postMessage.mock.calls
    assert.strictEqual(postCalls.length, 1)
    const msg = postCalls[0].arguments[0]
    assert.strictEqual(msg.type, 'kaboom_state_response')
    assert.strictEqual(msg.messageId, 'msg-1')
    assert.deepStrictEqual(msg.result, capturedState)
  })

  test('restore action calls restoreStateFn with state and include_url', async () => {
    const { handleStateCommand } = await import('../../extension/inject/settings.js')

    const stateToRestore = { url: 'http://localhost:3000', cookies: [], localStorage: {} }
    const captureFn = mock.fn()
    const restoreFn = mock.fn(() => ({ success: true }))

    handleStateCommand(
      { type: 'kaboom_state_command', messageId: 'msg-2', action: 'restore', state: stateToRestore, include_url: true },
      captureFn,
      restoreFn
    )

    assert.strictEqual(captureFn.mock.calls.length, 0)
    assert.strictEqual(restoreFn.mock.calls.length, 1)
    assert.deepStrictEqual(restoreFn.mock.calls[0].arguments[0], stateToRestore)
    assert.strictEqual(restoreFn.mock.calls[0].arguments[1], true, 'include_url should be true')

    const msg = globalThis.window.postMessage.mock.calls[0].arguments[0]
    assert.strictEqual(msg.type, 'kaboom_state_response')
    assert.strictEqual(msg.messageId, 'msg-2')
    assert.deepStrictEqual(msg.result, { success: true })
  })

  test('restore action defaults include_url to true when not specified', async () => {
    const { handleStateCommand } = await import('../../extension/inject/settings.js')

    const restoreFn = mock.fn()
    handleStateCommand(
      { type: 'kaboom_state_command', messageId: 'msg-3', action: 'restore', state: { url: '/' } },
      mock.fn(),
      restoreFn
    )

    assert.strictEqual(restoreFn.mock.calls[0].arguments[1], true, 'include_url should default to true')
  })

  test('invalid action returns error response', async () => {
    const { handleStateCommand } = await import('../../extension/inject/settings.js')

    handleStateCommand(
      { type: 'kaboom_state_command', messageId: 'msg-4', action: 'delete' },
      mock.fn(),
      mock.fn()
    )

    const msg = globalThis.window.postMessage.mock.calls[0].arguments[0]
    assert.strictEqual(msg.type, 'kaboom_state_response')
    assert.strictEqual(msg.messageId, 'msg-4')
    assert.ok(msg.result.error.includes('Invalid action'))
  })

  test('restore with missing state returns error response', async () => {
    const { handleStateCommand } = await import('../../extension/inject/settings.js')

    handleStateCommand(
      { type: 'kaboom_state_command', messageId: 'msg-5', action: 'restore' },
      mock.fn(),
      mock.fn()
    )

    const msg = globalThis.window.postMessage.mock.calls[0].arguments[0]
    assert.strictEqual(msg.type, 'kaboom_state_response')
    assert.ok(msg.result.error.includes('Invalid state object'))
  })

  test('captureStateFn exception returns error response', async () => {
    const { handleStateCommand } = await import('../../extension/inject/settings.js')

    const captureFn = mock.fn(() => { throw new Error('DOM not ready') })

    handleStateCommand(
      { type: 'kaboom_state_command', messageId: 'msg-6', action: 'capture' },
      captureFn,
      mock.fn()
    )

    const msg = globalThis.window.postMessage.mock.calls[0].arguments[0]
    assert.strictEqual(msg.type, 'kaboom_state_response')
    assert.strictEqual(msg.result.error, 'DOM not ready')
  })

  test('posts response to window.location.origin', async () => {
    const { handleStateCommand } = await import('../../extension/inject/settings.js')

    handleStateCommand(
      { type: 'kaboom_state_command', messageId: 'msg-7', action: 'capture' },
      mock.fn(() => ({})),
      mock.fn()
    )

    const targetOrigin = globalThis.window.postMessage.mock.calls[0].arguments[1]
    assert.strictEqual(targetOrigin, 'http://localhost:3000', 'Should post to window.location.origin')
  })
})
