// @ts-nocheck
/**
 * @fileoverview interception-deferral.test.js — Tests for two-phase interception.
 * Verifies that console/error capture starts immediately (Phase 1) while network,
 * WebSocket, and action capture defers until after the page load event (Phase 2).
 * Ensures zero performance impact on initial page render.
 * Spec: Phase 1 installs lightweight setup; Phase 2 adds interceptors after load + 100ms.
 */

import { test, describe, mock, beforeEach, afterEach } from 'node:test'
import assert from 'node:assert'
import { createMockWindow, createMockDocument } from '../shared/helpers.js'

let originalWindow, _originalDocument, originalPerformance, originalConsole

describe('Interception Deferral: Phase 1 (Immediate)', () => {
  beforeEach(() => {
    originalWindow = globalThis.window
    originalConsole = globalThis.console
    originalPerformance = globalThis.performance
    globalThis.window = createMockWindow({ href: 'http://localhost:3000/test', withOnerror: true })
    globalThis.document = createMockDocument()
    globalThis.console = {
      log: mock.fn(),
      warn: mock.fn(),
      error: mock.fn(),
      info: mock.fn(),
      debug: mock.fn()
    }
    globalThis.performance = {
      now: mock.fn(() => 42.5),
      mark: mock.fn((name) => ({ name, startTime: 42.5, duration: 0 })),
      measure: mock.fn((name) => ({ name, startTime: 42.5, duration: 0 })),
      getEntriesByType: mock.fn(() => [])
    }
    globalThis.PerformanceObserver = class MockPerfObserver {
      constructor(cb) {
        this.cb = cb
      }
      observe() {}
      disconnect() {}
    }
  })

  afterEach(() => {
    globalThis.window = originalWindow
    globalThis.console = originalConsole
    globalThis.performance = originalPerformance
    delete globalThis.document
    delete globalThis.PerformanceObserver
  })

  test(
    'Phase 1 should install window.__kaboom API',
    { skip: 'installKaboomAPI has double-install guard — module state persists across tests' },
    async () => {
      const { installPhase1 } = await import('../../../extension/inject/observers.js')

      // Ensure clean state
      delete globalThis.window.__kaboom

      installPhase1()

      assert.ok(globalThis.window.__kaboom, 'Phase 1 should install __kaboom API')
      assert.ok(globalThis.window.__kaboom.version, 'API should have version')

      // Cleanup
      delete globalThis.window.__kaboom
    }
  )

  test('Phase 1 should record injection timestamp', async () => {
    const { installPhase1, getDeferralState } = await import('../../../extension/inject/observers.js')

    installPhase1()

    const state = getDeferralState()
    assert.strictEqual(typeof state.injectionTimestamp, 'number', 'Should record injection timestamp')
    assert.ok(state.injectionTimestamp > 0, 'Injection timestamp should be positive')

    // Cleanup
    delete globalThis.window.__kaboom
  })

  test('Phase 1 should NOT modify console methods', async () => {
    const { installPhase1 } = await import('../../../extension/inject/observers.js')

    const originalLog = globalThis.console.log
    const originalError = globalThis.console.error
    const originalWarn = globalThis.console.warn

    installPhase1()

    assert.strictEqual(globalThis.console.log, originalLog, 'console.log should not be wrapped in Phase 1')
    assert.strictEqual(globalThis.console.error, originalError, 'console.error should not be wrapped in Phase 1')
    assert.strictEqual(globalThis.console.warn, originalWarn, 'console.warn should not be wrapped in Phase 1')

    // Cleanup
    delete globalThis.window.__kaboom
  })

  test('Phase 1 should NOT modify fetch', async () => {
    const { installPhase1 } = await import('../../../extension/inject/observers.js')

    globalThis.window.fetch = mock.fn()
    const fetchBefore = globalThis.window.fetch

    installPhase1()

    assert.strictEqual(globalThis.window.fetch, fetchBefore, 'fetch should not be wrapped in Phase 1')

    // Cleanup
    delete globalThis.window.__kaboom
  })

  test('Phase 1 should NOT modify WebSocket constructor', async () => {
    const { installPhase1 } = await import('../../../extension/inject/observers.js')

    class OriginalWebSocket {}
    globalThis.window.WebSocket = OriginalWebSocket

    installPhase1()

    assert.strictEqual(globalThis.window.WebSocket, OriginalWebSocket, 'WebSocket should not be replaced in Phase 1')

    // Cleanup
    delete globalThis.window.__kaboom
  })

  test(
    'Phase 1 should install PerformanceObservers (FCP, LCP, CLS)',
    { skip: 'installPerfObservers has double-install guard — module state persists across tests' },
    async () => {
      const { installPhase1 } = await import('../../../extension/inject/observers.js')

      let observerCount = 0
      globalThis.PerformanceObserver = class MockPerfObserver {
        constructor(cb) {
          this.cb = cb
        }
        observe() {
          observerCount++
        }
        disconnect() {}
      }

      installPhase1()

      // installPerfObservers creates observers for longtask, paint, LCP, CLS
      assert.ok(observerCount >= 3, `Expected at least 3 PerformanceObservers, got ${observerCount}`)

      // Cleanup
      delete globalThis.window.__kaboom
    }
  )

  test('Phase 1 should set phase2Installed to false', async () => {
    const { installPhase1, getDeferralState } = await import('../../../extension/inject/observers.js')

    installPhase1()

    const state = getDeferralState()
    assert.strictEqual(state.phase2Installed, false, 'Phase 2 should not be installed after Phase 1')

    // Cleanup
    delete globalThis.window.__kaboom
  })
})

