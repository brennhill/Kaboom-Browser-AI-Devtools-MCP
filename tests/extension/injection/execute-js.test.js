// @ts-nocheck
/**
 * @fileoverview execute-js.test.js — Tests for safeSerializeForExecute and executeJavaScript.
 * Covers serialization of primitives, objects, arrays, circular references, depth limits,
 * special types (Error, Date, RegExp, DOM Node), and JS execution with expression/statement
 * forms, promises, timeouts, errors, and CSP detection.
 */

import { test, describe, beforeEach } from 'node:test'
import assert from 'node:assert'

// --------------------------------------------------------------------------
// safeSerializeForExecute
// --------------------------------------------------------------------------

describe('safeSerializeForExecute', () => {
  let safeSerializeForExecute

  beforeEach(async () => {
    ;({ safeSerializeForExecute } = await import('../../../extension/inject/execute-js.js'))
  })

  // -- Primitives --

  test('preserves string', () => {
    assert.strictEqual(safeSerializeForExecute('hello'), 'hello')
  })

  test('preserves number', () => {
    assert.strictEqual(safeSerializeForExecute(42), 42)
    assert.strictEqual(safeSerializeForExecute(0), 0)
    assert.strictEqual(safeSerializeForExecute(-3.14), -3.14)
  })

  test('preserves boolean', () => {
    assert.strictEqual(safeSerializeForExecute(true), true)
    assert.strictEqual(safeSerializeForExecute(false), false)
  })

  test('preserves null', () => {
    assert.strictEqual(safeSerializeForExecute(null), null)
  })

  test('preserves undefined', () => {
    assert.strictEqual(safeSerializeForExecute(undefined), undefined)
  })

  // -- Functions --

  test('converts named function to [Function: name]', () => {
    function myFunc() {}
    assert.strictEqual(safeSerializeForExecute(myFunc), '[Function: myFunc]')
  })

  test('converts anonymous function to [Function: anonymous]', () => {
    // Use array slot to avoid variable-name inference
    const fn = [function () {}][0]
    assert.strictEqual(safeSerializeForExecute(fn), '[Function: anonymous]')
  })

  // -- Symbols --

  test('converts symbol to string representation', () => {
    assert.strictEqual(safeSerializeForExecute(Symbol('test')), 'Symbol(test)')
    assert.strictEqual(safeSerializeForExecute(Symbol()), 'Symbol()')
  })

  // -- Objects --

  test('serializes plain object', () => {
    assert.deepStrictEqual(safeSerializeForExecute({ a: 1, b: 'two' }), { a: 1, b: 'two' })
  })

  test('limits object keys to 50', () => {
    const big = {}
    for (let i = 0; i < 80; i++) big[`k${i}`] = i
    const result = safeSerializeForExecute(big)
    const keys = Object.keys(result)
    assert.ok(keys.length <= 51, `Expected <= 51 keys, got ${keys.length}`)
    assert.ok(result['...'].includes('more keys'))
  })

  // -- Arrays --

  test('serializes array', () => {
    assert.deepStrictEqual(safeSerializeForExecute([1, 'a', true]), [1, 'a', true])
  })

  test('limits array to 100 elements', () => {
    const big = Array.from({ length: 150 }, (_, i) => i)
    const result = safeSerializeForExecute(big)
    assert.strictEqual(result.length, 100)
    assert.strictEqual(result[99], 99)
  })

  // -- Circular references --

  test('handles circular reference in object', () => {
    const obj = { a: 1 }
    obj.self = obj
    const result = safeSerializeForExecute(obj)
    assert.strictEqual(result.a, 1)
    assert.strictEqual(result.self, '[Circular]')
  })

  test('handles circular reference in nested structure', () => {
    const a = { name: 'a' }
    const b = { name: 'b', ref: a }
    a.ref = b
    const result = safeSerializeForExecute(a)
    assert.strictEqual(result.ref.name, 'b')
    assert.strictEqual(result.ref.ref, '[Circular]')
  })

  // -- Max depth --

  test('returns [max depth exceeded] beyond depth 10', () => {
    let deep = { value: 'leaf' }
    for (let i = 0; i < 14; i++) deep = { n: deep }

    const result = safeSerializeForExecute(deep)
    let cur = result
    let depth = 0
    while (cur && typeof cur === 'object' && cur.n) {
      cur = cur.n
      depth++
    }
    assert.ok(depth <= 11, `Traversed ${depth} levels, expected <= 11`)
    // The leaf must be the sentinel string, not the original object
    assert.strictEqual(cur, '[max depth exceeded]')
  })

  // -- Error objects --

  test('serializes Error to { error, stack }', () => {
    const err = new Error('boom')
    const result = safeSerializeForExecute(err)
    assert.strictEqual(result.error, 'boom')
    assert.ok(typeof result.stack === 'string')
  })

  // -- Date objects --

  test('serializes Date to ISO string', () => {
    const d = new Date('2025-06-15T12:00:00.000Z')
    assert.strictEqual(safeSerializeForExecute(d), '2025-06-15T12:00:00.000Z')
  })

  // -- RegExp --

  test('serializes RegExp to toString', () => {
    assert.strictEqual(safeSerializeForExecute(/foo\d+/gi), '/foo\\d+/gi')
  })

  // -- DOM Node (mocked) --

  test('serializes DOM Node to [NODENAME#id]', () => {
    // Set up a minimal Node constructor so instanceof works
    const OrigNode = globalThis.Node
    globalThis.Node = function Node() {}
    globalThis.Node.prototype = {}

    const mockNode = { nodeName: 'DIV', id: 'app' }
    Object.setPrototypeOf(mockNode, globalThis.Node.prototype)

    const result = safeSerializeForExecute(mockNode)
    assert.strictEqual(result, '[DIV#app]')

    // Node without id
    const noId = { nodeName: 'SPAN' }
    Object.setPrototypeOf(noId, globalThis.Node.prototype)
    assert.strictEqual(safeSerializeForExecute(noId), '[SPAN]')

    // Restore
    if (OrigNode) globalThis.Node = OrigNode
    else delete globalThis.Node
  })

  test('serializes DOMRect-like host objects with prototype getters', () => {
    class MockDOMRect {}
    Object.defineProperty(MockDOMRect.prototype, 'x', { get: () => 100, enumerable: false })
    Object.defineProperty(MockDOMRect.prototype, 'y', { get: () => 200, enumerable: false })
    Object.defineProperty(MockDOMRect.prototype, 'width', { get: () => 80, enumerable: false })
    Object.defineProperty(MockDOMRect.prototype, 'height', { get: () => 32, enumerable: false })
    Object.defineProperty(MockDOMRect.prototype, 'top', { get: () => 200, enumerable: false })
    Object.defineProperty(MockDOMRect.prototype, 'right', { get: () => 180, enumerable: false })
    Object.defineProperty(MockDOMRect.prototype, 'bottom', { get: () => 232, enumerable: false })
    Object.defineProperty(MockDOMRect.prototype, 'left', { get: () => 100, enumerable: false })

    const result = safeSerializeForExecute(new MockDOMRect())
    assert.deepStrictEqual(result, {
      x: 100,
      y: 200,
      width: 80,
      height: 32,
      top: 200,
      right: 180,
      bottom: 232,
      left: 100
    })
  })
})

