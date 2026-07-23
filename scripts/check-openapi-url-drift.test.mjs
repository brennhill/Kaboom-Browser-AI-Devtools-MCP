// check-openapi-url-drift.test.mjs — unit tests for the URL drift checker.

import { describe, test } from 'node:test'
import assert from 'node:assert/strict'
import {
  extractDaemonPaths,
  pathMatchesSpec,
  readPathLiteral
} from './check-openapi-url-drift.js'

describe('readPathLiteral', () => {
  test('reads a simple path up to backtick', () => {
    assert.equal(readPathLiteral('/health`'), '/health')
  })

  test('reads a path up to whitespace', () => {
    assert.equal(readPathLiteral('/upgrade/install '), '/upgrade/install')
  })

  test('stops at query string', () => {
    assert.equal(readPathLiteral('/terminal?token=abc`'), '/terminal')
  })

  test('normalizes ${...} interpolations to {param}', () => {
    assert.equal(readPathLiteral('/clients/${id}`'), '/clients/{param}')
  })

  test('handles nested braces inside ${...}', () => {
    // Regression: a plain indexOf("}") on `${foo({a:1})}/extra` stops at the
    // inner `}` and truncates the suffix. Brace-depth tracking keeps the whole
    // interpolation and preserves the trailing path segment.
    assert.equal(readPathLiteral('/clients/${foo({a:1})}/extra`'), '/clients/{param}/extra')
    assert.equal(readPathLiteral('/a/${x({})}/b`'), '/a/{param}/b')
  })

  test('returns null on unbalanced ${...}', () => {
    assert.equal(readPathLiteral('/clients/${unterminated`'), null)
  })

  test('strips trailing sentence punctuation', () => {
    assert.equal(readPathLiteral('/api/foo.`'), '/api/foo')
    assert.equal(readPathLiteral('/api/foo, and more'), '/api/foo')
  })

  test('preserves internal dots (e.g., .json extension)', () => {
    assert.equal(readPathLiteral('/diagnostics.json`'), '/diagnostics.json')
  })

  test('returns null if it does not start with /', () => {
    assert.equal(readPathLiteral('health`'), null)
    assert.equal(readPathLiteral(''), null)
  })
})

describe('extractDaemonPaths', () => {
  test('extracts path from fetch template literal on serverUrl', () => {
    const src = 'await fetch(`${serverUrl}/health`, opts)'
    const hits = extractDaemonPaths(src, 'x.ts')
    assert.equal(hits.length, 1)
    assert.equal(hits[0].path, '/health')
    assert.equal(hits[0].line, 1)
  })

  test('extracts path from getServerUrl() and deps.getServerUrl()', () => {
    const src = [
      'postDaemonJSON(`${getServerUrl()}/screenshots`, b)',
      'postDaemonJSON(`${deps.getServerUrl()}/recordings/reveal`, b)'
    ].join('\n')
    const hits = extractDaemonPaths(src, 'x.ts')
    assert.deepEqual(hits.map((h) => h.path).sort(), ['/recordings/reveal', '/screenshots'])
  })

  test('ignores termUrl (terminal sub-server, not daemon)', () => {
    const src = 'fetch(`${termUrl}/terminal/start`)'
    assert.equal(extractDaemonPaths(src, 'x.ts').length, 0)
  })

  test('captures multiple hits on the same line', () => {
    const src = '[`${serverUrl}/a`, `${serverUrl}/b`]'
    const hits = extractDaemonPaths(src, 'x.ts')
    assert.deepEqual(hits.map((h) => h.path).sort(), ['/a', '/b'])
  })

  test('captures path references embedded in error-message strings', () => {
    // Error strings still reference real paths; they count toward the spec
    // check because a typo here is just as much drift as in a live fetch.
    const src = '`cannot reach daemon at ${serverUrl}/api/foo`'
    const hits = extractDaemonPaths(src, 'x.ts')
    assert.equal(hits.length, 1)
    assert.equal(hits[0].path, '/api/foo')
  })
})

describe('pathMatchesSpec', () => {
  const spec = new Set([
    '/health',
    '/clients/{id}',
    '/upgrade/install',
    '/api/os-automation/inject',
    '/diagnostics.json'
  ])

  test('exact match passes', () => {
    assert.ok(pathMatchesSpec('/health', spec))
    assert.ok(pathMatchesSpec('/upgrade/install', spec))
  })

  test('parameter segment on both sides matches', () => {
    assert.ok(pathMatchesSpec('/clients/{param}', spec))
  })

  test('literal-to-parameter across sides matches', () => {
    // spec has /clients/{id}; code path could be /clients/123 literal — also OK
    assert.ok(pathMatchesSpec('/clients/{anything}', spec))
  })

  test('unknown top-level path fails', () => {
    assert.equal(pathMatchesSpec('/api/extension-status', spec), false)
  })

  test('segment count mismatch fails', () => {
    assert.equal(pathMatchesSpec('/clients', spec), false)
    assert.equal(pathMatchesSpec('/clients/{id}/extra', spec), false)
  })

  test('dot-extension paths are literal, not stripped', () => {
    assert.ok(pathMatchesSpec('/diagnostics.json', spec))
    assert.equal(pathMatchesSpec('/diagnostics', spec), false)
  })
})
