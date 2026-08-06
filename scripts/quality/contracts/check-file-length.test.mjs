// check-file-length.test.mjs — Regression tests for the authored-source LOC gate.

import assert from 'node:assert/strict'
import { mkdtemp, mkdir, rm, writeFile } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import path from 'node:path'
import { spawnSync } from 'node:child_process'
import { afterEach, test } from 'node:test'

const SCRIPT = path.resolve('scripts/quality/contracts/check-file-length.sh')
const temporaryRoots = []

async function createRoot(files) {
  const root = await mkdtemp(path.join(tmpdir(), 'kaboom-file-length-'))
  temporaryRoots.push(root)
  for (const [relativePath, lines] of Object.entries(files)) {
    const target = path.join(root, relativePath)
    await mkdir(path.dirname(target), { recursive: true })
    await writeFile(target, `${'line\n'.repeat(lines)}`)
  }
  return root
}

function runGate(root) {
  return spawnSync('bash', [SCRIPT], {
    cwd: root,
    env: { ...process.env, CHECK_FILE_LENGTH_ROOT: root },
    encoding: 'utf8'
  })
}

afterEach(async () => {
  await Promise.all(temporaryRoots.splice(0).map((root) => rm(root, { recursive: true })))
})

test('rejects oversized authored JavaScript and test files', async () => {
  const root = await createRoot({
    'src/large.js': 801,
    'tests/large.test.js': 801,
    'src/within-limit.ts': 800
  })

  const result = runGate(root)

  assert.equal(result.status, 1)
  assert.match(result.stdout, /src\/large\.js: 801 lines/)
  assert.match(result.stdout, /tests\/large\.test\.js: 801 lines/)
  assert.doesNotMatch(result.stdout, /within-limit/)
})

test('ignores generated, compiled, bundled, vendored, and dependency output', async () => {
  const root = await createRoot({
    'generated/schema.js': 801,
    'src/generated/schema.ts': 801,
    'extension/content.js': 801,
    'dist/output.js': 801,
    'vendor/library.go': 801,
    'node_modules/library/index.js': 801,
    'src/small.js': 12
  })

  const result = runGate(root)

  assert.equal(result.status, 0, result.stdout + result.stderr)
  assert.match(result.stdout, /All authored source files are within the line limit/)
})

test('does not permit source-level waivers for oversized authored files', async () => {
  const root = await createRoot({
    'src/waived.go': 801
  })
  await writeFile(path.join(root, 'src/waived.go'), `// nolint:filelength - legacy waiver\n${'line\n'.repeat(800)}`)

  const result = runGate(root)

  assert.equal(result.status, 1)
  assert.match(result.stdout, /src\/waived\.go: 801 lines/)
})