// --------------------------------------------------------------------------
// executeJavaScript
// --------------------------------------------------------------------------

describe('executeJavaScript', () => {
  let executeJavaScript

  beforeEach(async () => {
    ;({ executeJavaScript } = await import('../../../extension/inject/execute-js.js'))
  })

  // -- Expression form --

  test('evaluates a simple expression and returns result', async () => {
    const res = await executeJavaScript('1 + 2')
    assert.strictEqual(res.success, true)
    assert.strictEqual(res.result, 3)
  })

  test('evaluates an object literal expression', async () => {
    const res = await executeJavaScript('({ x: 10 })')
    assert.strictEqual(res.success, true)
    assert.deepStrictEqual(res.result, { x: 10 })
  })

  // -- Statement form fallback --

  test('falls back to statement form for try/catch', async () => {
    const res = await executeJavaScript("try { JSON.parse('{}') } catch(e) {}")
    assert.strictEqual(res.success, true)
  })

  test('falls back to statement form for if/else', async () => {
    const res = await executeJavaScript('if (true) { 1 } else { 2 }')
    assert.strictEqual(res.success, true)
  })

  test('statement form returns undefined for non-returning code', async () => {
    const res = await executeJavaScript('var x = 42')
    assert.strictEqual(res.success, true)
    assert.strictEqual(res.result, undefined)
  })

  // -- Promises --

  test('resolves a promise result', async () => {
    const res = await executeJavaScript('Promise.resolve(99)')
    assert.strictEqual(res.success, true)
    assert.strictEqual(res.result, 99)
  })

  test('handles promise rejection', async () => {
    const res = await executeJavaScript('Promise.reject(new Error("nope"))')
    assert.strictEqual(res.success, false)
    assert.strictEqual(res.error, 'promise_rejected')
    assert.strictEqual(res.message, 'nope')
  })

  // -- Timeout --

  test('returns execution_timeout for slow promise', async () => {
    const res = await executeJavaScript('new Promise(r => setTimeout(r, 500))', 50)
    assert.strictEqual(res.success, false)
    assert.strictEqual(res.error, 'execution_timeout')
    assert.ok(res.message.includes('50'))
  })

  // -- Error handling --

  test('catches synchronous errors', async () => {
    const res = await executeJavaScript('(() => { throw new Error("fail") })()')
    assert.strictEqual(res.success, false)
    assert.strictEqual(res.error, 'execution_error')
    assert.strictEqual(res.message, 'fail')
    assert.ok(res.stack)
  })

  test('catches ReferenceError', async () => {
    const res = await executeJavaScript('nonExistentVariable')
    assert.strictEqual(res.success, false)
    assert.strictEqual(res.error, 'execution_error')
    assert.ok(res.message.includes('nonExistentVariable'))
  })

  // -- CSP detection --

  test('detects Content Security Policy errors', async () => {
    // We cannot easily trigger a real CSP block in Node, so we test that
    // the CSP branch would fire by importing the module and calling with
    // a script that throws an error containing the CSP keyword.
    const res = await executeJavaScript(
      '(() => { throw new Error("Refused to evaluate: Content Security Policy") })()'
    )
    assert.strictEqual(res.success, false)
    assert.strictEqual(res.error, 'csp_blocked')
    assert.ok(res.message.includes('Content Security Policy'))
  })

  test('detects unsafe-eval CSP errors', async () => {
    const res = await executeJavaScript('(() => { throw new Error("Blocked by unsafe-eval policy") })()')
    assert.strictEqual(res.success, false)
    assert.strictEqual(res.error, 'csp_blocked')
  })

  test('detects Trusted Type CSP errors', async () => {
    const res = await executeJavaScript('(() => { throw new Error("Blocked by Trusted Type") })()')
    assert.strictEqual(res.success, false)
    assert.strictEqual(res.error, 'csp_blocked')
  })

  // -- Result serialization integration --

  test('serializes complex return values through safeSerializeForExecute', async () => {
    const res = await executeJavaScript('({ a: [1,2], b: new Date("2025-01-01T00:00:00Z") })')
    assert.strictEqual(res.success, true)
    assert.deepStrictEqual(res.result.a, [1, 2])
    assert.strictEqual(res.result.b, '2025-01-01T00:00:00.000Z')
  })

  // -- Top-level await (regression: issue #598) --

  test('supports a top-level await expression', async () => {
    const res = await executeJavaScript('await Promise.resolve(7)')
    assert.strictEqual(res.success, true)
    assert.strictEqual(res.result, 7)
  })

  test('supports top-level await between statements with a trailing return (act → wait → read)', async () => {
    const res = await executeJavaScript(
      'let x = 1; await new Promise(r => setTimeout(r, 5)); x = 42; return x'
    )
    assert.strictEqual(res.success, true)
    assert.strictEqual(res.result, 42)
  })

  test('top-level await still honors the timeout budget', async () => {
    const res = await executeJavaScript('await new Promise(r => setTimeout(r, 500)); return "late"', 50)
    assert.strictEqual(res.success, false)
    assert.strictEqual(res.error, 'execution_timeout')
    assert.ok(res.message.includes('50'))
  })

  test('a rejection from a top-level await surfaces as promise_rejected', async () => {
    const res = await executeJavaScript('await Promise.reject(new Error("async boom")); return 1')
    assert.strictEqual(res.success, false)
    assert.strictEqual(res.error, 'promise_rejected')
    assert.strictEqual(res.message, 'async boom')
  })

  // Guard: enabling async compilation must NOT reclassify synchronous throws,
  // which still compile via the (sync) expression form and throw at call time.
  test('synchronous throw remains execution_error, not promise_rejected', async () => {
    const res = await executeJavaScript('(() => { throw new Error("sync fail") })()')
    assert.strictEqual(res.success, false)
    assert.strictEqual(res.error, 'execution_error')
    assert.strictEqual(res.message, 'sync fail')
  })
})
