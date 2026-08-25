// check-raw-jq.test.mjs — Pins the raw-jq ratchet.
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { countPipedJq, evaluate, lowered } from './check-raw-jq.mjs'

test('counts a pipe into jq', () => {
  assert.equal(countPipedJq(`printf '%s' "$t" | jq -r '.count'`), 1)
})

test('counts a bare pipe into jq with no arguments', () => {
  assert.equal(countPipedJq(`echo "$t" | jq`), 1)
})

// A checked-in table has guaranteed syntax, so reading it with jq is safe and
// gating it would bury the real finding in noise.
test('does not count jq reading a file argument', () => {
  assert.equal(countPipedJq(`jq -r '.cases[0].name' "$EXPECTED_TABLE"`), 0)
})

// The explanation of why piped jq is unsafe necessarily contains the pattern.
test('does not count the pattern inside a comment', () => {
  assert.equal(countPipedJq(`# never do: printf '%s' "$t" | jq -r '.x'`), 0)
  assert.equal(countPipedJq(`    # indented | jq comment`), 0)
})

test('counts each offending line once across a file', () => {
  const source = ["a | jq '.x'", "# b | jq '.y'", "c | jq -r '.z'", 'd=1'].join('\n')
  assert.equal(countPipedJq(source), 2)
})

test('does not count an identifier merely ending in jq', () => {
  assert.equal(countPipedJq(`echo "$t" | myjq -r '.x'`), 0)
})

test("growth above a file's budget is a violation", () => {
  const violations = evaluate({ 'a.sh': 5 }, { 'a.sh': 3 })
  assert.equal(violations.length, 1)
  assert.match(violations[0], /rose from 3 to 5/)
})

test('a file at or below its budget passes', () => {
  assert.deepEqual(evaluate({ 'a.sh': 3 }, { 'a.sh': 3 }), [])
  assert.deepEqual(evaluate({ 'a.sh': 1 }, { 'a.sh': 3 }), [])
})

// A new script must not start out with the unsafe form, whatever the old ones do.
test('a file absent from the baseline may not introduce piped jq', () => {
  const violations = evaluate({ 'new.sh': 1 }, {})
  assert.equal(violations.length, 1)
  assert.match(violations[0], /had none/)
  assert.match(violations[0], /json_field/)
})

test('a migrated file disappearing from the scan is not a violation', () => {
  assert.deepEqual(evaluate({}, { 'a.sh': 3 }), [])
})

// Without this the budget keeps the old slack and the next author can undo a
// migration for free.
test('refreezing lowers a budget to the current count and never raises it', () => {
  assert.deepEqual(lowered({ 'a.sh': 1 }, { 'a.sh': 3 }), { 'a.sh': 1 })
  assert.deepEqual(lowered({ 'a.sh': 5 }, { 'a.sh': 3 }), { 'a.sh': 3 })
})

test('refreezing records a file the baseline had never seen', () => {
  assert.deepEqual(lowered({ 'new.sh': 2 }, {}), { 'new.sh': 2 })
})
