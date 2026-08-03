// run-flake-detection.test.mjs — Deterministic flake campaign and replay tests.

import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

import { buildPlan, executePlan, seededShuffle } from './run-flake-detection.mjs'

test('seededShuffle is stable and does not mutate input', () => {
  const input = ['a', 'b', 'c', 'd', 'e', 'f']
  const first = seededShuffle(input, 4242)
  const second = seededShuffle(input, 4242)
  assert.deepEqual(first, second)
  assert.deepEqual(input, ['a', 'b', 'c', 'd', 'e', 'f'])
  assert.notDeepEqual(first, seededShuffle(input, 4243))
})

test('buildPlan records exact order, seed, concurrency, and pressure', () => {
  const plan = buildPlan({
    seed: 99,
    runs: 2,
    concurrency: 3,
    goPackages: ['./internal/a', './internal/b'],
    jsFiles: ['b.test.mjs', 'a.test.mjs']
  })
  assert.equal(plan.schema_version, '1')
  assert.equal(plan.seed, 99)
  assert.equal(plan.runs.length, 2)
  assert.equal(plan.resource_pressure.gomaxprocs, 3)
  assert.equal(plan.resource_pressure.node_test_concurrency, 3)
  assert.equal(plan.runs[0].commands[0].test_order.length, 2)
  assert.equal(plan.runs[0].commands[1].test_order.length, 2)
  assert.match(plan.replay_command, /KABOOM_FLAKE_SEED=99/)
})

test('a passing reproduction never turns the original campaign green', async () => {
  const plan = buildPlan({
    seed: 7,
    runs: 1,
    concurrency: 2,
    goPackages: ['./internal/a'],
    jsFiles: ['a.test.mjs']
  })
  const calls = []
  const outcomes = [
    { exit_code: 1, stdout: '', stderr: 'original failure', duration_ms: 12 },
    { exit_code: 0, stdout: 'retry passed', stderr: '', duration_ms: 10 },
    { exit_code: 0, stdout: 'retry passed', stderr: '', duration_ms: 9 },
    { exit_code: 0, stdout: 'js passed', stderr: '', duration_ms: 8 }
  ]
  const evidence = await executePlan(
    plan,
    async (command) => {
      calls.push(command)
      return outcomes.shift()
    },
    2
  )

  assert.equal(evidence.exit_code, 1)
  assert.equal(evidence.original_failures.length, 1)
  assert.equal(evidence.original_failures[0].stderr, 'original failure')
  assert.equal(evidence.reproductions[0].classification, 'flaky')
  assert.equal(evidence.reproductions[0].attempts.length, 2)
  assert.equal(calls[0].id, calls[1].id)
  assert.equal(calls[0].id, calls[2].id)
})

test('scheduled workflow always retains machine-readable replay evidence', () => {
  const workflow = readFileSync('.github/workflows/flake-detection.yml', 'utf8')
  assert.match(workflow, /schedule:/)
  assert.match(workflow, /KABOOM_FLAKE_SEED:/)
  assert.match(workflow, /if: always\(\)/)
  assert.match(workflow, /if-no-files-found: error/)
  assert.match(workflow, /KABOOM_TELEMETRY_DISABLED: "1"/)
})
