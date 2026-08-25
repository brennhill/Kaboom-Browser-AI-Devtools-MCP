#!/usr/bin/env node
// check-complexity.mjs — Fails when any authored TS/JS function exceeds the cyclomatic complexity budget.
//
// Budget: max 15 per function (declarations, methods, accessors, and anonymous
// functions are each judged separately; nested functions never add to their
// parent, matching ESLint complexity semantics). Counted branch nodes: if,
// ternary, &&, ||, for/of/in, while, do-while, switch case clauses, and catch.
//
// Scope: authored production source only. Generated output (wire types,
// OpenAPI types, generated dom-primitives files, bundles) is excluded because
// its canonical input — the Go source of truth, the generator, or the
// scripts/templates/*.tpl partials — is what must stay simple. Test files are
// excluded per the repo's standing policy (.codacy.yml): tests are inherently
// verbose and their complexity is not worth reducing.
//
// Usage: node scripts/quality/contracts/complexity/check-complexity.mjs
// Exit 0 if every function is within budget, 1 otherwise.

import { readdirSync, statSync, readFileSync, existsSync, writeFileSync } from 'node:fs'
import { join, relative, resolve, sep } from 'node:path'
import { createRequire } from 'node:module'
import { fileURLToPath } from 'node:url'

// typescript is a build-tool devDependency; resolve it from the repo root.
const require = createRequire(import.meta.url)
const ts = require('typescript')

export const MAX_COMPLEXITY = 15
export const MAX_PARAMS = 6
export const MAX_LENGTH = 80

const REPO_ROOT = resolve(fileURLToPath(new URL('.', import.meta.url)), '..', '..', '..', '..')
const LENGTH_BASELINE_PATH = join(REPO_ROOT, '.function-length-baseline-ts.json')

const SCAN_ROOTS = ['src', 'scripts', 'packages']

const EXCLUDED_DIR_NAMES = new Set([
  'node_modules',
  'dist',
  'build',
  'vendor',
  'coverage',
  'generated',
  'out', // Next.js fixture build output
  '.next'
])

// Generated artifacts whose authored sources ARE scanned (Go wire source,
// scripts/build generators, scripts/templates partials).
const EXCLUDED_PATH_PREFIXES = [
  join('src', 'types', 'wire'), // generated from internal/types/wire_*.go
  join('scripts', 'smoke-tests', 'framework', 'framework-fixtures', 'next-app', 'out')
]

// Generated from scripts/templates/*.tpl — the templates are scanned instead.
const GENERATED_BASENAMES = new Set([
  'dom-primitives-form.ts',
  'dom-primitives-pointer.ts',
  'dom-primitives-read.ts',
  'dom-primitives-intent.ts',
  'dom-primitives-overlay.ts'
])

const SOURCE_EXTENSIONS = /\.(ts|tsx|js|mjs|cjs|tpl)$/
const EXCLUDED_FILE_PATTERN = /\.(test|spec)\.[cm]?[jt]s$|\.bundled\.js$|\.min\.js$|\.d\.ts$/

function isExcludedDir(name) {
  return EXCLUDED_DIR_NAMES.has(name) || name.startsWith('.')
}

function isExcludedFile(relPath, name) {
  if (!SOURCE_EXTENSIONS.test(name) || EXCLUDED_FILE_PATTERN.test(name)) return true
  if (GENERATED_BASENAMES.has(name)) return true
  return EXCLUDED_PATH_PREFIXES.some((prefix) => relPath === prefix || relPath.startsWith(prefix + sep))
}

function isFunctionLike(node) {
  return (
    ts.isFunctionDeclaration(node) ||
    ts.isMethodDeclaration(node) ||
    ts.isGetAccessorDeclaration(node) ||
    ts.isSetAccessorDeclaration(node) ||
    ts.isConstructorDeclaration(node) ||
    ts.isArrowFunction(node) ||
    ts.isFunctionExpression(node)
  )
}

function functionName(node, sourceFile) {
  if ((ts.isFunctionDeclaration(node) || ts.isMethodDeclaration(node) || ts.isFunctionExpression(node)) && node.name) {
    return node.name.getText(sourceFile)
  }
  if (ts.isGetAccessorDeclaration(node)) return `get ${node.name.getText(sourceFile)}`
  if (ts.isSetAccessorDeclaration(node)) return `set ${node.name.getText(sourceFile)}`
  if (ts.isConstructorDeclaration(node)) return 'constructor'
  return '(anonymous)'
}

