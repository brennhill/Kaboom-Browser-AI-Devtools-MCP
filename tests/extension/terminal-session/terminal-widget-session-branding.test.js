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
    const { startSession } = await import('../../../extension/content/ui/terminal-widget-session.js')

    const result = await startSession({})

    assert.strictEqual(result, null)
    assert.strictEqual(warn.mock.calls.length, 1)
    const message = warn.mock.calls[0].arguments[0]
    assert.match(message, /KaBOOM! daemon running/)
    assert.match(message, /npx kaboom-agentic-browser/)
    assert.doesNotMatch(message, /Gasoline daemon|STRUM daemon|gasoline-agentic-browser|strum-agentic-browser/)
  })

  test('AI init command warns before API-billed credentials reach the CLI', async () => {
    const { buildAIInitCommand } = await import('../../../extension/content/ui/terminal-widget-session.js')

    const command = buildAIInitCommand('claude')

    assert.match(command, /ANTHROPIC_API_KEY/)
    assert.match(command, /ANTHROPIC_AUTH_TOKEN/)
    assert.match(command, /claude auth status/)
    assert.match(command, /KABOOM_EXECUTION_PROVIDER=api/)
    assert.match(command, /Continue with API billing\? \[y\/N\]/)
    assert.match(command, /read -r kaboom_api_confirm/)
    assert.match(command, /API launch cancelled/)
    assert.match(command, /subscription/)
    assert.match(command, /KABOOM_EXECUTION_PROVIDER=subscription/)
    assert.doesNotMatch(command, /\\$ANTHROPIC_API_KEY[^:]/,
      'the command must test credential presence without printing its value')
  })

  test('Codex init checks saved authentication mode as well as environment overrides', async () => {
    const { buildAIInitCommand } = await import('../../../extension/content/ui/terminal-widget-session.js')

    const command = buildAIInitCommand('codex')

    assert.match(command, /OPENAI_API_KEY/)
    assert.match(command, /codex login status/)
    assert.match(command, /API key/)
    assert.match(command, /Continue with API billing\? \[y\/N\]/)
    assert.match(command, /Logged in using ChatGPT/)
    assert.match(command, /KABOOM_EXECUTION_PROVIDER=subscription/)
    const environmentDetection = command.indexOf('kaboom_execution_provider=api')
    const codexAuthentication = command.indexOf('codex login status')
    assert.ok(environmentDetection >= 0 && environmentDetection < codexAuthentication,
      'environment credentials may provide a fallback before Codex authentication is known')
    assert.strictEqual(command.match(/if \[ -n /g)?.length, 1,
      'the generic environment fallback must run once, before authoritative Codex authentication')
  })
})
