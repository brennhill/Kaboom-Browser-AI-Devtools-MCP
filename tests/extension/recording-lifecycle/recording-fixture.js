// @ts-nocheck
/**
 * @fileoverview Canonical Chrome, offscreen-message, and module fixtures for recording tests.
 *
 * NOTE: recording.js imports from ./index.js and ./event-listeners.js,
 * so a comprehensive chrome mock is required before import. The module also
 * has side effects at load time (clearing stale recording state, installing
 * message listeners behind a chrome runtime guard).
 */

import { mock } from 'node:test'

// =============================================================================
// MOCK FACTORY
// =============================================================================

/** Accumulated onMessage listeners for simulating chrome runtime messages. */
let onMessageListeners = []

/**
 * Build a comprehensive chrome mock that supports recording module dependencies.
 * Tracks all chrome.tabs.sendMessage calls for toast/watermark assertions.
 */
export function createRecordingChromeMock(overrides = {}) {
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
export function simulateOffscreenStarted(success, error, connectionGeneration) {
  const message = {
    target: 'background',
    type: 'offscreen_recording_started',
    success,
    error: error || undefined,
    connection_generation: connectionGeneration
  }
  const sender = { id: globalThis.chrome.runtime.id }
  // Dispatch to all registered listeners
  for (const listener of [...onMessageListeners]) {
    listener(message, sender)
  }
}

// Simulate an OFFSCREEN_RECORDING_STOPPED message
export function simulateOffscreenStopped(overrides = {}) {
  const message = {
    target: 'background',
    type: 'offscreen_recording_stopped',
    status: overrides.status ?? 'saved',
    name: overrides.name ?? 'test-recording',
    duration_seconds: overrides.duration_seconds ?? 10,
    size_bytes: overrides.size_bytes ?? 1024000,
    truncated: overrides.truncated ?? false,
    path: overrides.path ?? '/tmp/test-recording.webm',
    error: overrides.error ?? undefined,
    connection_generation: overrides.connection_generation
  }
  const sender = { id: globalThis.chrome.runtime.id }
  for (const listener of [...onMessageListeners]) {
    listener(message, sender)
  }
}

// Simulate popup gesture grant that unblocks MCP-initiated recording start.
export function simulateRecordingGestureGranted() {
  const message = { type: 'recording_gesture_granted' }
  const sender = { id: globalThis.chrome.runtime.id }
  for (const listener of [...onMessageListeners]) {
    listener(message, sender)
  }
}

// Simulate popup denial for MCP-initiated recording start.
export function simulateRecordingGestureDenied() {
  const message = { type: 'recording_gesture_denied' }
  const sender = { id: globalThis.chrome.runtime.id }
  for (const listener of [...onMessageListeners]) {
    listener(message, sender)
  }
}

export async function waitForPendingRecordingIntent(timeoutMs = 2500) {
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
export const initialChromeMock = createRecordingChromeMock()
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
export const { isRecording, getRecordingInfo, startRecording, stopRecording, initRecording } = await import(
  '../../../extension/background/recording/index.js'
)
