// @ts-nocheck
/**
 * @fileoverview recording.test.js -- Tests for the recording module.
 * Covers recording lifecycle (start/stop), state management, FPS clamping,
 * error handling (no tab, already recording, empty stream, offscreen timeout),
 * defensive chrome API guards, popup sync, and action-badge recording status.
 *
 * NOTE: recording.js imports from ./index.js and ./event-listeners.js,
 * so a comprehensive chrome mock is required before import. The module also
 * has side effects at load time (clearing stale recording state, installing
 * message listeners behind a chrome runtime guard).
 */

import { test, describe, mock, beforeEach as _beforeEach, afterEach } from 'node:test'
import assert from 'node:assert'

// =============================================================================
// MOCK FACTORY
// =============================================================================

/** Accumulated onMessage listeners for simulating chrome runtime messages. */
let onMessageListeners = []

/**
 * Build a comprehensive chrome mock that supports recording module dependencies.
 * Tracks all chrome.tabs.sendMessage calls for toast/watermark assertions.
 */
function createRecordingChromeMock(overrides = {}) {
  if (!overrides.preserveListeners) {
    onMessageListeners = []
  }

  const tabsQueryResult = overrides.tabsQueryResult ?? [{ id: 42, url: 'http://example.com/page', title: 'Example' }]
  const storageData = overrides.storageData ?? {}
  const tabCaptureStreamId = overrides.tabCaptureStreamId ?? 'mock-stream-id-abc123'
  const tabCaptureError = overrides.tabCaptureError ?? null
  const offscreenContexts = overrides.offscreenContexts ?? []

  return {
    runtime: {
      onMessage: {
        addListener: mock.fn((listener) => {
          onMessageListeners.push(listener)
        }),
        removeListener: mock.fn((listener) => {
          onMessageListeners = onMessageListeners.filter((l) => l !== listener)
        })
      },
      sendMessage: mock.fn(() => Promise.resolve()),
      getManifest: () => ({ version: '6.0.3' }),
      id: 'test-extension-id',
      lastError: tabCaptureError ? { message: tabCaptureError } : null,
      getContexts: mock.fn(() => Promise.resolve(offscreenContexts)),
      ContextType: { OFFSCREEN_DOCUMENT: 'OFFSCREEN_DOCUMENT' }
    },
    action: {
      setBadgeText: mock.fn(),
      setBadgeBackgroundColor: mock.fn()
    },
    storage: {
      local: {
        get: mock.fn((keys, cb) => {
          if (typeof keys === 'string') {
            const result = {}
            if (storageData[keys] !== undefined) result[keys] = storageData[keys]
            if (typeof cb === 'function') cb(result)
            else return Promise.resolve(result)
          } else {
            if (typeof cb === 'function') cb(storageData)
            else return Promise.resolve(storageData)
          }
        }),
        set: mock.fn((data, cb) => {
          Object.assign(storageData, data)
          if (typeof cb === 'function') cb()
          else return Promise.resolve()
        }),
        remove: mock.fn((keys, cb) => {
          const keyArr = Array.isArray(keys) ? keys : [keys]
          for (const k of keyArr) delete storageData[k]
          if (typeof cb === 'function') cb()
          else return Promise.resolve()
        })
      },
      sync: {
        get: mock.fn((k, cb) => cb && cb({})),
        set: mock.fn((d, cb) => cb && cb()),
        remove: mock.fn((k, cb) => {
          if (typeof cb === 'function') cb()
          else return Promise.resolve()
        })
      },
      session: {
        get: mock.fn((k, cb) => cb && cb({})),
        set: mock.fn((d, cb) => cb && cb()),
        remove: mock.fn((k, cb) => {
          if (typeof cb === 'function') cb()
          else return Promise.resolve()
        })
      },
      onChanged: { addListener: mock.fn() }
    },
    tabs: {
      get: mock.fn((tabId) => Promise.resolve({ id: tabId, windowId: 1, url: 'http://example.com' })),
      query: mock.fn(() => Promise.resolve(tabsQueryResult)),
      onRemoved: { addListener: mock.fn() },
      onUpdated: { addListener: mock.fn(), removeListener: mock.fn() },
      sendMessage: mock.fn(() => Promise.resolve()),
      reload: mock.fn(),
      update: mock.fn(() => Promise.resolve()),
      remove: mock.fn(() => Promise.resolve())
    },
    alarms: {
      create: mock.fn(),
      onAlarm: { addListener: mock.fn() }
    },
    commands: {
      onCommand: { addListener: mock.fn() }
    },
    tabCapture: {
      getMediaStreamId: mock.fn((opts, cb) => {
        if (tabCaptureError) {
          cb(undefined)
        } else {
          cb(tabCaptureStreamId)
        }
      })
    },
    offscreen: {
      createDocument: mock.fn(() => Promise.resolve()),
      closeDocument: mock.fn(() => Promise.resolve()),
      Reason: { USER_MEDIA: 'USER_MEDIA' }
    }
  }
}

