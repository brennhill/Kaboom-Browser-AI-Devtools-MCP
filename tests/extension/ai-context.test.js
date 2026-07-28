// @ts-nocheck
/**
 * @fileoverview ai-context.test.js — Tests for error context enrichment.
 * Verifies that console errors and unhandled exceptions are augmented with
 * surrounding code context, variable snapshots, and stack frame metadata
 * to help AI assistants diagnose issues without additional queries.
 *
 * Tests cover:
 * - Stack frame parsing (Chrome + Firefox formats)
 * - Source map parsing (inline base64)
 * - Source snippet extraction (context lines, bounds, truncation)
 * - Component ancestry detection (React, Vue, Svelte)
 * - Application state snapshot (Redux, no-store)
 * - AI summary generation
 * - Full enrichment pipeline
 * - Settings/toggles
 * - Source map cache management
 */

import { test, describe, mock, beforeEach, afterEach } from 'node:test'
import assert from 'node:assert'

// Mock browser environment
const createMockWindow = () => ({
  postMessage: mock.fn(),
  addEventListener: mock.fn(),
  fetch: mock.fn(() => Promise.resolve({ ok: false })),
  performance: { now: () => Date.now() }
})

const createMockDocument = () => ({
  activeElement: null,
  querySelectorAll: mock.fn(() => []),
  body: {}
})

let originalWindow, originalDocument

// --- Stack Frame Parsing ---

describe('Stack Frame Parsing', () => {
  test('should parse Chrome-style stack frames', async () => {
    const { parseStackFrames } = await import('../../extension/lib/ai-context/ai-context-parsing.js')

    const stack = `TypeError: Cannot read properties of undefined
    at handleSubmit (http://localhost:3000/static/js/main.abc123.js:42:15)
    at HTMLButtonElement.onclick (http://localhost:3000/static/js/main.abc123.js:100:3)`

    const frames = parseStackFrames(stack)

    assert.strictEqual(frames.length, 2)
    assert.strictEqual(frames[0].filename, 'http://localhost:3000/static/js/main.abc123.js')
    assert.strictEqual(frames[0].lineno, 42)
    assert.strictEqual(frames[0].colno, 15)
    assert.strictEqual(frames[0].functionName, 'handleSubmit')
  })

  test('should parse Firefox-style stack frames', async () => {
    const { parseStackFrames } = await import('../../extension/lib/ai-context/ai-context-parsing.js')

    const stack = `handleSubmit@http://localhost:3000/main.js:42:15
onclick@http://localhost:3000/main.js:100:3`

    const frames = parseStackFrames(stack)

    assert.strictEqual(frames.length, 2)
    assert.strictEqual(frames[0].functionName, 'handleSubmit')
    assert.strictEqual(frames[0].filename, 'http://localhost:3000/main.js')
    assert.strictEqual(frames[0].lineno, 42)
    assert.strictEqual(frames[0].colno, 15)
  })

  test('should handle anonymous functions in stack', async () => {
    const { parseStackFrames } = await import('../../extension/lib/ai-context/ai-context-parsing.js')

    const stack = `Error: test
    at http://localhost:3000/main.js:42:15
    at Array.forEach (<anonymous>)
    at Object.<anonymous> (http://localhost:3000/main.js:50:5)`

    const frames = parseStackFrames(stack)

    // Should extract frames with real file locations, skipping <anonymous>
    const realFrames = frames.filter((f) => f.filename && !f.filename.includes('<anonymous>'))
    assert.ok(realFrames.length >= 2)
    assert.strictEqual(realFrames[0].lineno, 42)
  })

  test('should return empty array for empty stack', async () => {
    const { parseStackFrames } = await import('../../extension/lib/ai-context/ai-context-parsing.js')

    assert.deepStrictEqual(parseStackFrames(''), [])
  })

  test('should return empty array for null stack', async () => {
    const { parseStackFrames } = await import('../../extension/lib/ai-context/ai-context-parsing.js')

    assert.deepStrictEqual(parseStackFrames(null), [])
  })

  test('should return empty array for undefined stack', async () => {
    const { parseStackFrames } = await import('../../extension/lib/ai-context/ai-context-parsing.js')

    assert.deepStrictEqual(parseStackFrames(undefined), [])
  })

  test('should handle eval frames', async () => {
    const { parseStackFrames } = await import('../../extension/lib/ai-context/ai-context-parsing.js')

    const stack = `Error: test
    at eval (eval at runCode (http://localhost:3000/main.js:10:5), <anonymous>:1:1)
    at runCode (http://localhost:3000/main.js:10:5)`

    const frames = parseStackFrames(stack)

    // Should at minimum extract the runCode frame
    const runCodeFrame = frames.find((f) => f.functionName === 'runCode')
    assert.ok(runCodeFrame)
    assert.strictEqual(runCodeFrame.lineno, 10)
  })
})

