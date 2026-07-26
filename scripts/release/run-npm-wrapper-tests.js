// run-npm-wrapper-tests.js — Run every npm wrapper lib/*.test.js on any OS.
// Why: CI runs Node 20 (no `node --test` glob) across bash AND PowerShell (which
// does not expand `*.test.js`). Discovering the files here and passing them as
// explicit args makes the wrapper tests gate identically on every runner.

import { spawnSync } from 'node:child_process'
import { readdirSync } from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const here = path.dirname(fileURLToPath(import.meta.url))
const repoRoot = path.join(here, '..', '..')
const libDir = path.join(repoRoot, 'npm', 'kaboom-agentic-browser', 'lib')

const files = readdirSync(libDir)
  .filter((name) => name.endsWith('.test.js'))
  .sort()
  .map((name) => path.join(libDir, name))

if (files.length === 0) {
  console.error(`No *.test.js files found in ${libDir}`)
  process.exit(1)
}

const result = spawnSync(process.execPath, ['--test', ...files], { stdio: 'inherit' })
process.exit(result.status ?? 1)
