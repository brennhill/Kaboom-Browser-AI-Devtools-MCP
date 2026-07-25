// @ts-nocheck
/**
 * @fileoverview storage-utils.test.js -- Tests for storage-utils module.
 * Covers async local storage CRUD, async session storage CRUD,
 * graceful degradation when APIs are unavailable, diagnostics, and
 * service worker restart detection.
 *
 * Chrome storage mocks include remove() for local, sync, and session --
 * both callback and Promise patterns (known project requirement).
 */

import { test, describe, mock, beforeEach } from 'node:test'
import assert from 'node:assert'

// =============================================================================
// MOCK FACTORY: builds a mock chrome with configurable storage behavior
// =============================================================================

/**
 * Create a fresh chrome mock with working storage simulation.
 * Each storage area (local, session) maintains an in-memory store that
 * correctly supports both callback and Promise patterns.
 */
function createStorageMock() {
  function makeArea() {
    let store = {}
    return {
      _store: store,
      get: mock.fn((keys, callback) => {
        const result = {}
        const keyArr = Array.isArray(keys) ? keys : [keys]
        for (const k of keyArr) {
          if (store[k] !== undefined) result[k] = store[k]
        }
        if (typeof callback === 'function') callback(result)
        else return Promise.resolve(result)
      }),
      set: mock.fn((data, callback) => {
        Object.assign(store, data)
        if (typeof callback === 'function') callback()
        else return Promise.resolve()
      }),
      remove: mock.fn((keys, callback) => {
        const keyArr = Array.isArray(keys) ? keys : [keys]
        for (const k of keyArr) {
          delete store[k]
        }
        if (typeof callback === 'function') callback()
        else return Promise.resolve()
      }),
      clear: mock.fn((callback) => {
        store = {}
        if (typeof callback === 'function') callback()
        else return Promise.resolve()
      }),
      setAccessLevel: mock.fn((opts, callback) => {
        if (typeof callback === 'function') callback()
        else return Promise.resolve()
      })
    }
  }

  const local = makeArea()
  const session = makeArea()

  return {
    runtime: {
      lastError: null,
      onMessage: { addListener: mock.fn() },
      sendMessage: mock.fn(() => Promise.resolve()),
      getManifest: () => ({ version: '6.0.3' })
    },
    action: { setBadgeText: mock.fn(), setBadgeBackgroundColor: mock.fn() },
    storage: {
      local,
      sync: {
        get: mock.fn((k, cb) => cb && cb({})),
        set: mock.fn((d, cb) => cb && cb()),
        remove: mock.fn((k, cb) => {
          if (typeof cb === 'function') cb()
          else return Promise.resolve()
        })
      },
      session,
      onChanged: {
        addListener: mock.fn(),
        removeListener: mock.fn()
      }
    },
    tabs: {
      get: mock.fn(),
      query: mock.fn(),
      onRemoved: { addListener: mock.fn() }
    },
    alarms: { create: mock.fn(), onAlarm: { addListener: mock.fn() } }
  }
}

// We need to set up the chrome mock BEFORE importing the module because
// storage-utils checks chrome availability at import time.
let chromeMock = createStorageMock()
globalThis.chrome = chromeMock

// Mock navigator for getStorageDiagnostics
// navigator is a read-only getter in modern Node.js, so use defineProperty
if (!globalThis.navigator || !globalThis.navigator.userAgent) {
  Object.defineProperty(globalThis, 'navigator', {
    value: { userAgent: 'TestAgent/1.0' },
    writable: true,
    configurable: true
  })
}

// Dynamic import — import once and test against the live chrome global
let storageUtils
async function loadModule() {
  if (!storageUtils) {
    storageUtils = await import('../../extension/lib/storage-utils.js')
  }
  return storageUtils
}

// =============================================================================
// LOCAL STORAGE (async)
// =============================================================================