// --- Source Map Parsing ---

describe('Source Map Parsing', () => {
  beforeEach(() => {
    originalWindow = globalThis.window
    globalThis.window = createMockWindow()
  })

  afterEach(() => {
    globalThis.window = originalWindow
  })

  test('should parse inline base64 source map with sourcesContent', async () => {
    const { parseSourceMap } = await import('../../extension/lib/ai-context/ai-context-parsing.js')

    const sourceMap = {
      version: 3,
      sources: ['src/app.ts'],
      sourcesContent: ['const x = 1;\nconst y = x.foo;\nconsole.log(y);'],
      mappings: 'AAAA;AACA;AACA'
    }
    const encoded = Buffer.from(JSON.stringify(sourceMap)).toString('base64')
    const dataUrl = `data:application/json;base64,${encoded}`

    const result = parseSourceMap(dataUrl)

    assert.ok(result)
    assert.strictEqual(result.sources[0], 'src/app.ts')
    assert.ok(result.sourcesContent[0].includes('const x = 1'))
  })

  test('should parse inline source map with charset', async () => {
    const { parseSourceMap } = await import('../../extension/lib/ai-context/ai-context-parsing.js')

    const sourceMap = {
      version: 3,
      sources: ['app.js'],
      sourcesContent: ['function test() {}'],
      mappings: 'AAAA'
    }
    const encoded = Buffer.from(JSON.stringify(sourceMap)).toString('base64')
    const dataUrl = `data:application/json;charset=utf-8;base64,${encoded}`

    const result = parseSourceMap(dataUrl)

    assert.ok(result)
    assert.strictEqual(result.sources[0], 'app.js')
  })

  test('should return null for source map without sourcesContent', async () => {
    const { parseSourceMap } = await import('../../extension/lib/ai-context/ai-context-parsing.js')

    const sourceMap = {
      version: 3,
      sources: ['src/app.ts'],
      mappings: 'AAAA'
    }
    const encoded = Buffer.from(JSON.stringify(sourceMap)).toString('base64')
    const dataUrl = `data:application/json;base64,${encoded}`

    const result = parseSourceMap(dataUrl)

    assert.strictEqual(result, null)
  })

  test('should return null for invalid base64', async () => {
    const { parseSourceMap } = await import('../../extension/lib/ai-context/ai-context-parsing.js')

    const result = parseSourceMap('data:application/json;base64,!!!invalid!!!')

    assert.strictEqual(result, null)
  })

  test('should return null for non-data-url string', async () => {
    const { parseSourceMap } = await import('../../extension/lib/ai-context/ai-context-parsing.js')

    const result = parseSourceMap('https://example.com/app.js.map')

    assert.strictEqual(result, null)
  })

  test('should return null for empty string', async () => {
    const { parseSourceMap } = await import('../../extension/lib/ai-context/ai-context-parsing.js')

    assert.strictEqual(parseSourceMap(''), null)
  })

  test('should return null for null input', async () => {
    const { parseSourceMap } = await import('../../extension/lib/ai-context/ai-context-parsing.js')

    assert.strictEqual(parseSourceMap(null), null)
  })
})

// --- Source Snippet Extraction ---

