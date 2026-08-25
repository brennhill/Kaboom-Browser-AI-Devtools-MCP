// @ts-nocheck
/**
 * @fileoverview Successful recording lifecycle, offscreen, tracking, and toast tests.
 *
 * NOTE: recording.js imports from ./index.js and ./event-listeners.js,
 * so a comprehensive chrome mock is required before import. The module also
 * has side effects at load time (clearing stale recording state, installing
 * message listeners behind a chrome runtime guard).
 */

import { test, describe, mock, afterEach } from 'node:test'
import assert from 'node:assert'
import {
  createRecordingChromeMock,
  getRecordingInfo,
  isRecording,
  simulateOffscreenStarted,
  simulateOffscreenStopped,
  startRecording,
  stopRecording,
} from './recording-fixture.js'

describe('Successful Recording Lifecycle', () => {
  afterEach(async () => {
    // Ensure cleanup
    if (isRecording()) {
      const stopPromise = stopRecording()
      await new Promise((r) => setTimeout(r, 50))
      simulateOffscreenStopped()
      await stopPromise.catch(() => {})
    }
  })

  test('should complete start-stop lifecycle successfully', async () => {
    globalThis.chrome = createRecordingChromeMock()

    // START
    const startPromise = startRecording('lifecycle-test', 15, '', { queryId: 'q1', fromPopup: true })
    await new Promise((r) => setTimeout(r, 50))
    simulateOffscreenStarted(true)
    const startResult = await startPromise

    assert.strictEqual(startResult.status, 'recording')
    assert.strictEqual(startResult.name, 'lifecycle-test')
    assert.ok(typeof startResult.startTime === 'number')
    assert.ok(startResult.startTime > 0)
    assert.strictEqual(isRecording(), true)

    // Verify recording info
    const info = getRecordingInfo()
    assert.strictEqual(info.active, true)
    assert.strictEqual(info.name, 'lifecycle-test')
    assert.ok(info.startTime > 0)

    // STOP
    const stopPromise = stopRecording()
    await new Promise((r) => setTimeout(r, 50))
    simulateOffscreenStopped({
      status: 'saved',
      name: 'lifecycle-test',
      duration_seconds: 15,
      size_bytes: 2048000,
      path: '/tmp/lifecycle-test.webm'
    })
    const stopResult = await stopPromise

    assert.strictEqual(stopResult.status, 'saved')
    assert.strictEqual(stopResult.name, 'lifecycle-test')
    assert.strictEqual(stopResult.duration_seconds, 15)
    assert.strictEqual(stopResult.size_bytes, 2048000)
    assert.strictEqual(stopResult.path, '/tmp/lifecycle-test.webm')
    assert.strictEqual(isRecording(), false)

    // Verify state is fully reset
    const infoAfterStop = getRecordingInfo()
    assert.strictEqual(infoAfterStop.active, false)
    assert.strictEqual(infoAfterStop.name, '')
    assert.strictEqual(infoAfterStop.startTime, 0)
  })

  test('should persist recording state to storage for popup sync', async () => {
    globalThis.chrome = createRecordingChromeMock()

    const startPromise = startRecording('popup-sync-test', 15, '', { fromPopup: true })
    await new Promise((r) => setTimeout(r, 50))
    simulateOffscreenStarted(true)
    await startPromise

    // Check that kaboom_recording was saved to local storage
    const setCalls = globalThis.chrome.storage.local.set.mock.calls
    const recordingSet = setCalls.find((c) => c.arguments[0]?.kaboom_recording)
    assert.ok(recordingSet, 'Should persist recording state to local storage')
    assert.strictEqual(recordingSet.arguments[0].kaboom_recording.active, true)
    assert.strictEqual(recordingSet.arguments[0].kaboom_recording.name, 'popup-sync-test')

    // Cleanup
    const stopPromise = stopRecording()
    await new Promise((r) => setTimeout(r, 50))
    simulateOffscreenStopped()
    await stopPromise
  })

  test('should start recording badge timer on start', async () => {
    globalThis.chrome = createRecordingChromeMock()

    const startPromise = startRecording('watermark-test', 15, '', { fromPopup: true })
    await new Promise((r) => setTimeout(r, 50))
    simulateOffscreenStarted(true)
    await startPromise

    const bgCalls = globalThis.chrome.action.setBadgeBackgroundColor.mock.calls
    assert.ok(bgCalls.some((c) => c.arguments[0]?.color === '#dc2626'), 'Should set recording badge background color')

    const badgeCalls = globalThis.chrome.action.setBadgeText.mock.calls
    const hasNonEmptyBadge = badgeCalls.some((c) => {
      const text = c.arguments[0]?.text
      return typeof text === 'string' && text.length > 0
    })
    assert.ok(hasNonEmptyBadge, 'Should set non-empty recording badge text')

    // Cleanup
    const stopPromise = stopRecording()
    await new Promise((r) => setTimeout(r, 50))
    simulateOffscreenStopped()
    await stopPromise
  })

  test('should clear recording badge on stop', async () => {
    globalThis.chrome = createRecordingChromeMock()

    const startPromise = startRecording('watermark-hide', 15, '', { fromPopup: true })
    await new Promise((r) => setTimeout(r, 50))
    simulateOffscreenStarted(true)
    await startPromise

    // Reset mock to only capture stop-related calls
    globalThis.chrome.action.setBadgeText.mock.resetCalls()

    const stopPromise = stopRecording()
    await new Promise((r) => setTimeout(r, 50))
    simulateOffscreenStopped()
    await stopPromise

    const badgeCalls = globalThis.chrome.action.setBadgeText.mock.calls
    const clearedBadge = badgeCalls.find((c) => c.arguments[0]?.text === '')
    assert.ok(clearedBadge, 'Should clear recording badge text on stop')
  })

  test('should send offscreen start command with correct parameters', async () => {
    globalThis.chrome = createRecordingChromeMock()

    const startPromise = startRecording('params-test', 24, 'tab', { queryId: 'query-123', fromPopup: true })
    await new Promise((r) => setTimeout(r, 50))
    simulateOffscreenStarted(true)
    await startPromise

    const sendCalls = globalThis.chrome.runtime.sendMessage.mock.calls
    const startCmd = sendCalls.find(
      (c) => c.arguments[0]?.type === 'offscreen_start_recording'
    )
    assert.ok(startCmd, 'Should send OFFSCREEN_START_RECORDING message')
    const msg = startCmd.arguments[0]
    assert.strictEqual(msg.target, 'offscreen')
    assert.strictEqual(msg.name, 'params-test')
    assert.strictEqual(msg.fps, 24)
    assert.strictEqual(msg.audioMode, 'tab')
    assert.strictEqual(msg.tabId, 42)
    assert.ok(typeof msg.streamId === 'string')
    assert.ok(msg.streamId.length > 0)

    // Cleanup
    const stopPromise = stopRecording()
    await new Promise((r) => setTimeout(r, 50))
    simulateOffscreenStopped()
    await stopPromise
  })

  test('should not register tab update listener when using action-badge status', async () => {
    globalThis.chrome = createRecordingChromeMock()

    const startPromise = startRecording('tab-update', 15, '', { fromPopup: true })
    await new Promise((r) => setTimeout(r, 50))
    simulateOffscreenStarted(true)
    await startPromise

    // Watermark-based tab update listener was removed in favor of action badge timer.
    const addListenerCalls = globalThis.chrome.tabs.onUpdated.addListener.mock.calls
    assert.strictEqual(addListenerCalls.length, 0, 'Should not register tab update listener')

    // Cleanup
    const stopPromise = stopRecording()
    await new Promise((r) => setTimeout(r, 50))
    simulateOffscreenStopped()
    await stopPromise
  })

  test('should not remove tab update listener when none was registered', async () => {
    globalThis.chrome = createRecordingChromeMock()

    const startPromise = startRecording('listener-cleanup', 15, '', { fromPopup: true })
    await new Promise((r) => setTimeout(r, 50))
    simulateOffscreenStarted(true)
    await startPromise

    const stopPromise = stopRecording()
    await new Promise((r) => setTimeout(r, 50))
    simulateOffscreenStopped()
    await stopPromise

    // Watermark-based listener is no longer used.
    const removeListenerCalls = globalThis.chrome.tabs.onUpdated.removeListener.mock.calls
    assert.strictEqual(removeListenerCalls.length, 0, 'Should not remove a non-existent tab update listener')
  })
})

