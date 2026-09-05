// @ts-nocheck
/**
 * @fileoverview cdp-type-modifiers.test.js — kaboom-wpyt regression.
 *
 * The interact schema advertises `modifiers` on type, so a caller asking for ctrl+a expects a
 * select-all. Two ways that promise can be broken silently, both covered here:
 *
 *   1. The modifier bits never reach Input.dispatchKeyEvent — the page sees an unmodified
 *      keystroke and the shortcut never fires, while the call reports success.
 *   2. The bits reach it but `text` is sent alongside them — Chrome inserts the character
 *      regardless of the modifier bits, so ctrl+a types an "a" instead of selecting all.
 *
 * The DOM fallback (dispatch:"dom", frame- and nth-scoped calls) has the same two failure modes,
 * so the modifier names must also reach the injected primitive's options.
 */

import { test, describe, before, beforeEach, mock } from 'node:test'
import assert from 'node:assert'

const KEYS = '../../../../extension/background/dom/cdp/cdp-key-mappings.js'
const DISPATCH = '../../../../extension/background/dom/cdp/cdp-dispatch.js'

/** A lease that records every CDP command instead of talking to a browser. */
function recordingLease() {
  const sent = []
  return {
    sent,
    lease: {
      send: async (method, params) => {
        sent.push({ method, params })
        return undefined
      },
      ensureDomain: async () => undefined,
      release: () => undefined
    }
  }
}

describe('keyEventsForText', () => {
  let keyEventsForText

  before(async () => {
    ;({ keyEventsForText } = await import(KEYS))
  })

  test('holds the caller modifier on every key event of the sequence', () => {
    const events = keyEventsForText('ab', ['ctrl'])
    assert.equal(events.length, 4)
    for (const event of events) {
      assert.equal(event.modifiers, 2, `${event.type} ${event.key} lost the ctrl bit`)
    }
  })

  test('combines the held modifier with the shift a capital letter already needs', () => {
    const [down] = keyEventsForText('A', ['alt'])
    assert.equal(down.modifiers, 1 | 8)
  })

  test('a shortcut sends no text — ctrl+a selects all, it does not type an "a"', () => {
    const [down] = keyEventsForText('a', ['ctrl'])
    assert.equal(down.type, 'keyDown')
    assert.equal(down.modifiers, 2)
    assert.ok(!('text' in down), `keyDown carried text ${JSON.stringify(down.text)}; Chrome would insert it`)
    assert.ok(!('unmodifiedText' in down))
  })

  test('shift alone is still typing, so the text stays', () => {
    const [down] = keyEventsForText('a', ['shift'])
    assert.equal(down.text, 'a')
    assert.equal(down.modifiers, 8)
  })

  test('plain text is unchanged: shift for capitals, nothing else', () => {
    const events = keyEventsForText('Ab')
    assert.equal(events[0].modifiers, 8)
    assert.equal(events[0].text, 'A')
    assert.equal(events[0].unmodifiedText, 'a')
    assert.equal(events[2].modifiers, 0)
    assert.equal(events[2].text, 'b')
  })
})

describe('cdpDispatchKeySequence', () => {
  let cdpDispatchKeySequence

  before(async () => {
    ;({ cdpDispatchKeySequence } = await import(DISPATCH))
  })

  test('the modifier reaches the dispatched Input.dispatchKeyEvent', async () => {
    const { sent, lease } = recordingLease()
    await cdpDispatchKeySequence(lease, 'a', ['ctrl'])

    assert.deepEqual(
      sent.map((call) => call.method),
      ['Input.dispatchKeyEvent', 'Input.dispatchKeyEvent']
    )
    assert.equal(sent[0].params.type, 'keyDown')
    assert.equal(sent[0].params.modifiers, 2)
    assert.equal(sent[1].params.type, 'keyUp')
    assert.equal(sent[1].params.modifiers, 2)
    assert.equal(sent[0].params.text, undefined)
  })

  test('an unmodified sequence still dispatches its characters as text', async () => {
    const { sent, lease } = recordingLease()
    await cdpDispatchKeySequence(lease, 'hi')

    assert.equal(sent.length, 4)
    assert.equal(sent[0].params.text, 'h')
    assert.equal(sent[0].params.modifiers, 0)
    assert.equal(sent[2].params.text, 'i')
  })
})

/**
 * A DOM small enough to run the injected `type` primitive in Node. Only the surface the type
 * path touches is modelled: resolution, overlay probing, mutation tracking and the input node.
 */
