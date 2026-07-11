#!/usr/bin/env node
// scripts/verify-platform-binaries.js — Pre-publish guard for platform binary packages.
// Why: npm SILENTLY omits "files" entries that are absent on disk at publish time.
//      @brennhill/kaboom-agentic-browser-darwin-arm64@0.8.2 shipped with only a 312-byte
//      package.json (no Go binary), so `npx kaboom-agentic-browser` could not exec anything
//      and the MCP server failed to connect. This guard makes such a publish impossible.
// How: for a platform package dir, every bin/* path in package.json "files" MUST exist,
//      be a regular file, and exceed a sane minimum size (Go binaries are multi-MB).
// Used by: each platform package's prepublishOnly hook, `make npm-binaries`, and publish.yml.
// Docs: docs/core/known-issues.md (empty-binary-package incident).

import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

// Go binaries here are ~6–12MB. A real binary is never this small; anything under these is a
// stub, an LFS pointer, or the "files entry never staged" failure we are guarding against.
// Aligned with the GitHub-release installer (install.sh) so BOTH channels reject the same
// truncation range: 5MB for the server binary, 2MB for the smaller hooks binary.
export const MIN_BINARY_BYTES = 5_000_000
export const MIN_HOOKS_BINARY_BYTES = 2_000_000

// Per-binary minimum: the hooks binary is smaller than the server binary.
function minBytesFor(relPath) {
  return /hooks/i.test(path.basename(relPath)) ? MIN_HOOKS_BINARY_BYTES : MIN_BINARY_BYTES
}

// Reads an executable's magic bytes and returns its target { os, arch }. Lets the guard confirm
// each platform package ships a binary actually built for THAT platform — a wrong-arch or
// wrong-OS `cp` in `make npm-binaries` (right filename, wrong contents) passes on size alone and
// would only surface on the two platforms the post-publish matrix can't exec-test.
export function detectBinaryTarget(buf) {
  if (buf.length >= 20 && buf[0] === 0x7f && buf[1] === 0x45 && buf[2] === 0x4c && buf[3] === 0x46) {
    const machine = buf.readUInt16LE(18) // ELF e_machine (little-endian binaries)
    const arch = machine === 0x3e ? 'x64' : machine === 0xb7 ? 'arm64' : `elf-0x${machine.toString(16)}`
    return { os: 'linux', arch }
  }
  if (buf.length >= 8 && buf[0] === 0xcf && buf[1] === 0xfa && buf[2] === 0xed && buf[3] === 0xfe) {
    const cputype = buf.readUInt32LE(4) // Mach-O 64-bit thin, little-endian
    const arch = cputype === 0x01000007 ? 'x64' : cputype === 0x0100000c ? 'arm64' : `macho-0x${cputype.toString(16)}`
    return { os: 'darwin', arch }
  }
  if (buf.length >= 0x40 && buf[0] === 0x4d && buf[1] === 0x5a) { // 'MZ'
    const peOff = buf.readUInt32LE(0x3c)
    if (buf.length >= peOff + 6 && buf[peOff] === 0x50 && buf[peOff + 1] === 0x45) { // 'PE\0\0'
      const machine = buf.readUInt16LE(peOff + 4)
      const arch = machine === 0x8664 ? 'x64' : machine === 0xaa64 ? 'arm64' : `pe-0x${machine.toString(16)}`
      return { os: 'win32', arch }
    }
    return { os: 'win32', arch: 'unknown' }
  }
  return { os: 'unknown', arch: 'unknown' }
}

// Reads up to the first 4KB of a file — enough for the ELF/Mach-O headers and a Go PE header.
function readHead(abs) {
  const fd = fs.openSync(abs, 'r')
  try {
    const head = Buffer.alloc(4096)
    const n = fs.readSync(fd, head, 0, head.length, 0)
    return head.subarray(0, n)
  } finally {
    fs.closeSync(fd)
  }
}

// Verify a single platform package directory. Pure (no process.exit) so it is unit-testable.
// Returns { ok, pkg, checked: string[], problems: string[] }.
export function verifyPlatformPackage(pkgDir) {
  const resolved = path.resolve(pkgDir)
  const pkgJsonPath = path.join(resolved, 'package.json')

  if (!fs.existsSync(pkgJsonPath)) {
    return { ok: false, pkg: null, checked: [], problems: [`no package.json at ${resolved}`] }
  }

  let pkg
  try {
    pkg = JSON.parse(fs.readFileSync(pkgJsonPath, 'utf8'))
  } catch (e) {
    return { ok: false, pkg: null, checked: [], problems: [`unparseable package.json: ${e.message}`] }
  }

  const files = Array.isArray(pkg.files) ? pkg.files : []
  const binFiles = files.filter((f) => typeof f === 'string' && f.replace(/\\/g, '/').startsWith('bin/'))

  const problems = []
  if (binFiles.length === 0) {
    problems.push('package "files" lists no bin/ entries — a platform package must ship a binary')
    return { ok: false, pkg, checked: [], problems }
  }

  // The platform this package targets, from its npm os/cpu fields (real platform packages set
  // exactly one each). When absent (e.g. a synthetic test package), the magic-byte check is
  // skipped and only existence/size are enforced.
  const expectOs = Array.isArray(pkg.os) && pkg.os.length === 1 ? pkg.os[0] : null
  const expectCpu = Array.isArray(pkg.cpu) && pkg.cpu.length === 1 ? pkg.cpu[0] : null

  for (const rel of binFiles) {
    const abs = path.join(resolved, rel)
    if (!fs.existsSync(abs)) {
      problems.push(`${rel}: MISSING (npm would publish the package without it)`)
      continue
    }
    const st = fs.statSync(abs)
    if (!st.isFile()) {
      problems.push(`${rel}: not a regular file`)
      continue
    }
    const minBytes = minBytesFor(rel)
    if (st.size < minBytes) {
      problems.push(`${rel}: only ${st.size} bytes (< ${minBytes} minimum — looks like a stub, not a built binary)`)
      continue
    }
    // Confirm the binary was actually built for THIS package's platform.
    if (expectOs && expectCpu) {
      const target = detectBinaryTarget(readHead(abs))
      if (target.os !== expectOs || target.arch !== expectCpu) {
        problems.push(`${rel}: built for ${target.os}/${target.arch} but package targets ${expectOs}/${expectCpu} (wrong binary staged)`)
      }
    }
  }

  return { ok: problems.length === 0, pkg, checked: binFiles, problems }
}

// CLI: `node verify-platform-binaries.js [pkgDir ...]`. Defaults to cwd (the package being
// published, since npm runs prepublishOnly from the package directory). Exits non-zero on any
// failure so `npm publish` aborts.
function main(argv) {
  const targets = argv.length > 0 ? argv : [process.cwd()]
  let failed = false

  for (const target of targets) {
    const { ok, pkg, checked, problems } = verifyPlatformPackage(target)
    const name = pkg ? `${pkg.name}@${pkg.version}` : target
    if (ok) {
      console.log(`✅ ${name}: ${checked.length} binar${checked.length === 1 ? 'y' : 'ies'} present (${checked.map((f) => path.basename(f)).join(', ')})`)
    } else {
      failed = true
      console.error(`❌ ${name} would publish WITHOUT working binaries:`)
      for (const p of problems) console.error(`     - ${p}`)
    }
  }

  if (failed) {
    console.error('\nRun "make npm-binaries" to build and stage binaries before publishing.')
    process.exit(1)
  }
}

// Only run the CLI when invoked directly, not when imported by tests.
if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  main(process.argv.slice(2))
}
