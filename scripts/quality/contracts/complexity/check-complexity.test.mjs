// check-complexity.test.mjs — Pins cyclomatic complexity counting and gate scope rules.
import test from 'node:test'
import assert from 'node:assert/strict'
import { mkdtempSync, mkdirSync, writeFileSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join, relative } from 'node:path'
import { complexityOfSource, check, measureSource, evaluateBudgets, MAX_LENGTH } from './check-complexity.mjs'

function count(source, name = 'branches') {
  const findings = complexityOfSource('probe.ts', source, 0)
  const target = findings.find((f) => f.function === name)
  assert.ok(target, `want a named ${name} function, got ${JSON.stringify(findings)}`)
  return target.complexity
}

test('counts branches: if, logical operators, loops, case, catch, ternary', () => {
  const source = `
function branches(a, b, n) {
  let total = 1
  if (a) total++
  if (a && b) total++
  if (a || b) total++
  total += a ?? 0
  total += a ? 1 : 0
  for (let i = 0; i < n; i++) total++
  for (const x of [1]) total++
  for (const k in {}) total++
  while (false) total++
  do { total++ } while (false)
  try { total++ } catch { total++ }
  switch (n) {
    case 1: total++
    case 2: total++
    default: total++
  }
  return total
}
`
  // 1 base + 3 if + 2 logical + 1 nullish + 1 ternary + 5 loops + 1 catch + 3 clauses = 17
  assert.equal(count(source), 17)
})

test('nested functions are judged separately, not added to the parent', () => {
  const source = `
function outer(a) {
  if (a) {
    const inner = () => {
      if (a && a) throw new Error('nested')
    }
    inner()
  }
}
`
  assert.equal(count(source, 'outer'), 2)
})

test('reports only authored files over the limit', () => {
  const root = mkdtempSync(join(tmpdir(), 'ccx-'))
  try {
    const write = (rel, body) => {
      const path = join(root, rel)
      mkdirSync(join(path, '..'), { recursive: true })
      writeFileSync(path, body)
    }
    const hot = `export function hot(v) {
  switch (v) {
${Array.from({ length: 16 }, (_, i) => `    case ${i}: return ${i}`).join('\n')}
  }
}
`
    write(join('src', 'hot.ts'), hot)
    write(join('src', 'cool.ts'), 'export const cool = () => 1\n')
    write(join('src', 'generated', 'gen.ts'), hot)
    write(join('src', 'types', 'wire', 'wire-gen.ts'), hot)
    write(join('src', 'prims', 'dom-primitives-form.ts'), hot)
    write(join('src', 'hot.test.ts'), hot)
    write(join('src', 'bundle.bundled.js'), hot)
    write(join('src', 'decl.d.ts'), hot)

    const violations = check(root, 15)
    assert.equal(violations.length, 1, JSON.stringify(violations, null, 2))
    assert.match(relative(root, violations[0].file), /src[\\/]hot\.ts$/)
    assert.equal(violations[0].function, 'hot')
    assert.ok(violations[0].line > 0)
  } finally {
    rmSync(root, { recursive: true, force: true })
  }
})

test('template partials are checked as the source of generated primitives', () => {
  const root = mkdtempSync(join(tmpdir(), 'ccx-'))
  try {
    const path = join(root, 'scripts', 'templates', 'partials', '_hot.tpl')
    mkdirSync(join(path, '..'), { recursive: true })
    writeFileSync(
      path,
      `function resolveActionTarget(el: Element | null): boolean {
${Array.from({ length: 16 }, (_, i) => `  if (el && el.id === '${i}') return true`).join('\n')}
  return false
}
`
    )
    const violations = check(root, 15)
    assert.equal(violations.length, 1, JSON.stringify(violations, null, 2))
    assert.equal(violations[0].function, 'resolveActionTarget')
  } finally {
    rmSync(root, { recursive: true, force: true })
  }
})

test('param budget counts parameters excluding this and is hard', () => {
  const source = `
function wide(this: Window, a, b, c, d, e, f, g) { return a }
function narrow(this: Window, a, b, c, d, e, f) { return a }
`
  const measured = measureSource('probe.ts', source)
  const violations = evaluateBudgets(measured, 1000, () => MAX_LENGTH)
  assert.equal(violations.length, 1, JSON.stringify(violations))
  assert.equal(violations[0].kind, 'params')
  assert.equal(violations[0].function, 'wide')
  assert.equal(violations[0].params, 7)
})

test('length budget measures the whole node and ratchets via allowance', () => {
  const body = Array.from({ length: 100 }, (_, i) => `  _ = ${i}`).join('\n')
  const source = `function long() {\n${body}\n}\n`
  const measured = measureSource('probe.ts', source)
  assert.equal(measured[0].lines, 102, 'whole-node length including signature and closing brace')

  const frozen = evaluateBudgets(measured, 1000, () => 102)
  assert.equal(frozen.length, 0, 'frozen at current length: allowed')

  const grown = evaluateBudgets(measured, 1000, () => 101)
  assert.equal(grown.length, 1)
  assert.equal(grown[0].kind, 'length')
  assert.equal(grown[0].lines, 102)
})

test('named functions are keyed by name and line so same names cannot inherit allowances', () => {
  const body = Array.from({ length: 100 }, (_, i) => `  _ = ${i}`).join('\n')
  const source = `function handler() {\n${body}\n}\nconst other = () => {\n${body}\n}\n`
  const measured = measureSource('probe.ts', source)
  const first = measured.find((f) => f.function === 'handler')
  const second = measured.find((f) => f.function === '(anonymous)')
  assert.equal(first.key, 'handler:1')
  assert.equal(second.key, '(anonymous):103')
  // Only the first function's key has a frozen allowance; the second must fail.
  const violations = evaluateBudgets(measured, 1000, (key) => (key === 'probe.ts:handler:1' ? 102 : MAX_LENGTH))
  assert.equal(violations.length, 1)
  assert.equal(violations[0].function, '(anonymous)')
})

test('anonymous overlength functions are keyed by line for the baseline', () => {
  const body = Array.from({ length: 100 }, (_, i) => `  _ = ${i}`).join('\n')
  const source = `const handler = () => {\n${body}\n}\n`
  const measured = measureSource('probe.ts', source)
  assert.equal(measured[0].key, '(anonymous):1')
  const violations = evaluateBudgets(measured, 1000, (key) => (key === 'probe.ts:(anonymous):1' ? 102 : MAX_LENGTH))
  assert.equal(violations.length, 0)
})
