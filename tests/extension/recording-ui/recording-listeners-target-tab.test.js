// @ts-nocheck
import { beforeEach, describe, test, mock } from 'node:test'
import assert from 'node:assert'

let runtimeListeners = []

function createChromeMock({ trackedTabId = 99, pendingMicReturnTabId } = {}) {
  runtimeListeners = []
  return {
    runtime: {
      id: 'test-extension-id',
      onMessage: {
        addListener: mock.fn((listener) => runtimeListeners.push(listener))
      }
    },
    action: {
      setBadgeBackgroundColor: mock.fn(),
      setBadgeText: mock.fn()
    },
    storage: {
      local: {
        get: mock.fn((key, cb) => {
          const keys = Array.isArray(key) ? key : [key]
          let result = {}
          if (keys.includes('trackedTabId')) result = { trackedTabId }
          else if (keys.includes('kaboom_pending_mic_recording')) {
            result = { kaboom_pending_mic_recording: { returnTabId: pendingMicReturnTabId } }
          }
          if (typeof cb === 'function') {
            cb(result)
            return
          }
          return Promise.resolve(result)
        }),
        set: mock.fn(() => Promise.resolve()),
        remove: mock.fn(() => Promise.resolve())
      }
    },
    tabs: {
      get: mock.fn((tabId) =>
        Promise.resolve({
          id: tabId,
          windowId: 1,
          url: `https://tracked.example.com/path/${tabId}`,
          title: `Tracked ${tabId}`
        })
      ),
      query: mock.fn(() => Promise.resolve([{ id: 42, url: 'https://active.example.com', title: 'Active' }])),
      sendMessage: mock.fn(() => Promise.resolve({ draw_mode_active: false })),
      update: mock.fn(() => Promise.resolve()),
      remove: mock.fn(() => Promise.resolve())
    }
  }
}

function dispatchPopupMessage(message, senderOverrides = {}) {
  return new Promise((resolve) => {
    const sender = { id: globalThis.chrome.runtime.id, ...senderOverrides }
    const sendResponse = (response) => resolve(response)
    for (const listener of runtimeListeners) {
      const handledAsync = listener(message, sender, sendResponse)
      if (handledAsync === true) return
    }
    resolve(undefined)
  })
}

describe('recording listeners popup target selection', () => {
  beforeEach(() => {
    globalThis.chrome = createChromeMock()
    delete globalThis.KABOOM_TEST_TIMEOUT_SCALE
  })

  test('popup screen_recording_start targets tracked tab when available', async () => {
    const { installRecordingListeners } = await import('../../../extension/background/recording/listeners.js')

    const deps = {
      startRecording: mock.fn(async () => ({ status: 'recording', name: 'tracked', startTime: Date.now() })),
      stopRecording: mock.fn(async () => ({ status: 'saved', name: 'tracked' })),
      isActive: () => false,
      getTabId: () => 0,
      setInactive: () => {},
      clearRecordingState: async () => {},
      getServerUrl: () => 'http://localhost:7890'
    }

    installRecordingListeners(deps)
    const response = await dispatchPopupMessage({ type: 'screen_recording_start', audio: '' })

    assert.strictEqual(response?.status, 'recording')
    assert.strictEqual(deps.startRecording.mock.calls.length, 1)

    const startCall = deps.startRecording.mock.calls[0].arguments
    assert.strictEqual(startCall[5], 99, 'Expected tracked tab ID as targetTabId')
    assert.ok(String(startCall[0]).includes('tracked-example-com'), 'Expected slug derived from tracked tab URL')
  })

  test('mic permission guidance toast uses Kaboom copy', async () => {
    globalThis.KABOOM_TEST_TIMEOUT_SCALE = 0.001
    globalThis.chrome = createChromeMock({ pendingMicReturnTabId: 88 })

    const { installRecordingListeners } = await import('../../../extension/background/recording/listeners.js')

    const deps = {
      startRecording: mock.fn(async () => ({ status: 'recording', name: 'tracked', startTime: Date.now() })),
      stopRecording: mock.fn(async () => ({ status: 'saved', name: 'tracked' })),
      isActive: () => false,
      getTabId: () => 0,
      setInactive: () => {},
      clearRecordingState: async () => {},
      getServerUrl: () => 'http://localhost:7890'
    }

    installRecordingListeners(deps)
    await dispatchPopupMessage({ type: 'mic_granted_close_tab' }, { tab: { id: 501 } })
    await new Promise((resolve) => setTimeout(resolve, 10))

    assert.deepStrictEqual(globalThis.chrome.tabs.remove.mock.calls[0].arguments, [501])
    assert.deepStrictEqual(globalThis.chrome.tabs.update.mock.calls[0].arguments, [88, { active: true }])
    assert.match(globalThis.chrome.tabs.sendMessage.mock.calls[0].arguments[1].detail, /Open KaBOOM! and click Record/)
  })
})
