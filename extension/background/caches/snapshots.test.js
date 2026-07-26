// @ts-nocheck
/**
 * @fileoverview snapshots.test.js - Tests for source map functions
 * Co-located with snapshots.js implementation
 */

import { test, describe } from 'node:test'
import assert from 'node:assert'

// Import directly from the implementation file (not through barrel export)
import {
  decodeVLQ,
  parseMappings,
  parseStackFrame,
  extractSourceMapUrl,
  parseSourceMapData,
  findOriginalLocation
} from './snapshots.js'

describe('decodeVLQ', () => {
  test('should decode simple VLQ values', () => {
    // 'A' = 0
    assert.deepStrictEqual(decodeVLQ('A'), [0])

    // 'C' = 1
    assert.deepStrictEqual(decodeVLQ('C'), [1])

    // 'D' = -1
    assert.deepStrictEqual(decodeVLQ('D'), [-1])

    // 'K' = 5
    assert.deepStrictEqual(decodeVLQ('K'), [5])
  })

  test('should decode multi-value VLQ strings', () => {
    // 'AAAA' = [0, 0, 0, 0]
    assert.deepStrictEqual(decodeVLQ('AAAA'), [0, 0, 0, 0])

    // 'ACAC' = [0, 1, 0, 1]
    assert.deepStrictEqual(decodeVLQ('ACAC'), [0, 1, 0, 1])
  })

  test('should decode large VLQ values', () => {
    // 'gB' = 16 (uses continuation bit)
    assert.deepStrictEqual(decodeVLQ('gB'), [16])

    // '2B' = 27
    assert.deepStrictEqual(decodeVLQ('2B'), [27])
  })

  test('should throw on invalid VLQ character', () => {
    assert.throws(() => decodeVLQ('!'), /Invalid VLQ character/)
  })
})

describe('parseMappings', () => {
  test('should parse simple mappings string', () => {
    const result = parseMappings('AAAA;AACA')
    assert.strictEqual(result.length, 2)
    assert.strictEqual(result[0].length, 1)
    assert.strictEqual(result[1].length, 1)
  })

  test('should handle empty lines', () => {
    const result = parseMappings('AAAA;;AACA')
    assert.strictEqual(result.length, 3)
    assert.strictEqual(result[1].length, 0) // Empty line
  })

  test('should parse multiple segments per line', () => {
    const result = parseMappings('AAAA,CACA,EAEA')
    assert.strictEqual(result.length, 1)
    assert.strictEqual(result[0].length, 3)
  })
})

describe('parseStackFrame', () => {
  test('should parse standard Chrome stack frame', () => {
    const line = '    at functionName (http://localhost:3000/app.js:42:15)'
    const result = parseStackFrame(line)

    assert.ok(result)
    assert.strictEqual(result.functionName, 'functionName')
    assert.strictEqual(result.fileName, 'http://localhost:3000/app.js')
    assert.strictEqual(result.lineNumber, 42)
    assert.strictEqual(result.columnNumber, 15)
  })

  test('should parse anonymous stack frame', () => {
    const line = '    at http://localhost:3000/app.js:100:5'
    const result = parseStackFrame(line)

    assert.ok(result)
    assert.strictEqual(result.functionName, '<anonymous>')
    assert.strictEqual(result.lineNumber, 100)
  })

  test('should return null for non-stack lines', () => {
    assert.strictEqual(parseStackFrame('Error: Something went wrong'), null)
    assert.strictEqual(parseStackFrame(''), null)
  })
})

describe('extractSourceMapUrl', () => {
  test('should extract sourceMappingURL from script', () => {
    const content = 'function foo(){}\n//# sourceMappingURL=app.js.map'
    const url = extractSourceMapUrl(content)

    assert.strictEqual(url, 'app.js.map')
  })

  test('should extract data URL source map', () => {
    const content = 'function foo(){}\n//# sourceMappingURL=data:application/json;base64,eyJ2ZXJzaW9uIjozfQ=='
    const url = extractSourceMapUrl(content)

    assert.ok(url.startsWith('data:'))
  })

  test('should return null if no source map', () => {
    const content = 'function foo(){}'
    const url = extractSourceMapUrl(content)

    assert.strictEqual(url, null)
  })

  test('should handle deprecated @ syntax', () => {
    const content = 'function foo(){}\n//@ sourceMappingURL=old.js.map'
    const url = extractSourceMapUrl(content)

    assert.strictEqual(url, 'old.js.map')
  })
})

describe('parseSourceMapData', () => {
  test('should parse source map data', () => {
    const sourceMap = {
      version: 3,
      sources: ['src/app.ts'],
      names: ['foo', 'bar'],
      mappings: 'AAAA;AACA',
      sourceRoot: ''
    }

    const result = parseSourceMapData(sourceMap)

    assert.deepStrictEqual(result.sources, ['src/app.ts'])
    assert.deepStrictEqual(result.names, ['foo', 'bar'])
    assert.ok(Array.isArray(result.mappings))
  })

  test('should handle empty source map', () => {
    const result = parseSourceMapData({})

    assert.deepStrictEqual(result.sources, [])
    assert.deepStrictEqual(result.names, [])
  })
})

describe('findOriginalLocation', () => {
  test('should find original location from source map', () => {
    // A simple source map: one source file, one mapping at line 1, col 0
    // AAAA maps to: genCol=0, sourceIdx=0, origLine=0, origCol=0
    const sourceMap = parseSourceMapData({
      version: 3,
      sources: ['src/original.ts'],
      names: [],
      mappings: 'AAAA'
    })

    const result = findOriginalLocation(sourceMap, 1, 0)

    assert.ok(result)
    assert.strictEqual(result.source, 'src/original.ts')
    assert.strictEqual(result.line, 1)
    assert.strictEqual(result.column, 0)
  })

  test('should return null for out of bounds line', () => {
    const sourceMap = parseSourceMapData({
      version: 3,
      sources: ['src/app.ts'],
      mappings: 'AAAA'
    })

    const result = findOriginalLocation(sourceMap, 100, 0)

    assert.strictEqual(result, null)
  })

  test('should return null for null source map', () => {
    assert.strictEqual(findOriginalLocation(null, 1, 0), null)
  })
})
