// @ts-nocheck
/**
 * @fileoverview popup-status.test.js — Tests for the extension popup UI: connection
 * status display, clear logs button, status updates, context annotation warnings,
 * quick actions, and server URL display.
 */

import { test, describe, mock, beforeEach } from 'node:test'
import assert from 'node:assert'

// Mock Chrome APIs
const mockChrome = {
  runtime: {
    sendMessage: mock.fn(() => Promise.resolve()),
    onMessage: {
      addListener: mock.fn()
    }
  },
  storage: {
    local: {
      get: mock.fn((keys, callback) => callback({})),
      set: mock.fn((data, callback) => callback && callback()),
      remove: mock.fn((keys, callback) => callback && callback())
    },
    sync: {
      get: mock.fn((keys, callback) => callback({})),
      set: mock.fn((data, callback) => callback && callback())
    },
    onChanged: {
      addListener: mock.fn()
    }
  },
  tabs: {
    query: mock.fn((queryInfo, callback) => callback([{ id: 1, url: 'http://localhost:3000' }]))
  }
}

globalThis.chrome = mockChrome

// Mock DOM elements
const createMockDocument = () => {
  const elements = {}

  return {
    getElementById: mock.fn((id) => {
      if (!elements[id]) {
        elements[id] = createMockElement(id)
      }
      return elements[id]
    }),
    querySelector: mock.fn(),
    querySelectorAll: mock.fn(() => []),
    addEventListener: mock.fn()
  }
}

const createMockElement = (id) => ({
  id,
  textContent: '',
  innerHTML: '',
  className: '',
  classList: {
    add: mock.fn(),
    remove: mock.fn(),
    toggle: mock.fn()
  },
  style: {},
  addEventListener: mock.fn(),
  setAttribute: mock.fn(),
  getAttribute: mock.fn(),
  value: '',
  checked: false,
  disabled: false
})

let mockDocument

describe('Popup State Display', () => {
  beforeEach(() => {
    mock.reset()
    mockDocument = createMockDocument()
    globalThis.document = mockDocument
    // Restore default mock implementations after reset
    mockChrome.runtime.sendMessage.mock.mockImplementation(() => Promise.resolve())
    mockChrome.storage.local.get.mock.mockImplementation((keys, callback) => callback({}))
    mockChrome.tabs.query.mock.mockImplementation((queryInfo, callback) =>
      callback([{ id: 1, url: 'http://localhost:3000' }])
    )
  })

  test('should display connected status when server is up', async () => {
    const { updateConnectionStatus } = await import('../../../extension/popup.js')

    updateConnectionStatus({
      connected: true,
      entries: 42,
      maxEntries: 1000
    })

    const statusEl = mockDocument.getElementById('status')
    const entriesEl = mockDocument.getElementById('entries-count')

    assert.ok(statusEl.classList.add.mock.calls.some((c) => c.arguments[0] === 'connected'))
    assert.ok(statusEl.textContent.toLowerCase().includes('connected'))
    assert.strictEqual(entriesEl.textContent, '42 / 1000')
  })

  test('should display offline status when server is down', async () => {
    const { updateConnectionStatus } = await import('../../../extension/popup.js')

    updateConnectionStatus({
      connected: false,
      error: 'Connection refused'
    })

    const statusEl = mockDocument.getElementById('status')

    assert.ok(statusEl.classList.add.mock.calls.some((c) => c.arguments[0] === 'disconnected'))
    assert.ok(statusEl.textContent.toLowerCase().includes('offline'))
  })

  test('should show error message when disconnected', async () => {
    const { updateConnectionStatus } = await import('../../../extension/popup.js')

    updateConnectionStatus({
      connected: false,
      error: 'Connection refused'
    })

    const errorEl = mockDocument.getElementById('error-message')
    assert.ok(errorEl.textContent.includes('Connection refused'))
  })

  test('should render offline when daemon is reachable but heartbeat is missing', async () => {
    const { updateConnectionStatus } = await import('../../../extension/popup.js')

    updateConnectionStatus({
      connected: false,
      error: 'Server reachable, but extension heartbeat is missing. Open the Kaboom popup and click "Track This Tab".'
    })

    const statusEl = mockDocument.getElementById('status')
    const errorEl = mockDocument.getElementById('error-message')

    assert.ok(statusEl.textContent.toLowerCase().includes('offline'))
    assert.ok(errorEl.textContent.includes('heartbeat'))
  })

  test('should show insecure debug warning when security mode is insecure_proxy', async () => {
    const { updateConnectionStatus } = await import('../../../extension/popup.js')

    updateConnectionStatus({
      connected: true,
      securityMode: 'insecure_proxy',
      productionParity: false,
      insecureRewritesApplied: ['csp_headers']
    })

    const warningEl = mockDocument.getElementById('security-mode-warning')
    const detailEl = mockDocument.getElementById('security-mode-detail')
    assert.strictEqual(warningEl.style.display, 'block')
    assert.ok(detailEl.textContent.includes('INSECURE DEBUG MODE'))
    assert.ok(detailEl.textContent.includes('production_parity=false'))
  })

  test('should request status on popup open', async () => {
    const { initPopup } = await import('../../../extension/popup.js')

    await initPopup()

    // Should have sent getStatus message
    assert.ok(mockChrome.runtime.sendMessage.mock.calls.some((c) => c.arguments[0]?.type === 'get_status'))
  })
})

