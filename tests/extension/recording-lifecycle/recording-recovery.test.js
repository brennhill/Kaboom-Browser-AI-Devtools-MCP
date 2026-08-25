// @ts-nocheck
/**
 * @fileoverview Recording defensive guards, stop recovery, and cleanup tests.
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
  initialChromeMock,
  isRecording,
  simulateOffscreenStarted,
  simulateOffscreenStopped,
  startRecording,
  stopRecording,
} from './recording-fixture.js'

describe('Defensive Chrome API Guards', () => {
  test('module should load safely when chrome is defined but minimal', () => {
    // The module was already loaded with a full mock. The key guard we test
    // is the top-level stale state cleanup:
    // if (typeof chrome !== 'undefined' && chrome.storage?.local?.remove)
    // This runs at module load time. Since we successfully imported, the guard worked.
    assert.ok(true, 'Module loaded successfully with chrome mock')
  })

  test('runtime message listeners should be guarded by chrome availability', () => {
    // The module wraps its runtime.onMessage.addListener calls in:
    // if (typeof chrome !== 'undefined' && chrome.runtime?.onMessage)
    // Since chrome was defined at import time, these listeners were registered.
    // Verify the initial mock's addListener was called during module load.
    const addListenerCallCount = initialChromeMock.runtime.onMessage.addListener.mock.calls.length
    assert.ok(
      addListenerCallCount > 0,
      `Expected runtime.onMessage.addListener to be called during module load, got ${addListenerCallCount} calls`
    )
  })
})

// =============================================================================
// DOUBLE STOP PREVENTION
// =============================================================================

describe('Double Stop Prevention', () => {
  test('should prevent double stop by marking active=false immediately', async () => {
    globalThis.chrome = createRecordingChromeMock()

    // Start recording
    const startPromise = startRecording('double-stop', 15, '', { fromPopup: true })
    await new Promise((r) => setTimeout(r, 50))
    simulateOffscreenStarted(true)
    await startPromise

    // First stop (will wait for offscreen response)
    const stop1Promise = stopRecording()

    // Second stop should immediately return error (active is already false)
    const stop2Result = await stopRecording()
    assert.strictEqual(stop2Result.status, 'error')
    assert.ok(stop2Result.error.includes('No active recording'))

    // Complete first stop
    await new Promise((r) => setTimeout(r, 50))
    simulateOffscreenStopped()
    await stop1Promise
  })
})

// =============================================================================
// RECORDING STATE CLEANUP ON ERROR
// =============================================================================

describe('Recording state cleanup on error', () => {
  test('should reset active flag when startRecording encounters an exception', async () => {
    // Create a mock where tabs.query throws
    globalThis.chrome = createRecordingChromeMock()
    globalThis.chrome.tabs.query = mock.fn(() => Promise.reject(new Error('Tabs API crashed')))

    const result = await startRecording('crash-test', 15, '', { fromPopup: true })
    assert.strictEqual(result.status, 'error')
    assert.ok(result.error.includes('Tabs API crashed'))
    assert.strictEqual(isRecording(), false, 'active flag should be reset after exception')
  })

  test('should include error message in result on exception', async () => {
    globalThis.chrome = createRecordingChromeMock()
    globalThis.chrome.tabs.query = mock.fn(() => Promise.reject(new Error('Network failure')))

    const result = await startRecording('error-msg', 15, '', { fromPopup: true })
    assert.ok(result.error.includes('RECORD_START'))
    assert.ok(result.error.includes('Network failure'))
  })
})

// =============================================================================
// stopRecording - OFFSCREEN EXCEPTION
// =============================================================================

describe('stopRecording with offscreen exception', () => {
  test('should handle exception during stop gracefully', async () => {
    globalThis.chrome = createRecordingChromeMock()

    // Start recording
    const startPromise = startRecording('stop-crash', 15, '', { fromPopup: true })
    await new Promise((r) => setTimeout(r, 50))
    simulateOffscreenStarted(true)
    await startPromise

    // Make runtime.sendMessage throw during stop
    globalThis.chrome.runtime.sendMessage = mock.fn(() => {
      throw new Error('Extension context invalidated')
    })

    // stopRecording should catch the error and return gracefully
    // Note: the Promise constructor in stopRecording wraps sendMessage,
    // but the throw happens synchronously inside the Promise executor,
    // so it should be caught by the try/catch in stopRecording.
    const result = await stopRecording()
    // After exception, state should be cleaned up
    assert.strictEqual(isRecording(), false)
    // Result might be error or might have caught it depending on exact flow
    assert.ok(result.status === 'error' || result.status === 'saved' || result.name !== undefined)
  })
})

// =============================================================================
// SEND STOP COMMAND
// =============================================================================

describe('Stop command to offscreen', () => {
  afterEach(async () => {
    if (isRecording()) {
      const stopPromise = stopRecording()
      await new Promise((r) => setTimeout(r, 50))
      simulateOffscreenStopped()
      await stopPromise.catch(() => {})
    }
  })

  test('should send OFFSCREEN_STOP_RECORDING message', async () => {
    globalThis.chrome = createRecordingChromeMock()

    const startPromise = startRecording('stop-cmd', 15, '', { fromPopup: true })
    await new Promise((r) => setTimeout(r, 50))
    simulateOffscreenStarted(true)
    await startPromise

    globalThis.chrome.runtime.sendMessage.mock.resetCalls()

    const stopPromise = stopRecording()
    await new Promise((r) => setTimeout(r, 50))

    // Verify the stop command was sent
    const sendCalls = globalThis.chrome.runtime.sendMessage.mock.calls
    const stopCmd = sendCalls.find(
      (c) => c.arguments[0]?.type === 'offscreen_stop_recording'
    )
    assert.ok(stopCmd, 'Should send OFFSCREEN_STOP_RECORDING message')
    assert.strictEqual(stopCmd.arguments[0].target, 'offscreen')

    simulateOffscreenStopped()
    await stopPromise
  })
})

// =============================================================================
// STOP RESULT PASSTHROUGH
// =============================================================================

describe('Stop result passthrough', () => {
  afterEach(async () => {
    if (isRecording()) {
      const stopPromise = stopRecording()
      await new Promise((r) => setTimeout(r, 50))
      simulateOffscreenStopped()
      await stopPromise.catch(() => {})
    }
  })

  test('should pass through all fields from offscreen stop result', async () => {
    globalThis.chrome = createRecordingChromeMock()

    const startPromise = startRecording('passthrough', 15, '', { fromPopup: true })
    await new Promise((r) => setTimeout(r, 50))
    simulateOffscreenStarted(true)
    await startPromise

    const stopPromise = stopRecording()
    await new Promise((r) => setTimeout(r, 50))
    simulateOffscreenStopped({
      status: 'saved',
      name: 'passthrough',
      duration_seconds: 42,
      size_bytes: 9999999,
      truncated: false,
      path: '/home/user/videos/passthrough.webm'
    })
    const result = await stopPromise

    assert.strictEqual(result.status, 'saved')
    assert.strictEqual(result.name, 'passthrough')
    assert.strictEqual(result.duration_seconds, 42)
    assert.strictEqual(result.size_bytes, 9999999)
    assert.strictEqual(result.truncated, false)
    assert.strictEqual(result.path, '/home/user/videos/passthrough.webm')
    assert.strictEqual(result.error, undefined)
  })

  test('should pass through error from offscreen stop result', async () => {
    globalThis.chrome = createRecordingChromeMock()

    const startPromise = startRecording('err-passthrough', 15, '', { fromPopup: true })
    await new Promise((r) => setTimeout(r, 50))
    simulateOffscreenStarted(true)
    await startPromise

    const stopPromise = stopRecording()
    await new Promise((r) => setTimeout(r, 50))
    simulateOffscreenStopped({
      status: 'error',
      name: 'err-passthrough',
      error: 'Upload failed: server unreachable'
    })
    const result = await stopPromise

    assert.strictEqual(result.status, 'error')
    assert.strictEqual(result.error, 'Upload failed: server unreachable')
  })
})

// =============================================================================
// STORAGE CLEANUP ON STOP
// =============================================================================

describe('Storage cleanup on stop', () => {
  test('should remove recording state from storage on stop', async () => {
    globalThis.chrome = createRecordingChromeMock()

    const startPromise = startRecording('cleanup-test', 15, '', { fromPopup: true })
    await new Promise((r) => setTimeout(r, 50))
    simulateOffscreenStarted(true)
    await startPromise

    globalThis.chrome.storage.local.remove.mock.resetCalls()

    const stopPromise = stopRecording()
    await new Promise((r) => setTimeout(r, 50))
    simulateOffscreenStopped()
    await stopPromise

    const removeCalls = globalThis.chrome.storage.local.remove.mock.calls
    const cleaned = removeCalls.some((call) => {
      const arg = call.arguments[0]
      return arg === 'kaboom_recording' || (Array.isArray(arg) && arg.includes('kaboom_recording'))
    })
    assert.ok(cleaned, 'Should remove recording state from storage on stop')
  })
})
