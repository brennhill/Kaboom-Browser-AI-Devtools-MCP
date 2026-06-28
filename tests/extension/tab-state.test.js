// @ts-nocheck
import { beforeEach, describe, mock, test } from 'node:test'
import assert from 'node:assert'

import {
  pingContentScript,
  waitForTabLoad,
  forwardToAllContentScripts,
  loadSavedSettings,
  loadAiWebPilotState,
  loadDebugModeState,
  saveSetting,
  getTrackedTabInfo,
  setTrackedTab,
  clearTrackedTab,
  getAllConfigSettings,
  getActiveTab,
  sendTabToast
} from '../../extension/background/tab-state.js'

function createChromeMock() {
  return {
    action: {
      setBadgeText: mock.fn(),
      setBadgeBackgroundColor: mock.fn()
    },
    tabs: {
      sendMessage: mock.fn(() => Promise.resolve({ status: 'alive' })),
      query: mock.fn(() => Promise.resolve([])),
      get: mock.fn((tabId) =>
        Promise.resolve({
          id: tabId,
          status: 'complete',
          active: true
        })
      )
    },
    storage: {
      local: {
        get: mock.fn(() => Promise.resolve({})),
        set: mock.fn(() => Promise.resolve()),
        remove: mock.fn(() => Promise.resolve())
      }
    }
  }
}

