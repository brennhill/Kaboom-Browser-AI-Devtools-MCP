// @ts-nocheck
/**
 * @fileoverview performance-snapshot.test.js — Tests for on-demand performance snapshots.
 * Verifies collection of navigation timing, resource timing, Web Vitals (LCP, CLS, FCP),
 * long tasks, and memory usage into a single snapshot payload posted to the server.
 */

import { test, describe, mock, beforeEach, afterEach } from 'node:test'
import assert from 'node:assert'
import { createMockWindow } from './helpers.js'

// Mock PerformanceObserver (specialized - only needed by this test file)
class MockPerformanceObserver {
  constructor(callback) {
    MockPerformanceObserver.instances.push(this)
    this.callback = callback
    this.observedTypes = []
  }
  observe(opts) {
    this.observedTypes.push(opts.type || opts.entryTypes)
  }
  disconnect() {}
}
MockPerformanceObserver.instances = []

// Mock performance API (specialized - only needed by this test file)
const createMockPerformance = () => ({
  getEntriesByType: mock.fn((type) => {
    if (type === 'navigation') {
      return [
        {
          domContentLoadedEventEnd: 600,
          loadEventEnd: 1200,
          responseStart: 180,
          requestStart: 100,
          domInteractive: 500
        }
      ]
    }
    if (type === 'resource') {
      return [
        {
          initiatorType: 'script',
          transferSize: 50000,
          decodedBodySize: 100000,
          duration: 300,
          name: 'http://localhost/app.js'
        },
        {
          initiatorType: 'css',
          transferSize: 10000,
          decodedBodySize: 20000,
          duration: 100,
          name: 'http://localhost/style.css'
        },
        {
          initiatorType: 'img',
          transferSize: 80000,
          decodedBodySize: 80000,
          duration: 500,
          name: 'http://localhost/hero.png'
        },
        {
          initiatorType: 'fetch',
          transferSize: 2000,
          decodedBodySize: 5000,
          duration: 200,
          name: 'http://localhost/api/data'
        }
      ]
    }
    return []
  })
})

let originalWindow, originalPerformance, originalPerformanceObserver

