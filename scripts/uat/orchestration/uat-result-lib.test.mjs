// uat-result-lib.test.mjs — Regression contracts for UAT result aggregation helpers.
// Docs: docs/features/feature/self-testing/index.md

import assert from 'node:assert/strict'
import { execFileSync } from 'node:child_process'
import { fileURLToPath } from 'node:url'
import test from 'node:test'

const library = fileURLToPath(new URL('./uat-result-lib.sh', import.meta.url))

function idsMatch(expected, actual) {
  const output = execFileSync(
    '/bin/bash',
    [
      '-c',
      'source "$1"; if uat_category_ids_match "$2" "$3"; then printf match; else printf mismatch; fi',
      'test',
      library,
      expected,
      actual
    ],
    { encoding: 'utf8' }
  )
  return output === 'match'
}

test('zero-padded and numeric category IDs identify the same category', () => {
  assert.equal(idsMatch('01', '1'), true)
  assert.equal(idsMatch('09', '9'), true)
  assert.equal(idsMatch('10', '10'), true)
})

test('different or malformed category IDs remain mismatches', () => {
  assert.equal(idsMatch('01', '10'), false)
  assert.equal(idsMatch('01', ''), false)
  assert.equal(idsMatch('01', 'category-1'), false)
})

// uat_suite_passed is the runner's only exit-status rule. Everything below is a
// question the runner answers on every UAT invocation, so a wrong answer here is
// a wrong CI verdict.
function suitePassed(pass, fail, aggregationErrors, leaks = '', timedOut = '') {
  const output = execFileSync(
    '/bin/bash',
    [
      '-c',
      'source "$1"; if uat_suite_passed "$2" "$3" "$4" "$5" "$6"; then printf pass; else printf fail; fi',
      'test',
      library,
      String(pass),
      String(fail),
      String(aggregationErrors),
      String(leaks),
      String(timedOut)
    ],
    { encoding: 'utf8' }
  )
  return output === 'pass'
}

test('a suite that ran no assertions at all does not pass', () => {
  // The regression this exists for: with every counter at zero the runner
  // printed "FAILURES: ... of 0 tests" and still exited 0, so an emptied
  // OFFLINE_CAT_IDS would have produced a green CI job that verified nothing.
  assert.equal(suitePassed(0, 0, 0), false)
})

test('a clean suite with passing assertions passes', () => {
  // Control: without this, a rule that always answered "fail" would satisfy
  // every other case here.
  assert.equal(suitePassed(1, 0, 0), true)
  assert.equal(suitePassed(412, 0, 0), true)
})

test('failures, aggregation errors, and leaked processes each fail the suite', () => {
  assert.equal(suitePassed(412, 1, 0), false)
  assert.equal(suitePassed(412, 0, 1), false)
  assert.equal(suitePassed(412, 0, 0, ' 21'), false)
})

test('a category killed at its deadline fails the suite despite reporting no failures', () => {
  // timeout(1) kills the category, its EXIT trap writes the assertions it
  // reached, and FAIL_COUNT stays 0. Trusting those counters would read a
  // truncated category as a complete one.
  assert.equal(suitePassed(412, 0, 0, '', ' 05(120s)'), false)
})

test('a counter that is not a number fails rather than being coerced', () => {
  // An unparseable counter means the aggregation broke. Reading it as zero
  // would turn a broken run into a passing one.
  assert.equal(suitePassed('', 0, 0), false)
  assert.equal(suitePassed(412, 'x', 0), false)
  assert.equal(suitePassed(412, 0, '-1'), false)
})
