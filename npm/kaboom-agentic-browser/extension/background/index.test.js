// @ts-nocheck
/**
 * @fileoverview index.test.js - Tests for debug mode and export functions
 * Co-located with index.js implementation
 */

import { test, describe, mock, beforeEach } from 'node:test'
import assert from 'node:assert'

// Mock Chrome APIs before importing the module
const mockChrome = {
  runtime: {
    onMessage: {
      addListener: mock.fn()
    },
    onInstalled: {
      addListener: mock.fn()
    },
    sendMessage: mock.fn(() => Promise.resolve()),
    getManifest: () => ({ version: '5.5.0' })
  },
  action: {
    setBadgeText: mock.fn(),
    setBadgeBackgroundColor: mock.fn()
  },
  storage: {
    local: {
      get: mock.fn((keys, callback) => callback({ logLevel: 'error' })),
      set: mock.fn((data, callback) => callback && callback())
    },
    sync: {
      get: mock.fn((keys, callback) => callback({})),
      set: mock.fn((data, callback) => callback && callback())
    },
    session: {
      get: mock.fn((keys, callback) => callback({})),
      set: mock.fn((data, callback) => callback && callback())
    },
    onChanged: {
      addListener: mock.fn()
    }
  },
  alarms: {
    create: mock.fn(),
    onAlarm: {
      addListener: mock.fn()
    }
  },
  tabs: {
    get: mock.fn((tabId) => Promise.resolve({ id: tabId, windowId: 1, url: 'http://localhost:3000' })),
    captureVisibleTab: mock.fn(() =>
      Promise.resolve('data:image/jpeg;base64,/9j/4AAQSkZJRgABAQAAAQABAAD/2wBDAAgGBgcGBQgHBwcJCQgKDBQNDAsLDBkS')
    ),
    query: mock.fn((query, callback) => callback([{ id: 1, windowId: 1 }])),
    onRemoved: {
      addListener: mock.fn()
    }
  }
}

globalThis.chrome = mockChrome

// Import directly from the implementation file (not through barrel export)
import { setDebugMode, exportDebugLog, debugLog, clearDebugLog, DebugCategory } from './index.js'

describe('Debug Mode', () => {
  beforeEach(() => {
    mock.reset()
    clearDebugLog()
  })

  test('should toggle debug mode', () => {
    // Enable debug mode
    setDebugMode(true)

    const exported1 = JSON.parse(exportDebugLog())
    assert.strictEqual(exported1.debugMode, true)

    // Disable debug mode
    setDebugMode(false)

    const exported2 = JSON.parse(exportDebugLog())
    assert.strictEqual(exported2.debugMode, false)
  })

  test('should export debug log as JSON', () => {
    clearDebugLog()
    debugLog(DebugCategory.CAPTURE, 'Capture test')

    const exported = exportDebugLog()
    const parsed = JSON.parse(exported)

    assert.ok(parsed.exportedAt)
    assert.strictEqual(parsed.version, '5.5.0')
    assert.ok(Array.isArray(parsed.entries))
  })
})