function functionComplexity(fn) {
  let complexity = 1
  const visit = (node) => {
    if (node !== fn && isFunctionLike(node)) return // nested functions are judged separately
    if (
      ts.isIfStatement(node) ||
      ts.isConditionalExpression(node) ||
      ts.isForStatement(node) ||
      ts.isForOfStatement(node) ||
      ts.isForInStatement(node) ||
      ts.isWhileStatement(node) ||
      ts.isDoStatement(node) ||
      ts.isCaseClause(node) ||
      ts.isCatchClause(node)
    ) {
      complexity++
    } else if (
      ts.isBinaryExpression(node) &&
      (node.operatorToken.kind === ts.SyntaxKind.AmpersandAmpersandToken ||
        node.operatorToken.kind === ts.SyntaxKind.BarBarToken)
    ) {
      complexity++
    }
    ts.forEachChild(node, visit)
  }
  ts.forEachChild(fn, visit)
  return complexity
}

/** Human key for a function: nested-name for named functions, line-anchored for anonymous. */
export function functionKey(sourceFile, node, name) {
  const line = sourceFile.getLineAndCharacterOfPosition(node.getStart(sourceFile)).line + 1
  return name === '(anonymous)' ? `${name}:${line}` : name
}

/** Measure one function node: complexity, params (excluding `this`), and whole-node line count. */
export function measureFunction(node, sourceFile) {
  const start = node.getStart(sourceFile)
  const end = node.getEnd()
  return {
    line: sourceFile.getLineAndCharacterOfPosition(start).line + 1,
    lines:
      sourceFile.getLineAndCharacterOfPosition(end).line - sourceFile.getLineAndCharacterOfPosition(start).line + 1,
    params: node.parameters ? node.parameters.filter((p) => p.name.getText(sourceFile) !== 'this').length : 0,
    complexity: functionComplexity(node)
  }
}

/**
 * Parse one source file and return findings over `limit`, worst-first.
 * `file` is the path as reported; `source` is its text; `.tpl` templates are
 * parsed as TypeScript (they are the authored input to the dom-primitives
 * generator).
 */
export function complexityOfSource(file, source, limit) {
  const kind = /\.(ts|tsx|tpl)$/.test(file) ? ts.ScriptKind.TS : ts.ScriptKind.JS
  const sourceFile = ts.createSourceFile(file, source, ts.ScriptTarget.Latest, true, kind)
  const findings = []
  const walk = (node) => {
    if (isFunctionLike(node)) {
      const complexity = functionComplexity(node)
      if (complexity > limit) {
        findings.push({
          file,
          line: sourceFile.getLineAndCharacterOfPosition(node.getStart(sourceFile)).line + 1,
          function: functionName(node, sourceFile),
          complexity
        })
      }
    }
    ts.forEachChild(node, walk)
  }
  walk(sourceFile)
  findings.sort((a, b) => b.complexity - a.complexity || a.line - b.line)
  return findings
}

/** Measure every authored function in one source file against all three budgets. */
export function measureSource(file, source) {
  const kind = /\.(ts|tsx|tpl)$/.test(file) ? ts.ScriptKind.TS : ts.ScriptKind.JS
  const sourceFile = ts.createSourceFile(file, source, ts.ScriptTarget.Latest, true, kind)
  const measured = []
  const walk = (node) => {
    if (isFunctionLike(node)) {
      const name = functionName(node, sourceFile)
      measured.push({
        file,
        key: functionKey(sourceFile, node, name),
        function: name,
        ...measureFunction(node, sourceFile)
      })
    }
    ts.forEachChild(node, walk)
  }
  walk(sourceFile)
  return measured
}

/** Judge measurements against budgets; `lengthAllowance(key)` resolves the ratchet. */
export function evaluateBudgets(measured, complexityLimit = MAX_COMPLEXITY, lengthAllowance = () => MAX_LENGTH) {
  const violations = []
  for (const fn of measured) {
    if (fn.complexity > complexityLimit) violations.push({ ...fn, kind: 'complexity' })
    if (fn.params > MAX_PARAMS) violations.push({ ...fn, kind: 'params' })
    if (fn.lines > lengthAllowance(`${fn.file}:${fn.key}`)) violations.push({ ...fn, kind: 'length' })
  }
  return violations
}

