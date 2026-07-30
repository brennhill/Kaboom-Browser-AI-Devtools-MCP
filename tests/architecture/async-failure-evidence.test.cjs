// async-failure-evidence.test.cjs — Prevents production promise failures from disappearing.
/* global __dirname */

const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const test = require('node:test')

const sourceRoot = path.resolve(__dirname, '../../src')

function walk(directory) {
  return fs.readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
    const target = path.join(directory, entry.name)
    return entry.isDirectory() ? walk(target) : [target]
  })
}

test('production promise rejections retain failure evidence', () => {
  const violations = []
  for (const file of walk(sourceRoot).filter((candidate) => /\.(?:ts|js)$/.test(candidate))) {
    const source = fs.readFileSync(file, 'utf8')
    if (/\.catch\(\s*\(\)\s*=>\s*(?:undefined|\{\s*\})\s*\)/s.test(source)) {
      violations.push(path.relative(sourceRoot, file))
    }
  }
  assert.deepEqual(
    violations,
    [],
    `silent promise catches require logging or an explicit EXPECTED_ABSENCE rationale:\n${violations.join('\n')}`
  )
})
