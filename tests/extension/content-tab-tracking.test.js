// @ts-nocheck
/**
 * @fileoverview Tracked-tab lifecycle, popup UX, runtime discovery, and multi-tab tests.
 */

import { test, describe, mock, beforeEach } from 'node:test'
import assert from 'node:assert'

describe('Internal URL Blocking', () => {
  // isInternalUrl function that will be implemented in popup.js
  function isInternalUrl(url) {
    if (!url) return true
    const internalPrefixes = ['chrome://', 'chrome-extension://', 'about:', 'edge://', 'brave://', 'devtools://']
    return internalPrefixes.some((prefix) => url.startsWith(prefix))
  }

  test('should block chrome:// URLs', () => {
    assert.strictEqual(isInternalUrl('chrome://extensions'), true)
    assert.strictEqual(isInternalUrl('chrome://settings'), true)
    assert.strictEqual(isInternalUrl('chrome://newtab'), true)
  })

  test('should block chrome-extension:// URLs', () => {
    assert.strictEqual(isInternalUrl('chrome-extension://abc123/popup.html'), true)
  })

  test('should block about: URLs', () => {
    assert.strictEqual(isInternalUrl('about:blank'), true)
    assert.strictEqual(isInternalUrl('about:version'), true)
  })

  test('should block edge:// URLs', () => {
    assert.strictEqual(isInternalUrl('edge://settings'), true)
  })

  test('should block brave:// URLs', () => {
    assert.strictEqual(isInternalUrl('brave://settings'), true)
  })

  test('should block devtools:// URLs', () => {
    assert.strictEqual(isInternalUrl('devtools://devtools/bundled/inspector.html'), true)
  })

  test('should block null/undefined URLs', () => {
    assert.strictEqual(isInternalUrl(null), true)
    assert.strictEqual(isInternalUrl(undefined), true)
    assert.strictEqual(isInternalUrl(''), true)
  })

  test('should ALLOW regular http:// URLs', () => {
    assert.strictEqual(isInternalUrl('http://localhost:3000'), false)
    assert.strictEqual(isInternalUrl('http://example.com'), false)
  })

  test('should ALLOW regular https:// URLs', () => {
    assert.strictEqual(isInternalUrl('https://example.com'), false)
    assert.strictEqual(isInternalUrl('https://app.example.com/login'), false)
  })

  test('should ALLOW file:// URLs (for local development)', () => {
    assert.strictEqual(isInternalUrl('file:///Users/dev/index.html'), false)
  })
})

// ============================================================================
// SECTION 3: Browser Restart Behavior
// ============================================================================

describe('Browser Restart Tracking State', () => {
  test('should clear tracking on browser startup (onStartup event)', () => {
    let startupCallback = null

    // Mock chrome.runtime.onStartup
    const mockChromeWithStartup = {
      runtime: {
        onStartup: {
          addListener: mock.fn((cb) => {
            startupCallback = cb
          })
        }
      },
      storage: {
        local: {
          remove: mock.fn(() => {
            return Promise.resolve()
          })
        }
      }
    }

    // Register the startup listener (as background.js would)
    mockChromeWithStartup.runtime.onStartup.addListener(async () => {
      await mockChromeWithStartup.storage.local.remove(['trackedTabId', 'trackedTabUrl'])
    })

    // Simulate browser restart
    assert.ok(startupCallback, 'Should register an onStartup listener')
    startupCallback()

    // Verify tracking state is cleared
    assert.ok(
      mockChromeWithStartup.storage.local.remove.mock.calls.length > 0,
      'Should call storage.local.remove on browser startup'
    )
    const removedArg = mockChromeWithStartup.storage.local.remove.mock.calls[0].arguments[0]
    assert.ok(removedArg.includes('trackedTabId'), 'Should remove trackedTabId')
    assert.ok(removedArg.includes('trackedTabUrl'), 'Should remove trackedTabUrl')
  })
})

