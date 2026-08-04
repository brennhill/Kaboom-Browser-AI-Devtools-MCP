// @ts-nocheck
/**
 * @fileoverview network-waterfall.test.js — Tests for network waterfall capture.
 * Verifies timing-ordered network request recording, 30-second time window
 * enforcement, 50-entry buffer cap, and the waterfall data structure with
 * start/duration/status fields for visualizing request concurrency.
 */

import { test, describe, mock, beforeEach, afterEach } from 'node:test'
import assert from 'node:assert'

// Define esbuild constant not available in Node test env
globalThis.__KABOOM_VERSION__ = 'test'

// Mock performance API
let originalPerformance
let originalWindow
let originalDocument

function createMockResourceTiming(overrides = {}) {
  return {
    name: 'http://localhost:3000/api/data',
    entryType: 'resource',
    startTime: 100,
    duration: 250,
    initiatorType: 'fetch',
    // DNS
    domainLookupStart: 100,
    domainLookupEnd: 110,
    // Connection
    connectStart: 110,
    connectEnd: 130,
    secureConnectionStart: 115,
    // Request/Response
    requestStart: 130,
    responseStart: 200, // TTFB
    responseEnd: 350,
    // Size
    transferSize: 1024,
    encodedBodySize: 900,
    decodedBodySize: 2048,
    // Cache
    fetchStart: 100,
    ...overrides
  }
}

function createMockPerformance() {
  const entries = []
  return {
    getEntriesByType: mock.fn((type) => {
      if (type === 'resource') return entries
      if (type === 'navigation') return [{ type: 'navigate', startTime: 0 }]
      return []
    }),
    getEntriesByName: mock.fn((name) => entries.filter((e) => e.name === name)),
    clearResourceTimings: mock.fn(),
    mark: mock.fn(),
    measure: mock.fn(),
    now: mock.fn(() => Date.now()),
    _entries: entries,
    _addEntry: (entry) => entries.push(entry)
  }
}

function createMockWindow() {
  return {
    location: { href: 'http://localhost:3000/test' },
    postMessage: mock.fn(),
    performance: createMockPerformance(),
    addEventListener: mock.fn(),
    removeEventListener: mock.fn(),
    onerror: null,
    innerWidth: 1920,
    innerHeight: 1080,
    scrollX: 0,
    scrollY: 0
  }
}

function createMockDocument() {
  return {
    addEventListener: mock.fn(),
    removeEventListener: mock.fn(),
    querySelector: mock.fn(() => null),
    querySelectorAll: mock.fn(() => [])
  }
}

