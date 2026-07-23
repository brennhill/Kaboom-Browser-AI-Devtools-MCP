// @ts-nocheck
/**
 * @fileoverview registry_csp_navigation.test.js — Restricted-page handling for browser actions.
 */

import { beforeEach, describe, test } from 'node:test'
import assert from 'node:assert'

let queuedResults = []
let updateCalls = []
let sendMessageCalls = []
let storageState = {}
let tabsByID = new Map()
let activeTabs = []
let createCalls = []
let createShouldFail = false

function resetHarness() {
  queuedResults = []
  updateCalls = []
  sendMessageCalls = []
  storageState = {}
  tabsByID = new Map()
  activeTabs = []
  createCalls = []
  createShouldFail = false
}

function makeSyncClient() {
  return {
    queueCommandResult(result) {
      queuedResults.push(result)
    }
  }
}

globalThis.chrome = {
  runtime: {
    sendMessage: async () => ({}),
    getManifest: () => ({ version: '0.0.0-test' }),
    onMessage: { addListener: () => {} },
    onInstalled: { addListener: () => {} }
  },
  action: {
    setBadgeText: () => {},
    setBadgeBackgroundColor: () => {}
  },
  storage: {
    local: {
      get: (_keys, callback) => {
        if (typeof callback === 'function') {
          callback({ ...storageState })
          return
        }
        return Promise.resolve({ ...storageState })
      },
      set: (_data, callback) => {
        storageState = { ...storageState, ..._data }
        if (callback) {
          callback()
          return
        }
        return Promise.resolve()
      },
      remove: (_keys, callback) => {
        if (callback) {
          callback()
        }
        return Promise.resolve()
      }
    },
    onChanged: { addListener: () => {} }
  },
  alarms: {
    create: () => {},
    onAlarm: { addListener: () => {} }
  },
  commands: {
    onCommand: { addListener: () => {} }
  },
  tabs: {
    get: async (tabId) => {
      const tab = tabsByID.get(tabId)
      if (!tab) {
        throw new Error(`No tab with id ${tabId}`)
      }
      return { ...tab }
    },
    query: async (queryInfo) => {
      if (queryInfo && queryInfo.active) {
        return activeTabs.map((tab) => ({ ...tab }))
      }
      return Array.from(tabsByID.values()).map((tab) => ({ ...tab }))
    },
    update: async (tabId, props) => {
      updateCalls.push({ tabId, props })
      const current = tabsByID.get(tabId) || { id: tabId, status: 'complete', title: '' }
      const next = { ...current, url: props.url || current.url, status: 'complete' }
      tabsByID.set(tabId, next)
      return { ...next }
    },
    create: async ({ url, active }) => {
      if (createShouldFail) {
        throw new Error('create failed')
      }
      const id = 900 + createCalls.length
      createCalls.push({ url, active, id })
      const tab = { id, url, status: 'complete', title: 'Example' }
      tabsByID.set(id, tab)
      activeTabs = [tab]
      return { ...tab }
    },
    reload: async (_tabId) => {},
    goBack: async () => {},
    goForward: async () => {},
    sendMessage: async (tabId, message) => {
      sendMessageCalls.push({ tabId, message })
      if (message?.type === 'gasoline_ping') {
        return { status: 'alive' }
      }
      return {}
    },
    onRemoved: { addListener: () => {} },
    onUpdated: { addListener: () => {} }
  },
  scripting: {
    executeScript: async () => []
  }
}

globalThis.fetch = async () => ({ ok: true, json: async () => ({}) })

const { handlePendingQuery } = await import('./pending-queries.js')
const { markInitComplete } = await import('./state.js')

beforeEach(() => {
  resetHarness()
  markInitComplete()
})

