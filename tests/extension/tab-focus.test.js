// @ts-nocheck
/**
 * @fileoverview tab-focus.test.js — locks the contract of the shared
 * focusTabAndWindow() helper (DRY extraction). Three entry points (MCP
 * activate_tab, popup URL click, background tab-state) route "bring a tab to the
 * foreground" through this one function, so the activate-then-focus-window
 * sequence and the returned tab must stay exactly as the call sites expect.
 */

import { beforeEach, describe, test, mock } from 'node:test'
import assert from 'node:assert'

let calls

function installChrome(updateReturn) {
  calls = []
  globalThis.chrome = {
    tabs: {
      update: mock.fn(async (id, opts) => { calls.push(['tabs.update', id, opts]); return updateReturn }),
      get: mock.fn(async (id) => { calls.push(['tabs.get', id]); return { id, windowId: 99, url: 'u', title: 't' } })
    },
    windows: {
      update: mock.fn(async (winId, opts) => { calls.push(['windows.update', winId, opts]) })
    }
  }
}

const { focusTabAndWindow } = await import('../../extension/lib/tab-focus.js')

describe('focusTabAndWindow', () => {
  beforeEach(() => { mock.reset() })

  test('activates the tab, then focuses its window, and returns the updated tab', async () => {
    installChrome({ id: 7, windowId: 42, url: 'x', title: 'y' })
    const tab = await focusTabAndWindow(7)

    assert.deepStrictEqual(calls[0], ['tabs.update', 7, { active: true }])
    assert.deepStrictEqual(calls[1], ['windows.update', 42, { focused: true }])
    assert.strictEqual(tab.url, 'x', 'returns the tab from the update result (no redundant get)')
    assert.strictEqual(globalThis.chrome.tabs.get.mock.calls.length, 0)
  })

  test('falls back to a fresh get when update returns nothing', async () => {
    installChrome(undefined)
    const tab = await focusTabAndWindow(5)

    assert.strictEqual(globalThis.chrome.tabs.get.mock.calls.length, 1)
    assert.deepStrictEqual(calls.at(-1), ['windows.update', 99, { focused: true }])
    assert.strictEqual(tab.windowId, 99)
  })

  test('skips window focus when the tab has no windowId', async () => {
    installChrome({ id: 3, windowId: undefined })
    await focusTabAndWindow(3)

    assert.strictEqual(globalThis.chrome.windows.update.mock.calls.length, 0)
  })

  test('propagates an activate failure (callers wrap best-effort themselves)', async () => {
    installChrome({ id: 1 })
    globalThis.chrome.tabs.update = mock.fn(async () => { throw new Error('no such tab') })
    await assert.rejects(() => focusTabAndWindow(1), /no such tab/)
  })
})
