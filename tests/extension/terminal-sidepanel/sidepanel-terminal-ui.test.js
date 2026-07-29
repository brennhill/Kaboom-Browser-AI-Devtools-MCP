// @ts-nocheck
/**
 * @fileoverview Terminal-panel reopen, layout, fallback, close, and presence-port tests.
 */

import { beforeEach, describe, mock, test } from 'node:test'
import assert from 'node:assert'
import { StorageKey } from '../../../extension/lib/constants.js'
import {
  dispatchWindowEvent,
  findButton,
  getElementById,
  makeResponse,
  setupEnvironment,
  sidepanelState,
  sleep,
  walkTree,
} from './sidepanel-terminal-fixture.js'

describe('terminal side panel host', () => {
  beforeEach(() => {
    mock.reset()
    sidepanelState.localStorageData = { [StorageKey.SERVER_URL]: 'http://localhost:7890' }
    sidepanelState.sessionStorageData = {}
    setupEnvironment()
  })

  test('reopening a minimized session restores the full panel without starting a new session', async () => {
    sidepanelState.sessionStorageData[StorageKey.TERMINAL_SESSION] = { sessionId: 'session-min', token: 'token-min' }
    sidepanelState.sessionStorageData[StorageKey.TERMINAL_UI_STATE] = 'minimized'

    sidepanelState.fetchHandler = ({ url }) => {
      if (url.includes('/terminal/validate?token=')) {
        return Promise.resolve(makeResponse(200, { valid: true }))
      }
      throw new Error(`Unexpected fetch call: ${url}`)
    }

    const module = await import(`../../../extension/sidepanel.js?v=${++sidepanelState.importCounter}`)
    await module._terminalPanelForTests.bootTerminalPanel(true)

    const header = getElementById('kaboom-terminal-header')
    const minimizeButton = findButton(header, (node) => node.title === 'Minimize terminal')
    const terminalBody = getElementById('kaboom-terminal-body')

    assert.ok(minimizeButton, 'minimize button should be present after restore')
    assert.ok(terminalBody, 'terminal body should exist after restore')
    assert.strictEqual(terminalBody.style.display, 'block', 'reopened minimized session should restore the full panel')
    assert.strictEqual(sidepanelState.sessionStorageData[StorageKey.TERMINAL_UI_STATE], 'open', 'reopen should promote minimized session back to open')
  })

  test('panel mounts only the terminal shell so xterm can use the full panel height', async () => {
    sidepanelState.fetchHandler = ({ url }) => {
      if (url.endsWith('/terminal/start')) {
        return Promise.resolve(makeResponse(200, {
          session_id: 'session-full-height',
          token: 'token-full-height',
          pid: 999
        }))
      }
      throw new Error(`Unexpected fetch call: ${url}`)
    }

    const module = await import(`../../../extension/sidepanel.js?v=${++sidepanelState.importCounter}`)
    await module._terminalPanelForTests.bootTerminalPanel(true)

    const root = getElementById('kaboom-terminal-widget')
    const header = getElementById('kaboom-terminal-header')
    const iframe = getElementById('kaboom-terminal-iframe')
    const terminalShell = header?.parentElement || null
    const newProjectButton = findButton(root, (node) => node.textContent === 'New Project')
    const titleNode = walkTree(header, (child) => child.textContent === 'KaBOOM! Terminal')

    assert.ok(root, 'panel root should exist')
    assert.ok(header, 'terminal header should exist')
    assert.ok(iframe, 'terminal iframe should exist')
    assert.ok(terminalShell, 'terminal shell should wrap the header and iframe')
    assert.ok(titleNode, 'terminal header should show Kaboom Terminal')
    assert.strictEqual(newProjectButton, null, 'placeholder palette action should not be rendered')
    assert.strictEqual(root.children.length, 1, 'terminal shell should be the only top-level panel child')
  })

  test('API billing detected by the terminal becomes a visible subscription warning', async () => {
    sidepanelState.fetchHandler = ({ url }) => {
      if (url.endsWith('/terminal/start')) {
        return Promise.resolve(makeResponse(200, {
          session_id: 'session-api-billing',
          token: 'token-api-billing',
          pid: 999
        }))
      }
      throw new Error(`Unexpected fetch call: ${url}`)
    }

    const module = await import(`../../../extension/sidepanel.js?v=${++sidepanelState.importCounter}`)
    await module._terminalPanelForTests.bootTerminalPanel(true)
    dispatchWindowEvent('message', {
      origin: 'http://localhost:7891',
      data: { source: 'kaboom-terminal', event: 'api_billing_detected', data: {} }
    })

    const toast = getElementById('kaboom-action-toast')
    assert.ok(toast, 'API billing must be surfaced in the visible panel UI')
    const warning = walkTree(toast, (child) =>
      typeof child.textContent === 'string' && child.textContent.includes('API billing is active')
    )
    const guidance = walkTree(toast, (child) =>
      typeof child.textContent === 'string' && child.textContent.includes('not your subscription')
    )
    assert.ok(warning)
    assert.ok(guidance)
  })

  test('terminal header persistently shows the detected execution provider', async () => {
    sidepanelState.fetchHandler = ({ url }) => {
      if (url.endsWith('/terminal/start')) {
        return Promise.resolve(makeResponse(200, {
          session_id: 'session-provider',
          token: 'token-provider',
          pid: 998
        }))
      }
      throw new Error(`Unexpected fetch call: ${url}`)
    }

    const module = await import(`../../../extension/sidepanel.js?v=${++sidepanelState.importCounter}`)
    await module._terminalPanelForTests.bootTerminalPanel(true)
    dispatchWindowEvent('message', {
      origin: 'http://localhost:7891',
      data: {
        source: 'kaboom-terminal',
        event: 'execution_provider_detected',
        data: { provider: 'subscription', tool: 'codex' }
      }
    })

    const badge = getElementById('kaboom-terminal-provider-badge')
    assert.ok(badge)
    assert.strictEqual(badge.textContent, 'Codex · Subscription')
    assert.match(badge.title, /ChatGPT subscription/)
  })

  test('daemon-unavailable fallback uses Kaboom copy', async () => {
    sidepanelState.fetchHandler = ({ url }) => {
      if (url.endsWith('/terminal/start')) {
        return Promise.resolve(makeResponse(500, { error: 'daemon_unavailable' }))
      }
      throw new Error(`Unexpected fetch call: ${url}`)
    }

    const module = await import(`../../../extension/sidepanel.js?v=${++sidepanelState.importCounter}`)
    await module._terminalPanelForTests.bootTerminalPanel(true)

    const header = getElementById('kaboom-terminal-header')
    const terminalBody = getElementById('kaboom-terminal-body')
    const titleNode = walkTree(header, (child) => child.textContent === 'KaBOOM! Terminal')
    const fallbackNode = walkTree(terminalBody, (child) =>
      typeof child.textContent === 'string' && child.textContent.includes('KaBOOM! daemon')
    )
    // The dead-end sentence is replaced by a recoverable state: a session can be
    // started and the root folder changed without leaving the panel. Previously
    // an ended or failed session left nothing to click. The root folder moved
    // out of this state and into a bar that is always visible above the terminal.
    const startButton = walkTree(terminalBody, (child) => child.id === 'kaboom-terminal-start-button')
    const rootInput = getElementById('kaboom-terminal-root-folder-input')

    assert.ok(header, 'terminal header should exist')
    assert.ok(titleNode, 'terminal header should show Kaboom Terminal')
    assert.ok(terminalBody, 'terminal body should exist')
    assert.ok(fallbackNode, 'fallback should mention the Kaboom daemon')
    assert.ok(startButton, 'a session-less panel must offer a way to start one')
    assert.ok(rootInput, 'a session-less panel must let the user set the root folder')
    const rootBar = getElementById('kaboom-terminal-root-folder-bar')
    assert.ok(rootBar, 'the root folder bar must exist whether or not a session started')

    // Reachable-but-unavailable (500) is recoverable — it must NOT raise a
    // dead-end error toast; the no-session fallback IS the surface.
    assert.equal(getElementById('kaboom-action-toast'), null,
      'a reachable 500 must not surface a dead-end error toast')
  })

  test('daemon UNREACHABLE at open surfaces a visible error (not a silent drop)', async () => {
    // Transport failure ("Failed to fetch") while no panel body is mounted yet.
    // Previously showSandboxError early-returned on the missing body and the
    // failure only reached the console — the panel looked simply broken. It must
    // now be surfaced via a toast (fail-loud, repo rule 25).
    sidepanelState.fetchHandler = ({ url }) => {
      if (url.endsWith('/terminal/start')) {
        return Promise.reject(new Error('ECONNREFUSED terminal daemon'))
      }
      throw new Error(`Unexpected fetch call: ${url}`)
    }

    const module = await import(`../../../extension/sidepanel.js?v=${++sidepanelState.importCounter}`)
    await module._terminalPanelForTests.bootTerminalPanel(true)

    const toast = getElementById('kaboom-action-toast')
    assert.ok(toast, 'an unreachable daemon must surface a visible error toast, not vanish into the console')
    const messageNode = walkTree(toast, (child) =>
      typeof child.textContent === 'string' && child.textContent.includes('Terminal session start')
    )
    assert.ok(messageNode, 'the toast must carry the start-failure message')
  })

  test('close button closes the drawer and leaves the shell running', async () => {
    // Closing a drawer must not destroy the shell inside it. The old close
    // called exitTerminalSession(), so a user tidying the UI lost their session.
    const stopBodies = []
    globalThis.fetch = (url, init) => {
      if (url.includes('/terminal/start')) {
        return Promise.resolve(makeResponse(200, { session_id: 'session-1', token: 'tok-1', pid: 1 }))
      }
      if (url.includes('/terminal/stop')) {
        stopBodies.push(JSON.parse(init.body))
        return Promise.resolve(makeResponse(200, { status: 'stopped' }))
      }
      if (url.includes('/terminal/validate?token=')) {
        return Promise.resolve(makeResponse(200, { valid: false }))
      }
      throw new Error(`Unexpected fetch call: ${url}`)
    }

    const module = await import(`../../../extension/sidepanel.js?v=${++sidepanelState.importCounter}`)
    await module._terminalPanelForTests.bootTerminalPanel(true)

    const header = getElementById('kaboom-terminal-header')
    const closeButton = findButton(header, (node) => node.id === 'kaboom-terminal-close-button')
    assert.ok(closeButton, 'the panel needs an obvious close control')

    closeButton.dispatch('click')
    await sleep(0)

    assert.deepStrictEqual(stopBodies, [], 'closing the drawer must not stop the PTY')
    assert.notStrictEqual(
      sidepanelState.sessionStorageData[StorageKey.TERMINAL_SESSION], undefined,
      'the session must survive so reopening reconnects to it'
    )
    assert.strictEqual(chrome.sidePanel.close.mock.calls.length, 1, 'close should close the side panel')
  })

  // The background decides open-vs-close from whether a panel document exists.
  // It learns that from this port — a stored flag cannot see Chrome's own X
  // dismiss the panel, and a stale "open" left the user unable to reopen at all.
  describe('presence port', () => {
    function bootWithSession() {
      globalThis.fetch = (url) => {
        if (url.includes('/terminal/start')) {
          return Promise.resolve(makeResponse(200, { session_id: 'session-1', token: 'tok-1', pid: 1 }))
        }
        if (url.includes('/terminal/validate?token=')) {
          return Promise.resolve(makeResponse(200, { valid: false }))
        }
        throw new Error(`Unexpected fetch call: ${url}`)
      }
      return import(`../../../extension/sidepanel.js?v=${++sidepanelState.importCounter}`)
    }

    test('the panel announces itself on the terminal panel port while it is alive', async () => {
      const module = await bootWithSession()
      await module._terminalPanelForTests.bootTerminalPanel(true)

      const port = sidepanelState.connectedPorts.find((p) => p.name === 'kaboom_terminal_panel')
      assert.ok(port, 'the background has no other way to know a panel exists')
    })

    test('a close over the port closes the drawer without stopping the shell', async () => {
      const module = await bootWithSession()
      await module._terminalPanelForTests.bootTerminalPanel(true)
      const port = sidepanelState.connectedPorts.find((p) => p.name === 'kaboom_terminal_panel')

      for (const listener of port.messageListeners) listener({ type: 'close_terminal_panel' })
      await sleep(0)

      assert.strictEqual(getElementById('kaboom-terminal-widget'), null, 'the drawer should be gone')
      assert.notStrictEqual(
        sidepanelState.sessionStorageData[StorageKey.TERMINAL_SESSION], undefined,
        'closing the drawer must leave the shell running'
      )
    })

    test('a restore over the port rebuilds a panel that had been closed', async () => {
      // sidePanel.open() on an existing panel only focuses it — no code runs in
      // this document — so without this an already-open-but-blank panel stayed
      // blank and "Open Kaboom Terminal" looked broken.
      const module = await bootWithSession()
      await module._terminalPanelForTests.bootTerminalPanel(true)
      const port = sidepanelState.connectedPorts.find((p) => p.name === 'kaboom_terminal_panel')

      for (const listener of port.messageListeners) listener({ type: 'close_terminal_panel' })
      await sleep(0)
      assert.strictEqual(getElementById('kaboom-terminal-widget'), null)

      for (const listener of port.messageListeners) listener({ type: 'restore_terminal_panel' })
      await sleep(0)

      assert.ok(getElementById('kaboom-terminal-widget'), 'restore must put the terminal back')
      assert.strictEqual(sidepanelState.sessionStorageData[StorageKey.TERMINAL_UI_STATE], 'open')
    })

    test('a restore on a panel that has no terminal starts one', async () => {
      // Booting while the daemon was down leaves a mounted panel with no iframe.
      // Reopening has to try again, not just re-show the failure.
      let startCalls = 0
      globalThis.fetch = (url) => {
        if (url.includes('/terminal/start')) {
          startCalls += 1
          return startCalls === 1
            ? Promise.resolve(makeResponse(500, { error: 'daemon_unavailable' }))
            : Promise.resolve(makeResponse(200, { session_id: 's2', token: 'tok-2', pid: 2 }))
        }
        if (url.includes('/terminal/validate?token=')) {
          return Promise.resolve(makeResponse(200, { valid: false }))
        }
        throw new Error(`Unexpected fetch call: ${url}`)
      }
      const module = await import(`../../../extension/sidepanel.js?v=${++sidepanelState.importCounter}`)
      await module._terminalPanelForTests.bootTerminalPanel(true)
      const port = sidepanelState.connectedPorts.find((p) => p.name === 'kaboom_terminal_panel')
      assert.strictEqual(getElementById('kaboom-terminal-iframe'), null, 'no session means no iframe')

      for (const listener of port.messageListeners) listener({ type: 'restore_terminal_panel' })
      await sleep(0)

      assert.strictEqual(startCalls, 2, 'restore must retry the session')
      assert.ok(getElementById('kaboom-terminal-iframe'), 'the retry should mount a terminal')
    })
  })
})
