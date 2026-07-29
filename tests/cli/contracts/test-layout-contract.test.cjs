// test-layout-contract.test.cjs — Structural contracts for change-coupled test families.
'use strict'

const assert = require('node:assert/strict')
const { readdirSync } = require('node:fs')
const { join } = require('node:path')
const { describe, test } = require('node:test')

const TARGET_ROOTS = ['scripts/tests', 'tests/cli', 'tests/extension']

function immediateFiles(directory) {
  return readdirSync(directory, { withFileTypes: true }).filter((entry) => entry.isFile())
}

function directoriesBelow(root) {
  const directories = [root]
  for (const entry of readdirSync(root, { withFileTypes: true })) {
    if (entry.isDirectory()) directories.push(join(root, entry.name))
  }
  return directories
}

describe('test layout contract', () => {
  test('target roots contain only change-coupled family directories', () => {
    for (const root of TARGET_ROOTS) {
      assert.deepEqual(immediateFiles(root), [], `${root} must not accumulate uncategorized files`)
    }
  })

  test('every target family contains at most ten files', () => {
    for (const root of TARGET_ROOTS) {
      for (const directory of directoriesBelow(root)) {
        assert.ok(
          immediateFiles(directory).length <= 10,
          `${directory} exceeds the 10-file module limit`
        )
      }
    }
  })
})