// =============================================================================
// stopRecording ERROR PATH — name capture regression
// =============================================================================

describe('stopRecording error path', () => {
  test('returns the recording name when stop throws (name captured before state clear)', async () => {
    globalThis.chrome = createRecordingChromeMock()

    const startPromise = startRecording('stop-error-name', 15, '', { fromPopup: true })
    await new Promise((r) => setTimeout(r, 50))
    simulateOffscreenStarted(true)
    const startResult = await startPromise
    assert.strictEqual(startResult.status, 'recording')

    // Shrink the orphaned 30s stop timeout so it doesn't keep the event loop alive.
    const prevScale = globalThis.KABOOM_TEST_TIMEOUT_SCALE
    globalThis.KABOOM_TEST_TIMEOUT_SCALE = 0.001
    // Make the offscreen stop command throw synchronously to hit the catch block.
    globalThis.chrome.runtime.sendMessage = mock.fn(() => {
      throw new Error('runtime unavailable')
    })

    try {
      const result = await stopRecording()
      // Regression: the catch block used to clear state BEFORE reading the name,
      // so the error result always returned name: ''.
      assert.strictEqual(result.status, 'error')
      assert.strictEqual(result.name, 'stop-error-name', 'error result must include the recording name')
      assert.ok(result.error.includes('RECORD_STOP'))
      assert.strictEqual(isRecording(), false)
    } finally {
      if (prevScale === undefined) {
        delete globalThis.KABOOM_TEST_TIMEOUT_SCALE
      } else {
        globalThis.KABOOM_TEST_TIMEOUT_SCALE = prevScale
      }
      // Let the orphaned (shrunk) stop timeout fire and clean up its listener.
      await new Promise((r) => setTimeout(r, 20))
    }
  })
})

