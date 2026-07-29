// @ts-nocheck
/**
 * @fileoverview Reproduction selector computation and enhanced action recording tests.
 */

import { test, describe, mock, beforeEach, afterEach } from 'node:test'
import assert from 'node:assert'
import { createElement } from './reproduction-script-fixture.js'

let originalWindow


describe('Selector Computation', () => {
  test('should prioritize data-testid', async () => {
    const { computeSelectors } = await import('../../../extension/lib/page/reproduction.js')

    const el = createElement('button', { 'data-testid': 'login-btn' })

    const selectors = computeSelectors(el)

    assert.strictEqual(selectors.testId, 'login-btn')
  })

  test('should accept data-test-id variant', async () => {
    const { computeSelectors } = await import('../../../extension/lib/page/reproduction.js')

    const el = createElement('button', { 'data-test-id': 'submit-form' })

    const selectors = computeSelectors(el)

    assert.strictEqual(selectors.testId, 'submit-form')
  })

  test('should accept data-cy variant', async () => {
    const { computeSelectors } = await import('../../../extension/lib/page/reproduction.js')

    const el = createElement('button', { 'data-cy': 'next-step' })

    const selectors = computeSelectors(el)

    assert.strictEqual(selectors.testId, 'next-step')
  })

  test('should extract aria-label', async () => {
    const { computeSelectors } = await import('../../../extension/lib/page/reproduction.js')

    const el = createElement('button', { 'aria-label': 'Close dialog' })

    const selectors = computeSelectors(el)

    assert.strictEqual(selectors.ariaLabel, 'Close dialog')
  })

  test('should extract explicit role + accessible name', async () => {
    const { computeSelectors } = await import('../../../extension/lib/page/reproduction.js')

    const el = createElement('div', { role: 'button', 'aria-label': 'Submit' })

    const selectors = computeSelectors(el)

    assert.deepStrictEqual(selectors.role, { role: 'button', name: 'Submit' })
  })

  test('should extract id when present', async () => {
    const { computeSelectors } = await import('../../../extension/lib/page/reproduction.js')

    const el = createElement('input', { id: 'email-field' })

    const selectors = computeSelectors(el)

    assert.strictEqual(selectors.id, 'email-field')
  })

  test('should extract visible text for clickable elements', async () => {
    const { computeSelectors } = await import('../../../extension/lib/page/reproduction.js')

    const el = createElement('button', {}, { textContent: 'Sign In' })

    const selectors = computeSelectors(el)

    assert.strictEqual(selectors.text, 'Sign In')
  })

  test('should not extract text for non-clickable elements', async () => {
    const { computeSelectors } = await import('../../../extension/lib/page/reproduction.js')

    const el = createElement('div', {}, { textContent: 'Just a div' })

    const selectors = computeSelectors(el)

    assert.strictEqual(selectors.text, undefined)
  })

  test('should truncate text at 50 chars', async () => {
    const { computeSelectors } = await import('../../../extension/lib/page/reproduction.js')

    const el = createElement('button', {}, { textContent: 'x'.repeat(80) })

    const selectors = computeSelectors(el)

    if (selectors.text) {
      assert.ok(selectors.text.length <= 50)
    }
  })

  test('should include cssPath as last resort', async () => {
    const { computeSelectors } = await import('../../../extension/lib/page/reproduction.js')

    const el = createElement('button', { class: 'submit-btn' })

    const selectors = computeSelectors(el)

    assert.ok(selectors.cssPath, 'Expected cssPath to be computed')
  })

  test('should compute all available selectors', async () => {
    const { computeSelectors } = await import('../../../extension/lib/page/reproduction.js')

    const el = createElement(
      'button',
      {
        'data-testid': 'login',
        'aria-label': 'Log in',
        id: 'login-btn',
        class: 'btn primary'
      },
      { textContent: 'Log In' }
    )

    const selectors = computeSelectors(el)

    assert.ok(selectors.testId)
    assert.ok(selectors.ariaLabel)
    assert.ok(selectors.id)
    assert.ok(selectors.text)
    assert.ok(selectors.cssPath)
  })

  test('should handle element with no identifiers', async () => {
    const { computeSelectors } = await import('../../../extension/lib/page/reproduction.js')

    const el = createElement('div', {})

    const selectors = computeSelectors(el)

    // Should at least have cssPath
    assert.ok(selectors.cssPath)
    assert.strictEqual(selectors.testId, undefined)
    assert.strictEqual(selectors.ariaLabel, undefined)
    assert.strictEqual(selectors.id, undefined)
  })
})

