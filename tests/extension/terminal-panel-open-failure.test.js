// @ts-nocheck
/**
 * @fileoverview terminal-panel-open-failure.test.js — openTerminalPanel must not
 * swallow the reason the side panel refused to open.
 *
 * Regression: the bridge did `catch { return false }` and the caller did
 * `void openTerminalPanel()`, so a rejected `chrome.sidePanel.open()` produced
 * no console output, no toast, and no captured error. Clicking the launcher's
 * Terminal button was indistinguishable from clicking a dead element, and there
 * was nothing to diagnose from.
 *
 * console.error (not warn) is deliberate: the daemon's capture buffer collects
 * page errors, so a failure here is retrievable via observe(what:"errors").
 */

import { beforeEach, describe, mock, test } from 'node:test'
import assert from 'node:assert'

let importCounter = 0
let sendMessage

async function loadBridge() {
  return import(`../../extension/content/ui/terminal-panel-bridge.js?v=${++importCounter}`)
}

describe('openTerminalPanel failure reporting', () => {
  beforeEach(() => {
    mock.reset()
    sendMessage = mock.fn(async () => ({ success: true }))
    globalThis.chrome = {
      runtime: { id: 'test-ext', sendMessage: (...args) => sendMessage(...args), getURL: (p) => p },
      storage: {
        session: { get: mock.fn(async () => ({})) },
        onChanged: { addListener: mock.fn() }
      }
    }
  })

  test('a successful open reports no error', async () => {
    const err = mock.method(console, 'error', () => {})
    const { openTerminalPanel } = await loadBridge()

    assert.strictEqual(await openTerminalPanel(), true)
    assert.strictEqual(err.mock.calls.length, 0)
  })

  test('a rejected open surfaces the Chrome error verbatim', async () => {
    sendMessage = mock.fn(async () => ({
      success: false,
      error: '`sidePanel.open()` may only be called in response to a user gesture.'
    }))
    const err = mock.method(console, 'error', () => {})
    const { openTerminalPanel } = await loadBridge()

    assert.strictEqual(await openTerminalPanel(), false)
    assert.strictEqual(err.mock.calls.length, 1, 'the failure must be observable')
    assert.match(err.mock.calls[0].arguments.join(' '), /user gesture/,
      'the Chrome error is the only diagnostic signal — it must not be replaced')
  })

  test('a missing response is reported rather than silently treated as failure', async () => {
    sendMessage = mock.fn(async () => undefined)
    const err = mock.method(console, 'error', () => {})
    const { openTerminalPanel } = await loadBridge()

    assert.strictEqual(await openTerminalPanel(), false)
    assert.strictEqual(err.mock.calls.length, 1)
  })

  test('a thrown sendMessage is reported', async () => {
    sendMessage = mock.fn(async () => { throw new Error('Extension context invalidated.') })
    const err = mock.method(console, 'error', () => {})
    const { openTerminalPanel } = await loadBridge()

    assert.strictEqual(await openTerminalPanel(), false)
    assert.strictEqual(err.mock.calls.length, 1)
    assert.match(err.mock.calls[0].arguments.join(' '), /Extension context invalidated/)
  })
})
