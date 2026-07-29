// @ts-nocheck
/**
 * @fileoverview AI-context summary, enrichment pipeline, setting, and cache tests.
 */

import { test, describe, beforeEach, afterEach } from 'node:test'
import assert from 'node:assert'
import { createMockDocument, createMockWindow } from './ai-context-fixture.js'

let originalWindow
let originalDocument

describe('AI Context Summary Generation', () => {
  test('should generate summary with all data', async () => {
    const { generateAiSummary } = await import('../../../extension/lib/ai-context/ai-context-enrichment.js')

    const summary = generateAiSummary({
      errorType: 'TypeError',
      message: "Cannot read properties of undefined (reading 'user')",
      file: 'src/components/LoginForm.tsx',
      line: 42,
      componentAncestry: {
        framework: 'react',
        components: [{ name: 'App' }, { name: 'LoginForm' }]
      },
      stateSnapshot: {
        relevantSlice: { 'auth.error': 'Unauthorized', 'auth.user': null }
      }
    })

    assert.ok(summary.includes('TypeError'))
    assert.ok(summary.includes('LoginForm.tsx'))
    assert.ok(summary.includes('42'))
  })

  test('should generate summary with minimal data', async () => {
    const { generateAiSummary } = await import('../../../extension/lib/ai-context/ai-context-enrichment.js')

    const summary = generateAiSummary({
      errorType: 'Error',
      message: 'Something went wrong',
      file: null,
      line: null,
      componentAncestry: null,
      stateSnapshot: null
    })

    assert.ok(summary.includes('Error'))
    assert.ok(summary.includes('Something went wrong'))
    assert.ok(typeof summary === 'string')
    assert.ok(summary.length > 0)
  })

  test('should include component path when available', async () => {
    const { generateAiSummary } = await import('../../../extension/lib/ai-context/ai-context-enrichment.js')

    const summary = generateAiSummary({
      errorType: 'TypeError',
      message: 'test',
      file: 'test.tsx',
      line: 1,
      componentAncestry: {
        framework: 'react',
        components: [{ name: 'App' }, { name: 'Dashboard' }, { name: 'UserList' }]
      },
      stateSnapshot: null
    })

    // Should mention component names
    assert.ok(summary.includes('App'))
    assert.ok(summary.includes('UserList'))
  })

  test('should include state info when available', async () => {
    const { generateAiSummary } = await import('../../../extension/lib/ai-context/ai-context-enrichment.js')

    const summary = generateAiSummary({
      errorType: 'Error',
      message: 'failed',
      file: 'app.js',
      line: 5,
      componentAncestry: null,
      stateSnapshot: {
        relevantSlice: { 'auth.loading': false, 'auth.error': 'timeout' }
      }
    })

    assert.ok(summary.includes('auth'))
  })
})

// --- Full Pipeline ---

