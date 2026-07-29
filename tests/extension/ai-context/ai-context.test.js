// @ts-nocheck
/**
 * @fileoverview AI-context stack parsing, source-map parsing, and source-snippet tests.
 */

import { test, describe, beforeEach, afterEach } from 'node:test'
import assert from 'node:assert'
import { createMockWindow } from './ai-context-fixture.js'

let originalWindow

describe('Stack Frame Parsing', () => {
  test('should parse Chrome-style stack frames', async () => {
    const { parseStackFrames } = await import('../../../extension/lib/ai-context/ai-context-parsing.js')

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
    const { parseStackFrames } = await import('../../../extension/lib/ai-context/ai-context-parsing.js')

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
    const { parseStackFrames } = await import('../../../extension/lib/ai-context/ai-context-parsing.js')

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
    const { parseStackFrames } = await import('../../../extension/lib/ai-context/ai-context-parsing.js')

    assert.deepStrictEqual(parseStackFrames(''), [])
  })

  test('should return empty array for null stack', async () => {
    const { parseStackFrames } = await import('../../../extension/lib/ai-context/ai-context-parsing.js')

    assert.deepStrictEqual(parseStackFrames(null), [])
  })

  test('should return empty array for undefined stack', async () => {
    const { parseStackFrames } = await import('../../../extension/lib/ai-context/ai-context-parsing.js')

    assert.deepStrictEqual(parseStackFrames(undefined), [])
  })

  test('should handle eval frames', async () => {
    const { parseStackFrames } = await import('../../../extension/lib/ai-context/ai-context-parsing.js')

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
    const { parseSourceMap } = await import('../../../extension/lib/ai-context/ai-context-parsing.js')

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
    const { parseSourceMap } = await import('../../../extension/lib/ai-context/ai-context-parsing.js')

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
    const { parseSourceMap } = await import('../../../extension/lib/ai-context/ai-context-parsing.js')

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
    const { parseSourceMap } = await import('../../../extension/lib/ai-context/ai-context-parsing.js')

    const result = parseSourceMap('data:application/json;base64,!!!invalid!!!')

    assert.strictEqual(result, null)
  })

  test('should return null for non-data-url string', async () => {
    const { parseSourceMap } = await import('../../../extension/lib/ai-context/ai-context-parsing.js')

    const result = parseSourceMap('https://example.com/app.js.map')

    assert.strictEqual(result, null)
  })

  test('should return null for empty string', async () => {
    const { parseSourceMap } = await import('../../../extension/lib/ai-context/ai-context-parsing.js')

    assert.strictEqual(parseSourceMap(''), null)
  })

  test('should return null for null input', async () => {
    const { parseSourceMap } = await import('../../../extension/lib/ai-context/ai-context-parsing.js')

    assert.strictEqual(parseSourceMap(null), null)
  })
})

// --- Source Snippet Extraction ---

describe('Source Snippet Extraction', () => {
  test('should extract snippet with 5 lines before and after', async () => {
    const { extractSnippet } = await import('../../../extension/lib/ai-context/ai-context-parsing.js')

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
    const { extractSnippet } = await import('../../../extension/lib/ai-context/ai-context-parsing.js')

    const sourceContent = 'line 1\nline 2\nline 3\nline 4\nline 5\nline 6'

    const snippet = extractSnippet(sourceContent, 1)

    assert.ok(snippet)
    assert.strictEqual(snippet[0].line, 1)
    assert.strictEqual(snippet[0].isError, true)
    assert.ok(snippet.length <= 6)
  })

  test('should handle error on last line', async () => {
    const { extractSnippet } = await import('../../../extension/lib/ai-context/ai-context-parsing.js')

    const sourceContent = 'line 1\nline 2\nline 3\nline 4\nline 5'

    const snippet = extractSnippet(sourceContent, 5)

    assert.ok(snippet)
    const errorLine = snippet.find((s) => s.isError)
    assert.strictEqual(errorLine.line, 5)
    assert.strictEqual(errorLine.text, 'line 5')
  })

  test('should truncate lines longer than 200 chars', async () => {
    const { extractSnippet } = await import('../../../extension/lib/ai-context/ai-context-parsing.js')

    const longLine = 'x'.repeat(300)
    const sourceContent = `line 1\n${longLine}\nline 3`

    const snippet = extractSnippet(sourceContent, 2)

    const errorLine = snippet.find((s) => s.isError)
    assert.ok(errorLine.text.length <= 200)
  })

  test('should return null for line number out of range', async () => {
    const { extractSnippet } = await import('../../../extension/lib/ai-context/ai-context-parsing.js')

    const sourceContent = 'line 1\nline 2\nline 3'

    assert.strictEqual(extractSnippet(sourceContent, 100), null)
  })

  test('should return null for line 0', async () => {
    const { extractSnippet } = await import('../../../extension/lib/ai-context/ai-context-parsing.js')

    assert.strictEqual(extractSnippet('line 1', 0), null)
  })

  test('should return null for negative line', async () => {
    const { extractSnippet } = await import('../../../extension/lib/ai-context/ai-context-parsing.js')

    assert.strictEqual(extractSnippet('line 1', -1), null)
  })

  test('should return null for empty source content', async () => {
    const { extractSnippet } = await import('../../../extension/lib/ai-context/ai-context-parsing.js')

    assert.strictEqual(extractSnippet('', 1), null)
  })

  test('should return null for null source content', async () => {
    const { extractSnippet } = await import('../../../extension/lib/ai-context/ai-context-parsing.js')

    assert.strictEqual(extractSnippet(null, 1), null)
  })

  test('should mark only the error line with isError', async () => {
    const { extractSnippet } = await import('../../../extension/lib/ai-context/ai-context-parsing.js')

    const sourceContent = Array.from({ length: 20 }, (_, i) => `line ${i + 1}`).join('\n')

    const snippet = extractSnippet(sourceContent, 10)

    const errorLines = snippet.filter((s) => s.isError)
    assert.strictEqual(errorLines.length, 1)
    assert.strictEqual(errorLines[0].line, 10)
  })

  test('should only process top 3 stack frames', async () => {
    const { extractSourceSnippets } = await import('../../../extension/lib/ai-context/ai-context-parsing.js')

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
    const { extractSourceSnippets } = await import('../../../extension/lib/ai-context/ai-context-parsing.js')

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
