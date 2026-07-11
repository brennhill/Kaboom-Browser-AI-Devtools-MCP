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

import { verifyPlatformPackage, MIN_BINARY_BYTES, detectBinaryTarget } from './verify-platform-binaries.js'

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

// --- GOOS/GOARCH magic-byte guard (a wrong-arch/OS `cp` passes the size check) ---

function elfHeader(machine) {
  const b = Buffer.alloc(20, 0)
  b[0] = 0x7f; b[1] = 0x45; b[2] = 0x4c; b[3] = 0x46 // \x7fELF
  b.writeUInt16LE(machine, 18) // e_machine: 0x3e=x86-64, 0xb7=aarch64
  return b
}
function machoHeader(cputype) {
  const b = Buffer.alloc(8, 0)
  b[0] = 0xcf; b[1] = 0xfa; b[2] = 0xed; b[3] = 0xfe // Mach-O 64 LE
  b.writeUInt32LE(cputype, 4) // 0x01000007=x86_64, 0x0100000c=arm64
  return b
}
// Platform package whose binaries start with `header` and declare os/cpu.
function makePlatformPkg({ os: pkgOs, cpu, header }) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'kaboom-verify-'))
  fs.writeFileSync(
    path.join(dir, 'package.json'),
    JSON.stringify({ name: '@brennhill/kaboom-agentic-browser-test', version: '9.9.9', os: [pkgOs], cpu: [cpu], files: STD_FILES }, null, 2)
  )
  fs.mkdirSync(path.join(dir, 'bin'), { recursive: true })
  for (const f of STD_FILES) {
    const buf = Buffer.alloc(MIN_BINARY_BYTES + 1, 0)
    header.copy(buf, 0)
    fs.writeFileSync(path.join(dir, f), buf)
  }
  return dir
}

test('detectBinaryTarget reads ELF/Mach-O os + arch', () => {
  assert.deepEqual(detectBinaryTarget(elfHeader(0x3e)), { os: 'linux', arch: 'x64' })
  assert.deepEqual(detectBinaryTarget(elfHeader(0xb7)), { os: 'linux', arch: 'arm64' })
  assert.deepEqual(detectBinaryTarget(machoHeader(0x0100000c)), { os: 'darwin', arch: 'arm64' })
  assert.deepEqual(detectBinaryTarget(machoHeader(0x01000007)), { os: 'darwin', arch: 'x64' })
})

test('PASSES when the binary matches the package os/cpu', () => {
  const dir = makePlatformPkg({ os: 'linux', cpu: 'x64', header: elfHeader(0x3e) })
  const res = verifyPlatformPackage(dir)
  assert.equal(res.ok, true, res.problems.join('; '))
})

test('FAILS when a darwin binary is staged into a linux package (wrong-OS cp)', () => {
  const dir = makePlatformPkg({ os: 'linux', cpu: 'x64', header: machoHeader(0x0100000c) })
  const res = verifyPlatformPackage(dir)
  assert.equal(res.ok, false)
  assert.match(res.problems.join('\n'), /built for darwin|wrong binary staged/)
})

test('FAILS when the arch is wrong (arm64 binary in an x64 package)', () => {
  const dir = makePlatformPkg({ os: 'linux', cpu: 'x64', header: elfHeader(0xb7) })
  const res = verifyPlatformPackage(dir)
  assert.equal(res.ok, false)
  assert.match(res.problems.join('\n'), /arm64.*x64|wrong binary staged/)
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