// =============================================================================
// AUTO-TRACK TAB
// =============================================================================

describe('Auto-track Tab', () => {
  afterEach(async () => {
    if (isRecording()) {
      const stopPromise = stopRecording()
      await new Promise((r) => setTimeout(r, 50))
      simulateOffscreenStopped()
      await stopPromise.catch(() => {})
    }
  })

  test('should auto-track tab if not already tracked', async () => {
    globalThis.chrome = createRecordingChromeMock({ storageData: {} })

    const startPromise = startRecording('auto-track', 15, '', { fromPopup: true })
    await new Promise((r) => setTimeout(r, 50))
    simulateOffscreenStarted(true)
    await startPromise

    // Check storage.local.set was called with trackedTabId
    const setCalls = globalThis.chrome.storage.local.set.mock.calls
    const trackCall = setCalls.find((c) => c.arguments[0]?.trackedTabId !== undefined)
    assert.ok(trackCall, 'Should auto-track tab when not already tracked')
    assert.strictEqual(trackCall.arguments[0].trackedTabId, 42)

    // Cleanup
    const stopPromise = stopRecording()
    await new Promise((r) => setTimeout(r, 50))
    simulateOffscreenStopped()
    await stopPromise
  })
})

// =============================================================================
// OFFSCREEN DOCUMENT MANAGEMENT
// =============================================================================

