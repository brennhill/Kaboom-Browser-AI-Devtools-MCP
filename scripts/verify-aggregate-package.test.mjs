// scripts/verify-aggregate-package.test.mjs — Regression tests for the aggregate publish guard.
// Ensures the aggregate cannot publish without its launchers, CLI, and a real (JS-bundled)
// extension — the same silent "files" drop that caused the 0.8.2 empty-binary incident.
// Run: node --test scripts/verify-aggregate-package.test.mjs

import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import { verifyAggregatePackage } from './verify-aggregate-package.js'

// Build a throwaway aggregate package. Options omit pieces to simulate a broken publish.
function makeAggregate({ launchers = true, cli = true, manifest = true, bundles = true } = {}) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'kaboom-agg-'))
  fs.mkdirSync(path.join(dir, 'bin'), { recursive: true })
  fs.mkdirSync(path.join(dir, 'lib'), { recursive: true })
  fs.mkdirSync(path.join(dir, 'extension'), { recursive: true })
  if (launchers) {
    fs.writeFileSync(path.join(dir, 'bin/kaboom-agentic-browser'), '#!/usr/bin/env node\n')
    fs.writeFileSync(path.join(dir, 'bin/kaboom-hooks'), '#!/usr/bin/env node\n')
  }
  if (cli) fs.writeFileSync(path.join(dir, 'lib/cli.js'), '// cli\n')
  if (manifest) {
    fs.writeFileSync(
      path.join(dir, 'extension/manifest.json'),
      JSON.stringify({
        manifest_version: 3,
        version: '9.9.9',
        background: { service_worker: 'background.js' },
        content_scripts: [{ js: ['content.bundled.js'] }],
      })
    )
    if (bundles) {
      fs.writeFileSync(path.join(dir, 'extension/background.js'), '// sw\n')
      fs.writeFileSync(path.join(dir, 'extension/content.bundled.js'), '// content\n')
    }
  }
  return dir
}

test('PASSES for a fully staged aggregate', () => {
  const res = verifyAggregatePackage(makeAggregate())
  assert.equal(res.ok, true, res.problems.join('; '))
})

test('FAILS when the extension manifest is missing (extension/ never staged)', () => {
  const res = verifyAggregatePackage(makeAggregate({ manifest: false }))
  assert.equal(res.ok, false)
  assert.match(res.problems.join('\n'), /extension\/manifest\.json: MISSING/)
})

test('FAILS when the manifest is present but its JS bundles were not staged', () => {
  const res = verifyAggregatePackage(makeAggregate({ bundles: false }))
  assert.equal(res.ok, false)
  assert.match(res.problems.join('\n'), /background\.js: referenced by manifest but NOT staged|content\.bundled\.js: referenced/)
})

test('FAILS when a launcher is missing', () => {
  const res = verifyAggregatePackage(makeAggregate({ launchers: false }))
  assert.equal(res.ok, false)
  assert.match(res.problems.join('\n'), /bin\/kaboom-agentic-browser: MISSING/)
})

test('FAILS when lib/cli.js is missing', () => {
  const res = verifyAggregatePackage(makeAggregate({ cli: false }))
  assert.equal(res.ok, false)
  assert.match(res.problems.join('\n'), /lib\/cli\.js: MISSING/)
})
