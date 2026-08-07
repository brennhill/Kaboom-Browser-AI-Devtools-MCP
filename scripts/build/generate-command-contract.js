#!/usr/bin/env node
// generate-command-contract.js — Generates the shared extension command-contract identity.
// Why: Same-version daemon and extension builds must not silently disagree about executable commands.
// Docs: docs/features/feature/self-testing/index.md

import { createHash } from 'node:crypto'
import { existsSync, mkdirSync, readFileSync, readdirSync, writeFileSync } from 'node:fs'
import { dirname, join, relative, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const root = resolve(dirname(fileURLToPath(import.meta.url)), '../..')
const sourceRoot = join(root, 'src/background')
const goOutput = join(root, 'internal/commandcontract/generated.go')
const tsOutput = join(root, 'src/types/runtime/command-contract.ts')
const check = process.argv.includes('--check')

function sourceFiles(directory) {
  return readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
    const path = join(directory, entry.name)
    if (entry.isDirectory()) return sourceFiles(path)
    return entry.isFile() && entry.name.endsWith('.ts') ? [path] : []
  })
}

const commands = new Set()
const registration = /registerCommand\(\s*['"]([^'"]+)['"]/g
for (const file of sourceFiles(sourceRoot)) {
  const source = readFileSync(file, 'utf8')
  for (const match of source.matchAll(registration)) commands.add(match[1])
}
const sorted = [...commands].sort()
if (sorted.length === 0) throw new Error('no extension command registrations found')
const id = `sha256:${createHash('sha256').update(sorted.join('\n')).digest('hex')}`

const go = `// generated.go — Generated extension command-contract identity; DO NOT EDIT.\n// Source: literal registerCommand calls under src/background.\n\npackage commandcontract\n\nconst ID = ${JSON.stringify(id)}\n`
const ts = `// command-contract.ts — Generated extension command-contract identity; DO NOT EDIT.\n// Source: literal registerCommand calls under src/background.\n\nexport const EXTENSION_COMMAND_CONTRACT_ID = ${JSON.stringify(id)}\n`

function emit(path, content) {
  if (check) {
    if (!existsSync(path) || readFileSync(path, 'utf8') !== content) {
      throw new Error(`${relative(root, path)} is stale; run make generate-command-contract`)
    }
    return
  }
  mkdirSync(dirname(path), { recursive: true })
  writeFileSync(path, content)
}

emit(goOutput, go)
emit(tsOutput, ts)
