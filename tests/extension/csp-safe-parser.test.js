// @ts-nocheck
/**
 * @fileoverview csp-safe-parser.test.js — Tests for the CSP-safe expression parser.
 * Covers literals, globals, property chains, method calls, bracket access, constructors,
 * array/object literals, assignment, return stripping, and rejection of unsupported patterns.
 */

import { describe, test } from 'node:test'
import assert from 'node:assert'

const { parseExpression } = await import('../../extension/background/csp-safe/parser.js')

// Helper: assert parse succeeds and return the command
function mustParse(input) {
  const result = parseExpression(input)
  assert.strictEqual(result.ok, true, `Expected parse success for: ${input}, got: ${result.reason}`)
  return result.command
}

// Helper: assert parse fails
function mustFail(input) {
  const result = parseExpression(input)
  assert.strictEqual(result.ok, false, `Expected parse failure for: ${input}`)
  return result.reason
}

// =============================================================================
// Literals
// =============================================================================

describe('CSP-safe parser: literals', () => {
  test('parses string literal (single quotes)', () => {
    const cmd = mustParse("'hello'")
    assert.deepStrictEqual(cmd.expr, { type: 'literal', value: 'hello' })
  })

  test('parses string literal (double quotes)', () => {
    const cmd = mustParse('"world"')
    assert.deepStrictEqual(cmd.expr, { type: 'literal', value: 'world' })
  })

  test('parses string with escapes', () => {
    const cmd = mustParse("'it\\'s'")
    assert.deepStrictEqual(cmd.expr, { type: 'literal', value: "it's" })
  })

  test('parses backtick template literal (no interpolation)', () => {
    const cmd = mustParse('`hello`')
    assert.deepStrictEqual(cmd.expr, { type: 'literal', value: 'hello' })
  })

  test('parses integer', () => {
    const cmd = mustParse('42')
    assert.deepStrictEqual(cmd.expr, { type: 'literal', value: 42 })
  })

  test('parses negative number', () => {
    const cmd = mustParse('-3.14')
    assert.deepStrictEqual(cmd.expr, { type: 'literal', value: -3.14 })
  })

  test('parses zero', () => {
    const cmd = mustParse('0')
    assert.deepStrictEqual(cmd.expr, { type: 'literal', value: 0 })
  })

  test('parses true', () => {
    const cmd = mustParse('true')
    assert.deepStrictEqual(cmd.expr, { type: 'literal', value: true })
  })

  test('parses false', () => {
    const cmd = mustParse('false')
    assert.deepStrictEqual(cmd.expr, { type: 'literal', value: false })
  })

  test('parses null', () => {
    const cmd = mustParse('null')
    assert.deepStrictEqual(cmd.expr, { type: 'literal', value: null })
  })

  test('parses undefined', () => {
    const cmd = mustParse('undefined')
    assert.deepStrictEqual(cmd.expr, { type: 'undefined' })
  })
})

// =============================================================================
// Global references
// =============================================================================

describe('CSP-safe parser: globals', () => {
  test('parses bare identifier as global', () => {
    const cmd = mustParse('document')
    assert.deepStrictEqual(cmd.expr, { type: 'global', name: 'document' })
  })

  test('parses window as global', () => {
    const cmd = mustParse('window')
    assert.deepStrictEqual(cmd.expr, { type: 'global', name: 'window' })
  })

  test('parses property chain: document.title', () => {
    const cmd = mustParse('document.title')
    assert.strictEqual(cmd.expr.type, 'chain')
    assert.deepStrictEqual(cmd.expr.root, { type: 'global', name: 'document' })
    assert.deepStrictEqual(cmd.expr.steps, [{ op: 'access', key: 'title' }])
  })

  test('parses deep property chain: window.location.href', () => {
    const cmd = mustParse('window.location.href')
    assert.strictEqual(cmd.expr.type, 'chain')
    assert.deepStrictEqual(cmd.expr.root, { type: 'global', name: 'window' })
    assert.deepStrictEqual(cmd.expr.steps, [
      { op: 'access', key: 'location' },
      { op: 'access', key: 'href' }
    ])
  })
})

// =============================================================================
// Method calls
// =============================================================================