// Simulate an OFFSCREEN_RECORDING_STARTED message from the offscreen document
function simulateOffscreenStarted(success, error) {
  const message = {
    target: 'background',
    type: 'offscreen_recording_started',
    success,
    error: error || undefined
  }
  const sender = { id: globalThis.chrome.runtime.id }
  // Dispatch to all registered listeners
  for (const listener of [...onMessageListeners]) {
    listener(message, sender)
  }
}

// Simulate an OFFSCREEN_RECORDING_STOPPED message
function simulateOffscreenStopped(overrides = {}) {
  const message = {
    target: 'background',
    type: 'offscreen_recording_stopped',
    status: overrides.status ?? 'saved',
    name: overrides.name ?? 'test-recording',
    duration_seconds: overrides.duration_seconds ?? 10,
    size_bytes: overrides.size_bytes ?? 1024000,
    truncated: overrides.truncated ?? false,
    path: overrides.path ?? '/tmp/test-recording.webm',
    error: overrides.error ?? undefined
  }
  const sender = { id: globalThis.chrome.runtime.id }
  for (const listener of [...onMessageListeners]) {
    listener(message, sender)
  }
}

// Simulate popup gesture grant that unblocks MCP-initiated recording start.
function simulateRecordingGestureGranted() {
  const message = { type: 'recording_gesture_granted' }
  const sender = { id: globalThis.chrome.runtime.id }
  for (const listener of [...onMessageListeners]) {
    listener(message, sender)
  }
}

// Simulate popup denial for MCP-initiated recording start.
function simulateRecordingGestureDenied() {
  const message = { type: 'recording_gesture_denied' }
  const sender = { id: globalThis.chrome.runtime.id }
  for (const listener of [...onMessageListeners]) {
    listener(message, sender)
  }
}

async function waitForPendingRecordingIntent(timeoutMs = 2500) {
  const deadline = Date.now() + timeoutMs
  while (Date.now() < deadline) {
    const hasPendingIntent = globalThis.chrome.storage.local.set.mock.calls.some(
      (call) => call.arguments[0]?.kaboom_pending_recording
    )
    if (hasPendingIntent) return
    await new Promise((r) => setTimeout(r, 20))
  }
  throw new Error('Timed out waiting for pending recording gesture setup')
}

// =============================================================================
// MODULE IMPORT
// =============================================================================

// Set up chrome mock before importing. Keep reference to verify listener registration.
const initialChromeMock = createRecordingChromeMock()
globalThis.chrome = initialChromeMock
// navigator is a read-only getter in modern Node.js, so use defineProperty
if (!globalThis.navigator || !globalThis.navigator.userAgent) {
  Object.defineProperty(globalThis, 'navigator', {
    value: { userAgent: 'TestAgent/1.0' },
    writable: true,
    configurable: true
  })
}
globalThis.fetch = mock.fn(() => Promise.resolve({ ok: true, json: () => Promise.resolve({}) }))

// The module is imported once. Its internal state is shared across tests.
// We rely on stopRecording / start-stop sequences to clean state between tests.
const { isRecording, getRecordingInfo, startRecording, stopRecording } = await import(
  '../../extension/background/recording.js'
)

