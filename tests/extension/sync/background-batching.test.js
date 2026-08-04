// @ts-nocheck
/**
 * @fileoverview background-batching.test.js — Tests for the background service worker:
 * log batching/debouncing, server communication, health checks, badge updates,
 * log formatting, and log level filtering.
 */

import { test, describe, mock, beforeEach } from 'node:test'
import assert from 'node:assert'
import { MANIFEST_VERSION } from '../shared/helpers.js'

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
import { sendLogsToServer, checkServerHealth, updateBadge } from '../../../extension/background/sync/server.js'
import { formatLogEntry, shouldCaptureLog } from '../../../extension/background/sync/log-processing.js'
import {
  measureContextSize,
  checkContextAnnotations,
  getContextWarning,
  resetContextWarning
} from '../../../extension/background/caches/snapshots.js'

describe('sendLogsToServer', () => {
  beforeEach(() => {
    mock.reset()
  })

  test('should POST entries to server', async () => {
    const mockFetch = mock.fn(() =>
      Promise.resolve({
        ok: true,
        json: () => Promise.resolve({ entries: 2 })
      })
    )
    globalThis.fetch = mockFetch

    const entries = [
      { ts: '2024-01-22T10:00:00Z', level: 'error', msg: 'test1' },
      { ts: '2024-01-22T10:00:01Z', level: 'warn', msg: 'test2' }
    ]

    const result = await sendLogsToServer('http://localhost:7890', entries)

    assert.strictEqual(mockFetch.mock.calls.length, 1)
    const [url, options] = mockFetch.mock.calls[0].arguments
    assert.strictEqual(url, 'http://localhost:7890/logs')
    assert.strictEqual(options.method, 'POST')
    assert.strictEqual(options.headers['Content-Type'], 'application/json')

    const body = JSON.parse(options.body)
    assert.strictEqual(body.entries.length, 2)
    assert.strictEqual(body.entries[0].level, 'error')
    assert.strictEqual(body.entries[0].msg, 'test1')
    assert.strictEqual(body.entries[1].level, 'warn')
    assert.strictEqual(body.entries[1].msg, 'test2')
    assert.strictEqual(result.entries, 2)
  })

  test('should throw on server error', async () => {
    globalThis.fetch = mock.fn(() =>
      Promise.resolve({
        ok: false,
        status: 500,
        statusText: 'Internal Server Error'
      })
    )

    await assert.rejects(() => sendLogsToServer('http://localhost:7890', [{ msg: 'test' }]), {
      message: /Server error.*500/
    })
  })

  test('should throw on network error', async () => {
    globalThis.fetch = mock.fn(() => Promise.reject(new Error('Network error')))

    await assert.rejects(() => sendLogsToServer('http://localhost:7890', [{ msg: 'test' }]), {
      message: /Network error/
    })
  })
})

describe('checkServerHealth', () => {
  beforeEach(() => {
    mock.reset()
  })

  test('should return health status when server is up', async () => {
    globalThis.fetch = mock.fn(() =>
      Promise.resolve({
        ok: true,
        json: () =>
          Promise.resolve({
            status: 'ok',
            entries: 42,
            maxEntries: 1000,
            capture: {
              extension_connected: true
            }
          })
      })
    )

    const health = await checkServerHealth()

    assert.strictEqual(health.status, 'ok')
    assert.strictEqual(health.entries, 42)
    assert.strictEqual(health.connected, true)
  })

  test('should keep daemon connected when extension heartbeat is missing', async () => {
    globalThis.fetch = mock.fn(() =>
      Promise.resolve({
        ok: true,
        json: () =>
          Promise.resolve({
            status: 'ok',
            entries: 42,
            maxEntries: 1000,
            capture: {
              extension_connected: false,
              extension_last_seen: ''
            }
          })
      })
    )

    const health = await checkServerHealth()

    assert.strictEqual(health.connected, true)
    assert.strictEqual(health.extensionConnected, false)
    assert.ok(health.extensionError.includes('heartbeat'))
  })

  test('should return disconnected when server is down', async () => {
    globalThis.fetch = mock.fn(() => Promise.reject(new Error('Connection refused')))

    const health = await checkServerHealth()

    assert.strictEqual(health.connected, false)
    assert.ok(health.error.includes('Connection refused'))
  })

  test('should return disconnected on non-200 response', async () => {
    globalThis.fetch = mock.fn(() =>
      Promise.resolve({
        ok: false,
        status: 500
      })
    )

    const health = await checkServerHealth()

    assert.strictEqual(health.connected, false)
  })
})

