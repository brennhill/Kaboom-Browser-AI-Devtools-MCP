// @ts-nocheck
/**
 * @fileoverview terminal-panel-presence.test.js — "Is there a panel?" must be
 * answered by the panel itself, and opening must work on any tab.
 *
 * Two defects this pins, both of which left the user unable to *open* the panel:
 *
 *  1. `chrome.sidePanel.open({tabId})` fails with "No active side panel for
 *     tabId: N" when the panel is disabled for that tab. Scoping availability to
 *     the tracked tab turned every other tab into exactly that — so the Terminal
 *     button reported the Chrome error instead of opening anything. An explicit
 *     open must enable the panel for the target tab first.
 *
 *  2. Panel-open state was mirrored from chrome.storage. Closing the panel with
 *     Chrome's own X does not reliably flush a storage write, so the flag stuck
 *     at "open" and the toggle kept trying to *close* a panel that was already
 *     gone. Liveness now comes from a runtime port, which Chrome tears down when
 *     the document dies.
 */

import { beforeEach, describe, mock, test } from 'node:test'
import assert from 'node:assert'

let importCounter = 0
let order
let connectListener
let sentOverPort

function makePort(name) {
  const disconnectListeners = []
  return {
    name,
    postMessage: mock.fn((msg) => { sentOverPort.push(msg) }),
    onDisconnect: { addListener: mock.fn((fn) => { disconnectListeners.push(fn) }) },
    disconnect: () => { for (const fn of disconnectListeners) fn() }
  }
}

function installChrome() {
  order = []
  connectListener = null
  sentOverPort = []
  globalThis.chrome = {
    runtime: {
      id: 'test-ext',
      sendMessage: mock.fn(async () => ({})),
      onConnect: { addListener: mock.fn((fn) => { connectListener = fn }) }
    },
    sidePanel: {
      open: mock.fn(async () => { order.push('open') }),
      setOptions: mock.fn(async (opts) => { order.push(`setOptions:${opts.tabId ?? 'global'}`) })
    },
    tabs: {
      get: mock.fn(async () => ({ id: 7, groupId: -1 })),
      query: mock.fn(async () => [{ id: 7 }]),
      update: mock.fn(async () => ({}))
    },
    tabGroups: undefined,
    storage: {
      local: { get: mock.fn(async () => ({})), set: mock.fn(async () => {}), remove: mock.fn(async () => {}) },
      session: { get: mock.fn(async () => ({})), set: mock.fn(async () => {}), remove: mock.fn(async () => {}) },
      onChanged: { addListener: mock.fn() }
    }
  }
}

async function loadPanel() {
  return import(`../../extension/background/terminal-panel.js?v=${++importCounter}`)
}

describe('opening the panel on a tab that is not the tracked tab', () => {
  beforeEach(() => { mock.reset(); installChrome() })

  test('enables the panel for the target tab before calling open', async () => {
    // Without this, Chrome rejects with "No active side panel for tabId: N"
    // whenever availability has been scoped away from this tab.
    const { openTerminalSidePanel } = await loadPanel()

    const result = await openTerminalSidePanel(7)

    assert.strictEqual(result.success, true)
    assert.strictEqual(order[0], 'setOptions:7',
      'the panel must be enabled for the tab before open() is attempted')
    assert.strictEqual(order[1], 'open')
  })

  test('the enable call still runs with no await before open, so the gesture survives', async () => {
    const { openTerminalSidePanel } = await loadPanel()

    openTerminalSidePanel(7) // deliberately not awaited

    assert.strictEqual(globalThis.chrome.sidePanel.open.mock.calls.length, 1,
      'open() must be reached synchronously; setOptions is dispatched but not awaited')
  })

  test('the panel keeps one constant path so opening never reloads it', async () => {
    // A path change makes Chrome reload the side panel document. Setting a
    // per-tab path after open() tore down the xterm that had just booted and
    // started a second session underneath it.
    const { openTerminalSidePanel } = await loadPanel()
    const { SIDE_PANEL_PATH } = await import(
      `../../extension/background/side-panel-availability.js?v=${++importCounter}`
    )

    await openTerminalSidePanel(7)
    await new Promise((r) => setTimeout(r, 0)) // let best-effort follow-up work run

    const paths = globalThis.chrome.sidePanel.setOptions.mock.calls
      .map((c) => c.arguments[0]?.path)
      .filter((p) => p !== undefined)
    assert.ok(paths.length > 0, 'expected at least one setOptions with a path')
    for (const path of paths) {
      assert.strictEqual(path, SIDE_PANEL_PATH,
        `every setOptions must use the constant path; got ${path}`)
    }
  })

  test('a setOptions rejection does not stop the open attempt', async () => {
    globalThis.chrome.sidePanel.setOptions = mock.fn(async () => { throw new Error('No tab with id 7') })
    const { openTerminalSidePanel } = await loadPanel()

    const result = await openTerminalSidePanel(7)

    assert.strictEqual(result.success, true, 'open() must still be attempted')
  })
})

