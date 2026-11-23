// @ts-nocheck
/**
 * dormant-tests-sigpipe.test.cjs — Regression for kaboom-6b7e: under `set -o pipefail`,
 * `echo "$BIG" | grep -q` fails when grep exits on its first match while echo is still
 * writing (SIGPIPE), so check-dormant-tests.sh intermittently reported tracked tests as
 * missing from the runner inventory. Membership checks must not pipe into grep -q.
 */
const test = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')

const script = fs.readFileSync(
  path.join(__dirname, '..', '..', '..', 'scripts', 'quality', 'contracts', 'check-dormant-tests.sh'),
  'utf8'
)

test('check-dormant-tests never pipes a variable into grep -q under pipefail', () => {
  assert.match(script, /set -euo pipefail/)
  assert.doesNotMatch(script, /echo "\$[A-Z_]+" \| grep -q/, 'use a here-string (grep -q ... <<<"$VAR") instead')
})