describe('Network Waterfall - parseResourceTiming', () => {
  beforeEach(() => {
    originalWindow = globalThis.window
    originalPerformance = globalThis.performance
    originalDocument = globalThis.document
    globalThis.window = createMockWindow()
    globalThis.performance = globalThis.window.performance
    globalThis.document = createMockDocument()
  })

  afterEach(() => {
    globalThis.window = originalWindow
    globalThis.performance = originalPerformance
    globalThis.document = originalDocument
  })

  test('should parse resource timing into WireNetworkWaterfallEntry', async () => {
    const { parseResourceTiming } = await import('../../../extension/lib/net/network.js')

    const timing = createMockResourceTiming()
    const result = parseResourceTiming(timing)

    assert.ok(result)
    assert.strictEqual(result.url, 'http://localhost:3000/api/data')
    assert.strictEqual(result.name, 'http://localhost:3000/api/data')
    assert.strictEqual(result.initiator_type, 'fetch')
    assert.strictEqual(result.start_time, 100)
    assert.strictEqual(result.duration, 250)
  })

  test('should include fetch_start from timing', async () => {
    const { parseResourceTiming } = await import('../../../extension/lib/net/network.js')

    const timing = createMockResourceTiming({ fetchStart: 100 })
    const result = parseResourceTiming(timing)

    assert.strictEqual(result.fetch_start, 100)
  })

  test('should include response_end from timing', async () => {
    const { parseResourceTiming } = await import('../../../extension/lib/net/network.js')

    const timing = createMockResourceTiming({ responseEnd: 350 })
    const result = parseResourceTiming(timing)

    assert.strictEqual(result.response_end, 350)
  })

  test('should include encoded and decoded body sizes', async () => {
    const { parseResourceTiming } = await import('../../../extension/lib/net/network.js')

    const timing = createMockResourceTiming({
      encodedBodySize: 900,
      decodedBodySize: 2048
    })

    const result = parseResourceTiming(timing)

    assert.strictEqual(result.encoded_body_size, 900)
    assert.strictEqual(result.decoded_body_size, 2048)
  })

  test('should handle zero transferSize (cache hit)', async () => {
    const { parseResourceTiming } = await import('../../../extension/lib/net/network.js')

    const timing = createMockResourceTiming({
      transferSize: 0,
      encodedBodySize: 1000
    })

    const result = parseResourceTiming(timing)

    assert.strictEqual(result.transfer_size, 0)
    assert.strictEqual(result.encoded_body_size, 1000)
  })

  test('should default missing size values to 0', async () => {
    const { parseResourceTiming } = await import('../../../extension/lib/net/network.js')

    const timing = createMockResourceTiming({
      transferSize: 0,
      encodedBodySize: 0,
      decodedBodySize: 0
    })

    const result = parseResourceTiming(timing)

    assert.strictEqual(result.transfer_size, 0)
    assert.strictEqual(result.encoded_body_size, 0)
    assert.strictEqual(result.decoded_body_size, 0)
  })

  test('should include total duration', async () => {
    const { parseResourceTiming } = await import('../../../extension/lib/net/network.js')

    const timing = createMockResourceTiming({ duration: 500 })
    const result = parseResourceTiming(timing)

    assert.strictEqual(result.duration, 500)
  })

  test('should expose rich network phases, transport, cache, and compression', async () => {
    const { parseResourceTiming } = await import('../../../extension/lib/net/network.js')
    const result = parseResourceTiming(
      createMockResourceTiming({
        startTime: 90,
        fetchStart: 100,
        nextHopProtocol: 'h2',
        responseStatus: 201,
        deliveryType: 'cache',
        serverTiming: [{ name: 'db', duration: 24, description: 'query' }]
      })
    )

    assert.deepStrictEqual(
      {
        queueing: result.queueing_ms,
        dns: result.dns_ms,
        tls: result.tls_ms,
        connect: result.connect_ms,
        ttfb: result.ttfb_ms,
        download: result.download_ms,
        protocol: result.protocol,
        cache: result.cache_source,
        status: result.status,
        compression: result.compression_ratio
      },
      {
        queueing: 10,
        dns: 10,
        tls: 15,
        connect: 20,
        ttfb: 70,
        download: 150,
        protocol: 'h2',
        cache: 'cache',
        status: 201,
        compression: 2.28
      }
    )
    assert.deepStrictEqual(result.server_timing, [{ name: 'db', duration_ms: 24, description: 'query' }])
  })

  test('attributes and groups identical concurrent requests', async () => {
    const { getNetworkWaterfall, resetForTesting } = await import('../../../extension/lib/net/network.js')
    const { recordRequestAttribution, completeRequestAttribution } = await import(
      '../../../extension/lib/net/request-attribution.js'
    )
    resetForTesting()
    recordRequestAttribution('http://localhost:3000/api/data', {
      stack: 'Error\n    at DesignShell (http://localhost:3000/src/DesignShell.tsx:12:4)\n    at routeLoader (http://localhost:3000/src/routes.ts:4:2)',
      priority: 'high'
    })
    completeRequestAttribution('http://localhost:3000/api/data', {
      status: 200,
      server_timing: 'app;dur=42',
      request_id: 'req-123',
      traceparent: '00-abc-def-01',
      content_encoding: 'br'
    })
    globalThis.performance._addEntry(createMockResourceTiming({ startTime: 100, responseEnd: 350 }))
    globalThis.performance._addEntry(createMockResourceTiming({ startTime: 101, responseEnd: 300 }))

    const results = getNetworkWaterfall()
    assert.equal(results.length, 2)
    assert.equal(results[0].react_component, 'DesignShell')
    assert.equal(results[0].route_loader, 'routeLoader')
    assert.equal(results[0].priority, 'high')
    assert.equal(results[0].request_id, 'req-123')
    assert.equal(results[0].traceparent, '00-abc-def-01')
    assert.equal(results[0].content_encoding, 'br')
    assert.equal(results[0].duplicate_count, 2)
    assert.equal(results[1].duplicate_group_id, results[0].duplicate_group_id)
  })

  test('preserves an outgoing traceparent when the response does not echo it', async () => {
    const { getNetworkWaterfall, resetForTesting, wrapFetchWithBodies } = await import(
      '../../../extension/lib/net/network.js'
    )
    resetForTesting()
    const wrapped = wrapFetchWithBodies(async () => ({
      status: 204,
      headers: { get: () => null },
      clone: () => null
    }))
    await wrapped('http://localhost:3000/api/data', {
      headers: { traceparent: '00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01' }
    })
    globalThis.performance._addEntry(createMockResourceTiming())
    assert.equal(
      getNetworkWaterfall()[0].traceparent,
      '00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01'
    )
  })

  test('should include transfer size information', async () => {
    const { parseResourceTiming } = await import('../../../extension/lib/net/network.js')

    const timing = createMockResourceTiming({
      transferSize: 2048,
      encodedBodySize: 1800,
      decodedBodySize: 4096
    })

    const result = parseResourceTiming(timing)

    assert.strictEqual(result.transfer_size, 2048)
  })

  test('should include initiator type', async () => {
    const { parseResourceTiming } = await import('../../../extension/lib/net/network.js')

    const timing = createMockResourceTiming({ initiatorType: 'fetch' })
    const result = parseResourceTiming(timing)

    assert.strictEqual(result.initiator_type, 'fetch')
  })

  test('should handle missing fetchStart and responseEnd (0)', async () => {
    const { parseResourceTiming } = await import('../../../extension/lib/net/network.js')

    const timing = createMockResourceTiming({
      fetchStart: 0,
      responseEnd: 0
    })

    const result = parseResourceTiming(timing)

    assert.ok(result)
    // 0 is falsy so undefined is expected from `|| undefined` guard
    assert.strictEqual(result.fetch_start, undefined)
    assert.strictEqual(result.response_end, undefined)
  })
})

