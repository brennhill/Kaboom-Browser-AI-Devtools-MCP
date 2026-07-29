// @ts-nocheck
/**
 * @fileoverview ai-context-parsing.test.js — Unit tests for stack trace parsing,
 * source map resolution, code snippet extraction, and source map cache.
 */

import { describe, test, beforeEach } from 'node:test'
import assert from 'node:assert'
import { readFileSync } from 'node:fs'

import {
  parseStackFrames,
  parseSourceMap,
  extractSnippet,
  extractSourceSnippets,
  setSourceMapCache,
  getSourceMapCache,
  getSourceMapCacheSize,
  resetParsingForTesting
} from '../../../extension/lib/ai-context/ai-context-parsing.js'

// =============================================================================
// STACK FRAME PARSING
// =============================================================================

describe('parseStackFrames — Chrome format', () => {
  test('parses at functionName (file:line:col) format', () => {
    const stack = `TypeError: boom
    at handleClick (http://localhost:3000/app.js:42:15)
    at HTMLButtonElement.onclick (http://localhost:3000/app.js:100:3)`

    const frames = parseStackFrames(stack)

    assert.strictEqual(frames.length, 2)
    assert.strictEqual(frames[0].functionName, 'handleClick')
    assert.strictEqual(frames[0].filename, 'http://localhost:3000/app.js')
    assert.strictEqual(frames[0].lineno, 42)
    assert.strictEqual(frames[0].colno, 15)
  })

  test('parses anonymous functions (no function name)', () => {
    const stack = `Error: oops
    at http://localhost:3000/main.js:10:5`

    const frames = parseStackFrames(stack)

    assert.strictEqual(frames.length, 1)
    assert.strictEqual(frames[0].functionName, null)
    assert.strictEqual(frames[0].lineno, 10)
  })

  test('skips <anonymous> frames', () => {
    const stack = `Error: x
    at Array.forEach (<anonymous>)
    at run (http://localhost:3000/app.js:5:1)`

    const frames = parseStackFrames(stack)

    assert.strictEqual(frames.length, 1)
    assert.strictEqual(frames[0].functionName, 'run')
  })

  test('parses eval frames — extracts real frame after eval', () => {
    const stack = `Error: eval
    at eval (eval at runCode (http://localhost:3000/main.js:10:5), <anonymous>:1:1)
    at runCode (http://localhost:3000/main.js:10:5)`

    const frames = parseStackFrames(stack)

    const runFrame = frames.find((f) => f.functionName === 'runCode')
    assert.ok(runFrame)
    assert.strictEqual(runFrame.lineno, 10)
  })
})

describe('parseStackFrames — Firefox format', () => {
  test('parses functionName@file:line:col format', () => {
    const stack = `submitForm@http://localhost:3000/app.js:25:8
handleEvent@http://localhost:3000/app.js:50:3`

    const frames = parseStackFrames(stack)

    assert.strictEqual(frames.length, 2)
    assert.strictEqual(frames[0].functionName, 'submitForm')
    assert.strictEqual(frames[0].filename, 'http://localhost:3000/app.js')
    assert.strictEqual(frames[0].lineno, 25)
    assert.strictEqual(frames[0].colno, 8)
  })
})

describe('parseStackFrames — edge cases', () => {
  test('returns empty array for empty string', () => {
    assert.deepStrictEqual(parseStackFrames(''), [])
  })

  test('returns empty array for undefined', () => {
    assert.deepStrictEqual(parseStackFrames(undefined), [])
  })

  test('returns empty array for malformed stack', () => {
    assert.deepStrictEqual(parseStackFrames('just some random text\nno frames here'), [])
  })
})