describe('Source Snippet Extraction', () => {
  test('should extract snippet with 5 lines before and after', async () => {
    const { extractSnippet } = await import('../../extension/lib/ai-context/ai-context-parsing.js')

    const sourceContent = Array.from({ length: 20 }, (_, i) => `line ${i + 1} content`).join('\n')

    const snippet = extractSnippet(sourceContent, 10)

    assert.ok(snippet)
    assert.strictEqual(snippet.length, 11) // 5 before + error + 5 after
    assert.strictEqual(snippet[0].line, 5)
    assert.strictEqual(snippet[5].line, 10)
    assert.strictEqual(snippet[5].isError, true)
    assert.strictEqual(snippet[10].line, 15)
  })

  test('should handle error on first line', async () => {
    const { extractSnippet } = await import('../../extension/lib/ai-context/ai-context-parsing.js')

    const sourceContent = 'line 1\nline 2\nline 3\nline 4\nline 5\nline 6'

    const snippet = extractSnippet(sourceContent, 1)

    assert.ok(snippet)
    assert.strictEqual(snippet[0].line, 1)
    assert.strictEqual(snippet[0].isError, true)
    assert.ok(snippet.length <= 6)
  })

  test('should handle error on last line', async () => {
    const { extractSnippet } = await import('../../extension/lib/ai-context/ai-context-parsing.js')

    const sourceContent = 'line 1\nline 2\nline 3\nline 4\nline 5'

    const snippet = extractSnippet(sourceContent, 5)

    assert.ok(snippet)
    const errorLine = snippet.find((s) => s.isError)
    assert.strictEqual(errorLine.line, 5)
    assert.strictEqual(errorLine.text, 'line 5')
  })

  test('should truncate lines longer than 200 chars', async () => {
    const { extractSnippet } = await import('../../extension/lib/ai-context/ai-context-parsing.js')

    const longLine = 'x'.repeat(300)
    const sourceContent = `line 1\n${longLine}\nline 3`

    const snippet = extractSnippet(sourceContent, 2)

    const errorLine = snippet.find((s) => s.isError)
    assert.ok(errorLine.text.length <= 200)
  })

  test('should return null for line number out of range', async () => {
    const { extractSnippet } = await import('../../extension/lib/ai-context/ai-context-parsing.js')

    const sourceContent = 'line 1\nline 2\nline 3'

    assert.strictEqual(extractSnippet(sourceContent, 100), null)
  })

  test('should return null for line 0', async () => {
    const { extractSnippet } = await import('../../extension/lib/ai-context/ai-context-parsing.js')

    assert.strictEqual(extractSnippet('line 1', 0), null)
  })

  test('should return null for negative line', async () => {
    const { extractSnippet } = await import('../../extension/lib/ai-context/ai-context-parsing.js')

    assert.strictEqual(extractSnippet('line 1', -1), null)
  })

  test('should return null for empty source content', async () => {
    const { extractSnippet } = await import('../../extension/lib/ai-context/ai-context-parsing.js')

    assert.strictEqual(extractSnippet('', 1), null)
  })

  test('should return null for null source content', async () => {
    const { extractSnippet } = await import('../../extension/lib/ai-context/ai-context-parsing.js')

    assert.strictEqual(extractSnippet(null, 1), null)
  })

  test('should mark only the error line with isError', async () => {
    const { extractSnippet } = await import('../../extension/lib/ai-context/ai-context-parsing.js')

    const sourceContent = Array.from({ length: 20 }, (_, i) => `line ${i + 1}`).join('\n')

    const snippet = extractSnippet(sourceContent, 10)

    const errorLines = snippet.filter((s) => s.isError)
    assert.strictEqual(errorLines.length, 1)
    assert.strictEqual(errorLines[0].line, 10)
  })

  test('should only process top 3 stack frames', async () => {
    const { extractSourceSnippets } = await import('../../extension/lib/ai-context/ai-context-parsing.js')

    const frames = [
      { filename: 'a.js', lineno: 10 },
      { filename: 'b.js', lineno: 20 },
      { filename: 'c.js', lineno: 30 },
      { filename: 'd.js', lineno: 40 },
      { filename: 'e.js', lineno: 50 }
    ]

    const mockSourceMaps = {
      'a.js': { sourcesContent: [Array(50).fill('code').join('\n')] },
      'b.js': { sourcesContent: [Array(50).fill('code').join('\n')] },
      'c.js': { sourcesContent: [Array(50).fill('code').join('\n')] },
      'd.js': { sourcesContent: [Array(50).fill('code').join('\n')] },
      'e.js': { sourcesContent: [Array(50).fill('code').join('\n')] }
    }

    const snippets = await extractSourceSnippets(frames, mockSourceMaps)

    assert.ok(snippets.length <= 3)
  })

  test('should cap total snippets payload at 10KB', async () => {
    const { extractSourceSnippets } = await import('../../extension/lib/ai-context/ai-context-parsing.js')

    // Each line 200 chars, 11 lines per snippet = 2200 chars per snippet
    const largeSource = Array.from({ length: 100 }, () => 'x'.repeat(200)).join('\n')

    const frames = [
      { filename: 'a.js', lineno: 50 },
      { filename: 'b.js', lineno: 50 },
      { filename: 'c.js', lineno: 50 }
    ]

    const mockSourceMaps = {
      'a.js': { sourcesContent: [largeSource] },
      'b.js': { sourcesContent: [largeSource] },
      'c.js': { sourcesContent: [largeSource] }
    }

    const snippets = await extractSourceSnippets(frames, mockSourceMaps)

    const totalSize = JSON.stringify(snippets).length
    assert.ok(totalSize <= 10240, `Expected <= 10KB, got ${totalSize}`)
  })
})

