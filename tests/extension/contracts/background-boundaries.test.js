// @ts-nocheck
/**
 * @fileoverview background-boundaries.test.js — Architectural contracts for
 * feature-owned background routing and mutable state, including the single
 * authoritative writer of live screen recording storage keys (CLAUDE.md rule 18,
 * docs/features/feature/tab-recording/index.md "Recording Ownership").
 */

import { test, describe } from 'node:test'
import assert from 'node:assert'
import fs from 'node:fs'
import ts from 'typescript'
import { relative } from 'node:path'
import { listTsFiles, parseSource, SRC_ROOT } from './source-contract-utils.js'

test('background routing is composed from feature-owned handler modules', () => {
  const router = fs.readFileSync('src/background/message-handlers.ts', 'utf8')
  assert.doesNotMatch(router, /interface MessageHandlerDependencies/)
  assert.match(router, /MessageHandlerOwner/)
  assert.doesNotMatch(router, /export\s+(?:type\s+)?\{[^}]*\}\s+from/s)
})

test('background mutable state is owned by change-coupled modules', () => {
  assert.strictEqual(fs.existsSync('src/background/state.ts'), false)
  for (const owner of [
    'runtime-state/connection-state.ts',
    'runtime-state/settings-state.ts',
    'tab-tracking/pilot-state.ts',
    'runtime-state/log-queue.ts',
    'runtime-state/startup-state.ts'
  ]) {
    assert.strictEqual(fs.existsSync(`src/background/${owner}`), true, owner)
  }
})

test('aggregate and size-driven background facades stay deleted', () => {
  for (const obsolete of ['src/background/index.ts', 'src/background/ui/tab-state.ts', 'src/types/index.ts']) {
    assert.strictEqual(fs.existsSync(obsolete), false, obsolete)
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

test('extension diagnostics hydrate before startup logging and install a suspension approximation', () => {
  const init = fs.readFileSync('src/background/init.ts', 'utf8')
  const listeners = fs.readFileSync('src/background/event-listeners.ts', 'utf8')
  const hydrateAt = init.indexOf('await initializeExtensionLogQueue')
  const restartCheckAt = init.indexOf('await wasServiceWorkerRestarted')

  assert.ok(hydrateAt > 0, 'startup must hydrate the persisted diagnostic queue')
  assert.ok(hydrateAt < restartCheckAt, 'diagnostics must hydrate before restart events are recorded')
  assert.match(init, /installDiagnosticSuspendListener/)
  assert.match(listeners, /chrome\.runtime\.onSuspend\.addListener/)
})

// Live screen recording storage ownership: src/background/recording/ owns
// kaboom_recording and kaboom_pending_recording; other contexts may only read them.
const OWNED_KEYS = ['RECORDING', 'PENDING_RECORDING']
const OWNER_DIR = 'background/recording/'
const WRITERS = new Set(['setLocal', 'setLocals', 'removeLocal', 'removeLocals'])

function findForeignWrites(file) {
  const sf = parseSource(file)
  const hits = []
  const visit = (node) => {
    if (ts.isCallExpression(node) && ts.isIdentifier(node.expression) && WRITERS.has(node.expression.text)) {
      const text = node.arguments.map((a) => a.getText(sf)).join(',')
      for (const key of OWNED_KEYS) {
        if (new RegExp(`StorageKey\\.${key}\\b`).test(text)) hits.push(`${node.expression.text}(StorageKey.${key})`)
      }
    }
    ts.forEachChild(node, visit)
  }
  visit(sf)
  return hits
}

describe('live screen recording storage ownership', () => {
  test('only src/background/recording/ writes kaboom_recording / kaboom_pending_recording', () => {
    const violations = []
    for (const file of listTsFiles(['background', 'popup', 'content', 'offscreen', 'lib'])) {
      const rel = relative(SRC_ROOT, file)
      if (rel.startsWith(OWNER_DIR)) continue
      for (const hit of findForeignWrites(file)) violations.push(`${rel}: ${hit}`)
    }
    assert.deepStrictEqual(violations, [], 'Route writes through the background recording owner')
  })
})
