// @ts-nocheck
import { beforeEach, describe, test, mock } from 'node:test'
import assert from 'node:assert'

let onShownListener = null

function createChromeMock({ trackedTabId = null, drawModeActive = false } = {}) {
  onShownListener = null
  return {
    contextMenus: {
      removeAll: mock.fn((cb) => cb && cb()),
      create: mock.fn(),
      update: mock.fn((_id, _opts, cb) => cb && cb()),
      refresh: mock.fn(),
      onClicked: { addListener: mock.fn() },
      onShown: {
        addListener: mock.fn((listener) => {
          onShownListener = listener
        })
      }
    },
    storage: {
      local: {
        get: mock.fn((key) => {
          if (key === 'trackedTabId' || (Array.isArray(key) && key.includes('trackedTabId'))) {
            return Promise.resolve({ trackedTabId })
          }
          return Promise.resolve({})
        })
      }
    },
    tabs: {
      sendMessage: mock.fn((_tabId, message) => {
        if (message?.type === 'kaboom_get_annotations') {
          return Promise.resolve({ draw_mode_active: drawModeActive })
        }
        return Promise.resolve({})
      })
    },
    runtime: { id: 'test-extension-id' }
  }
}

describe('context menu dynamic labels', () => {
  beforeEach(() => {
    globalThis.chrome = createChromeMock()
  })

  test('shows stop labels when recording/action recording/draw mode are active', async () => {
    globalThis.chrome = createChromeMock({ trackedTabId: 88, drawModeActive: true })
    const { installContextMenus } = await import('../../../extension/background/ui/context-menus.js')
    installContextMenus(
      {
        isRecording: () => true,
        startRecording: async () => ({ status: 'recording', name: 'x', startTime: Date.now() }),
        stopRecording: async () => ({ status: 'saved', name: 'x' })
      },
      {
        isRecording: () => true,
        startRecording: async () => ({ status: 'recording' }),
        stopRecording: async () => ({ status: 'saved' })
      }
    )

    assert.ok(onShownListener, 'Expected onShown listener registration')
    onShownListener({}, { id: 88 })
    await new Promise((r) => setTimeout(r, 0))

    const updates = globalThis.chrome.contextMenus.update.mock.calls.map((c) => [c.arguments[0], c.arguments[1]?.title])
    assert.ok(updates.some(([id, title]) => id === 'kaboom-control-page' && title === 'Release Control'))
    assert.ok(updates.some(([id, title]) => id === 'kaboom-annotate-page' && title === 'Stop Annotation'))
    assert.ok(updates.some(([id, title]) => id === 'kaboom-record-screen' && title === 'Stop Screen Recording'))
    assert.ok(
      updates.some(([id, title]) => id === 'kaboom-action-record' && title === 'Stop User Action Recording')
    )
  })

  test('shows start labels when idle and tab is not controlled', async () => {
    globalThis.chrome = createChromeMock({ trackedTabId: 77, drawModeActive: false })
    const { installContextMenus } = await import('../../../extension/background/ui/context-menus.js')
    installContextMenus(
      {
        isRecording: () => false,
        startRecording: async () => ({ status: 'recording', name: 'x', startTime: Date.now() }),
        stopRecording: async () => ({ status: 'saved', name: 'x' })
      },
      {
        isRecording: () => false,
        startRecording: async () => ({ status: 'recording' }),
        stopRecording: async () => ({ status: 'saved' })
      }
    )

    assert.ok(onShownListener, 'Expected onShown listener registration')
    onShownListener({}, { id: 55 })
    await new Promise((r) => setTimeout(r, 0))

    const updates = globalThis.chrome.contextMenus.update.mock.calls.map((c) => [c.arguments[0], c.arguments[1]?.title])
    assert.ok(updates.some(([id, title]) => id === 'kaboom-control-page' && title === 'Control Tab'))
    assert.ok(updates.some(([id, title]) => id === 'kaboom-annotate-page' && title === 'Annotate Page'))
    assert.ok(updates.some(([id, title]) => id === 'kaboom-record-screen' && title === 'Record Screen'))
    assert.ok(updates.some(([id, title]) => id === 'kaboom-action-record' && title === 'Record User Actions'))
  })
})
