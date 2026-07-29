// @ts-nocheck
/**
 * @fileoverview Terminal-panel boot, session lifecycle, redraw, and recovery tests.
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
} from './sidepanel-terminal-fixture.js'

describe('terminal side panel host', () => {
  beforeEach(() => {
    mock.reset()
    sidepanelState.localStorageData = { [StorageKey.SERVER_URL]: 'http://localhost:7890' }
    sidepanelState.sessionStorageData = {}
    setupEnvironment()
  })

  test('boots a panel with terminal iframe and persists open state', async () => {
    sidepanelState.fetchHandler = ({ url }) => {
      if (url.endsWith('/terminal/start')) {
        return Promise.resolve(makeResponse(200, {
          session_id: 'session-1',
          token: 'token-1',
          pid: 999
        }))
      }
      throw new Error(`Unexpected fetch call: ${url}`)
    }

    const module = await import(`../../../extension/sidepanel.js?v=${++sidepanelState.importCounter}`)
    await module._terminalPanelForTests.bootTerminalPanel(true)

    const header = getElementById('kaboom-terminal-header')
    const iframe = getElementById('kaboom-terminal-iframe')
    assert.ok(header, 'terminal header should be mounted')
    assert.ok(iframe, 'terminal iframe should be mounted')
    assert.strictEqual(sidepanelState.sessionStorageData[StorageKey.TERMINAL_UI_STATE], 'open')

    const minimizeButton = findButton(header, (node) => node.title === 'Minimize terminal')
    assert.ok(minimizeButton, 'minimize button should exist')
    assert.strictEqual(minimizeButton.textContent, '\u2581')
  })

  test('re-booting with forceFresh unmounts the old panel and attaches the fresh shell (folder-reload fix)', async () => {
    let startCount = 0
    sidepanelState.fetchHandler = ({ url }) => {
      if (url.endsWith('/terminal/start')) {
        startCount += 1
        return Promise.resolve(makeResponse(200, {
          session_id: `session-${startCount}`,
          token: `token-${startCount}`,
          pid: 999
        }))
      }
      throw new Error(`Unexpected fetch call: ${url}`)
    }

    const module = await import(`../../../extension/sidepanel.js?v=${++sidepanelState.importCounter}`)
    await module._terminalPanelForTests.bootTerminalPanel(true)
    assert.ok(getElementById('kaboom-terminal-iframe').src.includes('token-1'), 'first boot mounts the token-1 shell')

    // A second forceFresh boot models applyRootFolder()/the retry button rebuilding
    // the panel for a NEW session. Without unmounting first, mountPanel() early-
    // returns while rootEl is set, so the fresh token-2 shell is never attached and
    // the user is left on the old, just-stopped session \u2014 the "terminal won't start
    // after picking a folder" failure.
    await module._terminalPanelForTests.bootTerminalPanel(true)
    const iframe = getElementById('kaboom-terminal-iframe')
    assert.ok(iframe, 'a terminal iframe is mounted after the fresh re-boot')
    assert.ok(iframe.src.includes('token-2'), 'forceFresh re-boot attaches the fresh token-2 shell, not the orphaned old one')
  })

  test('applyRootFolder persists the root, stops the old session, and mounts a fresh shell (reported bug, end-to-end)', async () => {
    let startCount = 0
    let stoppedId = null
    sidepanelState.fetchHandler = ({ url, options }) => {
      if (url.endsWith('/terminal/start')) {
        startCount += 1
        return Promise.resolve(makeResponse(200, {
          session_id: `session-${startCount}`,
          token: `token-${startCount}`,
          pid: 999
        }))
      }
      if (url.endsWith('/terminal/stop')) {
        try { stoppedId = JSON.parse(options.body || '{}').id } catch { stoppedId = 'parse-error' }
        return Promise.resolve(makeResponse(200, { ok: true }))
      }
      throw new Error(`Unexpected fetch call: ${url}`)
    }

    const module = await import(`../../../extension/sidepanel.js?v=${++sidepanelState.importCounter}`)
    await module._terminalPanelForTests.bootTerminalPanel(true)
    assert.ok(getElementById('kaboom-terminal-iframe').src.includes('token-1'), 'first boot mounts token-1')

    // The exact path that failed for the user: pick a folder and reload. A running
    // PTY can't change cwd, so the old session is stopped and a fresh one booted.
    await module._terminalPanelForTests.applyRootFolder('/Users/x/project')

    assert.strictEqual(stoppedId, 'session-1', 'the old session is stopped before rebuilding')
    assert.strictEqual(sidepanelState.localStorageData[StorageKey.TERMINAL_DEV_ROOT], '/Users/x/project', 'the chosen root is persisted')
    const iframe = getElementById('kaboom-terminal-iframe')
    assert.ok(iframe && iframe.src.includes('token-2'), 'the fresh shell for the new folder is attached — not the orphaned, just-stopped session')
  })

  test('disconnect button ends the current session and closes the side panel', async () => {
    let startCount = 0
    const stopBodies = []

    sidepanelState.fetchHandler = ({ url, options }) => {
      if (url.endsWith('/terminal/start')) {
        startCount += 1
        return Promise.resolve(makeResponse(200, {
          session_id: `session-${startCount}`,
          token: `token-${startCount}`,
          pid: 999
        }))
      }
      if (url.endsWith('/terminal/stop')) {
        stopBodies.push(JSON.parse(String(options.body || '{}')))
        return Promise.resolve(makeResponse(200, { ok: true }))
      }
      if (url.includes('/terminal/validate?token=')) {
        return Promise.resolve(makeResponse(200, { valid: false }))
      }
      throw new Error(`Unexpected fetch call: ${url}`)
    }

    const module = await import(`../../../extension/sidepanel.js?v=${++sidepanelState.importCounter}`)
    await module._terminalPanelForTests.bootTerminalPanel(true)

    const header = getElementById('kaboom-terminal-header')
    // Matched by id, not title: the title is user-facing copy and should be free
    // to change without breaking the behavioural contract below.
    const powerButton = findButton(header, (node) => node.id === 'kaboom-terminal-disconnect-button')
    assert.ok(powerButton, 'power button should be present')
    assert.match(powerButton.title, /end session/i, 'the power control must read as ending the session')
    assert.strictEqual(startCount, 1)

    powerButton.dispatch('click')
    await sleep(0)

    assert.strictEqual(stopBodies.length, 1, 'disconnect should stop the current session')
    assert.deepStrictEqual(stopBodies[0], { id: 'session-1' })
    assert.strictEqual(startCount, 1, 'disconnect should not boot a fresh session')
    assert.strictEqual(chrome.sidePanel.close.mock.calls.length, 1, 'disconnect should close the side panel')
    assert.strictEqual(chrome.sidePanel.close.mock.calls[0].arguments[0].tabId, 1)
    assert.strictEqual(sidepanelState.sessionStorageData[StorageKey.TERMINAL_SESSION], undefined, 'disconnect should clear persisted session')
    assert.strictEqual(sidepanelState.sessionStorageData[StorageKey.TERMINAL_UI_STATE], undefined, 'disconnect should clear persisted UI state')
    assert.strictEqual(getElementById('kaboom-terminal-widget'), null, 'disconnect should unmount the side panel shell')
  })

  test('minimize button hides the side panel and keeps the current session alive', async () => {
    let startCount = 0
    const stopBodies = []

    sidepanelState.fetchHandler = ({ url, options }) => {
      if (url.endsWith('/terminal/start')) {
        startCount += 1
        return Promise.resolve(makeResponse(200, {
          session_id: `session-${startCount}`,
          token: `token-${startCount}`,
          pid: 999
        }))
      }
      if (url.endsWith('/terminal/stop')) {
        stopBodies.push(JSON.parse(String(options.body || '{}')))
        return Promise.resolve(makeResponse(200, { ok: true }))
      }
      throw new Error(`Unexpected fetch call: ${url}`)
    }

    const module = await import(`../../../extension/sidepanel.js?v=${++sidepanelState.importCounter}`)
    await module._terminalPanelForTests.bootTerminalPanel(true)

    const header = getElementById('kaboom-terminal-header')
    const minimizeButton = findButton(header, (node) => node.title === 'Minimize terminal')
    assert.ok(minimizeButton, 'minimize button should be present')
    assert.strictEqual(startCount, 1)

    minimizeButton.dispatch('click')
    await sleep(0)

    assert.strictEqual(stopBodies.length, 0, 'minimize should not stop the current session')
    assert.strictEqual(startCount, 1, 'minimize should not boot a fresh session')
    assert.strictEqual(chrome.sidePanel.close.mock.calls.length, 1, 'minimize should close the side panel')
    assert.strictEqual(chrome.sidePanel.close.mock.calls[0].arguments[0].tabId, 1)
    assert.deepStrictEqual(
      sidepanelState.sessionStorageData[StorageKey.TERMINAL_SESSION],
      { sessionId: 'session-1', token: 'token-1' },
      'minimize should keep the persisted session'
    )
    assert.strictEqual(sidepanelState.sessionStorageData[StorageKey.TERMINAL_UI_STATE], 'minimized', 'minimize should persist hidden-session state')
    assert.strictEqual(getElementById('kaboom-terminal-widget'), null, 'minimize should unmount the side panel shell')
  })

  test('redraw button reloads iframe without starting a new session', async () => {
    let startCount = 0

    sidepanelState.fetchHandler = ({ url }) => {
      if (url.endsWith('/terminal/start')) {
        startCount += 1
        return Promise.resolve(makeResponse(200, {
          session_id: `session-${startCount}`,
          token: `token-${startCount}`,
          pid: 999
        }))
      }
      if (url.includes('/terminal/validate?token=')) {
        return Promise.resolve(makeResponse(200, { valid: true }))
      }
      throw new Error(`Unexpected fetch call: ${url}`)
    }

    const module = await import(`../../../extension/sidepanel.js?v=${++sidepanelState.importCounter}`)
    await module._terminalPanelForTests.bootTerminalPanel(true)

    const header = getElementById('kaboom-terminal-header')
    const iframe = getElementById('kaboom-terminal-iframe')
    const redrawButton = findButton(header, (node) => node.title === 'Redraw terminal graphics')
    assert.ok(iframe, 'terminal iframe should exist')
    assert.ok(redrawButton, 'redraw button should exist')

    const priorSrc = iframe.src
    redrawButton.dispatch('click')
    // The click handler is async (it revalidates the token and discovers the
    // terminal port before touching iframe.src). Let it settle INSIDE this test:
    // an un-awaited redraw finishes after the test returns, against the next
    // test's sidepanelState.fetchHandler, and its session start is then counted there.
    await sleep(0)

    assert.strictEqual(iframe.src, priorSrc, 'redraw should keep the same token URL')
    assert.strictEqual(startCount, 1, 'redraw should not start a new session')
  })

  test('redraw revalidates the token and rebuilds when the daemon restarted (dead token, L1)', async () => {
    let startCount = 0
    let tokenAlive = true
    sidepanelState.fetchHandler = ({ url }) => {
      if (url.endsWith('/terminal/start')) {
        startCount += 1
        return Promise.resolve(makeResponse(200, {
          session_id: `session-${startCount}`,
          token: `tok-${startCount}`,
          pid: 999
        }))
      }
      if (url.includes('/terminal/validate?token=')) {
        return Promise.resolve(makeResponse(200, { valid: tokenAlive }))
      }
      throw new Error(`Unexpected fetch call: ${url}`)
    }

    const module = await import(`../../../extension/sidepanel.js?v=${++sidepanelState.importCounter}`)
    await module._terminalPanelForTests.bootTerminalPanel(true)
    assert.ok(getElementById('kaboom-terminal-iframe').src.includes('tok-1'), 'first boot mounts tok-1')
    assert.strictEqual(startCount, 1)

    // Daemon restarted: the persisted token is now dead. A plain reload would
    // reconnect forever; redraw must revalidate and rebuild into a fresh session.
    tokenAlive = false
    await module._terminalPanelForTests.redrawTerminal()

    assert.strictEqual(startCount, 2, 'redraw on a dead token must boot a fresh session, not reload the dead one')
    assert.ok(getElementById('kaboom-terminal-iframe').src.includes('tok-2'), 'the rebuilt panel uses the fresh token')
  })

  test('reconnect_exhausted from the iframe revalidates and rebuilds a dead-token session', async () => {
    // The client half of daemon-restart recovery: after a full daemon restart the
    // iframe reconnects forever on a dead token, so it gives up and signals the
    // parent. The parent must revalidate + rebuild into a fresh session rather than
    // leaving a permanent silent disconnect.
    let startCount = 0
    let tokenAlive = true
    sidepanelState.fetchHandler = ({ url }) => {
      if (url.endsWith('/terminal/start')) {
        startCount += 1
        return Promise.resolve(makeResponse(200, {
          session_id: `session-${startCount}`,
          token: `tok-${startCount}`,
          pid: 999
        }))
      }
      if (url.includes('/terminal/validate?token=')) {
        return Promise.resolve(makeResponse(200, { valid: tokenAlive }))
      }
      throw new Error(`Unexpected fetch call: ${url}`)
    }

    const module = await import(`../../../extension/sidepanel.js?v=${++sidepanelState.importCounter}`)
    await module._terminalPanelForTests.bootTerminalPanel(true)
    assert.ok(getElementById('kaboom-terminal-iframe').src.includes('tok-1'), 'first boot mounts tok-1')
    assert.strictEqual(startCount, 1)

    // Daemon restarted -> token dead. The iframe exhausts reconnects and signals us.
    tokenAlive = false
    dispatchWindowEvent('message', {
      origin: 'http://localhost:7891',
      data: { source: 'kaboom-terminal', event: 'reconnect_exhausted', data: { attempts: 7 } }
    })

    await sleep(100) // let the async validate -> rebuild run
    assert.strictEqual(startCount, 2, 'reconnect_exhausted on a dead token must boot a fresh session')
    assert.ok(getElementById('kaboom-terminal-iframe').src.includes('tok-2'), 'the rebuilt panel uses the fresh token')
  })

  test('a flapping daemon stops rebuilding after the recovery ceiling and shows the no-session state (E-i)', async () => {
    // A daemon that stays up long enough for the 2s /terminal/validate but drops
    // the WS before onopen makes the iframe exhaust reconnects repeatedly. Each
    // exhaustion validates (true) and reloads the iframe — an endless thrash. Past
    // a bounded number of recoveries the parent must give up and drop to the
    // recoverable no-session state instead of re-looping forever.
    let startCount = 0
    sidepanelState.fetchHandler = ({ url }) => {
      if (url.endsWith('/terminal/start')) {
        startCount += 1
        return Promise.resolve(makeResponse(200, { session_id: `s-${startCount}`, token: `tok-${startCount}`, pid: 1 }))
      }
      if (url.includes('/terminal/validate?token=')) {
        return Promise.resolve(makeResponse(200, { valid: true })) // flap: token validates, but onopen keeps failing
      }
      throw new Error(`Unexpected fetch call: ${url}`)
    }

    const module = await import(`../../../extension/sidepanel.js?v=${++sidepanelState.importCounter}`)
    await module._terminalPanelForTests.bootTerminalPanel(true)
    assert.ok(getElementById('kaboom-terminal-iframe'), 'first boot mounts the terminal iframe')
    assert.strictEqual(startCount, 1)

    const flap = async () => {
      dispatchWindowEvent('message', {
        origin: 'http://localhost:7891',
        data: { source: 'kaboom-terminal', event: 'reconnect_exhausted', data: { attempts: 7 } }
      })
      await sleep(20) // let the async validate -> redraw run
    }

    // Under the ceiling: each exhaustion revalidates and reloads the same iframe.
    for (let i = 0; i < 3; i++) await flap()
    assert.ok(getElementById('kaboom-terminal-iframe'), 'still recovering under the ceiling — iframe present')
    assert.strictEqual(
      getElementById('kaboom-terminal-start-button'),
      null,
      'no no-session state while under the ceiling'
    )

    // One more exhaustion trips the ceiling: stop thrashing, show no-session state.
    await flap()
    assert.ok(
      getElementById('kaboom-terminal-start-button'),
      'past the ceiling the recoverable no-session state (Start terminal) is shown instead of another rebuild'
    )
    assert.strictEqual(getElementById('kaboom-terminal-iframe'), null, 'the flapping iframe is detached so the loop stops')
    assert.strictEqual(startCount, 1, 'the ceiling path does not start a new session (the flap kept the token valid)')
  })
})
