// @ts-nocheck
/**
 * @fileoverview csp-safe-executor.test.js — Tests for the CSP-safe structured command executor.
 * Covers value resolution, chain resolution with this binding, assignment, promise handling,
 * serialization, and error wrapping.
 */

import { describe, test, beforeEach } from 'node:test'
import assert from 'node:assert'

const { cspSafeExecutor } = await import('../../../extension/background/exec/csp-safe/executor.js')

// =============================================================================
// Value resolution
// =============================================================================

describe('CSP-safe executor: value resolution', () => {
  test('resolves string literal', () => {
    const result = cspSafeExecutor({ expr: { type: 'literal', value: 'hello' } })
    assert.strictEqual(result.success, true)
    assert.strictEqual(result.result, 'hello')
  })

  test('resolves number literal', () => {
    const result = cspSafeExecutor({ expr: { type: 'literal', value: 42 } })
    assert.strictEqual(result.success, true)
    assert.strictEqual(result.result, 42)
  })

  test('resolves boolean literal', () => {
    const result = cspSafeExecutor({ expr: { type: 'literal', value: true } })
    assert.strictEqual(result.success, true)
    assert.strictEqual(result.result, true)
  })

  test('resolves null literal', () => {
    const result = cspSafeExecutor({ expr: { type: 'literal', value: null } })
    assert.strictEqual(result.success, true)
    assert.strictEqual(result.result, null)
  })

  test('resolves undefined', () => {
    const result = cspSafeExecutor({ expr: { type: 'undefined' } })
    assert.strictEqual(result.success, true)
    assert.strictEqual(result.result, undefined)
  })

  test('resolves global reference', () => {
    globalThis.__testVal = 'works'
    const result = cspSafeExecutor({ expr: { type: 'global', name: '__testVal' } })
    assert.strictEqual(result.success, true)
    assert.strictEqual(result.result, 'works')
    delete globalThis.__testVal
  })

  test('resolves undefined global as undefined', () => {
    const result = cspSafeExecutor({ expr: { type: 'global', name: '__nonexistent' } })
    assert.strictEqual(result.success, true)
    assert.strictEqual(result.result, undefined)
  })
})

// =============================================================================
// Chain resolution
// =============================================================================

describe('CSP-safe executor: chain resolution', () => {
  beforeEach(() => {
    globalThis.__testObj = {
      name: 'test',
      nested: { deep: 'value' },
      items: ['a', 'b', 'c'],
      greet: function (who) { return `hello ${who}` },
      getThis: function () { return this }
    }
  })

  test('resolves property access', () => {
    const result = cspSafeExecutor({
      expr: {
        type: 'chain',
        root: { type: 'global', name: '__testObj' },
        steps: [{ op: 'access', key: 'name' }]
      }
    })
    assert.strictEqual(result.success, true)
    assert.strictEqual(result.result, 'test')
  })

  test('resolves nested property access', () => {
    const result = cspSafeExecutor({
      expr: {
        type: 'chain',
        root: { type: 'global', name: '__testObj' },
        steps: [
          { op: 'access', key: 'nested' },
          { op: 'access', key: 'deep' }
        ]
      }
    })
    assert.strictEqual(result.success, true)
    assert.strictEqual(result.result, 'value')
  })

  test('resolves index access', () => {
    const result = cspSafeExecutor({
      expr: {
        type: 'chain',
        root: { type: 'global', name: '__testObj' },
        steps: [
          { op: 'access', key: 'items' },
          { op: 'index', index: 1 }
        ]
      }
    })
    assert.strictEqual(result.success, true)
    assert.strictEqual(result.result, 'b')
  })

  test('resolves method call with args', () => {
    const result = cspSafeExecutor({
      expr: {
        type: 'chain',
        root: { type: 'global', name: '__testObj' },
        steps: [
          { op: 'access', key: 'greet' },
          { op: 'call', args: [{ type: 'literal', value: 'world' }] }
        ]
      }
    })
    assert.strictEqual(result.success, true)
    assert.strictEqual(result.result, 'hello world')
  })

  test('preserves this binding for method calls', () => {
    const result = cspSafeExecutor({
      expr: {
        type: 'chain',
        root: { type: 'global', name: '__testObj' },
        steps: [
          { op: 'access', key: 'getThis' },
          { op: 'call', args: [] }
        ]
      }
    })
    assert.strictEqual(result.success, true)
    // getThis() returns `this` which is __testObj, serialized as an object
    assert.strictEqual(result.result.name, 'test')
  })

  test('resolves nested args from global chain', () => {
    globalThis.__testArg = { val: 42 }
    const result = cspSafeExecutor({
      expr: {
        type: 'chain',
        root: { type: 'global', name: '__testObj' },
        steps: [
          { op: 'access', key: 'greet' },
          {
            op: 'call',
            args: [{
              type: 'chain',
              root: { type: 'global', name: '__testArg' },
              steps: [{ op: 'access', key: 'val' }]
            }]
          }
        ]
      }
    })
    assert.strictEqual(result.success, true)
    assert.strictEqual(result.result, 'hello 42')
    delete globalThis.__testArg
  })
})

// =============================================================================
// Array and object construction
// =============================================================================