describe('Interception Deferral: Phase 2 (Deferred)', () => {
  beforeEach(() => {
    originalWindow = globalThis.window
    originalConsole = globalThis.console
    originalPerformance = globalThis.performance
    globalThis.window = createMockWindow({ href: 'http://localhost:3000/test', withOnerror: true })
    globalThis.document = createMockDocument()
    globalThis.console = {
      log: mock.fn(),
      warn: mock.fn(),
      error: mock.fn(),
      info: mock.fn(),
      debug: mock.fn()
    }
    globalThis.performance = {
      now: mock.fn(() => 150.0),
      mark: mock.fn((name) => ({ name, startTime: 150.0, duration: 0 })),
      measure: mock.fn((name) => ({ name, startTime: 150.0, duration: 0 })),
      getEntriesByType: mock.fn(() => [])
    }
    globalThis.PerformanceObserver = class MockPerfObserver {
      constructor(cb) {
        this.cb = cb
      }
      observe() {}
      disconnect() {}
    }
  })

  afterEach(() => {
    globalThis.window = originalWindow
    globalThis.console = originalConsole
    globalThis.performance = originalPerformance
    delete globalThis.document
    delete globalThis.PerformanceObserver
  })

  test('Phase 2 should install console interceptors', async () => {
    const { installPhase2, uninstall } = await import('../../../extension/inject/observers.js')

    const originalLog = globalThis.console.log

    installPhase2()

    // Console should now be wrapped
    assert.notStrictEqual(globalThis.console.log, originalLog, 'console.log should be wrapped after Phase 2')

    uninstall()
  })

  test('Phase 2 should set phase2Installed to true', async () => {
    const { installPhase2, getDeferralState, uninstall } = await import('../../../extension/inject/observers.js')

    installPhase2()

    const state = getDeferralState()
    assert.strictEqual(state.phase2Installed, true, 'phase2Installed should be true after Phase 2')

    uninstall()
  })

  test('Phase 2 should record phase2Timestamp', async () => {
    const { installPhase2, getDeferralState, uninstall } = await import('../../../extension/inject/observers.js')

    installPhase2()

    const state = getDeferralState()
    assert.strictEqual(typeof state.phase2Timestamp, 'number', 'Should have phase2Timestamp')
    assert.ok(state.phase2Timestamp > 0, 'phase2Timestamp should be positive')

    uninstall()
  })

  test('Double-injection guard: Phase 2 should not run twice', async () => {
    const { installPhase2, getDeferralState, uninstall } = await import('../../../extension/inject/observers.js')

    installPhase2()
    const firstTimestamp = getDeferralState().phase2Timestamp

    // Call again - should be a no-op
    globalThis.performance.now = mock.fn(() => 999.0)
    installPhase2()

    const state = getDeferralState()
    assert.strictEqual(state.phase2Timestamp, firstTimestamp, 'Phase 2 should not run again if already installed')

    uninstall()
  })
})

