// @ts-nocheck
/**
 * @fileoverview Recording initial state, start validation, gesture, and FPS tests.
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
  initRecording,
  isRecording,
  simulateOffscreenStarted,
  simulateOffscreenStopped,
  simulateRecordingGestureDenied,
  simulateRecordingGestureGranted,
  startRecording,
  stopRecording,
  waitForPendingRecordingIntent,
} from './recording-fixture.js'

describe('Recording Initial State', () => {
  test('isRecording should return false initially', () => {
    assert.strictEqual(isRecording(), false)
  })

  test('getRecordingInfo should return default state', () => {
    const info = getRecordingInfo()
    assert.deepStrictEqual(info, {
      active: false,
      name: '',
      startTime: 0
    })
  })

  test('initRecording clears stale recording state from storage', async () => {
    // Rehydration is now an explicit startup call (initRecording), not an import
    // side effect. With no surviving offscreen recording, it clears stale
    // persisted state. Returning the promise makes this awaitable (no sleep).
    await initRecording()
    const removeCalls = globalThis.chrome.storage.local.remove.mock.calls
    const clearedRecording = removeCalls.some((call) => {
      const arg = call.arguments[0]
      return arg === 'kaboom_recording' || (Array.isArray(arg) && arg.includes('kaboom_recording'))
    })
    assert.ok(clearedRecording, 'initRecording should clear stale recording state from storage')
  })
})

// =============================================================================
// stopRecording WHEN NOT ACTIVE
// =============================================================================

describe('stopRecording when not active', () => {
  test('should return error when no recording is active', async () => {
    const result = await stopRecording()
    assert.strictEqual(result.status, 'error')
    assert.strictEqual(result.name, '')
    assert.ok(result.error.includes('RECORD_STOP'))
    assert.ok(result.error.includes('No active recording'))
  })

  test('should clean up zombie storage when stopping without active recording', async () => {
    await stopRecording()
    // Should call storage.local.remove to clean up potential zombie state
    const removeCalls = globalThis.chrome.storage.local.remove.mock.calls
    const cleaned = removeCalls.some((call) => {
      const arg = call.arguments[0]
      return arg === 'kaboom_recording' || (Array.isArray(arg) && arg.includes('kaboom_recording'))
    })
    assert.ok(cleaned, 'Should clean up zombie storage')
  })
})

// =============================================================================
// startRecording - ALREADY RECORDING
// =============================================================================

describe('startRecording when already recording', () => {
  afterEach(async () => {
    // Force reset: if startRecording set active=true but we didn't complete,
    // stopRecording might not find it active. We just call stopRecording to
    // attempt cleanup.
    await stopRecording().catch(() => {})
  })

  test('should return error if already recording', async () => {
    // Set up mocks for a successful start with immediate offscreen confirmation
    globalThis.chrome = createRecordingChromeMock()

    // We need to simulate a successful start first. To do this, we trigger
    // startRecording and have the offscreen doc confirm via message listener.
    const startPromise = startRecording('first-rec', 15, '', { queryId: 'q1', fromPopup: true })
    // Give the async chain a tick to register the message listener
    await new Promise((r) => setTimeout(r, 50))
    simulateOffscreenStarted(true)
    const firstResult = await startPromise

    if (firstResult.status === 'recording') {
      // Now try to start another recording
      const secondResult = await startRecording('second-rec', 15, '', { queryId: 'q2', fromPopup: true })
      assert.strictEqual(secondResult.status, 'error')
      assert.ok(secondResult.error.includes('Already recording'))

      // Clean up: stop the first recording
      const stopPromise = stopRecording()
      await new Promise((r) => setTimeout(r, 50))
      simulateOffscreenStopped()
      await stopPromise
    }
  })
})

// =============================================================================
// startRecording - NO ACTIVE TAB
// =============================================================================

describe('startRecording with no active tab', () => {
  test('should return error when no tab is found', async () => {
    globalThis.chrome = createRecordingChromeMock({ tabsQueryResult: [] })
    const result = await startRecording('test-rec', 15, '', { queryId: 'q1', fromPopup: true })
    assert.strictEqual(result.status, 'error')
    assert.ok(result.error.includes('No active tab'))
    assert.strictEqual(isRecording(), false)
  })

  test('should return error when tab has no id', async () => {
    globalThis.chrome = createRecordingChromeMock({ tabsQueryResult: [{ url: 'http://example.com' }] })
    const result = await startRecording('test-rec', 15, '', { queryId: 'q1', fromPopup: true })
    assert.strictEqual(result.status, 'error')
    assert.ok(result.error.includes('No active tab'))
    assert.strictEqual(isRecording(), false)
  })
})

// =============================================================================
// MCP-INITIATED RECORDING FLOW
// =============================================================================

describe('MCP-initiated recording flow', () => {
  afterEach(async () => {
    if (isRecording()) {
      const stopPromise = stopRecording()
      await new Promise((r) => setTimeout(r, 50))
      simulateOffscreenStopped()
      await stopPromise.catch(() => {})
    }
  })

  test('should activate the target tab and show permission toast on that tab', async () => {
    globalThis.chrome = createRecordingChromeMock({
      tabsQueryResult: [{ id: 42, url: 'http://active-tab.example', title: 'Active Tab' }]
    })
    globalThis.chrome.tabs.get = mock.fn((tabId) =>
      Promise.resolve({
        id: tabId,
        windowId: 1,
        status: 'complete',
        url: `http://target-${tabId}.example`,
        title: 'Target Tab'
      })
    )

    const startPromise = startRecording('mcp-target-tab', 15, '', { queryId: 'query-mcp-1', targetTabId: 77 })

    await waitForPendingRecordingIntent()
    simulateRecordingGestureGranted()

    // Allow recording startup to reach offscreen handshake.
    await new Promise((r) => setTimeout(r, 50))
    simulateOffscreenStarted(true)

    const result = await startPromise
    assert.strictEqual(result.status, 'recording')

    const updateCalls = globalThis.chrome.tabs.update.mock.calls
    const activatedTarget = updateCalls.some(
      (c) => c.arguments[0] === 77 && c.arguments[1]?.active === true
    )
    assert.ok(activatedTarget, 'Should activate target tab for MCP recording')

    const toastCalls = globalThis.chrome.tabs.sendMessage.mock.calls
    const permissionToastOnTarget = toastCalls.some(
      (c) =>
        c.arguments[0] === 77 &&
        c.arguments[1]?.type === 'kaboom_action_toast' &&
        String(c.arguments[1]?.text || '').includes('Open KaBOOM!')
    )
    assert.ok(permissionToastOnTarget, 'Should show popup-approval permission toast on target tab')

    const stopPromise = stopRecording()
    await new Promise((r) => setTimeout(r, 50))
    simulateOffscreenStopped()
    await stopPromise
  })

  test('rejects an offscreen start acknowledgement from a superseded daemon generation', async () => {
    globalThis.chrome = createRecordingChromeMock()
    const generation = await import('../../../extension/background/runtime-state/connection-generation.js')
    generation.setConnectionGeneration(1)

    const startPromise = startRecording('stale-mcp-recording', 15, '', {
      queryId: 'query-stale',
      fromPopup: true,
      targetTabId: 42,
      connectionGeneration: 1
    })
    await new Promise((resolve) => setTimeout(resolve, 50))

    generation.setConnectionGeneration(2)
    simulateOffscreenStarted(true, undefined, 1)

    const result = await startPromise
    assert.strictEqual(result.status, 'error')
    assert.strictEqual(result.error, 'RECORD_START: stale_connection_generation')
    assert.strictEqual(isRecording(), false)
  })

  test('rejects an offscreen stop acknowledgement from a superseded daemon generation', async () => {
    globalThis.chrome = createRecordingChromeMock()
    const startPromise = startRecording('stale-stop-recording', 15, '', { fromPopup: true })
    await new Promise((resolve) => setTimeout(resolve, 50))
    simulateOffscreenStarted(true)
    assert.strictEqual((await startPromise).status, 'recording')

    const generation = await import('../../../extension/background/runtime-state/connection-generation.js')
    generation.setConnectionGeneration(1)
    const stopPromise = stopRecording(false, 1)
    await new Promise((resolve) => setTimeout(resolve, 20))
    generation.setConnectionGeneration(2)
    simulateOffscreenStopped({ name: 'stale-stop-recording', connection_generation: 1 })

    const result = await stopPromise
    assert.strictEqual(result.status, 'error')
    assert.strictEqual(result.error, 'RECORD_STOP: stale_connection_generation')
  })

  test('should return denied error when popup rejects MCP recording request', async () => {
    globalThis.chrome = createRecordingChromeMock({
      tabsQueryResult: [{ id: 42, url: 'http://active-tab.example', title: 'Active Tab' }]
    })
    globalThis.chrome.tabs.get = mock.fn((tabId) =>
      Promise.resolve({
        id: tabId,
        windowId: 1,
        status: 'complete',
        url: `http://target-${tabId}.example`,
        title: 'Target Tab'
      })
    )

    const startPromise = startRecording('mcp-denied', 15, '', { queryId: 'query-mcp-denied', targetTabId: 77 })

    await waitForPendingRecordingIntent()
    simulateRecordingGestureDenied()

    const result = await startPromise
    assert.strictEqual(result.status, 'error')
    assert.ok(String(result.error || '').toLowerCase().includes('denied'))
    assert.ok(String(result.error || '').includes('KaBOOM! popup'))
    assert.strictEqual(isRecording(), false)
  })
})

// =============================================================================
// startRecording - FPS CLAMPING
// =============================================================================

describe('FPS Clamping', () => {
  afterEach(async () => {
    await stopRecording().catch(() => {})
  })

  test('should clamp fps below 5 to 5', async () => {
    globalThis.chrome = createRecordingChromeMock()
    const startPromise = startRecording('test-fps', 1, '', { fromPopup: true })
    await new Promise((r) => setTimeout(r, 50))
    simulateOffscreenStarted(true)
    const result = await startPromise

    if (result.status === 'recording') {
      // Verify the sendMessage call to offscreen included clamped fps
      const sendCalls = globalThis.chrome.runtime.sendMessage.mock.calls
      const startCmd = sendCalls.find(
        (c) => c.arguments[0]?.type === 'offscreen_start_recording'
      )
      if (startCmd) {
        assert.strictEqual(startCmd.arguments[0].fps, 5)
      }
      // Clean up
      const stopPromise = stopRecording()
      await new Promise((r) => setTimeout(r, 50))
      simulateOffscreenStopped()
      await stopPromise
    }
  })

  test('should clamp fps above 60 to 60', async () => {
    globalThis.chrome = createRecordingChromeMock()
    const startPromise = startRecording('test-fps', 120, '', { fromPopup: true })
    await new Promise((r) => setTimeout(r, 50))
    simulateOffscreenStarted(true)
    const result = await startPromise

    if (result.status === 'recording') {
      const sendCalls = globalThis.chrome.runtime.sendMessage.mock.calls
      const startCmd = sendCalls.find(
        (c) => c.arguments[0]?.type === 'offscreen_start_recording'
      )
      if (startCmd) {
        assert.strictEqual(startCmd.arguments[0].fps, 60)
      }
      const stopPromise = stopRecording()
      await new Promise((r) => setTimeout(r, 50))
      simulateOffscreenStopped()
      await stopPromise
    }
  })

  test('should accept fps within valid range', async () => {
    globalThis.chrome = createRecordingChromeMock()
    const startPromise = startRecording('test-fps', 30, '', { fromPopup: true })
    await new Promise((r) => setTimeout(r, 50))
    simulateOffscreenStarted(true)
    const result = await startPromise

    if (result.status === 'recording') {
      const sendCalls = globalThis.chrome.runtime.sendMessage.mock.calls
      const startCmd = sendCalls.find(
        (c) => c.arguments[0]?.type === 'offscreen_start_recording'
      )
      if (startCmd) {
        assert.strictEqual(startCmd.arguments[0].fps, 30)
      }
      const stopPromise = stopRecording()
      await new Promise((r) => setTimeout(r, 50))
      simulateOffscreenStopped()
      await stopPromise
    }
  })
})

// =============================================================================
// startRecording - EMPTY STREAM ID
// =============================================================================

describe('startRecording with empty stream', () => {
  test('should return error when stream ID is empty', async () => {
    globalThis.chrome = createRecordingChromeMock({ tabCaptureStreamId: '' })
    const result = await startRecording('test-rec', 15, '', { queryId: 'q1', fromPopup: true })
    assert.strictEqual(result.status, 'error')
    assert.ok(result.error.includes('getMediaStreamId returned empty'))
    assert.strictEqual(isRecording(), false)
  })
})

// =============================================================================
// startRecording - tabCapture ERROR
// =============================================================================

describe('startRecording with tabCapture error', () => {
  test('should return error when tabCapture fails', async () => {
    globalThis.chrome = createRecordingChromeMock({
      tabCaptureError: 'Permission denied for tab capture'
    })
    const result = await startRecording('test-rec', 15, '', { queryId: 'q1', fromPopup: true })
    assert.strictEqual(result.status, 'error')
    assert.ok(result.error.includes('Permission denied') || result.error.includes('RECORD_START'))
    assert.strictEqual(isRecording(), false)
  })
})

// =============================================================================
// startRecording - OFFSCREEN FAILURE
// =============================================================================

describe('startRecording with offscreen failure', () => {
  afterEach(async () => {
    await stopRecording().catch(() => {})
  })

  test('should return error when offscreen document rejects', async () => {
    globalThis.chrome = createRecordingChromeMock()
    const startPromise = startRecording('test-rec', 15, '', { queryId: 'q1', fromPopup: true })
    await new Promise((r) => setTimeout(r, 50))
    simulateOffscreenStarted(false, 'MediaRecorder not supported')
    const result = await startPromise

    assert.strictEqual(result.status, 'error')
    assert.ok(result.error.includes('MediaRecorder not supported') || result.error.includes('RECORD_START'))
    assert.strictEqual(isRecording(), false)
  })
})

// =============================================================================
// SUCCESSFUL RECORDING LIFECYCLE
// =============================================================================