describe('Performance Snapshot Capture', () => {
  beforeEach(() => {
    originalWindow = globalThis.window
    originalPerformance = globalThis.performance
    originalPerformanceObserver = globalThis.PerformanceObserver
    MockPerformanceObserver.instances = []
    globalThis.window = createMockWindow({
      href: 'http://localhost:3000/dashboard',
      pathname: '/dashboard'
    })
    globalThis.performance = createMockPerformance()
    globalThis.PerformanceObserver = MockPerformanceObserver
  })

  afterEach(() => {
    globalThis.window = originalWindow
    globalThis.performance = originalPerformance
    globalThis.PerformanceObserver = originalPerformanceObserver
  })

  test('capturePerformanceSnapshot returns spec-compliant shape', async () => {
    const { capturePerformanceSnapshot } = await import('../../extension/lib/analysis/perf-snapshot.js')
    const snapshot = capturePerformanceSnapshot()

    // Top-level fields from spec
    assert.ok('url' in snapshot, 'missing: url')
    assert.ok('timestamp' in snapshot, 'missing: timestamp')
    assert.ok('timing' in snapshot, 'missing: timing')
    assert.ok('network' in snapshot, 'missing: network')
    assert.ok('long_tasks' in snapshot, 'missing: long_tasks')
    assert.ok('cumulative_layout_shift' in snapshot, 'missing: cumulative_layout_shift')

    // timing fields from spec
    assert.ok('dom_content_loaded' in snapshot.timing, 'missing: timing.dom_content_loaded')
    assert.ok('load' in snapshot.timing, 'missing: timing.load')
    assert.ok('first_contentful_paint' in snapshot.timing, 'missing: timing.first_contentful_paint')
    assert.ok('largest_contentful_paint' in snapshot.timing, 'missing: timing.largest_contentful_paint')
    assert.ok('time_to_first_byte' in snapshot.timing, 'missing: timing.time_to_first_byte')
    assert.ok('dom_interactive' in snapshot.timing, 'missing: timing.dom_interactive')

    // network fields from spec
    assert.ok('request_count' in snapshot.network, 'missing: network.request_count')
    assert.ok('transfer_size' in snapshot.network, 'missing: network.transfer_size')
    assert.ok('decoded_size' in snapshot.network, 'missing: network.decoded_size')
    assert.ok('by_type' in snapshot.network, 'missing: network.by_type')
    assert.ok('slowest_requests' in snapshot.network, 'missing: network.slowest_requests')

    // longTasks fields from spec
    assert.ok('count' in snapshot.long_tasks, 'missing: long_tasks.count')
    assert.ok('total_blocking_time' in snapshot.long_tasks, 'missing: long_tasks.total_blocking_time')
    assert.ok('longest' in snapshot.long_tasks, 'missing: long_tasks.longest')
  })

  test('capturePerformanceSnapshot collects navigation timing', async () => {
    const { capturePerformanceSnapshot } = await import('../../extension/lib/analysis/perf-snapshot.js')

    const snapshot = capturePerformanceSnapshot()

    assert.ok(snapshot, 'Snapshot should not be null')
    assert.strictEqual(snapshot.url, '/dashboard')
    assert.strictEqual(snapshot.timing.dom_content_loaded, 600)
    assert.strictEqual(snapshot.timing.load, 1200)
    assert.strictEqual(snapshot.timing.time_to_first_byte, 80) // 180 - 100
    assert.strictEqual(snapshot.timing.dom_interactive, 500)
  })

  test('capturePerformanceSnapshot returns null when no navigation entry', async () => {
    globalThis.performance.getEntriesByType = mock.fn(() => [])

    const { capturePerformanceSnapshot } = await import('../../extension/lib/analysis/perf-snapshot.js')
    const snapshot = capturePerformanceSnapshot()

    assert.strictEqual(snapshot, null)
  })

  test('capturePerformanceSnapshot includes timestamp and url', async () => {
    const { capturePerformanceSnapshot } = await import('../../extension/lib/analysis/perf-snapshot.js')

    const snapshot = capturePerformanceSnapshot()

    assert.ok(snapshot.timestamp, 'Should have timestamp')
    assert.strictEqual(snapshot.url, '/dashboard')
    // Timestamp should be ISO format
    assert.ok(snapshot.timestamp.includes('T'), 'Timestamp should be ISO format')
  })

  test('aggregateResourceTiming groups by initiator type', async () => {
    const { aggregateResourceTiming } = await import('../../extension/lib/analysis/perf-snapshot.js')

    const result = aggregateResourceTiming()

    assert.strictEqual(result.request_count, 4)
    assert.strictEqual(result.by_type.script.count, 1)
    assert.strictEqual(result.by_type.script.size, 50000)
    assert.strictEqual(result.by_type.style.count, 1)
    assert.strictEqual(result.by_type.style.size, 10000)
    assert.strictEqual(result.by_type.image.count, 1)
    assert.strictEqual(result.by_type.image.size, 80000)
    assert.strictEqual(result.by_type.fetch.count, 1)
    assert.strictEqual(result.by_type.fetch.size, 2000)
  })

  test('aggregateResourceTiming calculates total sizes', async () => {
    const { aggregateResourceTiming } = await import('../../extension/lib/analysis/perf-snapshot.js')

    const result = aggregateResourceTiming()

    assert.strictEqual(result.transfer_size, 142000) // 50000 + 10000 + 80000 + 2000
    assert.strictEqual(result.decoded_size, 205000) // 100000 + 20000 + 80000 + 5000
  })

  test('aggregateResourceTiming returns top 3 slowest requests', async () => {
    const { aggregateResourceTiming } = await import('../../extension/lib/analysis/perf-snapshot.js')

    const result = aggregateResourceTiming()

    assert.strictEqual(result.slowest_requests.length, 3)
    // Should be sorted by duration descending
    assert.ok(result.slowest_requests[0].duration >= result.slowest_requests[1].duration)
    assert.ok(result.slowest_requests[1].duration >= result.slowest_requests[2].duration)
    // First should be the image (500ms)
    assert.strictEqual(result.slowest_requests[0].duration, 500)
  })

  test('aggregateResourceTiming truncates URLs to 80 chars', async () => {
    const longUrl = 'http://localhost/' + 'a'.repeat(100)
    globalThis.performance.getEntriesByType = mock.fn((type) => {
      if (type === 'resource') {
        return [{ initiatorType: 'script', transferSize: 1000, decodedBodySize: 2000, duration: 100, name: longUrl }]
      }
      return []
    })

    const { aggregateResourceTiming } = await import('../../extension/lib/analysis/perf-snapshot.js')
    const result = aggregateResourceTiming()

    assert.ok(result.slowest_requests[0].url.length <= 80)
  })

  test('mapInitiatorType maps known types correctly', async () => {
    const { mapInitiatorType } = await import('../../extension/lib/analysis/perf-snapshot.js')

    assert.strictEqual(mapInitiatorType('script'), 'script')
    assert.strictEqual(mapInitiatorType('link'), 'style')
    assert.strictEqual(mapInitiatorType('css'), 'style')
    assert.strictEqual(mapInitiatorType('img'), 'image')
    assert.strictEqual(mapInitiatorType('fetch'), 'fetch')
    assert.strictEqual(mapInitiatorType('xmlhttprequest'), 'fetch')
    assert.strictEqual(mapInitiatorType('font'), 'font')
    assert.strictEqual(mapInitiatorType('video'), 'other')
    assert.strictEqual(mapInitiatorType('unknown'), 'other')
  })
})