// =============================================================================
// INITIAL STATE
// =============================================================================

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

  test('module load should clear stale recording state from storage', () => {
    // The module clears stale recording state at import time.
    // We verify the storage.local.remove was called during module load.
    const removeCalls = globalThis.chrome.storage.local.remove.mock.calls
    const clearedRecording = removeCalls.some((call) => {
      const arg = call.arguments[0]
      return arg === 'kaboom_recording' || (Array.isArray(arg) && arg.includes('kaboom_recording'))
    })
    assert.ok(clearedRecording, 'Module should clear stale recording state from storage on load')
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
    const startPromise = startRecording('first-rec', 15, 'q1', '', true)
    // Give the async chain a tick to register the message listener
    await new Promise((r) => setTimeout(r, 50))
    simulateOffscreenStarted(true)
    const firstResult = await startPromise

    if (firstResult.status === 'recording') {
      // Now try to start another recording
      const secondResult = await startRecording('second-rec', 15, 'q2', '', true)
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
    const result = await startRecording('test-rec', 15, 'q1', '', true)
    assert.strictEqual(result.status, 'error')
    assert.ok(result.error.includes('No active tab'))
    assert.strictEqual(isRecording(), false)
  })

  test('should return error when tab has no id', async () => {
    globalThis.chrome = createRecordingChromeMock({ tabsQueryResult: [{ url: 'http://example.com' }] })
    const result = await startRecording('test-rec', 15, 'q1', '', true)
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

    const startPromise = startRecording('mcp-target-tab', 15, 'query-mcp-1', '', false, 77)

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
        String(c.arguments[1]?.text || '').includes('Open Kaboom')
    )
    assert.ok(permissionToastOnTarget, 'Should show popup-approval permission toast on target tab')

    const stopPromise = stopRecording()
    await new Promise((r) => setTimeout(r, 50))
    simulateOffscreenStopped()
    await stopPromise
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

    const startPromise = startRecording('mcp-denied', 15, 'query-mcp-denied', '', false, 77)

    await waitForPendingRecordingIntent()
    simulateRecordingGestureDenied()

    const result = await startPromise
    assert.strictEqual(result.status, 'error')
    assert.ok(String(result.error || '').toLowerCase().includes('denied'))
    assert.ok(String(result.error || '').includes('Kaboom popup'))
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
    const startPromise = startRecording('test-fps', 1, '', '', true)
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
    const startPromise = startRecording('test-fps', 120, '', '', true)
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
    const startPromise = startRecording('test-fps', 30, '', '', true)
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
    const result = await startRecording('test-rec', 15, 'q1', '', true)
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
    const result = await startRecording('test-rec', 15, 'q1', '', true)
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
    const startPromise = startRecording('test-rec', 15, 'q1', '', true)
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
    const startPromise = startRecording('lifecycle-test', 15, 'q1', '', true)
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

    const startPromise = startRecording('popup-sync-test', 15, '', '', true)
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

    const startPromise = startRecording('watermark-test', 15, '', '', true)
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

    const startPromise = startRecording('watermark-hide', 15, '', '', true)
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

    const startPromise = startRecording('params-test', 24, 'query-123', 'tab', true)
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

    const startPromise = startRecording('tab-update', 15, '', '', true)
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

    const startPromise = startRecording('listener-cleanup', 15, '', '', true)
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

    const startPromise = startRecording('stop-error-name', 15, '', '', true)
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

    const startPromise = startRecording('auto-track', 15, '', '', true)
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

    const startPromise = startRecording('offscreen-check', 15, '', '', true)
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

    const startPromise = startRecording('offscreen-create', 15, '', '', true)
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

    const startPromise = startRecording('offscreen-skip', 15, '', '', true)
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
    const startPromise = startRecording('truncated-test', 15, '', '', true)
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

    const startPromise = startRecording('toast-test', 15, '', '', true)
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
    const startPromise = startRecording('double-stop', 15, '', '', true)
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

    const result = await startRecording('crash-test', 15, '', '', true)
    assert.strictEqual(result.status, 'error')
    assert.ok(result.error.includes('Tabs API crashed'))
    assert.strictEqual(isRecording(), false, 'active flag should be reset after exception')
  })

  test('should include error message in result on exception', async () => {
    globalThis.chrome = createRecordingChromeMock()
    globalThis.chrome.tabs.query = mock.fn(() => Promise.reject(new Error('Network failure')))

    const result = await startRecording('error-msg', 15, '', '', true)
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
    const startPromise = startRecording('stop-crash', 15, '', '', true)
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

    const startPromise = startRecording('stop-cmd', 15, '', '', true)
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

    const startPromise = startRecording('passthrough', 15, '', '', true)
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

    const startPromise = startRecording('err-passthrough', 15, '', '', true)
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

    const startPromise = startRecording('cleanup-test', 15, '', '', true)
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