// Log Level Selector tests removed — log level dropdown was removed from popup UI.
// Log level is now hardcoded to 'all' in background/init.ts.

describe('Clear Logs Button', () => {
  beforeEach(async () => {
    mock.reset()
    mockDocument = createMockDocument()
    globalThis.document = mockDocument
    // Default mock that calls callback and returns Promise
    mockChrome.runtime.sendMessage.mock.mockImplementation((msg, callback) => {
      if (callback) callback({ success: true })
      return Promise.resolve()
    })
    mockChrome.storage.local.get.mock.mockImplementation((keys, callback) => callback({}))
    mockChrome.tabs.query.mock.mockImplementation((queryInfo, callback) =>
      callback([{ id: 1, url: 'http://localhost:3000' }])
    )
    // Reset confirmation state between tests
    const { resetClearConfirm } = await import('../../../extension/popup.js')
    resetClearConfirm()
  })

  test('should require confirmation before clearing', async () => {
    const { handleClearLogs } = await import('../../../extension/popup.js')

    // First click: shows confirmation prompt
    await handleClearLogs()

    const clearBtn = mockDocument.getElementById('clear-btn')
    assert.strictEqual(clearBtn.textContent, 'Confirm Clear?')

    // No clearLogs message sent yet
    assert.ok(!mockChrome.runtime.sendMessage.mock.calls.some((c) => c.arguments[0]?.type === 'clear_logs'))
  })

  test('should send clearLogs message on second click', async () => {
    const { handleClearLogs } = await import('../../../extension/popup.js')

    // First click: confirmation
    await handleClearLogs()
    // Second click: actually clears
    await handleClearLogs()

    assert.ok(mockChrome.runtime.sendMessage.mock.calls.some((c) => c.arguments[0]?.type === 'clear_logs'))
  })

  test('should update UI after clearing logs', async () => {
    mockChrome.runtime.sendMessage.mock.mockImplementation((msg, callback) => {
      if (msg.type === 'clear_logs') {
        callback({ success: true })
      }
    })

    const { handleClearLogs } = await import('../../../extension/popup.js')

    // First click: confirmation, second click: clear
    await handleClearLogs()
    await handleClearLogs()

    const entriesEl = mockDocument.getElementById('entries-count')
    assert.strictEqual(entriesEl.textContent, '0 / 1000')
  })

  test('should disable button while clearing', async () => {
    let resolvePromise
    mockChrome.runtime.sendMessage.mock.mockImplementation((msg, callback) => {
      if (msg.type === 'clear_logs') {
        return new Promise((resolve) => {
          resolvePromise = () => {
            callback({ success: true })
            resolve()
          }
        })
      }
    })

    const { handleClearLogs } = await import('../../../extension/popup.js')

    const clearBtn = mockDocument.getElementById('clear-btn')

    // First click: confirmation
    await handleClearLogs()
    // Second click: clear
    const promise = handleClearLogs()

    // Button should be disabled during operation
    assert.strictEqual(clearBtn.disabled, true)

    resolvePromise()
    await promise

    // Button should be re-enabled after
    assert.strictEqual(clearBtn.disabled, false)
  })

  test('should show error if clear fails', async () => {
    mockChrome.runtime.sendMessage.mock.mockImplementation((msg, callback) => {
      if (msg.type === 'clear_logs') {
        callback({ success: false, error: 'Server not responding' })
      }
    })

    const { handleClearLogs } = await import('../../../extension/popup.js')

    // First click: confirmation, second click: clear
    await handleClearLogs()
    await handleClearLogs()

    const errorEl = mockDocument.getElementById('error-message')
    assert.ok(errorEl.textContent.includes('Server not responding'))
  })
})

