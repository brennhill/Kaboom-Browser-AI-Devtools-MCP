#!/usr/bin/env node
/**
 * check-gesture-safety.mjs — Enforce the user-gesture contract around
 * `chrome.sidePanel.open()` mechanically instead of by comment.
 *
 * Chrome only runs `sidePanel.open()` while a user gesture is active, and **any
 * await before it expires the gesture** — Chrome then refuses to open the panel
 * and the entry point silently does nothing.
 *
 * This rule exists because the comment version did not hold. "Nothing may be
 * awaited before chrome.sidePanel.open()" was written into the source, and two
 * commits later a toggle awaited a storage read to decide open-vs-close and
 * broke "Open Kaboom Terminal" for everyone. A comment cannot fail a build.
 *
 * Two checks:
 *   1. `chrome.sidePanel.open()` may only be called from the one shared opener
 *      (repo rule 19). Every entry point routes through it, so the gesture rules
 *      live in exactly one place instead of being rediscovered per caller.
 *   2. No `await` may lexically precede that call inside the same function.
 *
 * Escape hatch: a `gesture-safety: allow — <reason>` comment on the awaiting
 * line or the three lines above it. Deliberate, reviewable, and it forces the
 * reason into the source.
 *
 * Uses the TypeScript compiler API (already a devDependency) rather than an
 * ESLint rule: .ts sources are not linted today, and wiring a TS parser into
 * ESLint is a much larger change than this one rule justifies.
 *
 * Usage: node scripts/check-gesture-safety.mjs [--json]
 * Exit 0 when clean, 1 when violations are found.
 */

import ts from 'typescript'
import { readFileSync, readdirSync, statSync } from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const REPO_ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')

/** Call that may only happen inside a live user gesture. */
export const GUARDED_CALL = 'chrome.sidePanel.open'

/** The single shared opener (repo rule 19), repo-relative. */
export const ALLOWED_CALLER = 'src/background/terminal-panel.ts'

/** Opt-out marker; everything after it on the line is the required reason. */
export const ALLOW_MARKER = 'gesture-safety: allow'

/** How many lines above an await are searched for the marker. */
const ALLOW_LOOKBEHIND_LINES = 3

function listTypeScriptFiles(dir) {
  const out = []
  for (const entry of readdirSync(dir)) {
    const full = path.join(dir, entry)
    if (statSync(full).isDirectory()) {
      out.push(...listTypeScriptFiles(full))
    } else if (entry.endsWith('.ts') && !entry.endsWith('.d.ts')) {
      out.push(full)
    }
  }
  return out
}

/** The nearest enclosing function-like node, or null at top level. */
function enclosingFunction(node) {
  let current = node.parent
  while (current) {
    if (
      ts.isFunctionDeclaration(current) ||
      ts.isFunctionExpression(current) ||
      ts.isArrowFunction(current) ||
      ts.isMethodDeclaration(current) ||
      ts.isConstructorDeclaration(current) ||
      ts.isGetAccessorDeclaration(current) ||
      ts.isSetAccessorDeclaration(current)
    ) {
      return current
    }
    current = current.parent
  }
  return null
}

function containsNode(outer, inner) {
  return inner.getStart() >= outer.getStart() && inner.end <= outer.end
}

function collectNodes(root, predicate) {
  const found = []
  const visit = (node) => {
    if (predicate(node)) found.push(node)
    ts.forEachChild(node, visit)
  }
  visit(root)
  return found
}

/**
 * Whether control can never fall out of `stmt` — it always returns or throws.
 *
 * Used to discount guard clauses: an `await` inside `if (fast) { …; return x }`
 * never runs before code that comes after the guard, because taking that branch
 * means never reaching it. Conservative by design — an `if` with no `else` is
 * treated as falling through, so it is never wrongly discounted.
 */
function alwaysExits(stmt) {
  if (!stmt) return false
  if (ts.isReturnStatement(stmt) || ts.isThrowStatement(stmt)) return true
  if (ts.isBlock(stmt)) return alwaysExits(stmt.statements[stmt.statements.length - 1])
  if (ts.isIfStatement(stmt)) {
    return Boolean(stmt.elseStatement) && alwaysExits(stmt.thenStatement) && alwaysExits(stmt.elseStatement)
  }
  return false
}

/** The statement `node` belongs to within `container`'s statement list. */
function statementIn(container, node) {
  let current = node
  while (current && current.parent !== container) current = current.parent
  return current ?? null
}

/**
 * Awaits that actually run before `call`.
 *
 * Walks the statement lists enclosing the call and takes the awaits in the
 * statements before it, plus any earlier in the call's own statement. Awaits in
 * sibling branches the call is not inside are never visited, and awaits behind a
 * guard clause that returns are discounted.
 */
