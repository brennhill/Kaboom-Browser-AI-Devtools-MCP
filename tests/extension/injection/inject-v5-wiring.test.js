// @ts-nocheck
/**
 * @fileoverview inject-v5-wiring.test.js — Tests for V5 wiring: exception handler
 * enrichment, enhanced action recording in handlers, enhanced action postMessage
 * emission, and navigation event recording in inject.js.
 */

import { test, describe, mock, beforeEach, afterEach } from 'node:test'
import assert from 'node:assert'
import { createMockWindow, createMockDocument } from '../shared/helpers.js'

// Define esbuild constant not available in Node test env
globalThis.__KABOOM_VERSION__ = 'test'

// Store original
let originalWindow

// =============================================================================
// V5 WIRING: Exception handlers call enrichErrorWithAiContext
// =============================================================================

describe('V5 Wiring: Exception handler enrichment', () => {
  beforeEach(() => {
    originalWindow = globalThis.window
    globalThis.window = createMockWindow({ href: 'http://localhost:3000/test', withOnerror: true })
    globalThis.document = createMockDocument({ activeElement: null })
  })

  afterEach(() => {
    globalThis.window = originalWindow
  })

  test('window.onerror should enrich error with AI context before posting', async () => {
    const { installExceptionCapture, uninstallExceptionCapture } = await import('../../../extension/lib/page/exceptions.js')
    const { setAiContextEnabled } = await import('../../../extension/lib/ai-context/ai-context-enrichment.js')

    setAiContextEnabled(true)
    installExceptionCapture()

    const error = new Error('TypeError: x is undefined')
    error.stack = 'TypeError: x is undefined\n    at foo (http://localhost:3000/main.js:10:5)'

    globalThis.window.onerror('TypeError: x is undefined', 'http://localhost:3000/main.js', 10, 5, error)

    // enrichErrorWithAiContext is async, wait for it
    await new Promise((resolve) => setTimeout(resolve, 50))

    // The posted message should include _aiContext
    const calls = globalThis.window.postMessage.mock.calls
    assert.ok(calls.length >= 1, 'Should have posted a message')

    const lastCall = calls[calls.length - 1]
    const message = lastCall.arguments[0]
    assert.strictEqual(message.type, 'kaboom_log')
    assert.strictEqual(message.payload.type, 'exception')
    assert.ok(message.payload._aiContext, 'Should have _aiContext field')
    assert.ok(message.payload._aiContext.summary, 'Should have summary in _aiContext')

    uninstallExceptionCapture()
  })

  test('unhandled rejection should enrich error with AI context', async () => {
    const { installExceptionCapture, uninstallExceptionCapture } = await import('../../../extension/lib/page/exceptions.js')
    const { setAiContextEnabled } = await import('../../../extension/lib/ai-context/ai-context-enrichment.js')

    setAiContextEnabled(true)
    installExceptionCapture()

    // Get the rejection handler
    const addListenerCalls = globalThis.window.addEventListener.mock.calls
    const rejectionHandler = addListenerCalls.find((call) => call.arguments[0] === 'unhandledrejection')
    assert.ok(rejectionHandler)

    const handler = rejectionHandler.arguments[1]
    handler({ reason: new Error('Async failure') })

    // Wait for async enrichment
    await new Promise((resolve) => setTimeout(resolve, 50))

    const calls = globalThis.window.postMessage.mock.calls
    assert.ok(calls.length >= 1)

    const lastCall = calls[calls.length - 1]
    const message = lastCall.arguments[0]
    assert.ok(message.payload._aiContext, 'Rejection should have _aiContext')

    uninstallExceptionCapture()
  })

  test('should still post error when AI context is disabled', async () => {
    const { installExceptionCapture, uninstallExceptionCapture } = await import('../../../extension/lib/page/exceptions.js')
    const { setAiContextEnabled } = await import('../../../extension/lib/ai-context/ai-context-enrichment.js')

    setAiContextEnabled(false)
    installExceptionCapture()

    globalThis.window.onerror('Test error', 'app.js', 1, 1, new Error('Test'))

    // Wait for async path
    await new Promise((resolve) => setTimeout(resolve, 50))

    const calls = globalThis.window.postMessage.mock.calls
    assert.ok(calls.length >= 1)
    const message = calls[calls.length - 1].arguments[0]
    assert.strictEqual(message.payload.type, 'exception')
    // No _aiContext when disabled
    assert.strictEqual(message.payload._aiContext, undefined)

    uninstallExceptionCapture()
    setAiContextEnabled(true)
  })
})