describe('updateBadge', () => {
  beforeEach(() => {
    mock.reset()
    mockChrome.action.setBadgeText.mock.resetCalls()
    mockChrome.action.setBadgeBackgroundColor.mock.resetCalls()
  })

  test('should show green badge when connected with zero errors', () => {
    updateBadge({ connected: true, errorCount: 0 })

    assert.strictEqual(mockChrome.action.setBadgeText.mock.calls.length, 1)
    assert.strictEqual(mockChrome.action.setBadgeBackgroundColor.mock.calls.length, 1)

    const textArg = mockChrome.action.setBadgeText.mock.calls[0].arguments[0]
    assert.strictEqual(textArg.text, '')
    assert.strictEqual(typeof textArg, 'object', 'setBadgeText argument should be an object')

    const colorArg = mockChrome.action.setBadgeBackgroundColor.mock.calls[0].arguments[0]
    assert.strictEqual(colorArg.color, '#3fb950') // green
    assert.strictEqual(typeof colorArg, 'object', 'setBadgeBackgroundColor argument should be an object')
  })

  test('should show error count when connected with errors', () => {
    updateBadge({ connected: true, errorCount: 5 })

    const [textCall] = mockChrome.action.setBadgeText.mock.calls
    assert.strictEqual(textCall.arguments[0].text, '5')

    const [colorCall] = mockChrome.action.setBadgeBackgroundColor.mock.calls
    assert.strictEqual(colorCall.arguments[0].color, '#3fb950') // green
  })

  test('should show 99+ when error count exceeds 99', () => {
    updateBadge({ connected: true, errorCount: 150 })

    const [textCall] = mockChrome.action.setBadgeText.mock.calls
    assert.strictEqual(textCall.arguments[0].text, '99+')
  })

  test('should show red badge when disconnected', () => {
    updateBadge({ connected: false, errorCount: 0 })

    const [textCall] = mockChrome.action.setBadgeText.mock.calls
    assert.strictEqual(textCall.arguments[0].text, '!')

    const [colorCall] = mockChrome.action.setBadgeBackgroundColor.mock.calls
    assert.strictEqual(colorCall.arguments[0].color, '#f85149') // red
  })
})

describe('formatLogEntry', () => {
  test('should add timestamp if not present', () => {
    const entry = formatLogEntry({ level: 'error', msg: 'test' })

    assert.ok(entry.ts)
    assert.ok(entry.ts.match(/^\d{4}-\d{2}-\d{2}T/)) // ISO format
    assert.strictEqual(entry.level, 'error')
    assert.strictEqual(entry.msg, 'test')
  })

  test('should preserve existing timestamp', () => {
    const ts = '2024-01-22T10:00:00.000Z'
    const entry = formatLogEntry({ ts, level: 'error', msg: 'test' })

    assert.strictEqual(entry.ts, ts)
  })

  test('should truncate large args at 10KB', () => {
    const largeString = 'x'.repeat(20000) // 20KB
    const entry = formatLogEntry({
      level: 'log',
      type: 'console',
      args: [largeString]
    })

    // Args should be truncated
    assert.ok(JSON.stringify(entry.args).length < 15000)
    assert.ok(entry.args[0].includes('[truncated]'))
  })

  test('should handle circular references', () => {
    const obj = { name: 'test' }
    obj.self = obj // circular

    const entry = formatLogEntry({
      level: 'log',
      type: 'console',
      args: [obj]
    })

    // Should not throw, should have placeholder
    assert.ok(entry.args)
    const serialized = JSON.stringify(entry.args)
    assert.ok(serialized.includes('[Circular') || serialized.includes('unserializable'))
  })

  test('should include URL from source', () => {
    const entry = formatLogEntry({
      level: 'error',
      msg: 'test',
      url: 'http://localhost:3000/page'
    })

    assert.strictEqual(entry.url, 'http://localhost:3000/page')
  })

  test('should preserve tabId when present in entry', () => {
    const entry = formatLogEntry({
      level: 'error',
      msg: 'test',
      tabId: 42
    })

    assert.strictEqual(entry.tabId, 42)
  })

  test('should work without tabId (backward compat)', () => {
    const entry = formatLogEntry({
      level: 'error',
      msg: 'test'
    })

    assert.strictEqual(entry.tabId, undefined)
  })
})

describe('shouldCaptureLog', () => {
  test('should capture all when level is "all"', () => {
    assert.strictEqual(shouldCaptureLog('debug', 'all'), true)
    assert.strictEqual(shouldCaptureLog('log', 'all'), true)
    assert.strictEqual(shouldCaptureLog('info', 'all'), true)
    assert.strictEqual(shouldCaptureLog('warn', 'all'), true)
    assert.strictEqual(shouldCaptureLog('error', 'all'), true)
  })

  test('should capture warn and error when level is "warn"', () => {
    assert.strictEqual(shouldCaptureLog('debug', 'warn'), false)
    assert.strictEqual(shouldCaptureLog('log', 'warn'), false)
    assert.strictEqual(shouldCaptureLog('info', 'warn'), false)
    assert.strictEqual(shouldCaptureLog('warn', 'warn'), true)
    assert.strictEqual(shouldCaptureLog('error', 'warn'), true)
  })

  test('should capture only error when level is "error"', () => {
    assert.strictEqual(shouldCaptureLog('debug', 'error'), false)
    assert.strictEqual(shouldCaptureLog('log', 'error'), false)
    assert.strictEqual(shouldCaptureLog('info', 'error'), false)
    assert.strictEqual(shouldCaptureLog('warn', 'error'), false)
    assert.strictEqual(shouldCaptureLog('error', 'error'), true)
  })

  test('should always capture network errors', () => {
    assert.strictEqual(shouldCaptureLog('error', 'error', 'network'), true)
    assert.strictEqual(shouldCaptureLog('error', 'warn', 'network'), true)
  })

  test('should always capture exceptions', () => {
    assert.strictEqual(shouldCaptureLog('error', 'error', 'exception'), true)
    assert.strictEqual(shouldCaptureLog('error', 'warn', 'exception'), true)
  })
})

