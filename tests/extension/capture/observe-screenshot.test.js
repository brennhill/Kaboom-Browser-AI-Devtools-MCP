// @ts-nocheck
/**
 * @fileoverview observe-screenshot.test.js — Regression tests for screenshot observe command.
 *
 * Two contracts:
 *  1. An explicit MCP screenshot request is never blocked by the local extension rate limiter.
 *  2. It captures over the tab's CDP lease and leaves the user's foreground alone. The
 *     previous behaviour activated the target tab, captured, and switched back — the user's
 *     window jumped once per screenshot and their in-progress typing lost focus.
 */

import { describe, test, mock, after } from 'node:test'
import assert from 'node:assert'

const registered = new Map()
const mockRegisterCommand = mock.fn((name, handler) => {
  registered.set(name, handler)
})

const mockCanTakeScreenshot = mock.fn(() => ({
  allowed: false,
  reason: 'rate_limit',
  nextAllowedIn: 5000
}))
const mockRecordScreenshot = mock.fn()
const mockDebugLog = mock.fn()

mock.module('../../../extension/background/commands/registry.js', {
  namedExports: {
    registerCommand: mockRegisterCommand
  }
})

mock.module('../../../extension/background/caches/cache-limits.js', {
  namedExports: {
    canTakeScreenshot: mockCanTakeScreenshot,
    recordScreenshot: mockRecordScreenshot
  }
})

mock.module('../../../extension/background/runtime-state/settings-state.js', {
  namedExports: {
    getServerUrl: () => 'http://localhost:7890'
  }
})

mock.module('../../../extension/background/debug.js', {
  namedExports: {
    DebugCategory: { CAPTURE: 'capture' },
    debugLog: mockDebugLog
  }
})

const cdpSends = []
const attachedTabs = new Set()

globalThis.chrome = {
  tabs: {
    // Tab 456 is the one the user is looking at; the capture targets tab 123.
    query: mock.fn(async () => [{ id: 456, windowId: 11 }]),
    get: mock.fn(async () => ({ windowId: 11, url: 'https://www.linkedin.com/feed/' })),
    update: mock.fn(async () => ({})),
    captureVisibleTab: mock.fn(async () => 'data:image/jpeg;base64,Zm9v')
  },
  windows: {
    update: mock.fn(async () => ({}))
  },
  // setKaboomOverlayVisibility hides/shows Kaboom overlays via executeScript before capture.
  scripting: {
    executeScript: mock.fn(async () => [])
  },
  debugger: {
    attach: async (target) => {
      attachedTabs.add(target.tabId)
    },
    detach: async (target) => {
      attachedTabs.delete(target.tabId)
    },
    sendCommand: async (target, method, params) => {
      cdpSends.push({ method, params })
      if (!attachedTabs.has(target.tabId)) {
        throw new Error(`Debugger is not attached to the tab with id: ${target.tabId}`)
      }
      if (method === 'Page.getLayoutMetrics') {
        return { cssVisualViewport: { pageX: 0, pageY: 0, clientWidth: 800, clientHeight: 600 } }
      }
      if (method === 'Runtime.evaluate') return { result: { value: 1 } }
      if (method === 'Page.captureScreenshot') return { data: 'Zm9v' }
      return {}
    },
    onDetach: { addListener: () => {} }
  }
}

globalThis.fetch = mock.fn(async () => ({
  ok: true,
  status: 200
}))

await import('../../../extension/background/commands/observe.js')
const { cdpSessions } = await import('../../../extension/background/dom/cdp/cdp-session.js')

// The warm session arms a 30s idle-detach timer; leaving it would hold the process open.
after(() => cdpSessions()?.abort(123, 'test_teardown'))

describe('observe screenshot command', () => {
  test('captures in the background without touching the user’s foreground', async () => {
    const handler = registered.get('screenshot')
    assert.ok(handler, 'screenshot handler should be registered')

    const sendResult = mock.fn()
    await handler({
      tabId: 123,
      query: { id: 'q-1' },
      params: {},
      sendResult
    })

    assert.strictEqual(sendResult.mock.calls.length, 0, 'success path should resolve via server/query_id')
    assert.strictEqual(globalThis.chrome.tabs.get.mock.calls.length, 1)

    // No tab activation and no window focus: the person using the browser is not interrupted.
    assert.strictEqual(globalThis.chrome.windows.update.mock.calls.length, 0, 'should not focus the window')
    assert.strictEqual(
      globalThis.chrome.tabs.update.mock.calls.length,
      0,
      'the ordinary capture path must never activate the tab it is capturing'
    )
    assert.strictEqual(
      globalThis.chrome.tabs.captureVisibleTab.mock.calls.length,
      0,
      'captureVisibleTab is reserved for contexts with no chrome.debugger'
    )

    assert.ok(
      cdpSends.some((c) => c.method === 'Page.captureScreenshot'),
      'the image comes from Page.captureScreenshot over the tab lease'
    )
    assert.ok(
      cdpSends.some((c) => c.method === 'Emulation.setFocusEmulationEnabled' && c.params.enabled === true),
      'the captured tab is made to behave as focused'
    )

    assert.strictEqual(mockRecordScreenshot.mock.calls.length, 1)
    assert.strictEqual(globalThis.fetch.mock.calls.length, 1)
    assert.strictEqual(mockCanTakeScreenshot.mock.calls.length, 0, 'local limiter should not gate explicit screenshot')
  })
})