describe('Status Updates', () => {
  beforeEach(() => {
    mock.reset()
    mockDocument = createMockDocument()
    globalThis.document = mockDocument
    // Restore default mock implementations after reset
    mockChrome.runtime.sendMessage.mock.mockImplementation(() => Promise.resolve())
    mockChrome.storage.local.get.mock.mockImplementation((keys, callback) => callback({}))
    mockChrome.tabs.query.mock.mockImplementation((queryInfo, callback) =>
      callback([{ id: 1, url: 'http://localhost:3000' }])
    )
  })

  test('should listen for status updates from background', async () => {
    const { initPopup } = await import('../../../extension/popup.js')

    await initPopup()

    // Should have registered message listener
    assert.ok(mockChrome.runtime.onMessage.addListener.mock.calls.length > 0)
  })

  test('should update display when status message received', async () => {
    let messageHandler

    mockChrome.runtime.onMessage.addListener.mock.mockImplementation((handler) => {
      messageHandler = handler
    })

    const { initPopup } = await import('../../../extension/popup.js')

    await initPopup()

    // Simulate status update from background
    messageHandler({ type: 'status_update', status: { connected: true, entries: 100 } })

    const statusEl = mockDocument.getElementById('status')
    assert.ok(statusEl.classList.add.mock.calls.some((c) => c.arguments[0] === 'connected'))
  })

  test('should update error count badge display', async () => {
    let messageHandler

    mockChrome.runtime.onMessage.addListener.mock.mockImplementation((handler) => {
      messageHandler = handler
    })

    const { initPopup } = await import('../../../extension/popup.js')

    await initPopup()

    messageHandler({
      type: 'status_update',
      status: {
        connected: true,
        errorCount: 5
      }
    })

    const errorCountEl = mockDocument.getElementById('error-count')
    assert.strictEqual(errorCountEl.textContent, '5')
  })
})

