// @ts-nocheck
/**
 * @fileoverview Content-script tab isolation, payload forwarding, and tracking-transition tests.
 */

import { test, describe, mock, beforeEach } from 'node:test'
import assert from 'node:assert'
import { createMockChrome } from './helpers.js'

// ============================================================================
// SECTION 1: Tab Filtering Logic (content.js behavior)
// ============================================================================

describe('Content Script Tab Filtering', () => {
  let mockChrome
  let messagesSent
  let _storageChangeListeners
  let isTrackedTab
  let currentTabId
  let contextValid

  // Simulate the content.js filtering logic
  function createContentScriptSimulator(trackedTabId, thisTabId) {
    isTrackedTab = thisTabId === trackedTabId
    currentTabId = thisTabId
    contextValid = true
    messagesSent = []
    _storageChangeListeners = []

    const MESSAGE_MAP = {
      kaboom_log: 'log',
      kaboom_ws: 'ws_event',
      kaboom_network_body: 'network_body',
      kaboom_enhanced_action: 'enhanced_action',
      kaboom_performance_snapshot: 'performance_snapshot'
    }

    function safeSendMessage(msg) {
      if (!contextValid) return
      messagesSent.push(msg)
    }

    // The message handler with tab filtering
    function handleMessage(event) {
      if (event.source !== globalThis.window) return undefined

      const { type: messageType, payload } = event.data || {}

      // Tab isolation filter: drop messages from untracked tabs
      if (!isTrackedTab) {
        return undefined // Drop message
      }

      // Forward messages with tabId attached
      const mapped = MESSAGE_MAP[messageType]
      if (mapped && payload && typeof payload === 'object') {
        safeSendMessage({ type: mapped, payload, tabId: currentTabId })
      }
      return undefined
    }

    function updateTrackingStatus(newTrackedTabId) {
      isTrackedTab = currentTabId === newTrackedTabId
    }

    return { handleMessage, updateTrackingStatus, MESSAGE_MAP }
  }

  beforeEach(() => {
    mockChrome = createMockChrome()
    globalThis.chrome = mockChrome
    messagesSent = []

    globalThis.window = {
      addEventListener: mock.fn(),
      postMessage: mock.fn(),
      location: { origin: 'http://localhost:3000', href: 'http://localhost:3000/' }
    }
  })

  // --------------------------------------------------------------------------
  // Core filtering tests
  // --------------------------------------------------------------------------

  test('should DROP messages from untracked tab', () => {
    const sim = createContentScriptSimulator(999, 1) // tracked=999, this=1

    sim.handleMessage({
      source: globalThis.window,
      data: { type: 'kaboom_log', payload: { level: 'error', message: 'should be dropped' } }
    })

    assert.strictEqual(messagesSent.length, 0, 'Messages from untracked tab should be dropped')
  })

  test('should FORWARD messages from tracked tab', () => {
    const sim = createContentScriptSimulator(1, 1) // tracked=1, this=1

    sim.handleMessage({
      source: globalThis.window,
      data: { type: 'kaboom_log', payload: { level: 'error', message: 'should be forwarded' } }
    })

    assert.strictEqual(messagesSent.length, 1, 'Messages from tracked tab should be forwarded')
    assert.strictEqual(messagesSent[0].type, 'log')
  })

  test('should DROP all message types from untracked tab', () => {
    const sim = createContentScriptSimulator(999, 1) // untracked

    const messageTypes = [
      { type: 'kaboom_log', payload: { level: 'error', message: 'test' } },
      { type: 'kaboom_ws', payload: { event: 'open', url: 'ws://localhost' } },
      { type: 'kaboom_network_body', payload: { url: '/api', method: 'GET', status: 200 } },
      { type: 'kaboom_enhanced_action', payload: { type: 'click', url: '/page' } },
      { type: 'kaboom_performance_snapshot', payload: { lcp: 100 } }
    ]

    for (const msg of messageTypes) {
      sim.handleMessage({ source: globalThis.window, data: msg })
    }

    assert.strictEqual(messagesSent.length, 0, 'All message types should be dropped from untracked tab')
  })

  test('should FORWARD all message types from tracked tab', () => {
    const sim = createContentScriptSimulator(42, 42) // tracked

    const messageTypes = [
      { type: 'kaboom_log', payload: { level: 'error', message: 'test' } },
      { type: 'kaboom_ws', payload: { event: 'open', url: 'ws://localhost' } },
      { type: 'kaboom_network_body', payload: { url: '/api', method: 'GET', status: 200 } },
      { type: 'kaboom_enhanced_action', payload: { type: 'click', url: '/page' } },
      { type: 'kaboom_performance_snapshot', payload: { lcp: 100 } }
    ]

    for (const msg of messageTypes) {
      sim.handleMessage({ source: globalThis.window, data: msg })
    }

    assert.strictEqual(messagesSent.length, 5, 'All 5 message types should be forwarded from tracked tab')
  })

  test('should DROP messages when no tab is tracked (trackedTabId undefined)', () => {
    const sim = createContentScriptSimulator(undefined, 1) // no tracking

    sim.handleMessage({
      source: globalThis.window,
      data: { type: 'kaboom_log', payload: { level: 'error', message: 'no tracking' } }
    })

    assert.strictEqual(messagesSent.length, 0, 'Messages should be dropped when no tab is tracked')
  })

  test('should DROP messages when no tab is tracked (trackedTabId null)', () => {
    const sim = createContentScriptSimulator(null, 1) // no tracking

    sim.handleMessage({
      source: globalThis.window,
      data: { type: 'kaboom_log', payload: { level: 'error', message: 'no tracking' } }
    })

    assert.strictEqual(messagesSent.length, 0, 'Messages should be dropped when trackedTabId is null')
  })

  // --------------------------------------------------------------------------
  // Tab ID attachment tests (USER REQUIREMENT)
  // --------------------------------------------------------------------------

  test('should attach tabId to ALL forwarded messages', () => {
    const sim = createContentScriptSimulator(42, 42)

    sim.handleMessage({
      source: globalThis.window,
      data: { type: 'kaboom_log', payload: { level: 'error', message: 'test' } }
    })

    assert.strictEqual(messagesSent.length, 1)
    assert.strictEqual(messagesSent[0].tabId, 42, 'tabId should be attached to forwarded message')
  })

  test('should attach correct tabId to each message type', () => {
    const tabId = 123
    const sim = createContentScriptSimulator(tabId, tabId)

    const messageTypes = [
      { type: 'kaboom_log', payload: { level: 'error', message: 'test' } },
      { type: 'kaboom_network_body', payload: { url: '/api', method: 'GET', status: 200 } },
      { type: 'kaboom_ws', payload: { event: 'open', url: 'ws://localhost' } }
    ]

    for (const msg of messageTypes) {
      sim.handleMessage({ source: globalThis.window, data: msg })
    }

    for (let i = 0; i < messagesSent.length; i++) {
      assert.strictEqual(messagesSent[i].tabId, tabId, `Message ${i} should have tabId ${tabId}`)
    }
  })

  // --------------------------------------------------------------------------
  // Cross-origin / source filtering
  // --------------------------------------------------------------------------

  test('should still reject messages from non-window sources regardless of tracking', () => {
    const sim = createContentScriptSimulator(1, 1) // tracked

    sim.handleMessage({
      source: {}, // Not window
      data: { type: 'kaboom_log', payload: { level: 'error', message: 'injected' } }
    })

    assert.strictEqual(messagesSent.length, 0, 'Messages from non-window sources should be rejected')
  })

  // --------------------------------------------------------------------------
  // Storage change events (tracking status updates)
  // --------------------------------------------------------------------------

  test('should update tracking status when trackedTabId changes in storage', () => {
    const sim = createContentScriptSimulator(999, 1) // initially untracked

    // Simulate trackedTabId changing to this tab
    sim.updateTrackingStatus(1)

    // Now should forward messages
    sim.handleMessage({
      source: globalThis.window,
      data: { type: 'kaboom_log', payload: { level: 'error', message: 'now tracked' } }
    })

    assert.strictEqual(messagesSent.length, 1, 'Should forward after tracking status changes to this tab')
  })

  test('should stop forwarding when tracking switches to different tab', () => {
    const sim = createContentScriptSimulator(1, 1) // initially tracked

    // Forward one message while tracked
    sim.handleMessage({
      source: globalThis.window,
      data: { type: 'kaboom_log', payload: { level: 'info', message: 'tracked' } }
    })
    assert.strictEqual(messagesSent.length, 1)

    // Switch tracking to different tab
    sim.updateTrackingStatus(999) // now tracking tab 999, not tab 1

    // This message should be dropped
    sim.handleMessage({
      source: globalThis.window,
      data: { type: 'kaboom_log', payload: { level: 'error', message: 'should drop' } }
    })

    assert.strictEqual(messagesSent.length, 1, 'Should stop forwarding when tracking switches away')
  })

  test('should stop forwarding when tracking is disabled (trackedTabId removed)', () => {
    const sim = createContentScriptSimulator(1, 1) // initially tracked

    // Forward while tracked
    sim.handleMessage({
      source: globalThis.window,
      data: { type: 'kaboom_log', payload: { level: 'info', message: 'tracked' } }
    })
    assert.strictEqual(messagesSent.length, 1)

    // Disable tracking
    sim.updateTrackingStatus(undefined) // removed

    // Should be dropped
    sim.handleMessage({
      source: globalThis.window,
      data: { type: 'kaboom_log', payload: { level: 'error', message: 'no tracking' } }
    })

    assert.strictEqual(messagesSent.length, 1, 'Should stop forwarding when tracking disabled')
  })
})

// ============================================================================
// SECTION 2: Internal URL Blocking (popup.js behavior)
// ============================================================================
