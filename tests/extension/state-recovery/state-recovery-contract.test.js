// state-recovery-contract.test.js — Guards extension persisted-state recovery reporting.
import assert from 'node:assert/strict'
import { readFileSync, readdirSync, statSync } from 'node:fs'
import { join, relative } from 'node:path'
import { test } from 'node:test'

function source(path) {
  return readFileSync(new URL(`../../../${path}`, import.meta.url), 'utf8')
}

test('runtime message contract owns state recovery reports', () => {
  const messages = source('src/types/runtime-messages.ts')
  assert.match(messages, /interface ReportStateRecoveryMessage/)
  assert.match(messages, /readonly type: 'report_state_recovery'/)
  assert.match(messages, /readonly lifecycle: StateRecoveryLifecycle/)
  assert.match(messages, /\| ReportStateRecoveryMessage/)
})

test('background recovery owner emits structured sync diagnostics', () => {
  const recovery = source('src/background/runtime-state/state-recovery.ts')
  assert.match(recovery, /category: 'state_recovery'/)
  assert.match(recovery, /pushExtensionLog/)
  assert.doesNotMatch(recovery, /chrome\.storage/)
})

test('cross-context reporter never includes raw persisted values', () => {
  const recovery = source('src/lib/storage/recovery.ts')
  assert.match(recovery, /report_state_recovery/)
  assert.match(recovery, /resolveStateRecovery/)
  assert.match(recovery, /sendTransition\('recovered'/)
  assert.doesNotMatch(recovery, /value|payload|raw_state/)
})

test('production storage reads stay inside validated owners', () => {
  const root = new URL('../../../src/', import.meta.url)
  const files = walk(root.pathname).filter((path) => path.endsWith('.ts') || path.endsWith('.js'))
  const allowedBatchReaders = new Set([
    'background/message-routing/pilot-handler.ts',
    'background/ui/settings-storage.ts',
    'background/ui/terminal-workspace.ts',
    'content/runtime-message-listener.ts',
    'content/script-injection.ts',
    'lib/tabs/tracked-tab-storage.ts',
    'options.ts',
    'popup.ts'
  ])
  for (const path of files) {
    const name = relative(root.pathname, path)
    const text = readFileSync(path, 'utf8')
    if (!name.startsWith('lib/storage/')) {
      assert.doesNotMatch(text, /\bgetLocal\(/, `${name} bypasses readLocalState`)
      assert.doesNotMatch(text, /\bgetSession\(/, `${name} bypasses readSessionState`)
    }
    if (/\bgetLocals\(/.test(text)) {
      assert.ok(
        name.startsWith('lib/storage/') || allowedBatchReaders.has(name),
        `${name} adds an unreviewed batch state loader`
      )
    }
    if (/chrome\.storage\.(?:local|session)\.get\s*\(/.test(text)) {
      assert.ok(
        name.startsWith('lib/storage/') || name === 'content/draw-mode/persistence-submission.js',
        `${name} bypasses canonical storage readers`
      )
    }
  }
})

function walk(directory) {
  const result = []
  for (const entry of readdirSync(directory)) {
    const path = join(directory, entry)
    if (statSync(path).isDirectory()) result.push(...walk(path))
    else result.push(path)
  }
  return result
}
