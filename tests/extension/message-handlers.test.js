// @ts-nocheck
/**
 * @fileoverview message-handlers.test.js — Tests for extension/background/message-handlers.js.
 * Covers sender validation, message routing, state snapshots.
 */

import { test, describe, mock, beforeEach } from 'node:test'
import assert from 'node:assert'
import { MANIFEST_VERSION as _MANIFEST_VERSION } from './helpers.js'

const {
  installMessageListener,
  saveStateSnapshot,
  loadStateSnapshot,
  listStateSnapshots,
  deleteStateSnapshot
} = await import('../../extension/background/message-handlers.js')

/**
 * Helper: installs the message listener and returns the raw handler function.
 */
function getInstalledHandler(depsOverrides = {}) {
  const addListenerFn = mock.fn()
  const origOnMessage = chrome.runtime.onMessage
  chrome.runtime.onMessage = { addListener: addListenerFn }

  const defaultDeps = {
    debugLog: mock.fn(),
    addToWsBatcher: mock.fn(),
    addToEnhancedActionBatcher: mock.fn(),
    addToNetworkBodyBatcher: mock.fn(),
    addToPerfBatcher: mock.fn(),
    addToLogBatcher: mock.fn(),
    handleLogMessage: mock.fn(() => Promise.resolve()),
    handleClearLogs: mock.fn(() => Promise.resolve({ success: true })),
    isNetworkBodyCaptureDisabled: mock.fn(() => false),
    getConnectionStatus: mock.fn(() => ({ connected: true })),
    getServerUrl: mock.fn(() => 'http://localhost:9222'),
    getScreenshotOnError: mock.fn(() => false),
    getSourceMapEnabled: mock.fn(() => false),
    getDebugMode: mock.fn(() => false),
    getContextWarning: mock.fn(() => null),
    getCircuitBreakerState: mock.fn(() => 'closed'),
    getMemoryPressureState: mock.fn(() => 'normal'),
    setCurrentLogLevel: mock.fn(),
    setScreenshotOnError: mock.fn(),
    setSourceMapEnabled: mock.fn(),
    setDebugMode: mock.fn(),
    setServerUrl: mock.fn(),
    setAiWebPilotEnabled: mock.fn((val, cb) => cb && cb()),
    getAiWebPilotEnabled: mock.fn(() => false),
    saveSetting: mock.fn(),
    exportDebugLog: mock.fn(() => []),
    clearDebugLog: mock.fn(),
    clearSourceMapCache: mock.fn(),
    captureScreenshot: mock.fn(() => Promise.resolve({ success: true })),
    forwardToAllContentScripts: mock.fn(),
    checkConnectionAndUpdate: mock.fn(),
    ...depsOverrides
  }

  installMessageListener(defaultDeps)
  chrome.runtime.onMessage = origOnMessage

  const handler = addListenerFn.mock.calls[0].arguments[0]
  return { handler, deps: defaultDeps }
}

// Trusted sender: content script with tab context
const contentScriptSender = { tab: { id: 1, url: 'http://localhost:3000' } }
// Trusted sender: extension internal
const extensionSender = { id: chrome.runtime.id }
// Untrusted sender: web page
const webPageSender = { id: 'other-extension-id', url: 'http://evil.com' }

// ============================================
// isValidMessageSender (tested via handler)
// ============================================

describe('sender validation', () => {
  test('content script sender is valid', () => {
    const { handler } = getInstalledHandler()
    const sendResponse = mock.fn()
    handler({ type: 'get_tab_id' }, contentScriptSender, sendResponse)
    assert.strictEqual(sendResponse.mock.calls.length, 1)
    assert.strictEqual(sendResponse.mock.calls[0].arguments[0].tabId, 1)
  })

  test('extension page sender is valid', () => {
    const { handler } = getInstalledHandler()
    const sendResponse = mock.fn()
    handler({ type: 'get_status' }, extensionSender, sendResponse)
    assert.strictEqual(sendResponse.mock.calls.length, 1)
    assert.ok(sendResponse.mock.calls[0].arguments[0].connected !== undefined)
  })

  test('web page sender is rejected', () => {
    const { handler, deps } = getInstalledHandler()
    const sendResponse = mock.fn()
    const result = handler({ type: 'get_status' }, webPageSender, sendResponse)
    assert.strictEqual(result, false)
    assert.strictEqual(sendResponse.mock.calls.length, 0)
    // debugLog should be called with rejection
    assert.ok(deps.debugLog.mock.calls.some(
      c => c.arguments[0] === 'error' && c.arguments[1].includes('untrusted')
    ))
  })
})

