// @ts-nocheck
/**
 * @fileoverview performance.test.js — Performance budget enforcement tests.
 * Validates that capture operations stay within SLO budgets to ensure the
 * extension doesn't degrade browsing performance. Measures per-operation
 * latency and fails if budgets are exceeded.
 *
 * SLOs (Service Level Objectives):
 * - Console interception: < 0.1ms per call
 * - Error serialization: < 1ms for typical payloads
 * - DOM snapshot: < 50ms for complex pages
 * - Message posting: < 0.5ms per message
 * - Error signature: < 0.1ms per computation
 * - Total error handling path: < 5ms
 */

import { test, describe, mock } from 'node:test'
import assert from 'node:assert'

// Performance measurement helper
function measureTime(fn, iterations = 1000) {
  const start = performance.now()
  for (let i = 0; i < iterations; i++) {
    fn()
  }
  const end = performance.now()
  return (end - start) / iterations
}

async function _measureTimeAsync(fn, iterations = 100) {
  const start = performance.now()
  for (let i = 0; i < iterations; i++) {
    await fn()
  }
  const end = performance.now()
  return (end - start) / iterations
}

// Mock window and document - MUST be set before importing inject.js
const mockWindow = {
  location: { href: 'http://localhost:3000/test' },
  innerWidth: 1920,
  innerHeight: 1080,
  postMessage: mock.fn(),
  addEventListener: mock.fn(),
  removeEventListener: mock.fn(),
  onerror: null,
  onunhandledrejection: null,
  performance: {
    now: () => performance.now(),
    getEntriesByType: () => [],
    mark: mock.fn(),
    measure: mock.fn()
  },
  PerformanceObserver: undefined,
  fetch: mock.fn(() => Promise.resolve({ ok: true, json: () => ({}) }))
}

const mockDocument = {
  body: {
    nodeType: 1,
    tagName: 'BODY',
    children: [],
    childNodes: [],
    attributes: [],
    textContent: '',
    getAttribute: () => null,
    parentElement: null
  },
  createElement: () => ({
    tagName: 'DIV',
    children: [],
    attributes: [],
    textContent: '',
    getAttribute: () => null
  }),
  addEventListener: mock.fn(),
  querySelector: mock.fn(() => null),
  querySelectorAll: mock.fn(() => [])
}

const mockConsole = {
  log: mock.fn(),
  warn: mock.fn(),
  error: mock.fn(),
  info: mock.fn(),
  debug: mock.fn()
}

// Set up global mocks BEFORE any imports
globalThis.window = mockWindow
globalThis.document = mockDocument
globalThis.console = mockConsole

