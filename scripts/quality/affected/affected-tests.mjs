#!/usr/bin/env node
// affected-tests.mjs — Selects the tests a change actually reaches.
//
// Why this exists: two branches reported green gates and broke four tests on
// merge, all in files their hand-written globs did not cover. The instruction
// not to run the whole JS suite concurrently is what forced the scoping, so the
// fix is not "run more" — it is to answer "which suites import this module?"
//
// The answer is deliberately an OVER-approximation. Every string literal in a
// file that looks like a path is treated as a dependency, because tests here
// import through variables (`await import(CDP)`) as often as through literals,
// and a selector that missed one would reintroduce the exact failure it exists
// to prevent.
//
// Fail-safe: when a change cannot be traced — a Makefile, a generator, a
// golden, a config — the answer is "run everything", with the file named. A
// selector that silently returns nothing is worse than no selector at all.
//
// Usage:
//   node scripts/quality/affected/affected-tests.mjs --base UNSTABLE
//   node scripts/quality/affected/affected-tests.mjs --files src/a.ts src/b.ts
//   node scripts/quality/affected/affected-tests.mjs --base UNSTABLE --format json

import { execFileSync } from 'node:child_process'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

export const REPO_ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', '..', '..')

/** Directories that hold first-party JS the tests reach. */
const SCAN_ROOTS = ['extension', 'tests', 'scripts', 'src']

/** A test file is one node:test runs. */
export function isTestFile(relative) {
  return /\.test\.(js|mjs|cjs)$/.test(relative) || /\.contract\.test\.mjs$/.test(relative)
}

/** Source extensions the graph understands. */
function isGraphFile(relative) {
  return /\.(js|mjs|cjs|ts)$/.test(relative) && !relative.endsWith('.d.ts')
}

/** Walk the scan roots and return every graph file, repo-relative. */
export function listGraphFiles(root = REPO_ROOT) {
  const found = []
  const walk = (absolute) => {
    let entries
    try {
      entries = fs.readdirSync(absolute, { withFileTypes: true })
    } catch {
      // EXPECTED_ABSENCE: a scan root may not exist in a partial checkout. That
      // is normal, and logging it would be misleading noise on every run.
      return
    }
    for (const entry of entries) {
      if (entry.name === 'node_modules' || entry.name.startsWith('.')) continue
      const next = path.join(absolute, entry.name)
      if (entry.isDirectory()) {
        walk(next)
        continue
      }
      const relative = path.relative(root, next)
      if (isGraphFile(relative)) found.push(relative)
    }
  }
  for (const scanRoot of SCAN_ROOTS) walk(path.join(root, scanRoot))
  return found.sort()
}

/** Every path-shaped string literal in a file. */
const PATH_LITERAL = /['"`](\.{1,2}\/[^'"`\n]+|(?:extension|src|tests|scripts)\/[^'"`\n]+)['"`]/g

/**
 * Resolve one literal against the importing file, to a repo-relative path.
 *
 * `.ts` sources compile to `extension/**.js`, so a test importing the compiled
 * file must also be selected when the TypeScript changes — that mapping is the
 * whole reason a src/ edit can break a test that never mentions src/.
 */
export function resolveLiteral(fromRelative, literal, root = REPO_ROOT) {
  const base = literal.startsWith('.')
    ? path.normalize(path.join(path.dirname(fromRelative), literal))
    : path.normalize(literal)
  const candidates = [base]
  if (base.endsWith('.js')) candidates.push(base.replace(/\.js$/, '.ts'))
  if (base.startsWith(`extension${path.sep}`)) {
    candidates.push(`src${base.slice('extension'.length)}`.replace(/\.js$/, '.ts'))
  }
  if (!path.extname(base)) candidates.push(`${base}.js`, `${base}.ts`, path.join(base, 'index.js'))
  return candidates.filter((candidate) => fs.existsSync(path.join(root, candidate)))
}

/** Build {file -> [files it reaches]}. */
export function buildGraph(files, root = REPO_ROOT) {
  const edges = new Map()
  for (const file of files) {
    let text
    try {
      text = fs.readFileSync(path.join(root, file), 'utf8')
    } catch {
      // EXPECTED_ABSENCE: the file listing and the read race a checkout switch.
      // Normal, and a log here would fire on every branch change.
      continue
    }
    const reached = new Set()
    for (const match of text.matchAll(PATH_LITERAL)) {
      for (const resolved of resolveLiteral(file, match[1], root)) {
        if (resolved !== file) reached.add(resolved)
      }
    }
    edges.set(file, [...reached])
  }
  return edges
}

/** Invert the graph: {file -> [files that reach it]}. */
export function invert(edges) {
  const importers = new Map()
  for (const [from, targets] of edges) {
    for (const target of targets) {
      if (!importers.has(target)) importers.set(target, [])
      importers.get(target).push(from)
    }
  }
  return importers
}

/**
 * Every test file that transitively reaches one of the changed files.
 *
 * The walk goes up the importer edges, so a change deep in a shared module
 * selects the tests of everything built on it, not only its direct importers.
 */