describe('Long Task Observer', () => {
  beforeEach(() => {
    originalWindow = globalThis.window
    originalPerformance = globalThis.performance
    originalPerformanceObserver = globalThis.PerformanceObserver
    MockPerformanceObserver.instances = []
    globalThis.window = createMockWindow({
      href: 'http://localhost:3000/dashboard',
      pathname: '/dashboard'
    })
    globalThis.performance = createMockPerformance()
    globalThis.PerformanceObserver = MockPerformanceObserver
  })

  afterEach(() => {
    globalThis.window = originalWindow
    globalThis.performance = originalPerformance
    globalThis.PerformanceObserver = originalPerformanceObserver
  })

  test('installPerfObservers creates longtask observer', async () => {
    const { installPerfObservers, uninstallPerfObservers } = await import('../../extension/lib/analysis/perf-snapshot.js')

    installPerfObservers()

    const longTaskObserver = MockPerformanceObserver.instances.find((obs) => obs.observedTypes.includes('longtask'))
    assert.ok(longTaskObserver, 'Should have created a longtask observer')

    uninstallPerfObservers()
  })

  test('getLongTaskMetrics returns accumulated long tasks', async () => {
    const { installPerfObservers, getLongTaskMetrics, uninstallPerfObservers } =
      await import('../../extension/lib/analysis/perf-snapshot.js')

    installPerfObservers()

    // Simulate long task entries
    const longTaskObserver = MockPerformanceObserver.instances.find((obs) => obs.observedTypes.includes('longtask'))
    assert.ok(longTaskObserver)

    // Call the observer callback with mock entries
    longTaskObserver.callback({
      getEntries: () => [
        { duration: 120, startTime: 500 },
        { duration: 80, startTime: 700 }
      ]
    })

    const metrics = getLongTaskMetrics()

    assert.strictEqual(metrics.count, 2)
    // TBT = sum of (duration - 50) for each: (120-50) + (80-50) = 70 + 30 = 100
    assert.strictEqual(metrics.total_blocking_time, 100)
    assert.strictEqual(metrics.longest, 120)

    uninstallPerfObservers()
  })

  test('getLongTaskMetrics caps at 50 entries', async () => {
    const { installPerfObservers, getLongTaskMetrics, uninstallPerfObservers } =
      await import('../../extension/lib/analysis/perf-snapshot.js')

    installPerfObservers()

    const longTaskObserver = MockPerformanceObserver.instances.find((obs) => obs.observedTypes.includes('longtask'))

    // Add 60 entries
    const entries = Array.from({ length: 60 }, (_, i) => ({
      duration: 60 + i,
      startTime: i * 100
    }))
    longTaskObserver.callback({ getEntries: () => entries })

    const metrics = getLongTaskMetrics()
    assert.strictEqual(metrics.count, 50) // Capped at 50

    uninstallPerfObservers()
  })
})

