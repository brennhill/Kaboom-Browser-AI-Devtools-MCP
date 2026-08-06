#!/usr/bin/env node
// check-folder-size.cjs — ratcheting gate on source files per folder.
//
// Target: no more than MAX_FILES authored files in any first-party folder. Existing
// violations are numerous, so a hard limit would red-line CI until
// a repo-wide restructure lands. Instead this ratchets: each folder's CURRENT count
// is frozen in a baseline, a folder may never grow past its baseline, and any folder
// not in the baseline gets the real limit. Every PR can therefore only improve the
// situation, and folders reaching the limit drop out of the baseline permanently.
//
// Usage:
//   node scripts/quality/contracts/check-folder-size.cjs            # check (CI)
//   node scripts/quality/contracts/check-folder-size.cjs --update   # re-freeze the baseline after improving
//
// Exit 0 if all folders are within budget, 1 otherwise.

const fs = require('fs')
const path = require('path')

const MAX_FILES = 10
const REPO_ROOT = process.env.CHECK_FOLDER_SIZE_ROOT
  ? path.resolve(process.env.CHECK_FOLDER_SIZE_ROOT)
  : path.resolve(__dirname, '..', '..', '..')
const BASELINE_PATH = path.join(REPO_ROOT, '.folder-size-baseline.json')

// Every authored file in a first-party product/documentation root counts. A
// module does not become easier to navigate because its eleventh owner is a
// test, fixture, shell script, schema, stylesheet, or Markdown document.
const FIRST_PARTY_ROOTS = [
  '.github',
  'claude_skill',
  'cmd',
  'docs',
  'gokaboom.dev',
  'internal',
  'npm',
  'packages',
  'plugin',
  'scripts',
  'server',
  'specs',
  'src',
  'tests'
]
const EXCLUDED_DIR = new Set([
  'node_modules',
  '.git',
  '.claude',
  'dist',
  'build',
  'vendor',
  'generated',
  'coverage',
  '.beads'
])
const GENERATED_OUTPUT_ROOTS = new Set([
  'cmd/browser-agent/internal/testpages/pages/frameworks',
  'cmd/browser-agent/internal/testpages/pages/_next',
  'scripts/smoke-tests/framework/framework-fixtures/next-app/out'
])

/** Walk first-party roots and return {relDir: authoredFileCount}. */
function countByFolder() {
  const counts = {}
  const walk = (abs) => {
    const entries = fs.readdirSync(abs, { withFileTypes: true })
    for (const entry of entries) {
      const full = path.join(abs, entry.name)
      if (entry.isDirectory()) {
        const relative = path.relative(REPO_ROOT, full)
        if (EXCLUDED_DIR.has(entry.name) || entry.name.startsWith('.') || GENERATED_OUTPUT_ROOTS.has(relative)) continue
        walk(full)
        continue
      }
      if (!entry.isFile()) continue
      const dir = path.relative(REPO_ROOT, abs) || '.'
      counts[dir] = (counts[dir] || 0) + 1
    }
  }
  for (const root of FIRST_PARTY_ROOTS) {
    const absolute = path.join(REPO_ROOT, root)
    if (fs.existsSync(absolute)) walk(absolute)
  }
  return counts
}

function loadBaseline() {
  try {
    return JSON.parse(fs.readFileSync(BASELINE_PATH, 'utf8')).folders || {}
  } catch {
    return {}
  }
}

function writeBaseline(counts) {
  // Only folders still over the limit need a baseline entry; anything at or under
  // MAX_FILES is held to the real rule from now on and can never regress.
  const folders = {}
  for (const [dir, n] of Object.entries(counts).sort(([a], [b]) => a.localeCompare(b))) {
    if (n > MAX_FILES) folders[dir] = n
  }
  fs.writeFileSync(
    BASELINE_PATH,
    JSON.stringify(
      {
        _comment:
          'Ratcheting baseline for scripts/quality/contracts/check-folder-size.cjs. A folder may never exceed its ' +
          'recorded count; folders absent here are held to the ' +
          MAX_FILES +
          '-file limit. ' +
          'Counts may only go DOWN — run `make folder-baseline-update` after reducing one.',
        max_files: MAX_FILES,
        folders
      },
      null,
      2
    ) + '\n'
  )
  return folders
}

function main() {
  const update = process.argv.includes('--update')
  const counts = countByFolder()

  if (update) {
    const folders = writeBaseline(counts)
    console.log(`Baseline written: ${Object.keys(folders).length} folder(s) still over ${MAX_FILES}.`)
    return 0
  }

  const baseline = loadBaseline()
  const violations = []
  const improved = []

  for (const [dir, n] of Object.entries(counts)) {
    const allowed = baseline[dir] !== undefined ? baseline[dir] : MAX_FILES
    if (n > allowed) {
      violations.push({ dir, count: n, allowed, isNew: baseline[dir] === undefined })
    } else if (baseline[dir] !== undefined && n < baseline[dir]) {
      improved.push({ dir, count: n, was: baseline[dir] })
    }
  }

  for (const i of improved.sort((a, b) => a.dir.localeCompare(b.dir))) {
    console.log(`✅ ${i.dir}: ${i.count} files (was ${i.was}) — improved`)
  }
  if (improved.length > 0) {
    console.log('\nRun `make folder-baseline-update` to lock in the improvement.\n')
  }

  if (violations.length === 0) {
    const over = Object.keys(baseline).length
    console.log(
      `✅ No folder exceeds its budget (limit ${MAX_FILES}; ${over} folder(s) still on a ratcheting baseline).`
    )
    return 0
  }

  console.log('')
  for (const v of violations.sort((a, b) => b.count - a.count)) {
    console.log(
      v.isNew
        ? `❌ ${v.dir}: ${v.count} files (limit ${MAX_FILES})`
        : `❌ ${v.dir}: ${v.count} files (baseline ${v.allowed} — must not grow)`
    )
  }
  console.log('\n────────────────────────────────────────────────────────────────')
  console.log(`Split these into focused sub-packages. The limit is ${MAX_FILES} source files per folder;`)
  console.log('folders already over it are frozen at their current count and may only shrink.')
  console.log('────────────────────────────────────────────────────────────────')
  return 1
}

process.exit(main())