describe('Local Storage (async)', () => {
  beforeEach(() => {
    chromeMock = createStorageMock()
    globalThis.chrome = chromeMock
  })

  test('setLocal should store value in local storage', async () => {
    const mod = await loadModule()
    await mod.setLocal('persistKey', { data: 'hello' })
    assert.strictEqual(chromeMock.storage.local.set.mock.calls.length, 1)
    const setArg = chromeMock.storage.local.set.mock.calls[0].arguments[0]
    assert.deepStrictEqual(setArg, { persistKey: { data: 'hello' } })
  })

  test('setLocals should store multiple values', async () => {
    const mod = await loadModule()
    await mod.setLocals({ a: 1, b: 2 })
    assert.strictEqual(chromeMock.storage.local.set.mock.calls.length, 1)
    const setArg = chromeMock.storage.local.set.mock.calls[0].arguments[0]
    assert.deepStrictEqual(setArg, { a: 1, b: 2 })
  })

  test('getLocal should retrieve value from local storage', async () => {
    const mod = await loadModule()
    chromeMock.storage.local.get = mock.fn(() => {
      return Promise.resolve({ serverUrl: 'http://localhost:3333' })
    })
    const value = await mod.getLocal('serverUrl')
    assert.strictEqual(value, 'http://localhost:3333')
  })

  test('getLocal should return undefined for missing key', async () => {
    const mod = await loadModule()
    chromeMock.storage.local.get = mock.fn(() => {
      return Promise.resolve({})
    })
    const value = await mod.getLocal('nope')
    assert.strictEqual(value, undefined)
  })

  test('getLocals should retrieve multiple values', async () => {
    const mod = await loadModule()
    chromeMock.storage.local.get = mock.fn(() => {
      return Promise.resolve({ a: 1, b: 2 })
    })
    const result = await mod.getLocals(['a', 'b'])
    assert.deepStrictEqual(result, { a: 1, b: 2 })
  })

  test('removeLocal should call local.remove', async () => {
    const mod = await loadModule()
    await mod.removeLocal('deadKey')
    assert.strictEqual(chromeMock.storage.local.remove.mock.calls.length, 1)
    const removeArg = chromeMock.storage.local.remove.mock.calls[0].arguments[0]
    assert.deepStrictEqual(removeArg, ['deadKey'])
  })

  test('removeLocals should remove multiple keys', async () => {
    const mod = await loadModule()
    await mod.removeLocals(['a', 'b'])
    assert.strictEqual(chromeMock.storage.local.remove.mock.calls.length, 1)
    const removeArg = chromeMock.storage.local.remove.mock.calls[0].arguments[0]
    assert.deepStrictEqual(removeArg, ['a', 'b'])
  })
})

// =============================================================================
// FAIL-LOUD WRITES (chrome.runtime.lastError) — F3
// =============================================================================

describe('Fail-loud writes (chrome.runtime.lastError)', () => {
  beforeEach(() => {
    chromeMock = createStorageMock()
    globalThis.chrome = chromeMock
  })

  test('setLocal rejects when lastError is set on the write callback', async () => {
    const mod = await loadModule()
    chromeMock.storage.local.set = mock.fn((_data, cb) => {
      chromeMock.runtime.lastError = { message: 'QUOTA_BYTES quota exceeded' }
      if (typeof cb === 'function') cb()
      chromeMock.runtime.lastError = null
    })
    await assert.rejects(() => mod.setLocal('k', 'v'), /write failed: QUOTA_BYTES quota exceeded/)
  })

  test('setLocals rejects when lastError is set on the write callback', async () => {
    const mod = await loadModule()
    chromeMock.storage.local.set = mock.fn((_data, cb) => {
      chromeMock.runtime.lastError = { message: 'context invalidated' }
      if (typeof cb === 'function') cb()
      chromeMock.runtime.lastError = null
    })
    await assert.rejects(() => mod.setLocals({ a: 1 }), /write failed: context invalidated/)
  })

  test('removeLocal rejects when lastError is set on the remove callback', async () => {
    const mod = await loadModule()
    chromeMock.storage.local.remove = mock.fn((_keys, cb) => {
      chromeMock.runtime.lastError = { message: 'gone' }
      if (typeof cb === 'function') cb()
      chromeMock.runtime.lastError = null
    })
    await assert.rejects(() => mod.removeLocal('k'), /remove failed: gone/)
  })

  test('setSession rejects when lastError is set on the write callback', async () => {
    const mod = await loadModule()
    chromeMock.storage.session.set = mock.fn((_data, cb) => {
      chromeMock.runtime.lastError = { message: 'session boom' }
      if (typeof cb === 'function') cb()
      chromeMock.runtime.lastError = null
    })
    await assert.rejects(() => mod.setSession('k', 'v'), /write failed: session boom/)
  })

  test('setLocal resolves normally when lastError is absent', async () => {
    const mod = await loadModule()
    await assert.doesNotReject(() => mod.setLocal('k', 'v'))
  })

  test('persist swallows a rejected write but logs a warning (never throws)', async () => {
    const mod = await loadModule()
    const warnings = []
    const origWarn = console.warn
    console.warn = (...args) => warnings.push(args.join(' '))
    try {
      mod.persist(Promise.reject(new Error('boom')), 'unit-test')
      await new Promise((r) => setTimeout(r, 0))
    } finally {
      console.warn = origWarn
    }
    assert.strictEqual(warnings.length, 1)
    assert.match(warnings[0], /storage write failed \(unit-test\)/)
  })

  test('persist does not log when the write succeeds', async () => {
    const mod = await loadModule()
    const warnings = []
    const origWarn = console.warn
    console.warn = (...args) => warnings.push(args.join(' '))
    try {
      mod.persist(Promise.resolve(), 'unit-test-ok')
      await new Promise((r) => setTimeout(r, 0))
    } finally {
      console.warn = origWarn
    }
    assert.strictEqual(warnings.length, 0)
  })
})

