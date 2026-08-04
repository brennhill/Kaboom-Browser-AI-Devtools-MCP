// @ts-nocheck
/**
 * @fileoverview options.test.js — Tests for the extension options/settings page.
 * Covers server URL persistence, domain filter management, toggle states
 * (screenshot, source maps, deferral), and chrome.storage.local integration.
 */

import { test, describe, mock, beforeEach } from 'node:test'
import assert from 'node:assert'

// Mock Chrome APIs
const mockChrome = {
  runtime: {
    sendMessage: mock.fn(),
    onMessage: { addListener: mock.fn() }
  },
  storage: {
    local: {
      get: mock.fn((keys, cb) => cb({})),
      set: mock.fn((data, cb) => cb && cb())
    }
  }
}

globalThis.chrome = mockChrome

// Mock DOM
function createMockDocument() {
  const elements = {}

  return {
    getElementById: (id) => {
      if (!elements[id]) {
        elements[id] = {
          id,
          value: '',
          textContent: '',
          classList: {
            _classes: new Set(),
            add(c) {
              this._classes.add(c)
            },
            remove(c) {
              this._classes.delete(c)
            },
            contains(c) {
              return this._classes.has(c)
            },
            toggle(c) {
              if (this._classes.has(c)) {
                this._classes.delete(c)
              } else {
                this._classes.add(c)
              }
            }
          },
          style: {},
          addEventListener: mock.fn()
        }
      }
      return elements[id]
    },
    addEventListener: mock.fn(),
    querySelector: mock.fn(() => null),
    querySelectorAll: mock.fn(() => []),
    readyState: 'complete',
    _elements: elements
  }
}

globalThis.document = createMockDocument()

const { loadOptions, saveOptions, toggleDeferral, toggleDebugMode } = await import('../../../extension/options.js')

describe('Options Deferral Toggle', () => {
  beforeEach(() => {
    globalThis.document = createMockDocument()
    mockChrome.runtime.sendMessage = mock.fn()
    mockChrome.storage.local.get = mock.fn((keys, cb) => cb({}))
    mockChrome.storage.local.set = mock.fn((data, cb) => cb && cb())
  })

  test('should load deferral toggle state from storage (default: true/active)', async () => {
    mockChrome.storage.local.get = mock.fn((keys, cb) => cb({}))

    await loadOptions()

    const toggle = document.getElementById('deferral-toggle')
    assert.ok(toggle.classList.contains('active'))
  })

  test('should load saved deferral state from storage (disabled)', async () => {
    mockChrome.storage.local.get = mock.fn((keys, cb) => {
      cb({ deferralEnabled: false })
    })

    await loadOptions()

    const toggle = document.getElementById('deferral-toggle')
    assert.ok(!toggle.classList.contains('active'))
  })

  test('should load saved deferral state from storage (enabled)', async () => {
    mockChrome.storage.local.get = mock.fn((keys, cb) => {
      cb({ deferralEnabled: true })
    })

    await loadOptions()

    const toggle = document.getElementById('deferral-toggle')
    assert.ok(toggle.classList.contains('active'))
  })

  test('should toggle deferral state on click', async () => {
    // Start with active state
    mockChrome.storage.local.get = mock.fn((keys, cb) => cb({ deferralEnabled: true }))
    await loadOptions()

    const toggle = document.getElementById('deferral-toggle')
    assert.ok(toggle.classList.contains('active'))

    // Toggle (simulates click handler)
    toggleDeferral()
    assert.ok(!toggle.classList.contains('active'))

    // Toggle again
    toggleDeferral()
    assert.ok(toggle.classList.contains('active'))
  })

  test('should include deferralEnabled in save', async () => {
    mockChrome.storage.local.get = mock.fn((keys, cb) => cb({}))
    await loadOptions()

    // Toggle is active by default
    await saveOptions()

    assert.ok(mockChrome.storage.local.set.mock.calls.some((c) => c.arguments[0].deferralEnabled === true))
  })

  test('should save deferralEnabled=false when toggle is inactive', async () => {
    mockChrome.storage.local.get = mock.fn((keys, cb) => cb({ deferralEnabled: false }))
    await loadOptions()

    await saveOptions()

    assert.ok(mockChrome.storage.local.set.mock.calls.some((c) => c.arguments[0].deferralEnabled === false))
  })

  test('should send setDeferralEnabled message on save', async () => {
    mockChrome.storage.local.get = mock.fn((keys, cb) => cb({}))
    await loadOptions()

    await saveOptions()

    assert.ok(
      mockChrome.runtime.sendMessage.mock.calls.some(
        (c) => c.arguments[0].type === 'set_deferral_enabled' && c.arguments[0].enabled === true
      )
    )
  })

  test('should send setDeferralEnabled=false when disabled', async () => {
    mockChrome.storage.local.get = mock.fn((keys, cb) => cb({ deferralEnabled: false }))
    await loadOptions()

    await saveOptions()

    assert.ok(
      mockChrome.runtime.sendMessage.mock.calls.some(
        (c) => c.arguments[0].type === 'set_deferral_enabled' && c.arguments[0].enabled === false
      )
    )
  })
})