describe('panel liveness comes from the panel document', () => {
  beforeEach(() => { mock.reset(); installChrome() })

  test('no panel is reported before one connects', async () => {
    const { isTerminalPanelOpenSync, watchTerminalPanelState } = await loadPanel()
    watchTerminalPanelState()

    assert.strictEqual(isTerminalPanelOpenSync(), false)
  })

  test('a connected panel port reports open, and disconnect reports closed', async () => {
    const { isTerminalPanelOpenSync, watchTerminalPanelState } = await loadPanel()
    watchTerminalPanelState()
    assert.ok(connectListener, 'expected a runtime.onConnect listener')

    const port = makePort('kaboom_terminal_panel')
    connectListener(port)
    assert.strictEqual(isTerminalPanelOpenSync(), true)

    // Chrome tears the port down when the document dies — including when the
    // user closes the panel with Chrome's own X, which no storage write sees.
    port.disconnect()
    assert.strictEqual(isTerminalPanelOpenSync(), false)
  })

  test('port connect mirrors "open" and disconnect mirrors "closed" into TERMINAL_UI_STATE (un-sticks the flame)', async () => {
    const { watchTerminalPanelState } = await loadPanel()
    watchTerminalPanelState()
    assert.ok(connectListener, 'expected a runtime.onConnect listener')
    const setSession = globalThis.chrome.storage.session.set

    const port = makePort('kaboom_terminal_panel')
    connectListener(port)
    // The in-page flame launcher reads TERMINAL_UI_STATE to know whether to hide;
    // a live port must mark it open.
    const onConnectWrite = setSession.mock.calls.at(-1)?.arguments[0]
    assert.strictEqual(onConnectWrite?.kaboom_terminal_ui_state, 'open')

    // A Chrome-native close only tears down the port — the panel document dies
    // with no chance to write 'closed'. If the background does not reset the key
    // here it stays stuck at 'open' and the flame is suppressed forever.
    port.disconnect()
    const onDisconnectWrite = setSession.mock.calls.at(-1)?.arguments[0]
    assert.strictEqual(onDisconnectWrite?.kaboom_terminal_ui_state, 'closed')
  })

  test('ports from other features do not count as a panel', async () => {
    const { isTerminalPanelOpenSync, watchTerminalPanelState } = await loadPanel()
    watchTerminalPanelState()

    connectListener(makePort('some-other-feature'))

    assert.strictEqual(isTerminalPanelOpenSync(), false)
  })
})

describe('toggling follows actual panel presence', () => {
  beforeEach(() => { mock.reset(); installChrome() })

  test('with a live panel the toggle closes it over the port', async () => {
    const { toggleTerminalSidePanel, watchTerminalPanelState } = await loadPanel()
    watchTerminalPanelState()
    connectListener(makePort('kaboom_terminal_panel'))

    const result = await toggleTerminalSidePanel(7)

    assert.strictEqual(result.success, true)
    assert.deepStrictEqual(sentOverPort, [{ type: 'close_terminal_panel' }])
    assert.strictEqual(globalThis.chrome.sidePanel.open.mock.calls.length, 0)
  })

  test('with no live panel the toggle opens one', async () => {
    const { toggleTerminalSidePanel, watchTerminalPanelState } = await loadPanel()
    watchTerminalPanelState()

    const result = await toggleTerminalSidePanel(7)

    assert.strictEqual(result.success, true)
    assert.strictEqual(globalThis.chrome.sidePanel.open.mock.calls.length, 1)
  })

  test('after the panel disconnects the toggle opens again rather than closing nothing', async () => {
    // The exact dead end the user hit: state said "open", so every attempt sent
    // a close to a document that no longer existed and nothing ever opened.
    const { toggleTerminalSidePanel, watchTerminalPanelState } = await loadPanel()
    watchTerminalPanelState()
    const port = makePort('kaboom_terminal_panel')
    connectListener(port)
    port.disconnect()

    await toggleTerminalSidePanel(7)

    assert.strictEqual(globalThis.chrome.sidePanel.open.mock.calls.length, 1)
  })
})
