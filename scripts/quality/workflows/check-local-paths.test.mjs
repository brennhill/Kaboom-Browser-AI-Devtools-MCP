// check-local-paths.test.mjs — Prevents workflow commands from drifting to deleted files.

import assert from 'node:assert/strict'
import path from 'node:path'
import test from 'node:test'
import { fileURLToPath } from 'node:url'
import { extractLocalPaths, findMissingWorkflowPaths } from './check-local-paths.mjs'

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../..')

test('every explicit first-party workflow command path exists', () => {
  assert.deepEqual(findMissingWorkflowPaths(repoRoot), [])
})

test('workflow path extraction is bounded to explicit first-party files', () => {
  assert.deepEqual(
    extractLocalPaths(`
      run: ./scripts/quality/check.sh --flag
      run: node scripts/quality/check.mjs
      uses: ./.github/workflows/reusable.yml
      path: lifecycle-evidence.json
      run: /tmp/private.sh
      run: echo scripts/generated-at-runtime.js
    `),
    [
      'scripts/quality/check.sh',
      'scripts/quality/check.mjs',
      '.github/workflows/reusable.yml',
      'scripts/generated-at-runtime.js'
    ]
  )
})
