// @ts-nocheck
/**
 * @fileoverview recording-shortcut-command.test.js — Tests keyboard shortcut
 * toggle behavior for action sequence recording (start/stop).
 */

import { beforeEach, describe, test, mock } from 'node:test'
import assert from 'node:assert'

let commandListener = null
let mockChrome = null

function setupChromeMock() {
  commandListener = null
  mockChrome = {
    commands: {
      onCommand: {
        addListener: mock.fn((fn) => {
          commandListener = fn
        })
      }
    },
    tabs: {
      query: mock.fn(async () => [{ id: 17 }]),
      sendMessage: mock.fn(async () => ({ ok: true }))
    }
  }
  globalThis.chrome = mockChrome
}

describe('recording shortcut command listener', () => {
  beforeEach(() => {
    mock.reset()
    setupChromeMock()
  })

  test('starts recording when idle', async () => {
    const { installRecordingShortcutCommandListener } = await import('../../../extension/background/ui/keyboard-shortcuts.js')
    const calls = []
    const handlers = {
      isRecording: () => false,
      startRecording: mock.fn(async (...args) => {
        calls.push(args)
        return { status: 'recording' }
      }),
      stopRecording: mock.fn(async () => ({ status: 'saved' }))
    }

    installRecordingShortcutCommandListener(handlers)
    assert.ok(commandListener, 'expected command listener to be registered')

    await commandListener('toggle_action_sequence_recording')

    assert.strictEqual(handlers.startRecording.mock.calls.length, 1)
    assert.strictEqual(handlers.stopRecording.mock.calls.length, 0)
    assert.ok(/^action-sequence--\d{4}-\d{2}-\d{2}-\d{6}$/.test(calls[0][0]), 'name should be timestamped slug')
    assert.strictEqual(calls[0][1], 15, 'default FPS should be 15')
    assert.strictEqual(calls[0][4], true, 'shortcut should use popup-style start path')
    assert.strictEqual(calls[0][5], 17, 'shortcut should target active tab')
  })

  test('stops recording when active', async () => {
    const { installRecordingShortcutCommandListener } = await import('../../../extension/background/ui/keyboard-shortcuts.js')
    const handlers = {
      isRecording: () => true,
      startRecording: mock.fn(async () => ({ status: 'recording' })),
      stopRecording: mock.fn(async () => ({ status: 'saved' }))
    }

    installRecordingShortcutCommandListener(handlers)
    assert.ok(commandListener, 'expected command listener to be registered')

    await commandListener('toggle_action_sequence_recording')

    assert.strictEqual(handlers.startRecording.mock.calls.length, 0)
    assert.strictEqual(handlers.stopRecording.mock.calls.length, 1)
    assert.strictEqual(handlers.stopRecording.mock.calls[0].arguments[0], false)
  })

  test('shows error toast when start fails', async () => {
    const { installRecordingShortcutCommandListener } = await import('../../../extension/background/ui/keyboard-shortcuts.js')
    const handlers = {
      isRecording: () => false,
      startRecording: mock.fn(async () => ({ status: 'error', error: 'permission denied' })),
      stopRecording: mock.fn(async () => ({ status: 'saved' }))
    }

    installRecordingShortcutCommandListener(handlers)
    await commandListener('toggle_action_sequence_recording')

    const toastCall = mockChrome.tabs.sendMessage.mock.calls.find(
      (c) => c.arguments[1]?.type === 'kaboom_action_toast'
    )
    assert.ok(toastCall, 'expected an error toast when shortcut start fails')
    assert.strictEqual(toastCall.arguments[1].text, 'Start recording failed')
    assert.ok(String(toastCall.arguments[1].detail).includes('permission denied'))
  })

  test('counts action_recording usage via the shared toggle helper (F6)', async () => {
    const { installRecordingShortcutCommandListener } = await import('../../../extension/background/ui/keyboard-shortcuts.js')
    const tracker = await import('../../../extension/background/ui/ui-usage-tracker.js')
    tracker.drainUIFeatures() // clear any prior state so this test reads only its own
    const handlers = {
      isRecording: () => false,
      startRecording: mock.fn(async () => ({ status: 'recording' })),
      stopRecording: mock.fn(async () => ({ status: 'saved' }))
    }

    installRecordingShortcutCommandListener(handlers)
    await commandListener('toggle_action_sequence_recording')

    const drained = tracker.drainUIFeatures()
    assert.ok(drained && drained.action_recording === true, 'shortcut should count action_recording usage')
  })

  test('ignores unrelated commands', async () => {
    const { installRecordingShortcutCommandListener } = await import('../../../extension/background/ui/keyboard-shortcuts.js')
    const handlers = {
      isRecording: () => false,
      startRecording: mock.fn(async () => ({ status: 'recording' })),
      stopRecording: mock.fn(async () => ({ status: 'saved' }))
    }

    installRecordingShortcutCommandListener(handlers)
    await commandListener('toggle_draw_mode')

    assert.strictEqual(handlers.startRecording.mock.calls.length, 0)
    assert.strictEqual(handlers.stopRecording.mock.calls.length, 0)
  })
})
