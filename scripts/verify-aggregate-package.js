#!/usr/bin/env node
// scripts/verify-aggregate-package.js — Pre-publish guard for the aggregate npm package.
// Why: npm SILENTLY omits "files" entries that are absent on disk at publish time. The aggregate
//   (kaboom-agentic-browser) ships bin/ launchers, lib/, and extension/ — but extension/ is
//   gitignored and only staged by `make npm-binaries`. A publish that skipped staging would ship
//   an aggregate with NO extension via the SAME silent-drop mechanism as the 0.8.2 empty-binary
//   incident, and the post-publish --version check (launcher + Go binary) would never notice.
//   This guard makes that impossible: the launchers, the CLI, and a real (parseable, non-empty)
//   extension whose manifest-referenced JS bundles exist on disk are all required before publish.
// Used by: the aggregate package's prepublishOnly hook, and `make preflight` (npm publish --dry-run).
// Docs: docs/core/known-issues.md (empty-package incident), docs/core/release.md.

import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

// Files the aggregate MUST ship for `npx kaboom-agentic-browser` and the extension to work.
const REQUIRED_FILES = ['bin/kaboom-agentic-browser', 'bin/kaboom-hooks', 'lib/cli.js', 'extension/manifest.json']

// Verify a single aggregate package directory. Pure (no process.exit) so it is unit-testable.
// Returns { ok, checked: string[], problems: string[] }.
export function verifyAggregatePackage(pkgDir) {
  const resolved = path.resolve(pkgDir)
  const problems = []
  const checked = []

  for (const rel of REQUIRED_FILES) {
    const abs = path.join(resolved, rel)
    if (!fs.existsSync(abs) || !fs.statSync(abs).isFile()) {
      problems.push(`${rel}: MISSING (npm would publish the aggregate without it)`)
    } else {
      checked.push(rel)
    }
  }

  // The extension manifest must parse and reference JS that is actually staged on disk — this is
  // what catches a manifest-only extension/ where the JS bundles were never copied in.
  const manifestPath = path.join(resolved, 'extension', 'manifest.json')
  if (fs.existsSync(manifestPath)) {
    let manifest = null
    try {
      manifest = JSON.parse(fs.readFileSync(manifestPath, 'utf8'))
    } catch (e) {
      problems.push(`extension/manifest.json: unparseable (${e.message})`)
    }
    if (manifest) {
      if (!manifest.version) problems.push('extension/manifest.json: no "version" field')
      const referenced = []
      if (manifest.background && manifest.background.service_worker) referenced.push(manifest.background.service_worker)
      for (const cs of manifest.content_scripts || []) {
        for (const js of cs.js || []) referenced.push(js)
      }
      if (referenced.length === 0) {
        problems.push('extension/manifest.json: declares no service_worker or content scripts (looks empty/stubbed)')
      }
      for (const js of referenced) {
        if (!fs.existsSync(path.join(resolved, 'extension', js))) {
          problems.push(`extension/${js}: referenced by manifest but NOT staged (extension JS bundle missing)`)
        } else {
          checked.push(`extension/${js}`)
        }
      }
    }
  }

  return { ok: problems.length === 0, checked, problems }
}

// CLI: `node verify-aggregate-package.js [pkgDir]`. Defaults to cwd (prepublishOnly runs from the
// package directory). Exits non-zero on any failure so `npm publish` aborts.
function main(argv) {
  const target = argv[0] || process.cwd()
  const { ok, checked, problems } = verifyAggregatePackage(target)
  if (ok) {
    console.log(`✅ aggregate package OK: ${checked.length} required files present (launchers, CLI, extension + JS bundles)`)
    return
  }
  console.error('❌ aggregate package would publish BROKEN:')
  for (const p of problems) console.error(`     - ${p}`)
  console.error('\nRun "make npm-binaries" to build and stage the launchers + extension before publishing.')
  process.exit(1)
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  main(process.argv.slice(2))
}