// --- Implicit Role Mapping ---

describe('Implicit Role Mapping', () => {
  test('should map button to button role', async () => {
    const { getImplicitRole } = await import('../../../extension/lib/page/reproduction.js')

    const el = createElement('button', {})
    assert.strictEqual(getImplicitRole(el), 'button')
  })

  test('should map anchor with href to link role', async () => {
    const { getImplicitRole } = await import('../../../extension/lib/page/reproduction.js')

    const el = createElement('a', { href: '/page' })
    assert.strictEqual(getImplicitRole(el), 'link')
  })

  test('should map anchor without href to null', async () => {
    const { getImplicitRole } = await import('../../../extension/lib/page/reproduction.js')

    const el = createElement('a', {})
    assert.strictEqual(getImplicitRole(el), null)
  })

  test('should map input[type=text] to textbox', async () => {
    const { getImplicitRole } = await import('../../../extension/lib/page/reproduction.js')

    const el = createElement('input', { type: 'text' })
    assert.strictEqual(getImplicitRole(el), 'textbox')
  })

  test('should map input[type=email] to textbox', async () => {
    const { getImplicitRole } = await import('../../../extension/lib/page/reproduction.js')

    const el = createElement('input', { type: 'email' })
    assert.strictEqual(getImplicitRole(el), 'textbox')
  })

  test('should map input[type=checkbox] to checkbox', async () => {
    const { getImplicitRole } = await import('../../../extension/lib/page/reproduction.js')

    const el = createElement('input', { type: 'checkbox' })
    assert.strictEqual(getImplicitRole(el), 'checkbox')
  })

  test('should map input[type=radio] to radio', async () => {
    const { getImplicitRole } = await import('../../../extension/lib/page/reproduction.js')

    const el = createElement('input', { type: 'radio' })
    assert.strictEqual(getImplicitRole(el), 'radio')
  })

  test('should map input[type=search] to searchbox', async () => {
    const { getImplicitRole } = await import('../../../extension/lib/page/reproduction.js')

    const el = createElement('input', { type: 'search' })
    assert.strictEqual(getImplicitRole(el), 'searchbox')
  })

  test('should map textarea to textbox', async () => {
    const { getImplicitRole } = await import('../../../extension/lib/page/reproduction.js')

    const el = createElement('textarea', {})
    assert.strictEqual(getImplicitRole(el), 'textbox')
  })

  test('should map select to combobox', async () => {
    const { getImplicitRole } = await import('../../../extension/lib/page/reproduction.js')

    const el = createElement('select', {})
    assert.strictEqual(getImplicitRole(el), 'combobox')
  })

  test('should map nav to navigation', async () => {
    const { getImplicitRole } = await import('../../../extension/lib/page/reproduction.js')

    const el = createElement('nav', {})
    assert.strictEqual(getImplicitRole(el), 'navigation')
  })

  test('should map main to main', async () => {
    const { getImplicitRole } = await import('../../../extension/lib/page/reproduction.js')

    const el = createElement('main', {})
    assert.strictEqual(getImplicitRole(el), 'main')
  })

  test('should return null for unknown elements', async () => {
    const { getImplicitRole } = await import('../../../extension/lib/page/reproduction.js')

    const el = createElement('div', {})
    assert.strictEqual(getImplicitRole(el), null)
  })

  test('should default input without type to textbox', async () => {
    const { getImplicitRole } = await import('../../../extension/lib/page/reproduction.js')

    const el = createElement('input', {}) // No type attribute
    assert.strictEqual(getImplicitRole(el), 'textbox')
  })
})

// --- Dynamic Class Detection ---

