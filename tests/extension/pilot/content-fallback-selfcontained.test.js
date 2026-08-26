// @ts-nocheck
/**
 * @fileoverview content-fallback-selfcontained.test.js — Guards the chrome.scripting
 * injection contract for content extraction fallbacks. chrome.scripting serializes
 * ONLY the function passed as `func`; module-scope helpers and constants are lost
 * and would throw ReferenceError inside the page. Each FALLBACK_SCRIPTS entry must
 * therefore be fully self-contained (regression: the scripts used to call the
 * module-level pickMainElement, which does not exist after serialization).
 */

import { test } from 'node:test'
import assert from 'node:assert'
import ts from 'typescript'

const { FALLBACK_SCRIPTS } = await import('../../../extension/background/exec/content-fallback-scripts.js')

const BROWSER_AND_JS_GLOBALS = new Set([
  'document', 'window', 'location', 'navigator', 'URL', 'NodeFilter', 'getComputedStyle', 'fetch',
  'console', 'JSON', 'Math', 'Array', 'Object', 'String', 'Number', 'Boolean', 'Date', 'RegExp',
  'Error', 'TypeError', 'Promise', 'Set', 'Map', 'Symbol', 'parseInt', 'parseFloat', 'isNaN',
  'encodeURIComponent', 'decodeURIComponent', 'requestAnimationFrame', 'cancelAnimationFrame',
  'MutationObserver', 'ResizeObserver', 'IntersectionObserver', 'PerformanceObserver',
  'Node', 'Element', 'HTMLElement', 'HTMLDialogElement', 'HTMLInputElement', 'ShadowRoot',
  'CustomEvent', 'Event', 'EventTarget', 'DOMParser', 'XMLSerializer', 'AbortController',
  'setTimeout', 'clearTimeout', 'setInterval', 'clearInterval', 'queueMicrotask',
  'structuredClone', 'crypto', 'self', 'chrome', 'undefined', 'NaN', 'Infinity', 'globalThis',
  'arguments', 'this'
])

/** Returns identifiers referenced in `source` that are neither declared inside it nor known globals. */
const undeclaredReferences = (source) => {
  const sf = ts.createSourceFile('fn.js', source, ts.ScriptTarget.Latest, true, ts.ScriptKind.JS)
  const declared = new Set()
  const referenced = new Set()

  const declarePattern = (nameNode) => {
    if (!nameNode) return
    if (ts.isIdentifier(nameNode)) {
      declared.add(nameNode.text)
      return
    }
    for (const element of nameNode.elements || []) declarePattern(element.name || element)
  }

  const visit = (node) => {
    if (ts.isIdentifier(node)) {
      referenced.add(node.text)
      return
    }
    if (
      ts.isFunctionDeclaration(node) ||
      ts.isFunctionExpression(node) ||
      ts.isArrowFunction(node) ||
      ts.isConstructorDeclaration(node) ||
      ts.isMethodDeclaration(node) ||
      ts.isGetAccessorDeclaration(node) ||
      ts.isSetAccessorDeclaration(node)
    ) {
      if (node.name && ts.isIdentifier(node.name)) declared.add(node.name.text)
      for (const param of node.parameters) {
        declarePattern(param.name)
        // Default values are evaluated in the injected context: a module-scope
        // reference here breaks serialization exactly like one in the body.
        if (param.initializer) visit(param.initializer)
      }
      if (node.body) visit(node.body)
      return
    }
    if (ts.isVariableDeclaration(node) || ts.isBindingElement(node)) {
      declarePattern(node.name)
      if (node.initializer) visit(node.initializer)
      return
    }
    if (ts.isPropertyAccessExpression(node) || ts.isPropertySignature(node)) {
      visit(node.expression || node.name)
      return
    }
    if (ts.isPropertyAssignment(node)) {
      // Computed keys evaluate in the injected context too.
      if (node.name && ts.isComputedPropertyName(node.name)) visit(node.name.expression)
      visit(node.initializer)
      return
    }
    if (ts.isShorthandPropertyAssignment(node)) {
      referenced.add(node.name.text)
      return
    }
    if (ts.isCatchClause(node)) {
      if (node.variableDeclaration) declarePattern(node.variableDeclaration.name)
      visit(node.block)
      return
    }
    ts.forEachChild(node, visit)
  }
  visit(sf)

  const violations = []
  for (const name of referenced) {
    if (!declared.has(name) && !BROWSER_AND_JS_GLOBALS.has(name)) violations.push(name)
  }
  return violations.sort()
}

test('module-scope leaks through parameter initializers and computed keys are detected', () => {
  // The exact regression class this guard exists for, smuggled through the two
  // syntax forms the original visitor never visited.
  const leaky = `function broken(minLen = MODULE_THRESHOLD, scale = pickMainElement(100)) {
    const cfg = { [SELECTOR_LIMIT]: scale }
    return cfg[minLen]
  }`
  assert.deepStrictEqual(undeclaredReferences(leaky), [
    'MODULE_THRESHOLD',
    'SELECTOR_LIMIT',
    'pickMainElement'
  ])

  // Self-contained equivalents stay clean.
  const clean = `function fine(minLen = 100) {
    const cfg = { limit: minLen }
    return cfg[minLen]
  }`
  assert.deepStrictEqual(undeclaredReferences(clean), [])
})

test('every fallback script serializes without module-scope references', () => {
  assert.ok(Object.keys(FALLBACK_SCRIPTS).length >= 3, 'fallback registry should expose all three extractors')
  for (const [name, fn] of Object.entries(FALLBACK_SCRIPTS)) {
    const violations = undeclaredReferences(fn.toString())
    assert.deepStrictEqual(
      violations,
      [],
      `${name} references identifiers absent after chrome.scripting serialization: ${violations.join(', ')}`
    )
  }
})
