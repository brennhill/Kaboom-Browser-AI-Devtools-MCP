// @ts-nocheck
/**
 * @fileoverview terminal-widget-session-branding.test.js — Verifies Kaboom daemon guidance in terminal session failures.
 */

import { beforeEach, describe, mock, test } from 'node:test'
import assert from 'node:assert'

describe('terminal widget session branding', () => {
  beforeEach(() => {
    mock.reset()
    globalThis.chrome = {
      storage: {
        local: {
          get: mock.fn(async () => ({})),
          set: mock.fn(async () => {})
        },
        session: {
          get: mock.fn(async () => ({})),
          set: mock.fn(async () => {}),
          remove: mock.fn(async () => {})
        }
      }
    }
    globalThis.fetch = mock.fn(async () => {
      throw new Error('daemon offline')
    })
  })

  test('startSession failure points to the Kaboom daemon and command', async () => {
    const warn = mock.method(console, 'warn', () => {})
    const { startSession } = await import('../../extension/content/ui/terminal-widget-session.js')

    const result = await startSession({})

    assert.strictEqual(result, null)
    assert.strictEqual(warn.mock.calls.length, 1)
    const message = warn.mock.calls[0].arguments[0]
    assert.match(message, /KaBOOM! daemon running/)
    assert.match(message, /npx kaboom-agentic-browser/)
    assert.doesNotMatch(message, /Gasoline daemon|STRUM daemon|gasoline-agentic-browser|strum-agentic-browser/)
  })
})