export function testsReaching(changed, importers) {
  const seen = new Set()
  const tests = new Set()
  const queue = [...changed]
  while (queue.length > 0) {
    const file = queue.pop()
    if (seen.has(file)) continue
    seen.add(file)
    if (isTestFile(file)) tests.add(file)
    for (const importer of importers.get(file) ?? []) {
      if (!seen.has(importer)) queue.push(importer)
    }
  }
  return [...tests].sort()
}

/**
 * Changed files the graph cannot trace.
 *
 * A Makefile, a generator, a golden, a schema or a config can change behaviour
 * everywhere while importing nothing, so their presence forces the full suite.
 */
export function untraceable(changed) {
  return changed.filter((file) => !isGraphFile(file) && !file.endsWith('.go')).sort()
}

/** Go packages holding changed Go files, plus every package that imports them. */
export function goPackages(changed, root = REPO_ROOT) {
  const dirs = new Set()
  for (const file of changed) {
    if (file.endsWith('.go')) dirs.add('./' + path.dirname(file))
  }
  if (dirs.size === 0) return []
  let listing
  try {
    listing = execFileSync('go', ['list', '-f', '{{.ImportPath}} {{join .Deps " "}}', './...'], {
      cwd: root,
      encoding: 'utf8',
      maxBuffer: 64 * 1024 * 1024
    })
  } catch (err) {
    // Fail loud: without the dependency listing the Go selection would be a
    // guess, and a guess here is what shipped the four broken tests.
    throw new Error(`go list failed, so Go test selection cannot be trusted: ${err.message}`, { cause: err })
  }
  const changedImportPaths = new Set()
  const rows = []
  for (const line of listing.split('\n')) {
    const [importPath, ...deps] = line.trim().split(/\s+/)
    if (!importPath) continue
    rows.push([importPath, deps])
    for (const dir of dirs) {
      if (importPath.endsWith(dir.slice(1))) changedImportPaths.add(importPath)
    }
  }
  const selected = new Set(changedImportPaths)
  for (const [importPath, deps] of rows) {
    if (deps.some((dep) => changedImportPaths.has(dep))) selected.add(importPath)
  }
  return [...selected].sort()
}

/** The whole answer for one set of changed files. */
export function selectTests(changed, root = REPO_ROOT) {
  const blockers = untraceable(changed)
  const graph = buildGraph(listGraphFiles(root), root)
  const importers = invert(graph)
  const alwaysRun = loadAlwaysRun(root)
  const reached = testsReaching(changed, importers)
  const jsTests = [...new Set([...reached, ...changed.filter(isTestFile), ...alwaysRun.map((entry) => entry.file)])].sort()
  return {
    full_suite: blockers.length > 0,
    full_suite_reason: blockers.length > 0
      ? `these changed files import nothing, so nothing can bound their effect: ${blockers.join(', ')}`
      : '',
    js_tests: jsTests,
    go_packages: goPackages(changed, root),
    always_run: alwaysRun.map((entry) => entry.file)
  }
}

/** AlwaysRunPath holds tests that mirror production wiring by hand. */
export const AlwaysRunPath = 'scripts/quality/affected/always-run.json'

/**
 * Tests that must run whatever changed.
 *
 * Import graphs cannot see a test that re-declares production wiring instead of
 * importing it: that is how a new handler dependency killed a route in two
 * files while every scoped gate stayed green. Each entry carries the reason it
 * cannot be traced, so the list stays short and reviewable rather than becoming
 * a dumping ground.
 */
export function loadAlwaysRun(root = REPO_ROOT) {
  const file = path.join(root, AlwaysRunPath)
  let raw
  try {
    raw = fs.readFileSync(file, 'utf8')
  } catch {
    // EXPECTED_ABSENCE: no always-run list is a normal state — it means every
    // test is reachable through imports — and logging it would be misleading.
    return []
  }
  const parsed = JSON.parse(raw)
  return parsed.always_run ?? []
}

function parseArgs(argv) {
  const options = { base: '', files: [], format: 'list' }
  for (let i = 0; i < argv.length; i++) {
    if (argv[i] === '--base') options.base = argv[++i]
    else if (argv[i] === '--format') options.format = argv[++i]
    else if (argv[i] === '--files') {
      while (i + 1 < argv.length && !argv[i + 1].startsWith('--')) options.files.push(argv[++i])
    }
  }
  return options
}

function changedFrom(options) {
  if (options.files.length > 0) return options.files
  if (!options.base) throw new Error('pass --base <ref> or --files <paths>')
  const out = execFileSync('git', ['diff', '--name-only', `${options.base}...HEAD`], {
    cwd: REPO_ROOT,
    encoding: 'utf8'
  })
  return out.split('\n').map((line) => line.trim()).filter(Boolean)
}

function main() {
  const options = parseArgs(process.argv.slice(2))
  const selection = selectTests(changedFrom(options))
  if (options.format === 'json') {
    process.stdout.write(JSON.stringify(selection, null, 2) + '\n')
    return
  }
  if (selection.full_suite) {
    process.stdout.write(`FULL SUITE: ${selection.full_suite_reason}\n`)
    return
  }
  process.stdout.write(selection.js_tests.join('\n') + '\n')
  if (selection.go_packages.length > 0) {
    process.stdout.write('\nGo packages:\n' + selection.go_packages.join('\n') + '\n')
  }
}

if (process.argv[1] && process.argv[1].endsWith('affected-tests.mjs')) main()
