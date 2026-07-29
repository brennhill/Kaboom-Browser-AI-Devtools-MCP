// @ts-nocheck
/**
 * @fileoverview background-boundaries.test.js — Architectural contracts for
 * feature-owned background routing and mutable state.
 */

import { test } from 'node:test'
import assert from 'node:assert'
import fs from 'node:fs'

test('background routing is composed from feature-owned handler modules', () => {
  const router = fs.readFileSync('src/background/message-handlers.ts', 'utf8')
  assert.doesNotMatch(router, /interface MessageHandlerDependencies/)
  assert.match(router, /MessageHandlerOwner/)
  assert.doesNotMatch(router, /export\s+(?:type\s+)?\{[^}]*\}\s+from/s)
})

test('background mutable state is owned by change-coupled modules', () => {
  assert.strictEqual(fs.existsSync('src/background/state.ts'), false)
  for (const owner of ['connection-state.ts', 'settings-state.ts', 'pilot-state.ts', 'log-queue.ts', 'startup-state.ts']) {
    assert.strictEqual(fs.existsSync(`src/background/runtime-state/${owner}`), true, owner)
  }
})

test('extension log queue only exposes snapshots', async () => {
  const queue = await import('../../../extension/background/runtime-state/log-queue.js')
  queue.clearExtensionLogsForTesting()
  queue.pushExtensionLog({ timestamp: 'now', level: 'debug', message: 'one', source: 'test', category: 'test' })
  const snapshot = queue.getExtensionLogQueueSnapshot()
  snapshot.push({ timestamp: 'later', level: 'debug', message: 'two', source: 'test', category: 'test' })
  assert.strictEqual(queue.getExtensionLogQueueSnapshot().length, 1)
})