describe('Web Vitals Observers', () => {
  beforeEach(() => {
    originalWindow = globalThis.window
    originalPerformance = globalThis.performance
    originalPerformanceObserver = globalThis.PerformanceObserver
    MockPerformanceObserver.instances = []
    globalThis.window = createMockWindow({
      href: 'http://localhost:3000/dashboard',
      pathname: '/dashboard'
    })
    globalThis.performance = createMockPerformance()
    globalThis.PerformanceObserver = MockPerformanceObserver
  })

  afterEach(() => {
    globalThis.window = originalWindow
    globalThis.performance = originalPerformance
    globalThis.PerformanceObserver = originalPerformanceObserver
  })

  test('installPerfObservers creates paint observer for FCP', async () => {
    const { installPerfObservers, uninstallPerfObservers } = await import('../../extension/lib/analysis/perf-snapshot.js')

    installPerfObservers()

    const paintObserver = MockPerformanceObserver.instances.find((obs) => obs.observedTypes.includes('paint'))
    assert.ok(paintObserver, 'Should have created a paint observer')

    uninstallPerfObservers()
  })

  test('FCP value is captured from paint observer', async () => {
    const { installPerfObservers, getFCP, uninstallPerfObservers } = await import('../../extension/lib/analysis/perf-snapshot.js')

    installPerfObservers()

    const paintObserver = MockPerformanceObserver.instances.find((obs) => obs.observedTypes.includes('paint'))
    paintObserver.callback({
      getEntries: () => [
        { name: 'first-paint', startTime: 100 },
        { name: 'first-contentful-paint', startTime: 250 }
      ]
    })

    assert.strictEqual(getFCP(), 250)

    uninstallPerfObservers()
  })

  test('LCP value captured from largest-contentful-paint observer', async () => {
    const { installPerfObservers, getLCP, uninstallPerfObservers } = await import('../../extension/lib/analysis/perf-snapshot.js')

    installPerfObservers()

    const lcpObserver = MockPerformanceObserver.instances.find((obs) =>
      obs.observedTypes.includes('largest-contentful-paint')
    )
    assert.ok(lcpObserver, 'Should have created an LCP observer')

    lcpObserver.callback({
      getEntries: () => [
        { startTime: 500 },
        { startTime: 800 } // Last one wins
      ]
    })

    assert.strictEqual(getLCP(), 800)

    uninstallPerfObservers()
  })

  test('CLS accumulated from layout-shift observer', async () => {
    const { installPerfObservers, getCLS, uninstallPerfObservers } = await import('../../extension/lib/analysis/perf-snapshot.js')

    installPerfObservers()

    const clsObserver = MockPerformanceObserver.instances.find((obs) => obs.observedTypes.includes('layout-shift'))
    assert.ok(clsObserver, 'Should have created a layout-shift observer')

    clsObserver.callback({
      getEntries: () => [
        { value: 0.05, hadRecentInput: false },
        { value: 0.03, hadRecentInput: false },
        { value: 0.1, hadRecentInput: true } // Should be ignored
      ]
    })

    // 0.05 + 0.03 = 0.08 (the 0.1 is ignored because hadRecentInput)
    const cls = getCLS()
    assert.ok(Math.abs(cls - 0.08) < 0.001, `Expected CLS ~0.08, got ${cls}`)

    uninstallPerfObservers()
  })

  test('CLS ignores entries with hadRecentInput', async () => {
    const { installPerfObservers, getCLS, uninstallPerfObservers } = await import('../../extension/lib/analysis/perf-snapshot.js')

    installPerfObservers()

    const clsObserver = MockPerformanceObserver.instances.find((obs) => obs.observedTypes.includes('layout-shift'))

    clsObserver.callback({
      getEntries: () => [{ value: 0.5, hadRecentInput: true }]
    })

    assert.strictEqual(getCLS(), 0)

    uninstallPerfObservers()
  })
})

