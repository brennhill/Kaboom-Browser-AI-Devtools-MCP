// uat-harness-regressions.test.cjs — Regression contracts for deterministic comprehensive UAT accounting.
'use strict'

const assert = require('node:assert/strict')
const { execFileSync } = require('node:child_process')
const { readFileSync } = require('node:fs')
const { describe, test } = require('node:test')

function frameworkCall(command) {
  return execFileSync('/bin/bash', ['-c', `source scripts/tests/framework.sh; ${command}`], {
    cwd: process.cwd(),
    encoding: 'utf8'
  }).trim()
}

describe('comprehensive UAT harness regressions', () => {
  test('elapsed time uses numeric arithmetic without quoted operands', () => {
    assert.equal(frameworkCall('elapsed_seconds 100 106'), '6')
  })

  test('JSON boolean parsing preserves false instead of treating it as missing', () => {
    assert.equal(frameworkCall(`json_boolean '{"valid":true}' valid`), 'true')
    assert.equal(frameworkCall(`json_boolean '{"valid":false}' valid`), 'false')
    assert.equal(frameworkCall(`json_boolean '{}' valid`), '')
  })

  test('connected-extension categories share one sequential daemon port', () => {
    const runner = readFileSync('scripts/test-all-tools-comprehensive.sh', 'utf8')

    assert.match(runner, /UAT_PORT=7890/)
    assert.match(runner, /Running \d+ categories sequentially/)
    assert.doesNotMatch(runner, /Running \d+ parallel groups/)
    assert.doesNotMatch(runner, /PORT_GROUP\d+=/)
  })

  test('long-running categories retain complete result accounting', () => {
    const runner = readFileSync('scripts/test-all-tools-comprehensive.sh', 'utf8')
    const dynamicUpgrade = readFileSync('scripts/tests/cat-26-dynamic-upgrade.sh', 'utf8')

    assert.match(runner, /19\) echo 600/)
    assert.match(dynamicUpgrade.trimEnd(), /finish_category$/)
  })
})