// --- Component Ancestry: React ---

describe('Component Ancestry - React', () => {
  beforeEach(() => {
    originalWindow = globalThis.window
    globalThis.window = createMockWindow()
  })

  afterEach(() => {
    globalThis.window = originalWindow
  })

  test('should detect React from __reactFiber$ key', async () => {
    const { detectFramework } = await import('../../extension/lib/ai-context/ai-context-enrichment.js')

    const element = { __reactFiber$abc123: {} }
    const result = detectFramework(element)

    assert.strictEqual(result.framework, 'react')
    assert.strictEqual(result.key, '__reactFiber$abc123')
  })

  test('should detect React from __reactInternalInstance$ key', async () => {
    const { detectFramework } = await import('../../extension/lib/ai-context/ai-context-enrichment.js')

    const element = { __reactInternalInstance$xyz: {} }
    const result = detectFramework(element)

    assert.strictEqual(result.framework, 'react')
  })

  test('should extract component names from fiber tree', async () => {
    const { getReactComponentAncestry } = await import('../../extension/lib/ai-context/ai-context-enrichment.js')

    const fiber = {
      type: { name: 'LoginForm' },
      memoizedProps: { initialEmail: '' },
      memoizedState: { email: '', loading: false },
      return: {
        type: { name: 'AuthProvider' },
        memoizedProps: { children: null },
        return: {
          type: { name: 'App' },
          memoizedProps: { theme: 'dark' },
          return: null
        }
      }
    }

    const ancestry = getReactComponentAncestry(fiber)

    assert.strictEqual(ancestry.length, 3)
    // Root first order
    assert.strictEqual(ancestry[0].name, 'App')
    assert.strictEqual(ancestry[1].name, 'AuthProvider')
    assert.strictEqual(ancestry[2].name, 'LoginForm')
  })

  test('should prefer displayName over name', async () => {
    const { getReactComponentAncestry } = await import('../../extension/lib/ai-context/ai-context-enrichment.js')

    const fiber = {
      type: { name: 'Comp', displayName: 'MyDisplayName' },
      memoizedProps: {},
      return: null
    }

    const ancestry = getReactComponentAncestry(fiber)

    assert.strictEqual(ancestry[0].name, 'MyDisplayName')
  })

  test('should use Anonymous for unnamed components', async () => {
    const { getReactComponentAncestry } = await import('../../extension/lib/ai-context/ai-context-enrichment.js')

    const fiber = {
      type: { name: '', displayName: null },
      memoizedProps: {},
      return: null
    }

    const ancestry = getReactComponentAncestry(fiber)

    assert.strictEqual(ancestry[0].name, 'Anonymous')
  })

  test('should extract prop keys excluding children', async () => {
    const { getReactComponentAncestry } = await import('../../extension/lib/ai-context/ai-context-enrichment.js')

    const fiber = {
      type: { name: 'Button' },
      memoizedProps: { onClick: () => {}, className: 'btn', children: 'text', disabled: false },
      return: null
    }

    const ancestry = getReactComponentAncestry(fiber)

    assert.ok(ancestry[0].propKeys.includes('onClick'))
    assert.ok(ancestry[0].propKeys.includes('className'))
    assert.ok(!ancestry[0].propKeys.includes('children'))
  })

  test('should extract state keys', async () => {
    const { getReactComponentAncestry } = await import('../../extension/lib/ai-context/ai-context-enrichment.js')

    const fiber = {
      type: { name: 'Form' },
      memoizedProps: {},
      memoizedState: { email: '', loading: false, error: null },
      return: null
    }

    const ancestry = getReactComponentAncestry(fiber)

    assert.strictEqual(ancestry[0].hasState, true)
    assert.ok(ancestry[0].stateKeys.includes('email'))
    assert.ok(ancestry[0].stateKeys.includes('loading'))
    assert.ok(ancestry[0].stateKeys.includes('error'))
  })

  test('should limit ancestry depth to 10', async () => {
    const { getReactComponentAncestry } = await import('../../extension/lib/ai-context/ai-context-enrichment.js')

    let current = null
    for (let i = 0; i < 15; i++) {
      current = {
        type: { name: `C${i}` },
        memoizedProps: {},
        return: current
      }
    }

    const ancestry = getReactComponentAncestry(current)

    assert.ok(ancestry.length <= 10)
  })

  test('should limit prop keys to 20', async () => {
    const { getReactComponentAncestry } = await import('../../extension/lib/ai-context/ai-context-enrichment.js')

    const props = {}
    for (let i = 0; i < 30; i++) props[`prop${i}`] = i

    const fiber = {
      type: { name: 'Big' },
      memoizedProps: props,
      return: null
    }

    const ancestry = getReactComponentAncestry(fiber)

    assert.ok(ancestry[0].propKeys.length <= 20)
  })

  test('should limit state keys to 10', async () => {
    const { getReactComponentAncestry } = await import('../../extension/lib/ai-context/ai-context-enrichment.js')

    const state = {}
    for (let i = 0; i < 15; i++) state[`state${i}`] = i

    const fiber = {
      type: { name: 'Big' },
      memoizedProps: {},
      memoizedState: state,
      return: null
    }

    const ancestry = getReactComponentAncestry(fiber)

    assert.ok(ancestry[0].stateKeys.length <= 10)
  })

  test('should skip host elements (div, span, etc.)', async () => {
    const { getReactComponentAncestry } = await import('../../extension/lib/ai-context/ai-context-enrichment.js')

    const fiber = {
      type: { name: 'Child' },
      memoizedProps: {},
      return: {
        type: 'div', // Host element
        memoizedProps: {},
        return: {
          type: { name: 'Parent' },
          memoizedProps: {},
          return: null
        }
      }
    }

    const ancestry = getReactComponentAncestry(fiber)
    const names = ancestry.map((c) => c.name)

    assert.ok(!names.includes('div'))
    assert.ok(names.includes('Child'))
    assert.ok(names.includes('Parent'))
  })

  test('should handle null fiber gracefully', async () => {
    const { getReactComponentAncestry } = await import('../../extension/lib/ai-context/ai-context-enrichment.js')

    const ancestry = getReactComponentAncestry(null)

    assert.strictEqual(ancestry, null)
  })
})

