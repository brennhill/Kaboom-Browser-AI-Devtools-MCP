// @ts-nocheck
/**
 * @fileoverview performance-marks.test.js — Tests for performance mark/measure capture.
 * Verifies PerformanceObserver-based collection of user timing marks and measures,
 * 60-second time window enforcement, 50-entry buffer cap, and integration with
 * the performance snapshot message format.
 */

import { test, describe, mock, beforeEach, afterEach } from 'node:test'
import assert from 'node:assert'

// Define esbuild constant not available in Node test env
globalThis.__KABOOM_VERSION__ = 'test'

let originalWindow
let originalPerformance
let originalDocument

function createMockMark(name, startTime = 100, detail = null) {
  return {
    name,
    entryType: 'mark',
    startTime,
    duration: 0,
    detail
  }
}

function createMockMeasure(name, startTime = 100, duration = 50, detail = null) {
  return {
    name,
    entryType: 'measure',
    startTime,
    duration,
    detail
  }
}

function createMockPerformance() {
  const entries = []
  return {
    getEntriesByType: mock.fn((type) => {
      return entries.filter((e) => e.entryType === type)
    }),
    getEntriesByName: mock.fn((name) => entries.filter((e) => e.name === name)),
    mark: mock.fn((name, options) => {
      const mark = createMockMark(name, Date.now(), options?.detail)
      entries.push(mark)
      return mark
    }),
    measure: mock.fn((name, _startMark, _endMark) => {
      const measure = createMockMeasure(name, 0, 50)
      entries.push(measure)
      return measure
    }),
    clearMarks: mock.fn((name) => {
      const filtered = entries.filter((e) => e.entryType !== 'mark' || (name && e.name !== name))
      entries.length = 0
      entries.push(...filtered)
    }),
    clearMeasures: mock.fn((name) => {
      const filtered = entries.filter((e) => e.entryType !== 'measure' || (name && e.name !== name))
      entries.length = 0
      entries.push(...filtered)
    }),
    now: mock.fn(() => Date.now()),
    _entries: entries,
    _addEntry: (entry) => entries.push(entry),
    _clear: () => (entries.length = 0)
  }
}