describe('Interception Deferral: Deferral Logic', () => {
  beforeEach(() => {
    originalWindow = globalThis.window
    originalConsole = globalThis.console
    originalPerformance = globalThis.performance
    globalThis.window = createMockWindow({ href: 'http://localhost:3000/test', withOnerror: true })
    globalThis.document = createMockDocument()
    globalThis.console = {
      log: mock.fn(),
      warn: mock.fn(),
      error: mock.fn(),
      info: mock.fn(),
      debug: mock.fn()
    }
    globalThis.performance = {
      now: mock.fn(() => 10.0),
      mark: mock.fn((name) => ({ name, startTime: 10.0, duration: 0 })),
      measure: mock.fn((name) => ({ name, startTime: 10.0, duration: 0 })),
      getEntriesByType: mock.fn(() => [])
    }
    globalThis.PerformanceObserver = class MockPerfObserver {
      constructor(cb) {
        this.cb = cb
      }
      observe() {}
      disconnect() {}
    }
  })

  afterEach(() => {
    globalThis.window = originalWindow
    globalThis.console = originalConsole
    globalThis.performance = originalPerformance
    delete globalThis.document
    delete globalThis.PerformanceObserver
  })

  test('Default: Phase 2 installs after load event + 100ms delay', async () => {
    const { installPhase1, getDeferralState, setDeferralEnabled, uninstall } = await import('../../../extension/inject/observers.js')

    setDeferralEnabled(true)

    // Mock document.readyState as 'loading' (load not fired yet)
    Object.defineProperty(globalThis.document, 'readyState', { value: 'loading', configurable: true })

    installPhase1()

    // Phase 2 should NOT be installed yet
    assert.strictEqual(
      getDeferralState().phase2Installed,
      false,
      'Phase 2 should not install immediately when deferral is enabled'
    )

    // Find the load event listener that was registered
    const addListenerCalls = globalThis.window.addEventListener.mock.calls
    const loadHandler = addListenerCalls.find((call) => call.arguments[0] === 'load')
    assert.ok(loadHandler, 'Should register a load event listener')

    // Simulate load event firing
    loadHandler.arguments[1]()

    // Phase 2 still not installed (needs 100ms delay)
    assert.strictEqual(getDeferralState().phase2Installed, false, 'Phase 2 should wait 100ms after load')

    // Wait 150ms for setTimeout to fire
    await new Promise((resolve) => setTimeout(resolve, 150))

    assert.strictEqual(getDeferralState().phase2Installed, true, 'Phase 2 should install after load + 100ms')

    uninstall()
    delete globalThis.window.__kaboom
  })

  test('deferralEnabled=false: Phase 2 installs immediately', async () => {
    const { installPhase1, getDeferralState, setDeferralEnabled, uninstall } = await import('../../../extension/inject/observers.js')

    setDeferralEnabled(false)

    installPhase1()

    // Phase 2 should be installed immediately
    assert.strictEqual(
      getDeferralState().phase2Installed,
      true,
      'Phase 2 should install immediately when deferral is disabled'
    )

    uninstall()
    setDeferralEnabled(true) // reset
    delete globalThis.window.__kaboom
  })

  test('document.readyState=complete at injection: installs immediately (+100ms)', async () => {
    const { installPhase1, getDeferralState, setDeferralEnabled, uninstall } = await import('../../../extension/inject/observers.js')

    setDeferralEnabled(true)

    // Simulate page already loaded
    Object.defineProperty(globalThis.document, 'readyState', { value: 'complete', configurable: true })

    installPhase1()

    // Should NOT be installed immediately (still needs 100ms)
    assert.strictEqual(
      getDeferralState().phase2Installed,
      false,
      'Phase 2 should wait 100ms even when readyState=complete'
    )

    // Wait for the 100ms setTimeout
    await new Promise((resolve) => setTimeout(resolve, 150))

    assert.strictEqual(
      getDeferralState().phase2Installed,
      true,
      'Phase 2 should install 100ms after detecting readyState=complete'
    )

    uninstall()
    delete globalThis.window.__kaboom
  })

  test('10-second timeout fallback: Phase 2 installs if load never fires', async () => {
    const { installPhase1, getDeferralState, setDeferralEnabled, uninstall } = await import('../../../extension/inject/observers.js')

    setDeferralEnabled(true)

    Object.defineProperty(globalThis.document, 'readyState', { value: 'loading', configurable: true })

    // Spy on setTimeout to capture the 10s fallback callback
    const originalSetTimeout = globalThis.setTimeout
    const timeoutCalls = []
    const timerIds = []
    globalThis.setTimeout = (fn, delay) => {
      timeoutCalls.push({ fn, delay })
      if (delay > 5000) return 0 // Don't schedule the 10s timer (would hang the test)
      const id = originalSetTimeout(fn, delay)
      timerIds.push(id)
      return id
    }

    installPhase1()

    // Phase 2 should not be installed yet
    assert.strictEqual(getDeferralState().phase2Installed, false, 'Phase 2 should not install before timeout')

    // Verify a setTimeout with 10000ms was registered (the fallback)
    const fallbackTimeout = timeoutCalls.find((c) => c.delay === 10000)
    assert.ok(fallbackTimeout, 'Should register a 10-second fallback timeout')

    // Manually invoke the fallback callback to simulate the timeout firing
    fallbackTimeout.fn()

    // Phase 2 should now be installed
    assert.strictEqual(getDeferralState().phase2Installed, true, 'Phase 2 should install when 10s fallback fires')

    timerIds.forEach((id) => clearTimeout(id))
    globalThis.setTimeout = originalSetTimeout
    uninstall()
    delete globalThis.window.__kaboom
  })

  test('Console logs before Phase 2 are not captured (intentional)', async () => {
    const { installPhase1, getDeferralState, setDeferralEnabled } = await import('../../../extension/inject/observers.js')

    setDeferralEnabled(true)
    Object.defineProperty(globalThis.document, 'readyState', { value: 'loading', configurable: true })

    installPhase1()

    // Phase 2 not installed yet
    assert.strictEqual(getDeferralState().phase2Installed, false)

    // Log something - should NOT be captured since interceptors aren't installed
    globalThis.console.log('pre-phase2 message')

    // postMessage should not have been called with a GASOLINE_LOG for this
    const gasolineLogs = globalThis.window.postMessage.mock.calls.filter(
      (call) => call.arguments[0]?.type === 'kaboom_log'
    )
    assert.strictEqual(gasolineLogs.length, 0, 'Console logs before Phase 2 should not be captured')

    delete globalThis.window.__kaboom
  })

  test('SPA navigation after Phase 2 does not re-defer interceptors', async () => {
    const { installPhase1, getDeferralState, setDeferralEnabled, uninstall } = await import('../../../extension/inject/observers.js')

    setDeferralEnabled(false) // Install immediately for this test
    installPhase1()

    assert.strictEqual(getDeferralState().phase2Installed, true)
    const phase2Time = getDeferralState().phase2Timestamp

    // Simulate SPA navigation via popstate event
    const popstateHandlers = globalThis.window.addEventListener.mock.calls.filter((c) => c.arguments[0] === 'popstate')
    if (popstateHandlers.length > 0) {
      popstateHandlers.forEach((h) => h.arguments[1]({ type: 'popstate' }))
    }

    // Phase 2 should stay active (not re-deferred, timestamp unchanged)
    const stateAfterNav = getDeferralState()
    assert.strictEqual(stateAfterNav.phase2Installed, true, 'Phase 2 should stay active after SPA navigation')
    assert.strictEqual(stateAfterNav.phase2Timestamp, phase2Time, 'Phase 2 timestamp should not change on navigation')

    uninstall()
    setDeferralEnabled(true)
    delete globalThis.window.__kaboom
  })
})