function installFakeDOM() {
  const dispatched = []

  class FakeEvent {
    constructor(type, init = {}) {
      this.type = type
      Object.assign(this, init)
    }
  }

  class FakeHTMLElement extends FakeEvent {}
  class FakeInput {
    constructor() {
      this.tagName = 'INPUT'
      this.type = 'text'
      this.id = 'field'
      this.className = ''
      this.isConnected = true
      this.isContentEditable = false
      this.parentElement = null
      this.offsetParent = {}
      this.textContent = ''
      this.children = []
      this._value = ''
    }
    get value() {
      return this._value
    }
    set value(next) {
      this._value = next
    }
    focus() {}
    getAttribute() {
      return null
    }
    hasAttribute() {
      return false
    }
    closest() {
      return null
    }
    matches() {
      return false
    }
    contains() {
      return false
    }
    getBoundingClientRect() {
      return { x: 0, y: 0, top: 0, left: 0, right: 100, bottom: 20, width: 100, height: 20 }
    }
    dispatchEvent(event) {
      dispatched.push(event)
      return true
    }
  }

  const node = new FakeInput()
  const body = { children: [], scrollHeight: 800, getAttribute: () => null, parentElement: null }

  const previous = {}
  const globals = {
    document: {
      body,
      documentElement: { clientWidth: 1024, clientHeight: 768, scrollHeight: 800, children: [] },
      querySelector: (sel) => (sel === '#field' ? node : null),
      querySelectorAll: (sel) => (sel === '#field' ? [node] : []),
      getSelection: () => null
    },
    window: { innerWidth: 1024, innerHeight: 768, scrollX: 0, scrollY: 0 },
    getComputedStyle: () => ({ visibility: 'visible', display: 'block', position: 'static', zIndex: 'auto' }),
    MutationObserver: class {
      observe() {}
      disconnect() {}
    },
    KeyboardEvent: FakeEvent,
    InputEvent: FakeEvent,
    Event: FakeEvent,
    HTMLElement: FakeHTMLElement,
    HTMLInputElement: FakeInput,
    HTMLTextAreaElement: class {},
    HTMLSelectElement: class {},
    CSS: { escape: (value) => value }
  }
  for (const [name, value] of Object.entries(globals)) {
    previous[name] = globalThis[name]
    globalThis[name] = value
  }

  return {
    node,
    dispatched,
    restore() {
      for (const [name, value] of Object.entries(previous)) {
        if (value === undefined) delete globalThis[name]
        else globalThis[name] = value
      }
    }
  }
}

describe('the injected type primitive honors held modifiers', () => {
  let domPrimitiveForm
  let dom

  before(async () => {
    ;({ domPrimitiveForm } = await import('../../../../extension/background/dom/primitives/dom-primitives-form.js'))
  })

  beforeEach(() => {
    dom = installFakeDOM()
  })

  test('a held ctrl reaches every KeyboardEvent the page receives', async () => {
    const result = await domPrimitiveForm('type', '#field', { text: 'a', modifiers: ['ctrl'] })

    const keydown = dom.dispatched.find((event) => event.type === 'keydown')
    assert.ok(keydown, 'no keydown reached the element')
    assert.equal(keydown.ctrlKey, true, 'the page saw an unmodified keystroke, so ctrl+a never fired')
    assert.equal(keydown.key, 'a')
    assert.equal(result.success, true)
    dom.restore()
  })

  test('ctrl+a selects the field instead of typing an "a" into it', async () => {
    const result = await domPrimitiveForm('type', '#field', { text: 'a', modifiers: ['ctrl'] })

    assert.equal(dom.node.value, '', 'the shortcut typed its own letter into the field')
    assert.equal(
      dom.dispatched.some((event) => event.type === 'input'),
      false,
      'a shortcut fired an input event, so the page thinks the value changed'
    )
    assert.equal(result.insertion_strategy, 'modified_keystroke')
    dom.restore()
  })

  test('alt and cmd map to their own flags', async () => {
    await domPrimitiveForm('type', '#field', { text: 'a', modifiers: ['alt', 'cmd'] })

    const keydown = dom.dispatched.find((event) => event.type === 'keydown')
    assert.equal(keydown.altKey, true)
    assert.equal(keydown.metaKey, true)
    assert.equal(keydown.ctrlKey, false)
    dom.restore()
  })

  test('without modifiers the text is still typed and the value still set', async () => {
    const result = await domPrimitiveForm('type', '#field', { text: 'hi', clear: true })

    const keydown = dom.dispatched.find((event) => event.type === 'keydown')
    assert.equal(keydown.ctrlKey, false)
    assert.equal(keydown.altKey, false)
    assert.equal(keydown.metaKey, false)
    assert.equal(dom.node.value, 'hi')
    assert.equal(result.insertion_strategy, 'native_setter')
    dom.restore()
  })
})

describe('the DOM fallback receives the modifier names', () => {
  let executeDOMAction
  let injected

  beforeEach(async () => {
    injected = []
    globalThis.chrome = {
      scripting: {
        executeScript: mock.fn((options) => {
          injected.push(options)
          return Promise.resolve([{ frameId: 0, result: { success: true, action: 'type', selector: '#field' } }])
        })
      },
      storage: {
        local: {
          get: mock.fn(() => Promise.resolve({})),
          set: mock.fn(() => Promise.resolve()),
          remove: mock.fn(() => Promise.resolve())
        }
      }
    }
    ;({ executeDOMAction } = await import('../../../../extension/background/dom/dom-dispatch.js'))
  })

  test('type forwards modifiers into the injected primitive options', async () => {
    await executeDOMAction(
      {
        id: 'query-type',
        correlation_id: 'correlation-type',
        type: 'dom_action',
        params: JSON.stringify({
          action: 'type',
          selector: '#field',
          text: 'a',
          dispatch: 'dom',
          modifiers: ['ctrl']
        }),
        created_at: Date.now()
      },
      1,
      { id: 'test-client' },
      mock.fn(),
      mock.fn()
    )

    assert.equal(injected.length, 1)
    const options = injected[0].args?.[2]
    assert.deepEqual(
      options?.modifiers,
      ['ctrl'],
      'the injected type primitive never saw the modifier, so the page cannot see it either'
    )
  })
})
