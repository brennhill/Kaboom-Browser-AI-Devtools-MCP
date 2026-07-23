// @ts-nocheck
/**
 * @fileoverview event-listeners.test.js — Tests for tracked tab URL/title updates.
 *
 * Bug: handleTrackedTabUrlChange() updates trackedTabUrl in storage but never
 * updates trackedTabTitle. After any navigation, the title stays stale from
 * when tracking was first enabled. This causes observe(page) to return titles
 * from completely unrelated pages (e.g., a news article title shows up on
 * example.com after the user navigated away from the article).
 *
 * These tests assert CORRECT behavior — they FAIL until the bug is fixed.
 *
 * Run: node --test extension/background/event-listeners.test.js
 */

import { test, describe, beforeEach, mock } from 'node:test'
import assert from 'node:assert'

// ---------------------------------------------------------------------------
// Chrome API mock with observable storage
// ---------------------------------------------------------------------------
let storageData = {}
const mockChrome = {
  storage: {
    local: {
      get: mock.fn((keys, callback) => {
        const result = {}
        const keyList = Array.isArray(keys) ? keys : [keys]
        for (const k of keyList) {
          if (k in storageData) result[k] = storageData[k]
        }
        if (callback) callback(result)
        return Promise.resolve(result)
      }),
      set: mock.fn((data, callback) => {
        Object.assign(storageData, data)
        if (callback) callback()
        return Promise.resolve()
      }),
      remove: mock.fn((keys, callback) => {
        const keyList = Array.isArray(keys) ? keys : [keys]
        for (const k of keyList) delete storageData[k]
        if (callback) callback()
        return Promise.resolve()
      })
    },
    sync: {
      get: mock.fn((keys, callback) => {
        if (callback) callback({})
        return Promise.resolve({})
      }),
      set: mock.fn((data, callback) => {
        if (callback) callback()
        return Promise.resolve()
      }),
      remove: mock.fn((keys, callback) => {
        if (callback) callback()
        return Promise.resolve()
      })
    },
    session: {
      get: mock.fn((keys, callback) => {
        if (callback) callback({})
        return Promise.resolve({})
      }),
      set: mock.fn((data, callback) => {
        if (callback) callback()
        return Promise.resolve()
      }),
      remove: mock.fn((keys, callback) => {
        if (callback) callback()
        return Promise.resolve()
      })
    },
    onChanged: { addListener: mock.fn() }
  },
  tabs: {
    get: mock.fn((tabId) =>
      Promise.resolve({
        id: tabId,
        url: 'https://example.com',
        title: 'Example Domain'
      })
    ),
    onUpdated: { addListener: mock.fn() },
    onRemoved: { addListener: mock.fn() }
  },
  alarms: {
    create: mock.fn(),
    onAlarm: { addListener: mock.fn() }
  },
  runtime: {
    onInstalled: { addListener: mock.fn() },
    onStartup: { addListener: mock.fn() },
    sendMessage: mock.fn(() => Promise.resolve())
  },
  commands: {
    onCommand: { addListener: mock.fn() }
  },
  windows: {
    getLastFocused: mock.fn(() => Promise.resolve({ id: 1 }))
  }
}

globalThis.chrome = mockChrome

// Import after chrome is set up
import { handleTrackedTabUrlChange, getTrackedTabInfo } from './event-listeners.js'

describe('handleTrackedTabUrlChange — title staleness bug', () => {
  beforeEach(() => {
    // Simulate: user started tracking while reading a news article
    storageData = {
      trackedTabId: 42,
      trackedTabUrl: 'https://news.example.com/article',
      trackedTabTitle: 'Breaking News: Something Important'
    }
    mock.restoreAll()

    // After navigation, chrome.tabs.get returns the NEW page info
    mockChrome.tabs.get = mock.fn((tabId) =>
      Promise.resolve({
        id: tabId,
        url: 'https://example.com',
        title: 'Example Domain'
      })
    )
  })

  test('updates trackedTabTitle when tracked tab navigates', async () => {
    // User's tracked tab navigates from news article to example.com
    handleTrackedTabUrlChange(42, 'https://example.com')

    // Allow async storage operations to complete
    await new Promise((resolve) => setTimeout(resolve, 50))

    const info = await getTrackedTabInfo()

    // URL should be updated
    assert.strictEqual(info.trackedTabUrl, 'https://example.com', 'URL should update to new page')

    // Title MUST also update — this is the bug.
    // Currently, title stays as "Breaking News: Something Important"
    // even though the tab is now on example.com.
    assert.notStrictEqual(
      info.trackedTabTitle,
      'Breaking News: Something Important',
      'Title must NOT stay stale from the previous page after navigation.\n' +
        'handleTrackedTabUrlChange only updates trackedTabUrl but never trackedTabTitle.\n' +
        'This causes observe(page) to return titles from completely unrelated pages.'
    )
  })

  test('title matches the new page after URL change', async () => {
    // Tab navigates to a new URL
    handleTrackedTabUrlChange(42, 'https://example.com')

    await new Promise((resolve) => setTimeout(resolve, 50))

    const info = await getTrackedTabInfo()

    // The title should reflect the new page, not the old one
    assert.strictEqual(
      info.trackedTabTitle,
      'Example Domain',
      'After navigation, title should match the new page.\n' +
        'chrome.tabs.get() returns the correct title, but handleTrackedTabUrlChange\n' +
        'never calls it — it only calls chrome.storage.local.set({ trackedTabUrl }).'
    )
  })

  test('does not update title for non-tracked tabs', async () => {
    // A different tab (id=99) navigates — should not affect tracked tab title
    handleTrackedTabUrlChange(99, 'https://other-site.com')

    await new Promise((resolve) => setTimeout(resolve, 50))

    const info = await getTrackedTabInfo()

    // Title should remain unchanged for the tracked tab
    assert.strictEqual(
      info.trackedTabTitle,
      'Breaking News: Something Important',
      'Title for tracked tab should not change when a different tab navigates'
    )
    assert.strictEqual(
      info.trackedTabUrl,
      'https://news.example.com/article',
      'URL for tracked tab should not change when a different tab navigates'
    )
  })
})