// =============================================================================
// LOCAL STORAGE GRACEFUL DEGRADATION
// =============================================================================

describe('Local Storage Graceful Degradation', () => {
  beforeEach(() => {
    chromeMock = createStorageMock()
    globalThis.chrome = chromeMock
  })

  test('setLocal should resolve when chrome is undefined', async () => {
    const mod = await loadModule()
    const savedChrome = globalThis.chrome
    delete globalThis.chrome
    await assert.doesNotReject(async () => {
      await mod.setLocal('key', 'val')
    })
    globalThis.chrome = savedChrome
  })

  test('getLocal should return undefined when chrome is undefined', async () => {
    const mod = await loadModule()
    const savedChrome = globalThis.chrome
    delete globalThis.chrome
    const value = await mod.getLocal('key')
    assert.strictEqual(value, undefined)
    globalThis.chrome = savedChrome
  })

  test('getLocals should return empty object when chrome is undefined', async () => {
    const mod = await loadModule()
    const savedChrome = globalThis.chrome
    delete globalThis.chrome
    const result = await mod.getLocals(['key'])
    assert.deepStrictEqual(result, {})
    globalThis.chrome = savedChrome
  })

  test('removeLocal should resolve when chrome is undefined', async () => {
    const mod = await loadModule()
    const savedChrome = globalThis.chrome
    delete globalThis.chrome
    await assert.doesNotReject(async () => {
      await mod.removeLocal('key')
    })
    globalThis.chrome = savedChrome
  })

  test('setLocal should resolve when chrome.storage is missing', async () => {
    const mod = await loadModule()
    delete globalThis.chrome.storage
    await assert.doesNotReject(async () => {
      await mod.setLocal('key', 'val')
    })
  })

  test('getLocal should return undefined when chrome.storage is missing', async () => {
    const mod = await loadModule()
    delete globalThis.chrome.storage
    const value = await mod.getLocal('key')
    assert.strictEqual(value, undefined)
  })
})

// =============================================================================
// SESSION STORAGE (async)
// =============================================================================

describe('Session Storage (async)', () => {
  beforeEach(() => {
    chromeMock = createStorageMock()
    globalThis.chrome = chromeMock
  })

  test('setSession should store value in session storage', async () => {
    const mod = await loadModule()
    await mod.setSession('testKey', 'testValue')
    assert.strictEqual(chromeMock.storage.session.set.mock.calls.length, 1)
    const setArg = chromeMock.storage.session.set.mock.calls[0].arguments[0]
    assert.deepStrictEqual(setArg, { testKey: 'testValue' })
  })

  test('getSession should retrieve value from session storage', async () => {
    const mod = await loadModule()
    chromeMock.storage.session.get = mock.fn(() => {
      return Promise.resolve({ myKey: 42 })
    })
    const value = await mod.getSession('myKey')
    assert.strictEqual(value, 42)
  })

  test('getSession should return undefined for missing key', async () => {
    const mod = await loadModule()
    chromeMock.storage.session.get = mock.fn(() => {
      return Promise.resolve({})
    })
    const value = await mod.getSession('missing')
    assert.strictEqual(value, undefined)
  })

  test('removeSession should call session.remove', async () => {
    const mod = await loadModule()
    await mod.removeSession('deleteMe')
    assert.strictEqual(chromeMock.storage.session.remove.mock.calls.length, 1)
    const removeArg = chromeMock.storage.session.remove.mock.calls[0].arguments[0]
    assert.deepStrictEqual(removeArg, ['deleteMe'])
  })

  test('removeSessions should remove multiple keys', async () => {
    const mod = await loadModule()
    await mod.removeSessions(['a', 'b'])
    assert.strictEqual(chromeMock.storage.session.remove.mock.calls.length, 1)
    const removeArg = chromeMock.storage.session.remove.mock.calls[0].arguments[0]
    assert.deepStrictEqual(removeArg, ['a', 'b'])
  })
})

