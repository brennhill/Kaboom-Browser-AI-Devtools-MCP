// @ts-nocheck
/**
 * @fileoverview pilot-state.test.js — Tests for browser state management.
 * Covers captureState/restoreState in inject.js and snapshot CRUD in background.js.
 * State includes localStorage, sessionStorage, and cookies.
 */

import { test, describe, mock, beforeEach } from 'node:test'
import assert from 'node:assert'
import { MANIFEST_VERSION } from '../shared/helpers.js'

// Mock Chrome APIs
const mockChrome = {
  runtime: {
    sendMessage: mock.fn(() => Promise.resolve()),
    onMessage: {
      addListener: mock.fn()
    },
    getManifest: () => ({ version: MANIFEST_VERSION })
  },
  storage: {
    sync: {
      get: mock.fn((keys, callback) => callback({})),
      set: mock.fn((data, callback) => callback && callback()),
      remove: mock.fn((keys, callback) => {
        if (typeof callback === 'function') callback()
        else return Promise.resolve()
      })
    },
    local: {
      get: mock.fn((key, callback) => callback({})),
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
  tabs: {
    query: mock.fn(() => Promise.resolve([{ id: 1 }])),
    sendMessage: mock.fn(() => Promise.resolve()),
    onRemoved: { addListener: mock.fn() }
  },
  alarms: {
    create: mock.fn(),
    onAlarm: { addListener: mock.fn() }
  }
}

globalThis.chrome = mockChrome

// Mock localStorage
const localStorageData = {}
const mockLocalStorage = {
  getItem: mock.fn((key) => localStorageData[key] || null),
  setItem: mock.fn((key, value) => {
    localStorageData[key] = value
  }),
  removeItem: mock.fn((key) => {
    delete localStorageData[key]
  }),
  clear: mock.fn(() => {
    Object.keys(localStorageData).forEach((key) => delete localStorageData[key])
  }),
  key: mock.fn((index) => Object.keys(localStorageData)[index] || null),
  get length() {
    return Object.keys(localStorageData).length
  }
}

// Mock sessionStorage
const sessionStorageData = {}
const mockSessionStorage = {
  getItem: mock.fn((key) => sessionStorageData[key] || null),
  setItem: mock.fn((key, value) => {
    sessionStorageData[key] = value
  }),
  removeItem: mock.fn((key) => {
    delete sessionStorageData[key]
  }),
  clear: mock.fn(() => {
    Object.keys(sessionStorageData).forEach((key) => delete sessionStorageData[key])
  }),
  key: mock.fn((index) => Object.keys(sessionStorageData)[index] || null),
  get length() {
    return Object.keys(sessionStorageData).length
  }
}

// Mock document
let mockCookie = ''
const mockDocument = {
  get cookie() {
    return mockCookie
  },
  set cookie(value) {
    // Handle cookie expiration (delete)
    if (value.includes('expires=Thu, 01 Jan 1970')) {
      const name = value.split('=')[0].trim()
      const cookies = mockCookie.split(';').filter((c) => c.trim() && !c.trim().startsWith(name + '='))
      mockCookie = cookies.join('; ')
    } else {
      // Add new cookie
      const parts = value.split(';')
      const cookiePart = parts[0].trim()
      if (cookiePart) {
        const existing = mockCookie.split(';').filter((c) => c.trim())
        const name = cookiePart.split('=')[0]
        const filtered = existing.filter((c) => !c.trim().startsWith(name + '='))
        filtered.push(cookiePart)
        mockCookie = filtered.join('; ')
      }
    }
  },
  readyState: 'complete',
  querySelector: () => null,
  querySelectorAll: () => []
}

// Mock window
const mockWindow = {
  location: {
    href: 'http://localhost:3000/test'
  },
  postMessage: mock.fn(),
  addEventListener: mock.fn()
}

globalThis.document = mockDocument
globalThis.window = mockWindow
globalThis.localStorage = mockLocalStorage
globalThis.sessionStorage = mockSessionStorage

describe('captureState', () => {
  beforeEach(() => {
    // Reset storage
    Object.keys(localStorageData).forEach((key) => delete localStorageData[key])
    Object.keys(sessionStorageData).forEach((key) => delete sessionStorageData[key])
    mockCookie = ''
    mock.reset()
  })

  test('should capture empty state when no storage exists', async () => {
    const { captureState } = await import('../../../extension/inject/state.js')

    const state = captureState()

    assert.strictEqual(state.url, 'http://localhost:3000/test')
    assert.ok(state.timestamp, 'Should have timestamp')
    assert.deepStrictEqual(state.localStorage, {})
    assert.deepStrictEqual(state.sessionStorage, {})
    assert.strictEqual(state.cookies, '')
  })

  test('should capture localStorage values', async () => {
    localStorageData['user'] = 'john'
    localStorageData['theme'] = 'dark'

    const { captureState } = await import('../../../extension/inject/state.js')

    const state = captureState()

    assert.strictEqual(state.localStorage['user'], 'john')
    assert.strictEqual(state.localStorage['theme'], 'dark')
    assert.strictEqual(Object.keys(state.localStorage).length, 2)
  })

  test('should capture sessionStorage values', async () => {
    sessionStorageData['viewId'] = 'abc123'
    sessionStorageData['cart'] = '["item1","item2"]'

    const { captureState } = await import('../../../extension/inject/state.js')

    const state = captureState()

    assert.strictEqual(state.sessionStorage['viewId'], 'abc123')
    assert.strictEqual(state.sessionStorage['cart'], '["item1","item2"]')
    assert.strictEqual(Object.keys(state.sessionStorage).length, 2)
  })

  test('should capture cookies', async () => {
    mockCookie = 'lang=en; preference=compact'

    const { captureState } = await import('../../../extension/inject/state.js')

    const state = captureState()

    assert.strictEqual(state.cookies, 'lang=en; preference=compact')
  })

  test('should redact sensitive cookie values', async () => {
    mockCookie = 'token=xyz789; preference=compact'

    const { captureState } = await import('../../../extension/inject/state.js')

    const state = captureState()

    assert.strictEqual(state.cookies, 'token=[REDACTED]; preference=compact')
  })

  test('should capture all storage types together', async () => {
    localStorageData['local_key'] = 'local_value'
    sessionStorageData['session_key'] = 'session_value'
    mockCookie = 'cookie_key=cookie_value'

    const { captureState } = await import('../../../extension/inject/state.js')

    const state = captureState()

    assert.strictEqual(state.localStorage['local_key'], 'local_value')
    assert.strictEqual(state.sessionStorage['session_key'], 'session_value')
    assert.strictEqual(state.cookies, 'cookie_key=cookie_value')
  })
})

describe('restoreState', () => {
  beforeEach(() => {
    Object.keys(localStorageData).forEach((key) => delete localStorageData[key])
    Object.keys(sessionStorageData).forEach((key) => delete sessionStorageData[key])
    mockCookie = ''
    mock.reset()
  })

  test('should restore localStorage values', async () => {
    const state = {
      url: 'http://localhost:3000/test',
      timestamp: Date.now(),
      localStorage: { user: 'john', theme: 'dark' },
      sessionStorage: {},
      cookies: ''
    }

    const { restoreState } = await import('../../../extension/inject/state.js')

    const result = restoreState(state, false)

    assert.strictEqual(result.success, true)
    assert.strictEqual(result.restored.localStorage, 2)
    assert.strictEqual(localStorageData['user'], 'john')
    assert.strictEqual(localStorageData['theme'], 'dark')
  })

  test('should restore sessionStorage values', async () => {
    const state = {
      url: 'http://localhost:3000/test',
      timestamp: Date.now(),
      localStorage: {},
      sessionStorage: { sessionId: 'abc123', cart: '["item1"]' },
      cookies: ''
    }

    const { restoreState } = await import('../../../extension/inject/state.js')

    const result = restoreState(state, false)

    assert.strictEqual(result.success, true)
    assert.strictEqual(result.restored.sessionStorage, 2)
    assert.strictEqual(sessionStorageData['sessionId'], 'abc123')
    assert.strictEqual(sessionStorageData['cart'], '["item1"]')
  })

  test('should restore cookies', async () => {
    const state = {
      url: 'http://localhost:3000/test',
      timestamp: Date.now(),
      localStorage: {},
      sessionStorage: {},
      cookies: 'token=xyz789; preference=compact'
    }

    const { restoreState } = await import('../../../extension/inject/state.js')

    const result = restoreState(state, false)

    assert.strictEqual(result.success, true)
    assert.strictEqual(result.restored.cookies, 2)
  })

  test('should preserve existing keys and add snapshot keys on restore', async () => {
    // Set up existing state
    localStorageData['existing'] = 'old_value'
    sessionStorageData['existing_session'] = 'old_session'
    mockCookie = 'old_cookie=old'

    const state = {
      url: 'http://localhost:3000/test',
      timestamp: Date.now(),
      localStorage: { new_key: 'new_value' },
      sessionStorage: { new_session: 'new_session_value' },
      cookies: 'new_cookie=new'
    }

    const { restoreState } = await import('../../../extension/inject/state.js')

    restoreState(state, false)

    // Existing keys are preserved (restore only overwrites snapshot keys)
    assert.strictEqual(localStorageData['existing'], 'old_value')
    assert.strictEqual(sessionStorageData['existing_session'], 'old_session')

    // New values should be present
    assert.strictEqual(localStorageData['new_key'], 'new_value')
    assert.strictEqual(sessionStorageData['new_session'], 'new_session_value')
  })

  test('should handle empty state object', async () => {
    localStorageData['existing'] = 'old_value'

    const state = {
      url: 'http://localhost:3000/test',
      timestamp: Date.now(),
      localStorage: {},
      sessionStorage: {},
      cookies: ''
    }

    const { restoreState } = await import('../../../extension/inject/state.js')

    const result = restoreState(state, false)

    assert.strictEqual(result.success, true)
    assert.strictEqual(result.restored.localStorage, 0)
    assert.strictEqual(result.restored.sessionStorage, 0)
    assert.strictEqual(result.restored.cookies, 0)
  })

  test('should handle undefined storage properties', async () => {
    const state = {
      url: 'http://localhost:3000/test',
      timestamp: Date.now()
      // localStorage, sessionStorage, cookies are undefined
    }

    const { restoreState } = await import('../../../extension/inject/state.js')

    const result = restoreState(state, false)

    assert.strictEqual(result.success, true)
    assert.strictEqual(result.restored.localStorage, 0)
    assert.strictEqual(result.restored.sessionStorage, 0)
    assert.strictEqual(result.restored.cookies, 0)
  })
})

describe('State Round-trip', () => {
  beforeEach(() => {
    Object.keys(localStorageData).forEach((key) => delete localStorageData[key])
    Object.keys(sessionStorageData).forEach((key) => delete sessionStorageData[key])
    mockCookie = ''
    mock.reset()
  })

  test('save -> clear -> load should restore original state', async () => {
    // Set up initial state
    localStorageData['user'] = 'john'
    localStorageData['settings'] = '{"volume":80}'
    sessionStorageData['temp'] = 'session_data'
    mockCookie = 'auth=token123'

    const { captureState, restoreState } = await import('../../../extension/inject/state.js')

    // Capture state
    const savedState = captureState()

    // Clear everything
    Object.keys(localStorageData).forEach((key) => delete localStorageData[key])
    Object.keys(sessionStorageData).forEach((key) => delete sessionStorageData[key])
    mockCookie = ''

    // Verify cleared
    assert.strictEqual(Object.keys(localStorageData).length, 0)
    assert.strictEqual(Object.keys(sessionStorageData).length, 0)

    // Restore state
    restoreState(savedState, false)

    // Verify restored
    assert.strictEqual(localStorageData['user'], 'john')
    assert.strictEqual(localStorageData['settings'], '{"volume":80}')
    assert.strictEqual(sessionStorageData['temp'], 'session_data')
  })
})

describe('Snapshot CRUD in background.js', () => {
  let snapshotStorage = {}

  beforeEach(() => {
    mock.reset()
    snapshotStorage = {}

    // Mock chrome.storage.local for snapshots
    mockChrome.storage.local.get.mock.mockImplementation((key, callback) => {
      if (typeof key === 'string') {
        callback({ [key]: snapshotStorage[key] })
      } else if (Array.isArray(key)) {
        const result = {}
        key.forEach((k) => {
          result[k] = snapshotStorage[k]
        })
        callback(result)
      } else {
        callback(snapshotStorage)
      }
    })

    mockChrome.storage.local.set.mock.mockImplementation((data, callback) => {
      Object.assign(snapshotStorage, data)
      if (callback) callback()
    })
  })

  test('saveStateSnapshot should store snapshot with name and size', async () => {
    const { saveStateSnapshot } = await import('../../../extension/background/message-handlers.js')

    const state = {
      url: 'http://localhost:3000/test',
      timestamp: Date.now(),
      localStorage: { key: 'value' },
      sessionStorage: {},
      cookies: ''
    }

    const result = await saveStateSnapshot('my-snapshot', state)

    assert.strictEqual(result.success, true)
    assert.strictEqual(result.snapshot_name, 'my-snapshot')
    assert.ok(result.size_bytes > 0, 'Should have size_bytes')

    // Verify stored
    const stored = snapshotStorage['kaboom_state_snapshots']
    assert.ok(stored['my-snapshot'], 'Snapshot should be stored')
    assert.strictEqual(stored['my-snapshot'].name, 'my-snapshot')
  })

  test('loadStateSnapshot should retrieve stored snapshot', async () => {
    const { saveStateSnapshot, loadStateSnapshot } = await import('../../../extension/background/message-handlers.js')

    const state = {
      url: 'http://localhost:3000/test',
      timestamp: 12345,
      localStorage: { user: 'john' },
      sessionStorage: { session: 'data' },
      cookies: 'a=b'
    }

    await saveStateSnapshot('test-snapshot', state)
    const loaded = await loadStateSnapshot('test-snapshot')

    assert.ok(loaded, 'Should return snapshot')
    assert.strictEqual(loaded.url, 'http://localhost:3000/test')
    assert.strictEqual(loaded.localStorage.user, 'john')
    assert.strictEqual(loaded.sessionStorage.session, 'data')
    assert.strictEqual(loaded.cookies, 'a=b')
  })

  test('loadStateSnapshot should return null for non-existent snapshot', async () => {
    const { loadStateSnapshot } = await import('../../../extension/background/message-handlers.js')

    const loaded = await loadStateSnapshot('non-existent')

    assert.strictEqual(loaded, null)
  })

  test('listStateSnapshots should return metadata for all snapshots', async () => {
    const { saveStateSnapshot, listStateSnapshots } = await import('../../../extension/background/message-handlers.js')

    await saveStateSnapshot('snap1', {
      url: 'http://localhost:3000/page1',
      timestamp: 1000,
      localStorage: {},
      sessionStorage: {},
      cookies: ''
    })

    await saveStateSnapshot('snap2', {
      url: 'http://localhost:3000/page2',
      timestamp: 2000,
      localStorage: { key: 'value' },
      sessionStorage: {},
      cookies: 'cookie=value'
    })

    const list = await listStateSnapshots()

    assert.strictEqual(list.length, 2)

    const snap1 = list.find((s) => s.name === 'snap1')
    const snap2 = list.find((s) => s.name === 'snap2')

    assert.ok(snap1, 'Should find snap1')
    assert.ok(snap2, 'Should find snap2')

    assert.strictEqual(snap1.url, 'http://localhost:3000/page1')
    assert.strictEqual(snap2.url, 'http://localhost:3000/page2')
    assert.ok(snap1.size_bytes > 0)
    assert.ok(snap2.size_bytes > 0)
  })

  test('deleteStateSnapshot should remove snapshot', async () => {
    const { saveStateSnapshot, loadStateSnapshot, deleteStateSnapshot } = await import(
      '../../../extension/background/message-handlers.js'
    )

    await saveStateSnapshot('to-delete', {
      url: 'http://localhost:3000/',
      timestamp: Date.now(),
      localStorage: {},
      sessionStorage: {},
      cookies: ''
    })

    // Verify it exists
    let loaded = await loadStateSnapshot('to-delete')
    assert.ok(loaded, 'Snapshot should exist')

    // Delete it
    const result = await deleteStateSnapshot('to-delete')
    assert.strictEqual(result.success, true)
    assert.strictEqual(result.deleted, 'to-delete')

    // Verify it's gone
    loaded = await loadStateSnapshot('to-delete')
    assert.strictEqual(loaded, null)
  })

  test('listStateSnapshots should return empty array when no snapshots', async () => {
    const { listStateSnapshots } = await import('../../../extension/background/message-handlers.js')

    const list = await listStateSnapshots()

    assert.deepStrictEqual(list, [])
  })
})

describe('include_url parameter', () => {
  beforeEach(() => {
    Object.keys(localStorageData).forEach((key) => delete localStorageData[key])
    Object.keys(sessionStorageData).forEach((key) => delete sessionStorageData[key])
    mockCookie = ''
    mock.reset()
    mockWindow.location.href = 'http://localhost:3000/current'
  })

  test('restoreState with include_url=false should not navigate', async () => {
    const state = {
      url: 'http://localhost:3000/different-page',
      timestamp: Date.now(),
      localStorage: { key: 'value' },
      sessionStorage: {},
      cookies: ''
    }

    const { restoreState } = await import('../../../extension/inject/state.js')

    restoreState(state, false)

    // URL should not have changed
    assert.strictEqual(mockWindow.location.href, 'http://localhost:3000/current')
  })

  test('restoreState with include_url=true defaults should try to navigate when URL differs', async () => {
    const state = {
      url: 'http://localhost:3000/different-page',
      timestamp: Date.now(),
      localStorage: {},
      sessionStorage: {},
      cookies: ''
    }

    const { restoreState } = await import('../../../extension/inject/state.js')

    // Note: In the test environment, we can't actually navigate,
    // but the function should attempt to set location.href
    restoreState(state, true)

    // The function attempts to navigate (in real browser this would trigger navigation)
    // We verify the function completed without error
  })

  test('restoreState with same URL should not attempt navigation', async () => {
    mockWindow.location.href = 'http://localhost:3000/same-page'

    const state = {
      url: 'http://localhost:3000/same-page',
      timestamp: Date.now(),
      localStorage: { key: 'value' },
      sessionStorage: {},
      cookies: ''
    }

    const { restoreState } = await import('../../../extension/inject/state.js')

    const result = restoreState(state, true)

    assert.strictEqual(result.success, true)
    // URL should remain the same
    assert.strictEqual(mockWindow.location.href, 'http://localhost:3000/same-page')
  })
})
