// check-folder-size.test.mjs — Regression tests for production-folder classification.

import assert from 'node:assert/strict'
import { mkdtemp, mkdir, rm, writeFile } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import path from 'node:path'
import { spawnSync } from 'node:child_process'
import { afterEach, test } from 'node:test'

const SCRIPT = path.resolve('scripts/check-folder-size.cjs')
const temporaryRoots = []

async function createRoot(relativePaths) {
  const root = await mkdtemp(path.join(tmpdir(), 'kaboom-folder-size-'))
  temporaryRoots.push(root)
  for (const relativePath of relativePaths) {
    const target = path.join(root, relativePath)
    await mkdir(path.dirname(target), { recursive: true })
    await writeFile(target, '// test\n')
  }
  return root
}

function runGate(root) {
  return spawnSync(process.execPath, [SCRIPT], {
    cwd: root,
    env: { ...process.env, CHECK_FOLDER_SIZE_ROOT: root },
    encoding: 'utf8'
  })
}

afterEach(async () => {
  await Promise.all(temporaryRoots.splice(0).map((root) => rm(root, { recursive: true })))
})

test('rejects production folders with more than ten source files', async () => {
  const root = await createRoot(Array.from({ length: 11 }, (_, index) => `src/module-${index + 1}.js`))

  const result = runGate(root)

  assert.equal(result.status, 1)
  assert.match(result.stdout, /src: 11 files \(limit 10\)/)
})

test('does not count test suites or their fixture modules as production files', async () => {
  const root = await createRoot([
    'src/production.js',
    ...Array.from({ length: 11 }, (_, index) => `src/responsibility-${index + 1}-fixture.js`),
    ...Array.from({ length: 11 }, (_, index) => `src/suite-${index + 1}.test.js`)
  ])

  const result = runGate(root)

  assert.equal(result.status, 0, result.stdout + result.stderr)
  assert.match(result.stdout, /No folder exceeds its budget/)
})