describe('Context Annotation Warning', () => {
  beforeEach(() => {
    mock.reset()
    mockDocument = createMockDocument()
    globalThis.document = mockDocument
    // Restore default mock implementations after reset
    mockChrome.runtime.sendMessage.mock.mockImplementation(() => Promise.resolve())
    mockChrome.storage.local.get.mock.mockImplementation((keys, callback) => callback({}))
    mockChrome.tabs.query.mock.mockImplementation((queryInfo, callback) =>
      callback([{ id: 1, url: 'http://localhost:3000' }])
    )
  })

  test('should show context warning when status has contextWarning', async () => {
    const { updateConnectionStatus } = await import('../../../extension/popup.js')

    updateConnectionStatus({
      connected: true,
      entries: 10,
      contextWarning: {
        sizeKB: 25,
        count: 4,
        triggeredAt: Date.now()
      }
    })

    const warningEl = mockDocument.getElementById('context-warning')
    assert.strictEqual(warningEl.style.display, 'block')
  })

  test('should populate warning text with size and count info', async () => {
    const { updateConnectionStatus } = await import('../../../extension/popup.js')

    updateConnectionStatus({
      connected: true,
      entries: 10,
      contextWarning: {
        sizeKB: 30,
        count: 5,
        triggeredAt: Date.now()
      }
    })

    const warningTextEl = mockDocument.getElementById('context-warning-text')
    assert.ok(warningTextEl.textContent.includes('30'))
    assert.ok(warningTextEl.textContent.includes('5'))
  })

  test('should hide context warning when contextWarning is null', async () => {
    const { updateConnectionStatus } = await import('../../../extension/popup.js')

    updateConnectionStatus({
      connected: true,
      entries: 10,
      contextWarning: null
    })

    const warningEl = mockDocument.getElementById('context-warning')
    assert.strictEqual(warningEl.style.display, 'none')
  })

  test('should hide context warning when contextWarning is undefined', async () => {
    const { updateConnectionStatus } = await import('../../../extension/popup.js')

    updateConnectionStatus({
      connected: true,
      entries: 10
    })

    const warningEl = mockDocument.getElementById('context-warning')
    assert.strictEqual(warningEl.style.display, 'none')
  })

  test('should hide context warning when disconnected even if warning exists', async () => {
    const { updateConnectionStatus } = await import('../../../extension/popup.js')

    updateConnectionStatus({
      connected: false,
      error: 'Connection refused',
      contextWarning: {
        sizeKB: 25,
        count: 3,
        triggeredAt: Date.now()
      }
    })

    const warningEl = mockDocument.getElementById('context-warning')
    assert.strictEqual(warningEl.style.display, 'none')
  })
})

describe('Quick Actions', () => {
  beforeEach(() => {
    mock.reset()
    mockDocument = createMockDocument()
    globalThis.document = mockDocument
    // Restore default mock implementations after reset
    mockChrome.runtime.sendMessage.mock.mockImplementation(() => Promise.resolve())
    mockChrome.storage.local.get.mock.mockImplementation((keys, callback) => callback({}))
    mockChrome.tabs.query.mock.mockImplementation((queryInfo, callback) =>
      callback([{ id: 1, url: 'http://localhost:3000' }])
    )
  })

  test('should have link to open log file', async () => {
    const { initPopup } = await import('../../../extension/popup.js')

    await initPopup()

    const openLogLink = mockDocument.getElementById('open-log-file')
    assert.ok(openLogLink)
  })

  test('should have link to options page', async () => {
    const { initPopup } = await import('../../../extension/popup.js')

    await initPopup()

    const optionsLink = mockDocument.getElementById('options-link')
    assert.ok(optionsLink)
  })
})

describe('Server URL Display', () => {
  beforeEach(() => {
    mock.reset()
    mockDocument = createMockDocument()
    globalThis.document = mockDocument
    // Restore default mock implementations after reset
    mockChrome.runtime.sendMessage.mock.mockImplementation(() => Promise.resolve())
    mockChrome.storage.local.get.mock.mockImplementation((keys, callback) => callback({}))
    mockChrome.tabs.query.mock.mockImplementation((queryInfo, callback) =>
      callback([{ id: 1, url: 'http://localhost:3000' }])
    )
  })

  test('should display server URL', async () => {
    const { updateConnectionStatus } = await import('../../../extension/popup.js')

    updateConnectionStatus({
      connected: true,
      serverUrl: 'http://localhost:7890'
    })

    const serverUrlEl = mockDocument.getElementById('server-url')
    assert.ok(serverUrlEl.textContent.includes('localhost:7890'))
  })

  test('should display log file path when connected', async () => {
    const { updateConnectionStatus } = await import('../../../extension/popup.js')

    updateConnectionStatus({
      connected: true,
      logFile: '/Users/dev/browser-agent-logs.jsonl'
    })

    const logFileEl = mockDocument.getElementById('log-file-path')
    assert.ok(logFileEl.textContent.includes('browser-agent-logs.jsonl'))
  })
})