describe('Offscreen Document Management', () => {
  afterEach(async () => {
    if (isRecording()) {
      const stopPromise = stopRecording()
      await new Promise((r) => setTimeout(r, 50))
      simulateOffscreenStopped()
      await stopPromise.catch(() => {})
    }
  })

  test('should check for existing offscreen documents', async () => {
    globalThis.chrome = createRecordingChromeMock({ offscreenContexts: [] })

    const startPromise = startRecording('offscreen-check', 15, '', { fromPopup: true })
    await new Promise((r) => setTimeout(r, 50))
    simulateOffscreenStarted(true)
    await startPromise

    assert.ok(
      globalThis.chrome.runtime.getContexts.mock.calls.length > 0,
      'Should check for existing offscreen documents'
    )

    // Cleanup
    const stopPromise = stopRecording()
    await new Promise((r) => setTimeout(r, 50))
    simulateOffscreenStopped()
    await stopPromise
  })

  test('should create offscreen document when none exists', async () => {
    globalThis.chrome = createRecordingChromeMock({ offscreenContexts: [] })

    const startPromise = startRecording('offscreen-create', 15, '', { fromPopup: true })
    await new Promise((r) => setTimeout(r, 50))
    simulateOffscreenStarted(true)
    await startPromise

    assert.ok(
      globalThis.chrome.offscreen.createDocument.mock.calls.length > 0,
      'Should create offscreen document'
    )

    // Cleanup
    const stopPromise = stopRecording()
    await new Promise((r) => setTimeout(r, 50))
    simulateOffscreenStopped()
    await stopPromise
  })

  test('should skip creating offscreen document when one already exists', async () => {
    globalThis.chrome = createRecordingChromeMock({
      offscreenContexts: [{ contextType: 'OFFSCREEN_DOCUMENT' }]
    })

    const startPromise = startRecording('offscreen-skip', 15, '', { fromPopup: true })
    await new Promise((r) => setTimeout(r, 50))
    simulateOffscreenStarted(true)
    await startPromise

    assert.strictEqual(
      globalThis.chrome.offscreen.createDocument.mock.calls.length,
      0,
      'Should NOT create offscreen document when one exists'
    )

    // Cleanup
    const stopPromise = stopRecording()
    await new Promise((r) => setTimeout(r, 50))
    simulateOffscreenStopped()
    await stopPromise
  })
})

// =============================================================================
// STOP RECORDING WITH TRUNCATED FLAG
// =============================================================================

describe('stopRecording with truncated flag', () => {
  afterEach(async () => {
    if (isRecording()) {
      const stopPromise = stopRecording()
      await new Promise((r) => setTimeout(r, 50))
      simulateOffscreenStopped()
      await stopPromise.catch(() => {})
    }
  })

  test('should pass truncated flag through to result', async () => {
    globalThis.chrome = createRecordingChromeMock()

    // Start recording first
    const startPromise = startRecording('truncated-test', 15, '', { fromPopup: true })
    await new Promise((r) => setTimeout(r, 50))
    simulateOffscreenStarted(true)
    await startPromise

    // Stop with truncated=true
    const stopPromise = stopRecording(true)
    await new Promise((r) => setTimeout(r, 50))
    simulateOffscreenStopped({ truncated: true })
    const result = await stopPromise

    assert.strictEqual(result.truncated, true)
  })
})

// =============================================================================
// STOP RECORDING WITH SAVE TOAST
// =============================================================================

describe('stopRecording save toast', () => {
  afterEach(async () => {
    if (isRecording()) {
      const stopPromise = stopRecording()
      await new Promise((r) => setTimeout(r, 50))
      simulateOffscreenStopped()
      await stopPromise.catch(() => {})
    }
  })

  test('should show save toast when recording is saved', async () => {
    globalThis.chrome = createRecordingChromeMock()

    const startPromise = startRecording('toast-test', 15, '', { fromPopup: true })
    await new Promise((r) => setTimeout(r, 50))
    simulateOffscreenStarted(true)
    await startPromise

    globalThis.chrome.tabs.sendMessage.mock.resetCalls()

    const stopPromise = stopRecording()
    await new Promise((r) => setTimeout(r, 50))
    simulateOffscreenStopped({
      status: 'saved',
      name: 'toast-test',
      size_bytes: 5242880,
      path: '/tmp/toast-test.webm'
    })
    await stopPromise

    const sendCalls = globalThis.chrome.tabs.sendMessage.mock.calls
    const toastCall = sendCalls.find(
      (c) => c.arguments[1]?.type === 'kaboom_action_toast' && c.arguments[1]?.text === 'Recording saved'
    )
    assert.ok(toastCall, 'Should show "Recording saved" toast')
    assert.ok(toastCall.arguments[1].detail.includes('5.0 MB'), 'Toast should include file size')
  })
})

// =============================================================================
// DEFENSIVE GUARDS: chrome API AVAILABILITY
// =============================================================================
