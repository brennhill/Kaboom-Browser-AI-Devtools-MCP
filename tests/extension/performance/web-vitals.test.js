// @ts-nocheck
/**
 * @fileoverview web-vitals.test.js — Tests for Web Vitals capture via PerformanceObserver.
 * Verifies the exported API (installPerfObservers, uninstallPerfObservers,
 * getFCP, getLCP, getCLS, sendPerformanceSnapshot) and validates that Core Web
 * Vitals metrics are correctly collected, stored, and included in snapshots.
 */

import { test, describe, mock, beforeEach, afterEach } from 'node:test'
import assert from 'node:assert'

// Mock PerformanceObserver
class MockPerformanceObserver {
  constructor(callback) {
    MockPerformanceObserver._instances.push(this)
    this._callback = callback
    this._types = []
  }
  observe(options) {
    if (options.type) this._types.push(options.type)
  }
  disconnect() {
    this._disconnected = true
  }
  // Helper to simulate entries
  _emit(entries) {
    this._callback({ getEntries: () => entries })
  }
}
MockPerformanceObserver._instances = []
MockPerformanceObserver.supportedEntryTypes = ['paint', 'largest-contentful-paint', 'layout-shift', 'longtask', 'event']

let originalWindow, originalPerformanceObserver, originalPerformance