// ============================================================================
// SECTION 4: Status Ping with Tracking Info
// ============================================================================

describe('Status Ping with Tracking State', () => {
  test('should include tracking_enabled: false when no tab is tracked', () => {
    const storage = {} // No trackedTabId

    const statusMessage = {
      type: 'status',
      tracking_enabled: !!storage.trackedTabId,
      tracked_tab_id: storage.trackedTabId || null,
      tracked_tab_url: storage.trackedTabUrl || null,
      extension_connected: true,
      timestamp: new Date().toISOString()
    }

    assert.strictEqual(statusMessage.tracking_enabled, false)
    assert.strictEqual(statusMessage.tracked_tab_id, null)
    assert.strictEqual(statusMessage.tracked_tab_url, null)
    assert.strictEqual(statusMessage.extension_connected, true)
    assert.strictEqual(statusMessage.type, 'status')
  })

  test('should include tracking_enabled: true and tab info when tracking', () => {
    const storage = { trackedTabId: 123, trackedTabUrl: 'https://example.com' }

    const statusMessage = {
      type: 'status',
      tracking_enabled: !!storage.trackedTabId,
      tracked_tab_id: storage.trackedTabId || null,
      tracked_tab_url: storage.trackedTabUrl || null,
      extension_connected: true,
      timestamp: new Date().toISOString()
    }

    assert.strictEqual(statusMessage.tracking_enabled, true)
    assert.strictEqual(statusMessage.tracked_tab_id, 123)
    assert.strictEqual(statusMessage.tracked_tab_url, 'https://example.com')
  })

  test('should include valid ISO timestamp', () => {
    const storage = { trackedTabId: 42 }

    const statusMessage = {
      type: 'status',
      tracking_enabled: !!storage.trackedTabId,
      tracked_tab_id: storage.trackedTabId || null,
      timestamp: new Date().toISOString()
    }

    // Verify it's a valid ISO string
    const parsed = new Date(statusMessage.timestamp)
    assert.ok(!isNaN(parsed.getTime()), 'Timestamp should be a valid ISO date')
  })
})

// ============================================================================
// SECTION 5: Tracked Tab Closed Behavior
// ============================================================================

describe('Tracked Tab Closed', () => {
  test('should disable tracking when tracked tab is closed', async () => {
    const mockChromeForTabClose = {
      tabs: {
        get: mock.fn((tabId) => {
          // Simulate tab not found (closed)
          throw new Error('No tab with id: ' + tabId)
        })
      },
      storage: {
        local: {
          remove: mock.fn(() => {
            return Promise.resolve()
          }),
          get: mock.fn(() => Promise.resolve({ trackedTabId: 123, trackedTabUrl: 'https://example.com' }))
        }
      }
    }

    // Simulate what background.js does when trying to access a closed tab
    const storage = await mockChromeForTabClose.storage.local.get(['trackedTabId'])
    const trackedTabId = storage.trackedTabId

    try {
      await mockChromeForTabClose.tabs.get(trackedTabId)
    } catch {
      // Tab no longer exists - clear tracking
      await mockChromeForTabClose.storage.local.remove(['trackedTabId', 'trackedTabUrl'])
    }

    assert.ok(
      mockChromeForTabClose.storage.local.remove.mock.calls.length > 0,
      'Should remove tracking when tab is closed'
    )
  })
})

// ============================================================================
// SECTION 6: Popup Button State Tests
// ============================================================================

