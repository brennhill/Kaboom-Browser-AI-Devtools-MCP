// check-go-test-targets.test.mjs — Prevents workflow test selectors from targeting empty packages.

import assert from 'node:assert/strict'
import path from 'node:path'
import test from 'node:test'
import { fileURLToPath } from 'node:url'
import { extractTargetedGoTests, findMissingTargetedGoTests } from './check-go-test-targets.mjs'

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../..')

test('extracts targeted Go test package and selector pairs', () => {
  assert.deepEqual(
    extractTargetedGoTests(`
      run: go test ./internal/state -run TestRoot -count=1
      run: KABOOM_STATE_DIR=/tmp/test go test -race -tags=integration ./cmd/browser-agent/integration/bridge -run 'TestFastStart_.*' -count=3
      run: go test ./...
    `),
    [
      { packagePath: './internal/state', pattern: 'TestRoot', tags: '' },
      { packagePath: './cmd/browser-agent/integration/bridge', pattern: 'TestFastStart_.*', tags: 'integration' }
    ]
  )
})

test('every targeted workflow Go test resolves in its selected package', () => {
  assert.deepEqual(findMissingTargetedGoTests(repoRoot), [])
})