describe('Web Vitals Capture', () => {
  beforeEach(() => {
    MockPerformanceObserver._instances = []
    originalWindow = globalThis.window
    originalPerformanceObserver = globalThis.PerformanceObserver
    originalPerformance = globalThis.performance

    globalThis.PerformanceObserver = MockPerformanceObserver
    globalThis.performance = {
      getEntriesByType: mock.fn(() => []),
      now: () => 1000,
      mark: mock.fn((name, _options) => ({ name, startTime: 1000, entryType: 'mark' })),
      measure: mock.fn((name, _startMark, _endMark) => ({ name, duration: 0, startTime: 1000, entryType: 'measure' }))
    }
    globalThis.window = {
      postMessage: mock.fn(),
      addEventListener: mock.fn(),
      removeEventListener: mock.fn(),
      location: { href: 'http://localhost:3000/test' },
      history: { pushState: mock.fn(), replaceState: mock.fn() },
      onerror: null,
      onunhandledrejection: null,
      WebSocket: class {
        addEventListener() {}
      }
    }
    globalThis.document = {
      readyState: 'complete',
      addEventListener: mock.fn(),
      querySelector: () => null,
      querySelectorAll: () => []
    }
    if (!globalThis.console) {
      globalThis.console = {
        log: mock.fn(),
        warn: mock.fn(),
        error: mock.fn(),
        info: mock.fn(),
        debug: mock.fn()
      }
    }
  })

  afterEach(() => {
    globalThis.window = originalWindow
    globalThis.PerformanceObserver = originalPerformanceObserver
    globalThis.performance = originalPerformance
    delete globalThis.document
  })

  test('installPerfObservers creates observers for paint, LCP, CLS, longtask', async () => {
    const mod = await import('../../../extension/lib/analysis/perf-snapshot.js')
    mod.installPerfObservers()

    const types = MockPerformanceObserver._instances.flatMap((obs) => obs._types)
    assert.ok(types.includes('paint'), 'Should observe paint entries (FCP)')
    assert.ok(types.includes('largest-contentful-paint'), 'Should observe LCP entries')
    assert.ok(types.includes('layout-shift'), 'Should observe CLS entries')
    assert.ok(types.includes('longtask'), 'Should observe long tasks')
  })

  test('FCP is captured from paint entries', async () => {
    const mod = await import('../../../extension/lib/analysis/perf-snapshot.js')
    mod.installPerfObservers()

    const paintObs = MockPerformanceObserver._instances.find((obs) => obs._types.includes('paint'))
    assert.ok(paintObs, 'Paint observer should exist')

    paintObs._emit([{ name: 'first-contentful-paint', startTime: 1200 }])

    assert.strictEqual(mod.getFCP(), 1200)
  })

  test('LCP updates to latest entry', async () => {
    const mod = await import('../../../extension/lib/analysis/perf-snapshot.js')
    mod.installPerfObservers()

    const lcpObs = MockPerformanceObserver._instances.find((obs) => obs._types.includes('largest-contentful-paint'))
    assert.ok(lcpObs, 'LCP observer should exist')

    lcpObs._emit([
      { startTime: 1000, element: { tagName: 'DIV' }, size: 5000 },
      { startTime: 2400, element: { tagName: 'IMG' }, size: 150000 }
    ])

    // Implementation keeps the last entry's startTime
    assert.strictEqual(mod.getLCP(), 2400)
  })

  test('CLS accumulates layout shifts (ignores input-driven)', async () => {
    const mod = await import('../../../extension/lib/analysis/perf-snapshot.js')
    mod.installPerfObservers()

    const clsObs = MockPerformanceObserver._instances.find((obs) => obs._types.includes('layout-shift'))
    assert.ok(clsObs, 'CLS observer should exist')

    clsObs._emit([
      { value: 0.02, hadRecentInput: false },
      { value: 0.03, hadRecentInput: false },
      { value: 0.1, hadRecentInput: true } // Should be ignored
    ])

    // Only non-input shifts are accumulated: 0.02 + 0.03 = 0.05
    const cls = mod.getCLS()
    assert.ok(Math.abs(cls - 0.05) < 0.001, `CLS should be ~0.05, got ${cls}`)
  })

  test('installPerfObservers propagates observer errors', async () => {
    let callCount = 0
    globalThis.PerformanceObserver = class {
      constructor(cb) {
        this._cb = cb
      }
      observe() {
        callCount++
        if (callCount === 1) throw new Error('Not supported')
      }
      disconnect() {}
    }
    globalThis.PerformanceObserver.supportedEntryTypes = []

    const mod = await import('../../../extension/lib/analysis/perf-snapshot.js')
    // Observer errors propagate (not silently swallowed)
    assert.throws(() => mod.installPerfObservers(), { message: 'Not supported' })
  })

  test('installPerfObservers resets values', async () => {
    const mod = await import('../../../extension/lib/analysis/perf-snapshot.js')
    mod.installPerfObservers()

    // Simulate FCP
    const paintObs = MockPerformanceObserver._instances.find((obs) => obs._types.includes('paint'))
    if (paintObs) paintObs._emit([{ name: 'first-contentful-paint', startTime: 500 }])
    assert.strictEqual(mod.getFCP(), 500)

    // Re-install should reset values
    mod.installPerfObservers()
    assert.strictEqual(mod.getFCP(), null)
    assert.strictEqual(mod.getLCP(), null)
    assert.strictEqual(mod.getCLS(), 0)
  })

  test('uninstallPerfObservers disconnects all observers', async () => {
    const mod = await import('../../../extension/lib/analysis/perf-snapshot.js')
    mod.installPerfObservers()

    const observersBefore = MockPerformanceObserver._instances.filter((obs) => !obs._disconnected)
    assert.ok(observersBefore.length > 0, 'Should have active observers')

    mod.uninstallPerfObservers()

    const observersAfter = MockPerformanceObserver._instances.filter((obs) => obs._disconnected)
    assert.ok(observersAfter.length > 0, 'All observers should be disconnected')
  })

  test('getFCP returns null before any paint entry', async () => {
    const mod = await import('../../../extension/lib/analysis/perf-snapshot.js')
    mod.installPerfObservers()
    assert.strictEqual(mod.getFCP(), null)
  })

  test('getLCP returns null before any LCP entry', async () => {
    const mod = await import('../../../extension/lib/analysis/perf-snapshot.js')
    mod.installPerfObservers()
    assert.strictEqual(mod.getLCP(), null)
  })

  test('getCLS returns 0 before any layout shift', async () => {
    const mod = await import('../../../extension/lib/analysis/perf-snapshot.js')
    mod.installPerfObservers()
    assert.strictEqual(mod.getCLS(), 0)
  })

  test('INP observer is created for event type entries when installPerfObservers is called', async () => {
    const mod = await import('../../../extension/lib/analysis/perf-snapshot.js')
    mod.installPerfObservers()

    const types = MockPerformanceObserver._instances.flatMap((obs) => obs._types)
    assert.ok(types.includes('event'), 'Should observe event entries (INP)')
  })

  test('getINP returns null before any interaction', async () => {
    const mod = await import('../../../extension/lib/analysis/perf-snapshot.js')
    mod.installPerfObservers()
    assert.strictEqual(mod.getINP(), null)
  })

  test('INP captures the highest duration from event entries with interactionId', async () => {
    const mod = await import('../../../extension/lib/analysis/perf-snapshot.js')
    mod.installPerfObservers()

    const eventObs = MockPerformanceObserver._instances.find((obs) => obs._types.includes('event'))
    assert.ok(eventObs, 'Event observer should exist')

    eventObs._emit([
      { duration: 120, interactionId: 1 },
      { duration: 200, interactionId: 2 },
      { duration: 80, interactionId: 3 }
    ])

    assert.strictEqual(mod.getINP(), 200)
  })

  test('INP ignores entries without interactionId', async () => {
    const mod = await import('../../../extension/lib/analysis/perf-snapshot.js')
    mod.installPerfObservers()

    const eventObs = MockPerformanceObserver._instances.find((obs) => obs._types.includes('event'))
    assert.ok(eventObs, 'Event observer should exist')

    eventObs._emit([
      { duration: 500, interactionId: 0 }, // falsy interactionId
      { duration: 300 }, // no interactionId
      { duration: 100, interactionId: 1 } // valid
    ])

    assert.strictEqual(mod.getINP(), 100)
  })

  test('INP updates when a higher duration interaction occurs', async () => {
    const mod = await import('../../../extension/lib/analysis/perf-snapshot.js')
    mod.installPerfObservers()

    const eventObs = MockPerformanceObserver._instances.find((obs) => obs._types.includes('event'))
    assert.ok(eventObs, 'Event observer should exist')

    eventObs._emit([{ duration: 150, interactionId: 1 }])
    assert.strictEqual(mod.getINP(), 150)

    eventObs._emit([{ duration: 300, interactionId: 2 }])
    assert.strictEqual(mod.getINP(), 300)

    // Lower duration should not update
    eventObs._emit([{ duration: 100, interactionId: 3 }])
    assert.strictEqual(mod.getINP(), 300)
  })

  test('uninstallPerfObservers disconnects INP observer', async () => {
    const mod = await import('../../../extension/lib/analysis/perf-snapshot.js')
    mod.installPerfObservers()

    const eventObs = MockPerformanceObserver._instances.find((obs) => obs._types.includes('event'))
    assert.ok(eventObs, 'Event observer should exist')
    assert.ok(!eventObs._disconnected, 'Should not be disconnected before uninstall')

    mod.uninstallPerfObservers()
    assert.ok(eventObs._disconnected, 'INP observer should be disconnected after uninstall')
  })

  test('installPerfObservers resets INP value', async () => {
    const mod = await import('../../../extension/lib/analysis/perf-snapshot.js')
    mod.installPerfObservers()

    const eventObs = MockPerformanceObserver._instances.find((obs) => obs._types.includes('event'))
    if (eventObs) eventObs._emit([{ duration: 250, interactionId: 1 }])
    assert.strictEqual(mod.getINP(), 250)

    // Re-install should reset INP
    mod.installPerfObservers()
    assert.strictEqual(mod.getINP(), null)
  })
})

