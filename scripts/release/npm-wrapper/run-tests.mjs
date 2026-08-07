// run-tests.mjs — Runs every change-coupled npm wrapper test on any OS.

import { spawnSync } from 'node:child_process'
import { readdirSync } from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

export function discoverTestFiles(root) {
  const discovered = []
  const pending = [root]
  while (pending.length > 0) {
    const directory = pending.pop()
    for (const entry of readdirSync(directory, { withFileTypes: true })) {
      const entryPath = path.join(directory, entry.name)
      if (entry.isDirectory()) pending.push(entryPath)
      else if (entry.isFile() && entry.name.endsWith('.test.js')) discovered.push(entryPath)
    }
  }
  return discovered.sort()
}

function main() {
  const here = path.dirname(fileURLToPath(import.meta.url))
  const packageRoot = path.join(here, '..', '..', '..', 'npm', 'kaboom-agentic-browser', 'lib')
  const files = discoverTestFiles(packageRoot)
  if (files.length === 0) {
    process.stderr.write(`No *.test.js files found below ${packageRoot}\n`)
    process.exitCode = 1
    return
  }

  const result = spawnSync(process.execPath, ['--test', ...files], { stdio: 'inherit' })
  process.exitCode = result.status ?? 1
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) main()
