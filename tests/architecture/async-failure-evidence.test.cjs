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

test('expected absence comments explicitly justify silence', () => {
  const violations = []
  for (const file of walk(sourceRoot).filter((candidate) => /\.(?:ts|js)$/.test(candidate))) {
    const source = fs.readFileSync(file, 'utf8')
    for (const match of source.matchAll(/EXPECTED_ABSENCE:([\s\S]{0,300})/g)) {
      const rationale = match[1].split(/\n\s*(?:\}|\)|return|[a-zA-Z_$][\w$]*\s*[.(])/)[0]
      if (!/\b(?:normal|expected|benign)\b/i.test(rationale) || !/\blogging\b/i.test(rationale)) {
        violations.push(`${path.relative(sourceRoot, file)}:${source.slice(0, match.index).split('\n').length}`)
      }
    }
  }
  assert.deepEqual(
    violations,
    [],
    `EXPECTED_ABSENCE must explain why absence is normal and why logging would mislead:\n${violations.join('\n')}`
  )
})
