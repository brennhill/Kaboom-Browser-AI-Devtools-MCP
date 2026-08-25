#!/usr/bin/env node
// check-ts-strictness.mjs — Ratchets @ts-nocheck and explicit `any` out of authored src/ TS.
//
// AGENTS.md rule 2 claims "No `any` — TypeScript strict mode". This gate makes
// that claim enforced: authored TypeScript under src/ may contain zero
// @ts-nocheck directives and a never-growing number of explicit `any`
// annotations (frozen in .ts-strictness-baseline.json, only-down like the
// folder-size gate). Generated output (wire types, generated dom-primitives,
// src/generated/) is exempt — its canonical input is Go or the generator.
//
// Usage:
//   node scripts/quality/contracts/check-ts-strictness.mjs            # check (CI)
//   node scripts/quality/contracts/check-ts-strictness.mjs --update   # re-freeze after reducing
// Exit 0 when within budget, 1 otherwise.

import { readdirSync, readFileSync, statSync, existsSync, writeFileSync } from 'node:fs'
import { join, relative, resolve } from 'node:path'
import { createRequire } from 'node:module'
import { fileURLToPath } from 'node:url'

const require = createRequire(import.meta.url)
const ts = require('typescript')

const REPO_ROOT = resolve(fileURLToPath(new URL('.', import.meta.url)), '..', '..', '..', '..')
const SCAN_ROOT = join(REPO_ROOT, 'src')
const BASELINE_PATH = join(REPO_ROOT, '.ts-strictness-baseline.json')

const EXCLUDED_DIR_NAMES = new Set(['node_modules', 'dist', 'build', 'vendor', 'coverage', 'generated'])
// Generated from internal/types/wire_*.go and scripts/build/generate-dom-primitives.js.
const EXCLUDED_PATH_PREFIXES = [join('types', 'wire')]
const GENERATED_BASENAMES = new Set([
  'dom-primitives-form.ts',
  'dom-primitives-pointer.ts',
  'dom-primitives-read.ts',
  'dom-primitives-intent.ts',
  'dom-primitives-overlay.ts'
])

function isExcluded(relPath, name) {
  if (!name.endsWith('.ts') || name.endsWith('.d.ts') || name.endsWith('.test.ts')) return true
  if (GENERATED_BASENAMES.has(name)) return true
  return EXCLUDED_PATH_PREFIXES.some((prefix) => relPath === prefix || relPath.startsWith(prefix + '/'))
}

function collectTsFiles(root, dir, files) {
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    const full = join(dir, entry.name)
    if (entry.isDirectory()) {
      if (!EXCLUDED_DIR_NAMES.has(entry.name) && !entry.name.startsWith('.')) collectTsFiles(root, full, files)
      continue
    }
    if (!entry.isFile()) continue
    const relPath = relative(root, full)
    if (!isExcluded(relPath, entry.name)) files.push({ full, relPath })
  }
}

/** Count strictness escapes in authored TS under `root`. */
export function countStrictness(root = SCAN_ROOT) {
  const files = []
  if (existsSync(root) && statSync(root).isDirectory()) collectTsFiles(root, root, files)
  const counts = { explicit_any: 0, ts_nocheck: 0 }
  for (const { full, relPath } of files) {
    const source = readFileSync(full, 'utf8')
    if (source.includes('@ts-nocheck')) counts.ts_nocheck++
    const sf = ts.createSourceFile(relPath, source, ts.ScriptTarget.Latest, true, ts.ScriptKind.TS)
    const visit = (node) => {
      // AnyKeyword keyword nodes only occur in type positions; identifier or
      // property accesses named `any` parse as Identifier and never match.
      if (node.kind === ts.SyntaxKind.AnyKeyword) {
        counts.explicit_any++
      }
      ts.forEachChild(node, visit)
    }
    visit(sf)
  }
  return counts
}

function loadBaseline() {
  try {
    const parsed = JSON.parse(readFileSync(BASELINE_PATH, 'utf8'))
    if (parsed.version !== 1) throw new Error('unsupported ts-strictness baseline version')
    return parsed
  } catch (error) {
    if (error.code === 'ENOENT') return { version: 1, ts_nocheck: 0, explicit_any: 0 }
    throw error
  }
}

function writeBaseline(counts) {
  writeFileSync(BASELINE_PATH, JSON.stringify({ version: 1, ...counts }, null, 2) + '\n')
}

function main() {
  const counts = countStrictness()
  if (process.argv.includes('--update')) {
    writeBaseline(counts)
    console.log(
      `TS-strictness baseline written: ${counts.ts_nocheck} @ts-nocheck file(s), ${counts.explicit_any} explicit any annotation(s).`
    )
    return 0
  }

  const baseline = loadBaseline()
  const failures = []
  if (counts.ts_nocheck > baseline.ts_nocheck) {
    failures.push(`@ts-nocheck: ${counts.ts_nocheck} file(s), baseline allows ${baseline.ts_nocheck} (target: 0)`)
  }
  if (counts.explicit_any > baseline.explicit_any) {
    failures.push(`explicit any: ${counts.explicit_any} annotation(s), baseline allows ${baseline.explicit_any}`)
  }

  if (failures.length === 0) {
    if (counts.explicit_any < baseline.explicit_any || counts.ts_nocheck < baseline.ts_nocheck) {
      console.log('TS-strictness improved — run `make ts-strictness-baseline-update` to lock it in.')
    }
    console.log(
      `✅ Authored TS strictness within budget (${counts.ts_nocheck} @ts-nocheck, ${counts.explicit_any} explicit any; baselines ${baseline.ts_nocheck}/${baseline.explicit_any})`
    )
    return 0
  }

  console.error('❌ TS strictness regressed:\n')
  for (const failure of failures) console.error(`  - ${failure}`)
  console.error('\nType the value; @ts-nocheck and new `any` annotations are not allowed.')
  return 1
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  process.exit(main())
}
