// @ts-nocheck
/**
 * @fileoverview fail-loud-contract.test.js — Guardrail for bug Class 3 ("silent
 * failure on a mutating path", CLAUDE.md rule 25).
 *
 * A state-mutating function (name starts with start/save/write/persist/upload/
 * stop/spawn/connect) must not catch an error and mask it as a benign
 * `return false` / `return null` — that is exactly how the terminal-start 409 and
 * the relay WriteToFirst bugs hid real failures. ESLint is JS-only in this repo,
 * so this AST-scans the TypeScript source directly.
 *
 * If this fails: don't swallow into a benign return. Either handle the error,
 * rethrow, or distinguish "expected/absent" from "actually failed" and surface
 * the latter (see docs/core/reliability/bug-class-audit.md).
 */

import { describe, test } from 'node:test'
import assert from 'node:assert'
import ts from 'typescript'
import { listTsFiles, parseSource, findFailLoudViolations, SRC_ROOT } from '../contracts/source-contract-utils.js'

// Verbs that denote a state mutation / side effect whose failure must be visible.
const MUTATION_VERB = /^(start|save|write|persist|upload|stop|spawn|connect)/i

describe('fail-loud contract (Class 3, rule 25)', () => {
  test('no state-mutating function swallows an error into a bare return false/null', () => {
    const files = listTsFiles(['background', 'popup', 'content', 'lib', 'inject'])
    const violations = []
    for (const file of files) {
      const source = parseSource(file)
      for (const v of findFailLoudViolations(source, MUTATION_VERB)) {
        violations.push(`${file.replace(SRC_ROOT, 'src/')}:${v.line}  ${v.fn}() — catch { return ${'false/null'} }`)
      }
    }
    assert.strictEqual(
      violations.length,
      0,
      `Fail-loud violations (a mutating function masking failure as a benign return):\n  ${violations.join('\n  ')}`
    )
  })

  test('the detector actually trips on the anti-pattern (not vacuously passing)', () => {
    const bad = ts.createSourceFile(
      'synthetic.ts',
      'async function saveThing() { try { await doIt() } catch { return false } }',
      ts.ScriptTarget.Latest,
      true
    )
    assert.strictEqual(findFailLoudViolations(bad, MUTATION_VERB).length, 1, 'detector must flag a mutating swallow')

    const good = ts.createSourceFile(
      'synthetic2.ts',
      'function isReady() { try { return check() } catch { return false } }',
      ts.ScriptTarget.Latest,
      true
    )
    assert.strictEqual(findFailLoudViolations(good, MUTATION_VERB).length, 0, 'a non-mutating predicate is fine')
  })

  test('the detector reaches catches nested in promise/callback arrows (the MV3 shape)', () => {
    // The dominant real shape: a mutating side effect runs inside a .then()/chrome
    // callback arrow. Attribution must walk up to the named mutating function.
    const nested = ts.createSourceFile(
      'synthetic3.ts',
      'function startSession() { doThing().then(() => { try { risky() } catch { return null } }) }',
      ts.ScriptTarget.Latest,
      true
    )
    assert.strictEqual(
      findFailLoudViolations(nested, MUTATION_VERB).length,
      1,
      'a swallow inside a callback of a mutating function must still be flagged'
    )
  })
})