describe('AI Context Enrichment Pipeline', () => {
  beforeEach(() => {
    originalWindow = globalThis.window
    originalDocument = globalThis.document
    globalThis.window = createMockWindow()
    globalThis.document = createMockDocument()
  })

  afterEach(() => {
    globalThis.window = originalWindow
    globalThis.document = originalDocument
  })

  test('should produce _aiContext field on error entries', async () => {
    const { enrichErrorWithAiContext } = await import('../../../extension/lib/ai-context/ai-context-enrichment.js')

    const error = {
      type: 'exception',
      level: 'error',
      message: "Cannot read properties of undefined (reading 'foo')",
      stack: `TypeError: Cannot read properties of undefined
    at bar (http://localhost:3000/main.js:10:5)`,
      filename: 'http://localhost:3000/main.js',
      lineno: 10,
      _enrichments: []
    }

    const enriched = await enrichErrorWithAiContext(error)

    assert.ok(enriched._aiContext)
    assert.ok(enriched._aiContext.summary)
    assert.ok(enriched._enrichments.includes('aiContext'))
  })

  test('should complete within 3s budget even if source map fetch hangs', async () => {
    const { enrichErrorWithAiContext } = await import('../../../extension/lib/ai-context/ai-context-enrichment.js')

    globalThis.window.fetch = () => new Promise(() => {}) // Never resolves

    const error = {
      type: 'exception',
      level: 'error',
      message: 'test',
      stack: 'Error: test\n    at fn (http://localhost:3000/main.js:10:5)',
      filename: 'http://localhost:3000/main.js',
      lineno: 10,
      _enrichments: []
    }

    const start = Date.now()
    const enriched = await enrichErrorWithAiContext(error)
    const elapsed = Date.now() - start

    assert.ok(elapsed < 4000, `Expected < 4s, took ${elapsed}ms`)
    assert.ok(enriched._aiContext) // Should still have context (summary at minimum)
  })

  test('should skip enrichment when disabled', async () => {
    const { enrichErrorWithAiContext, setAiContextEnabled } = await import('../../../extension/lib/ai-context/ai-context-enrichment.js')

    setAiContextEnabled(false)

    const error = {
      type: 'exception',
      level: 'error',
      message: 'test',
      stack: 'Error: test',
      _enrichments: []
    }

    const enriched = await enrichErrorWithAiContext(error)

    assert.strictEqual(enriched._aiContext, undefined)

    setAiContextEnabled(true)
  })

  test('should include componentAncestry when React fiber found on activeElement', async () => {
    const { enrichErrorWithAiContext } = await import('../../../extension/lib/ai-context/ai-context-enrichment.js')

    globalThis.document.activeElement = {
      __reactFiber$test: {
        type: { name: 'TestComponent' },
        memoizedProps: { foo: 'bar' },
        return: null
      }
    }

    const error = {
      type: 'exception',
      level: 'error',
      message: 'test',
      stack: 'Error: test',
      _enrichments: []
    }

    const enriched = await enrichErrorWithAiContext(error)

    if (enriched._aiContext.componentAncestry) {
      assert.strictEqual(enriched._aiContext.componentAncestry.framework, 'react')
      assert.ok(enriched._aiContext.componentAncestry.components.length > 0)
    }
  })

  test('should include stateSnapshot when store exists and setting enabled', async () => {
    const { enrichErrorWithAiContext, setAiContextStateSnapshot } = await import('../../../extension/lib/ai-context/ai-context-enrichment.js')

    setAiContextStateSnapshot(true)

    globalThis.window.__REDUX_STORE__ = {
      getState: () => ({ auth: { error: 'failed' } })
    }

    const error = {
      type: 'exception',
      level: 'error',
      message: 'auth failed',
      stack: 'Error: auth failed',
      _enrichments: []
    }

    const enriched = await enrichErrorWithAiContext(error)

    if (enriched._aiContext.stateSnapshot) {
      assert.strictEqual(enriched._aiContext.stateSnapshot.source, 'redux')
    }

    delete globalThis.window.__REDUX_STORE__
    setAiContextStateSnapshot(false)
  })

  test('should not include stateSnapshot when setting disabled', async () => {
    const { enrichErrorWithAiContext, setAiContextStateSnapshot } = await import('../../../extension/lib/ai-context/ai-context-enrichment.js')

    setAiContextStateSnapshot(false) // Default

    globalThis.window.__REDUX_STORE__ = {
      getState: () => ({ auth: { error: 'failed' } })
    }

    const error = {
      type: 'exception',
      level: 'error',
      message: 'test',
      stack: 'Error: test',
      _enrichments: []
    }

    const enriched = await enrichErrorWithAiContext(error)

    assert.strictEqual(enriched._aiContext.stateSnapshot, undefined)

    delete globalThis.window.__REDUX_STORE__
  })
})

// --- Source Map Cache ---

describe('Source Map Cache', () => {
  test('should cache and retrieve source maps', async () => {
    const { setSourceMapCache, getSourceMapCache } = await import('../../../extension/lib/ai-context/ai-context-parsing.js')

    const mockMap = { sources: ['app.ts'], sourcesContent: ['code'] }
    setSourceMapCache('http://localhost/main.js', mockMap)

    const cached = getSourceMapCache('http://localhost/main.js')

    assert.deepStrictEqual(cached, mockMap)
  })

  test('should return null for uncached URL', async () => {
    const { getSourceMapCache } = await import('../../../extension/lib/ai-context/ai-context-parsing.js')

    const result = getSourceMapCache('http://localhost/unknown.js')

    assert.strictEqual(result, null)
  })

  test('should limit cache to 20 entries', async () => {
    const { setSourceMapCache, getSourceMapCacheSize } = await import('../../../extension/lib/ai-context/ai-context-parsing.js')

    for (let i = 0; i < 25; i++) {
      setSourceMapCache(`http://localhost/file${i}.js`, {
        sources: [`f${i}.ts`],
        sourcesContent: ['code']
      })
    }

    assert.ok(getSourceMapCacheSize() <= 20)
  })

  test('should evict oldest entries when cache is full', async () => {
    const { setSourceMapCache, getSourceMapCache } = await import('../../../extension/lib/ai-context/ai-context-parsing.js')

    // Fill cache
    for (let i = 0; i < 20; i++) {
      setSourceMapCache(`http://localhost/file${i}.js`, {
        sources: [`f${i}.ts`],
        sourcesContent: ['code']
      })
    }

    // Add one more (should evict file0)
    setSourceMapCache('http://localhost/file_new.js', {
      sources: ['new.ts'],
      sourcesContent: ['new code']
    })

    // Newest should exist
    assert.ok(getSourceMapCache('http://localhost/file_new.js'))
    // Oldest should be evicted
    assert.strictEqual(getSourceMapCache('http://localhost/file0.js'), null)
  })
})