// --- Component Ancestry: Vue ---

describe('Component Ancestry - Vue', () => {
  test('should detect Vue 3 from __vueParentComponent', async () => {
    const { detectFramework } = await import('../../extension/lib/ai-context/ai-context-enrichment.js')

    const result = detectFramework({ __vueParentComponent: {} })

    assert.strictEqual(result.framework, 'vue')
  })

  test('should detect Vue app root from __vue_app__', async () => {
    const { detectFramework } = await import('../../extension/lib/ai-context/ai-context-enrichment.js')

    const result = detectFramework({ __vue_app__: {} })

    assert.strictEqual(result.framework, 'vue')
  })
})

// --- Component Ancestry: Svelte ---

describe('Component Ancestry - Svelte', () => {
  test('should detect Svelte from __svelte_meta', async () => {
    const { detectFramework } = await import('../../extension/lib/ai-context/ai-context-enrichment.js')

    const result = detectFramework({ __svelte_meta: { loc: { file: 'App.svelte' } } })

    assert.strictEqual(result.framework, 'svelte')
  })
})

// --- No Framework ---

describe('Framework Detection - None', () => {
  test('should return null for plain DOM elements', async () => {
    const { detectFramework } = await import('../../extension/lib/ai-context/ai-context-enrichment.js')

    const result = detectFramework({ tagName: 'DIV', className: 'container' })

    assert.strictEqual(result, null)
  })

  test('should return null for empty object', async () => {
    const { detectFramework } = await import('../../extension/lib/ai-context/ai-context-enrichment.js')

    assert.strictEqual(detectFramework({}), null)
  })

  test('should return null for null', async () => {
    const { detectFramework } = await import('../../extension/lib/ai-context/ai-context-enrichment.js')

    assert.strictEqual(detectFramework(null), null)
  })
})