// ============================================
// Message routing
// ============================================

describe('message routing', () => {
  test('GET_TAB_ID returns sender tab id', () => {
    const { handler } = getInstalledHandler()
    const sendResponse = mock.fn()
    handler({ type: 'get_tab_id' }, contentScriptSender, sendResponse)
    assert.strictEqual(sendResponse.mock.calls[0].arguments[0].tabId, 1)
  })

  test('ws_event routes to WS batcher', () => {
    const { handler, deps } = getInstalledHandler()
    const payload = { event: 'message', data: 'hello' }
    handler({ type: 'ws_event', payload }, contentScriptSender, mock.fn())
    assert.strictEqual(deps.addToWsBatcher.mock.calls.length, 1)
    assert.deepStrictEqual(deps.addToWsBatcher.mock.calls[0].arguments[0], payload)
  })

  test('enhanced_action routes to action batcher', () => {
    const { handler, deps } = getInstalledHandler()
    const payload = { type: 'click', timestamp: 1000 }
    handler({ type: 'enhanced_action', payload }, contentScriptSender, mock.fn())
    assert.strictEqual(deps.addToEnhancedActionBatcher.mock.calls.length, 1)
  })

  test('network_body routes to body batcher with tab_id', () => {
    const { handler, deps } = getInstalledHandler()
    handler({
      type: 'network_body',
      payload: { url: 'https://api.example.com', status: 200 },
      tabId: 42
    }, contentScriptSender, mock.fn())
    assert.strictEqual(deps.addToNetworkBodyBatcher.mock.calls.length, 1)
    assert.strictEqual(deps.addToNetworkBodyBatcher.mock.calls[0].arguments[0].tab_id, 42)
  })

  test('network_body dropped when capture disabled', () => {
    const { handler, deps } = getInstalledHandler({
      isNetworkBodyCaptureDisabled: mock.fn(() => true)
    })
    handler({
      type: 'network_body',
      payload: { url: 'https://api.example.com' }
    }, contentScriptSender, mock.fn())
    assert.strictEqual(deps.addToNetworkBodyBatcher.mock.calls.length, 0)
    assert.ok(deps.debugLog.mock.calls.some(
      c => c.arguments[1].includes('capture disabled')
    ))
  })

  test('getStatus returns connection info', () => {
    const { handler } = getInstalledHandler({
      getConnectionStatus: mock.fn(() => ({ connected: true, latency: 5 })),
      getServerUrl: mock.fn(() => 'http://localhost:9222')
    })
    const sendResponse = mock.fn()
    handler({ type: 'get_status' }, extensionSender, sendResponse)
    const resp = sendResponse.mock.calls[0].arguments[0]
    assert.strictEqual(resp.connected, true)
    assert.strictEqual(resp.serverUrl, 'http://localhost:9222')
  })

  test('getStatus includes security mode fields when present', () => {
    const { handler } = getInstalledHandler({
      getConnectionStatus: mock.fn(() => ({
        connected: true,
        securityMode: 'insecure_proxy',
        productionParity: false,
        insecureRewritesApplied: ['csp_headers']
      }))
    })
    const sendResponse = mock.fn()
    handler({ type: 'get_status' }, extensionSender, sendResponse)
    const resp = sendResponse.mock.calls[0].arguments[0]
    assert.strictEqual(resp.securityMode, 'insecure_proxy')
    assert.strictEqual(resp.productionParity, false)
    assert.deepStrictEqual(resp.insecureRewritesApplied, ['csp_headers'])
  })

  test('clearLogs calls handler and responds', async () => {
    const { handler } = getInstalledHandler({
      handleClearLogs: mock.fn(() => Promise.resolve({ cleared: 50 }))
    })
    const sendResponse = mock.fn()
    handler({ type: 'clear_logs' }, extensionSender, sendResponse)
    // Wait for async handler
    await new Promise(r => setTimeout(r, 10))
    assert.strictEqual(sendResponse.mock.calls.length, 1)
    assert.strictEqual(sendResponse.mock.calls[0].arguments[0].cleared, 50)
  })

  test('setLogLevel updates level and persists', () => {
    const { handler, deps } = getInstalledHandler()
    handler({ type: 'set_log_level', level: 'warn' }, extensionSender, mock.fn())
    assert.strictEqual(deps.setCurrentLogLevel.mock.calls[0].arguments[0], 'warn')
    assert.ok(deps.saveSetting.mock.calls.some(
      c => c.arguments[0] === 'logLevel' && c.arguments[1] === 'warn'
    ))
  })

  test('unknown message type returns false', () => {
    const { handler } = getInstalledHandler()
    const result = handler({ type: 'UNKNOWN_TYPE_XYZ' }, contentScriptSender, mock.fn())
    assert.strictEqual(result, false)
  })

  test('performance_snapshot routes to perf batcher', () => {
    const { handler, deps } = getInstalledHandler()
    handler({ type: 'performance_snapshot', payload: { metrics: {} } }, contentScriptSender, mock.fn())
    assert.strictEqual(deps.addToPerfBatcher.mock.calls.length, 1)
  })

  test('open_terminal_panel opens the side panel before awaiting setOptions', async () => {
    let resolveSetOptions
    const open = mock.fn(() => Promise.resolve())
    const setOptions = mock.fn(() => new Promise((resolve) => {
      resolveSetOptions = resolve
    }))

    chrome.sidePanel = {
      open,
      setOptions
    }

    const { handler } = getInstalledHandler()
    const sendResponse = mock.fn()

    const result = handler({ type: 'open_terminal_panel' }, contentScriptSender, sendResponse)

    assert.strictEqual(result, true)
    await new Promise((resolve) => setTimeout(resolve, 10))
    assert.strictEqual(setOptions.mock.calls.length, 1)
    assert.strictEqual(open.mock.calls.length, 1, 'sidePanel.open should fire before setOptions resolves')
    assert.strictEqual(open.mock.calls[0].arguments[0].tabId, 1)

    resolveSetOptions()
    await new Promise((resolve) => setTimeout(resolve, 10))
  })

  test('open_terminal_panel calls sidePanel.open synchronously (preserves user gesture, #terminal-panel-broken)', async () => {
    // Regression: the previous code awaited workspace resolution (tab lookup +
    // grouping + focus) BEFORE sidePanel.open(). Those awaits expired the
    // forwarded user gesture, Chrome rejected open(), and the panel never
    // appeared. open() must be invoked synchronously — before any await.
    const open = mock.fn(() => Promise.resolve())
    const setOptions = mock.fn(() => Promise.resolve())
    chrome.sidePanel = { open, setOptions }

    const { handler } = getInstalledHandler()
    handler({ type: 'open_terminal_panel' }, contentScriptSender, mock.fn())

    // No await: open() must already have been invoked within the same tick as
    // dispatch, i.e. before the openTerminalSidePanel body reaches any await.
    assert.strictEqual(open.mock.calls.length, 1, 'sidePanel.open must fire synchronously, before any workspace await')
    assert.strictEqual(open.mock.calls[0].arguments[0].tabId, 1)

    // Let the best-effort workspace refinement settle so it does not bleed.
    await new Promise((resolve) => setTimeout(resolve, 0))
  })

  test('open_terminal_panel creates a Kaboom workspace group around the tracked tab and opens there', async () => {
    chrome.storage.local.get = mock.fn((keys, callback) => {
      const keyList = Array.isArray(keys) ? keys : [keys]
      const result = {}
      for (const key of keyList) {
        if (key === 'trackedTabId') result[key] = 42
        else result[key] = undefined
      }
      callback?.(result)
      return Promise.resolve(result)
    })

    chrome.storage.local.set = mock.fn((_data, callback) => {
      callback?.()
      return Promise.resolve()
    })

    chrome.tabs.get = mock.fn((tabId) => {
      if (tabId === 42) {
        return Promise.resolve({ id: 42, url: 'https://tracked.example/', groupId: -1, windowId: 7, active: false })
      }
      return Promise.resolve({ id: 1, url: 'https://other.example/', groupId: -1, windowId: 1, active: true })
    })
    chrome.tabs.group = mock.fn(() => Promise.resolve(77))
    chrome.tabs.update = mock.fn(() => Promise.resolve())
    chrome.windows = { update: mock.fn(() => Promise.resolve()) }
    chrome.tabGroups = {
      TAB_GROUP_ID_NONE: -1,
      Color: { ORANGE: 'orange' },
      update: mock.fn(() => Promise.resolve())
    }
    // tabGroups is an optional permission; grouping activates only once granted.
    chrome.permissions = { contains: mock.fn(() => Promise.resolve(true)) }

    const open = mock.fn(() => Promise.resolve())
    const setOptions = mock.fn(() => Promise.resolve())
    chrome.sidePanel = { open, setOptions }

    const { handler } = getInstalledHandler()
    const sendResponse = mock.fn()

    const result = handler({ type: 'open_terminal_panel' }, contentScriptSender, sendResponse)

    assert.strictEqual(result, true)
    await new Promise((resolve) => setTimeout(resolve, 0))

    assert.strictEqual(chrome.tabs.group.mock.calls.length, 1, 'tracked tab should be upgraded into a Chrome tab group')
    assert.deepStrictEqual(chrome.tabs.group.mock.calls[0].arguments[0], { tabIds: [42] })
    assert.strictEqual(chrome.tabGroups.update.mock.calls.length, 1, 'workspace group should be named and styled')
    assert.strictEqual(chrome.tabGroups.update.mock.calls[0].arguments[0], 77)
    assert.deepStrictEqual(chrome.tabGroups.update.mock.calls[0].arguments[1], {
      title: 'KaBOOM!',
      color: 'orange',
      collapsed: false
    })
    assert.strictEqual(chrome.tabs.update.mock.calls.length, 1, 'tracked workspace tab should become active (after open, best-effort)')
    assert.deepStrictEqual(chrome.tabs.update.mock.calls[0].arguments, [42, { active: true }])
    assert.strictEqual(open.mock.calls.length, 1)
    // open() fires synchronously on the sender's tab within the user gesture;
    // the workspace group is resolved afterward and only refines setOptions.
    assert.strictEqual(open.mock.calls[0].arguments[0].tabId, 1)
    // The sender's tab is enabled before open() (Chrome rejects open on a tab
    // where the panel is disabled), then the resolved workspace host is enabled
    // too. Both use the one constant path: changing the path reloads the side
    // panel document, which would tear down the xterm that just booted.
    const optionCalls = setOptions.mock.calls.map((c) => c.arguments[0])
    assert.deepStrictEqual(
      optionCalls.map((o) => o.tabId), [1, 42],
      'the opened tab first, then the workspace host'
    )
    for (const options of optionCalls) {
      assert.strictEqual(options.path, 'sidepanel.html', 'the panel path must never vary')
      assert.strictEqual(options.enabled, true)
    }
  })

  test('open_terminal_panel skips workspace grouping when the optional tabGroups permission is not granted', async () => {
    chrome.storage.local.get = mock.fn((keys, callback) => {
      const keyList = Array.isArray(keys) ? keys : [keys]
      const result = {}
      for (const key of keyList) {
        if (key === 'trackedTabId') result[key] = 42
        else result[key] = undefined
      }
      callback?.(result)
      return Promise.resolve(result)
    })
    chrome.storage.local.set = mock.fn((_data, callback) => {
      callback?.()
      return Promise.resolve()
    })
    chrome.tabs.get = mock.fn((tabId) => {
      if (tabId === 42) {
        return Promise.resolve({ id: 42, url: 'https://tracked.example/', groupId: -1, windowId: 7, active: false })
      }
      return Promise.resolve({ id: 1, url: 'https://other.example/', groupId: -1, windowId: 1, active: true })
    })
    chrome.tabs.group = mock.fn(() => Promise.resolve(77))
    chrome.tabs.update = mock.fn(() => Promise.resolve())
    chrome.windows = { update: mock.fn(() => Promise.resolve()) }
    chrome.tabGroups = {
      TAB_GROUP_ID_NONE: -1,
      Color: { ORANGE: 'orange' },
      update: mock.fn(() => Promise.resolve())
    }
    // Permission withheld: grouping must be skipped, never prompted.
    chrome.permissions = { contains: mock.fn(() => Promise.resolve(false)) }

    const open = mock.fn(() => Promise.resolve())
    const setOptions = mock.fn(() => Promise.resolve())
    chrome.sidePanel = { open, setOptions }

    const { handler } = getInstalledHandler()
    const result = handler({ type: 'open_terminal_panel' }, contentScriptSender, mock.fn())

    assert.strictEqual(result, true)
    await new Promise((resolve) => setTimeout(resolve, 0))

    assert.strictEqual(chrome.tabs.group.mock.calls.length, 0, 'no tab group may be created without the tabGroups grant')
    assert.strictEqual(chrome.tabGroups.update.mock.calls.length, 0, 'the workspace group must not be styled when grouping is skipped')
    // The panel itself still opens — grouping is cosmetic and independent.
    assert.strictEqual(open.mock.calls.length, 1, 'the terminal panel must still open without the grouping permission')
  })

  test('open_terminal_panel keeps the current tab when it already belongs to the Kaboom workspace group', async () => {
    chrome.storage.local.get = mock.fn((keys, callback) => {
      const keyList = Array.isArray(keys) ? keys : [keys]
      const result = {}
      for (const key of keyList) {
        if (key === 'trackedTabId') result[key] = 42
        else if (key === 'kaboom_terminal_workspace_group_id') result[key] = 77
        else if (key === 'kaboom_terminal_workspace_main_tab_id') result[key] = 42
        else result[key] = undefined
      }
      callback?.(result)
      return Promise.resolve(result)
    })

    chrome.tabs.get = mock.fn((tabId) => {
      if (tabId === 42) {
        return Promise.resolve({ id: 42, url: 'https://tracked.example/', groupId: 77, windowId: 7, active: false })
      }
      return Promise.resolve({ id: 1, url: 'https://secondary.example/', groupId: 77, windowId: 7, active: true })
    })
    chrome.tabs.group = mock.fn(() => Promise.resolve(77))
    chrome.tabs.update = mock.fn(() => Promise.resolve())
    chrome.windows = { update: mock.fn(() => Promise.resolve()) }
    chrome.tabGroups = {
      TAB_GROUP_ID_NONE: -1,
      Color: { ORANGE: 'orange' },
      update: mock.fn(() => Promise.resolve())
    }

    const open = mock.fn(() => Promise.resolve())
    const setOptions = mock.fn(() => Promise.resolve())
    chrome.sidePanel = { open, setOptions }

    const { handler } = getInstalledHandler()
    const sendResponse = mock.fn()

    const result = handler({ type: 'open_terminal_panel' }, contentScriptSender, sendResponse)

    assert.strictEqual(result, true)
    await new Promise((resolve) => setTimeout(resolve, 0))

    assert.strictEqual(chrome.tabs.group.mock.calls.length, 0, 'existing workspace group should be reused')
    assert.strictEqual(chrome.tabs.update.mock.calls.length, 0, 'active workspace tab should stay active')
    assert.strictEqual(open.mock.calls.length, 1)
    assert.strictEqual(open.mock.calls[0].arguments[0].tabId, 1)
    // The sender's tab is already the workspace host, so it is enabled once and
    // the refinement pass has nothing left to do.
    assert.deepStrictEqual(
      setOptions.mock.calls.map((c) => c.arguments[0]),
      [{ tabId: 1, path: 'sidepanel.html', enabled: true }]
    )
  })

  test('qa_scan_requested injects the Phase 1 audit prompt into the terminal server', async () => {
    globalThis.fetch = mock.fn(() =>
      Promise.resolve({
        ok: true,
        json: () => Promise.resolve({ injected: true })
      })
    )

    const { handler } = getInstalledHandler({
      getServerUrl: mock.fn(() => 'http://localhost:7890')
    })
    const sendResponse = mock.fn()

    const result = handler({ type: 'qa_scan_requested', page_url: 'https://tracked.example/' }, contentScriptSender, sendResponse)

    assert.strictEqual(result, true)
    await new Promise((resolve) => setTimeout(resolve, 10))

    assert.strictEqual(globalThis.fetch.mock.calls.length, 1)
    const [url, options] = globalThis.fetch.mock.calls[0].arguments
    assert.match(url, /\/terminal\/inject$/)

    const payload = JSON.parse(options.body)
    assert.match(payload.text, /audit/i)
    assert.match(payload.text, /\/kaboom\/audit|\/audit|audit workflow/i)
    assert.doesNotMatch(payload.text, /Find Problems.*QA skill or start with: analyze\(what:"page_issues"/)

    assert.deepStrictEqual(sendResponse.mock.calls[0].arguments[0], {
      success: true,
      method: 'terminal_inject'
    })
  })

  test('setDebugMode updates and persists', () => {
    const { handler, deps } = getInstalledHandler()
    const sendResponse = mock.fn()
    handler({ type: 'set_debug_mode', enabled: true }, extensionSender, sendResponse)
    assert.strictEqual(deps.setDebugMode.mock.calls[0].arguments[0], true)
    assert.ok(deps.saveSetting.mock.calls.some(c => c.arguments[0] === 'debugMode'))
    assert.strictEqual(sendResponse.mock.calls[0].arguments[0].success, true)
  })

  test('getDebugLog returns log array', () => {
    const { handler } = getInstalledHandler({
      exportDebugLog: mock.fn(() => ['line1', 'line2'])
    })
    const sendResponse = mock.fn()
    handler({ type: 'get_debug_log' }, extensionSender, sendResponse)
    assert.deepStrictEqual(sendResponse.mock.calls[0].arguments[0].log, ['line1', 'line2'])
  })

  test('clearDebugLog clears and responds', () => {
    const { handler, deps } = getInstalledHandler()
    const sendResponse = mock.fn()
    handler({ type: 'clear_debug_log' }, extensionSender, sendResponse)
    assert.strictEqual(deps.clearDebugLog.mock.calls.length, 1)
    assert.strictEqual(sendResponse.mock.calls[0].arguments[0].success, true)
  })
})

// ============================================
// State snapshots
// ============================================

describe('state snapshots', () => {
  beforeEach(() => {
    // Reset storage mock to return empty snapshots
    chrome.storage.local.get = mock.fn((_keys, callback) => {
      const result = { kaboom_state_snapshots: {} }
      callback?.(result)
      return Promise.resolve(result)
    })
    chrome.storage.local.set = mock.fn((_data, callback) => {
      callback?.()
      return Promise.resolve()
    })
  })

  test('save and load roundtrip', async () => {
    const stored = {}
    chrome.storage.local.get = mock.fn((_keys, callback) => {
      const result = { kaboom_state_snapshots: stored }
      callback?.(result)
      return Promise.resolve(result)
    })
    chrome.storage.local.set = mock.fn((data, callback) => {
      Object.assign(stored, data.kaboom_state_snapshots)
      callback?.()
      return Promise.resolve()
    })

    const state = { url: 'https://example.com', timestamp: '2024-01-01T00:00:00Z' }
    const result = await saveStateSnapshot('test-snap', state)
    assert.strictEqual(result.success, true)
    assert.strictEqual(result.snapshot_name, 'test-snap')

    chrome.storage.local.get = mock.fn((_keys, callback) => {
      const result = { kaboom_state_snapshots: stored }
      callback?.(result)
      return Promise.resolve(result)
    })

    const loaded = await loadStateSnapshot('test-snap')
    assert.strictEqual(loaded.url, 'https://example.com')
    assert.strictEqual(loaded.name, 'test-snap')
  })

  test('load nonexistent snapshot returns null', async () => {
    const loaded = await loadStateSnapshot('nonexistent')
    assert.strictEqual(loaded, null)
  })

  test('list returns array of snapshot metadata', async () => {
    chrome.storage.local.get = mock.fn((_keys, callback) => {
      const result = {
        kaboom_state_snapshots: {
          snap1: { name: 'snap1', url: 'https://a.com', timestamp: '2024-01-01T00:00:00Z', size_bytes: 100 },
          snap2: { name: 'snap2', url: 'https://b.com', timestamp: '2024-01-02T00:00:00Z', size_bytes: 200 }
        }
      }
      callback?.(result)
      return Promise.resolve(result)
    })

    const list = await listStateSnapshots()
    assert.strictEqual(list.length, 2)
    assert.ok(list.some(s => s.name === 'snap1'))
    assert.ok(list.some(s => s.name === 'snap2'))
  })

  test('delete removes snapshot', async () => {
    const stored = {
      snap1: { name: 'snap1', url: 'https://a.com' }
    }
    chrome.storage.local.get = mock.fn((_keys, callback) => {
      const result = { kaboom_state_snapshots: { ...stored } }
      callback?.(result)
      return Promise.resolve(result)
    })
    chrome.storage.local.set = mock.fn((data, callback) => {
      Object.assign(stored, data.kaboom_state_snapshots)
      callback?.()
      return Promise.resolve()
    })

    const result = await deleteStateSnapshot('snap1')
    assert.strictEqual(result.success, true)
    assert.strictEqual(result.deleted, 'snap1')
  })
})