describe('Performance Snapshot Message', () => {
  beforeEach(() => {
    originalWindow = globalThis.window
    originalPerformance = globalThis.performance
    originalPerformanceObserver = globalThis.PerformanceObserver
    MockPerformanceObserver.instances = []
    globalThis.window = createMockWindow({
      href: 'http://localhost:3000/dashboard',
      pathname: '/dashboard'
    })
    globalThis.performance = createMockPerformance()
    globalThis.PerformanceObserver = MockPerformanceObserver
  })

  afterEach(() => {
    globalThis.window = originalWindow
    globalThis.performance = originalPerformance
    globalThis.PerformanceObserver = originalPerformanceObserver
  })

  test('sendPerformanceSnapshot posts correct message type', async () => {
    const { sendPerformanceSnapshot } = await import('../../extension/lib/analysis/perf-snapshot.js')

    sendPerformanceSnapshot()

    assert.strictEqual(globalThis.window.postMessage.mock.calls.length, 1)
    const [message, origin] = globalThis.window.postMessage.mock.calls[0].arguments
    assert.strictEqual(message.type, 'kaboom_performance_snapshot')
    assert.strictEqual(origin, 'http://localhost:3000')
    assert.ok(message.payload, 'Should have payload')
    assert.strictEqual(message.payload.url, '/dashboard')
  })

  test('sendPerformanceSnapshot includes all sections', async () => {
    const { sendPerformanceSnapshot } = await import('../../extension/lib/analysis/perf-snapshot.js')

    sendPerformanceSnapshot()

    const { payload } = globalThis.window.postMessage.mock.calls[0].arguments[0]
    assert.ok(payload.timing, 'Should have timing')
    assert.ok(payload.network, 'Should have network')
    assert.ok(payload.long_tasks, 'Should have long_tasks')
    assert.ok(payload.timestamp, 'Should have timestamp')
    assert.ok('cumulative_layout_shift' in payload, 'Should have cumulative_layout_shift')
    assert.ok('first_contentful_paint' in payload.timing, 'Should have timing.first_contentful_paint')
    assert.ok('largest_contentful_paint' in payload.timing, 'Should have timing.largest_contentful_paint')
  })

  test('sendPerformanceSnapshot does nothing when no navigation entry', async () => {
    globalThis.performance.getEntriesByType = mock.fn(() => [])

    const { sendPerformanceSnapshot } = await import('../../extension/lib/analysis/perf-snapshot.js')
    sendPerformanceSnapshot()

    assert.strictEqual(globalThis.window.postMessage.mock.calls.length, 0)
  })
})

describe('Performance Snapshot Toggle', () => {
  beforeEach(() => {
    originalWindow = globalThis.window
    originalPerformance = globalThis.performance
    originalPerformanceObserver = globalThis.PerformanceObserver
    MockPerformanceObserver.instances = []
    globalThis.window = createMockWindow({
      href: 'http://localhost:3000/dashboard',
      pathname: '/dashboard'
    })
    globalThis.performance = createMockPerformance()
    globalThis.PerformanceObserver = MockPerformanceObserver
  })

  afterEach(() => {
    globalThis.window = originalWindow
    globalThis.performance = originalPerformance
    globalThis.PerformanceObserver = originalPerformanceObserver
  })

  test('performance snapshot is enabled by default', async () => {
    const { isPerformanceSnapshotEnabled } = await import('../../extension/lib/analysis/perf-snapshot.js')
    assert.strictEqual(isPerformanceSnapshotEnabled(), true)
  })

  test('setPerformanceSnapshotEnabled can disable capture', async () => {
    const { setPerformanceSnapshotEnabled, isPerformanceSnapshotEnabled } = await import('../../extension/lib/analysis/perf-snapshot.js')

    setPerformanceSnapshotEnabled(false)
    assert.strictEqual(isPerformanceSnapshotEnabled(), false)

    // Re-enable for other tests
    setPerformanceSnapshotEnabled(true)
  })
})
