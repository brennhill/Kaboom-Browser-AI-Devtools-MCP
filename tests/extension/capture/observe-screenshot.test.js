// @ts-nocheck
/**
 * @fileoverview observe-screenshot.test.js — Regression tests for screenshot observe command.
 * Ensures explicit MCP screenshot requests are not blocked by local extension rate-limiting.
 */

import { describe, test, mock } from 'node:test'
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

mock.module('../../../extension/background/index.js', {
  namedExports: {
    debugLog: mockDebugLog
  }
})

mock.module('../../../extension/background/runtime-state/settings-state.js', {
  namedExports: {
    getServerUrl: () => 'http://localhost:7890'
  }
})

mock.module('../../../extension/background/debug.js', {
  namedExports: {
    DebugCategory: { CAPTURE: 'capture' }
  }
})

globalThis.chrome = {
  tabs: {
    // captureVisibleTabSafe queries the currently-active tab so it can restore it
    // afterward. Returning [] means "no previously-active tab to restore", so the
    // capture performs exactly one tabs.update (activating the target tab).
    query: mock.fn(async () => []),
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
  }
}

globalThis.fetch = mock.fn(async () => ({
  ok: true,
  status: 200
}))

await import('../../../extension/background/commands/observe.js')

describe('observe screenshot command', () => {
  test('bypasses local screenshot limiter for explicit observe(screenshot)', async () => {
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

    // Verify tab activation before capture (no window focus — avoid interrupting the user)
    assert.strictEqual(globalThis.chrome.windows.update.mock.calls.length, 0, 'should not focus the window')
    assert.strictEqual(globalThis.chrome.tabs.update.mock.calls.length, 1)
    assert.deepStrictEqual(globalThis.chrome.tabs.update.mock.calls[0].arguments, [123, { active: true }])

    assert.strictEqual(globalThis.chrome.tabs.captureVisibleTab.mock.calls.length, 1)
    assert.strictEqual(mockRecordScreenshot.mock.calls.length, 1)
    assert.strictEqual(globalThis.fetch.mock.calls.length, 1)
    assert.strictEqual(mockCanTakeScreenshot.mock.calls.length, 0, 'local limiter should not gate explicit screenshot')
  })
})
