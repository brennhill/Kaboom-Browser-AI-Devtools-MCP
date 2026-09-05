// @ts-nocheck
/**
 * @fileoverview visible-tab-capture-fallback.test.js — The one path that may still take the
 * foreground.
 *
 * `chrome.tabs.captureVisibleTab` photographs whatever is visible in the window; it cannot
 * see a background tab at all. So when `chrome.debugger` is unavailable — a constrained
 * extension context, or a build without the debugger permission — the only way to get the
 * right pixels is to activate the tab and hand the foreground straight back.
 *
 * This file exists in isolation because the CDP session manager is a process-wide singleton
 * memoised on the first `cdpSessions()` call: a test that needs `chrome.debugger` absent
 * cannot share a process with one that needs it present, and node:test gives each FILE its
 * own process. Both background-capture files live in this subfolder rather than in
 * tests/extension/capture so that folder stays inside the 10-file budget.
 */

import { describe, test, mock, beforeEach } from 'node:test'
import assert from 'node:assert'

globalThis.chrome = {
  tabs: {
    query: mock.fn(async () => [{ id: 99, windowId: 11 }]),
    get: mock.fn(async (tabId) => ({ id: tabId, windowId: 11, url: 'https://example.test/' })),
    update: mock.fn(async () => ({})),
    captureVisibleTab: mock.fn(async () => 'data:image/jpeg;base64,VklTSUJMRQ==')
  },
  scripting: { executeScript: mock.fn(async () => []) }
  // No chrome.debugger: cdpSessions() must report itself unavailable rather than throw.
}

const { captureTabImage } = await import('../../../../extension/background/ui/tracked-tab-state.js')

describe('captureTabImage — no-debugger fallback', () => {
  beforeEach(() => {
    globalThis.chrome.tabs.update.mock.resetCalls()
    globalThis.chrome.tabs.captureVisibleTab.mock.resetCalls()
    globalThis.chrome.scripting.executeScript.mock.resetCalls()
  })

  test('captures the visible tab and restores the tab the user had', async () => {
    const dataUrl = await captureTabImage(7, 11, { format: 'jpeg', quality: 80 })
    assert.strictEqual(dataUrl, 'data:image/jpeg;base64,VklTSUJMRQ==')
    assert.deepStrictEqual(globalThis.chrome.tabs.captureVisibleTab.mock.calls[0].arguments, [
      11,
      { format: 'jpeg', quality: 80 }
    ])
    assert.deepStrictEqual(
      globalThis.chrome.tabs.update.mock.calls.map((c) => c.arguments),
      [
        [7, { active: true }],
        [99, { active: true }]
      ],
      'borrow the foreground, then give it back'
    )
  })

  test('does not touch the foreground when the target tab is already the visible one', async () => {
    globalThis.chrome.tabs.query.mock.mockImplementationOnce(async () => [{ id: 7, windowId: 11 }])
    await captureTabImage(7, 11, { format: 'jpeg', quality: 80 })
    assert.strictEqual(globalThis.chrome.tabs.update.mock.calls.length, 0)
  })

  test('still strips Kaboom overlays before the capture', async () => {
    await captureTabImage(7, 11, { format: 'jpeg', quality: 80 })
    assert.deepStrictEqual(
      globalThis.chrome.scripting.executeScript.mock.calls.map((c) => c.arguments[0].args[0]),
      [false, true]
    )
  })
})