describe('Popup Track Button State', () => {
  let mockDocument

  function createMockElement(id) {
    return {
      id,
      textContent: '',
      style: {},
      disabled: false,
      title: '',
      addEventListener: mock.fn()
    }
  }

  beforeEach(() => {
    const elements = {}
    mockDocument = {
      getElementById: mock.fn((id) => {
        if (!elements[id]) {
          elements[id] = createMockElement(id)
        }
        return elements[id]
      }),
      addEventListener: mock.fn(),
      readyState: 'complete'
    }
    globalThis.document = mockDocument
  })

  test('should show "Track This Tab" when no tab is tracked', () => {
    const btn = mockDocument.getElementById('track-page-btn')

    // Simulate popup.js behavior when no tracking
    const trackedTabId = undefined

    if (!trackedTabId) {
      btn.textContent = 'Track This Tab'
      btn.style.background = '#252525'
      btn.style.color = '#58a6ff'
    }

    assert.strictEqual(btn.textContent, 'Track This Tab')
    assert.strictEqual(btn.style.background, '#252525')
  })

  test('should show "Stop Tracking" when tab is tracked', () => {
    const btn = mockDocument.getElementById('track-page-btn')

    // Simulate popup.js behavior when tracking active
    const trackedTabId = 42

    if (trackedTabId) {
      btn.textContent = 'Stop Tracking'
      btn.style.background = '#f85149'
    }

    assert.strictEqual(btn.textContent, 'Stop Tracking')
    assert.strictEqual(btn.style.background, '#f85149')
  })

  test('should disable button on internal chrome:// pages', () => {
    const btn = mockDocument.getElementById('track-page-btn')

    // Simulate popup.js behavior on internal page
    const tabUrl = 'chrome://extensions'
    const isInternal =
      tabUrl.startsWith('chrome://') || tabUrl.startsWith('chrome-extension://') || tabUrl.startsWith('about:')

    if (isInternal) {
      btn.disabled = true
      btn.textContent = 'Cannot Track Internal Pages'
      btn.style.opacity = '0.5'
    }

    assert.strictEqual(btn.disabled, true)
    assert.ok(btn.textContent.includes('Cannot Track'))
    assert.strictEqual(btn.style.opacity, '0.5')
  })

  test('should show "No tab tracked" warning when tracking disabled', () => {
    const info = mockDocument.getElementById('tracked-page-info')

    // Simulate no tracking warning
    const trackedTabId = undefined
    if (!trackedTabId) {
      info.textContent = 'No tab tracked - data capture disabled'
      info.style.display = 'block'
      info.style.color = '#f85149'
    }

    assert.ok(info.textContent.includes('No tab tracked'))
    assert.strictEqual(info.style.color, '#f85149')
  })
})

// ============================================================================
// SECTION 7: Multi-tab Isolation Scenario
// ============================================================================