describe('Performance Snapshot Message Flow', () => {
  beforeEach(() => {
    MockPerformanceObserver._instances = []
    originalWindow = globalThis.window
    originalPerformanceObserver = globalThis.PerformanceObserver
    originalPerformance = globalThis.performance

    globalThis.PerformanceObserver = MockPerformanceObserver
    globalThis.performance = {
      getEntriesByType: mock.fn(() => []),
      getEntries: mock.fn(() => []),
      now: () => 1000
    }
    globalThis.window = {
      postMessage: mock.fn(),
      addEventListener: mock.fn(),
      removeEventListener: mock.fn(),
      location: { href: 'http://localhost:3000/test' },
      history: { pushState: mock.fn(), replaceState: mock.fn() },
      onerror: null,
      onunhandledrejection: null,
      WebSocket: class {
        addEventListener() {}
      }
    }
    globalThis.document = {
      readyState: 'complete',
      addEventListener: mock.fn(),
      querySelector: () => null,
      querySelectorAll: () => []
    }
    if (!globalThis.console) {
      globalThis.console = {
        log: mock.fn(),
        warn: mock.fn(),
        error: mock.fn(),
        info: mock.fn(),
        debug: mock.fn()
      }
    }
  })

  afterEach(() => {
    globalThis.window = originalWindow
    globalThis.PerformanceObserver = originalPerformanceObserver
    globalThis.performance = originalPerformance
    delete globalThis.document
  })

  test('sendPerformanceSnapshot posts kaboom_performance_snapshot message', async () => {
    // capturePerformanceSnapshot requires a navigation entry
    globalThis.performance.getEntriesByType = mock.fn((type) => {
      if (type === 'navigation') {
        return [
          {
            domContentLoadedEventEnd: 200,
            loadEventEnd: 500,
            responseStart: 80,
            requestStart: 10,
            domInteractive: 150
          }
        ]
      }
      return []
    })
    globalThis.performance.getEntries = mock.fn(() => [])

    const mod = await import('../../../extension/lib/analysis/perf-snapshot.js')
    mod.setPerformanceSnapshotEnabled(true)
    mod.installPerfObservers()

    // Simulate FCP capture
    const paintObs = MockPerformanceObserver._instances.find((obs) => obs._types.includes('paint'))
    if (paintObs) paintObs._emit([{ name: 'first-contentful-paint', startTime: 800 }])

    mod.sendPerformanceSnapshot()

    const calls = globalThis.window.postMessage.mock.calls
    const snapshotMessage = calls.find((c) => c.arguments[0]?.type === 'kaboom_performance_snapshot')
    assert.ok(snapshotMessage, 'Should post kaboom_performance_snapshot message')
    assert.ok(snapshotMessage.arguments[0].payload.timing, 'Payload should include timing')
  })

  test('sendPerformanceSnapshot includes INP in the payload', async () => {
    globalThis.performance.getEntriesByType = mock.fn((type) => {
      if (type === 'navigation') {
        return [
          {
            domContentLoadedEventEnd: 200,
            loadEventEnd: 500,
            responseStart: 80,
            requestStart: 10,
            domInteractive: 150
          }
        ]
      }
      return []
    })
    globalThis.performance.getEntries = mock.fn(() => [])

    const mod = await import('../../../extension/lib/analysis/perf-snapshot.js')
    mod.setPerformanceSnapshotEnabled(true)
    mod.installPerfObservers()

    // Simulate INP capture
    const eventObs = MockPerformanceObserver._instances.find((obs) => obs._types.includes('event'))
    if (eventObs) eventObs._emit([{ duration: 175, interactionId: 1 }])

    mod.sendPerformanceSnapshot()

    const calls = globalThis.window.postMessage.mock.calls
    const snapshotMessage = calls.find((c) => c.arguments[0]?.type === 'kaboom_performance_snapshot')
    assert.ok(snapshotMessage, 'Should post kaboom_performance_snapshot message')
    assert.strictEqual(
      snapshotMessage.arguments[0].payload.timing.interaction_to_next_paint,
      175,
      'Payload timing should include interaction_to_next_paint'
    )
  })

  test('sendPerformanceSnapshot does nothing when disabled', async () => {
    const mod = await import('../../../extension/lib/analysis/perf-snapshot.js')
    mod.setPerformanceSnapshotEnabled(false)

    mod.sendPerformanceSnapshot()

    const calls = globalThis.window.postMessage.mock.calls
    const snapshotMessage = calls.find((c) => c.arguments[0]?.type === 'kaboom_performance_snapshot')
    assert.strictEqual(snapshotMessage, undefined, 'Should not post when disabled')
  })
})
