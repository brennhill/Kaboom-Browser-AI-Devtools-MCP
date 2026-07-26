// @ts-nocheck
/**
 * @fileoverview terminal-panel-gesture-entrypoints.test.js — The terminal side
 * panel must be reachable from entry points Chrome grants a full user gesture.
 *
 * Background: `chrome.sidePanel.open()` requires an active user gesture. Chrome
 * gives `runtime.onMessage` listeners only a *restricted* gesture, which
 * sidePanel.open() rejects on some Chrome/Brave builds (crbug 355266358) — so the
 * in-page launcher button alone is not a dependable way in. `commands.onCommand`
 * and `contextMenus.onClicked` get a full gesture and hand us the tab
 * synchronously, so both must exist and both must reach the same shared opener.
 */

import { beforeEach, describe, mock, test } from 'node:test'
import assert from 'node:assert'
import { readFileSync } from 'node:fs'

let importCounter = 0
let openCalls
let commandListener
let sidePanelOpen

function installChrome() {
  openCalls = []
  commandListener = null
  sidePanelOpen = mock.fn(async ({ tabId }) => { openCalls.push(tabId) })
  globalThis.chrome = {
    runtime: { id: 'test-ext', lastError: null },
    commands: {
      onCommand: { addListener: mock.fn((fn) => { commandListener = fn }) }
    },
    sidePanel: {
      open: (...args) => sidePanelOpen(...args),
      setOptions: mock.fn(async () => {})
    },
    tabs: { query: mock.fn(async () => [{ id: 42 }]), get: mock.fn(async () => ({ id: 42, windowId: 1 })) },
    storage: {
      local: { get: mock.fn(async () => ({})), set: mock.fn(async () => {}), remove: mock.fn(async () => {}) },
      session: { get: mock.fn(async () => ({})), set: mock.fn(async () => {}) }
    }
  }
}

describe('terminal side panel gesture-native entry points', () => {
  beforeEach(() => {
    mock.reset()
    installChrome()
  })

  test('the manifest declares a keyboard command for the terminal panel', () => {
    const manifest = JSON.parse(readFileSync('extension/manifest.json', 'utf8'))
    assert.ok(
      manifest.commands?.open_terminal_panel,
      'without a command there is no gesture-native way to open the panel'
    )
  })

  test('the terminal command ships unbound so the manifest stays loadable', () => {
    // Chrome refuses the ENTIRE manifest past four suggested_key commands
    // ("Too many shortcuts specified for 'commands': The maximum is 4"), and four
    // are already taken. Giving this one a default key broke the extension
    // outright. See chrome-platform-limits.test.js for the cap itself.
    const manifest = JSON.parse(readFileSync('extension/manifest.json', 'utf8'))
    assert.strictEqual(
      manifest.commands.open_terminal_panel.suggested_key,
      undefined,
      'open_terminal_panel must stay unbound; users assign a key at chrome://extensions/shortcuts'
    )
  })

  test('the keyboard command opens the panel on the tab Chrome hands the listener', async () => {
    const { installTerminalPanelCommandListener } = await import(
      `../../extension/background/ui/keyboard-shortcuts.js?v=${++importCounter}`
    )
    installTerminalPanelCommandListener()
    assert.ok(commandListener, 'expected a commands.onCommand listener')

    await commandListener('open_terminal_panel', { id: 42 })

    assert.deepStrictEqual(openCalls, [42],
      'must open on the tab supplied synchronously by onCommand, not one looked up via await')
  })

  test('the keyboard command ignores unrelated commands', async () => {
    const { installTerminalPanelCommandListener } = await import(
      `../../extension/background/ui/keyboard-shortcuts.js?v=${++importCounter}`
    )
    installTerminalPanelCommandListener()

    await commandListener('toggle_draw_mode', { id: 42 })

    assert.deepStrictEqual(openCalls, [])
  })

  test('the keyboard command closes an open panel instead of reopening it (toggle, F10)', async () => {
    // Wire onConnect so a live panel port registers on the shared (non
    // cache-busted) terminal-panel module — the same instance the keyboard
    // listener toggles against.
    let connectListener
    globalThis.chrome.runtime.onConnect = {
      addListener: mock.fn((fn) => { connectListener = fn })
    }

    const termPanel = await import('../../extension/background/ui/terminal-panel.js')
    termPanel.watchTerminalPanelState()

    const posted = []
    let disconnectListener
    connectListener({
      name: 'kaboom_terminal_panel', // TERMINAL_PANEL_PORT
      postMessage: mock.fn((m) => posted.push(m)),
      onDisconnect: { addListener: mock.fn((fn) => { disconnectListener = fn }) }
    })
    assert.strictEqual(termPanel.isTerminalPanelOpenSync(), true, 'port connect marks the panel open')

    const { installTerminalPanelCommandListener } = await import(
      `../../extension/background/ui/keyboard-shortcuts.js?v=${++importCounter}`
    )
    installTerminalPanelCommandListener()
    await commandListener('open_terminal_panel', { id: 42 })

    assert.deepStrictEqual(openCalls, [], 'toggle must not reopen an already-open panel')
    assert.ok(
      posted.some((m) => m.type === 'close_terminal_panel'),
      'the keyboard toggle should ask the open panel to close'
    )

    // Reset the shared singleton so later tests see no live port.
    disconnectListener?.()
    assert.strictEqual(termPanel.isTerminalPanelOpenSync(), false)
  })

  test('the shared opener reaches sidePanel.open with no await in front of it', async () => {
    // Ordering is the whole contract: an *await* before open() expires the
    // gesture. Dispatching setOptions first does not — it is fired, never
    // awaited — and it has to happen, or Chrome rejects the open with "No active
    // side panel for tabId" on any tab availability scoping has disabled.
    const order = []
    globalThis.chrome.sidePanel.setOptions = mock.fn(async () => { order.push('setOptions') })
    sidePanelOpen = mock.fn(async ({ tabId }) => { order.push('open'); openCalls.push(tabId) })

    const { openTerminalSidePanel } = await import(
      `../../extension/background/ui/terminal-panel.js?v=${++importCounter}`
    )
    const pending = openTerminalSidePanel(42) // deliberately not awaited yet

    assert.deepStrictEqual(order, ['setOptions', 'open'],
      'both must land synchronously, before any microtask boundary')
    assert.deepStrictEqual(await pending, { success: true })
  })

  test('the shared opener reports the Chrome error instead of swallowing it', async () => {
    sidePanelOpen = mock.fn(async () => {
      throw new Error('`sidePanel.open()` may only be called in response to a user gesture.')
    })
    const { openTerminalSidePanel } = await import(
      `../../extension/background/ui/terminal-panel.js?v=${++importCounter}`
    )

    const result = await openTerminalSidePanel(42)

    assert.strictEqual(result.success, false)
    assert.match(result.error, /user gesture/)
  })
})