describe('V5 Wiring: Kaboom API exports', () => {
  test('API module should export Kaboom API installers only', async () => {
    const apiModule = await import('../../../extension/inject/api.js')

    // Kaboom-named installers are the public surface...
    assert.strictEqual(typeof apiModule.installKaboomAPI, 'function')
    assert.strictEqual(typeof apiModule.uninstallKaboomAPI, 'function')
    // ...and the pre-rebrand Gasoline aliases must no longer be exported.
    assert.strictEqual(apiModule.installGasolineAPI, undefined)
    assert.strictEqual(apiModule.uninstallGasolineAPI, undefined)
  })
})

// =============================================================================
// V5 WIRING: Action handlers call recordEnhancedAction
// =============================================================================

describe('V5 Wiring: Enhanced action recording in handlers', () => {
  beforeEach(() => {
    originalWindow = globalThis.window
    globalThis.window = createMockWindow({ href: 'http://localhost:3000/test', withOnerror: true })
    globalThis.document = createMockDocument()
  })

  afterEach(() => {
    globalThis.window = originalWindow
  })

  test('handleClick should also call recordEnhancedAction', async () => {
    const { handleClick, setActionCaptureEnabled } = await import('../../../extension/lib/page/actions.js')
    const { getEnhancedActionBuffer, clearEnhancedActionBuffer } = await import('../../../extension/lib/page/reproduction.js')

    clearEnhancedActionBuffer()
    setActionCaptureEnabled(true)

    const mockElement = {
      tagName: 'BUTTON',
      id: 'submit-btn',
      className: '',
      textContent: 'Submit',
      innerText: 'Submit',
      getAttribute: (name) => {
        if (name === 'data-testid') return 'submit-btn'
        return null
      },
      hasAttribute: (name) => name === 'data-testid',
      parentElement: null,
      children: [],
      childNodes: []
    }

    const mockEvent = {
      target: mockElement,
      clientX: 100,
      clientY: 200
    }

    handleClick(mockEvent)

    const enhanced = getEnhancedActionBuffer()
    assert.ok(enhanced.length >= 1, 'Should have recorded enhanced action')
    assert.strictEqual(enhanced[enhanced.length - 1].type, 'click')
    assert.ok(enhanced[enhanced.length - 1].selectors, 'Should have selectors')

    clearEnhancedActionBuffer()
  })

  test('handleInput should also call recordEnhancedAction', async () => {
    const { handleInput, setActionCaptureEnabled } = await import('../../../extension/lib/page/actions.js')
    const { getEnhancedActionBuffer, clearEnhancedActionBuffer } = await import('../../../extension/lib/page/reproduction.js')

    clearEnhancedActionBuffer()
    setActionCaptureEnabled(true)

    const mockElement = {
      tagName: 'INPUT',
      id: 'email-input',
      className: '',
      type: 'email',
      value: 'test@example.com',
      textContent: '',
      getAttribute: (name) => {
        if (name === 'type') return 'email'
        if (name === 'autocomplete') return 'email'
        return null
      },
      hasAttribute: (name) => name === 'type',
      parentElement: null,
      children: [],
      childNodes: []
    }

    const mockEvent = { target: mockElement }

    handleInput(mockEvent)

    const enhanced = getEnhancedActionBuffer()
    assert.ok(enhanced.length >= 1, 'Should have recorded enhanced action')
    const lastAction = enhanced[enhanced.length - 1]
    assert.strictEqual(lastAction.type, 'input')
    assert.strictEqual(lastAction.input_type, 'email')

    clearEnhancedActionBuffer()
  })

  test('handleInput should redact password fields in enhanced action', async () => {
    const { handleInput, setActionCaptureEnabled } = await import('../../../extension/lib/page/actions.js')
    const { getEnhancedActionBuffer, clearEnhancedActionBuffer } = await import('../../../extension/lib/page/reproduction.js')

    clearEnhancedActionBuffer()
    setActionCaptureEnabled(true)

    const mockElement = {
      tagName: 'INPUT',
      id: 'password-input',
      className: '',
      type: 'password',
      value: 'secret123',
      textContent: '',
      getAttribute: (name) => {
        if (name === 'type') return 'password'
        return null
      },
      hasAttribute: (name) => name === 'type',
      parentElement: null,
      children: [],
      childNodes: []
    }

    handleInput({ target: mockElement })

    const enhanced = getEnhancedActionBuffer()
    const lastAction = enhanced[enhanced.length - 1]
    assert.strictEqual(lastAction.value, '[redacted]')

    clearEnhancedActionBuffer()
  })

  test('handleScroll should call recordEnhancedAction with scroll type', async () => {
    const { handleScroll, setActionCaptureEnabled } = await import('../../../extension/lib/page/actions.js')
    const { getEnhancedActionBuffer, clearEnhancedActionBuffer } = await import('../../../extension/lib/page/reproduction.js')

    clearEnhancedActionBuffer()
    setActionCaptureEnabled(true)

    // Wait for scroll throttle to expire (250ms)
    await new Promise((r) => setTimeout(r, 300))

    globalThis.window.scrollX = 0
    globalThis.window.scrollY = 750

    const mockEvent = {
      target: globalThis.document
    }

    handleScroll(mockEvent)

    const enhanced = getEnhancedActionBuffer()
    const scrollAction = enhanced.find((a) => a.type === 'scroll')
    assert.ok(scrollAction, 'handleScroll should record enhanced action')
    assert.strictEqual(scrollAction.scroll_y, 750)
    assert.strictEqual(scrollAction.type, 'scroll')

    clearEnhancedActionBuffer()
  })

  test('keydown handler should call recordEnhancedAction with keypress type', async () => {
    const { handleKeydown, setActionCaptureEnabled } = await import('../../../extension/lib/page/actions.js')
    const { getEnhancedActionBuffer, clearEnhancedActionBuffer } = await import('../../../extension/lib/page/reproduction.js')

    clearEnhancedActionBuffer()
    setActionCaptureEnabled(true)

    const mockElement = {
      tagName: 'INPUT',
      id: 'search-input',
      className: '',
      textContent: '',
      getAttribute: (_name) => null,
      hasAttribute: () => false,
      parentElement: null,
      children: [],
      childNodes: []
    }

    const mockEvent = {
      target: mockElement,
      key: 'Enter'
    }

    handleKeydown(mockEvent)

    const enhanced = getEnhancedActionBuffer()
    const keyAction = enhanced.find((a) => a.type === 'keypress')
    assert.ok(keyAction, 'keydown handler should record enhanced action')
    assert.strictEqual(keyAction.key, 'Enter')
    assert.ok(keyAction.selectors, 'Should have selectors')

    clearEnhancedActionBuffer()
  })

  test('keydown handler should only record actionable keys (Enter, Escape, Tab, arrows)', async () => {
    const { handleKeydown, setActionCaptureEnabled } = await import('../../../extension/lib/page/actions.js')
    const { getEnhancedActionBuffer, clearEnhancedActionBuffer } = await import('../../../extension/lib/page/reproduction.js')

    clearEnhancedActionBuffer()
    setActionCaptureEnabled(true)

    const mockElement = {
      tagName: 'INPUT',
      id: 'text-input',
      className: '',
      textContent: '',
      getAttribute: () => null,
      hasAttribute: () => false,
      parentElement: null,
      children: [],
      childNodes: []
    }

    // Regular character keys should NOT be recorded
    handleKeydown({ target: mockElement, key: 'a' })
    handleKeydown({ target: mockElement, key: '5' })
    handleKeydown({ target: mockElement, key: ' ' })

    const enhanced = getEnhancedActionBuffer()
    assert.strictEqual(
      enhanced.filter((a) => a.type === 'keypress').length,
      0,
      'Regular character keys should not be recorded'
    )

    // Actionable keys SHOULD be recorded
    handleKeydown({ target: mockElement, key: 'Enter' })
    handleKeydown({ target: mockElement, key: 'Escape' })
    handleKeydown({ target: mockElement, key: 'Tab' })
    handleKeydown({ target: mockElement, key: 'ArrowDown' })

    const enhanced2 = getEnhancedActionBuffer()
    assert.strictEqual(enhanced2.filter((a) => a.type === 'keypress').length, 4, 'Actionable keys should be recorded')

    clearEnhancedActionBuffer()
  })

  test('change handler on select should call recordEnhancedAction with select type', async () => {
    const { handleChange, setActionCaptureEnabled } = await import('../../../extension/lib/page/actions.js')
    const { getEnhancedActionBuffer, clearEnhancedActionBuffer } = await import('../../../extension/lib/page/reproduction.js')

    clearEnhancedActionBuffer()
    setActionCaptureEnabled(true)

    const mockElement = {
      tagName: 'SELECT',
      id: 'country-select',
      className: '',
      textContent: '',
      value: 'us',
      getAttribute: (_name) => null,
      hasAttribute: () => false,
      parentElement: null,
      children: [],
      childNodes: [],
      options: [
        { value: 'uk', text: 'United Kingdom', selected: false },
        { value: 'us', text: 'United States', selected: true }
      ],
      selectedIndex: 1
    }

    handleChange({ target: mockElement })

    const enhanced = getEnhancedActionBuffer()
    const selectAction = enhanced.find((a) => a.type === 'select')
    assert.ok(selectAction, 'change handler on select should record enhanced action')
    assert.strictEqual(selectAction.selected_value, 'us')
    assert.strictEqual(selectAction.selected_text, 'United States')

    clearEnhancedActionBuffer()
  })

  test('change handler should ignore non-select elements', async () => {
    const { handleChange, setActionCaptureEnabled } = await import('../../../extension/lib/page/actions.js')
    const { getEnhancedActionBuffer, clearEnhancedActionBuffer } = await import('../../../extension/lib/page/reproduction.js')

    clearEnhancedActionBuffer()
    setActionCaptureEnabled(true)

    const mockElement = {
      tagName: 'INPUT',
      id: 'text-input',
      className: '',
      type: 'text',
      value: 'hello',
      textContent: '',
      getAttribute: () => null,
      hasAttribute: () => false,
      parentElement: null,
      children: [],
      childNodes: []
    }

    handleChange({ target: mockElement })

    const enhanced = getEnhancedActionBuffer()
    assert.strictEqual(
      enhanced.filter((a) => a.type === 'select').length,
      0,
      'change handler should not record for non-select elements'
    )

    clearEnhancedActionBuffer()
  })

  test('installActionCapture should register keydown and change listeners', async () => {
    const { installActionCapture, uninstallActionCapture } = await import('../../../extension/lib/page/actions.js')

    installActionCapture()

    // Check document.addEventListener was called with keydown
    const docCalls = globalThis.document.addEventListener.mock.calls
    const keydownCall = docCalls.find((c) => c.arguments[0] === 'keydown')
    assert.ok(keydownCall, 'installActionCapture should register keydown listener')
    assert.deepStrictEqual(keydownCall.arguments[2], { capture: true, passive: true })

    // Check document.addEventListener was called with change
    const changeCall = docCalls.find((c) => c.arguments[0] === 'change')
    assert.ok(changeCall, 'installActionCapture should register change listener')
    assert.deepStrictEqual(changeCall.arguments[2], { capture: true, passive: true })

    uninstallActionCapture()
  })
})