describe('tab-state helpers', () => {
  beforeEach(() => {
    mock.reset()
    globalThis.chrome = createChromeMock()
  })

  test('pingContentScript returns true for alive response', async () => {
    const result = await pingContentScript(7, 25)
    assert.strictEqual(result, true)
  })

  test('pingContentScript returns false on timeout and exceptions', async () => {
    globalThis.chrome.tabs.sendMessage = mock.fn(() => new Promise(() => {}))
    assert.strictEqual(await pingContentScript(8, 5), false)

    globalThis.chrome.tabs.sendMessage = mock.fn(() => Promise.reject(new Error('boom')))
    assert.strictEqual(await pingContentScript(8, 5), false)
  })

  test('pingContentScript returns false for non-alive responses', async () => {
    globalThis.chrome.tabs.sendMessage = mock.fn(() => Promise.resolve({ status: 'not-alive' }))
    assert.strictEqual(await pingContentScript(9, 25), false)
  })

  test('waitForTabLoad returns true when tab is complete and false on timeout/error', async () => {
    assert.strictEqual(await waitForTabLoad(11, 25), true)

    globalThis.chrome.tabs.get = mock.fn(() => Promise.resolve({ id: 11, status: 'loading', active: true }))
    assert.strictEqual(await waitForTabLoad(11, 110), false)
    assert.strictEqual(await waitForTabLoad(11, 0), false)

    globalThis.chrome.tabs.get = mock.fn(() => Promise.reject(new Error('closed')))
    assert.strictEqual(await waitForTabLoad(11, 25), false)
  })

  test('forwardToAllContentScripts logs only unexpected errors', async () => {
    const debugLog = mock.fn()
    globalThis.chrome.tabs.query = mock.fn(() =>
      Promise.resolve([
        { id: 1 },
        { id: 2 },
        { id: 3 }
      ])
    )
    globalThis.chrome.tabs.sendMessage = mock.fn((tabId) => {
      if (tabId === 2) return Promise.reject(new Error('Could not establish connection'))
      if (tabId === 3) return Promise.reject(new Error('unexpected send failure'))
      return Promise.resolve({})
    })

    await forwardToAllContentScripts({ type: 'TEST' }, debugLog)
    await new Promise((resolve) => setTimeout(resolve, 0))

    assert.strictEqual(globalThis.chrome.tabs.sendMessage.mock.calls.length, 3)
    assert.strictEqual(debugLog.mock.calls.length, 1)
    assert.strictEqual(debugLog.mock.calls[0].arguments[0], 'error')
  })

  test('forwardToAllContentScripts no-ops when chrome tabs API is unavailable', async () => {
    globalThis.chrome = undefined
    await forwardToAllContentScripts({ type: 'TEST_NOOP' })
  })

  test('forwardToAllContentScripts skips tabless entries and tolerated receiver-missing errors', async () => {
    const debugLog = mock.fn()
    globalThis.chrome.tabs.query = mock.fn(() => Promise.resolve([{ id: 0 }, {}, { id: 7 }]))
    globalThis.chrome.tabs.sendMessage = mock.fn(() => Promise.reject(new Error('Receiving end does not exist')))

    await forwardToAllContentScripts({ type: 'TEST_TOLERATED' }, debugLog)
    await new Promise((resolve) => setTimeout(resolve, 0))

    assert.strictEqual(globalThis.chrome.tabs.sendMessage.mock.calls.length, 1)
    assert.strictEqual(debugLog.mock.calls.length, 0)
  })

  test('loadSavedSettings handles unavailable storage and get failures', async () => {
    const warn = mock.method(console, 'warn')
    globalThis.chrome = undefined
    assert.deepStrictEqual(await loadSavedSettings(), {})

    globalThis.chrome = createChromeMock()
    globalThis.chrome.storage.local.get = mock.fn(() => Promise.resolve({ serverUrl: 'http://localhost:7890' }))
    assert.deepStrictEqual(await loadSavedSettings(), { serverUrl: 'http://localhost:7890' })

    globalThis.chrome.storage.local.get = mock.fn(() => Promise.reject(new Error('storage down')))
    assert.deepStrictEqual(await loadSavedSettings(), {})
    assert.strictEqual(warn.mock.calls.length, 1)
  })

  test('loadAiWebPilotState and loadDebugModeState read persisted flags', async () => {
    globalThis.chrome.storage.local.get = mock.fn((keys) => {
      const keyList = Array.isArray(keys) ? keys : [keys]
      if (keyList.includes('aiWebPilotEnabled')) return Promise.resolve({ aiWebPilotEnabled: false })
      if (keyList.includes('debugMode')) return Promise.resolve({ debugMode: true })
      return Promise.resolve({})
    })

    const logs = []
    assert.strictEqual(await loadAiWebPilotState((line) => logs.push(line)), false)
    assert.ok(logs[0].includes('AI Web Pilot loaded on startup: false'))
    assert.strictEqual(await loadDebugModeState(), true)
  })

  test('loadAiWebPilotState defaults to enabled and loadDebugModeState defaults to disabled when chrome is unavailable', async () => {
    globalThis.chrome = undefined
    // AI Web Pilot is opt-out: enabled unless explicitly set to false (getLocal -> undefined => true).
    assert.strictEqual(await loadAiWebPilotState(), true)
    // Debug mode is opt-in: disabled unless explicitly set to true.
    assert.strictEqual(await loadDebugModeState(), false)
  })

  test('saveSetting writes key/value to local storage', () => {
    saveSetting('featureX', 'on')
    assert.deepStrictEqual(globalThis.chrome.storage.local.set.mock.calls[0].arguments[0], { featureX: 'on' })
  })

  test('saveSetting no-ops when storage is unavailable', () => {
    globalThis.chrome = undefined
    saveSetting('featureY', 'off')
  })

  test('tracked tab helpers persist, retrieve, and clear tab state', async () => {
    await setTrackedTab({ id: 99, url: 'https://example.com/app', title: 'Example' })
    assert.deepStrictEqual(globalThis.chrome.storage.local.set.mock.calls[0].arguments[0], {
      trackedTabId: 99,
      trackedTabUrl: 'https://example.com/app',
      trackedTabTitle: 'Example'
    })

    globalThis.chrome.storage.local.get = mock.fn(() =>
      Promise.resolve({
        trackedTabId: 99,
        trackedTabUrl: 'https://example.com/app',
        trackedTabTitle: 'Example'
      })
    )
    globalThis.chrome.tabs.get = mock.fn(() => Promise.resolve({ id: 99, status: 'loading', active: false }))
    const tracked = await getTrackedTabInfo()
    assert.deepStrictEqual(tracked, {
      trackedTabId: 99,
      trackedTabUrl: 'https://example.com/app',
      trackedTabTitle: 'Example',
      tabStatus: 'loading',
      trackedTabActive: false
    })

    clearTrackedTab()
    assert.deepStrictEqual(globalThis.chrome.storage.local.remove.mock.calls[0].arguments[0], [
      'trackedTabId',
      'trackedTabUrl',
      'trackedTabTitle'
    ])
  })

  test('getTrackedTabInfo reports complete status when tracked tab is loaded', async () => {
    globalThis.chrome.storage.local.get = mock.fn(() =>
      Promise.resolve({
        trackedTabId: 101,
        trackedTabUrl: 'https://ready.example',
        trackedTabTitle: 'Ready'
      })
    )
    globalThis.chrome.tabs.get = mock.fn(() => Promise.resolve({ id: 101, status: 'complete', active: true }))

    assert.deepStrictEqual(await getTrackedTabInfo(), {
      trackedTabId: 101,
      trackedTabUrl: 'https://ready.example',
      trackedTabTitle: 'Ready',
      tabStatus: 'complete',
      trackedTabActive: true
    })
  })

  test('setTrackedTab no-ops for missing tab id', async () => {
    await setTrackedTab({ id: undefined, url: 'https://x.test', title: 'X' })
    assert.strictEqual(globalThis.chrome.storage.local.set.mock.calls.length, 0)
  })

  test('clearTrackedTab no-ops when storage is unavailable', () => {
    globalThis.chrome = undefined
    clearTrackedTab()
  })

  test('getTrackedTabInfo returns null-state when chrome is unavailable', async () => {
    globalThis.chrome = undefined
    assert.deepStrictEqual(await getTrackedTabInfo(), {
      trackedTabId: null,
      trackedTabUrl: null,
      trackedTabTitle: null,
      tabStatus: null,
      trackedTabActive: null
    })
  })

  test('getTrackedTabInfo tolerates tab lookup failures', async () => {
    globalThis.chrome.storage.local.get = mock.fn(() =>
      Promise.resolve({
        trackedTabId: 44,
        trackedTabUrl: 'https://missing-tab.test',
        trackedTabTitle: 'Missing Tab'
      })
    )
    globalThis.chrome.tabs.get = mock.fn(() => Promise.reject(new Error('No tab with id')))

    assert.deepStrictEqual(await getTrackedTabInfo(), {
      trackedTabId: 44,
      trackedTabUrl: 'https://missing-tab.test',
      trackedTabTitle: 'Missing Tab',
      tabStatus: null,
      trackedTabActive: null
    })
  })

  test('getAllConfigSettings and getActiveTab return deterministic fallbacks', async () => {
    globalThis.chrome.storage.local.get = mock.fn(() => Promise.resolve({ aiWebPilotEnabled: true }))
    assert.deepStrictEqual(await getAllConfigSettings(), { aiWebPilotEnabled: true })

    globalThis.chrome.tabs.query = mock.fn(() => Promise.resolve([{ id: 123, url: 'https://active.test' }]))
    assert.deepStrictEqual(await getActiveTab(), { id: 123, url: 'https://active.test' })

    globalThis.chrome.tabs.query = mock.fn(() => Promise.resolve([{ url: 'https://missing-id.test' }]))
    assert.strictEqual(await getActiveTab(), null)

    globalThis.chrome = undefined
    assert.deepStrictEqual(await getAllConfigSettings(), {})
  })

  test('sendTabToast sends toast and ignores content-script errors', async () => {
    globalThis.chrome = createChromeMock()
    globalThis.chrome.tabs.sendMessage = mock.fn(() => Promise.reject(new Error('content script missing')))
    sendTabToast(55, 'Saved', 'detail', 'success', 1234)
    await new Promise((resolve) => setTimeout(resolve, 0))
    assert.strictEqual(globalThis.chrome.tabs.sendMessage.mock.calls.length, 1)
  })
})