describe('Restricted/CSP page handling', () => {
  test('browser_action navigate bypasses restricted-page gate to escape', async () => {
    const trackedTab = { id: 7, url: 'chrome://extensions', status: 'complete', title: 'Extensions' }
    tabsByID.set(7, trackedTab)
    activeTabs = [trackedTab]
    storageState = {
      trackedTabId: 7,
      trackedTabUrl: trackedTab.url,
      trackedTabTitle: trackedTab.title
    }

    await handlePendingQuery(
      {
        id: 'q-nav',
        type: 'browser_action',
        correlation_id: 'corr-nav',
        params: { action: 'navigate', url: 'https://example.com' }
      },
      makeSyncClient()
    )

    assert.strictEqual(updateCalls.length, 1, 'navigate should call tabs.update')
    assert.strictEqual(updateCalls[0].tabId, 7)
    assert.strictEqual(queuedResults.length, 1)
    assert.strictEqual(queuedResults[0].status, 'complete')
    assert.strictEqual(queuedResults[0].result.success, true)
    assert.strictEqual(queuedResults[0].result.target_context.source, 'tracked_tab')
  })

  test('browser_action what=navigate bypasses restricted-page gate to escape', async () => {
    const trackedTab = { id: 8, url: 'chrome://extensions', status: 'complete', title: 'Extensions' }
    tabsByID.set(8, trackedTab)
    activeTabs = [trackedTab]
    storageState = {
      trackedTabId: 8,
      trackedTabUrl: trackedTab.url,
      trackedTabTitle: trackedTab.title
    }

    await handlePendingQuery(
      {
        id: 'q-nav-what',
        type: 'browser_action',
        correlation_id: 'corr-nav-what',
        params: { what: 'navigate', url: 'https://example.com' }
      },
      makeSyncClient()
    )

    assert.strictEqual(updateCalls.length, 1, 'navigate should call tabs.update')
    assert.strictEqual(updateCalls[0].tabId, 8)
    assert.strictEqual(queuedResults.length, 1)
    assert.strictEqual(queuedResults[0].status, 'complete')
    assert.strictEqual(queuedResults[0].result.success, true)
    assert.strictEqual(queuedResults[0].result.target_context.source, 'tracked_tab')
  })

  test('non-browser actions on restricted pages return explicit CSP error', async () => {
    const trackedTab = { id: 9, url: 'chrome://extensions', status: 'complete', title: 'Extensions' }
    tabsByID.set(9, trackedTab)
    activeTabs = [trackedTab]
    storageState = {
      trackedTabId: 9,
      trackedTabUrl: trackedTab.url,
      trackedTabTitle: trackedTab.title
    }

    await handlePendingQuery(
      {
        id: 'q-exec',
        type: 'execute',
        correlation_id: 'corr-exec',
        params: { script: '1+1' }
      },
      makeSyncClient()
    )

    assert.strictEqual(queuedResults.length, 1)
    assert.strictEqual(queuedResults[0].status, 'error')
    assert.strictEqual(queuedResults[0].error, 'csp_blocked_page')
    assert.strictEqual(queuedResults[0].result.error, 'csp_blocked_page')
    assert.strictEqual(queuedResults[0].result.csp_blocked, true)
    assert.strictEqual(queuedResults[0].result.failure_cause, 'csp')
  })

  test('browser_action navigate falls back to active tab when tracking is missing', async () => {
    const activeTab = { id: 11, url: 'chrome://settings', status: 'complete', title: 'Settings' }
    tabsByID.set(11, activeTab)
    activeTabs = [activeTab]
    storageState = {}

    await handlePendingQuery(
      {
        id: 'q-nav-fallback',
        type: 'browser_action',
        correlation_id: 'corr-nav-fallback',
        params: { action: 'navigate', url: 'https://example.com/path' }
      },
      makeSyncClient()
    )

    assert.strictEqual(createCalls.length, 1, 'fallback should open a trackable tab when only internal pages exist')
    assert.strictEqual(updateCalls.length, 1, 'navigate should run on the newly opened trackable tab')
    assert.strictEqual(updateCalls[0].tabId, createCalls[0].id)
    assert.strictEqual(queuedResults.length, 1)
    assert.strictEqual(queuedResults[0].status, 'complete')
    assert.strictEqual(queuedResults[0].result.success, true)
    assert.strictEqual(queuedResults[0].result.target_context.source, 'auto_tracked_new_tab')
  })

  test('missing tracking + active non-internal tab auto-tracks active tab', async () => {
    const activeTab = { id: 21, url: 'https://news.ycombinator.com', status: 'complete', title: 'HN' }
    tabsByID.set(activeTab.id, activeTab)
    activeTabs = [activeTab]
    storageState = {}

    await handlePendingQuery(
      {
        id: 'q-nav-active-track',
        type: 'browser_action',
        correlation_id: 'corr-nav-active-track',
        params: { action: 'navigate', url: 'https://example.com/path' }
      },
      makeSyncClient()
    )

    assert.strictEqual(createCalls.length, 0, 'active non-internal tab should not require creating a new tab')
    assert.strictEqual(updateCalls.length, 1, 'navigate should execute directly on the active tab')
    assert.strictEqual(updateCalls[0].tabId, 21)
    assert.strictEqual(queuedResults.length, 1)
    assert.strictEqual(queuedResults[0].status, 'complete')
    assert.strictEqual(queuedResults[0].result.target_context.source, 'auto_tracked_active_tab')
    assert.strictEqual(storageState.trackedTabId, 21)
  })

  test('missing tracking + active internal tab switches to random non-internal tab', async () => {
    const internalTab = { id: 31, url: 'chrome://settings', status: 'complete', title: 'Settings' }
    const normalTab = { id: 32, url: 'https://example.org/workspace', status: 'complete', title: 'Workspace' }
    tabsByID.set(internalTab.id, internalTab)
    tabsByID.set(normalTab.id, normalTab)
    activeTabs = [internalTab]
    storageState = {}

    await handlePendingQuery(
      {
        id: 'q-nav-random-track',
        type: 'browser_action',
        correlation_id: 'corr-nav-random-track',
        params: { action: 'navigate', url: 'https://example.com/path' }
      },
      makeSyncClient()
    )

    assert.strictEqual(createCalls.length, 0, 'should reuse an existing non-internal tab instead of opening a new tab')
    assert.strictEqual(updateCalls.length, 2, 'should activate candidate tab, then navigate on it')
    assert.strictEqual(updateCalls[0].tabId, 32)
    assert.deepStrictEqual(updateCalls[0].props, { active: true })
    assert.strictEqual(updateCalls[1].tabId, 32)
    assert.strictEqual(queuedResults.length, 1)
    assert.strictEqual(queuedResults[0].status, 'complete')
    assert.strictEqual(queuedResults[0].result.target_context.source, 'auto_tracked_random_tab')
    assert.strictEqual(storageState.trackedTabId, 32)
  })

  test('missing tracking + no recoverable tabs returns explicit no_trackable_tab error', async () => {
    activeTabs = []
    tabsByID = new Map()
    storageState = {}
    createShouldFail = true

    await handlePendingQuery(
      {
        id: 'q-exec-no-trackable',
        type: 'execute',
        correlation_id: 'corr-exec-no-trackable',
        params: { script: '1+1' }
      },
      makeSyncClient()
    )

    assert.strictEqual(queuedResults.length, 1)
    assert.strictEqual(queuedResults[0].status, 'error')
    assert.strictEqual(queuedResults[0].error, 'no_trackable_tab')
    assert.strictEqual(queuedResults[0].result.error, 'no_trackable_tab')
    assert.ok(Array.isArray(queuedResults[0].result.attempted_recovery))
    assert.ok(queuedResults[0].result.attempted_recovery.length >= 3)
  })
})