// =============================================================================
// V5 WIRING: recordEnhancedAction emits postMessage
// =============================================================================

describe('V5 Wiring: Enhanced action postMessage emission', () => {
  beforeEach(() => {
    originalWindow = globalThis.window
    globalThis.window = createMockWindow({ href: 'http://localhost:3000/test', withOnerror: true })
    globalThis.document = createMockDocument({ activeElement: null })
  })

  afterEach(() => {
    globalThis.window = originalWindow
  })

  test('enhanced action payload has spec-compliant base shape', async () => {
    const { recordEnhancedAction, clearEnhancedActionBuffer } = await import('../../../extension/lib/page/reproduction.js')
    const { setActionCaptureEnabled } = await import('../../../extension/lib/page/actions.js')

    clearEnhancedActionBuffer()
    setActionCaptureEnabled(true)

    const mockElement = {
      tagName: 'BUTTON',
      id: 'submit',
      className: 'btn',
      textContent: 'Submit',
      innerText: 'Submit',
      getAttribute: (name) => (name === 'data-testid' ? 'submit-btn' : null),
      hasAttribute: (name) => name === 'data-testid',
      parentElement: null,
      children: [],
      childNodes: []
    }

    recordEnhancedAction('click', mockElement)

    const postCalls = globalThis.window.postMessage.mock.calls
    const enhancedCall = postCalls.find((c) => c.arguments[0]?.type === 'kaboom_enhanced_action')
    assert.ok(enhancedCall, 'Expected kaboom_enhanced_action message')
    const payload = enhancedCall.arguments[0].payload

    // Base shape: type, timestamp, url, selectors
    assert.ok('type' in payload, 'missing: type')
    assert.ok('timestamp' in payload, 'missing: timestamp')
    assert.ok('url' in payload, 'missing: url')
    assert.ok('selectors' in payload, 'missing: selectors')
    assert.strictEqual(typeof payload.timestamp, 'number')

    // Selectors shape
    assert.strictEqual(typeof payload.selectors, 'object')

    clearEnhancedActionBuffer()
  })

  test('recordEnhancedAction should emit kaboom_enhanced_action via postMessage', async () => {
    const { recordEnhancedAction, clearEnhancedActionBuffer } = await import('../../../extension/lib/page/reproduction.js')
    const { setActionCaptureEnabled } = await import('../../../extension/lib/page/actions.js')

    clearEnhancedActionBuffer()
    setActionCaptureEnabled(true)

    const mockElement = {
      tagName: 'BUTTON',
      id: 'save-btn',
      className: '',
      textContent: 'Save',
      innerText: 'Save',
      getAttribute: (name) => {
        if (name === 'data-testid') return 'save-btn'
        return null
      },
      hasAttribute: (name) => name === 'data-testid',
      parentElement: null,
      children: [],
      childNodes: []
    }

    recordEnhancedAction('click', mockElement)

    // Should have posted kaboom_enhanced_action message
    const postCalls = globalThis.window.postMessage.mock.calls
    const enhancedCall = postCalls.find((c) => c.arguments[0]?.type === 'kaboom_enhanced_action')
    assert.ok(enhancedCall, 'recordEnhancedAction should emit kaboom_enhanced_action')
    assert.strictEqual(enhancedCall.arguments[0].payload.type, 'click')
    assert.ok(enhancedCall.arguments[0].payload.selectors, 'Payload should include selectors')
    assert.ok(enhancedCall.arguments[0].payload.timestamp, 'Payload should include timestamp')
    assert.strictEqual(enhancedCall.arguments[1], 'http://localhost:3000')

    clearEnhancedActionBuffer()
  })

  test('recordEnhancedAction should include all action fields in postMessage', async () => {
    const { recordEnhancedAction, clearEnhancedActionBuffer } = await import('../../../extension/lib/page/reproduction.js')
    const { setActionCaptureEnabled } = await import('../../../extension/lib/page/actions.js')

    clearEnhancedActionBuffer()
    setActionCaptureEnabled(true)

    const mockElement = {
      tagName: 'INPUT',
      id: 'email',
      className: '',
      textContent: '',
      type: 'email',
      getAttribute: (name) => {
        if (name === 'type') return 'email'
        return null
      },
      hasAttribute: () => false,
      parentElement: null,
      children: [],
      childNodes: []
    }

    recordEnhancedAction('input', mockElement, { value: 'test@example.com' })

    const postCalls = globalThis.window.postMessage.mock.calls
    const enhancedCall = postCalls.find((c) => c.arguments[0]?.type === 'kaboom_enhanced_action')
    assert.ok(enhancedCall)
    assert.strictEqual(enhancedCall.arguments[0].payload.input_type, 'email')
    assert.strictEqual(enhancedCall.arguments[0].payload.value, 'test@example.com')

    clearEnhancedActionBuffer()
  })
})