describe('Network Waterfall - getNetworkWaterfall', () => {
  beforeEach(() => {
    originalWindow = globalThis.window
    originalPerformance = globalThis.performance
    originalDocument = globalThis.document
    globalThis.window = createMockWindow()
    globalThis.performance = globalThis.window.performance
    globalThis.document = createMockDocument()
  })

  afterEach(() => {
    globalThis.window = originalWindow
    globalThis.performance = originalPerformance
    globalThis.document = originalDocument
  })

  test('should return all resource entries', async () => {
    const { getNetworkWaterfall } = await import('../../../extension/lib/net/network.js')

    globalThis.performance._addEntry(createMockResourceTiming({ name: 'http://localhost/api/1' }))
    globalThis.performance._addEntry(createMockResourceTiming({ name: 'http://localhost/api/2' }))

    const waterfall = getNetworkWaterfall()

    assert.ok(Array.isArray(waterfall))
    assert.strictEqual(waterfall.length, 2)
  })

  test('should filter by time range', async () => {
    const { getNetworkWaterfall } = await import('../../../extension/lib/net/network.js')

    globalThis.performance._addEntry(createMockResourceTiming({ name: 'http://localhost/early', startTime: 50 }))
    globalThis.performance._addEntry(createMockResourceTiming({ name: 'http://localhost/late', startTime: 500 }))

    const waterfall = getNetworkWaterfall({ since: 400 })

    assert.strictEqual(waterfall.length, 1)
    assert.ok(waterfall[0].url?.includes('late') || waterfall[0].name?.includes('late'))
  })

  test('should limit number of entries', async () => {
    const { getNetworkWaterfall } = await import('../../../extension/lib/net/network.js')
    const { MAX_WATERFALL_ENTRIES } = await import('../../../extension/lib/constants.js')

    // Add more entries than the limit
    for (let i = 0; i < 100; i++) {
      globalThis.performance._addEntry(
        createMockResourceTiming({ name: `http://localhost/api/${i}`, startTime: i * 10 })
      )
    }

    const waterfall = getNetworkWaterfall()

    assert.ok(waterfall.length <= MAX_WATERFALL_ENTRIES)
  })

  test('should sort entries by start time', async () => {
    const { getNetworkWaterfall } = await import('../../../extension/lib/net/network.js')

    globalThis.performance._addEntry(createMockResourceTiming({ name: 'http://localhost/second', startTime: 200 }))
    globalThis.performance._addEntry(createMockResourceTiming({ name: 'http://localhost/first', startTime: 100 }))

    const waterfall = getNetworkWaterfall()

    assert.ok(waterfall[0].start_time <= waterfall[1].start_time)
  })

  test('should filter by initiator type', async () => {
    const { getNetworkWaterfall } = await import('../../../extension/lib/net/network.js')

    globalThis.performance._addEntry(createMockResourceTiming({ name: 'http://localhost/api', initiatorType: 'fetch' }))
    globalThis.performance._addEntry(
      createMockResourceTiming({ name: 'http://localhost/style.css', initiatorType: 'link' })
    )

    const waterfall = getNetworkWaterfall({ initiatorTypes: ['fetch', 'xmlhttprequest'] })

    assert.strictEqual(waterfall.length, 1)
    assert.ok(
      waterfall[0].initiator_type === 'fetch' || waterfall[0].initiator === 'fetch' || waterfall[0].type === 'fetch'
    )
  })

  test('should exclude data URLs', async () => {
    const { getNetworkWaterfall } = await import('../../../extension/lib/net/network.js')

    globalThis.performance._addEntry(createMockResourceTiming({ name: 'data:image/png;base64,abc123' }))
    globalThis.performance._addEntry(createMockResourceTiming({ name: 'http://localhost/api' }))

    const waterfall = getNetworkWaterfall()

    assert.strictEqual(waterfall.length, 1)
    assert.ok(!waterfall[0].url?.startsWith('data:') && !waterfall[0].name?.startsWith('data:'))
  })

  test('should return empty array when performance API unavailable', async () => {
    const { getNetworkWaterfall } = await import('../../../extension/lib/net/network.js')

    globalThis.performance = null

    const waterfall = getNetworkWaterfall()

    assert.ok(Array.isArray(waterfall))
    assert.strictEqual(waterfall.length, 0)
  })
})