// =============================================================================
// SESSION STORAGE GRACEFUL DEGRADATION
// =============================================================================

describe('Session Storage Graceful Degradation', () => {
  beforeEach(() => {
    chromeMock = createStorageMock()
    globalThis.chrome = chromeMock
  })

  test('setSession should resolve when session storage unavailable', async () => {
    const mod = await loadModule()
    delete globalThis.chrome.storage.session
    await assert.doesNotReject(async () => {
      await mod.setSession('key', 'val')
    })
  })

  test('getSession should return undefined when session storage unavailable', async () => {
    const mod = await loadModule()
    delete globalThis.chrome.storage.session
    const value = await mod.getSession('key')
    assert.strictEqual(value, undefined)
  })

  test('removeSession should resolve when session storage unavailable', async () => {
    const mod = await loadModule()
    delete globalThis.chrome.storage.session
    await assert.doesNotReject(async () => {
      await mod.removeSession('key')
    })
  })

  test('setSession should resolve when chrome is undefined', async () => {
    const mod = await loadModule()
    const savedChrome = globalThis.chrome
    delete globalThis.chrome
    await assert.doesNotReject(async () => {
      await mod.setSession('key', 'val')
    })
    globalThis.chrome = savedChrome
  })

  test('getSession should return undefined when chrome is undefined', async () => {
    const mod = await loadModule()
    const savedChrome = globalThis.chrome
    delete globalThis.chrome
    const value = await mod.getSession('key')
    assert.strictEqual(value, undefined)
    globalThis.chrome = savedChrome
  })
})

// =============================================================================
// STORAGE CHANGE LISTENER
// =============================================================================

describe('Storage Change Listener', () => {
  beforeEach(() => {
    chromeMock = createStorageMock()
    globalThis.chrome = chromeMock
  })

  test('onStorageChanged should register a listener', async () => {
    const mod = await loadModule()
    const listener = () => {}
    mod.onStorageChanged(listener)
    assert.strictEqual(chromeMock.storage.onChanged.addListener.mock.calls.length, 1)
    assert.strictEqual(chromeMock.storage.onChanged.addListener.mock.calls[0].arguments[0], listener)
  })

  test('onStorageChanged should return an unsubscribe function', async () => {
    const mod = await loadModule()
    const listener = () => {}
    const unsub = mod.onStorageChanged(listener)
    assert.strictEqual(typeof unsub, 'function')
    unsub()
    assert.strictEqual(chromeMock.storage.onChanged.removeListener.mock.calls.length, 1)
  })

  test('onStorageChanged should return noop when chrome is undefined', async () => {
    const mod = await loadModule()
    const savedChrome = globalThis.chrome
    delete globalThis.chrome
    const unsub = mod.onStorageChanged(() => {})
    assert.strictEqual(typeof unsub, 'function')
    unsub() // should not throw
    globalThis.chrome = savedChrome
  })
})

// =============================================================================
// DIAGNOSTICS
// =============================================================================

describe('Storage Diagnostics', () => {
  beforeEach(() => {
    chromeMock = createStorageMock()
    globalThis.chrome = chromeMock
  })

  test('should report session and local storage availability', async () => {
    const mod = await loadModule()
    const diag = mod.getStorageDiagnostics()
    assert.strictEqual(diag.sessionStorageAvailable, true)
    assert.strictEqual(diag.localStorageAvailable, true)
    assert.strictEqual(typeof diag.browserVersion, 'string')
  })

  test('should report session storage unavailable when missing', async () => {
    const mod = await loadModule()
    delete globalThis.chrome.storage.session
    const diag = mod.getStorageDiagnostics()
    assert.strictEqual(diag.sessionStorageAvailable, false)
    assert.strictEqual(diag.localStorageAvailable, true)
  })

  test('should report local storage unavailable when chrome is undefined', async () => {
    const mod = await loadModule()
    const savedChrome = globalThis.chrome
    delete globalThis.chrome
    const diag = mod.getStorageDiagnostics()
    assert.strictEqual(diag.sessionStorageAvailable, false)
    assert.strictEqual(diag.localStorageAvailable, false)
    globalThis.chrome = savedChrome
  })
})

// =============================================================================
// SERVICE WORKER RESTART DETECTION
// =============================================================================

