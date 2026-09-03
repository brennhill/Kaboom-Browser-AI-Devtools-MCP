// @ts-nocheck
/**
 * @fileoverview performance-trace-target-refusal.test.js — What happens when Chrome
 * refuses to attach the debugger to the resolved target.
 *
 * Live reproduction behind kaboom-kedt.2.6.2.3: a tracked tab sat on the HTTP
 * fixture and ran content scripts, yet `chrome.debugger.attach` kept failing with
 * "Cannot access a chrome-extension:// URL of different extension".
 *
 * A profile names one tab. Silently moving to a different tab would hand back an
 * artifact attributed to a page nobody asked about — worse than no artifact. So the
 * refusal is reported against the tab that was refused and is never retried, and
 * tracking is left alone: only CDP attach was refused, and the same tab still serves
 * every other tool (execute_js, get_text, click) perfectly well.
 */

import { beforeEach, describe, test, mock } from 'node:test'
import assert from 'node:assert/strict'
import { MANIFEST_VERSION } from '../shared/helpers.js'

const TRACKED_TAB_ID = 41
const CHROME_REFUSAL = 'Cannot access a chrome-extension:// URL of different extension'

let storageState
let attachCalls
let detachCalls
let daemonRequests

function buildChrome() {
  storageState = {
    serverUrl: 'http://localhost:7890',
    aiWebPilotEnabled: true,
    trackedTabId: TRACKED_TAB_ID,
    trackedTabUrl: 'https://app.example.com/fixture',
    trackedTabTitle: 'Fixture'
  }
  attachCalls = []
  detachCalls = []
  return {
    runtime: {
      onMessage: { addListener: mock.fn() },
      onInstalled: { addListener: mock.fn() },
      sendMessage: mock.fn(() => Promise.resolve()),
      getManifest: () => ({ version: MANIFEST_VERSION })
    },
    action: { setBadgeText: mock.fn(), setBadgeBackgroundColor: mock.fn() },
    tabs: {
      get: mock.fn((tabId) =>
        Promise.resolve({ id: tabId, windowId: 1, status: 'complete', url: 'https://app.example.com/fixture' })
      ),
      query: mock.fn((query, callback) => {
        const result = query?.active
          ? [{ id: 77, windowId: 1, url: 'https://app.example.com/other', title: 'Other' }]
          : []
        if (callback) callback(result)
        return Promise.resolve(result)
      }),
      sendMessage: mock.fn(() => Promise.resolve({ success: true })),
      update: mock.fn(() => Promise.resolve()),
      create: mock.fn(() => Promise.resolve({ id: 99 })),
      remove: mock.fn(() => Promise.resolve()),
      onRemoved: { addListener: mock.fn() }
    },
    debugger: {
      attach: mock.fn((target) => {
        attachCalls.push(target.tabId)
        return Promise.reject(new Error(CHROME_REFUSAL))
      }),
      detach: mock.fn((target) => {
        detachCalls.push(target.tabId)
        return Promise.resolve()
      }),
      // Chrome rejects every command on an unattached target. Attach always fails in this
      // fixture, so nothing is ever attached — and the session manager's adoption probe must
      // see that. A fake that resolves here would report the refused tab as already attached.
      sendCommand: mock.fn(() =>
        Promise.reject(new Error(`Debugger is not attached to the tab with id: ${TRACKED_TAB_ID}`))
      ),
      onEvent: { addListener: mock.fn() },
      onDetach: { addListener: mock.fn() }
    },
    scripting: { executeScript: mock.fn(() => Promise.resolve([{ result: {} }])) },
    storage: {
      local: {
        get: mock.fn((keys, callback) => {
          const data = { ...storageState }
          if (callback) callback(data)
          return Promise.resolve(data)
        }),
        set: mock.fn((data, callback) => {
          Object.assign(storageState, data)
          if (callback) callback()
          return Promise.resolve()
        }),
        remove: mock.fn((keys, callback) => {
          for (const key of typeof keys === 'string' ? [keys] : keys) delete storageState[key]
          if (callback) callback()
          return Promise.resolve()
        })
      },
      sync: {
        get: mock.fn((keys, callback) => {
          if (callback) callback({})
          return Promise.resolve({})
        }),
        set: mock.fn(() => Promise.resolve())
      },
      session: {
        get: mock.fn((keys, callback) => {
          if (callback) callback({})
          return Promise.resolve({})
        }),
        set: mock.fn(() => Promise.resolve())
      },
      onChanged: { addListener: mock.fn() }
    },
    alarms: { create: mock.fn(), onAlarm: { addListener: mock.fn() } }
  }
}

async function startTrace(params) {
  const syncClient = { queueCommandResult: mock.fn() }
  const { handlePendingQuery } = await import('../../../extension/background/pending-queries.js')
  const { markInitComplete } = await import('../../../extension/background/runtime-state/startup-state.js')
  const { resetPilotCacheForTesting } = await import('../../../extension/background/runtime-state/pilot-state.js')
  markInitComplete()
  resetPilotCacheForTesting(true)
  await handlePendingQuery(
    {
      id: 'q-trace',
      type: 'performance_trace',
      correlation_id: 'corr-trace',
      params: JSON.stringify({ action: 'start', ...params })
    },
    syncClient
  )
  return syncClient.queueCommandResult.mock.calls[0].arguments[0]
}

describe('performance trace target refusal', () => {
  beforeEach(() => {
    mock.reset()
    daemonRequests = []
    globalThis.chrome = buildChrome()
    globalThis.fetch = mock.fn((url) => {
      daemonRequests.push(String(url))
      return Promise.resolve({
        ok: true,
        json: () => Promise.resolve({ trace_id: 'trace-1', queries: [] })
      })
    })
  })

  for (const named of [false, true]) {
    const label = named ? 'a tab the caller named' : 'the tracked tab'
    test(`names the refused tab and keeps tracking when the target is ${label}`, async () => {
      const queued = await startTrace(named ? { tab_id: TRACKED_TAB_ID } : {})

      assert.equal(attachCalls[0], TRACKED_TAB_ID)
      assert.equal(queued.result.error, 'performance_trace_target_not_debuggable')
      assert.equal(queued.result.tab_id, TRACKED_TAB_ID, 'the refusal must name the tab that was refused')
      // Attaching is a stage of acquiring the tab's exclusive CDP lease, so the refusal is
      // reported against CDPSession.acquire. What matters is unchanged: the message names
      // the stage, the refused tab, and what the caller can do instead.
      assert.match(queued.result.message, /CDPSession\.acquire/)
      assert.match(queued.result.message, /chrome-extension/)
      assert.equal(
        queued.result.retryable,
        false,
        'Chrome will refuse this target again; a retry can only waste the caller time'
      )
      assert.equal(
        storageState.trackedTabId,
        TRACKED_TAB_ID,
        'only CDP attach was refused — the tab still serves every other tool, so tracking must survive'
      )
      assert.ok(
        daemonRequests.some((url) => url.endsWith('/performance-trace/abort')),
        'the daemon artifact opened for the failed start must be aborted'
      )
    })
  }

  test('never profiles a tab other than the one that was targeted', async () => {
    await startTrace({})

    assert.deepEqual(
      attachCalls,
      [TRACKED_TAB_ID],
      'a refused target must not be swapped for another tab: the artifact would describe a page nobody asked about'
    )
  })
})
