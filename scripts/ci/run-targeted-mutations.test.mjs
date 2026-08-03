// run-targeted-mutations.test.mjs — Proves mutation execution and score enforcement.
import assert from 'node:assert/strict'
import { execFileSync, spawnSync } from 'node:child_process'
import { mkdtempSync, mkdirSync, readFileSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import test from 'node:test'

const runner = join(process.cwd(), 'scripts/ci/run-targeted-mutations.mjs')

function fixture() {
  const root = mkdtempSync(join(tmpdir(), 'kaboom-mutation-test-'))
  mkdirSync(join(root, 'sample'))
  writeFileSync(join(root, 'go.mod'), 'module mutation.fixture\n\ngo 1.25.12\n')
  writeFileSync(join(root, 'sample', 'sample.go'), 'package sample\n\nfunc Positive(value int) bool { return value > 0 }\n')
  writeFileSync(
    join(root, 'sample', 'sample_test.go'),
    'package sample\n\nimport "testing"\n\nfunc TestPositive(t *testing.T) { if !Positive(1) || Positive(0) { t.Fatal("boundary") } }\n'
  )
  return root
}

test('mutation runner reports a killed semantic regression', () => {
  const root = fixture()
  const output = join(root, 'report.json')
  const config = join(root, 'cases.json')
  writeFileSync(config, JSON.stringify({
    version: 1,
    minimum_score: 100,
    cases: [{ id: 'positive_boundary', file: 'sample/sample.go', package: './sample', from: 'value > 0', to: 'value >= 0' }]
  }))

  execFileSync(process.execPath, [runner, '--root', root, '--config', config, '--output', output])
  const report = JSON.parse(readFileSync(output, 'utf8'))
  assert.equal(report.killed, 1)
  assert.equal(report.survived, 0)
  assert.equal(report.score, 100)
})

test('mutation runner fails its gate and records a surviving regression', () => {
  const root = fixture()
  const output = join(root, 'report.json')
  const config = join(root, 'cases.json')
  writeFileSync(config, JSON.stringify({
    version: 1,
    minimum_score: 100,
    cases: [{ id: 'untested_negative', file: 'sample/sample.go', package: './sample', from: 'value > 0', to: 'value != 0' }]
  }))

  const result = spawnSync(process.execPath, [runner, '--root', root, '--config', config, '--output', output])
  assert.notEqual(result.status, 0)
  const report = JSON.parse(readFileSync(output, 'utf8'))
  assert.equal(report.killed, 0)
  assert.equal(report.survived, 1)
  assert.deepEqual(report.survivors, ['untested_negative'])
})

test('mutation runner rejects a failing baseline instead of claiming a kill', () => {
  const root = fixture()
  const output = join(root, 'report.json')
  const config = join(root, 'cases.json')
  writeFileSync(join(root, 'sample', 'sample_test.go'), 'package sample\n\nimport "testing"\n\nfunc TestBroken(t *testing.T) { t.Fatal("baseline") }\n')
  writeFileSync(config, JSON.stringify({
    version: 1,
    minimum_score: 100,
    cases: [{ id: 'not_a_kill', file: 'sample/sample.go', package: './sample', from: 'value > 0', to: 'value >= 0' }]
  }))

  const result = spawnSync(process.execPath, [runner, '--root', root, '--config', config, '--output', output], { encoding: 'utf8' })
  assert.notEqual(result.status, 0)
  assert.match(result.stderr, /baseline failed for \.\/sample/)
})
