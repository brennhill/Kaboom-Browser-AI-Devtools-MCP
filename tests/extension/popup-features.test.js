// @ts-nocheck
/**
 * @fileoverview popup-features.test.js — Tests for popup feature controls:
 * WebSocket toggle, debug logging, and health indicators (circuit breaker,
 * memory pressure, section visibility).
 */

import { test, describe, mock, beforeEach } from 'node:test'
import assert from 'node:assert'
import {
  applyFeatureTogglesFromMockStorage,
  applyWebSocketModeFromMockStorage
} from './helpers.js'

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

test('popup exposes only batched-read apply functions for feature settings', async () => {
  const popup = await import('../../extension/popup.js')
  assert.strictEqual(popup.initFeatureToggles, undefined)
  assert.strictEqual(popup.initWebSocketModeSelector, undefined)
  assert.strictEqual(typeof popup.applyFeatureToggles, 'function')
  assert.strictEqual(typeof popup.applyWebSocketMode, 'function')
})

describe('WebSocket Toggle', () => {
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
    mockChrome.storage.local.set.mock.mockImplementation((data, callback) => callback && callback())
  })

  test('should load saved WebSocket capture state on init', async () => {
    mockChrome.storage.local.get.mock.mockImplementation((keys, callback) => {
      callback({ webSocketCaptureEnabled: true, webSocketCaptureMode: 'high' })
    })

    await applyFeatureTogglesFromMockStorage()

    const wsToggle = mockDocument.getElementById('toggle-websocket')
    assert.strictEqual(wsToggle.checked, true)
  })

  test('should default WebSocket capture to ON', async () => {
    mockChrome.storage.local.get.mock.mockImplementation((keys, callback) => {
      callback({}) // No saved value — defaults to ON
    })

    await applyFeatureTogglesFromMockStorage()

    const wsToggle = mockDocument.getElementById('toggle-websocket')
    assert.strictEqual(wsToggle.checked, true)
  })

  test('should send message to background when WebSocket toggled', async () => {
    const { handleFeatureToggle } = await import('../../extension/popup.js')

    handleFeatureToggle('webSocketCaptureEnabled', 'set_web_socket_capture_enabled', true)

    assert.ok(
      mockChrome.runtime.sendMessage.mock.calls.some(
        (c) => c.arguments[0].type === 'set_web_socket_capture_enabled' && c.arguments[0].enabled === true
      )
    )
  })

  test('should send mode change message to background', async () => {
    const { handleWebSocketModeChange } = await import('../../extension/popup.js')

    handleWebSocketModeChange('high')

    assert.ok(
      mockChrome.runtime.sendMessage.mock.calls.some(
        (c) => c.arguments[0].type === 'set_web_socket_capture_mode' && c.arguments[0].mode === 'high'
      )
    )
  })

  test('should default mode to medium', async () => {
    mockChrome.storage.local.get.mock.mockImplementation((keys, callback) => {
      callback({}) // No saved value
    })

    await applyWebSocketModeFromMockStorage()

    const modeSelect = mockDocument.getElementById('ws-mode')
    assert.strictEqual(modeSelect.value, 'medium')
  })

  test('should load saved mode on init', async () => {
    mockChrome.storage.local.get.mock.mockImplementation((keys, callback) => {
      callback({ webSocketCaptureMode: 'high' })
    })

    await applyWebSocketModeFromMockStorage()

    const modeSelect = mockDocument.getElementById('ws-mode')
    assert.strictEqual(modeSelect.value, 'high')
  })
})

describe('Debug Logging', () => {
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
    mockChrome.storage.local.set.mock.mockImplementation((data, callback) => callback && callback())
  })

  test('should toggle debug mode and send message to background', async () => {
    const { handleFeatureToggle } = await import('../../extension/popup.js')

    handleFeatureToggle('debugMode', 'set_debug_mode', true)

    // Should have sent message to background (popup does not save to storage)
    assert.ok(
      mockChrome.runtime.sendMessage.mock.calls.some(
        (c) => c.arguments[0].type === 'set_debug_mode' && c.arguments[0].enabled === true
      )
    )
  })
})