describe('Dynamic Class Detection', () => {
  test('should detect css-* prefix as dynamic', async () => {
    const { isDynamicClass } = await import('../../../extension/lib/page/reproduction.js')

    assert.strictEqual(isDynamicClass('css-1a2b3c'), true)
  })

  test('should detect sc-* prefix as dynamic (styled-components)', async () => {
    const { isDynamicClass } = await import('../../../extension/lib/page/reproduction.js')

    assert.strictEqual(isDynamicClass('sc-bdnxRM'), true)
  })

  test('should detect emotion-* prefix as dynamic', async () => {
    const { isDynamicClass } = await import('../../../extension/lib/page/reproduction.js')

    assert.strictEqual(isDynamicClass('emotion-abc123'), true)
  })

  test('should detect styled-* prefix as dynamic', async () => {
    const { isDynamicClass } = await import('../../../extension/lib/page/reproduction.js')

    assert.strictEqual(isDynamicClass('styled-xyz789'), true)
  })

  test('should detect chakra-* prefix as dynamic', async () => {
    const { isDynamicClass } = await import('../../../extension/lib/page/reproduction.js')

    assert.strictEqual(isDynamicClass('chakra-button'), true)
  })

  test('should detect random hash classes as dynamic', async () => {
    const { isDynamicClass } = await import('../../../extension/lib/page/reproduction.js')

    // 5-8 lowercase chars that look like generated hashes
    assert.strictEqual(isDynamicClass('abcdef'), true)
    assert.strictEqual(isDynamicClass('xyzabcde'), true)
  })

  test('should not flag real class names', async () => {
    const { isDynamicClass } = await import('../../../extension/lib/page/reproduction.js')

    assert.strictEqual(isDynamicClass('btn'), false)
    assert.strictEqual(isDynamicClass('container'), false)
    assert.strictEqual(isDynamicClass('user-profile'), false)
    assert.strictEqual(isDynamicClass('is-active'), false)
    assert.strictEqual(isDynamicClass('form-control'), false)
  })

  test('should not flag short classes', async () => {
    const { isDynamicClass } = await import('../../../extension/lib/page/reproduction.js')

    assert.strictEqual(isDynamicClass('btn'), false)
    assert.strictEqual(isDynamicClass('card'), false)
  })

  test('should not flag classes with uppercase or numbers', async () => {
    const { isDynamicClass } = await import('../../../extension/lib/page/reproduction.js')

    assert.strictEqual(isDynamicClass('Button'), false)
    assert.strictEqual(isDynamicClass('col-12'), false)
  })
})

// --- CSS Path Computation ---

describe('CSS Path Computation', () => {
  test('should generate tag-based path', async () => {
    const { computeCssPath } = await import('../../../extension/lib/page/reproduction.js')

    const parent = createElement('form', { id: 'login' })
    const el = createElement('button', { class: 'submit' }, { parent })

    const path = computeCssPath(el)

    assert.ok(path.includes('button'))
  })

  test('should stop at element with ID', async () => {
    const { computeCssPath } = await import('../../../extension/lib/page/reproduction.js')

    const root = createElement('div', {})
    const parent = createElement('form', { id: 'myform' }, { parent: root })
    const el = createElement('input', { class: 'field' }, { parent })

    const path = computeCssPath(el)

    assert.ok(path.includes('#myform'))
    // Should not go above the ID element
    assert.ok(!path.includes('div'))
  })

  test('should limit depth to 5 levels', async () => {
    const { computeCssPath } = await import('../../../extension/lib/page/reproduction.js')

    // Build 8-deep nesting
    let current = createElement('div', { class: 'root' })
    for (let i = 0; i < 7; i++) {
      const child = createElement('div', { class: `level${i}` }, { parent: current })
      current = child
    }

    const path = computeCssPath(current)

    const parts = path.split(' > ')
    assert.ok(parts.length <= 5, `Expected max 5 parts, got ${parts.length}`)
  })

  test('should filter dynamic classes from path', async () => {
    const { computeCssPath } = await import('../../../extension/lib/page/reproduction.js')

    const el = createElement('div', { class: 'container css-abc123 styled-xyz' })

    const path = computeCssPath(el)

    assert.ok(!path.includes('css-abc123'))
    assert.ok(!path.includes('styled-xyz'))
    assert.ok(path.includes('container'))
  })

  test('should include max 2 classes per element', async () => {
    const { computeCssPath } = await import('../../../extension/lib/page/reproduction.js')

    const el = createElement('div', { class: 'a b c d e' })

    const path = computeCssPath(el)

    // Count class selectors (dots) in the element's portion
    const classDots = (path.match(/\./g) || []).length
    assert.ok(classDots <= 2)
  })
})

// --- Enhanced Action Recording ---

