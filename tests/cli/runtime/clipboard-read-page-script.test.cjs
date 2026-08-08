// clipboard-read-page-script.test.cjs — Behavioral contract for the bounded clipboard-read page script.
// Docs: docs/features/feature/interact-explore/index.md
'use strict'

const assert = require('node:assert/strict')
const { readFileSync } = require('node:fs')
const { join } = require('node:path')
const { describe, test } = require('node:test')
const vm = require('node:vm')

const SCRIPT_PATH = join('cmd', 'browser-agent', 'internal', 'toolinteract', 'pagescripts', 'clipboard-read.js')
const SCRIPT = readFileSync(join(process.cwd(), SCRIPT_PATH), 'utf8')

/** Let every already-queued microtask run before driving the next fixture step. */
async function drainMicrotasks() {
  for (let i = 0; i < 50; i += 1) {
    await Promise.resolve()
  }
}

/**
 * Run the real page script against controlled page APIs.
 * Timers and page-lifecycle events are driven explicitly so every outcome is
 * deterministic — no sleeps, no reliance on real clipboard permissions.
 */
function runClipboardRead({ permissionState, permissionsThrows = false, readText }) {
  const readTextCalls = []
  const listeners = new Map()
  const deadlines = []
  const context = vm.createContext({
    navigator: {
      permissions: {
        query: async (descriptor) => {
          if (permissionsThrows) throw new TypeError(`unsupported permission: ${descriptor.name}`)
          assert.equal(descriptor.name, 'clipboard-read')
          return { state: permissionState }
        }
      },
      clipboard: {
        readText: () => {
          readTextCalls.push(Date.now)
          return readText()
        }
      }
    },
    window: {
      addEventListener: (type, handler) => listeners.set(type, handler),
      removeEventListener: (type) => listeners.delete(type)
    },
    setTimeout: (handler) => deadlines.push(handler),
    clearTimeout: () => deadlines.splice(0, deadlines.length)
  })

  return {
    result: vm.runInContext(SCRIPT, context),
    readTextCalls,
    async expireDeadline() {
      await drainMicrotasks()
      assert.equal(deadlines.length, 1, 'the page script must arm exactly one bounded deadline')
      deadlines.splice(0, deadlines.length).forEach((handler) => handler())
    },
    async firePageEvent(type) {
      await drainMicrotasks()
      const handler = listeners.get(type)
      assert.ok(handler, `expected the page script to listen for ${type}`)
      handler()
    }
  }
}

function rejectWith(name, message) {
  const error = new Error(message)
  error.name = name
  return () => Promise.reject(error)
}

describe('clipboard read page script', () => {
  test('returns the clipboard text when the permission is already granted', async () => {
    const run = runClipboardRead({ permissionState: 'granted', readText: async () => 'Kaboom connected coverage' })
    const result = await run.result
    assert.equal(result.text, 'Kaboom connected coverage')
    assert.equal(result.permission_state, 'granted')
    assert.equal(result.error, undefined)
  })

  test('classifies a denied permission without touching the clipboard API', async () => {
    const run = runClipboardRead({ permissionState: 'denied', readText: async () => 'never read' })
    const result = await run.result
    assert.equal(result.error, 'clipboard_permission_denied')
    assert.equal(result.permission_state, 'denied')
    assert.equal(run.readTextCalls.length, 0)
  })

  test('classifies an unanswered permission prompt without opening one', async () => {
    // Calling readText() in the `prompt` state makes Chrome raise a modal the
    // agent cannot answer; the promise then hangs until the executor's generic
    // execution_timeout fires and the prompt strands every later action.
    const run = runClipboardRead({ permissionState: 'prompt', readText: async () => 'never read' })
    const result = await run.result
    assert.equal(result.error, 'clipboard_permission_prompt_required')
    assert.equal(result.permission_state, 'prompt')
    assert.equal(run.readTextCalls.length, 0)
  })

  test('still attempts the read when the permissions API cannot describe clipboard-read', async () => {
    const run = runClipboardRead({ permissionsThrows: true, readText: async () => 'from an older browser' })
    const result = await run.result
    assert.equal(result.text, 'from an older browser')
    assert.equal(result.permission_state, 'unknown')
    assert.equal(run.readTextCalls.length, 1)
  })

  test('bounds a granted read that never settles', async () => {
    const run = runClipboardRead({ permissionState: 'granted', readText: () => new Promise(() => {}) })
    await run.expireDeadline()
    const result = await run.result
    assert.equal(result.error, 'clipboard_read_timeout')
    assert.equal(result.permission_state, 'granted')
  })

  test('classifies a read cancelled by page navigation', async () => {
    const run = runClipboardRead({ permissionState: 'granted', readText: () => new Promise(() => {}) })
    await run.firePageEvent('pagehide')
    const result = await run.result
    assert.equal(result.error, 'clipboard_read_navigation_cancelled')
    assert.equal(result.permission_state, 'granted')
  })

  test('classifies a read whose execution context was destroyed', async () => {
    const run = runClipboardRead({
      permissionState: 'granted',
      readText: rejectWith('Error', 'Execution context was destroyed.')
    })
    const result = await run.result
    assert.equal(result.error, 'clipboard_read_context_destroyed')
  })

  test('classifies an unfocused document separately from a denied permission', async () => {
    const run = runClipboardRead({
      permissionState: 'granted',
      readText: rejectWith('NotAllowedError', 'Document is not focused.')
    })
    const result = await run.result
    assert.equal(result.error, 'clipboard_document_not_focused')
  })

  test('classifies a runtime NotAllowedError as a denied permission', async () => {
    const run = runClipboardRead({
      permissionState: 'granted',
      readText: rejectWith('NotAllowedError', 'Read permission denied.')
    })
    const result = await run.result
    assert.equal(result.error, 'clipboard_permission_denied')
  })

  test('keeps an unclassified failure bounded and redacted', async () => {
    const run = runClipboardRead({
      permissionState: 'granted',
      readText: rejectWith('DataError', `clipboard blew up: ${'s'.repeat(500)}`)
    })
    const result = await run.result
    assert.equal(result.error, 'clipboard_read_failed')
    assert.equal(result.reason, 'DataError')
    assert.ok(result.detail.length <= 200, `detail must be bounded, got ${result.detail.length}`)
    assert.equal(result.text, undefined, 'a failure must never carry clipboard contents')
  })

  test('no classified outcome leaks clipboard contents or unbounded detail', async () => {
    for (const outcome of ['denied', 'prompt']) {
      const run = runClipboardRead({ permissionState: outcome, readText: async () => 'secret' })
      const result = await run.result
      assert.equal(result.text, undefined)
      assert.ok(result.message.length <= 200)
    }
  })
})
