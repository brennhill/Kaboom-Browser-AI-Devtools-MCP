// @ts-nocheck
/**
 * @fileoverview background-errors-comms.test.js — Tests for error deduplication,
 * screenshot capture, debug logging, context annotation monitoring, enhanced actions
 * server communication, GET_TAB_ID message handler, and performance snapshot batching.
 */

import { test, describe, mock, beforeEach } from 'node:test'
import assert from 'node:assert'
import { MANIFEST_VERSION } from '../shared/helpers.js'
import { DebugCategory } from '../../../extension/background/debug.js'
import { composeBackgroundHandlers } from '../shared/background-message-router.js'

// Mock Chrome APIs
const mockChrome = {
  runtime: {
    onMessage: {
      addListener: mock.fn()
    },
    onInstalled: {
      addListener: mock.fn()
    },
    sendMessage: mock.fn(() => Promise.resolve()),
    getManifest: () => ({ version: MANIFEST_VERSION })
  },
  action: {
    setBadgeText: mock.fn(),
    setBadgeBackgroundColor: mock.fn()
  },
  storage: {
    local: {
      get: mock.fn((keys, callback) => callback({ logLevel: 'error' })),
      set: mock.fn((data, callback) => callback && callback()),
      remove: mock.fn((keys, callback) => {
        if (typeof callback === 'function') callback()
        else return Promise.resolve()
      })
    },
    sync: {
      get: mock.fn((keys, callback) => callback({})),
      set: mock.fn((data, callback) => callback && callback()),
      remove: mock.fn((keys, callback) => {
        if (typeof callback === 'function') callback()
        else return Promise.resolve()
      })
    },
    session: {
      get: mock.fn((keys, callback) => callback({})),
      set: mock.fn((data, callback) => callback && callback()),
      remove: mock.fn((keys, callback) => {
        if (typeof callback === 'function') callback()
        else return Promise.resolve()
      })
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

// Set global chrome mock
globalThis.chrome = mockChrome

// Import after mocking
import { createLogBatcher } from '../../../extension/background/sync/batchers.js'
import { sendEnhancedActionsToServer } from '../../../extension/background/sync/server.js'
import {
  createErrorSignature,
  processErrorGroup,
  flushErrorGroups
} from '../../../extension/background/caches/error-groups.js'
import { canTakeScreenshot, recordScreenshot } from '../../../extension/background/caches/cache-limits.js'

// Import installMessageListener directly (not re-exported from barrel)
import { installMessageListener } from '../../../extension/background/message-handlers.js'

// =============================================================================
// Helper: Install the real message handler and extract the registered listener
// =============================================================================
/**
 * Installs the real message handler via installMessageListener and returns
 * the actual handler function that was registered on chrome.runtime.onMessage.
 * Allows tests to invoke the real production handler directly.
 */
function getInstalledMessageHandler(depsOverrides = {}) {
  // Reset the addListener mock to capture the new registration
  mockChrome.runtime.onMessage.addListener.mock.resetCalls()

  const baseDeps = {
    getServerUrl: () => 'http://localhost:7890',
    getConnectionStatus: () => ({ connected: true }),
    getDebugMode: () => false,
    getScreenshotOnError: () => false,
    getSourceMapEnabled: () => false,
    getCurrentLogLevel: () => 'all',
    getContextWarning: () => null,
    getCircuitBreakerState: () => 'closed',
    getMemoryPressureState: () => ({ level: 'normal' }),
    getAiWebPilotEnabled: () => false,
    isNetworkBodyCaptureDisabled: () => false,
    setServerUrl: mock.fn(),
    setCurrentLogLevel: mock.fn(),
    setScreenshotOnError: mock.fn(),
    setSourceMapEnabled: mock.fn(),
    setDebugMode: mock.fn(),
    setAiWebPilotEnabled: mock.fn(),
    addToLogBatcher: mock.fn(),
    addToWsBatcher: mock.fn(),
    addToEnhancedActionBatcher: mock.fn(),
    addToNetworkBodyBatcher: mock.fn(),
    addToPerfBatcher: mock.fn(),
    handleLogMessage: mock.fn(),
    handleClearLogs: mock.fn(),
    captureScreenshot: mock.fn(),
    checkConnectionAndUpdate: mock.fn(),
    clearSourceMapCache: mock.fn(),
    debugLog: mock.fn(),
    exportDebugLog: mock.fn(),
    clearDebugLog: mock.fn(),
    saveSetting: mock.fn(),
    forwardToAllContentScripts: mock.fn(),
    ...depsOverrides
  }

  installMessageListener({ debugLog: baseDeps.debugLog, handlers: composeBackgroundHandlers(baseDeps) })

  // The real handler was registered via chrome.runtime.onMessage.addListener
  const calls = mockChrome.runtime.onMessage.addListener.mock.calls
  assert.ok(calls.length > 0, 'installMessageListener should register a listener')
  const handler = calls[calls.length - 1].arguments[0]
  assert.strictEqual(typeof handler, 'function', 'Registered listener should be a function')

  return { handler, deps: baseDeps }
}

describe('createErrorSignature', () => {
  test('should create consistent signature for exception', () => {
    const entry = {
      type: 'exception',
      level: 'error',
      message: 'Cannot read property x',
      stack: 'Error: Cannot read property x\n    at foo.js:10:5'
    }

    const sig1 = createErrorSignature(entry)
    const sig2 = createErrorSignature(entry)

    assert.strictEqual(sig1, sig2)
    assert.ok(sig1.includes('exception'))
    assert.ok(sig1.includes('error'))
    assert.ok(sig1.includes('Cannot read property x'))
  })

  test('should create different signatures for different exceptions', () => {
    const entry1 = {
      type: 'exception',
      level: 'error',
      message: 'Error A',
      stack: 'Error: Error A\n    at file1.js:10'
    }
    const entry2 = {
      type: 'exception',
      level: 'error',
      message: 'Error B',
      stack: 'Error: Error B\n    at file2.js:20'
    }

    const sig1 = createErrorSignature(entry1)
    const sig2 = createErrorSignature(entry2)

    assert.notStrictEqual(sig1, sig2)
  })

  test('should create signature for network error', () => {
    const entry = {
      type: 'network',
      level: 'error',
      method: 'POST',
      url: 'http://localhost:3000/api/users?id=123',
      status: 401
    }

    const sig = createErrorSignature(entry)

    assert.ok(sig.includes('network'))
    assert.ok(sig.includes('POST'))
    assert.ok(sig.includes('/api/users')) // Path without query
    assert.ok(sig.includes('401'))
  })

  test('should create signature for console log', () => {
    const entry = {
      type: 'console',
      level: 'error',
      args: ['User authentication failed']
    }

    const sig = createErrorSignature(entry)

    assert.ok(sig.includes('console'))
    assert.ok(sig.includes('User authentication failed'))
  })
})

describe('processErrorGroup', () => {
  beforeEach(() => {
    // Clear error groups between tests by calling flushErrorGroups repeatedly
    flushErrorGroups()
    flushErrorGroups()
  })

  test('should send first occurrence of error', () => {
    const entry = {
      type: 'exception',
      level: 'error',
      message: 'Test error',
      ts: new Date().toISOString()
    }

    const result = processErrorGroup(entry)

    assert.strictEqual(result.shouldSend, true)
    assert.deepStrictEqual(result.entry, entry)
    assert.strictEqual(result.entry.type, 'exception')
    assert.strictEqual(result.entry.level, 'error')
    assert.strictEqual(result.entry.message, 'Test error')
  })

  test('should not send duplicate error within dedup window', () => {
    const entry = {
      type: 'exception',
      level: 'error',
      message: 'Duplicate error test',
      ts: new Date().toISOString()
    }

    // First occurrence
    const result1 = processErrorGroup(entry)
    assert.strictEqual(result1.shouldSend, true)

    // Immediate duplicate
    const result2 = processErrorGroup({ ...entry, ts: new Date().toISOString() })
    assert.strictEqual(result2.shouldSend, false)
  })

  test('should always send non-error logs', () => {
    const entry = {
      type: 'console',
      level: 'log',
      args: ['Info message'],
      ts: new Date().toISOString()
    }

    const result1 = processErrorGroup(entry)
    const result2 = processErrorGroup({ ...entry })

    assert.strictEqual(result1.shouldSend, true)
    assert.strictEqual(result2.shouldSend, true)
  })

  test('should track warn level for grouping', () => {
    const entry = {
      type: 'console',
      level: 'warn',
      args: ['Warning message'],
      ts: new Date().toISOString()
    }

    const result1 = processErrorGroup(entry)
    assert.strictEqual(result1.shouldSend, true)

    const result2 = processErrorGroup({ ...entry })
    assert.strictEqual(result2.shouldSend, false)
  })
})

describe('flushErrorGroups', () => {
  beforeEach(() => {
    // Clear error groups
    flushErrorGroups()
    flushErrorGroups()
  })

  test('should return empty array when no duplicates', () => {
    const entry = {
      type: 'exception',
      level: 'error',
      message: 'Single error',
      ts: new Date().toISOString()
    }

    processErrorGroup(entry)
    const flushed = flushErrorGroups()

    // Only 1 occurrence, count = 1, nothing to aggregate
    assert.strictEqual(flushed.length, 0)
  })

  test('should return aggregated entry when duplicates exist', () => {
    const entry = {
      type: 'exception',
      level: 'error',
      message: 'Repeated error for flush',
      ts: new Date().toISOString()
    }

    // Create duplicates
    processErrorGroup(entry)
    processErrorGroup({ ...entry })
    processErrorGroup({ ...entry })

    const flushed = flushErrorGroups()

    assert.strictEqual(flushed.length, 1)
    assert.strictEqual(flushed[0]._aggregatedCount, 3)
    assert.ok(flushed[0]._firstSeen)
    assert.ok(flushed[0]._lastSeen)
    assert.strictEqual(flushed[0].type, 'exception')
    assert.strictEqual(flushed[0].level, 'error')
    assert.strictEqual(flushed[0].message, 'Repeated error for flush')
    assert.strictEqual(typeof flushed[0]._firstSeen, 'string')
    assert.strictEqual(typeof flushed[0]._lastSeen, 'string')
  })
})

describe('canTakeScreenshot', () => {
  test('should allow first screenshot', () => {
    const result = canTakeScreenshot(999) // Use unique tab ID

    assert.strictEqual(result.allowed, true)
  })

  test('should rate limit rapid screenshots', () => {
    const tabId = 1000

    // First screenshot - allowed
    recordScreenshot(tabId)

    // Immediate second - should be rate limited
    const result = canTakeScreenshot(tabId)

    assert.strictEqual(result.allowed, false)
    assert.strictEqual(result.reason, 'rate_limit')
    assert.ok(result.nextAllowedIn > 0)
  })

  test('should enforce session limit', () => {
    const tabId = 1001

    // Record 30 screenshots (max per session)
    for (let i = 0; i < 30; i++) {
      recordScreenshot(tabId)
    }

    // Wait a bit to pass rate limit
    const result = canTakeScreenshot(tabId)

    // Either rate limited or session limited
    assert.strictEqual(result.allowed, false)
  })
})

describe('recordScreenshot', () => {
  test('should record timestamp', () => {
    const tabId = 2000

    // First check - should be allowed
    const before = canTakeScreenshot(tabId)
    assert.strictEqual(before.allowed, true)

    // Record screenshot
    recordScreenshot(tabId)

    // Second check - should be rate limited
    const after = canTakeScreenshot(tabId)
    assert.strictEqual(after.allowed, false)
  })
})

// NOTE: captureScreenshot is not exported from background.js; screenshot.ts owns it.
// These tests are skipped until captureScreenshot is exposed via the public API
describe('captureScreenshot', () => {
  test('should capture screenshot and save via server', async (t) => {
    t.skip('captureScreenshot not exported - owned by screenshot.js')
  })

  test('should POST screenshot data to server', async (t) => {
    t.skip('captureScreenshot not exported - owned by screenshot.js')
  })

  test('should link screenshot to error when relatedErrorId provided', async (t) => {
    t.skip('captureScreenshot not exported - owned by screenshot.js')
  })

  test('should set trigger to manual when no relatedErrorId', async (t) => {
    t.skip('captureScreenshot not exported - owned by screenshot.js')
  })

  test('should respect rate limiting', async (t) => {
    t.skip('captureScreenshot not exported - owned by screenshot.js')
  })

  test('should handle capture errors gracefully', async (t) => {
    t.skip('captureScreenshot not exported - owned by screenshot.js')
  })

  test('should handle server error gracefully', async (t) => {
    t.skip('captureScreenshot not exported - owned by screenshot.js')
  })
})

// NOTE: Source map tests (decodeVLQ, parseMappings, parseStackFrame, extractSourceMapUrl,
// parseSourceMapData, findOriginalLocation, clearSourceMapCache) have been moved to
// co-located test file: extension/background/snapshots.test.js

describe('Debug Logging', () => {
  test('should log debug entries', async () => {
    const { debugLog, getDebugLog, clearDebugLog } = await import('../../../extension/background/index.js')

    // Clear any existing entries
    clearDebugLog()

    // Log some entries
    debugLog(DebugCategory.LIFECYCLE, 'Test message 1')
    debugLog(DebugCategory.CONNECTION, 'Test message 2', { extra: 'data' })

    const entries = getDebugLog()
    assert.ok(entries.length >= 2)

    // Find our test entries
    const msg1 = entries.find((e) => e.message === 'Test message 1')
    const msg2 = entries.find((e) => e.message === 'Test message 2')

    assert.ok(msg1)
    assert.strictEqual(msg1.category, 'lifecycle')

    assert.ok(msg2)
    assert.strictEqual(msg2.category, 'connection')
    assert.deepStrictEqual(msg2.data, { extra: 'data' })
  })

  test('should clear debug log', async () => {
    const { debugLog, getDebugLog, clearDebugLog } = await import('../../../extension/background/index.js')

    // Add an entry
    debugLog(DebugCategory.ERROR, 'Error test')

    // Clear
    clearDebugLog()

    const entries = getDebugLog()
    // Should have at least one entry (from the clear itself logging)
    // But not the error test entry
    const errorEntry = entries.find((e) => e.message === 'Error test')
    assert.ok(!errorEntry)
  })

  test('should export debug log as JSON', async () => {
    const { debugLog, exportDebugLog, clearDebugLog } = await import('../../../extension/background/index.js')

    clearDebugLog()
    debugLog(DebugCategory.CAPTURE, 'Capture test')

    const exported = exportDebugLog()
    const parsed = JSON.parse(exported)

    assert.ok(parsed.exportedAt)
    assert.strictEqual(typeof parsed.exportedAt, 'string')
    assert.strictEqual(parsed.version, MANIFEST_VERSION)
    assert.ok(Array.isArray(parsed.entries))
    assert.ok(parsed.entries.length > 0, 'Expected at least one entry in exported log')
    // Verify entry shape
    const captureEntry = parsed.entries.find((e) => e.message === 'Capture test')
    assert.ok(captureEntry, 'Expected to find the "Capture test" entry')
    assert.strictEqual(captureEntry.category, 'capture')
    assert.ok(captureEntry.ts, 'Expected ts field on entry')
    assert.strictEqual(typeof captureEntry.ts, 'string')
  })

  // NOTE: setDebugMode test moved to co-located test file: extension/background/index.test.js

  test('should limit debug log buffer size', async () => {
    const { debugLog, getDebugLog, clearDebugLog } = await import('../../../extension/background/index.js')

    clearDebugLog()

    // Add more than 200 entries
    for (let i = 0; i < 250; i++) {
      debugLog(DebugCategory.CAPTURE, `Entry ${i}`)
    }

    const entries = getDebugLog()
    // Should be capped at 200
    assert.ok(entries.length <= 200)
  })

  test('should queue debug logs even while disconnected for later sync flush', async () => {
    const { debugLog } = await import('../../../extension/background/index.js')
    const { clearExtensionLogsForTesting, getExtensionLogQueueSnapshot } = await import(
      '../../../extension/background/runtime-state/log-queue.js'
    )
    const { setConnectionStatus } = await import(
      '../../../extension/background/runtime-state/connection-state.js'
    )

    clearExtensionLogsForTesting()
    setConnectionStatus({ connected: false })

    debugLog(DebugCategory.CONNECTION, 'Sync failed, retrying', { attempt: 1 })

    const queued = getExtensionLogQueueSnapshot()
    assert.ok(
      queued.some((entry) => entry.message === 'Sync failed, retrying'),
      'Expected disconnected debug logs to remain queued for next successful sync'
    )
  })
})

// =============================================================================
// V5: Enhanced Actions Server Communication
// =============================================================================

describe('Enhanced Actions Server Communication', () => {
  beforeEach(() => {
    globalThis.fetch = mock.fn(() =>
      Promise.resolve({
        ok: true,
        json: () => Promise.resolve({ received: 1 })
      })
    )
  })

  test('sendEnhancedActionsToServer should POST actions to /enhanced-actions', async () => {
    const actions = [
      { type: 'click', timestamp: 1705312200000, url: 'http://localhost:3000', selectors: { testId: 'btn' } },
      {
        type: 'input',
        timestamp: 1705312201000,
        url: 'http://localhost:3000',
        selectors: { id: 'email' },
        value: 'test@test.com',
        input_type: 'email'
      }
    ]

    await sendEnhancedActionsToServer('http://localhost:7890', actions)

    assert.strictEqual(globalThis.fetch.mock.calls.length, 1)
    const [url, opts] = globalThis.fetch.mock.calls[0].arguments
    assert.ok(url.endsWith('/enhanced-actions'))
    assert.strictEqual(opts.method, 'POST')
    assert.strictEqual(opts.headers['Content-Type'], 'application/json')

    const body = JSON.parse(opts.body)
    assert.ok(Array.isArray(body.actions))
    assert.strictEqual(body.actions.length, 2)
    assert.strictEqual(body.actions[0].type, 'click')
    assert.strictEqual(body.actions[0].timestamp, 1705312200000)
    assert.strictEqual(body.actions[0].url, 'http://localhost:3000')
    assert.deepStrictEqual(body.actions[0].selectors, { testId: 'btn' })
    assert.strictEqual(body.actions[1].type, 'input')
    assert.strictEqual(body.actions[1].value, 'test@test.com')
    assert.strictEqual(body.actions[1].input_type, 'email')
  })

  test('sendEnhancedActionsToServer should throw on non-ok response', async () => {
    globalThis.fetch = mock.fn(() => Promise.resolve({ ok: false, status: 500 }))

    const actions = [{ type: 'click', timestamp: 1000, url: 'http://localhost:3000', selectors: {} }]

    await assert.rejects(
      () => sendEnhancedActionsToServer('http://localhost:7890', actions),
      (err) => err.message.includes('500')
    )
  })

  test('enhanced action batcher should batch and send actions', async () => {
    const flushFn = mock.fn()
    const batcher = createLogBatcher(flushFn, { debounceMs: 50, maxBatchSize: 50 })

    const action1 = { type: 'click', timestamp: 1000, url: 'http://localhost:3000', selectors: { id: 'btn' } }
    const action2 = {
      type: 'input',
      timestamp: 1001,
      url: 'http://localhost:3000',
      selectors: { id: 'input' },
      value: 'hi'
    }

    batcher.add(action1)
    batcher.add(action2)

    // Wait for debounce
    await new Promise((r) => setTimeout(r, 100))

    assert.strictEqual(flushFn.mock.calls.length, 1)
    const flushedActions = flushFn.mock.calls[0].arguments[0]
    assert.strictEqual(flushedActions.length, 2)
    assert.strictEqual(flushedActions[0].type, 'click')
    assert.strictEqual(flushedActions[0].timestamp, 1000)
    assert.strictEqual(flushedActions[0].url, 'http://localhost:3000')
    assert.deepStrictEqual(flushedActions[0].selectors, { id: 'btn' })
    assert.strictEqual(flushedActions[1].type, 'input')
    assert.strictEqual(flushedActions[1].timestamp, 1001)
    assert.strictEqual(flushedActions[1].value, 'hi')
  })

  test('message handler should process enhanced_action messages via batcher', () => {
    // Install the real message handler with a spy on addToEnhancedActionBatcher
    const addToEnhancedActionBatcher = mock.fn()
    const { handler } = getInstalledMessageHandler({ addToEnhancedActionBatcher })

    const sendResponse = mock.fn()
    const sender = { tab: { id: 1, url: 'http://localhost:3000' } }
    const payload = { type: 'click', timestamp: 1000, url: 'http://localhost:3000', selectors: { testId: 'btn' } }
    const message = { type: 'enhanced_action', payload }

    // Invoke the real message handler with an enhanced_action message
    handler(message, sender, sendResponse)

    // The real handler should have called addToEnhancedActionBatcher with the payload
    assert.strictEqual(addToEnhancedActionBatcher.mock.calls.length, 1)
    assert.deepStrictEqual(addToEnhancedActionBatcher.mock.calls[0].arguments[0], payload)
    assert.strictEqual(addToEnhancedActionBatcher.mock.calls[0].arguments[0].type, 'click')
  })
})

// ============================================
// GET_TAB_ID message handler (content script tab ID resolution)
// ============================================

describe('GET_TAB_ID Message Handler', () => {
  test('should respond with sender.tab.id when content script requests tab ID', () => {
    // Install the real message handler and get the registered listener
    const { handler } = getInstalledMessageHandler()

    const sendResponse = mock.fn()
    // Sender must pass isValidMessageSender: needs tab.id and tab.url
    const sender = { tab: { id: 42, url: 'http://localhost:3000' } }
    const message = { type: 'get_tab_id' }

    // Invoke the real handler registered by installMessageListener
    handler(message, sender, sendResponse)

    assert.strictEqual(sendResponse.mock.calls.length, 1)
    assert.deepStrictEqual(sendResponse.mock.calls[0].arguments[0], { tabId: 42 })
  })

  test('should respond with undefined tabId when sender has no tab', () => {
    // Install the real message handler and get the registered listener
    const { handler } = getInstalledMessageHandler()

    const sendResponse = mock.fn()
    // Sender from popup/extension page: uses chrome.runtime.id for validation
    mockChrome.runtime.id = 'test-extension-id'
    const sender = { id: 'test-extension-id' } // No tab (e.g., from popup)
    const message = { type: 'get_tab_id' }

    handler(message, sender, sendResponse)

    assert.strictEqual(sendResponse.mock.calls.length, 1)
    assert.deepStrictEqual(sendResponse.mock.calls[0].arguments[0], { tabId: undefined })

    // Clean up
    delete mockChrome.runtime.id
  })

  test('should return true for async response handling', () => {
    // Install the real message handler and get the registered listener
    const { handler } = getInstalledMessageHandler()

    const sendResponse = mock.fn()
    const sender = { tab: { id: 99, url: 'http://localhost:3000' } }
    const message = { type: 'get_tab_id' }

    // The real handler returns true to keep the sendResponse channel open
    const returnValue = handler(message, sender, sendResponse)

    assert.strictEqual(returnValue, true, 'Handler should return true for sendResponse')
  })
})

// ============================================
// Phase 6 (W6): perfBatcher replaces direct fetch
// ============================================

describe('Performance Snapshot Batching (W6)', () => {
  test('sendPerformanceSnapshotToServer does NOT exist — replaced by perfBatcher', async () => {
    const bgModule = await import('../../../extension/background/sync/server.js')
    assert.strictEqual(
      typeof bgModule.sendPerformanceSnapshotToServer,
      'undefined',
      'sendPerformanceSnapshotToServer should be removed; performance snapshots now use perfBatcher'
    )
  })

  test('sendPerformanceSnapshotsToServer exists as the batch sender', async () => {
    const serverModule = await import('../../../extension/background/sync/server.js')
    assert.strictEqual(
      typeof serverModule.sendPerformanceSnapshotsToServer,
      'function',
      'sendPerformanceSnapshotsToServer (plural, batch) should be owned by server.js'
    )
  })
})