describe('Enhanced Action Recording', () => {
  beforeEach(() => {
    originalWindow = globalThis.window
    globalThis.window = {
      postMessage: mock.fn(),
      addEventListener: mock.fn(),
      location: { href: 'http://localhost:3000/app' }
    }
  })

  afterEach(() => {
    globalThis.window = originalWindow
  })

  test('should record click with multi-strategy selectors', async () => {
    const { recordEnhancedAction } = await import('../../../extension/lib/page/reproduction.js')

    const el = createElement(
      'button',
      {
        'data-testid': 'submit',
        'aria-label': 'Submit form'
      },
      { textContent: 'Submit' }
    )

    const action = recordEnhancedAction('click', el)

    assert.strictEqual(action.type, 'click')
    assert.ok(action.selectors)
    assert.strictEqual(action.selectors.testId, 'submit')
    assert.strictEqual(action.selectors.ariaLabel, 'Submit form')
    assert.ok(action.url)
    assert.ok(action.timestamp)
  })

  test('should record input with value (non-sensitive)', async () => {
    const { recordEnhancedAction } = await import('../../../extension/lib/page/reproduction.js')

    const el = createElement('input', {
      type: 'email',
      'data-testid': 'email-input'
    })

    const action = recordEnhancedAction('input', el, { value: 'user@test.com' })

    assert.strictEqual(action.type, 'input')
    assert.strictEqual(action.value, 'user@test.com')
    assert.strictEqual(action.input_type, 'email')
  })

  test('should redact password input values', async () => {
    const { recordEnhancedAction } = await import('../../../extension/lib/page/reproduction.js')

    const el = createElement('input', { type: 'password', 'data-testid': 'pw' })

    const action = recordEnhancedAction('input', el, { value: 'secret123' })

    assert.strictEqual(action.value, '[redacted]')
  })

  test('should record keypress events', async () => {
    const { recordEnhancedAction } = await import('../../../extension/lib/page/reproduction.js')

    const el = createElement('input', { 'data-testid': 'search' })

    const action = recordEnhancedAction('keypress', el, { key: 'Enter' })

    assert.strictEqual(action.type, 'keypress')
    assert.strictEqual(action.key, 'Enter')
  })

  test('should record navigation events', async () => {
    const { recordEnhancedAction } = await import('../../../extension/lib/page/reproduction.js')

    const action = recordEnhancedAction('navigate', null, {
      from_url: 'http://localhost:3000/login',
      to_url: 'http://localhost:3000/dashboard'
    })

    assert.strictEqual(action.type, 'navigate')
    assert.strictEqual(action.from_url, 'http://localhost:3000/login')
    assert.strictEqual(action.to_url, 'http://localhost:3000/dashboard')
  })

  test('should record select changes', async () => {
    const { recordEnhancedAction } = await import('../../../extension/lib/page/reproduction.js')

    const el = createElement('select', { 'data-testid': 'country' })

    const action = recordEnhancedAction('select', el, {
      selected_value: 'us',
      selected_text: 'United States'
    })

    assert.strictEqual(action.type, 'select')
    assert.strictEqual(action.selected_value, 'us')
    assert.strictEqual(action.selected_text, 'United States')
  })

  test('should include current URL with each action', async () => {
    const { recordEnhancedAction } = await import('../../../extension/lib/page/reproduction.js')

    globalThis.window.location = { href: 'http://localhost:3000/page' }
    const el = createElement('button', {})

    const action = recordEnhancedAction('click', el)

    assert.strictEqual(action.url, 'http://localhost:3000/page')
  })

  test('should buffer up to 50 actions', async () => {
    const { recordEnhancedAction, getEnhancedActionBuffer } = await import('../../../extension/lib/page/reproduction.js')

    for (let i = 0; i < 60; i++) {
      const el = createElement('button', { 'data-testid': `btn-${i}` })
      recordEnhancedAction('click', el)
    }

    const buffer = getEnhancedActionBuffer()
    assert.ok(buffer.length <= 50)
  })

  test('should drop oldest actions when buffer is full', async () => {
    const { recordEnhancedAction, getEnhancedActionBuffer, clearEnhancedActionBuffer } =
      await import('../../../extension/lib/page/reproduction.js')

    clearEnhancedActionBuffer()

    for (let i = 0; i < 55; i++) {
      const el = createElement('button', { 'data-testid': `btn-${i}` })
      recordEnhancedAction('click', el)
    }

    const buffer = getEnhancedActionBuffer()
    // Should have the latest actions, not the earliest
    const lastAction = buffer[buffer.length - 1]
    assert.strictEqual(lastAction.selectors.testId, 'btn-54')
  })
})

// --- Playwright Script Generation ---