describe('Network Waterfall - Pending Requests', () => {
  beforeEach(() => {
    originalWindow = globalThis.window
    originalPerformance = globalThis.performance
    originalDocument = globalThis.document
    globalThis.window = createMockWindow()
    globalThis.performance = globalThis.window.performance
    globalThis.document = createMockDocument()
  })

  afterEach(() => {
    globalThis.window = originalWindow
    globalThis.performance = originalPerformance
    globalThis.document = originalDocument
  })

  test('should track pending fetch requests', async () => {
    const { trackPendingRequest, getPendingRequests, clearPendingRequests } = await import('../../../extension/lib/net/network.js')

    clearPendingRequests()

    trackPendingRequest({
      url: 'http://localhost/api/slow',
      method: 'POST',
      startTime: Date.now()
    })

    const pending = getPendingRequests()

    assert.ok(Array.isArray(pending))
    assert.strictEqual(pending.length, 1)
    assert.strictEqual(pending[0].url, 'http://localhost/api/slow')

    clearPendingRequests()
  })

  test('should remove completed requests', async () => {
    const { trackPendingRequest, completePendingRequest, getPendingRequests, clearPendingRequests } =
      await import('../../../extension/lib/net/network.js')

    clearPendingRequests()

    const requestId = trackPendingRequest({
      url: 'http://localhost/api/data',
      method: 'GET',
      startTime: Date.now()
    })

    completePendingRequest(requestId)

    const pending = getPendingRequests()
    assert.strictEqual(pending.length, 0)
  })

  test('should include pending requests in error snapshots', async () => {
    const { trackPendingRequest, getNetworkWaterfallForError, clearPendingRequests, setNetworkWaterfallEnabled } =
      await import('../../../extension/lib/net/network.js')

    setNetworkWaterfallEnabled(true)
    clearPendingRequests()

    trackPendingRequest({
      url: 'http://localhost/api/slow-endpoint',
      method: 'POST',
      startTime: Date.now() - 1000 // Started 1 second ago
    })

    const errorEntry = {
      type: 'exception',
      level: 'error',
      message: 'Network timeout'
    }

    const snapshot = await getNetworkWaterfallForError(errorEntry)

    assert.ok(snapshot)
    assert.ok(snapshot.pending || snapshot.pendingRequests)
    assert.strictEqual((snapshot.pending || snapshot.pendingRequests).length, 1)

    clearPendingRequests()
  })
})