describe('Performance Benchmarks', () => {
  describe('Serialization Performance', () => {
    test('should serialize simple objects under 0.1ms', async () => {
      const { safeSerialize } = await import('../../../extension/lib/page/serialize.js')

      const simpleObject = {
        message: 'Test error',
        code: 'ERR_001',
        timestamp: Date.now()
      }

      const avgTime = measureTime(() => safeSerialize(simpleObject))

      console.log(`  Simple object serialization: ${avgTime.toFixed(4)}ms`)
      assert.ok(avgTime < 0.1, `Serialization took ${avgTime}ms, expected < 0.1ms`)
    })

    test('should serialize nested objects under 0.5ms', async () => {
      const { safeSerialize } = await import('../../../extension/lib/page/serialize.js')

      const nestedObject = {
        level1: {
          level2: {
            level3: {
              level4: {
                value: 'deep',
                array: [1, 2, 3, 4, 5]
              }
            }
          }
        },
        metadata: {
          timestamp: Date.now(),
          source: 'test'
        }
      }

      const avgTime = measureTime(() => safeSerialize(nestedObject))

      console.log(`  Nested object serialization: ${avgTime.toFixed(4)}ms`)
      assert.ok(avgTime < 0.5, `Serialization took ${avgTime}ms, expected < 0.5ms`)
    })

    test('should serialize large arrays under 1ms', async () => {
      const { safeSerialize } = await import('../../../extension/lib/page/serialize.js')

      const largeArray = Array.from({ length: 100 }, (_, i) => ({
        id: i,
        name: `Item ${i}`,
        value: Math.random()
      }))

      const avgTime = measureTime(() => safeSerialize(largeArray), 100)

      console.log(`  Large array serialization (100 items): ${avgTime.toFixed(4)}ms`)
      assert.ok(avgTime < 1, `Serialization took ${avgTime}ms, expected < 1ms`)
    })

    test('should handle circular references without hanging', async () => {
      const { safeSerialize } = await import('../../../extension/lib/page/serialize.js')

      const circular = { a: 1 }
      circular.self = circular
      circular.nested = { parent: circular }

      const avgTime = measureTime(() => safeSerialize(circular))

      console.log(`  Circular reference handling: ${avgTime.toFixed(4)}ms`)
      assert.ok(avgTime < 0.5, `Circular handling took ${avgTime}ms, expected < 0.5ms`)
    })

    test('should truncate large strings efficiently', async () => {
      const { safeSerialize } = await import('../../../extension/lib/page/serialize.js')

      // 100KB string
      const largeString = 'x'.repeat(100 * 1024)

      const avgTime = measureTime(() => safeSerialize(largeString), 100)

      console.log(`  Large string truncation (100KB): ${avgTime.toFixed(4)}ms`)
      assert.ok(avgTime < 1, `Truncation took ${avgTime}ms, expected < 1ms`)
    })
  })

  describe('Error Signature Performance', () => {
    test('should compute error signature under 1ms', async () => {
      const { createErrorSignature } = await import('../../../extension/background/caches/error-groups.js')

      const errorEntry = {
        type: 'exception',
        level: 'error',
        message: 'Cannot read property "x" of undefined',
        stack: `TypeError: Cannot read property "x" of undefined
    at handleClick (app.js:42:15)
    at HTMLButtonElement.onclick (index.html:10:1)`,
        url: 'http://localhost:3000/app'
      }

      const avgTime = measureTime(() => createErrorSignature(errorEntry))

      console.log(`  Error signature computation: ${avgTime.toFixed(4)}ms`)
      assert.ok(avgTime < 1.0, `Signature took ${avgTime}ms, expected < 1.0ms`)
    })

    test('should compute network error signature under 0.1ms', async () => {
      const { createErrorSignature } = await import('../../../extension/background/caches/error-groups.js')

      const networkEntry = {
        type: 'network',
        level: 'error',
        method: 'POST',
        url: 'http://localhost:8789/api/users',
        status: 500
      }

      const avgTime = measureTime(() => createErrorSignature(networkEntry))

      console.log(`  Network error signature: ${avgTime.toFixed(4)}ms`)
      assert.ok(avgTime < 0.1, `Signature took ${avgTime}ms, expected < 0.1ms`)
    })
  })

  describe('Log Entry Formatting Performance', () => {
    test('should format log entry under 0.1ms', async () => {
      const { formatLogEntry } = await import('../../../extension/background/sync/log-processing.js')

      const rawEntry = {
        level: 'error',
        type: 'console',
        args: ['Error:', { code: 'ERR_001', details: 'Something went wrong' }],
        url: 'http://localhost:3000/app'
      }

      const avgTime = measureTime(() => formatLogEntry(rawEntry))

      console.log(`  Log entry formatting: ${avgTime.toFixed(4)}ms`)
      assert.ok(avgTime < 0.1, `Formatting took ${avgTime}ms, expected < 0.1ms`)
    })

    test('should format entry with large args under 1ms', async () => {
      const { formatLogEntry } = await import('../../../extension/background/sync/log-processing.js')

      const rawEntry = {
        level: 'error',
        type: 'console',
        args: Array.from({ length: 50 }, (_, i) => ({ key: i, value: `item-${i}` })),
        url: 'http://localhost:3000/app'
      }

      const avgTime = measureTime(() => formatLogEntry(rawEntry), 100)

      console.log(`  Large args formatting (50 items): ${avgTime.toFixed(4)}ms`)
      assert.ok(avgTime < 1, `Formatting took ${avgTime}ms, expected < 1ms`)
    })
  })

  describe('Error Grouping Performance', () => {
    test('should process error group under 0.2ms', async () => {
      const { processErrorGroup } = await import('../../../extension/background/caches/error-groups.js')

      const entry = {
        type: 'exception',
        level: 'error',
        message: 'Test error',
        stack: 'Error: Test\n    at test.js:1:1',
        url: 'http://localhost:3000',
        ts: new Date().toISOString()
      }

      // Include signature computation time
      const avgTime = measureTime(() => {
        processErrorGroup({ ...entry, ts: new Date().toISOString() })
      }, 500)

      console.log(`  Error group processing: ${avgTime.toFixed(4)}ms`)
      assert.ok(avgTime < 0.2, `Processing took ${avgTime}ms, expected < 0.2ms`)
    })
  })

  describe('Network Waterfall Performance', () => {
    test('should parse resource timing under 0.1ms per entry', async () => {
      const { parseResourceTiming } = await import('../../../extension/lib/net/network.js')

      const mockTiming = {
        name: 'http://localhost:3000/api/data',
        startTime: 100,
        duration: 150,
        domainLookupStart: 100,
        domainLookupEnd: 110,
        connectStart: 110,
        connectEnd: 120,
        secureConnectionStart: 115,
        requestStart: 120,
        responseStart: 200,
        responseEnd: 250,
        transferSize: 5000,
        encodedBodySize: 4500,
        decodedBodySize: 10000,
        initiatorType: 'fetch'
      }

      const avgTime = measureTime(() => parseResourceTiming(mockTiming))

      console.log(`  Resource timing parsing: ${avgTime.toFixed(4)}ms`)
      assert.ok(avgTime < 0.1, `Parsing took ${avgTime}ms, expected < 0.1ms`)
    })

    test('should get waterfall for 50 entries under 5ms', async () => {
      const { getNetworkWaterfall } = await import('../../../extension/lib/net/network.js')

      // Mock performance API with 50 entries
      const mockEntries = Array.from({ length: 50 }, (_, i) => ({
        name: `http://localhost:3000/api/resource-${i}`,
        startTime: i * 10,
        duration: 100,
        domainLookupStart: i * 10,
        domainLookupEnd: i * 10 + 5,
        connectStart: i * 10 + 5,
        connectEnd: i * 10 + 10,
        secureConnectionStart: 0,
        requestStart: i * 10 + 10,
        responseStart: i * 10 + 50,
        responseEnd: i * 10 + 100,
        transferSize: 1000,
        encodedBodySize: 900,
        decodedBodySize: 1000,
        initiatorType: 'fetch'
      }))

      globalThis.window.performance.getEntriesByType = () => mockEntries

      const avgTime = measureTime(() => getNetworkWaterfall(), 100)

      console.log(`  Network waterfall (50 entries): ${avgTime.toFixed(4)}ms`)
      assert.ok(avgTime < 5, `Waterfall took ${avgTime}ms, expected < 5ms`)
    })
  })

  describe('User Action Buffer Performance', () => {
    test('should record action under 0.1ms', async () => {
      const { recordAction, clearActionBuffer } = await import('../../../extension/lib/page/actions.js')

      clearActionBuffer()

      const mockEvent = {
        type: 'click',
        target: {
          tagName: 'BUTTON',
          id: 'submit-btn',
          className: 'btn primary',
          getAttribute: (name) => (name === 'data-testid' ? 'submit' : null),
          textContent: 'Submit',
          type: 'button',
          name: ''
        }
      }

      const avgTime = measureTime(() => recordAction(mockEvent))

      console.log(`  Action recording: ${avgTime.toFixed(4)}ms`)
      assert.ok(avgTime < 0.1, `Recording took ${avgTime}ms, expected < 0.1ms`)
    })

    test('should maintain buffer under 20 items efficiently', async () => {
      const { recordAction, getActionBuffer, clearActionBuffer } = await import('../../../extension/lib/page/actions.js')

      clearActionBuffer()

      const mockEvent = {
        type: 'click',
        target: {
          tagName: 'BUTTON',
          id: 'btn',
          className: '',
          getAttribute: () => null,
          textContent: 'Click',
          type: 'button',
          name: ''
        }
      }

      // Record 50 actions (should maintain 20)
      const avgTime = measureTime(() => {
        for (let i = 0; i < 50; i++) {
          recordAction(mockEvent)
        }
      }, 10)

      console.log(`  50 action recordings: ${avgTime.toFixed(4)}ms`)
      assert.ok(avgTime < 5, `50 recordings took ${avgTime}ms, expected < 5ms`)

      const buffer = getActionBuffer()
      assert.ok(buffer.length <= 20, `Buffer has ${buffer.length} items, expected <= 20`)
    })
  })

  describe('Full Error Path Performance', () => {
    test('should handle complete error flow under 5ms', async () => {
      const { formatLogEntry } = await import('../../../extension/background/sync/log-processing.js')
      const { createErrorSignature, processErrorGroup } = await import(
        '../../../extension/background/caches/error-groups.js'
      )
      const { safeSerialize } = await import('../../../extension/lib/page/serialize.js')

      // Simulate the complete error handling path
      const rawError = {
        level: 'error',
        type: 'exception',
        message: 'Cannot read property "x" of undefined',
        stack: `TypeError: Cannot read property "x" of undefined
    at handleClick (app.js:42:15)
    at HTMLButtonElement.onclick (index.html:10:1)`,
        args: [{ code: 'ERR_001', context: { user: 'test' } }],
        url: 'http://localhost:3000/app'
      }

      const avgTime = measureTime(() => {
        // 1. Serialize args
        const serialized = safeSerialize(rawError.args)

        // 2. Format entry
        const entry = formatLogEntry({ ...rawError, args: serialized })

        // 3. Compute signature
        createErrorSignature(entry)

        // 4. Process through error grouping
        processErrorGroup(entry)
      }, 100)

      console.log(`  Complete error path: ${avgTime.toFixed(4)}ms`)
      assert.ok(avgTime < 5, `Complete path took ${avgTime}ms, expected < 5ms`)
    })
  })

  describe('Memory Safety', () => {
    test('should maintain bounded memory with repeated operations', async () => {
      const { createErrorSignature, processErrorGroup } = await import(
        '../../../extension/background/caches/error-groups.js'
      )

      // Run 3 batches to check for memory growth stabilization
      const memoryReadings = []

      for (let batch = 0; batch < 3; batch++) {
        // Perform operations
        for (let i = 0; i < 200; i++) {
          const entry = {
            type: 'exception',
            level: 'error',
            message: `Error ${batch}-${i}`,
            stack: `Error: ${batch}-${i}\n    at test.js:${i}:1`,
            url: 'http://localhost:3000',
            ts: new Date().toISOString()
          }
          createErrorSignature(entry)
          processErrorGroup(entry)
        }
        memoryReadings.push(process.memoryUsage().heapUsed)
      }

      // Memory growth between batches should stabilize (not grow unbounded)
      const growth1 = (memoryReadings[1] - memoryReadings[0]) / 1024 / 1024
      const growth2 = (memoryReadings[2] - memoryReadings[1]) / 1024 / 1024

      console.log(`  Memory growth batch1→2: ${growth1.toFixed(2)}MB, batch2→3: ${growth2.toFixed(2)}MB`)

      // Second batch growth should not be significantly more than first (linear or bounded)
      // This catches unbounded memory leaks
      // Note: GC timing is non-deterministic, so we use a generous threshold.
      // The key invariant is that growth doesn't accelerate unboundedly.
      const growthRatio = growth2 > 0 ? growth2 / Math.max(growth1, 0.1) : 0
      assert.ok(growthRatio < 20, `Memory growth accelerating: ratio ${growthRatio.toFixed(2)}, expected < 20`)
    })
  })
})
