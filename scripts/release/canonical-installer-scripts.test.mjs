// Purpose: Prevent compatibility cleanup branches from returning to platform scripts.
// Docs: docs/features/feature/enhanced-cli-config/index.md

import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import test from 'node:test'
import { fileURLToPath } from 'node:url'

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', '..')
const scripts = [
  'scripts/clean-old-daemons.sh',
  'scripts/install-bundled-skills.sh',
  'scripts/install.ps1',
  'scripts/install.sh',
  'scripts/rebuild.sh',
  'scripts/uninstall.ps1',
  'scripts/uninstall.sh',
]

test('platform scripts use only canonical Kaboom identities', () => {
  for (const relativePath of scripts) {
    const source = fs.readFileSync(path.join(repoRoot, relativePath), 'utf8')
    assert.doesNotMatch(source, /\b(?:gasoline|strum)\b/i, `${relativePath} retains an old-brand path`)
    assert.doesNotMatch(source, /\blegacy\b/i, `${relativePath} retains a migration branch`)
    assert.doesNotMatch(source, /\bkaboom-agentic-devtools\b/i, `${relativePath} retains an obsolete binary name`)
  }
})

test('authored installer scripts stay below 800 lines', () => {
  for (const relativePath of scripts) {
    const source = fs.readFileSync(path.join(repoRoot, relativePath), 'utf8')
    assert.ok(source.split('\n').length - 1 < 800, `${relativePath} exceeds the 800-line limit`)
  }
})
