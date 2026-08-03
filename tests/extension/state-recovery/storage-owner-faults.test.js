// storage-owner-faults.test.js — Exercises extension state owners with canonical failure scenarios.
import assert from 'node:assert/strict'
import { afterEach, mock, test } from 'node:test'

import { createStorageFaultScenario } from './storage-fault-fixture.js'

const originalChrome = globalThis.chrome
afterEach(() => {
  globalThis.chrome = originalChrome
  mock.restoreAll()
})

function installStorageRead(result) {
  const transitions = []
  globalThis.chrome = {
    runtime: {
      sendMessage: mock.fn((message) => {
        transitions.push(message)
        return Promise.resolve()
      })
    },
    storage: {
      local: {
        get: mock.fn(() => (result instanceof Error ? Promise.reject(result) : Promise.resolve(result))),
        set: mock.fn(() => Promise.resolve()),
        remove: mock.fn(() => Promise.resolve())
      }
    }
  }
  return transitions
}

async function loadTrackedTabStorage() {
  return import(`../../../extension/lib/tabs/tracked-tab-storage.js?fault=${Date.now()}-${Math.random()}`)
}

async function loadSettingsStorage() {
  return import(`../../../extension/background/ui/settings-storage.js?fault=${Date.now()}-${Math.random()}`)
}

test('tracked-tab owner classifies read, quota, and cancellation failures without private state', async () => {
  for (const kind of ['read', 'quota', 'cancellation']) {
    const scenario = createStorageFaultScenario(kind, 'private-tab-url')
    const transitions = installStorageRead(scenario.error)
    const warn = mock.method(console, 'warn', () => undefined)
    const { readTrackedTab } = await loadTrackedTabStorage()

    assert.deepEqual(await readTrackedTab(), {})
    assert.equal(transitions.at(-1).diagnostic.name, 'tracked_tab_state')
    assert.match(transitions.at(-1).diagnostic.detail, new RegExp(`${kind} failure`))
    assert.doesNotMatch(JSON.stringify(transitions), /private-tab-url/)
    assert.equal(warn.mock.calls.length, 1)
    warn.mock.restore()
  }
})

test('tracked-tab owner distinguishes corrupt and partial snapshots', async () => {
  for (const kind of ['corruption', 'partial_write']) {
    const scenario = createStorageFaultScenario(kind, 'private-tab-title')
    const stored = scenario.storedValue({ trackedTabId: 'invalid', trackedTabUrl: 'https://private.example/' })
    const transitions = installStorageRead(stored)
    mock.method(console, 'warn', () => undefined)
    const { readTrackedTab } = await loadTrackedTabStorage()

    assert.deepEqual(await readTrackedTab(), {})
    assert.equal(transitions.at(-1).diagnostic.name, 'tracked_tab_state')
    assert.match(transitions.at(-1).diagnostic.detail, /corruption failure/)
    assert.doesNotMatch(JSON.stringify(transitions), /private-tab-title/)
  }
})

test('settings owner classifies operational failures and logs only redacted context', async () => {
  for (const kind of ['read', 'quota', 'cancellation']) {
    const scenario = createStorageFaultScenario(kind, 'private-setting-value')
    installStorageRead(scenario.error)
    const warn = mock.method(console, 'warn', () => undefined)
    const { loadSavedSettings } = await loadSettingsStorage()

    assert.deepEqual(await loadSavedSettings(), {})
    const logged = warn.mock.calls.map((call) => call.arguments.join(' ')).join('\n')
    assert.match(logged, new RegExp(`${kind} failure`))
    assert.doesNotMatch(logged, /private-setting-value/)
    warn.mock.restore()
  }
})

test('settings owner reports malformed and partial snapshots as corruption', async () => {
  for (const kind of ['corruption', 'partial_write']) {
    const scenario = createStorageFaultScenario(kind, 'private-server-url')
    installStorageRead(scenario.storedValue({ serverUrl: 42, debugMode: 'invalid' }))
    const warn = mock.method(console, 'warn', () => undefined)
    const { loadSavedSettings } = await loadSettingsStorage()

    assert.deepEqual(await loadSavedSettings(), {})
    const logged = warn.mock.calls.map((call) => call.arguments.join(' ')).join('\n')
    assert.match(logged, /corruption failure/)
    assert.doesNotMatch(logged, /private-server-url/)
    warn.mock.restore()
  }
})

test('tracked-tab writes retain the prior durable value and publish classified recovery', async () => {
  for (const kind of ['write', 'quota', 'cancellation']) {
    const scenario = createStorageFaultScenario(kind, 'private-tab-write')
    const transitions = installStorageRead({})
    globalThis.chrome.storage.local.set = mock.fn(() => Promise.reject(scenario.error))
    const warn = mock.method(console, 'warn', () => undefined)
    const { setTrackedTab } = await loadTrackedTabStorage()

    await assert.rejects(setTrackedTab({ id: 42, url: 'https://example.test/', title: 'Example' }))
    assert.equal(transitions.at(-1).diagnostic.name, 'extension_storage_write_state')
    assert.match(transitions.at(-1).diagnostic.detail, new RegExp(`${kind} failure`))
    assert.doesNotMatch(JSON.stringify(transitions), /private-tab-write/)
    assert.ok(warn.mock.calls.length >= 1)
    warn.mock.restore()
  }
})
