// @ts-nocheck
/**
 * terminal-panel-bridge.test.js — the annotation→terminal write must not vanish
 * silently. writeToTerminal sends a runtime message to the side-panel DOCUMENT;
 * only that document acks it (the background never replies to this type), so a
 * missing ack means no panel received the write — typically the panel was closed
 * with Chrome's own X while the TERMINAL_UI_STATE mirror we gate on stayed 'open'
 * (rule 18). The bridge must fail loud (toast, rule 25) AND reconcile the stale
 * mirror so isTerminalVisible() stops reporting a panel that is gone.
 */
import { beforeEach, afterEach, describe, test } from 'node:test'
import assert from 'node:assert'

import {
  initTerminalPanelBridge,
  isTerminalVisible,
  writeToTerminal,
  _terminalPanelBridgeForTests
} from '../../extension/content/ui/terminal-panel-bridge.js'

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

  test('an unacked write (panel gone / stale mirror) reconciles visibility to false', async () => {
    // Panel document is not there: no ack in the response.
    sendMessageImpl = () => Promise.resolve(undefined)
    await initTerminalPanelBridge()
    assert.equal(isTerminalVisible(), true, 'stale mirror still says open before the write')

    let sent = null
    sendMessageImpl = (msg) => { sent = msg; return Promise.resolve({}) } // reachable but no ack
    writeToTerminal('nudge')
    await flush()

    assert.ok(sent && sent.type === 'terminal_panel_write', 'the write is still attempted')
    assert.equal(isTerminalVisible(), false,
      'a write no panel acked must reconcile the stale visibility mirror to false')
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