describe('Health Indicators', () => {
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

  describe('Circuit Breaker Status', () => {
    test('should hide circuit breaker indicator when state is "closed"', async () => {
      const { updateConnectionStatus } = await import('../../extension/popup.js')

      updateConnectionStatus({
        connected: true,
        entries: 5,
        circuitBreakerState: 'closed',
        memoryPressure: { memoryPressureLevel: 'normal' }
      })

      const cbEl = mockDocument.getElementById('health-circuit-breaker')
      assert.strictEqual(cbEl.style.display, 'none')
    })

    test('should display circuit breaker "open" with error styling', async () => {
      const { updateConnectionStatus } = await import('../../extension/popup.js')

      updateConnectionStatus({
        connected: true,
        entries: 5,
        circuitBreakerState: 'open',
        memoryPressure: { memoryPressureLevel: 'normal' }
      })

      const cbEl = mockDocument.getElementById('health-circuit-breaker')
      assert.notStrictEqual(cbEl.style.display, 'none')
      assert.ok(cbEl.classList.add.mock.calls.some((c) => c.arguments[0] === 'health-error'))
      assert.ok(cbEl.textContent.includes('paused'))
    })

    test('should display circuit breaker "half-open" with warning styling', async () => {
      const { updateConnectionStatus } = await import('../../extension/popup.js')

      updateConnectionStatus({
        connected: true,
        entries: 5,
        circuitBreakerState: 'half-open',
        memoryPressure: { memoryPressureLevel: 'normal' }
      })

      const cbEl = mockDocument.getElementById('health-circuit-breaker')
      assert.notStrictEqual(cbEl.style.display, 'none')
      assert.ok(cbEl.classList.add.mock.calls.some((c) => c.arguments[0] === 'health-warning'))
      assert.ok(cbEl.textContent.includes('recovering'))
    })
  })

  describe('Memory Pressure Status', () => {
    test('should hide memory pressure indicator when level is "normal"', async () => {
      const { updateConnectionStatus } = await import('../../extension/popup.js')

      updateConnectionStatus({
        connected: true,
        entries: 5,
        circuitBreakerState: 'closed',
        memoryPressure: { memoryPressureLevel: 'normal' }
      })

      const mpEl = mockDocument.getElementById('health-memory-pressure')
      assert.strictEqual(mpEl.style.display, 'none')
    })

    test('should display memory pressure "soft" with warning styling', async () => {
      const { updateConnectionStatus } = await import('../../extension/popup.js')

      updateConnectionStatus({
        connected: true,
        entries: 5,
        circuitBreakerState: 'closed',
        memoryPressure: { memoryPressureLevel: 'soft', reducedCapacities: true }
      })

      const mpEl = mockDocument.getElementById('health-memory-pressure')
      assert.notStrictEqual(mpEl.style.display, 'none')
      assert.ok(mpEl.classList.add.mock.calls.some((c) => c.arguments[0] === 'health-warning'))
      assert.ok(mpEl.textContent.includes('elevated'))
    })

    test('should display memory pressure "hard" with error styling', async () => {
      const { updateConnectionStatus } = await import('../../extension/popup.js')

      updateConnectionStatus({
        connected: true,
        entries: 5,
        circuitBreakerState: 'closed',
        memoryPressure: { memoryPressureLevel: 'hard', networkBodyCaptureDisabled: true }
      })

      const mpEl = mockDocument.getElementById('health-memory-pressure')
      assert.notStrictEqual(mpEl.style.display, 'none')
      assert.ok(mpEl.classList.add.mock.calls.some((c) => c.arguments[0] === 'health-error'))
      assert.ok(mpEl.textContent.includes('critical'))
    })
  })

  describe('Section Visibility', () => {
    test('should hide health section when all indicators are healthy', async () => {
      const { updateConnectionStatus } = await import('../../extension/popup.js')

      updateConnectionStatus({
        connected: true,
        entries: 5,
        circuitBreakerState: 'closed',
        memoryPressure: { memoryPressureLevel: 'normal' }
      })

      const sectionEl = mockDocument.getElementById('health-indicators')
      assert.strictEqual(sectionEl.style.display, 'none')
    })

    test('should show health section when circuit breaker is unhealthy', async () => {
      const { updateConnectionStatus } = await import('../../extension/popup.js')

      updateConnectionStatus({
        connected: true,
        entries: 5,
        circuitBreakerState: 'open',
        memoryPressure: { memoryPressureLevel: 'normal' }
      })

      const sectionEl = mockDocument.getElementById('health-indicators')
      assert.notStrictEqual(sectionEl.style.display, 'none')
    })

    test('should show health section when memory pressure is elevated', async () => {
      const { updateConnectionStatus } = await import('../../extension/popup.js')

      updateConnectionStatus({
        connected: true,
        entries: 5,
        circuitBreakerState: 'closed',
        memoryPressure: { memoryPressureLevel: 'soft' }
      })

      const sectionEl = mockDocument.getElementById('health-indicators')
      assert.notStrictEqual(sectionEl.style.display, 'none')
    })

    test('should hide health indicators when disconnected', async () => {
      const { updateConnectionStatus } = await import('../../extension/popup.js')

      updateConnectionStatus({
        connected: false,
        error: 'Connection refused',
        circuitBreakerState: 'open',
        memoryPressure: { memoryPressureLevel: 'hard' }
      })

      const sectionEl = mockDocument.getElementById('health-indicators')
      assert.strictEqual(sectionEl.style.display, 'none')
    })

    test('should handle missing health data gracefully', async () => {
      const { updateConnectionStatus } = await import('../../extension/popup.js')

      // No circuitBreakerState or memoryPressure in status
      assert.doesNotThrow(() => {
        updateConnectionStatus({
          connected: true,
          entries: 10
        })
      })
    })
  })
})
