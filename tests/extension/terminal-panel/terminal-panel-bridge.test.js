// @ts-nocheck
/**
 * terminal-panel-bridge.test.js — the annotation→terminal write must not vanish
 * silently. writeToTerminal sends a runtime message to the side-panel DOCUMENT;
 * only that document acks it (the background never replies to this type), so a
 * missing ack means no panel received the write — typically the panel was closed
 * with Chrome's own X while the TERMINAL_UI_STATE mirror we gate on stayed 'open'
 * (rule 18). The bridge must fail loud (toast, rule 25) AND reconcile the stale
 * mirror so isTerminalVisible() stops reporting a panel that is gone.
 *
 * A single miss is retried once first: it can be the panel's brief boot window
 * (document up, onMessage listener not installed yet). Only when the retry also
 * misses do we conclude the panel is gone and reconcile — so a boot-window race
 * cannot falsely wedge the mirror to false.
 */
import { beforeEach, afterEach, describe, test } from 'node:test'
import assert from 'node:assert'

import {
  initTerminalPanelBridge,
  isTerminalVisible,
  writeToTerminal,
  _terminalPanelBridgeForTests
} from '../../../extension/content/ui/terminal-panel-bridge.js'

let sendMessageImpl

function installEnv() {
  // Minimal DOM so showActionToast runs headless.
  const makeEl = () => ({ id: '', className: '', textContent: '', style: {}, appendChild() {}, remove() {} })
  globalThis.document = {
    body: makeEl(),
    documentElement: makeEl(),
    head: makeEl(),
    getElementById: () => null,
    createElement: () => makeEl()
  }
  globalThis.requestAnimationFrame = () => {}

  globalThis.chrome = {
    runtime: {
      id: 'ext',
      getURL: () => '',
      sendMessage: (msg) => sendMessageImpl(msg)
    },
    storage: {
      session: {
        get: (key, cb) => {
          const result = { [key]: 'open' } // panel mirror says "open"
          cb?.(result)
          return Promise.resolve(result)
        },
        set: (_v, cb) => { cb?.(); return Promise.resolve() },
        remove: (_k, cb) => { cb?.(); return Promise.resolve() }
      },
      onChanged: { addListener: () => {} }
    }
  }
}

// Flush pending microtasks so the sendMessage promise's .then runs.
async function flush() {
  await Promise.resolve()
  await Promise.resolve()
}

// Flush a macrotask (the retry setTimeout) plus microtasks.
async function macroFlush() {
  await new Promise((resolve) => setTimeout(resolve, 0))
  await flush()
}

describe('terminal-panel-bridge writeToTerminal delivery', () => {
  beforeEach(() => {
    _terminalPanelBridgeForTests.reset()
    installEnv()
  })

  afterEach(() => {
    _terminalPanelBridgeForTests.reset()
  })

  test('a delivered (acked) write leaves the panel visible and surfaces nothing', async () => {
    sendMessageImpl = () => Promise.resolve({ received: true })
    await initTerminalPanelBridge()
    assert.equal(isTerminalVisible(), true, 'mirror says open -> visible')

    writeToTerminal('nudge')
    await flush()

    assert.equal(isTerminalVisible(), true, 'a delivered write must not flip visibility')
  })

  test('an unacked write reconciles to false only AFTER the retry also misses', async () => {
    sendMessageImpl = () => Promise.resolve(undefined)
    await initTerminalPanelBridge()
    assert.equal(isTerminalVisible(), true, 'stale mirror still says open before the write')

    _terminalPanelBridgeForTests.setWriteRetryDelay(0)
    let calls = 0
    let sent = null
    sendMessageImpl = (msg) => { calls += 1; sent = msg; return Promise.resolve({}) } // reachable, never acks
    writeToTerminal('nudge')
    await flush()

    assert.ok(sent && sent.type === 'terminal_panel_write', 'the write is attempted')
    assert.equal(calls, 1, 'first attempt only so far')
    assert.equal(isTerminalVisible(), true,
      'a single miss must NOT reconcile — it can be the panel boot window')

    await macroFlush() // the retry fires and also misses
    assert.equal(calls, 2, 'the write is retried exactly once')
    assert.equal(isTerminalVisible(), false,
      'after the retry also misses, reconcile the stale visibility mirror to false')
  })

  test('a boot-window miss that acks on retry does NOT reconcile (panel finished booting)', async () => {
    sendMessageImpl = () => Promise.resolve(undefined)
    await initTerminalPanelBridge()
    assert.equal(isTerminalVisible(), true)

    _terminalPanelBridgeForTests.setWriteRetryDelay(0)
    let calls = 0
    // First send lands before the panel's listener is installed (no ack); the
    // retry lands after it is (acked).
    sendMessageImpl = () => { calls += 1; return Promise.resolve(calls >= 2 ? { received: true } : {}) }
    writeToTerminal('nudge')
    await flush()
    await macroFlush() // retry acks

    assert.equal(calls, 2, 'the boot-window miss is retried once')
    assert.equal(isTerminalVisible(), true,
      'a retry that acks means the panel is present — must NOT reconcile the mirror')
  })

  test('a retry scheduled then teardown+reopen does not re-send or reconcile the fresh panel (finding J)', async () => {
    sendMessageImpl = () => Promise.resolve(undefined)
    await initTerminalPanelBridge()
    assert.equal(isTerminalVisible(), true)

    _terminalPanelBridgeForTests.setWriteRetryDelay(30)
    let calls = 0
    sendMessageImpl = () => { calls += 1; return Promise.resolve({}) } // reachable, never acks -> schedules a retry
    writeToTerminal('stale nudge')
    await flush()
    assert.equal(calls, 1, 'the first send happened and a retry is scheduled')

    // The panel is torn down and a fresh one opens BEFORE the retry fires. An
    // untracked retry timer would still fire against the new session.
    _terminalPanelBridgeForTests.reset()
    await initTerminalPanelBridge() // fresh panel: mirror says 'open' -> visible
    assert.equal(isTerminalVisible(), true, 'the fresh panel is visible')

    await new Promise((resolve) => setTimeout(resolve, 60)) // past the 30ms retry delay
    await flush()

    assert.equal(calls, 1, 'a retry after teardown must NOT re-send (stale nudge into a new session)')
    assert.equal(isTerminalVisible(), true, 'a stale retry must NOT reconcile the FRESH panel to hidden')
  })

  test('a synchronous send throw (context invalidated) is surfaced but does NOT reconcile', async () => {
    sendMessageImpl = () => Promise.resolve({ received: true })
    await initTerminalPanelBridge()
    assert.equal(isTerminalVisible(), true)

    // Extension context invalidated — sendMessage throws synchronously. The panel
    // state is unknown, so visibility must be left as-is (not reconciled).
    sendMessageImpl = () => { throw new Error('Extension context invalidated') }
    writeToTerminal('nudge')
    await flush()

    assert.equal(isTerminalVisible(), true,
      'an ambiguous transport throw must not reconcile visibility (panel state unknown)')
  })
})
