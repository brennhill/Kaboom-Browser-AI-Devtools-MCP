// @ts-nocheck
/**
 * @fileoverview popup-untrack-storage.test.js — Regression tests for popup untrack drift.
 * The popup previously removed only [trackedTabId, trackedTabUrl] on untrack, leaving a
 * stale trackedTabTitle behind, and re-implemented track/untrack with raw storage calls.
 * It must use the shared tracked-tab storage helpers (CLAUDE.md rule 18) so all three
 * keys are written/cleared together.
 */

import { beforeEach, describe, mock, test } from 'node:test'
import assert from 'node:assert'

let storageState = {}

function buildChromeMock() {
  return {
    runtime: {
      id: 'test-extension-id',
      sendMessage: mock.fn((_msg, callback) => {
        if (typeof callback === 'function') callback()
        return Promise.resolve()
      }),
      onMessage: { addListener: mock.fn() },
      getManifest: () => ({ version: '0.0.0' })
    },
    storage: {
      local: {
        get: mock.fn((keys, callback) => {
          const result = {}
          const keyList = typeof keys === 'string' ? [keys] : keys
          for (const k of keyList) {
            if (storageState[k] !== undefined) result[k] = storageState[k]
          }
          if (typeof callback === 'function') {
            callback(result)
            return
          }
          return Promise.resolve(result)
        }),
        set: mock.fn((data, callback) => {
          Object.assign(storageState, data)
          if (typeof callback === 'function') {
            callback()
            return
          }
          return Promise.resolve()
        }),
        remove: mock.fn((keys, callback) => {
          const keyList = typeof keys === 'string' ? [keys] : keys
          for (const k of keyList) delete storageState[k]
          if (typeof callback === 'function') {
            callback()
            return
          }
          return Promise.resolve()
        })
      },
      onChanged: { addListener: mock.fn() }
    },
    tabs: {
      query: mock.fn(() => Promise.resolve([{ id: 7, url: 'https://app.example/page', title: 'App Page' }])),
      get: mock.fn((tabId) => Promise.resolve({ id: tabId, windowId: 1 })),
      sendMessage: mock.fn(() => Promise.resolve({ status: 'alive' })),
      update: mock.fn(() => Promise.resolve({ id: 7 })),
      reload: mock.fn(() => Promise.resolve())
    },
    windows: {
      update: mock.fn(() => Promise.resolve())
    }
  }
}

const { handleStopTracking, handleTrackPageClick } = await import('../../extension/popup/tab-tracking-api.js')

describe('popup untrack clears all tracked-tab keys', () => {
  beforeEach(() => {
    storageState = {}
    globalThis.chrome = buildChromeMock()
    globalThis.document = {
      getElementById: mock.fn(() => null),
      addEventListener: mock.fn(),
      querySelector: mock.fn(() => null),
      querySelectorAll: mock.fn(() => [])
    }
  })

  test('handleStopTracking removes trackedTabTitle along with id and url', async () => {
    storageState = {
      trackedTabId: 7,
      trackedTabUrl: 'https://app.example/page',
      trackedTabTitle: 'App Page'
    }

    await handleStopTracking(mock.fn())

    // Regression: trackedTabTitle used to be left behind on popup untrack.
    const removeCalls = globalThis.chrome.storage.local.remove.mock.calls
    assert.ok(removeCalls.length >= 1, 'storage remove should be called')
    const removedKeys = removeCalls.flatMap((c) => (Array.isArray(c.arguments[0]) ? c.arguments[0] : [c.arguments[0]]))
    assert.ok(removedKeys.includes('trackedTabId'), 'trackedTabId must be removed')
    assert.ok(removedKeys.includes('trackedTabUrl'), 'trackedTabUrl must be removed')
    assert.ok(removedKeys.includes('trackedTabTitle'), 'trackedTabTitle must be removed (stale-title regression)')
    assert.strictEqual(storageState.trackedTabTitle, undefined, 'no stale title left in storage')
  })

  test('handleTrackPageClick persists all three tracked-tab keys together', async () => {
    const noop = mock.fn()
    await handleTrackPageClick(noop, noop, noop, noop)

    const setCalls = globalThis.chrome.storage.local.set.mock.calls
    const trackedSet = setCalls.find((c) => c.arguments[0]?.trackedTabId !== undefined)
    assert.ok(trackedSet, 'tracked tab state should be persisted')
    assert.deepStrictEqual(trackedSet.arguments[0], {
      trackedTabId: 7,
      trackedTabUrl: 'https://app.example/page',
      trackedTabTitle: 'App Page'
    })
  })
})
