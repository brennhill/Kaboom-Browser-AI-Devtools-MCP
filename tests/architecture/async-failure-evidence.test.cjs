// async-failure-evidence.test.cjs — Prevents production promise failures from disappearing.
/* global __dirname */

const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const test = require('node:test')
const ts = require('typescript')

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
      const markerLine = source.slice(0, match.index).split('\n').length - 1
      const lines = source.split('\n')
      const rationaleLines = []
      for (const line of lines.slice(markerLine, markerLine + 5)) {
        if (!/^\s*(?:\/\/|\/?\*)/.test(line)) break
        rationaleLines.push(line)
      }
      const rationale = rationaleLines.join('\n')
      if (!/\b(?:normal(?:ly)?|expected|benign)\b/i.test(rationale) || !/\blogging\b/i.test(rationale)) {
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

test('comment-only promise catches explicitly classify expected absence', () => {
  const violations = []
  for (const file of walk(sourceRoot).filter((candidate) => /\.(?:ts|js)$/.test(candidate))) {
    const source = fs.readFileSync(file, 'utf8')
    if (source.includes('AUTO-GENERATED FILE. DO NOT EDIT DIRECTLY.')) continue
    const sourceFile = ts.createSourceFile(file, source, ts.ScriptTarget.Latest, true)
    const visit = (node) => {
      if (
        ts.isCallExpression(node) &&
        ts.isPropertyAccessExpression(node.expression) &&
        node.expression.name.text === 'catch'
      ) {
        const handler = node.arguments[0]
        if (
          (ts.isArrowFunction(handler) || ts.isFunctionExpression(handler)) &&
          ts.isBlock(handler.body) &&
          handler.body.statements.length === 0 &&
          !handler.body.getFullText(sourceFile).includes('EXPECTED_ABSENCE:')
        ) {
          const line = sourceFile.getLineAndCharacterOfPosition(node.getStart(sourceFile)).line + 1
          violations.push(`${path.relative(sourceRoot, file)}:${line}`)
        }
      }
      ts.forEachChild(node, visit)
    }
    visit(sourceFile)
  }
  assert.deepEqual(
    violations,
    [],
    `comment-only promise catches require an EXPECTED_ABSENCE rationale:\n${violations.join('\n')}`
  )
})

test('empty synchronous catches explicitly classify expected absence', () => {
  const violations = []
  for (const file of walk(sourceRoot).filter((candidate) => /\.(?:ts|js)$/.test(candidate))) {
    const source = fs.readFileSync(file, 'utf8')
    if (source.includes('AUTO-GENERATED FILE. DO NOT EDIT DIRECTLY.')) continue
    const sourceFile = ts.createSourceFile(file, source, ts.ScriptTarget.Latest, true)
    const visit = (node) => {
      if (
        ts.isCatchClause(node) &&
        node.block.statements.length === 0 &&
        !node.block.getFullText(sourceFile).includes('EXPECTED_ABSENCE:')
      ) {
        const line = sourceFile.getLineAndCharacterOfPosition(node.getStart(sourceFile)).line + 1
        violations.push(`${path.relative(sourceRoot, file)}:${line}`)
      }
      ts.forEachChild(node, visit)
    }
    visit(sourceFile)
  }
  assert.deepEqual(
    violations,
    [],
    `empty synchronous catches require an EXPECTED_ABSENCE rationale:\n${violations.join('\n')}`
  )
})