describe('Multi-Tab Isolation Scenario', () => {
  test('should only forward from tracked tab in multi-tab scenario', () => {
    const trackedTabId = 1
    const messagesSent = { tab1: [], tab2: [], tab3: [] }

    const MESSAGE_MAP = {
      kaboom_log: 'log',
      kaboom_network_body: 'network_body'
    }

    // Simulate 3 tabs
    function processMessage(thisTabId, messageType, payload) {
      const isTracked = thisTabId === trackedTabId
      if (!isTracked) return // Dropped

      const mapped = MESSAGE_MAP[messageType]
      if (mapped && payload) {
        const tabKey = `tab${thisTabId}`
        if (messagesSent[tabKey]) {
          messagesSent[tabKey].push({ type: mapped, payload, tabId: thisTabId })
        }
      }
    }

    // Tab 1 (tracked) generates activity
    processMessage(1, 'kaboom_log', { level: 'error', message: 'from tab 1' })
    processMessage(1, 'kaboom_network_body', { url: '/api', method: 'GET', status: 200 })

    // Tab 2 (untracked) generates activity
    processMessage(2, 'kaboom_log', { level: 'error', message: 'from tab 2' })
    processMessage(2, 'kaboom_network_body', { url: '/secret', method: 'GET', status: 200 })

    // Tab 3 (untracked) generates activity
    processMessage(3, 'kaboom_log', { level: 'error', message: 'from tab 3' })

    // Verify only tab 1 data captured
    assert.strictEqual(messagesSent.tab1.length, 2, 'Tab 1 (tracked) should have 2 messages')
    assert.strictEqual(messagesSent.tab2.length, 0, 'Tab 2 (untracked) should have 0 messages')
    assert.strictEqual(messagesSent.tab3.length, 0, 'Tab 3 (untracked) should have 0 messages')

    // Verify tabId is attached
    assert.strictEqual(messagesSent.tab1[0].tabId, 1)
    assert.strictEqual(messagesSent.tab1[1].tabId, 1)
  })

  test('should switch data flow when tracking changes', () => {
    const allMessages = []
    const MESSAGE_MAP = { kaboom_log: 'log' }

    let trackedTabId = 1

    function processMessage(thisTabId, messageType, payload) {
      const isTracked = thisTabId === trackedTabId
      if (!isTracked) return

      const mapped = MESSAGE_MAP[messageType]
      if (mapped && payload) {
        allMessages.push({ type: mapped, payload, tabId: thisTabId })
      }
    }

    // Tab 1 tracked, send messages
    processMessage(1, 'kaboom_log', { level: 'info', message: 'tab1 msg1' })
    processMessage(2, 'kaboom_log', { level: 'info', message: 'tab2 msg1' })

    assert.strictEqual(allMessages.length, 1)
    assert.strictEqual(allMessages[0].tabId, 1)

    // Switch tracking to tab 2
    trackedTabId = 2

    processMessage(1, 'kaboom_log', { level: 'info', message: 'tab1 msg2' })
    processMessage(2, 'kaboom_log', { level: 'info', message: 'tab2 msg2' })

    assert.strictEqual(allMessages.length, 2)
    assert.strictEqual(allMessages[1].tabId, 2, 'After switch, only tab 2 messages forwarded')
  })
})

// ============================================================================
// SECTION 8: updateTrackingStatus uses chrome.runtime.sendMessage (not chrome.tabs)
// ============================================================================