test('stack formats share one canonical matched-frame decoder', () => {
  const source = readFileSync('src/lib/ai-context/ai-context-parsing.ts', 'utf8')
  assert.match(source, /function parseMatchedFrame\(/)
  assert.match(source, /parseMatchedFrame\(line,\s*CHROME_FRAME_RE\)/)
  assert.match(source, /parseMatchedFrame\(line,\s*FIREFOX_FRAME_RE\)/)
})

// =============================================================================
// SOURCE MAP PARSING
// =============================================================================

describe('parseSourceMap', () => {
  test('parses inline base64 source map with sourcesContent', () => {
    const map = {
      version: 3,
      sources: ['app.ts'],
      sourcesContent: ['const x = 1;\nconsole.log(x);'],
      mappings: 'AAAA'
    }
    const encoded = Buffer.from(JSON.stringify(map)).toString('base64')
    const dataUrl = `data:application/json;base64,${encoded}`

    const result = parseSourceMap(dataUrl)

    assert.ok(result)
    assert.strictEqual(result.sources[0], 'app.ts')
    assert.ok(result.sourcesContent[0].includes('const x = 1'))
  })

  test('returns null for source map without sourcesContent', () => {
    const map = { version: 3, sources: ['a.ts'], mappings: 'AAAA' }
    const encoded = Buffer.from(JSON.stringify(map)).toString('base64')

    assert.strictEqual(parseSourceMap(`data:application/json;base64,${encoded}`), null)
  })

  test('returns null for non-data URL', () => {
    assert.strictEqual(parseSourceMap('https://example.com/map.js'), null)
  })

  test('returns null for invalid base64', () => {
    assert.strictEqual(parseSourceMap('data:application/json;base64,!!!bad!!!'), null)
  })

  test('returns null for null/undefined/empty', () => {
    assert.strictEqual(parseSourceMap(null), null)
    assert.strictEqual(parseSourceMap(undefined), null)
    assert.strictEqual(parseSourceMap(''), null)
  })
})

// =============================================================================
// CODE SNIPPET EXTRACTION
// =============================================================================

describe('extractSnippet', () => {
  const source20 = Array.from({ length: 20 }, (_, i) => `line ${i + 1} content`).join('\n')

  test('extracts context lines around error', () => {
    const snippet = extractSnippet(source20, 10)

    assert.ok(snippet)
    // 5 before + error + 5 after = 11
    assert.strictEqual(snippet.length, 11)
    assert.strictEqual(snippet[0].line, 5)
    const errorEntry = snippet.find((s) => s.isError)
    assert.strictEqual(errorEntry.line, 10)
  })

  test('handles error on first line', () => {
    const snippet = extractSnippet('first\nsecond\nthird', 1)

    assert.ok(snippet)
    assert.strictEqual(snippet[0].line, 1)
    assert.strictEqual(snippet[0].isError, true)
  })

  test('handles error on last line', () => {
    const snippet = extractSnippet('a\nb\nc', 3)

    const errorEntry = snippet.find((s) => s.isError)
    assert.strictEqual(errorEntry.line, 3)
    assert.strictEqual(errorEntry.text, 'c')
  })

  test('truncates lines longer than 200 chars', () => {
    const longLine = 'x'.repeat(300)
    const snippet = extractSnippet(`ok\n${longLine}\nok`, 2)

    const errorEntry = snippet.find((s) => s.isError)
    assert.ok(errorEntry.text.length <= 200)
  })

  test('returns null for line 0', () => {
    assert.strictEqual(extractSnippet('a', 0), null)
  })

  test('returns null for negative line', () => {
    assert.strictEqual(extractSnippet('a', -5), null)
  })

  test('returns null for line beyond file length', () => {
    assert.strictEqual(extractSnippet('one\ntwo', 100), null)
  })

  test('returns null for null/empty source', () => {
    assert.strictEqual(extractSnippet(null, 1), null)
    assert.strictEqual(extractSnippet('', 1), null)
  })

  test('marks only the error line with isError', () => {
    const snippet = extractSnippet(source20, 10)
    const errorLines = snippet.filter((s) => s.isError)

    assert.strictEqual(errorLines.length, 1)
    assert.strictEqual(errorLines[0].line, 10)
  })
})

// =============================================================================
// EXTRACT SOURCE SNIPPETS (multi-frame)
// =============================================================================

describe('extractSourceSnippets', () => {
  test('processes at most 3 frames', async () => {
    const code = Array(50).fill('code').join('\n')
    const frames = Array.from({ length: 5 }, (_, i) => ({ filename: `f${i}.js`, lineno: 10, colno: 1 }))
    const maps = Object.fromEntries(frames.map((f) => [f.filename, { sourcesContent: [code] }]))

    const snippets = await extractSourceSnippets(frames, maps)

    assert.ok(snippets.length <= 3)
  })

  test('skips frames with no source map', async () => {
    const frames = [{ filename: 'missing.js', lineno: 1, colno: 1 }]

    const snippets = await extractSourceSnippets(frames, {})

    assert.strictEqual(snippets.length, 0)
  })

  test('caps total payload at 10KB', async () => {
    const bigSource = Array.from({ length: 100 }, () => 'x'.repeat(200)).join('\n')
    const frames = [
      { filename: 'a.js', lineno: 50, colno: 1 },
      { filename: 'b.js', lineno: 50, colno: 1 },
      { filename: 'c.js', lineno: 50, colno: 1 }
    ]
    const maps = Object.fromEntries(frames.map((f) => [f.filename, { sourcesContent: [bigSource] }]))

    const snippets = await extractSourceSnippets(frames, maps)

    assert.ok(JSON.stringify(snippets).length <= 10240)
  })
})

// =============================================================================
// SOURCE MAP CACHE
// =============================================================================

describe('Source Map Cache', () => {
  beforeEach(() => {
    resetParsingForTesting()
  })

  test('set and get round-trip', () => {
    const map = { sources: ['a.ts'], sourcesContent: ['code'] }
    setSourceMapCache('http://localhost/a.js', map)

    assert.deepStrictEqual(getSourceMapCache('http://localhost/a.js'), map)
  })

  test('returns null for uncached URL', () => {
    assert.strictEqual(getSourceMapCache('http://localhost/nope.js'), null)
  })

  test('evicts oldest entry at capacity (20)', () => {
    for (let i = 0; i < 20; i++) {
      setSourceMapCache(`http://localhost/f${i}.js`, { sources: [`f${i}`], sourcesContent: ['c'] })
    }
    // Add one more — should evict f0
    setSourceMapCache('http://localhost/new.js', { sources: ['new'], sourcesContent: ['n'] })

    assert.ok(getSourceMapCacheSize() <= 20)
    assert.strictEqual(getSourceMapCache('http://localhost/f0.js'), null)
    assert.ok(getSourceMapCache('http://localhost/new.js'))
  })

  test('resetParsingForTesting clears cache', () => {
    setSourceMapCache('http://localhost/x.js', { sources: ['x'], sourcesContent: ['c'] })
    assert.strictEqual(getSourceMapCacheSize(), 1)

    resetParsingForTesting()

    assert.strictEqual(getSourceMapCacheSize(), 0)
  })
})
