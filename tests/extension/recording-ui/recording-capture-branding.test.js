// @ts-nocheck
import { beforeEach, describe, mock, test } from 'node:test'
import assert from 'node:assert'

let runtimeListeners = []
let storageState = {}

function createChromeMock() {
  runtimeListeners = []
  storageState = {}

  return {
    tabs: {
      update: mock.fn(() => Promise.resolve()),
      sendMessage: mock.fn(() => Promise.resolve({ ok: true }))
    },
    runtime: {
      onMessage: {
        addListener: mock.fn((listener) => runtimeListeners.push(listener)),
        removeListener: mock.fn((listener) => {
          runtimeListeners = runtimeListeners.filter((candidate) => candidate !== listener)
        })
      }
    },
    storage: {
      local: {
        set: mock.fn((data, callback) => {
          storageState = { ...storageState, ...data }
          callback?.()
          return Promise.resolve()
        }),
        remove: mock.fn((keys, callback) => {
          for (const key of Array.isArray(keys) ? keys : [keys]) {
            delete storageState[key]
          }
          callback?.()
          return Promise.resolve()
        })
      }
    },
    action: {
      setBadgeText: mock.fn(),
      setBadgeBackgroundColor: mock.fn()
    }
  }
}

function dispatchRuntimeMessage(message) {
  for (const listener of [...runtimeListeners]) {
    listener(message)
  }
}

describe('recording capture branding', () => {
  beforeEach(() => {
    mock.reset()
    globalThis.chrome = createChromeMock()
    delete globalThis.KABOOM_TEST_TIMEOUT_SCALE
  })

  test('requestRecordingGesture uses Kaboom approval copy for denied requests', async () => {
    const { requestRecordingGesture } = await import('../../../extension/background/recording/capture.js')

    const pending = requestRecordingGesture(
      { id: 77, url: 'https://target.example/form' },
      'brand-check',
      15,
      '',
      'Screen'
    )

    await new Promise((resolve) => setTimeout(resolve, 0))

    assert.deepStrictEqual(globalThis.chrome.tabs.update.mock.calls[0].arguments, [77, { active: true }])
    assert.strictEqual(globalThis.chrome.tabs.sendMessage.mock.calls.length, 1)
    assert.match(globalThis.chrome.tabs.sendMessage.mock.calls[0].arguments[1].text, /Open KaBOOM!/)

    dispatchRuntimeMessage({ type: 'recording_gesture_denied' })
    const result = await pending

    assert.strictEqual(result.status, 'error')
    assert.match(String(result.error || ''), /KaBOOM! popup/)
  })

  test('requestRecordingGesture timeout reminder keeps Kaboom popup copy', async () => {
    globalThis.KABOOM_TEST_TIMEOUT_SCALE = 0.001
    const { requestRecordingGesture } = await import('../../../extension/background/recording/capture.js')

    const result = await requestRecordingGesture(
      { id: 88, url: 'https://target.example/timeout' },
      'brand-timeout',
      15,
      '',
      'Screen'
    )

    assert.strictEqual(result.status, 'error')
    assert.match(String(result.error || ''), /Open the KaBOOM! popup, click Approve/)
    assert.strictEqual(globalThis.chrome.tabs.sendMessage.mock.calls.length, 2)
    assert.match(globalThis.chrome.tabs.sendMessage.mock.calls[0].arguments[1].text, /Open KaBOOM!/)
    assert.match(globalThis.chrome.tabs.sendMessage.mock.calls[1].arguments[1].text, /Open KaBOOM!/)
  })
})