describe('Options Screenshot Toggle', () => {
  beforeEach(() => {
    globalThis.document = createMockDocument()
    mockChrome.runtime.sendMessage = mock.fn()
    mockChrome.storage.local.get = mock.fn((keys, cb) => cb({}))
    mockChrome.storage.local.set = mock.fn((data, cb) => cb && cb())
  })

  test('should not activate screenshot toggle when no saved value (default: off)', async () => {
    mockChrome.storage.local.get = mock.fn((keys, cb) => cb({}))

    await loadOptions()

    const toggle = document.getElementById('screenshot-toggle')
    assert.ok(!toggle.classList.contains('active'))
  })

  test('should activate screenshot toggle when saved as true', async () => {
    mockChrome.storage.local.get = mock.fn((keys, cb) => {
      cb({ screenshotOnError: true })
    })

    await loadOptions()

    const toggle = document.getElementById('screenshot-toggle')
    assert.ok(toggle.classList.contains('active'))
  })

  test('should not activate screenshot toggle when saved as false', async () => {
    mockChrome.storage.local.get = mock.fn((keys, cb) => {
      cb({ screenshotOnError: false })
    })

    await loadOptions()

    const toggle = document.getElementById('screenshot-toggle')
    assert.ok(!toggle.classList.contains('active'))
  })

  test('should include screenshotOnError=false in save when inactive', async () => {
    mockChrome.storage.local.get = mock.fn((keys, cb) => cb({}))
    await loadOptions()

    await saveOptions()

    assert.ok(mockChrome.storage.local.set.mock.calls.some((c) => c.arguments[0].screenshotOnError === false))
  })

  test('should include screenshotOnError=true in save when active', async () => {
    mockChrome.storage.local.get = mock.fn((keys, cb) => cb({ screenshotOnError: true }))
    await loadOptions()

    await saveOptions()

    assert.ok(mockChrome.storage.local.set.mock.calls.some((c) => c.arguments[0].screenshotOnError === true))
  })

  test('should send setScreenshotOnError message on save', async () => {
    mockChrome.storage.local.get = mock.fn((keys, cb) => cb({ screenshotOnError: true }))
    await loadOptions()

    await saveOptions()

    assert.ok(
      mockChrome.runtime.sendMessage.mock.calls.some(
        (c) => c.arguments[0].type === 'set_screenshot_on_error' && c.arguments[0].enabled === true
      )
    )
  })
})

describe('Options Source Map Toggle', () => {
  beforeEach(() => {
    globalThis.document = createMockDocument()
    mockChrome.runtime.sendMessage = mock.fn()
    mockChrome.storage.local.get = mock.fn((keys, cb) => cb({}))
    mockChrome.storage.local.set = mock.fn((data, cb) => cb && cb())
  })

  test('should not activate source map toggle when no saved value (default: off)', async () => {
    mockChrome.storage.local.get = mock.fn((keys, cb) => cb({}))

    await loadOptions()

    const toggle = document.getElementById('sourcemap-toggle')
    assert.ok(!toggle.classList.contains('active'))
  })

  test('should activate source map toggle when saved as true', async () => {
    mockChrome.storage.local.get = mock.fn((keys, cb) => {
      cb({ sourceMapEnabled: true })
    })

    await loadOptions()

    const toggle = document.getElementById('sourcemap-toggle')
    assert.ok(toggle.classList.contains('active'))
  })

  test('should not activate source map toggle when saved as false', async () => {
    mockChrome.storage.local.get = mock.fn((keys, cb) => {
      cb({ sourceMapEnabled: false })
    })

    await loadOptions()

    const toggle = document.getElementById('sourcemap-toggle')
    assert.ok(!toggle.classList.contains('active'))
  })

  test('should include sourceMapEnabled=false in save when inactive', async () => {
    mockChrome.storage.local.get = mock.fn((keys, cb) => cb({}))
    await loadOptions()

    await saveOptions()

    assert.ok(mockChrome.storage.local.set.mock.calls.some((c) => c.arguments[0].sourceMapEnabled === false))
  })

  test('should include sourceMapEnabled=true in save when active', async () => {
    mockChrome.storage.local.get = mock.fn((keys, cb) => cb({ sourceMapEnabled: true }))
    await loadOptions()

    await saveOptions()

    assert.ok(mockChrome.storage.local.set.mock.calls.some((c) => c.arguments[0].sourceMapEnabled === true))
  })

  test('should send setSourceMapEnabled message on save', async () => {
    mockChrome.storage.local.get = mock.fn((keys, cb) => cb({ sourceMapEnabled: true }))
    await loadOptions()

    await saveOptions()

    assert.ok(
      mockChrome.runtime.sendMessage.mock.calls.some(
        (c) => c.arguments[0].type === 'set_source_map_enabled' && c.arguments[0].enabled === true
      )
    )
  })
})