describe('CSP-safe executor: array and object construction', () => {
  test('constructs array from elements', () => {
    const result = cspSafeExecutor({
      expr: {
        type: 'array',
        elements: [
          { type: 'literal', value: 1 },
          { type: 'literal', value: 'two' },
          { type: 'literal', value: true }
        ]
      }
    })
    assert.strictEqual(result.success, true)
    assert.deepStrictEqual(result.result, [1, 'two', true])
  })

  test('constructs object from entries', () => {
    const result = cspSafeExecutor({
      expr: {
        type: 'object',
        entries: [
          { key: 'a', value: { type: 'literal', value: 1 } },
          { key: 'b', value: { type: 'literal', value: 'two' } }
        ]
      }
    })
    assert.strictEqual(result.success, true)
    assert.deepStrictEqual(result.result, { a: 1, b: 'two' })
  })
})

// =============================================================================
// Constructor (new)
// =============================================================================

describe('CSP-safe executor: constructors', () => {
  test('constructs new URL', () => {
    const result = cspSafeExecutor({
      expr: {
        type: 'chain',
        root: { type: 'global', name: 'URL' },
        steps: [
          { op: 'construct', args: [{ type: 'literal', value: 'https://example.com/path' }] },
          { op: 'access', key: 'pathname' }
        ]
      }
    })
    assert.strictEqual(result.success, true)
    assert.strictEqual(result.result, '/path')
  })
})

// =============================================================================
// Assignment
// =============================================================================

describe('CSP-safe executor: assignment', () => {
  test('assigns to property', () => {
    globalThis.__assignTarget = { value: 'old' }
    const result = cspSafeExecutor({
      expr: { type: 'literal', value: 'new' },
      assign: {
        target: { type: 'global', name: '__assignTarget' },
        steps: [],
        key: 'value'
      }
    })
    assert.strictEqual(result.success, true)
    assert.strictEqual(globalThis.__assignTarget.value, 'new')
    delete globalThis.__assignTarget
  })

  test('assigns to nested property', () => {
    globalThis.__assignTarget = { nested: { prop: 'old' } }
    const result = cspSafeExecutor({
      expr: { type: 'literal', value: 'updated' },
      assign: {
        target: { type: 'global', name: '__assignTarget' },
        steps: [{ op: 'access', key: 'nested' }],
        key: 'prop'
      }
    })
    assert.strictEqual(result.success, true)
    assert.strictEqual(globalThis.__assignTarget.nested.prop, 'updated')
    delete globalThis.__assignTarget
  })
})

// =============================================================================
// Promise handling
// =============================================================================

describe('CSP-safe executor: promise handling', () => {
  test('resolves async results', async () => {
    globalThis.__asyncFn = () => Promise.resolve(99)
    const result = await cspSafeExecutor({
      expr: {
        type: 'chain',
        root: { type: 'global', name: '__asyncFn' },
        steps: [{ op: 'call', args: [] }]
      }
    })
    assert.strictEqual(result.success, true)
    assert.strictEqual(result.result, 99)
    delete globalThis.__asyncFn
  })

  test('handles promise rejection', async () => {
    globalThis.__asyncFn = () => Promise.reject(new Error('async fail'))
    const result = await cspSafeExecutor({
      expr: {
        type: 'chain',
        root: { type: 'global', name: '__asyncFn' },
        steps: [{ op: 'call', args: [] }]
      }
    })
    assert.strictEqual(result.success, false)
    assert.strictEqual(result.error, 'promise_rejected')
    assert.strictEqual(result.message, 'async fail')
    delete globalThis.__asyncFn
  })
})

// =============================================================================
// Serialization
// =============================================================================

describe('CSP-safe executor: serialization', () => {
  test('serializes Date', () => {
    globalThis.__testDate = new Date('2025-01-01T00:00:00Z')
    const result = cspSafeExecutor({
      expr: { type: 'global', name: '__testDate' }
    })
    assert.strictEqual(result.success, true)
    assert.strictEqual(result.result, '2025-01-01T00:00:00.000Z')
    delete globalThis.__testDate
  })

  test('handles circular references', () => {
    const obj = { a: 1 }
    obj.self = obj
    globalThis.__circular = obj
    const result = cspSafeExecutor({
      expr: { type: 'global', name: '__circular' }
    })
    assert.strictEqual(result.success, true)
    assert.strictEqual(result.result.a, 1)
    assert.strictEqual(result.result.self, '[Circular]')
    delete globalThis.__circular
  })

  test('tags results with execution_mode', () => {
    const result = cspSafeExecutor({ expr: { type: 'literal', value: 1 } })
    assert.strictEqual(result.execution_mode, 'csp_safe_structured')
  })
})

// =============================================================================
// Error handling
// =============================================================================

describe('CSP-safe executor: errors', () => {
  test('wraps TypeError on null access', () => {
    const result = cspSafeExecutor({
      expr: {
        type: 'chain',
        root: { type: 'literal', value: null },
        steps: [{ op: 'access', key: 'foo' }]
      }
    })
    assert.strictEqual(result.success, false)
    assert.strictEqual(result.error, 'structured_execution_error')
    assert.ok(result.message.includes('null'))
  })

  test('wraps TypeError on calling non-function', () => {
    globalThis.__notFn = { x: 42 }
    const result = cspSafeExecutor({
      expr: {
        type: 'chain',
        root: { type: 'global', name: '__notFn' },
        steps: [
          { op: 'access', key: 'x' },
          { op: 'call', args: [] }
        ]
      }
    })
    assert.strictEqual(result.success, false)
    assert.strictEqual(result.error, 'structured_execution_error')
    delete globalThis.__notFn
  })

  test('all errors include execution_mode tag', () => {
    const result = cspSafeExecutor({
      expr: {
        type: 'chain',
        root: { type: 'literal', value: null },
        steps: [{ op: 'access', key: 'x' }]
      }
    })
    assert.strictEqual(result.execution_mode, 'csp_safe_structured')
  })
})
