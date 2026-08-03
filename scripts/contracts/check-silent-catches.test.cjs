// check-silent-catches.test.cjs — Regression tests for silent-catch policy enforcement.
'use strict'

const assert = require('node:assert/strict')
const { mkdirSync, mkdtempSync, writeFileSync } = require('node:fs')
const { tmpdir } = require('node:os')
const { join } = require('node:path')
const { spawnSync } = require('node:child_process')
const { test } = require('node:test')

function check(source, relativePath = 'src/fixture.ts') {
  const root = mkdtempSync(join(tmpdir(), 'kaboom-silent-catch-'))
  const fixture = join(root, relativePath)
  mkdirSync(join(fixture, '..'), { recursive: true })
  writeFileSync(fixture, source)
  return spawnSync('node', ['scripts/contracts/check-silent-catches.cjs', root], {
    cwd: process.cwd(),
    encoding: 'utf8'
  })
}

test('rejects silent block and Promise catches', () => {
  assert.equal(check('try { work() } catch { return false }\n').status, 1)
  assert.equal(check('work().catch(() => {})\n').status, 1)
  assert.equal(check('work().catch(() => undefined)\n').status, 1)
  assert.equal(check('try { outer() } catch { try { inner() } catch { return null } }\n').status, 1)
  assert.equal(check('try { generated() } catch { return null }\n', 'scripts/templates/fixture.ts.tpl').status, 1)
})

test('accepts classified absence and explicit error propagation', () => {
  assert.equal(check('try { work() } catch { // EXPECTED_ABSENCE: optional page API.\n return false }\n').status, 0)
  assert.equal(check('work().catch((error) => { throw error })\n').status, 0)
})
