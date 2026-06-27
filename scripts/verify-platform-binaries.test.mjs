// scripts/verify-platform-binaries.test.mjs — Regression tests for the platform-binary publish guard.
// Guards against the @brennhill/...@0.8.2 incident, where a platform package shipped with only a
// 312-byte package.json (no Go binary), breaking `npx kaboom-agentic-browser` / the MCP server.
// Run: node --test scripts/verify-platform-binaries.test.mjs

import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import { execFileSync } from 'node:child_process'
import { fileURLToPath } from 'node:url'

import { verifyPlatformPackage, MIN_BINARY_BYTES } from './verify-platform-binaries.js'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const SCRIPT = path.join(__dirname, 'verify-platform-binaries.js')
const ROOT = path.join(__dirname, '..')
const PLATFORM_DIRS = ['darwin-arm64', 'darwin-x64', 'linux-arm64', 'linux-x64', 'win32-x64']

// Build a throwaway platform package directory. `binBytes === null` omits the binary entirely
// (the exact 0.8.2 failure: a "files" entry that does not exist on disk).
function makePkg({ files, binBytes }) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'kaboom-verify-'))
  fs.writeFileSync(
    path.join(dir, 'package.json'),
    JSON.stringify({ name: '@brennhill/kaboom-agentic-browser-test', version: '9.9.9', files }, null, 2)
  )
  if (binBytes !== null) {
    fs.mkdirSync(path.join(dir, 'bin'), { recursive: true })
    for (const f of files.filter((x) => x.startsWith('bin/'))) {
      fs.writeFileSync(path.join(dir, f), Buffer.alloc(binBytes, 1))
    }
  }
  return dir
}

const STD_FILES = ['bin/kaboom-agentic-browser', 'bin/kaboom-hooks']

test('PASSES when all binaries are present and full-size', () => {
  const dir = makePkg({ files: STD_FILES, binBytes: MIN_BINARY_BYTES + 1 })
  const res = verifyPlatformPackage(dir)
  assert.equal(res.ok, true, res.problems.join('; '))
  assert.equal(res.checked.length, 2)
  assert.equal(res.problems.length, 0)
})

test('FAILS when a binary is missing (reproduces the 0.8.2 empty-package bug)', () => {
  const dir = makePkg({ files: STD_FILES, binBytes: null })
  const res = verifyPlatformPackage(dir)
  assert.equal(res.ok, false)
  assert.match(res.problems.join('\n'), /MISSING/)
})

test('FAILS when a binary is a stub (package.json-only / truncated)', () => {
  // 312 bytes == the actual size of the broken 0.8.2 tarball's only file.
  const dir = makePkg({ files: STD_FILES, binBytes: 312 })
  const res = verifyPlatformPackage(dir)
  assert.equal(res.ok, false)
  assert.match(res.problems.join('\n'), /stub|bytes/)
})

test('FAILS when package "files" declares no bin/ entry at all', () => {
  const dir = makePkg({ files: ['README.md'], binBytes: null })
  const res = verifyPlatformPackage(dir)
  assert.equal(res.ok, false)
  assert.match(res.problems.join('\n'), /no bin\//)
})

test('CLI exits non-zero on a broken package so `npm publish` aborts', () => {
  const dir = makePkg({ files: STD_FILES, binBytes: null })
  assert.throws(
    () => execFileSync('node', [SCRIPT, dir], { stdio: 'pipe' }),
    (err) => err.status === 1
  )
})

test('CLI exits zero on a good package', () => {
  const dir = makePkg({ files: STD_FILES, binBytes: MIN_BINARY_BYTES + 1 })
  // Throws on non-zero exit; reaching the assert means it succeeded.
  execFileSync('node', [SCRIPT, dir], { stdio: 'pipe' })
  assert.ok(true)
})

// Wiring guard: every real platform package must keep prepublishOnly pointed at this script,
// so the protection cannot be silently dropped in a future edit.
test('every platform package wires prepublishOnly to the guard', () => {
  for (const d of PLATFORM_DIRS) {
    const pkgPath = path.join(ROOT, 'npm', d, 'package.json')
    const pkg = JSON.parse(fs.readFileSync(pkgPath, 'utf8'))
    const hook = pkg.scripts?.prepublishOnly ?? ''
    assert.match(
      hook,
      /verify-platform-binaries/,
      `npm/${d}/package.json must run verify-platform-binaries in prepublishOnly (found: ${JSON.stringify(hook)})`
    )
    assert.ok(
      Array.isArray(pkg.files) && pkg.files.some((f) => f.startsWith('bin/')),
      `npm/${d}/package.json must whitelist a bin/ binary`
    )
  }
})