describe('Interception Deferral: State Management', () => {
  beforeEach(() => {
    originalWindow = globalThis.window
    originalConsole = globalThis.console
    originalPerformance = globalThis.performance
    globalThis.window = createMockWindow({ href: 'http://localhost:3000/test', withOnerror: true })
    globalThis.document = createMockDocument()
    globalThis.console = {
      log: mock.fn(),
      warn: mock.fn(),
      error: mock.fn(),
      info: mock.fn(),
      debug: mock.fn()
    }
    globalThis.performance = {
      now: mock.fn(() => 50.0),
      mark: mock.fn((name) => ({ name, startTime: 50.0, duration: 0 })),
      measure: mock.fn((name) => ({ name, startTime: 50.0, duration: 0 })),
      getEntriesByType: mock.fn(() => [])
    }
    globalThis.PerformanceObserver = class MockPerfObserver {
      constructor(cb) {
        this.cb = cb
      }
      observe() {}
      disconnect() {}
    }
  })

  afterEach(() => {
    globalThis.window = originalWindow
    globalThis.console = originalConsole
    globalThis.performance = originalPerformance
    delete globalThis.document
    delete globalThis.PerformanceObserver
  })

  test('getDeferralState returns correct initial state', async () => {
    const { getDeferralState, setDeferralEnabled } = await import('../../../extension/inject/observers.js')

    setDeferralEnabled(true)

    const state = getDeferralState()
    assert.strictEqual(typeof state.deferralEnabled, 'boolean', 'deferralEnabled should be boolean')
    assert.strictEqual(typeof state.phase2Installed, 'boolean', 'phase2Installed should be boolean')
    assert.strictEqual(typeof state.injectionTimestamp, 'number', 'injectionTimestamp should be number')
    assert.strictEqual(typeof state.phase2Timestamp, 'number', 'phase2Timestamp should be number')
  })

  test('setDeferralEnabled should update state', async () => {
    const { getDeferralState, setDeferralEnabled } = await import('../../../extension/inject/observers.js')

    setDeferralEnabled(true)
    assert.strictEqual(getDeferralState().deferralEnabled, true)

    setDeferralEnabled(false)
    assert.strictEqual(getDeferralState().deferralEnabled, false)

    setDeferralEnabled(true) // reset
  })

  test('getDeferralState includes timing diagnostics after Phase 2', async () => {
    const { installPhase1, getDeferralState, setDeferralEnabled, uninstall } = await import('../../../extension/inject/observers.js')

    setDeferralEnabled(false)

    globalThis.performance.now = mock.fn(() => 10.0)
    installPhase1()

    globalThis.performance.now = mock.fn(() => 250.0)
    // Phase 2 was already called by installPhase1 since deferral is disabled
    // But let's verify the state
    const state = getDeferralState()
    assert.ok(state.injectionTimestamp > 0, 'Should have injection timestamp')
    assert.ok(state.phase2Timestamp > 0, 'Should have phase2 timestamp')

    uninstall()
    setDeferralEnabled(true)
    delete globalThis.window.__kaboom
  })
})

