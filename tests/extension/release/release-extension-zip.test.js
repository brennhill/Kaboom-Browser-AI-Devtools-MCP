/**
 * @fileoverview Regression guard for release extension zip packaging.
 * Ensures the extension-zip target archives the full extension tree so MV3 module imports resolve at runtime.
 */

import { test } from 'node:test'
import assert from 'node:assert'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { spawnSync } from 'node:child_process'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const REPO_ROOT = path.resolve(__dirname, '../../..')
const VERSION = fs.readFileSync(path.join(REPO_ROOT, 'VERSION'), 'utf8').trim()
const ZIP_PATH = path.join(REPO_ROOT, 'dist', `kaboom-extension-v${VERSION}.zip`)

function getMakeTargetBlock(makefileText, targetName) {
  const start = makefileText.indexOf(`${targetName}:`)
  assert.notStrictEqual(start, -1, `Makefile target "${targetName}" should exist`)
  const rest = makefileText.slice(start)
  const nextTarget = rest.match(/\n[A-Za-z0-9_.-]+:\n/)
  if (!nextTarget || nextTarget.index === 0) return rest
  return rest.slice(0, nextTarget.index)
}

test('extension-zip target archives full extension tree', () => {
  const makefilePath = path.join(REPO_ROOT, 'Makefile')
  const makefile = fs.readFileSync(makefilePath, 'utf8')
  const block = getMakeTargetBlock(makefile, 'extension-zip')

  assert.ok(
    block.includes('cd extension && zip -r ../$(BUILD_DIR)/kaboom-extension-v$(VERSION).zip \\'),
    'extension-zip target should invoke zip from extension/'
  )
  assert.match(
    block,
    /\n\s+\.\s+\\/m,
    'extension-zip target must package "." to include module directories required by MV3 service worker imports'
  )
})

test('extension-zip artifact includes module graph entrypoints', (t) => {
  // The target shells out to `zip`, which some minimal container images (e.g.
  // node:20) omit. Skip rather than fail, mirroring the `unzip` guard below.
  if (spawnSync('zip', ['-v'], { encoding: 'utf8' }).error?.code === 'ENOENT') {
    t.skip('zip binary not available in this environment')
    return
  }

  const makeResult = spawnSync('make', ['extension-zip'], {
    cwd: REPO_ROOT,
    encoding: 'utf8'
  })
  assert.strictEqual(
    makeResult.status,
    0,
    `make extension-zip failed\nstdout:\n${makeResult.stdout || ''}\nstderr:\n${makeResult.stderr || ''}`
  )
  assert.ok(fs.existsSync(ZIP_PATH), `Expected extension zip at ${ZIP_PATH}`)

  const unzipList = spawnSync('unzip', ['-Z', '-1', ZIP_PATH], { encoding: 'utf8' })
  if (unzipList.error && unzipList.error.code === 'ENOENT') {
    t.skip('unzip binary not available in this environment')
    return
  }
  assert.strictEqual(
    unzipList.status,
    0,
    `Failed to list extension zip\nstdout:\n${unzipList.stdout || ''}\nstderr:\n${unzipList.stderr || ''}`
  )

  const entries = new Set(
    unzipList.stdout
      .split('\n')
      .map((s) => s.trim())
      .filter(Boolean)
  )

  const required = [
    'manifest.json',
    'background.js',
    'content.bundled.js',
    'inject.bundled.js',
    'early-patch.bundled.js'
  ]
  for (const file of required) {
    assert.ok(entries.has(file), `extension zip must include ${file}`)
  }

  for (const entry of entries) {
    assert.ok(!entry.includes('__tests__/'), `extension zip must not include test folder: ${entry}`)
    assert.ok(!entry.endsWith('.test.js'), `extension zip must not include JS test file: ${entry}`)
    assert.ok(!entry.endsWith('.test.cjs'), `extension zip must not include CJS test file: ${entry}`)
  }
})