describe('Options Debug Mode Toggle', () => {
  beforeEach(() => {
    globalThis.document = createMockDocument()
    mockChrome.runtime.sendMessage = mock.fn()
    mockChrome.storage.local.get = mock.fn((keys, cb) => cb({}))
    mockChrome.storage.local.set = mock.fn((data, cb) => cb && cb())
  })

  test('should not activate debug mode toggle when no saved value (default: off)', async () => {
    mockChrome.storage.local.get = mock.fn((keys, cb) => cb({}))

    await loadOptions()

    const toggle = document.getElementById('debug-mode-toggle')
    assert.ok(!toggle.classList.contains('active'))
  })

  test('should activate debug mode toggle when saved as true', async () => {
    mockChrome.storage.local.get = mock.fn((keys, cb) => {
      cb({ debugMode: true })
    })

    await loadOptions()

    const toggle = document.getElementById('debug-mode-toggle')
    assert.ok(toggle.classList.contains('active'))
  })

  test('should not activate debug mode toggle when saved as false', async () => {
    mockChrome.storage.local.get = mock.fn((keys, cb) => {
      cb({ debugMode: false })
    })

    await loadOptions()

    const toggle = document.getElementById('debug-mode-toggle')
    assert.ok(!toggle.classList.contains('active'))
  })

  test('should toggle debug mode state on click', async () => {
    mockChrome.storage.local.get = mock.fn((keys, cb) => cb({ debugMode: true }))
    await loadOptions()

    const toggle = document.getElementById('debug-mode-toggle')
    assert.ok(toggle.classList.contains('active'))

    toggleDebugMode()
    assert.ok(!toggle.classList.contains('active'))

    toggleDebugMode()
    assert.ok(toggle.classList.contains('active'))
  })

  test('should include debugMode=false in save when inactive', async () => {
    mockChrome.storage.local.get = mock.fn((keys, cb) => cb({}))
    await loadOptions()

    await saveOptions()

    assert.ok(mockChrome.storage.local.set.mock.calls.some((c) => c.arguments[0].debugMode === false))
  })

  test('should include debugMode=true in save when active', async () => {
    mockChrome.storage.local.get = mock.fn((keys, cb) => cb({ debugMode: true }))
    await loadOptions()

    await saveOptions()

    assert.ok(mockChrome.storage.local.set.mock.calls.some((c) => c.arguments[0].debugMode === true))
  })

  test('should send setDebugMode message on save', async () => {
    mockChrome.storage.local.get = mock.fn((keys, cb) => cb({ debugMode: true }))
    await loadOptions()

    await saveOptions()

    assert.ok(
      mockChrome.runtime.sendMessage.mock.calls.some(
        (c) => c.arguments[0].type === 'set_debug_mode' && c.arguments[0].enabled === true
      )
    )
  })

  test('should send setDebugMode=false when disabled', async () => {
    mockChrome.storage.local.get = mock.fn((keys, cb) => cb({ debugMode: false }))
    await loadOptions()

    await saveOptions()

    assert.ok(
      mockChrome.runtime.sendMessage.mock.calls.some(
        (c) => c.arguments[0].type === 'set_debug_mode' && c.arguments[0].enabled === false
      )
    )
  })
})

describe('Options daemon synchronization', () => {
  beforeEach(() => {
    globalThis.document = createMockDocument()
    mockChrome.runtime.sendMessage = mock.fn()
    mockChrome.storage.local.get = mock.fn((keys, cb) => cb({}))
    mockChrome.storage.local.set = mock.fn((data, cb) => cb && cb())
  })

  test('reports a resolved daemon rejection to Doctor without exposing the saved path', async () => {
    document.getElementById('terminal-dev-root').value = '/private/project'
    globalThis.fetch = mock.fn(() => Promise.resolve({ ok: false, status: 500 }))

    await saveOptions()

    const report = mockChrome.runtime.sendMessage.mock.calls.find(
      (call) =>
        call.arguments[0].type === 'report_state_recovery' &&
        call.arguments[0].diagnostic.name === 'active_codebase_sync'
    )?.arguments[0]
    assert.equal(report?.lifecycle, 'active')
    assert.equal(report?.diagnostic.name, 'active_codebase_sync')
    assert.doesNotMatch(JSON.stringify(report), /private\/project/)
  })

  test('treats an unavailable daemon as expected absence while preserving the local save', async () => {
    document.getElementById('terminal-dev-root').value = '/local/project'
    globalThis.fetch = mock.fn(() => Promise.reject(new Error('offline')))

    await saveOptions()

    assert.ok(mockChrome.storage.local.set.mock.calls.length > 0)
    assert.equal(
      mockChrome.runtime.sendMessage.mock.calls.some(
        (call) =>
          call.arguments[0].type === 'report_state_recovery' &&
          call.arguments[0].diagnostic.name === 'active_codebase_sync'
      ),
      false
    )
  })
})
