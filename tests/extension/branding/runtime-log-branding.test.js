// @ts-nocheck
/**
 * @fileoverview runtime-log-branding.test.js — Guards selected runtime modules against legacy Gasoline/STRUM log prefixes.
 */

import { describe, test } from 'node:test'
import assert from 'node:assert'
import { readFile } from 'node:fs/promises'

const RUNTIME_LOG_SOURCES = [
  'src/background.ts',
  'src/background/commands/analyze.ts',
  'src/background/commands/helpers.ts',
  'src/background/event-listeners.ts',
  'src/background/init.ts',
  'src/background/index.ts',
  'src/background/message-handlers.ts',
  'src/background/sync/server.ts',
  'src/background/ui/tab-state.ts',
  'src/inject/api.ts',
  'src/inject/execute-js.ts',
  'src/inject/message-handlers.ts',
  'src/inject/observers.ts',
  'src/inject/settings.ts',
  'src/inject/state.ts',
  'src/popup/tabs/tab-tracking-api.ts',
  'src/popup.ts',
  'src/popup/ai-web-pilot.ts',
  'src/popup/feature-toggles.ts',
  'src/lib/page/context.ts',
  'src/lib/page/exceptions.ts',
  'src/lib/net/network.ts',
  'src/lib/analysis/performance.ts',
  'src/lib/net/websocket.ts',
  'src/content/runtime-message-listener.ts'
]

describe('runtime log branding', () => {
  test('selected runtime modules do not hardcode legacy log prefixes', async () => {
    for (const relativePath of RUNTIME_LOG_SOURCES) {
      const contents = await readFile(new URL(`../../../${relativePath}`, import.meta.url), 'utf8')
      assert.doesNotMatch(
        contents,
        /\[(Gasoline|STRUM)(?::|\])/,
        `${relativePath} still hardcodes a legacy runtime log prefix`
      )
    }
  })

  test('websocket capture internals use Kaboom names', async () => {
    const contents = await readFile(new URL('../../../src/lib/net/websocket.ts', import.meta.url), 'utf8')

    assert.doesNotMatch(contents, /\bGasolineWsMessage\b|\bStrumWsMessage\b/, 'src/lib/net/websocket.ts still uses a legacy message type')
    assert.doesNotMatch(contents, /\bGasolineWebSocket\b|\bStrumWebSocket\b/, 'src/lib/net/websocket.ts still uses a legacy constructor name')
    assert.match(contents, /\bKaboomWsMessage\b/, 'src/lib/net/websocket.ts should use KaboomWsMessage')
    assert.match(contents, /\bKaboomWebSocket\b/, 'src/lib/net/websocket.ts should use KaboomWebSocket')
  })
})