function loadLengthBaseline(path) {
  try {
    const parsed = JSON.parse(readFileSync(path, 'utf8'))
    if (parsed.version !== 1) throw new Error(`unsupported function-length baseline version in ${path}`)
    return parsed.functions || {}
  } catch (error) {
    if (error.code === 'ENOENT') return {}
    throw error
  }
}

function writeLengthBaseline(path, functions) {
  writeFileSync(path, JSON.stringify({ version: 1, max_lines: MAX_LENGTH, functions }, null, 2) + '\n')
}

function collectFiles(root, dir, files) {
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    const full = join(dir, entry.name)
    if (entry.isDirectory()) {
      if (!isExcludedDir(entry.name)) collectFiles(root, full, files)
      continue
    }
    if (!entry.isFile()) continue
    const relPath = relative(root, full)
    if (!isExcludedFile(relPath, entry.name)) files.push({ full, relPath })
  }
}

function scanRoots(root) {
  const files = []
  for (const scanRoot of SCAN_ROOTS) {
    const absolute = join(root, scanRoot)
    if (existsSync(absolute) && statSync(absolute).isDirectory()) collectFiles(root, absolute, files)
  }
  return files
}

/** Scan the repo's authored TS/JS roots under `root` and return complexity violations over `limit`. */
export function check(root, limit = MAX_COMPLEXITY) {
  const violations = []
  for (const { full, relPath } of scanRoots(root)) {
    violations.push(...complexityOfSource(relPath, readFileSync(full, 'utf8'), limit))
  }
  violations.sort((a, b) => b.complexity - a.complexity || a.file.localeCompare(b.file) || a.line - b.line)
  return violations
}

/** Scan all budgets; used by the CLI entry point. */
export function checkAllBudgets(root, baseline = {}) {
  const measured = []
  for (const { full, relPath } of scanRoots(root)) {
    measured.push(...measureSource(relPath, readFileSync(full, 'utf8')))
  }
  const allowance = (key) => (baseline[key] !== undefined ? baseline[key] : MAX_LENGTH)
  return evaluateBudgets(measured, MAX_COMPLEXITY, allowance)
}

function reportKind(violations, kind, describe) {
  const rows = violations.filter((v) => v.kind === kind)
  if (rows.length === 0) return false
  console.error(describe(rows.length))
  for (const v of rows) {
    console.error(
      `  ${String(kind === 'complexity' ? v.complexity : kind === 'params' ? v.params : v.lines).padStart(3)}  ${v.file}:${v.line}  ${v.function}`
    )
  }
  console.error('')
  return true
}

function main() {
  if (process.argv.includes('--update-length-baseline')) {
    const measured = []
    for (const { full, relPath } of scanRoots(REPO_ROOT)) {
      measured.push(...measureSource(relPath, readFileSync(full, 'utf8')))
    }
    const functions = {}
    for (const fn of measured) {
      if (fn.lines > MAX_LENGTH) functions[`${fn.file}:${fn.key}`] = fn.lines
    }
    writeLengthBaseline(LENGTH_BASELINE_PATH, functions)
    console.log(`Length baseline written: ${Object.keys(functions).length} function(s) still over ${MAX_LENGTH} lines.`)
    return 0
  }

  const baseline = loadLengthBaseline(LENGTH_BASELINE_PATH)
  const violations = checkAllBudgets(REPO_ROOT, baseline)
  violations.sort((a, b) => a.file.localeCompare(b.file) || a.line - b.line)

  const complexityFailed = reportKind(
    violations,
    'complexity',
    (n) => `❌ ${n} TS/JS function(s) exceed the cyclomatic complexity budget (max ${MAX_COMPLEXITY}):\n`
  )
  const paramsFailed = reportKind(
    violations,
    'params',
    (n) => `❌ ${n} TS/JS function(s) exceed ${MAX_PARAMS} parameters (introduce a parameter object):\n`
  )
  const lengthFailed = reportKind(
    violations,
    'length',
    (n) => `❌ ${n} TS/JS function(s) exceed their length budget (max ${MAX_LENGTH}, ratcheting):\n`
  )

  if (!complexityFailed && !paramsFailed && !lengthFailed) {
    console.log(
      `✅ All authored TS/JS functions are within budgets (complexity ≤${MAX_COMPLEXITY}, params ≤${MAX_PARAMS}, length ≤${MAX_LENGTH} or ratcheted)`
    )
    return 0
  }
  console.error('Split budget-heavy functions into focused helpers; waivers are not allowed.')
  return 1
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  process.exit(main())
}
