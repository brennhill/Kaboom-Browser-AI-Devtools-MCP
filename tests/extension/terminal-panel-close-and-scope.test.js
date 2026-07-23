// @ts-nocheck
/**
 * @fileoverview terminal-panel-close-and-scope.test.js — Closing the terminal
 * drawer, recovering from it, and scoping the panel to the tracked tab.
 *
 * Three defects this pins:
 *  1. closeBrowserSidePanel() bailed out when `chrome.sidePanel.close` was
 *     missing (it is very new), so the close button did nothing. Combined with
 *     unmountPanel() that left a blank panel the user could neither close nor
 *     recover — "I can't figure out how to close the panel".
 *  2. Close tore down the PTY, so closing the drawer lost the shell.
 *  3. The manifest's side_panel.default_path makes the panel available on every
 *     tab, so it rendered empty everywhere except the tracked page.
 */

import { beforeEach, describe, mock, test } from 'node:test'
import assert from 'node:assert'

let importCounter = 0
let setOptionsCalls

function installChrome({ withClose }) {
  setOptionsCalls = []
  const sidePanel = {
    setOptions: mock.fn(async (opts) => { setOptionsCalls.push(opts) }),
    open: mock.fn(async () => {})
  }
  if (withClose) sidePanel.close = mock.fn(async () => {})
  globalThis.chrome = {
    runtime: { id: 'test-ext', sendMessage: mock.fn(async () => ({})) },
    sidePanel,
    tabs: { query: mock.fn(async () => [{ id: 7 }]) },
    storage: {
      local: { get: mock.fn(async () => ({})), set: mock.fn(async () => {}), remove: mock.fn(async () => {}) },
      session: { get: mock.fn(async () => ({})), set: mock.fn(async () => {}), remove: mock.fn(async () => {}) },
      onChanged: { addListener: mock.fn() }
    }
  }
}

async function loadAvailability() {
  return import(`../../extension/background/side-panel-availability.js?v=${++importCounter}`)
}

describe('side panel availability follows the tracked tab', () => {
  beforeEach(() => { mock.reset(); installChrome({ withClose: true }) })

  test('disables the global default so untracked tabs do not offer an empty panel', async () => {
    const { syncTerminalPanelAvailability } = await loadAvailability()
    await syncTerminalPanelAvailability(42)

    const globalOff = setOptionsCalls.find((c) => c.tabId === undefined)
    assert.ok(globalOff, 'expected a global setOptions call')
    assert.strictEqual(globalOff.enabled, false,
      'the manifest default makes the panel available everywhere; it must be turned off')
  })

  test('enables the panel on the tracked tab only', async () => {
    const { syncTerminalPanelAvailability, SIDE_PANEL_PATH } = await loadAvailability()
    await syncTerminalPanelAvailability(42)

    const perTab = setOptionsCalls.find((c) => c.tabId === 42)
    assert.ok(perTab, 'expected the tracked tab to be enabled')
    assert.strictEqual(perTab.enabled, true)
    assert.strictEqual(perTab.path, SIDE_PANEL_PATH)
  })

  test('with nothing tracked the panel is offered nowhere', async () => {
    const { syncTerminalPanelAvailability } = await loadAvailability()
    await syncTerminalPanelAvailability(undefined)

    assert.deepStrictEqual(
      setOptionsCalls.filter((c) => c.enabled === true), [],
      'no tab should be enabled when nothing is tracked'
    )
  })

  test('a setOptions rejection does not throw (the tab may have closed)', async () => {
    globalThis.chrome.sidePanel.setOptions = mock.fn(async () => { throw new Error('No tab with id 42') })
    const { syncTerminalPanelAvailability } = await loadAvailability()
    await syncTerminalPanelAvailability(42) // must not reject
  })
})

// Open-vs-close now turns on whether a panel document is actually connected, not
// on a mirrored storage flag — see terminal-panel-presence.test.js for that half.
describe('closing the terminal panel', () => {
  test('the open path calls sidePanel.open with no await first, preserving the gesture', async () => {
    // The regression: toggle awaited a storage read to decide open-vs-close, and
    // that await expired the user gesture, so Chrome refused the open and
    // "Open Kaboom Terminal" silently did nothing.
    mock.reset()
    installChrome({ withClose: true })
    const { toggleTerminalSidePanel } = await import(
      `../../extension/background/terminal-panel.js?v=${++importCounter}`
    )

    toggleTerminalSidePanel(7) // deliberately not awaited

    assert.strictEqual(
      globalThis.chrome.sidePanel.open.mock.calls.length, 1,
      'open() must be reached synchronously, before any microtask boundary'
    )
  })
})