function awaitsReaching(call, scope) {
  const found = []
  let node = call

  while (node && node !== scope) {
    const parent = node.parent
    if (!parent) break
    const statements = parent.statements
    if (Array.isArray(statements) || (statements && typeof statements.length === 'number')) {
      const own = statementIn(parent, call)
      for (const statement of statements) {
        if (statement === own) break
        for (const await_ of collectNodes(statement, ts.isAwaitExpression)) {
          if (!discountedByGuard(await_, statement)) found.push(await_)
        }
      }
    }
    node = parent
  }

  // Arguments are evaluated before the call, so an await in them costs the
  // gesture even though it reads as coming after: open({ tabId: await x }).
  for (const argument of call.arguments) {
    found.push(...collectNodes(argument, ts.isAwaitExpression))
  }

  // Same statement as the call, textually earlier: `foo(await a) && open()`.
  const ownStatement = statementIn(scope.body ?? scope, call)
  if (ownStatement) {
    for (const await_ of collectNodes(ownStatement, ts.isAwaitExpression)) {
      if (await_.getStart() < call.getStart() && !containsNode(await_, call)) found.push(await_)
    }
  }

  return [...new Set(found)]
}

/** Whether `await_` sits inside a block within `statement` that always exits. */
function discountedByGuard(await_, statement) {
  let node = await_
  while (node && node !== statement) {
    if (ts.isBlock(node) && alwaysExits(node)) return true
    node = node.parent
  }
  return false
}

/**
 * Whether an `await` carries the opt-out marker.
 *
 * Looks at its own line and the lines just above, so both a trailing comment
 * and a comment block introducing the statement work.
 */
function isAllowed(sourceFile, node) {
  const { line } = sourceFile.getLineAndCharacterOfPosition(node.getStart())
  const lines = sourceFile.getFullText().split('\n')
  const from = Math.max(0, line - ALLOW_LOOKBEHIND_LINES)
  return lines.slice(from, line + 1).some((text) => text.includes(ALLOW_MARKER))
}

function describe(sourceFile, node) {
  const { line, character } = sourceFile.getLineAndCharacterOfPosition(node.getStart())
  return { line: line + 1, column: character + 1 }
}

/**
 * Find every gesture-safety violation in `files` (absolute paths).
 *
 * Returns `{ file, line, column, rule, message }` objects; empty means clean.
 */
export function findGestureViolations(files) {
  return files.flatMap((file) =>
    analyzeSource(path.relative(REPO_ROOT, file), readFileSync(file, 'utf8'))
  )
}

/**
 * Analyze one source file's text. Split out so the rule is testable without
 * touching the filesystem — `relative` is only used for reporting and for the
 * single-opener check.
 */
export function analyzeSource(relative, text) {
  const violations = []
  if (!text.includes(GUARDED_CALL)) return violations

  const sourceFile = ts.createSourceFile(relative, text, ts.ScriptTarget.Latest, true)
  const guardedCalls = collectNodes(
    sourceFile,
    (node) => ts.isCallExpression(node) && node.expression.getText(sourceFile) === GUARDED_CALL
  )
  if (guardedCalls.length === 0) return violations

  if (relative !== ALLOWED_CALLER) {
    for (const call of guardedCalls) {
      violations.push({
        file: relative,
        ...describe(sourceFile, call),
        rule: 'single-opener',
        message:
          `${GUARDED_CALL}() may only be called from ${ALLOWED_CALLER}. ` +
          'Every entry point shares one opener so the gesture rules live in one place (repo rule 19).'
      })
    }
    return violations
  }

  for (const call of guardedCalls) {
    const scope = enclosingFunction(call)
    if (!scope) continue

    for (const await_ of awaitsReaching(call, scope)) {
      // Awaiting the guarded call itself is fine — that is not "before".
      if (containsNode(await_, call)) continue
      // An await inside a nested callback does not precede the call.
      if (enclosingFunction(await_) !== scope) continue
      if (isAllowed(sourceFile, await_)) continue

      violations.push({
        file: relative,
        ...describe(sourceFile, await_),
        rule: 'no-await-before-open',
        message:
          `This await runs before ${GUARDED_CALL}() at line ${describe(sourceFile, call).line} ` +
          'and expires the user gesture, so Chrome will refuse to open the panel. ' +
          `Dispatch without awaiting, or annotate with "${ALLOW_MARKER} — <reason>".`
      })
    }
  }

  return violations
}

function main() {
  const files = listTypeScriptFiles(path.join(REPO_ROOT, 'src'))
  const violations = findGestureViolations(files)

  if (process.argv.includes('--json')) {
    console.log(JSON.stringify(violations, null, 2))
  } else if (violations.length === 0) {
    console.log(`gesture safety: OK (${files.length} files scanned)`)
  } else {
    console.error(`gesture safety: ${violations.length} violation(s)\n`)
    for (const v of violations) {
      console.error(`  ${v.file}:${v.line}:${v.column}  [${v.rule}]`)
      console.error(`    ${v.message}\n`)
    }
  }
  process.exit(violations.length === 0 ? 0 : 1)
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  main()
}