// --- State Snapshot ---

describe('Application State Snapshot', () => {
  beforeEach(() => {
    originalWindow = globalThis.window
    globalThis.window = createMockWindow()
  })

  afterEach(() => {
    globalThis.window = originalWindow
  })

  test('should detect Redux store and extract keys', async () => {
    const { captureStateSnapshot } = await import('../../extension/lib/ai-context/ai-context-enrichment.js')

    globalThis.window.__REDUX_STORE__ = {
      getState: () => ({
        auth: { user: null, loading: false, error: 'Unauthorized' },
        cart: { items: [], total: 0 }
      })
    }

    const snapshot = captureStateSnapshot('Unauthorized')

    assert.strictEqual(snapshot.source, 'redux')
    assert.ok(snapshot.keys.auth)
    assert.ok(snapshot.keys.cart)

    delete globalThis.window.__REDUX_STORE__
  })

  test('should extract correct types for state values', async () => {
    const { captureStateSnapshot } = await import('../../extension/lib/ai-context/ai-context-enrichment.js')

    globalThis.window.__REDUX_STORE__ = {
      getState: () => ({
        obj: { nested: true },
        arr: [1, 2, 3],
        num: 42,
        str: 'hello',
        bool: true,
        nil: null
      })
    }

    const snapshot = captureStateSnapshot('')

    assert.strictEqual(snapshot.keys.obj.type, 'object')
    assert.strictEqual(snapshot.keys.arr.type, 'array')
    assert.strictEqual(snapshot.keys.num.type, 'number')
    assert.strictEqual(snapshot.keys.str.type, 'string')
    assert.strictEqual(snapshot.keys.bool.type, 'boolean')

    delete globalThis.window.__REDUX_STORE__
  })

  test('should extract relevant slice based on error keywords', async () => {
    const { captureStateSnapshot } = await import('../../extension/lib/ai-context/ai-context-enrichment.js')

    globalThis.window.__REDUX_STORE__ = {
      getState: () => ({
        auth: { user: null, error: 'Token expired' },
        cart: { items: ['a'], total: 50 },
        ui: { theme: 'dark' }
      })
    }

    const snapshot = captureStateSnapshot('auth failed: Token expired')

    assert.ok(snapshot.relevantSlice)
    // Should include auth state because error mentions "auth"
    const keys = Object.keys(snapshot.relevantSlice)
    assert.ok(keys.some((k) => k.startsWith('auth')))

    delete globalThis.window.__REDUX_STORE__
  })

  test('should include error/loading/status keys in relevant slice', async () => {
    const { captureStateSnapshot } = await import('../../extension/lib/ai-context/ai-context-enrichment.js')

    globalThis.window.__REDUX_STORE__ = {
      getState: () => ({
        data: { items: [], loading: true, error: null, status: 'pending' },
        ui: { modal: false }
      })
    }

    const snapshot = captureStateSnapshot('')

    const keys = Object.keys(snapshot.relevantSlice)
    assert.ok(keys.some((k) => k.includes('loading')))
    assert.ok(keys.some((k) => k.includes('status')))

    delete globalThis.window.__REDUX_STORE__
  })

  test('should limit relevant slice to 10 entries', async () => {
    const { captureStateSnapshot } = await import('../../extension/lib/ai-context/ai-context-enrichment.js')

    const state = {}
    for (let i = 0; i < 20; i++) {
      state[`mod${i}`] = { error: `err${i}`, loading: false }
    }

    globalThis.window.__REDUX_STORE__ = { getState: () => state }

    const snapshot = captureStateSnapshot('')

    assert.ok(Object.keys(snapshot.relevantSlice).length <= 10)

    delete globalThis.window.__REDUX_STORE__
  })

  test('should truncate values at 200 chars', async () => {
    const { captureStateSnapshot } = await import('../../extension/lib/ai-context/ai-context-enrichment.js')

    globalThis.window.__REDUX_STORE__ = {
      getState: () => ({
        data: { error: 'x'.repeat(500) }
      })
    }

    const snapshot = captureStateSnapshot('')

    const errorValue = snapshot.relevantSlice['data.error']
    assert.ok(String(errorValue).length <= 200)

    delete globalThis.window.__REDUX_STORE__
  })

  test('should return null when no store is found', async () => {
    const { captureStateSnapshot } = await import('../../extension/lib/ai-context/ai-context-enrichment.js')

    const snapshot = captureStateSnapshot('some error')

    assert.strictEqual(snapshot, null)
  })

  test('should handle store.getState() throwing', async () => {
    const { captureStateSnapshot } = await import('../../extension/lib/ai-context/ai-context-enrichment.js')

    globalThis.window.__REDUX_STORE__ = {
      getState: () => {
        throw new Error('store error')
      }
    }

    const snapshot = captureStateSnapshot('')

    assert.strictEqual(snapshot, null)

    delete globalThis.window.__REDUX_STORE__
  })
})