describe('CSP-safe parser: method calls', () => {
  test('parses no-arg method call', () => {
    const cmd = mustParse('document.getSelection()')
    assert.strictEqual(cmd.expr.type, 'chain')
    assert.deepStrictEqual(cmd.expr.steps, [
      { op: 'access', key: 'getSelection' },
      { op: 'call', args: [] }
    ])
  })

  test('parses single-arg method call', () => {
    const cmd = mustParse("document.querySelector('#app')")
    assert.strictEqual(cmd.expr.type, 'chain')
    const callStep = cmd.expr.steps[1]
    assert.strictEqual(callStep.op, 'call')
    assert.strictEqual(callStep.args.length, 1)
    assert.deepStrictEqual(callStep.args[0], { type: 'literal', value: '#app' })
  })

  test('parses multi-arg method call', () => {
    const cmd = mustParse("localStorage.setItem('key', 'value')")
    assert.strictEqual(cmd.expr.type, 'chain')
    const callStep = cmd.expr.steps[1]
    assert.strictEqual(callStep.op, 'call')
    assert.strictEqual(callStep.args.length, 2)
    assert.deepStrictEqual(callStep.args[0], { type: 'literal', value: 'key' })
    assert.deepStrictEqual(callStep.args[1], { type: 'literal', value: 'value' })
  })

  test('parses chained method calls', () => {
    const cmd = mustParse("document.querySelector('a').getAttribute('href')")
    assert.strictEqual(cmd.expr.type, 'chain')
    assert.strictEqual(cmd.expr.steps.length, 4) // access, call, access, call
    assert.deepStrictEqual(cmd.expr.steps[0], { op: 'access', key: 'querySelector' })
    assert.strictEqual(cmd.expr.steps[1].op, 'call')
    assert.deepStrictEqual(cmd.expr.steps[2], { op: 'access', key: 'getAttribute' })
    assert.strictEqual(cmd.expr.steps[3].op, 'call')
  })

  test('parses nested args', () => {
    const cmd = mustParse("getComputedStyle(document.querySelector('h1')).color")
    assert.strictEqual(cmd.expr.type, 'chain')
    // root is getComputedStyle global, call with nested chain arg, then .color
    assert.deepStrictEqual(cmd.expr.root, { type: 'global', name: 'getComputedStyle' })
    const callStep = cmd.expr.steps[0]
    assert.strictEqual(callStep.op, 'call')
    assert.strictEqual(callStep.args.length, 1)
    // The nested arg should be a chain: document.querySelector('h1')
    const nestedArg = callStep.args[0]
    assert.strictEqual(nestedArg.type, 'chain')
    assert.deepStrictEqual(cmd.expr.steps[1], { op: 'access', key: 'color' })
  })
})

// =============================================================================
// Bracket access
// =============================================================================

describe('CSP-safe parser: bracket access', () => {
  test('parses string key bracket access', () => {
    const cmd = mustParse("obj['key']")
    assert.strictEqual(cmd.expr.type, 'chain')
    assert.deepStrictEqual(cmd.expr.steps[0], { op: 'access', key: 'key' })
  })

  test('parses numeric index bracket access', () => {
    const cmd = mustParse('arr[0]')
    assert.strictEqual(cmd.expr.type, 'chain')
    assert.deepStrictEqual(cmd.expr.steps[0], { op: 'index', index: 0 })
  })

  test('parses chained bracket access', () => {
    const cmd = mustParse("obj['a']['b']")
    assert.strictEqual(cmd.expr.type, 'chain')
    assert.deepStrictEqual(cmd.expr.steps[0], { op: 'access', key: 'a' })
    assert.deepStrictEqual(cmd.expr.steps[1], { op: 'access', key: 'b' })
  })
})

// =============================================================================
// Array literals
// =============================================================================

describe('CSP-safe parser: array literals', () => {
  test('parses empty array', () => {
    const cmd = mustParse('[]')
    assert.deepStrictEqual(cmd.expr, { type: 'array', elements: [] })
  })

  test('parses array with elements', () => {
    const cmd = mustParse("[1, 'two', true]")
    assert.strictEqual(cmd.expr.type, 'array')
    assert.strictEqual(cmd.expr.elements.length, 3)
    assert.deepStrictEqual(cmd.expr.elements[0], { type: 'literal', value: 1 })
    assert.deepStrictEqual(cmd.expr.elements[1], { type: 'literal', value: 'two' })
    assert.deepStrictEqual(cmd.expr.elements[2], { type: 'literal', value: true })
  })

  test('parses array with dynamic values', () => {
    const cmd = mustParse('[document.title, location.href]')
    assert.strictEqual(cmd.expr.type, 'array')
    assert.strictEqual(cmd.expr.elements.length, 2)
    assert.strictEqual(cmd.expr.elements[0].type, 'chain')
    assert.strictEqual(cmd.expr.elements[1].type, 'chain')
  })
})

// =============================================================================
// Object literals
// =============================================================================

describe('CSP-safe parser: object literals', () => {
  test('parses empty object', () => {
    const cmd = mustParse('({})')
    assert.deepStrictEqual(cmd.expr, { type: 'object', entries: [] })
  })

  test('parses object with entries', () => {
    const cmd = mustParse("({title: document.title, url: location.href})")
    assert.strictEqual(cmd.expr.type, 'object')
    assert.strictEqual(cmd.expr.entries.length, 2)
    assert.strictEqual(cmd.expr.entries[0].key, 'title')
    assert.strictEqual(cmd.expr.entries[0].value.type, 'chain')
    assert.strictEqual(cmd.expr.entries[1].key, 'url')
    assert.strictEqual(cmd.expr.entries[1].value.type, 'chain')
  })

  test('parses object with string keys', () => {
    const cmd = mustParse("({'my-key': 42})")
    assert.strictEqual(cmd.expr.type, 'object')
    assert.strictEqual(cmd.expr.entries[0].key, 'my-key')
    assert.deepStrictEqual(cmd.expr.entries[0].value, { type: 'literal', value: 42 })
  })
})

