// @ts-nocheck
import { beforeEach, describe, mock, test } from 'node:test'
import assert from 'node:assert'

let commandListeners = []

describe('push handler branding', () => {
  beforeEach(() => {
    mock.reset()
    commandListeners = []

    globalThis.fetch = mock.fn(async (url) => {
      if (String(url).endsWith('/push/capabilities')) {
        return {
          ok: true,
          json: async () => ({
            push_enabled: true,
            supports_sampling: true,
            supports_notifications: false,
            client_name: 'Claude',
            inbox_count: 0
          })
        }
      }
      if (String(url).endsWith('/push/screenshot')) {
        return { ok: false, status: 503, json: async () => ({}) }
      }
      throw new Error(`unexpected fetch url: ${url}`)
    })

    globalThis.chrome = {
      runtime: {
        id: 'test-extension-id',
        getManifest: () => ({ version: '1.0.0' })
      },
      commands: {
        onCommand: {
          addListener: mock.fn((listener) => commandListeners.push(listener))
        }
      },
      tabs: {
        query: mock.fn(() => Promise.resolve([{ id: 44, url: 'https://active.example', windowId: 2 }])),
        captureVisibleTab: mock.fn(() => Promise.resolve('data:image/png;base64,abc')),
        sendMessage: mock.fn(() => Promise.resolve({ ok: true }))
      },
      windows: {
        WINDOW_ID_CURRENT: -2
      }
    }
  })

  test('failed screenshot push mentions the Kaboom daemon', async () => {
    const { installPushCommandListener } = await import('../../../extension/background/push-handler.js')

    installPushCommandListener()
    await commandListeners[0]('push_screenshot')

    const failureToast = globalThis.chrome.tabs.sendMessage.mock.calls.at(-1).arguments[1]
    assert.strictEqual(failureToast.text, 'Screenshot push failed')
    assert.match(failureToast.detail, /KaBOOM! daemon/)
  })
})
