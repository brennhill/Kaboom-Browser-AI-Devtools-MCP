// @ts-nocheck
/**
 * @fileoverview Terminal-panel write guards, queued I/O, boot races, and reset tests.
 */

import { beforeEach, describe, mock, test } from 'node:test'
import assert from 'node:assert'
import { StorageKey } from '../../extension/lib/constants.js'
import {
  dispatchWindowEvent,
  getElementById,
  getPostMessagePayloads,
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

  test('write guard waits while user is typing and flushes after blur', async () => {
    sidepanelState.fetchHandler = ({ url }) => {
      if (url.endsWith('/terminal/start')) {
        return Promise.resolve(makeResponse(200, {
          session_id: 'session-typing-guard',
          token: 'token-typing-guard',
          pid: 999
        }))
      }
      throw new Error(`Unexpected fetch call: ${url}`)
    }

    const module = await import(`../../extension/sidepanel.js?v=${++sidepanelState.importCounter}`)
    await module._terminalPanelForTests.bootTerminalPanel(true)

    const iframe = getElementById('kaboom-terminal-iframe')
    assert.ok(iframe, 'terminal iframe should exist')

    dispatchWindowEvent('message', {
      origin: 'http://localhost:7891',
      data: { source: 'kaboom-terminal', event: 'connected' }
    })
    dispatchWindowEvent('message', {
      origin: 'http://localhost:7891',
      data: { source: 'kaboom-terminal', event: 'focus', data: { focused: true } }
    })
    dispatchWindowEvent('message', {
      origin: 'http://localhost:7891',
      data: { source: 'kaboom-terminal', event: 'typing', data: { at: Date.now() } }
    })

    const callStart = iframe.contentWindow.postMessage.mock.calls.length
    module._terminalPanelForTests.writeToTerminal('queued command')

    await sleep(80)
    const whileTypingPayloads = getPostMessagePayloads(iframe, callStart)
    assert.strictEqual(whileTypingPayloads.filter((payload) => payload?.command === 'write').length, 0)

    dispatchWindowEvent('message', {
      origin: 'http://localhost:7891',
      data: { source: 'kaboom-terminal', event: 'focus', data: { focused: false } }
    })

    await sleep(800)
    const flushedPayloads = getPostMessagePayloads(iframe, callStart)
    const flushedWrites = flushedPayloads
      .filter((payload) => payload?.command === 'write')
      .map((payload) => payload.text)

    assert.deepStrictEqual(flushedWrites, ['queued command', '\r'])
  })

  test('a write that arrives before the socket is connected is queued and replayed, not dropped', async () => {
    sidepanelState.fetchHandler = ({ url }) => {
      if (url.endsWith('/terminal/start')) {
        return Promise.resolve(makeResponse(200, {
          session_id: 'session-preconnect',
          token: 'token-preconnect',
          pid: 999
        }))
      }
      throw new Error(`Unexpected fetch call: ${url}`)
    }

    const module = await import(`../../extension/sidepanel.js?v=${++sidepanelState.importCounter}`)
    await module._terminalPanelForTests.bootTerminalPanel(true)

    const iframe = getElementById('kaboom-terminal-iframe')
    assert.ok(iframe, 'terminal iframe should exist')

    // No 'connected' event yet -> the WebSocket is not OPEN. A write here used to
    // be sent straight to the iframe (which drops it) while the submit fired a
    // bare Enter later -> the AI's text vanished. It must be QUEUED instead.
    const callStart = iframe.contentWindow.postMessage.mock.calls.length
    module._terminalPanelForTests.writeToTerminal('deploy the service')

    await sleep(20)
    const preConnectWrites = getPostMessagePayloads(iframe, callStart)
      .filter((p) => p?.command === 'write')
      .map((p) => p.text)
    assert.deepStrictEqual(preConnectWrites, [], 'nothing may be written to a not-yet-connected socket')

    // Socket comes up -> the queued write must be replayed (text first, then Enter).
    dispatchWindowEvent('message', {
      origin: 'http://localhost:7891',
      data: { source: 'kaboom-terminal', event: 'connected' }
    })

    await sleep(800)
    const afterConnectWrites = getPostMessagePayloads(iframe, callStart)
      .filter((p) => p?.command === 'write')
      .map((p) => p.text)
    assert.deepStrictEqual(afterConnectWrites, ['deploy the service', '\r'],
      'the queued write must be replayed in full once connected, not lost with a bare Enter')
  })

  test('every message posted to the iframe carries target:"kaboom-terminal"', async () => {
    // Regression for the eb248ff6 refactor, which dropped `target` from
    // notifyIframe. terminal.html's listener is `if (event.data.target !==
    // 'kaboom-terminal') return`, so a target-less message is silently dropped —
    // the annotation nudge (and every agent write/focus/redraw) never reached the
    // shell, while user keystrokes hid it by going straight to the socket.
    sidepanelState.fetchHandler = ({ url }) => {
      if (url.endsWith('/terminal/start')) {
        return Promise.resolve(makeResponse(200, {
          session_id: 'session-target',
          token: 'token-target',
          pid: 777
        }))
      }
      throw new Error(`Unexpected fetch call: ${url}`)
    }

    const module = await import(`../../extension/sidepanel.js?v=${++sidepanelState.importCounter}`)
    await module._terminalPanelForTests.bootTerminalPanel(true)

    const iframe = getElementById('kaboom-terminal-iframe')
    assert.ok(iframe, 'terminal iframe should exist')

    dispatchWindowEvent('message', {
      origin: 'http://localhost:7891',
      data: { source: 'kaboom-terminal', event: 'connected' }
    })

    const callStart = iframe.contentWindow.postMessage.mock.calls.length
    module._terminalPanelForTests.writeToTerminal('Check the kaboom annotations and add each comment to your todo list, then work through them')
    await sleep(800)

    const payloads = getPostMessagePayloads(iframe, callStart)
    const writes = payloads.filter((p) => p?.command === 'write')
    assert.ok(writes.length > 0, 'the write must reach the iframe')
    for (const payload of payloads) {
      assert.strictEqual(
        payload?.target,
        'kaboom-terminal',
        `iframe message ${JSON.stringify(payload)} must carry target:'kaboom-terminal' or the iframe drops it`
      )
    }
  })

  test('two concurrent forceFresh boots keep writes on the on-screen iframe (boot race)', async () => {
    // Double-clicked "Start", or a rapid folder re-pick, fires bootTerminalPanel(true)
    // twice while the first is still awaiting the network. Without serialization,
    // createPanelShell() from the 2nd boot rebinds state.iframeEl to a fresh iframe
    // while mountPanel() early-returns (rootEl already set) — so the VISIBLE terminal
    // is the 1st iframe but state.iframeEl points at the detached 2nd, and every write
    // vanishes into the off-screen frame with no error. The write below must reach the
    // iframe that is actually mounted.
    let starts = 0
    sidepanelState.fetchHandler = ({ url }) => {
      if (url.endsWith('/terminal/start')) {
        starts += 1
        return Promise.resolve(makeResponse(200, {
          session_id: `session-race-${starts}`,
          token: `token-race-${starts}`,
          pid: 900 + starts
        }))
      }
      if (url.endsWith('/terminal/validate')) {
        return Promise.resolve(makeResponse(200, { valid: true }))
      }
      throw new Error(`Unexpected fetch call: ${url}`)
    }

    const module = await import(`../../extension/sidepanel.js?v=${++sidepanelState.importCounter}`)

    // Fire both boots without awaiting the first — this is the race.
    const boot1 = module._terminalPanelForTests.bootTerminalPanel(true)
    const boot2 = module._terminalPanelForTests.bootTerminalPanel(true)
    await Promise.all([boot1, boot2])

    const iframe = getElementById('kaboom-terminal-iframe')
    assert.ok(iframe, 'exactly one terminal iframe is mounted after the raced boots')

    dispatchWindowEvent('message', {
      origin: 'http://localhost:7891',
      data: { source: 'kaboom-terminal', event: 'connected' }
    })

    const callStart = iframe.contentWindow.postMessage.mock.calls.length
    module._terminalPanelForTests.writeToTerminal('reach the visible frame')

    const writes = getPostMessagePayloads(iframe, callStart)
      .filter((p) => p?.command === 'write')
      .map((p) => p.text)
    assert.ok(
      writes.includes('reach the visible frame'),
      'the write must land on the mounted iframe (state.iframeEl), not a detached one from a raced boot'
    )
  })

  test('a hung prior boot does not block a fresh Start (generation supersede)', async () => {
    let mode = 'hang'
    let starts = 0
    sidepanelState.fetchHandler = ({ url }) => {
      if (url.endsWith('/terminal/validate')) {
        return Promise.resolve(makeResponse(200, { valid: false }))
      }
      if (url.endsWith('/terminal/start')) {
        if (mode === 'hang') return new Promise(() => {}) // never resolves — a dead/hung daemon
        starts += 1
        return Promise.resolve(makeResponse(200, { session_id: 's', token: `tok-${starts}`, pid: 1 }))
      }
      throw new Error(`Unexpected fetch call: ${url}`)
    }
    const module = await import(`../../extension/sidepanel.js?v=${++sidepanelState.importCounter}`)

    // First Start hangs on /terminal/start (daemon not answering). Do NOT await it —
    // that is the whole point: with the old bootChain, the next Start would await this
    // forever and the button would appear dead.
    const hung = module._terminalPanelForTests.bootTerminalPanel(true)
    await sleep(20) // let it reach the hung fetch

    // A fresh Start must supersede the stuck boot and actually open a terminal.
    mode = 'ok'
    await module._terminalPanelForTests.bootTerminalPanel(true)

    assert.strictEqual(starts, 1, 'the fresh Start started a session despite the hung prior boot')
    const iframe = getElementById('kaboom-terminal-iframe')
    assert.ok(iframe && iframe.src.includes('tok-1'), 'the fresh terminal is mounted, not blocked by the hung boot')

    void hung // intentionally never settles
  })

  test('resetPanelUi clears the panel so a plain boot re-boots (encapsulated state)', async () => {
    let starts = 0
    sidepanelState.fetchHandler = ({ url }) => {
      if (url.endsWith('/terminal/start')) {
        starts += 1
        return Promise.resolve(makeResponse(200, { session_id: 's', token: `tok-${starts}`, pid: 1 }))
      }
      if (url.endsWith('/terminal/validate')) {
        return Promise.resolve(makeResponse(200, { valid: false })) // force a fresh startSession
      }
      throw new Error(`Unexpected fetch call: ${url}`)
    }
    const module = await import(`../../extension/sidepanel.js?v=${++sidepanelState.importCounter}`)

    await module._terminalPanelForTests.bootTerminalPanel(true)
    assert.strictEqual(starts, 1, 'first boot starts a session')

    // A plain (non-force) boot is a no-op while the panel reports ready.
    await module._terminalPanelForTests.bootTerminalPanel(false)
    assert.strictEqual(starts, 1, 'a plain boot no-ops while panelReady is set')

    // Resetting the encapsulated panel state (panelReady among it) makes a plain
    // boot behave as first-boot again — the whole point of one resettable object.
    module._terminalPanelForTests.resetPanelUi()
    await module._terminalPanelForTests.bootTerminalPanel(false)
    assert.strictEqual(starts, 2, 'after resetPanelUi, a plain boot re-boots')
  })

  test('terminal submit re-guards if focus returns before auto-enter', async () => {
    sidepanelState.fetchHandler = ({ url }) => {
      if (url.endsWith('/terminal/start')) {
        return Promise.resolve(makeResponse(200, {
          session_id: 'session-re-guard',
          token: 'token-re-guard',
          pid: 999
        }))
      }
      throw new Error(`Unexpected fetch call: ${url}`)
    }

    const module = await import(`../../extension/sidepanel.js?v=${++sidepanelState.importCounter}`)
    await module._terminalPanelForTests.bootTerminalPanel(true)

    const iframe = getElementById('kaboom-terminal-iframe')
    assert.ok(iframe, 'terminal iframe should exist')

    dispatchWindowEvent('message', {
      origin: 'http://localhost:7891',
      data: { source: 'kaboom-terminal', event: 'connected' }
    })
    dispatchWindowEvent('message', {
      origin: 'http://localhost:7891',
      data: { source: 'kaboom-terminal', event: 'focus', data: { focused: false } }
    })

    const callStart = iframe.contentWindow.postMessage.mock.calls.length
    module._terminalPanelForTests.writeToTerminal('submit guard command')

    await sleep(80)
    dispatchWindowEvent('message', {
      origin: 'http://localhost:7891',
      data: { source: 'kaboom-terminal', event: 'focus', data: { focused: true } }
    })
    dispatchWindowEvent('message', {
      origin: 'http://localhost:7891',
      data: { source: 'kaboom-terminal', event: 'typing', data: { at: Date.now() } }
    })

    await sleep(680)
    const blockedPayloads = getPostMessagePayloads(iframe, callStart)
    const blockedWrites = blockedPayloads
      .filter((payload) => payload?.command === 'write')
      .map((payload) => payload.text)
    assert.deepStrictEqual(blockedWrites, ['submit guard command'])

    dispatchWindowEvent('message', {
      origin: 'http://localhost:7891',
      data: { source: 'kaboom-terminal', event: 'focus', data: { focused: false } }
    })

    await sleep(320)
    const releasedPayloads = getPostMessagePayloads(iframe, callStart)
    const releasedWrites = releasedPayloads
      .filter((payload) => payload?.command === 'write')
      .map((payload) => payload.text)
    assert.deepStrictEqual(releasedWrites, ['submit guard command', '\r'])
  })
})