describe('Interception Deferral: GASOLINE_SETTING integration', () => {
  beforeEach(() => {
    originalWindow = globalThis.window
    originalConsole = globalThis.console
    originalPerformance = globalThis.performance
    globalThis.window = createMockWindow({ href: 'http://localhost:3000/test', withOnerror: true })
    globalThis.document = createMockDocument()
    globalThis.console = {
      log: mock.fn(),
      warn: mock.fn(),
      error: mock.fn(),
      info: mock.fn(),
      debug: mock.fn()
    }
    globalThis.performance = {
      now: mock.fn(() => 5.0),
      mark: mock.fn((name) => ({ name, startTime: 5.0, duration: 0 })),
      measure: mock.fn((name) => ({ name, startTime: 5.0, duration: 0 })),
      getEntriesByType: mock.fn(() => [])
    }
    globalThis.PerformanceObserver = class MockPerfObserver {
      constructor(cb) {
        this.cb = cb
      }
      observe() {}
      disconnect() {}
    }
  })

  afterEach(() => {
    globalThis.window = originalWindow
    globalThis.console = originalConsole
    globalThis.performance = originalPerformance
    delete globalThis.document
    delete globalThis.PerformanceObserver
  })

  test('setDeferralEnabled setting should update deferral state', async () => {
    const { getDeferralState, setDeferralEnabled } = await import('../../../extension/inject/observers.js')

    setDeferralEnabled(true)
    assert.strictEqual(getDeferralState().deferralEnabled, true)

    setDeferralEnabled(false)
    assert.strictEqual(getDeferralState().deferralEnabled, false)

    // Reset
    setDeferralEnabled(true)
  })
})