// =============================================================================
// Constructors
// =============================================================================

describe('CSP-safe parser: constructors', () => {
  test('parses new URL()', () => {
    const cmd = mustParse("new URL('https://example.com')")
    assert.strictEqual(cmd.expr.type, 'chain')
    assert.deepStrictEqual(cmd.expr.root, { type: 'global', name: 'URL' })
    assert.strictEqual(cmd.expr.steps[0].op, 'construct')
    assert.deepStrictEqual(cmd.expr.steps[0].args[0], { type: 'literal', value: 'https://example.com' })
  })

  test('parses new URL().pathname', () => {
    const cmd = mustParse("new URL('https://example.com/path').pathname")
    assert.strictEqual(cmd.expr.type, 'chain')
    assert.strictEqual(cmd.expr.steps[0].op, 'construct')
    assert.deepStrictEqual(cmd.expr.steps[1], { op: 'access', key: 'pathname' })
  })
})

// =============================================================================
// Assignment
// =============================================================================

describe('CSP-safe parser: assignment', () => {
  test('parses simple assignment', () => {
    const cmd = mustParse("document.title = 'New Title'")
    assert.ok(cmd.assign)
    assert.strictEqual(cmd.assign.key, 'title')
    assert.deepStrictEqual(cmd.assign.target, { type: 'global', name: 'document' })
    assert.deepStrictEqual(cmd.expr, { type: 'literal', value: 'New Title' })
  })

  test('parses nested assignment', () => {
    const cmd = mustParse("window.app.config.debug = true")
    assert.ok(cmd.assign)
    assert.strictEqual(cmd.assign.key, 'debug')
    assert.deepStrictEqual(cmd.assign.target, { type: 'global', name: 'window' })
    assert.deepStrictEqual(cmd.assign.steps, [
      { op: 'access', key: 'app' },
      { op: 'access', key: 'config' }
    ])
  })
})

// =============================================================================
// Return stripping
// =============================================================================

describe('CSP-safe parser: return stripping', () => {
  test('strips return keyword', () => {
    const cmd = mustParse('return document.title')
    assert.strictEqual(cmd.expr.type, 'chain')
    assert.deepStrictEqual(cmd.expr.root, { type: 'global', name: 'document' })
    assert.deepStrictEqual(cmd.expr.steps, [{ op: 'access', key: 'title' }])
  })
})

// =============================================================================
// Rejection of unsupported patterns
// =============================================================================

describe('CSP-safe parser: rejection', () => {
  test('rejects arrow functions', () => {
    const reason = mustFail('() => 42')
    assert.ok(reason.length > 0)
  })

  test('rejects function keyword', () => {
    const reason = mustFail('function foo() {}')
    assert.ok(reason.length > 0)
  })

  test('rejects if statement', () => {
    const reason = mustFail('if (true) { 1 }')
    assert.ok(reason.length > 0)
  })

  test('rejects for loop', () => {
    const reason = mustFail('for (let i = 0; i < 10; i++) {}')
    assert.ok(reason.length > 0)
  })

  test('rejects while loop', () => {
    const reason = mustFail('while (true) {}')
    assert.ok(reason.length > 0)
  })

  test('rejects var/let/const', () => {
    mustFail('var x = 1')
    mustFail('let x = 1')
    mustFail('const x = 1')
  })

  test('rejects arithmetic operators', () => {
    mustFail('1 + 2')
  })

  test('rejects ternary', () => {
    mustFail('true ? 1 : 2')
  })

  test('rejects async/await', () => {
    mustFail('async () => {}')
    mustFail('await fetch("/")')
  })

  test('rejects IIFE', () => {
    mustFail('(function() { return 1 })()')
  })

  test('rejects template literal with interpolation', () => {
    mustFail('`hello ${name}`')
  })

  test('rejects empty string', () => {
    mustFail('')
  })

  test('rejects whitespace only', () => {
    mustFail('   ')
  })

  test('rejects spread', () => {
    mustFail('[...arr]')
  })

  test('rejects destructuring', () => {
    mustFail('const {a} = obj')
  })
})

// =============================================================================
// Edge cases
// =============================================================================

describe('CSP-safe parser: edge cases', () => {
  test('handles leading/trailing whitespace', () => {
    const cmd = mustParse('  document.title  ')
    assert.strictEqual(cmd.expr.type, 'chain')
  })

  test('handles semicolons', () => {
    const cmd = mustParse('document.title;')
    assert.strictEqual(cmd.expr.type, 'chain')
  })

  test('parses localStorage.getItem', () => {
    const cmd = mustParse("localStorage.getItem('token')")
    assert.strictEqual(cmd.expr.type, 'chain')
    assert.deepStrictEqual(cmd.expr.root, { type: 'global', name: 'localStorage' })
  })

  test('parses document.querySelectorAll with tag', () => {
    const cmd = mustParse("document.querySelectorAll('div')")
    assert.strictEqual(cmd.expr.type, 'chain')
  })
})