describe('Service Worker Restart Detection', () => {
  beforeEach(() => {
    chromeMock = createStorageMock()
    globalThis.chrome = chromeMock
  })

  test('wasServiceWorkerRestarted should return true when version not set', async () => {
    const mod = await loadModule()
    chromeMock.storage.session.get = mock.fn((keys, callback) => {
      if (typeof callback === 'function') callback({})
      else return Promise.resolve({})
    })
    const wasRestarted = await mod.wasServiceWorkerRestarted()
    assert.strictEqual(wasRestarted, true)
  })

  test('wasServiceWorkerRestarted should return false after markStateVersion', async () => {
    const mod = await loadModule()

    // markStateVersion stores the version in session storage
    // Then wasServiceWorkerRestarted checks if it matches
    let storedVersion = null
    chromeMock.storage.session.set = mock.fn((data, cb) => {
      for (const [k, v] of Object.entries(data)) {
        storedVersion = { key: k, value: v }
      }
      if (typeof cb === 'function') cb()
      else return Promise.resolve()
    })
    chromeMock.storage.session.get = mock.fn((keys, callback) => {
      const result = storedVersion ? { [storedVersion.key]: storedVersion.value } : {}
      if (typeof callback === 'function') callback(result)
      else return Promise.resolve(result)
    })

    await mod.markStateVersion()
    const wasRestarted = await mod.wasServiceWorkerRestarted()
    assert.strictEqual(wasRestarted, false)
  })

  test('wasServiceWorkerRestarted should return false when session storage unavailable', async () => {
    const mod = await loadModule()
    delete globalThis.chrome.storage.session
    const wasRestarted = await mod.wasServiceWorkerRestarted()
    assert.strictEqual(wasRestarted, false)
  })

  test('markStateVersion should resolve without error', async () => {
    const mod = await loadModule()
    await assert.doesNotReject(async () => {
      await mod.markStateVersion()
    })
  })
})

// =============================================================================
// DATA SERIALIZATION (async)
// =============================================================================

describe('Data Serialization', () => {
  beforeEach(() => {
    chromeMock = createStorageMock()
    globalThis.chrome = chromeMock
  })

  test('should store and retrieve complex objects via local storage', async () => {
    const mod = await loadModule()
    const complexData = {
      nested: { array: [1, 2, 3], obj: { a: true } },
      number: 42,
      string: 'hello',
      nullVal: null
    }

    const stored = {}
    chromeMock.storage.local.set = mock.fn((data) => {
      Object.assign(stored, data)
      return Promise.resolve()
    })
    chromeMock.storage.local.get = mock.fn(() => {
      return Promise.resolve(stored)
    })

    await mod.setLocal('complex', complexData)
    const value = await mod.getLocal('complex')
    assert.deepStrictEqual(value, complexData)
  })

  test('should handle boolean values via session storage', async () => {
    const mod = await loadModule()
    const stored = {}
    chromeMock.storage.session.set = mock.fn((data) => {
      Object.assign(stored, data)
      return Promise.resolve()
    })
    chromeMock.storage.session.get = mock.fn(() => {
      return Promise.resolve(stored)
    })

    await mod.setSession('flag', true)
    const value = await mod.getSession('flag')
    assert.strictEqual(value, true)
  })

  test('should handle numeric values including zero via session storage', async () => {
    const mod = await loadModule()
    const stored = {}
    chromeMock.storage.session.set = mock.fn((data) => {
      Object.assign(stored, data)
      return Promise.resolve()
    })
    chromeMock.storage.session.get = mock.fn(() => {
      return Promise.resolve(stored)
    })

    await mod.setSession('count', 0)
    const value = await mod.getSession('count')
    assert.strictEqual(value, 0)
  })
})

// =============================================================================
// SESSION ACCESS LEVEL
// =============================================================================

describe('Session Access Level', () => {
  beforeEach(() => {
    chromeMock = createStorageMock()
    globalThis.chrome = chromeMock
  })

  test('setSessionAccessLevel should call session.setAccessLevel', async () => {
    const mod = await loadModule()
    await mod.setSessionAccessLevel('TRUSTED_AND_UNTRUSTED_CONTEXTS')
    assert.strictEqual(chromeMock.storage.session.setAccessLevel.mock.calls.length, 1)
    const arg = chromeMock.storage.session.setAccessLevel.mock.calls[0].arguments[0]
    assert.deepStrictEqual(arg, { accessLevel: 'TRUSTED_AND_UNTRUSTED_CONTEXTS' })
  })

  test('setSessionAccessLevel should resolve when session storage unavailable', async () => {
    const mod = await loadModule()
    delete globalThis.chrome.storage.session
    await assert.doesNotReject(async () => {
      await mod.setSessionAccessLevel('TRUSTED_CONTEXTS')
    })
  })
})