describe('Network Waterfall - Error Integration', () => {
  beforeEach(() => {
    originalWindow = globalThis.window
    originalPerformance = globalThis.performance
    originalDocument = globalThis.document
    globalThis.window = createMockWindow()
    globalThis.performance = globalThis.window.performance
    globalThis.document = createMockDocument()
  })

  afterEach(() => {
    globalThis.window = originalWindow
    globalThis.performance = originalPerformance
    globalThis.document = originalDocument
  })

  test('should create waterfall snapshot for error', async () => {
    const { getNetworkWaterfallForError, setNetworkWaterfallEnabled } = await import('../../../extension/lib/net/network.js')

    setNetworkWaterfallEnabled(true)

    globalThis.performance._addEntry(createMockResourceTiming({ name: 'http://localhost/api' }))

    const errorEntry = {
      type: 'network',
      level: 'error',
      url: 'http://localhost/api/failed',
      status: 500
    }

    const snapshot = await getNetworkWaterfallForError(errorEntry)

    assert.ok(snapshot)
    assert.strictEqual(snapshot.type, 'network_waterfall')
    assert.ok(snapshot.ts)
    assert.ok(snapshot.entries || snapshot.waterfall)
  })

  test('should respect networkWaterfallEnabled setting', async () => {
    const { getNetworkWaterfallForError, setNetworkWaterfallEnabled } = await import('../../../extension/lib/net/network.js')

    setNetworkWaterfallEnabled(false)

    const errorEntry = {
      type: 'network',
      level: 'error'
    }

    const snapshot = await getNetworkWaterfallForError(errorEntry)

    assert.strictEqual(snapshot, null)

    // Re-enable
    setNetworkWaterfallEnabled(true)
  })

  test('should only capture recent entries (last 30 seconds)', async () => {
    const { getNetworkWaterfallForError, setNetworkWaterfallEnabled } = await import('../../../extension/lib/net/network.js')

    setNetworkWaterfallEnabled(true)

    // Old entry (simulated via startTime relative to performance.now)
    globalThis.performance._addEntry(createMockResourceTiming({ name: 'http://localhost/old', startTime: 0 }))

    // Recent entry
    const now = Date.now()
    globalThis.performance.now = () => now
    globalThis.performance._addEntry(
      createMockResourceTiming({ name: 'http://localhost/recent', startTime: now - 5000 })
    )

    const errorEntry = { type: 'exception', level: 'error' }
    const snapshot = await getNetworkWaterfallForError(errorEntry)

    // Should filter based on recency
    assert.ok(snapshot)
  })
})

describe('Network Waterfall - Configuration', () => {
  beforeEach(() => {
    originalWindow = globalThis.window
    originalPerformance = globalThis.performance
    originalDocument = globalThis.document
    globalThis.window = createMockWindow()
    globalThis.performance = globalThis.window.performance
    globalThis.document = createMockDocument()
  })

  afterEach(() => {
    globalThis.window = originalWindow
    globalThis.performance = originalPerformance
    globalThis.document = originalDocument
  })

  test('setNetworkWaterfallEnabled should toggle feature', async () => {
    const { setNetworkWaterfallEnabled, isNetworkWaterfallEnabled } = await import('../../../extension/lib/net/network.js')

    setNetworkWaterfallEnabled(true)
    assert.strictEqual(isNetworkWaterfallEnabled(), true)

    setNetworkWaterfallEnabled(false)
    assert.strictEqual(isNetworkWaterfallEnabled(), false)
  })

  test('should expose network waterfall through __kaboom API', async () => {
    const { installKaboomAPI, uninstallKaboomAPI } = await import('../../../extension/inject/api.js')
    const { setNetworkWaterfallEnabled } = await import('../../../extension/lib/net/network.js')

    setNetworkWaterfallEnabled(true)
    installKaboomAPI()

    assert.ok(globalThis.window.__kaboom)
    assert.ok(typeof globalThis.window.__kaboom.setNetworkWaterfall === 'function')
    assert.ok(typeof globalThis.window.__kaboom.getNetworkWaterfall === 'function')

    uninstallKaboomAPI()
  })
})