// =============================================================================
// V5 WIRING: Navigation events call recordEnhancedAction
// =============================================================================

describe('V5 Wiring: Navigation event recording', () => {
  beforeEach(() => {
    originalWindow = globalThis.window
    globalThis.window = createMockWindow({
      href: 'http://localhost:3000/test',
      withOnerror: true,
      overrides: {
        history: {
          pushState: mock.fn(),
          replaceState: mock.fn()
        }
      }
    })
    globalThis.document = createMockDocument()
  })

  afterEach(() => {
    globalThis.window = originalWindow
  })

  test('should record enhanced action on popstate', async () => {
    const { installNavigationCapture, uninstallNavigationCapture } = await import('../../../extension/lib/page/actions.js')
    const { getEnhancedActionBuffer, clearEnhancedActionBuffer } = await import('../../../extension/lib/page/reproduction.js')

    clearEnhancedActionBuffer()

    installNavigationCapture()

    // Find the popstate handler
    const addListenerCalls = globalThis.window.addEventListener.mock.calls
    const popstateHandler = addListenerCalls.find((call) => call.arguments[0] === 'popstate')
    assert.ok(popstateHandler, 'Should have registered popstate handler')

    // Simulate popstate
    globalThis.window.location.href = 'http://localhost:3000/new-page'
    popstateHandler.arguments[1]({ state: {} })

    const enhanced = getEnhancedActionBuffer()
    const navAction = enhanced.find((a) => a.type === 'navigate')
    assert.ok(navAction, 'Should have navigate action')
    assert.ok(navAction.to_url, 'Should have to_url')

    uninstallNavigationCapture()
    clearEnhancedActionBuffer()
  })

  test('should record enhanced action on pushState', async () => {
    const { installNavigationCapture, uninstallNavigationCapture } = await import('../../../extension/lib/page/actions.js')
    const { getEnhancedActionBuffer, clearEnhancedActionBuffer } = await import('../../../extension/lib/page/reproduction.js')

    clearEnhancedActionBuffer()

    const _originalPushState = globalThis.window.history.pushState

    installNavigationCapture()

    // Call the patched pushState
    globalThis.window.history.pushState({}, '', '/dashboard')

    const enhanced = getEnhancedActionBuffer()
    const navAction = enhanced.find((a) => a.type === 'navigate')
    assert.ok(navAction, 'pushState should trigger navigate action')
    assert.strictEqual(navAction.to_url, '/dashboard')

    uninstallNavigationCapture()
    clearEnhancedActionBuffer()
  })

  test('should record enhanced action on replaceState', async () => {
    const { installNavigationCapture, uninstallNavigationCapture } = await import('../../../extension/lib/page/actions.js')
    const { getEnhancedActionBuffer, clearEnhancedActionBuffer } = await import('../../../extension/lib/page/reproduction.js')

    clearEnhancedActionBuffer()

    installNavigationCapture()

    // Call the patched replaceState
    globalThis.window.history.replaceState({}, '', '/login')

    const enhanced = getEnhancedActionBuffer()
    const navAction = enhanced.find((a) => a.type === 'navigate')
    assert.ok(navAction, 'replaceState should trigger navigate action')
    assert.strictEqual(navAction.to_url, '/login')

    uninstallNavigationCapture()
    clearEnhancedActionBuffer()
  })

  test('navigate action should include from_url', async () => {
    const { installNavigationCapture, uninstallNavigationCapture } = await import('../../../extension/lib/page/actions.js')
    const { getEnhancedActionBuffer, clearEnhancedActionBuffer } = await import('../../../extension/lib/page/reproduction.js')

    clearEnhancedActionBuffer()

    globalThis.window.location.href = 'http://localhost:3000/home'

    installNavigationCapture()

    globalThis.window.history.pushState({}, '', '/about')

    const enhanced = getEnhancedActionBuffer()
    const navAction = enhanced.find((a) => a.type === 'navigate')
    assert.ok(navAction)
    assert.strictEqual(navAction.from_url, 'http://localhost:3000/home')

    uninstallNavigationCapture()
    clearEnhancedActionBuffer()
  })
})
