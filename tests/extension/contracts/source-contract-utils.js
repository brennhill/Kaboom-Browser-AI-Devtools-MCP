// @ts-nocheck
/**
 * @fileoverview source-contract-utils.js — Static-analysis helpers for the
 * prevention guardrail tests. Uses the TypeScript compiler (already a dev dep)
 * to AST-scan the TypeScript under src/, so the checks have real reach over source that
 * ESLint (JS-only in this repo) cannot see.
 */

import ts from 'typescript'
import { readFileSync, readdirSync, statSync } from 'node:fs'
import { join } from 'node:path'
import { fileURLToPath } from 'node:url'

export const SRC_ROOT = fileURLToPath(new URL('../../../src/', import.meta.url))

/** Recursively list .ts source files (excluding .d.ts) under the given src subpaths. */
export function listTsFiles(subpaths) {
  const out = []
  const walk = (path) => {
    const st = statSync(path)
    if (st.isDirectory()) {
      for (const name of readdirSync(path)) walk(join(path, name))
    } else if (path.endsWith('.ts') && !path.endsWith('.d.ts')) {
      out.push(path)
    }
  }
  for (const sub of subpaths) walk(join(SRC_ROOT, sub))
  return out
}

/** Read a src file relative to SRC_ROOT as text. */
export function readSrc(relPath) {
  return readFileSync(join(SRC_ROOT, relPath), 'utf8')
}

/** Parse a file into a TS SourceFile with parent pointers set. */
export function parseSource(file) {
  return ts.createSourceFile(file, readFileSync(file, 'utf8'), ts.ScriptTarget.Latest, true)
}

/** Best-effort name of a function-like node (declaration, method, or var/prop-assigned arrow). */
function functionName(node) {
  if (node.name && ts.isIdentifier(node.name)) return node.name.text
  const parent = node.parent
  if (parent && ts.isVariableDeclaration(parent) && ts.isIdentifier(parent.name)) return parent.name.text
  if (parent && ts.isPropertyAssignment(parent) && ts.isIdentifier(parent.name)) return parent.name.text
  return null
}

function isFunctionLike(node) {
  return (
    ts.isFunctionDeclaration(node) ||
    ts.isFunctionExpression(node) ||
    ts.isArrowFunction(node) ||
    ts.isMethodDeclaration(node)
  )
}

/** A catch block whose ONLY statement is `return false` or `return null`. */
function isSingleReturnFalsy(block) {
  if (!block || !ts.isBlock(block) || block.statements.length !== 1) return false
  const stmt = block.statements[0]
  if (!ts.isReturnStatement(stmt) || !stmt.expression) return false
  return stmt.expression.kind === ts.SyntaxKind.FalseKeyword || stmt.expression.kind === ts.SyntaxKind.NullKeyword
}

/**
 * Find catch clauses that mask a failure as a benign `return false/null` inside a
 * state-mutating function (name starts with a mutation verb). This is the Class 3
 * "silent failure on a mutating path" shape (CLAUDE.md rule 25) — the same shape
 * as the terminal-start 409 and relay WriteToFirst bugs.
 *
 * Attribution walks up to the nearest *named* enclosing function, so a catch
 * nested inside an anonymous `.then(() => …)` / chrome callback still counts
 * against the mutating function that owns it (the dominant MV3 shape). Known
 * limitation: only bare `return false/null` is matched — an object return that
 * masks failure as success (the 409-with-token shape) is not machine-detectable
 * here without semantic analysis, so it is not covered.
 */
export function findFailLoudViolations(sourceFile, mutationVerbRe) {
  const violations = []
  const stack = [] // names of enclosing function-likes (null for anonymous)
  const visit = (node) => {
    const pushed = isFunctionLike(node)
    if (pushed) stack.push(functionName(node))
    if (ts.isCatchClause(node) && isSingleReturnFalsy(node.block)) {
      // Nearest enclosing function that actually has a name.
      let enclosing = null
      for (let i = stack.length - 1; i >= 0; i--) {
        if (stack[i]) {
          enclosing = stack[i]
          break
        }
      }
      if (enclosing && mutationVerbRe.test(enclosing)) {
        const { line } = sourceFile.getLineAndCharacterOfPosition(node.getStart())
        violations.push({ fn: enclosing, line: line + 1 })
      }
    }
    ts.forEachChild(node, visit)
    if (pushed) stack.pop()
  }
  visit(sourceFile)
  return violations
}

/** True if any CallExpression in the file calls a bare identifier named `callee`. */
export function fileContainsCall(sourceFile, callee) {
  let found = false
  const visit = (node) => {
    if (ts.isCallExpression(node) && ts.isIdentifier(node.expression) && node.expression.text === callee) {
      found = true
    }
    if (!found) ts.forEachChild(node, visit)
  }
  visit(sourceFile)
  return found
}

/**
 * True if the named function (declaration or the initializer of `const NAME = …`)
 * contains a CallExpression to a bare identifier `callee`. Verifies routing —
 * that the entry point actually CALLS the shared helper, not merely imports or
 * defines a same-named symbol.
 */
export function functionContainsCall(sourceFile, fnName, callee) {
  let target = null
  const findFn = (node) => {
    if (isFunctionLike(node) && functionName(node) === fnName) target = node
    if (!target) ts.forEachChild(node, findFn)
  }
  findFn(sourceFile)
  if (!target) return false
  let found = false
  const scan = (node) => {
    if (ts.isCallExpression(node) && ts.isIdentifier(node.expression) && node.expression.text === callee) {
      found = true
    }
    if (!found) ts.forEachChild(node, scan)
  }
  ts.forEachChild(target, scan)
  return found
}
