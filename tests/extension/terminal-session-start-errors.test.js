// @ts-nocheck
/**
 * @fileoverview terminal-session-start-errors.test.js — startSession must surface
 * the real reason a terminal failed to start.
 *
 * Regression: a non-409/non-503 rejection returned null after only a console.warn,
 * so the side panel rendered nothing at all — "clicking the terminal loads nothing".
 * And the 503 sandbox body's `detail` (the actual underlying error) was dropped,
 * leaving only the daemon's guess about the cause.
 */

import { beforeEach, describe, mock, test } from 'node:test'
import assert from 'node:assert'

let importCounter = 0

function jsonResponse(status, body) {
  return {
    ok: status >= 200 && status < 300,
    status,
    json: async () => body
  }
}

async function loadSession() {
  return import(`../../extension/content/ui/terminal-widget-session.js?v=${++importCounter}`)
}

describe('startSession failure reporting', () => {
  beforeEach(() => {
    mock.reset()
    globalThis.chrome = {
      storage: {
        local: { get: mock.fn(async () => ({})), set: mock.fn(async () => {}) },
        session: {
          get: mock.fn(async () => ({})),
          set: mock.fn(async () => {}),
          remove: mock.fn(async () => {})
        }
      }
    }
  })

  test('sandbox 503 passes the daemon detail through to the error handler', async () => {
    globalThis.fetch = mock.fn(async () => jsonResponse(503, {
      error: 'sandbox_restricted',
      message: 'The daemon could not spawn a terminal process: the OS denied fork/exec.',
      instruction: 'Run this command in a separate terminal:',
      command: 'kaboom-agentic-browser --stop && kaboom-agentic-browser --daemon',
      detail: 'start /bin/zsh: fork/exec /bin/zsh: operation not permitted'
    }))
    const { startSession } = await loadSession()
    const onError = mock.fn()

    const result = await startSession({}, onError)

    assert.strictEqual(result, null)
    assert.strictEqual(onError.mock.calls.length, 1)
    const [message, instruction, command] = onError.mock.calls[0].arguments
    assert.match(message, /fork\/exec \/bin\/zsh: operation not permitted/,
      'the underlying error is the fact — it must reach the user')
    assert.match(instruction, /separate terminal/)
    assert.match(command, /kaboom-agentic-browser --stop/)
  })

  test('a non-sandbox rejection still reaches the error handler so the panel is never blank', async () => {
    globalThis.fetch = mock.fn(async () => jsonResponse(500, {
      error: 'pty: maximum concurrent sessions reached: limit 10'
    }))
    const { startSession } = await loadSession()
    const onError = mock.fn()

    const result = await startSession({}, onError)

    assert.strictEqual(result, null)
    assert.strictEqual(onError.mock.calls.length, 1,
      'a silent null leaves the side panel empty with no explanation')
    const [message, instruction, command] = onError.mock.calls[0].arguments
    assert.match(message, /maximum concurrent sessions reached/)
    // Not a sandbox problem — restarting the daemon is not the remedy.
    assert.strictEqual(instruction, '')
    assert.strictEqual(command, '')
  })

  test('a transport failure reaches the error handler with the daemon hint', async () => {
    globalThis.fetch = mock.fn(async () => { throw new Error('daemon offline') })
    const { startSession } = await loadSession()
    const onError = mock.fn()

    const result = await startSession({}, onError)

    assert.strictEqual(result, null)
    assert.strictEqual(onError.mock.calls.length, 1)
    assert.match(onError.mock.calls[0].arguments[0], /daemon offline/)
  })

  test('409 still reconnects to the existing session rather than reporting an error', async () => {
    globalThis.fetch = mock.fn(async () => jsonResponse(409, {
      error: 'pty: session already exists: default',
      session_id: 'default',
      token: 'existing-token'
    }))
    const { startSession } = await loadSession()
    const onError = mock.fn()

    const result = await startSession({}, onError)

    assert.deepStrictEqual(result, { sessionId: 'default', token: 'existing-token' })
    assert.strictEqual(onError.mock.calls.length, 0)
  })
})