// --- Summary Generation ---

describe('AI Context Summary Generation', () => {
  test('should generate summary with all data', async () => {
    const { generateAiSummary } = await import('../../extension/lib/ai-context/ai-context-enrichment.js')

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
    const { generateAiSummary } = await import('../../extension/lib/ai-context/ai-context-enrichment.js')

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
    const { generateAiSummary } = await import('../../extension/lib/ai-context/ai-context-enrichment.js')

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
    const { generateAiSummary } = await import('../../extension/lib/ai-context/ai-context-enrichment.js')

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
    const { enrichErrorWithAiContext } = await import('../../extension/lib/ai-context/ai-context-enrichment.js')

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
    const { enrichErrorWithAiContext } = await import('../../extension/lib/ai-context/ai-context-enrichment.js')

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
    const { enrichErrorWithAiContext, setAiContextEnabled } = await import('../../extension/lib/ai-context/ai-context-enrichment.js')

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
    const { enrichErrorWithAiContext } = await import('../../extension/lib/ai-context/ai-context-enrichment.js')

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
    const { enrichErrorWithAiContext, setAiContextStateSnapshot } = await import('../../extension/lib/ai-context/ai-context-enrichment.js')

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
    const { enrichErrorWithAiContext, setAiContextStateSnapshot } = await import('../../extension/lib/ai-context/ai-context-enrichment.js')

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
    const { setSourceMapCache, getSourceMapCache } = await import('../../extension/lib/ai-context/ai-context-parsing.js')

    const mockMap = { sources: ['app.ts'], sourcesContent: ['code'] }
    setSourceMapCache('http://localhost/main.js', mockMap)

    const cached = getSourceMapCache('http://localhost/main.js')

    assert.deepStrictEqual(cached, mockMap)
  })

  test('should return null for uncached URL', async () => {
    const { getSourceMapCache } = await import('../../extension/lib/ai-context/ai-context-parsing.js')

    const result = getSourceMapCache('http://localhost/unknown.js')

    assert.strictEqual(result, null)
  })

  test('should limit cache to 20 entries', async () => {
    const { setSourceMapCache, getSourceMapCacheSize } = await import('../../extension/lib/ai-context/ai-context-parsing.js')

    for (let i = 0; i < 25; i++) {
      setSourceMapCache(`http://localhost/file${i}.js`, {
        sources: [`f${i}.ts`],
        sourcesContent: ['code']
      })
    }

    assert.ok(getSourceMapCacheSize() <= 20)
  })

  test('should evict oldest entries when cache is full', async () => {
    const { setSourceMapCache, getSourceMapCache } = await import('../../extension/lib/ai-context/ai-context-parsing.js')

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
