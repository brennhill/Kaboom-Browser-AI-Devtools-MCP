#!/usr/bin/env node
// scripts/release/verify-platform-binaries.js — Pre-publish guard for platform binary packages.
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

// Go binaries here are ~6–12MB. A real binary is never this small; anything under this is a
// stub, an LFS pointer, or the "files entry never staged" failure we are guarding against.
export const MIN_BINARY_BYTES = 1_000_000

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
    if (st.size < MIN_BINARY_BYTES) {
      problems.push(`${rel}: only ${st.size} bytes (< ${MIN_BINARY_BYTES} minimum — looks like a stub, not a built binary)`)
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