describe('Context Annotation Monitoring', () => {
  beforeEach(() => {
    resetContextWarning()
  })

  describe('measureContextSize', () => {
    test('should return 0 for entries without _context', () => {
      const entry = { level: 'error', type: 'console', args: ['test'] }
      assert.strictEqual(measureContextSize(entry), 0)
    })

    test('should return 0 for entries with empty _context', () => {
      const entry = { level: 'error', _context: {} }
      assert.strictEqual(measureContextSize(entry), 0)
    })

    test('should return approximate byte size of _context', () => {
      const entry = {
        level: 'error',
        _context: {
          user: { id: 123, name: 'test' },
          page: { route: '/checkout' }
        }
      }
      const size = measureContextSize(entry)
      assert.ok(size > 0)
      assert.ok(size < 200) // Small context should be well under 200 bytes
    })

    test('should measure large context correctly', () => {
      const largeValue = 'x'.repeat(4000)
      const entry = {
        level: 'error',
        _context: {
          key1: largeValue,
          key2: largeValue,
          key3: largeValue,
          key4: largeValue,
          key5: largeValue,
          key6: largeValue // 6 x 4000 = ~24KB, over threshold
        }
      }
      const size = measureContextSize(entry)
      assert.ok(size > 20 * 1024, `Expected > 20KB, got ${size}`)
    })
  })

  describe('checkContextAnnotations', () => {
    test('should not warn for entries with small context', () => {
      const entries = [
        { level: 'error', _context: { user: { id: 1 } } },
        { level: 'error', _context: { page: '/home' } }
      ]
      checkContextAnnotations(entries)
      assert.strictEqual(getContextWarning(), null)
    })

    test('should not warn for entries without context', () => {
      const entries = [
        { level: 'error', args: ['test'] },
        { level: 'warn', msg: 'hello' }
      ]
      checkContextAnnotations(entries)
      assert.strictEqual(getContextWarning(), null)
    })

    test('should not warn for fewer than 3 excessive entries', () => {
      const largeContext = {}
      for (let i = 0; i < 6; i++) {
        largeContext[`key${i}`] = 'x'.repeat(4000)
      }

      // Only 2 excessive entries
      checkContextAnnotations([{ level: 'error', _context: largeContext }])
      checkContextAnnotations([{ level: 'error', _context: largeContext }])

      assert.strictEqual(getContextWarning(), null)
    })

    test('should warn after 3 excessive entries within 60s', () => {
      const largeContext = {}
      for (let i = 0; i < 6; i++) {
        largeContext[`key${i}`] = 'x'.repeat(4000)
      }

      checkContextAnnotations([{ level: 'error', _context: largeContext }])
      checkContextAnnotations([{ level: 'error', _context: largeContext }])
      checkContextAnnotations([{ level: 'error', _context: largeContext }])

      const warning = getContextWarning()
      assert.ok(warning !== null, 'Expected warning to be set')
      assert.ok(warning.sizeKB > 20, `Expected > 20KB, got ${warning.sizeKB}KB`)
      assert.ok(warning.count >= 3, `Expected count >= 3, got ${warning.count}`)
    })

    test('should include average size in warning', () => {
      const largeContext = {}
      for (let i = 0; i < 10; i++) {
        largeContext[`key${i}`] = 'x'.repeat(4000)
      }

      for (let i = 0; i < 3; i++) {
        checkContextAnnotations([{ level: 'error', _context: largeContext }])
      }

      const warning = getContextWarning()
      assert.ok(warning.sizeKB > 30) // 10 x 4000 = ~40KB
    })

    test('should handle batch with mix of large and small entries', () => {
      const largeContext = {}
      for (let i = 0; i < 6; i++) {
        largeContext[`key${i}`] = 'x'.repeat(4000)
      }

      // 3 batches, each with one large entry mixed with small ones
      for (let i = 0; i < 3; i++) {
        checkContextAnnotations([
          { level: 'error', _context: { small: 'val' } },
          { level: 'error', _context: largeContext },
          { level: 'warn', msg: 'no context' }
        ])
      }

      const warning = getContextWarning()
      assert.ok(warning !== null, 'Expected warning from large entries in mixed batches')
    })
  })

  describe('resetContextWarning', () => {
    test('should clear the warning state', () => {
      const largeContext = {}
      for (let i = 0; i < 6; i++) {
        largeContext[`key${i}`] = 'x'.repeat(4000)
      }

      for (let i = 0; i < 3; i++) {
        checkContextAnnotations([{ level: 'error', _context: largeContext }])
      }

      assert.ok(getContextWarning() !== null)
      resetContextWarning()
      assert.strictEqual(getContextWarning(), null)
    })
  })
})
