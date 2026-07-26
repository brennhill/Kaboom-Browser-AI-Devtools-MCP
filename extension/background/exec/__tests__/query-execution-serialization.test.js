// query-execution-serialization.test.js — Regression tests for execute_js host-object serialization.
//
// Bug #389: DOMRect-like results can serialize to {} when values live on the prototype.

import { beforeEach, describe, mock, test } from 'node:test'
import assert from 'node:assert'

const executeScriptMock = mock.fn(async ({ func, args = [] }) => [{ result: await func(...args) }])

if (typeof globalThis.chrome === 'undefined') {
  globalThis.chrome = {}
}
globalThis.chrome.runtime = globalThis.chrome.runtime || {
  getURL: () => '',
  sendMessage: () => Promise.resolve(),
  onMessage: { addListener: () => {} },
  onInstalled: { addListener: () => {} },
  getManifest: () => ({ version: '1.0.0' })
}
globalThis.chrome.storage = globalThis.chrome.storage || {
  local: { get: (_k, cb) => cb && cb({}), set: (_d, cb) => cb && cb() },
  onChanged: { addListener: () => {} }
}
globalThis.chrome.tabs = globalThis.chrome.tabs || {
  get: () => Promise.resolve({}),
  query: () => Promise.resolve([]),
  onRemoved: { addListener: () => {} },
  onUpdated: { addListener: () => {} },
  onActivated: { addListener: () => {} },
  onCreated: { addListener: () => {} }
}
globalThis.chrome.action = globalThis.chrome.action || { setBadgeText: () => {}, setBadgeBackgroundColor: () => {} }
globalThis.chrome.alarms = globalThis.chrome.alarms || { create: () => {}, clear: () => {}, onAlarm: { addListener: () => {} } }
globalThis.chrome.commands = globalThis.chrome.commands || { onCommand: { addListener: () => {} } }
globalThis.chrome.webNavigation = globalThis.chrome.webNavigation || { onCommitted: { addListener: () => {} } }
globalThis.chrome.offscreen = globalThis.chrome.offscreen || { hasDocument: async () => false, createDocument: async () => {}, closeDocument: async () => {} }
globalThis.chrome.windows = globalThis.chrome.windows || { update: async () => {} }
globalThis.chrome.scripting = { executeScript: executeScriptMock }

if (typeof globalThis.fetch === 'undefined') {
  globalThis.fetch = async () => ({ ok: false, status: 500, json: async () => ({}) })
}

const { executeViaScriptingAPI } = await import('../query-execution.js')

describe('execute_js serialization for host objects', () => {
  beforeEach(() => {
    executeScriptMock.mock.resetCalls()
    globalThis.window = globalThis
    globalThis.document = {
      querySelector: () => ({
        getBoundingClientRect: () => ({})
      })
    }
  })

  test('serializes DOMRect-like objects via toJSON', async () => {
    class DOMRectLike {
      toJSON() {
        return { x: 100, y: 200, width: 80, height: 32, top: 200, right: 180, bottom: 232, left: 100 }
      }
    }

    globalThis.document = {
      querySelector: () => ({
        getBoundingClientRect: () => new DOMRectLike()
      })
    }

    const result = await executeViaScriptingAPI(1, `document.querySelector('button').getBoundingClientRect()`, 500, 'MAIN')

    assert.strictEqual(result.success, true)
    assert.strictEqual(result.result.x, 100)
    assert.strictEqual(result.result.y, 200)
    assert.strictEqual(result.result.width, 80)
    assert.strictEqual(result.result.height, 32)
  })

  test('serializes getter-only prototype objects (no enumerable own keys)', async () => {
    class GetterRectLike {
      get x() { return 12 }
      get y() { return 34 }
      get width() { return 56 }
      get height() { return 78 }
      get top() { return 34 }
      get right() { return 68 }
      get bottom() { return 112 }
      get left() { return 12 }
    }

    globalThis.document = {
      querySelector: () => ({
        getBoundingClientRect: () => new GetterRectLike()
      })
    }

    const result = await executeViaScriptingAPI(1, `document.querySelector('button').getBoundingClientRect()`, 500, 'MAIN')

    assert.strictEqual(result.success, true)
    assert.strictEqual(result.result.x, 12)
    assert.strictEqual(result.result.y, 34)
    assert.strictEqual(result.result.width, 56)
    assert.strictEqual(result.result.height, 78)
    assert.ok(Object.keys(result.result).length >= 4, `expected non-empty serialized object, got: ${JSON.stringify(result.result)}`)
  })
})