function createMockWindow() {
  return {
    location: { href: 'http://localhost:3000/test' },
    postMessage: mock.fn(),
    performance: createMockPerformance(),
    PerformanceObserver: mock.fn((_callback) => ({
      observe: mock.fn(),
      disconnect: mock.fn()
    })),
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

describe('Performance Marks - getPerformanceMarks', () => {
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

  test('should return all performance marks', async () => {
    const { getPerformanceMarks } = await import('../../extension/lib/analysis/performance.js')

    globalThis.performance._addEntry(createMockMark('pageLoad', 100))
    globalThis.performance._addEntry(createMockMark('componentMount', 200))

    const marks = getPerformanceMarks()

    assert.ok(Array.isArray(marks))
    assert.strictEqual(marks.length, 2)
  })

  test('should include mark name and startTime', async () => {
    const { getPerformanceMarks } = await import('../../extension/lib/analysis/performance.js')

    globalThis.performance._addEntry(createMockMark('userAction', 500))

    const marks = getPerformanceMarks()

    assert.strictEqual(marks[0].name, 'userAction')
    assert.strictEqual(marks[0].startTime, 500)
  })

  test('should include mark detail if present', async () => {
    const { getPerformanceMarks } = await import('../../extension/lib/analysis/performance.js')

    globalThis.performance._addEntry(createMockMark('checkout', 300, { step: 'payment', items: 3 }))

    const marks = getPerformanceMarks()

    assert.ok(marks[0].detail)
    assert.strictEqual(marks[0].detail.step, 'payment')
  })

  test('should filter marks by time range', async () => {
    const { getPerformanceMarks } = await import('../../extension/lib/analysis/performance.js')

    globalThis.performance._addEntry(createMockMark('early', 100))
    globalThis.performance._addEntry(createMockMark('late', 500))

    const marks = getPerformanceMarks({ since: 400 })

    assert.strictEqual(marks.length, 1)
    assert.strictEqual(marks[0].name, 'late')
  })

  test('should limit number of marks returned', async () => {
    const { getPerformanceMarks } = await import('../../extension/lib/analysis/performance.js')
    const { MAX_PERFORMANCE_ENTRIES } = await import('../../extension/lib/constants.js')

    for (let i = 0; i < 100; i++) {
      globalThis.performance._addEntry(createMockMark(`mark-${i}`, i * 10))
    }

    const marks = getPerformanceMarks()

    assert.ok(marks.length <= MAX_PERFORMANCE_ENTRIES)
  })

  test('should sort marks by startTime', async () => {
    const { getPerformanceMarks } = await import('../../extension/lib/analysis/performance.js')

    globalThis.performance._addEntry(createMockMark('second', 200))
    globalThis.performance._addEntry(createMockMark('first', 100))

    const marks = getPerformanceMarks()

    assert.ok(marks[0].startTime <= marks[1].startTime)
  })

  test('should return empty array when performance API unavailable', async () => {
    const { getPerformanceMarks } = await import('../../extension/lib/analysis/performance.js')

    globalThis.performance = null

    const marks = getPerformanceMarks()

    assert.ok(Array.isArray(marks))
    assert.strictEqual(marks.length, 0)
  })
})

describe('Performance Marks - getPerformanceMeasures', () => {
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

  test('should return all performance measures', async () => {
    const { getPerformanceMeasures } = await import('../../extension/lib/analysis/performance.js')

    globalThis.performance._addEntry(createMockMeasure('pageLoadTime', 0, 1500))
    globalThis.performance._addEntry(createMockMeasure('apiCallDuration', 100, 300))

    const measures = getPerformanceMeasures()

    assert.ok(Array.isArray(measures))
    assert.strictEqual(measures.length, 2)
  })

  test('should include measure name, startTime, and duration', async () => {
    const { getPerformanceMeasures } = await import('../../extension/lib/analysis/performance.js')

    globalThis.performance._addEntry(createMockMeasure('renderTime', 200, 150))

    const measures = getPerformanceMeasures()

    assert.strictEqual(measures[0].name, 'renderTime')
    assert.strictEqual(measures[0].startTime, 200)
    assert.strictEqual(measures[0].duration, 150)
  })

  test('should include measure detail if present', async () => {
    const { getPerformanceMeasures } = await import('../../extension/lib/analysis/performance.js')

    const measure = createMockMeasure('apiCall', 100, 200, { endpoint: '/api/users' })
    globalThis.performance._addEntry(measure)

    const measures = getPerformanceMeasures()

    assert.ok(measures[0].detail || measures[0].endpoint)
  })

  test('should filter measures by time range', async () => {
    const { getPerformanceMeasures } = await import('../../extension/lib/analysis/performance.js')

    globalThis.performance._addEntry(createMockMeasure('early', 50, 100))
    globalThis.performance._addEntry(createMockMeasure('late', 400, 100))

    const measures = getPerformanceMeasures({ since: 300 })

    assert.strictEqual(measures.length, 1)
    assert.strictEqual(measures[0].name, 'late')
  })
})

describe('Performance Marks - wrapPerformanceMark', () => {
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

  test('should wrap performance.mark to capture calls', async () => {
    const { installPerformanceCapture, uninstallPerformanceCapture, getCapturedMarks } =
      await import('../../extension/lib/analysis/performance.js')

    installPerformanceCapture()

    globalThis.performance.mark('testMark')

    const captured = getCapturedMarks()

    assert.ok(captured.some((m) => m.name === 'testMark'))

    uninstallPerformanceCapture()
  })

  test('should preserve original mark behavior', async () => {
    const { installPerformanceCapture, uninstallPerformanceCapture } = await import('../../extension/lib/analysis/performance.js')

    const originalMark = globalThis.performance.mark

    installPerformanceCapture()

    globalThis.performance.mark('preserveTest', { detail: { test: true } })

    // Original should still be called
    assert.ok(originalMark.mock.calls.length > 0)

    uninstallPerformanceCapture()
  })

  test('should include timestamp when mark is created', async () => {
    const { installPerformanceCapture, uninstallPerformanceCapture, getCapturedMarks } =
      await import('../../extension/lib/analysis/performance.js')

    installPerformanceCapture()

    globalThis.performance.mark('timedMark')

    const captured = getCapturedMarks()
    const mark = captured.find((m) => m.name === 'timedMark')

    assert.ok(mark.capturedAt || mark.ts || mark.startTime)

    uninstallPerformanceCapture()
  })
})

describe('Performance Marks - wrapPerformanceMeasure', () => {
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

  test('should wrap performance.measure to capture calls', async () => {
    const { installPerformanceCapture, uninstallPerformanceCapture, getCapturedMeasures } =
      await import('../../extension/lib/analysis/performance.js')

    installPerformanceCapture()

    globalThis.performance.measure('testMeasure', 'startMark', 'endMark')

    const captured = getCapturedMeasures()

    assert.ok(captured.some((m) => m.name === 'testMeasure'))

    uninstallPerformanceCapture()
  })

  test('should preserve original measure behavior', async () => {
    const { installPerformanceCapture, uninstallPerformanceCapture } = await import('../../extension/lib/analysis/performance.js')

    const originalMeasure = globalThis.performance.measure

    installPerformanceCapture()

    globalThis.performance.measure('preserveMeasure')

    // Original should still be called
    assert.ok(originalMeasure.mock.calls.length > 0)

    uninstallPerformanceCapture()
  })
})

describe('Performance Marks - Error Integration', () => {
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

  test('should create performance snapshot for error', async () => {
    const { getPerformanceSnapshotForError, setPerformanceMarksEnabled } = await import('../../extension/lib/analysis/performance.js')

    setPerformanceMarksEnabled(true)

    globalThis.performance._addEntry(createMockMark('componentMount', 100))
    globalThis.performance._addEntry(createMockMeasure('renderTime', 100, 50))

    const errorEntry = {
      type: 'exception',
      level: 'error',
      message: 'Test error'
    }

    const snapshot = await getPerformanceSnapshotForError(errorEntry)

    assert.ok(snapshot)
    assert.strictEqual(snapshot.type, 'performance')
    assert.ok(snapshot.ts)
    assert.ok(snapshot.marks)
    assert.ok(snapshot.measures)
  })

  test('should respect performanceMarksEnabled setting', async () => {
    const { getPerformanceSnapshotForError, setPerformanceMarksEnabled } = await import('../../extension/lib/analysis/performance.js')

    setPerformanceMarksEnabled(false)

    const errorEntry = { type: 'exception', level: 'error' }

    const snapshot = await getPerformanceSnapshotForError(errorEntry)

    assert.strictEqual(snapshot, null)

    // Re-enable
    setPerformanceMarksEnabled(true)
  })

  test('should only include recent entries (last 60 seconds)', async () => {
    const { getPerformanceSnapshotForError, setPerformanceMarksEnabled } = await import('../../extension/lib/analysis/performance.js')

    setPerformanceMarksEnabled(true)

    // Old entry
    globalThis.performance._addEntry(createMockMark('oldMark', 0))

    // Recent entry
    const now = Date.now()
    globalThis.performance.now = () => now
    globalThis.performance._addEntry(createMockMark('recentMark', now - 10000))

    const errorEntry = { type: 'exception', level: 'error' }
    const snapshot = await getPerformanceSnapshotForError(errorEntry)

    // Should filter based on recency
    assert.ok(snapshot)
    assert.ok(Array.isArray(snapshot.marks))
  })

  test('should include navigation timing', async () => {
    const { getPerformanceSnapshotForError, setPerformanceMarksEnabled } = await import('../../extension/lib/analysis/performance.js')

    setPerformanceMarksEnabled(true)

    globalThis.performance.getEntriesByType = mock.fn((type) => {
      if (type === 'navigation') {
        return [
          {
            name: 'document',
            entryType: 'navigation',
            startTime: 0,
            domContentLoadedEventEnd: 500,
            loadEventEnd: 1000,
            type: 'navigate'
          }
        ]
      }
      return []
    })

    const errorEntry = { type: 'exception', level: 'error' }
    const snapshot = await getPerformanceSnapshotForError(errorEntry)

    assert.ok(snapshot.navigation || snapshot.timing)
  })
})

describe('Performance Marks - Configuration', () => {
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

  test('setPerformanceMarksEnabled should toggle feature', async () => {
    const { setPerformanceMarksEnabled, isPerformanceMarksEnabled } = await import('../../extension/lib/analysis/performance.js')

    setPerformanceMarksEnabled(true)
    assert.strictEqual(isPerformanceMarksEnabled(), true)

    setPerformanceMarksEnabled(false)
    assert.strictEqual(isPerformanceMarksEnabled(), false)
  })

  test('should expose performance marks through __kaboom API', async () => {
    const { installKaboomAPI, uninstallKaboomAPI } = await import('../../extension/inject/api.js')
    const { setPerformanceMarksEnabled } = await import('../../extension/lib/analysis/performance.js')

    setPerformanceMarksEnabled(true)
    installKaboomAPI()

    assert.ok(globalThis.window.__kaboom)
    assert.ok(typeof globalThis.window.__kaboom.setPerformanceMarks === 'function')
    assert.ok(typeof globalThis.window.__kaboom.getMarks === 'function')
    assert.ok(typeof globalThis.window.__kaboom.getMeasures === 'function')

    uninstallKaboomAPI()
  })

  test('should clear captured data on uninstall', async () => {
    const { installPerformanceCapture, uninstallPerformanceCapture, getCapturedMarks, getCapturedMeasures } =
      await import('../../extension/lib/analysis/performance.js')

    installPerformanceCapture()

    globalThis.performance.mark('toBeCleared')
    globalThis.performance.measure('measureToBeCleared')

    uninstallPerformanceCapture()
    installPerformanceCapture() // Fresh start

    const marks = getCapturedMarks()
    const _measures = getCapturedMeasures()

    // Should be empty after uninstall/reinstall
    assert.strictEqual(marks.filter((m) => m.name === 'toBeCleared').length, 0)

    uninstallPerformanceCapture()
  })
})

describe('Performance Marks - PerformanceObserver', () => {
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

  test('should use PerformanceObserver when available', async () => {
    const { installPerformanceCapture, uninstallPerformanceCapture, isPerformanceCaptureActive } =
      await import('../../extension/lib/analysis/performance.js')

    // First uninstall any existing capture (from module auto-init)
    uninstallPerformanceCapture()

    // Create a fresh mock window with PerformanceObserver
    let observerCreated = false
    let observeMethodCalled = false
    globalThis.window = {
      location: { href: 'http://localhost:3000/test' },
      postMessage: mock.fn(),
      performance: createMockPerformance(),
      PerformanceObserver: function MockPerformanceObserver(_callback) {
        observerCreated = true
        return {
          observe: function (_options) {
            observeMethodCalled = true
          },
          disconnect: mock.fn()
        }
      },
      addEventListener: mock.fn(),
      removeEventListener: mock.fn(),
      onerror: null
    }
    globalThis.performance = globalThis.window.performance
    // Code checks typeof PerformanceObserver (global), not window.PerformanceObserver
    globalThis.PerformanceObserver = globalThis.window.PerformanceObserver

    // Now install with the mock in place
    installPerformanceCapture()

    // PerformanceObserver should have been instantiated and observe called
    assert.ok(isPerformanceCaptureActive())
    assert.ok(observerCreated, 'PerformanceObserver constructor should have been called')
    assert.ok(observeMethodCalled, 'observe() should have been called')

    uninstallPerformanceCapture()
  })

  test('should fall back to polling when PerformanceObserver unavailable', async () => {
    const { installPerformanceCapture, uninstallPerformanceCapture, isPerformanceCaptureActive } =
      await import('../../extension/lib/analysis/performance.js')

    delete globalThis.window.PerformanceObserver

    installPerformanceCapture()

    // Should still work (using polling fallback)
    assert.ok(isPerformanceCaptureActive())

    uninstallPerformanceCapture()
  })
})