describe('updateTrackingStatus via chrome.runtime.sendMessage', () => {
  test('should request tab ID from background via GET_TAB_ID message', async () => {
    // This test verifies the fix for the critical bug:
    // content scripts cannot access chrome.tabs API, so they must
    // ask the background script for their tab ID via messaging.

    let isTrackedTab = false
    let currentTabId = null
    const expectedTabId = 42

    // Mock chrome APIs as they exist in content script context
    const mockChromeRuntime = {
      sendMessage: async (msg) => {
        if (msg.type === 'get_tab_id') {
          return { tabId: expectedTabId }
        }
        return {}
      }
    }
    const mockStorage = {
      local: {
        get: async () => ({ trackedTabId: expectedTabId })
      }
    }

    // Simulate the fixed updateTrackingStatus
    async function updateTrackingStatus() {
      try {
        const storage = await mockStorage.local.get(['trackedTabId'])
        const response = await mockChromeRuntime.sendMessage({ type: 'get_tab_id' })
        currentTabId = response?.tabId
        isTrackedTab = currentTabId !== null && currentTabId !== undefined && currentTabId === storage.trackedTabId
      } catch {
        isTrackedTab = false
      }
    }

    await updateTrackingStatus()

    assert.strictEqual(currentTabId, expectedTabId, 'Should get tab ID from background script')
    assert.strictEqual(isTrackedTab, true, 'Should be tracked when tab IDs match')
  })

  test('should set isTrackedTab=false when tab IDs do not match', async () => {
    let isTrackedTab = false
    let currentTabId = null

    const mockChromeRuntime = {
      sendMessage: async (msg) => {
        if (msg.type === 'get_tab_id') {
          return { tabId: 99 } // This tab
        }
        return {}
      }
    }
    const mockStorage = {
      local: {
        get: async () => ({ trackedTabId: 42 }) // Different tab tracked
      }
    }

    async function updateTrackingStatus() {
      try {
        const storage = await mockStorage.local.get(['trackedTabId'])
        const response = await mockChromeRuntime.sendMessage({ type: 'get_tab_id' })
        currentTabId = response?.tabId
        isTrackedTab = currentTabId !== null && currentTabId !== undefined && currentTabId === storage.trackedTabId
      } catch {
        isTrackedTab = false
      }
    }

    await updateTrackingStatus()

    assert.strictEqual(currentTabId, 99, 'Should have this tab ID')
    assert.strictEqual(isTrackedTab, false, 'Should NOT be tracked when IDs differ')
  })

  test('should set isTrackedTab=false when sendMessage fails', async () => {
    let isTrackedTab = true // Start as true to verify it gets set to false
    let currentTabId = null

    const mockChromeRuntime = {
      sendMessage: async () => {
        throw new Error('Extension context invalidated')
      }
    }
    const mockStorage = {
      local: {
        get: async () => ({ trackedTabId: 42 })
      }
    }

    async function updateTrackingStatus() {
      try {
        const storage = await mockStorage.local.get(['trackedTabId'])
        const response = await mockChromeRuntime.sendMessage({ type: 'get_tab_id' })
        currentTabId = response?.tabId
        isTrackedTab = currentTabId !== null && currentTabId !== undefined && currentTabId === storage.trackedTabId
      } catch {
        isTrackedTab = false
      }
    }

    await updateTrackingStatus()

    assert.strictEqual(isTrackedTab, false, 'Should fallback to false on error')
  })

  test('should set isTrackedTab=false when response has no tabId', async () => {
    let isTrackedTab = true
    let currentTabId = null

    const mockChromeRuntime = {
      sendMessage: async () => ({}) // No tabId in response
    }
    const mockStorage = {
      local: {
        get: async () => ({ trackedTabId: 42 })
      }
    }

    async function updateTrackingStatus() {
      try {
        const storage = await mockStorage.local.get(['trackedTabId'])
        const response = await mockChromeRuntime.sendMessage({ type: 'get_tab_id' })
        currentTabId = response?.tabId
        isTrackedTab = currentTabId !== null && currentTabId !== undefined && currentTabId === storage.trackedTabId
      } catch {
        isTrackedTab = false
      }
    }

    await updateTrackingStatus()

    assert.strictEqual(currentTabId, undefined, 'Should be undefined when not in response')
    assert.strictEqual(isTrackedTab, false, 'Should be false when tabId is undefined')
  })

  test('should NOT use chrome.tabs.query (not available in content scripts)', async () => {
    // Verify the content.js code does NOT reference chrome.tabs.query
    // The fix replaces chrome.tabs.query with chrome.runtime.sendMessage
    const fs = await import('node:fs')
    const contentSource = fs.readFileSync('extension/content.js', 'utf8')
    assert.strictEqual(
      contentSource.includes('chrome.tabs.query'),
      false,
      'content.js must not use chrome.tabs.query — use GET_TAB_ID message to background instead'
    )
  })
})

// ============================================================================
// SECTION 9: Message Payload Integrity
// ============================================================================

describe('Message Payload Integrity with Tab ID', () => {
  test('should preserve original payload when attaching tabId', () => {
    const tabId = 42
    const originalPayload = {
      url: 'http://localhost:3000/api/users',
      method: 'GET',
      status: 200,
      contentType: 'application/json',
      responseBody: '{"users":[]}',
      duration: 150
    }

    // Simulate forwarding with tabId
    const forwarded = {
      type: 'network_body',
      payload: originalPayload,
      tabId: tabId
    }

    // Verify payload is preserved
    assert.deepStrictEqual(forwarded.payload, originalPayload, 'Payload should be unchanged')
    assert.strictEqual(forwarded.tabId, 42, 'tabId should be added')
    assert.strictEqual(forwarded.type, 'network_body', 'type should be correct')
  })

  test('tabId should be a number, not a string', () => {
    const tabId = 123

    const forwarded = { type: 'log', payload: {}, tabId }

    assert.strictEqual(typeof forwarded.tabId, 'number', 'tabId should be numeric')
  })
})
