// @ts-nocheck
/**
 * @fileoverview reproduction-script.test.js — Tests for Playwright repro script generation.
 * Verifies that recorded user actions are converted into runnable Playwright test
 * scripts with smart selector strategies, timing annotations, and sensitive data redaction.
 *
 * Tests cover:
 * - Selector computation (multi-strategy: testId, aria, role, id, text, cssPath)
 * - CSS path generation (depth limit, dynamic class filtering, nth-child)
 * - Implicit role mapping
 * - Dynamic class detection
 * - Enhanced action recording (new action types)
 * - Playwright script generation (locator priority, action mapping, timing)
 * - Sensitive data handling (redaction, warnings)
 * - Edge cases (empty buffer, single action, max buffer)
 */

import { test, describe, mock, beforeEach, afterEach } from 'node:test'
import assert from 'node:assert'

let originalWindow

// Mock DOM elements
function createElement(tag, attrs = {}, opts = {}) {
  const el = {
    tagName: tag.toUpperCase(),
    id: attrs.id || '',
    className: attrs.class || '',
    classList: { from: [] },
    textContent: opts.textContent || '',
    innerText: opts.innerText || opts.textContent || '',
    parentElement: opts.parent || null,
    children: opts.children || [],
    childNodes: opts.children || [],
    getAttribute: (name) => {
      if (name === 'data-testid') return attrs['data-testid'] || null
      if (name === 'data-test-id') return attrs['data-test-id'] || null
      if (name === 'data-cy') return attrs['data-cy'] || null
      if (name === 'aria-label') return attrs['aria-label'] || null
      if (name === 'role') return attrs.role || null
      if (name === 'type') return attrs.type || null
      if (name === 'href') return attrs.href || null
      return attrs[name] || null
    },
    hasAttribute: (name) => name in attrs,
    querySelectorAll: mock.fn(() => [])
  }
  el.classList = Array.from(new Set((attrs.class || '').split(' ').filter(Boolean)))
  el.classList.from = el.classList
  return el
}

// --- Selector Computation ---

describe('Selector Computation', () => {
  test('should prioritize data-testid', async () => {
    const { computeSelectors } = await import('../../extension/lib/page/reproduction.js')

    const el = createElement('button', { 'data-testid': 'login-btn' })

    const selectors = computeSelectors(el)

    assert.strictEqual(selectors.testId, 'login-btn')
  })

  test('should accept data-test-id variant', async () => {
    const { computeSelectors } = await import('../../extension/lib/page/reproduction.js')

    const el = createElement('button', { 'data-test-id': 'submit-form' })

    const selectors = computeSelectors(el)

    assert.strictEqual(selectors.testId, 'submit-form')
  })

  test('should accept data-cy variant', async () => {
    const { computeSelectors } = await import('../../extension/lib/page/reproduction.js')

    const el = createElement('button', { 'data-cy': 'next-step' })

    const selectors = computeSelectors(el)

    assert.strictEqual(selectors.testId, 'next-step')
  })

  test('should extract aria-label', async () => {
    const { computeSelectors } = await import('../../extension/lib/page/reproduction.js')

    const el = createElement('button', { 'aria-label': 'Close dialog' })

    const selectors = computeSelectors(el)

    assert.strictEqual(selectors.ariaLabel, 'Close dialog')
  })

  test('should extract explicit role + accessible name', async () => {
    const { computeSelectors } = await import('../../extension/lib/page/reproduction.js')

    const el = createElement('div', { role: 'button', 'aria-label': 'Submit' })

    const selectors = computeSelectors(el)

    assert.deepStrictEqual(selectors.role, { role: 'button', name: 'Submit' })
  })

  test('should extract id when present', async () => {
    const { computeSelectors } = await import('../../extension/lib/page/reproduction.js')

    const el = createElement('input', { id: 'email-field' })

    const selectors = computeSelectors(el)

    assert.strictEqual(selectors.id, 'email-field')
  })

  test('should extract visible text for clickable elements', async () => {
    const { computeSelectors } = await import('../../extension/lib/page/reproduction.js')

    const el = createElement('button', {}, { textContent: 'Sign In' })

    const selectors = computeSelectors(el)

    assert.strictEqual(selectors.text, 'Sign In')
  })

  test('should not extract text for non-clickable elements', async () => {
    const { computeSelectors } = await import('../../extension/lib/page/reproduction.js')

    const el = createElement('div', {}, { textContent: 'Just a div' })

    const selectors = computeSelectors(el)

    assert.strictEqual(selectors.text, undefined)
  })

  test('should truncate text at 50 chars', async () => {
    const { computeSelectors } = await import('../../extension/lib/page/reproduction.js')

    const el = createElement('button', {}, { textContent: 'x'.repeat(80) })

    const selectors = computeSelectors(el)

    if (selectors.text) {
      assert.ok(selectors.text.length <= 50)
    }
  })

  test('should include cssPath as last resort', async () => {
    const { computeSelectors } = await import('../../extension/lib/page/reproduction.js')

    const el = createElement('button', { class: 'submit-btn' })

    const selectors = computeSelectors(el)

    assert.ok(selectors.cssPath, 'Expected cssPath to be computed')
  })

  test('should compute all available selectors', async () => {
    const { computeSelectors } = await import('../../extension/lib/page/reproduction.js')

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
    const { computeSelectors } = await import('../../extension/lib/page/reproduction.js')

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
    const { getImplicitRole } = await import('../../extension/lib/page/reproduction.js')

    const el = createElement('button', {})
    assert.strictEqual(getImplicitRole(el), 'button')
  })

  test('should map anchor with href to link role', async () => {
    const { getImplicitRole } = await import('../../extension/lib/page/reproduction.js')

    const el = createElement('a', { href: '/page' })
    assert.strictEqual(getImplicitRole(el), 'link')
  })

  test('should map anchor without href to null', async () => {
    const { getImplicitRole } = await import('../../extension/lib/page/reproduction.js')

    const el = createElement('a', {})
    assert.strictEqual(getImplicitRole(el), null)
  })

  test('should map input[type=text] to textbox', async () => {
    const { getImplicitRole } = await import('../../extension/lib/page/reproduction.js')

    const el = createElement('input', { type: 'text' })
    assert.strictEqual(getImplicitRole(el), 'textbox')
  })

  test('should map input[type=email] to textbox', async () => {
    const { getImplicitRole } = await import('../../extension/lib/page/reproduction.js')

    const el = createElement('input', { type: 'email' })
    assert.strictEqual(getImplicitRole(el), 'textbox')
  })

  test('should map input[type=checkbox] to checkbox', async () => {
    const { getImplicitRole } = await import('../../extension/lib/page/reproduction.js')

    const el = createElement('input', { type: 'checkbox' })
    assert.strictEqual(getImplicitRole(el), 'checkbox')
  })

  test('should map input[type=radio] to radio', async () => {
    const { getImplicitRole } = await import('../../extension/lib/page/reproduction.js')

    const el = createElement('input', { type: 'radio' })
    assert.strictEqual(getImplicitRole(el), 'radio')
  })

  test('should map input[type=search] to searchbox', async () => {
    const { getImplicitRole } = await import('../../extension/lib/page/reproduction.js')

    const el = createElement('input', { type: 'search' })
    assert.strictEqual(getImplicitRole(el), 'searchbox')
  })

  test('should map textarea to textbox', async () => {
    const { getImplicitRole } = await import('../../extension/lib/page/reproduction.js')

    const el = createElement('textarea', {})
    assert.strictEqual(getImplicitRole(el), 'textbox')
  })

  test('should map select to combobox', async () => {
    const { getImplicitRole } = await import('../../extension/lib/page/reproduction.js')

    const el = createElement('select', {})
    assert.strictEqual(getImplicitRole(el), 'combobox')
  })

  test('should map nav to navigation', async () => {
    const { getImplicitRole } = await import('../../extension/lib/page/reproduction.js')

    const el = createElement('nav', {})
    assert.strictEqual(getImplicitRole(el), 'navigation')
  })

  test('should map main to main', async () => {
    const { getImplicitRole } = await import('../../extension/lib/page/reproduction.js')

    const el = createElement('main', {})
    assert.strictEqual(getImplicitRole(el), 'main')
  })

  test('should return null for unknown elements', async () => {
    const { getImplicitRole } = await import('../../extension/lib/page/reproduction.js')

    const el = createElement('div', {})
    assert.strictEqual(getImplicitRole(el), null)
  })

  test('should default input without type to textbox', async () => {
    const { getImplicitRole } = await import('../../extension/lib/page/reproduction.js')

    const el = createElement('input', {}) // No type attribute
    assert.strictEqual(getImplicitRole(el), 'textbox')
  })
})

// --- Dynamic Class Detection ---

describe('Dynamic Class Detection', () => {
  test('should detect css-* prefix as dynamic', async () => {
    const { isDynamicClass } = await import('../../extension/lib/page/reproduction.js')

    assert.strictEqual(isDynamicClass('css-1a2b3c'), true)
  })

  test('should detect sc-* prefix as dynamic (styled-components)', async () => {
    const { isDynamicClass } = await import('../../extension/lib/page/reproduction.js')

    assert.strictEqual(isDynamicClass('sc-bdnxRM'), true)
  })

  test('should detect emotion-* prefix as dynamic', async () => {
    const { isDynamicClass } = await import('../../extension/lib/page/reproduction.js')

    assert.strictEqual(isDynamicClass('emotion-abc123'), true)
  })

  test('should detect styled-* prefix as dynamic', async () => {
    const { isDynamicClass } = await import('../../extension/lib/page/reproduction.js')

    assert.strictEqual(isDynamicClass('styled-xyz789'), true)
  })

  test('should detect chakra-* prefix as dynamic', async () => {
    const { isDynamicClass } = await import('../../extension/lib/page/reproduction.js')

    assert.strictEqual(isDynamicClass('chakra-button'), true)
  })

  test('should detect random hash classes as dynamic', async () => {
    const { isDynamicClass } = await import('../../extension/lib/page/reproduction.js')

    // 5-8 lowercase chars that look like generated hashes
    assert.strictEqual(isDynamicClass('abcdef'), true)
    assert.strictEqual(isDynamicClass('xyzabcde'), true)
  })

  test('should not flag real class names', async () => {
    const { isDynamicClass } = await import('../../extension/lib/page/reproduction.js')

    assert.strictEqual(isDynamicClass('btn'), false)
    assert.strictEqual(isDynamicClass('container'), false)
    assert.strictEqual(isDynamicClass('user-profile'), false)
    assert.strictEqual(isDynamicClass('is-active'), false)
    assert.strictEqual(isDynamicClass('form-control'), false)
  })

  test('should not flag short classes', async () => {
    const { isDynamicClass } = await import('../../extension/lib/page/reproduction.js')

    assert.strictEqual(isDynamicClass('btn'), false)
    assert.strictEqual(isDynamicClass('card'), false)
  })

  test('should not flag classes with uppercase or numbers', async () => {
    const { isDynamicClass } = await import('../../extension/lib/page/reproduction.js')

    assert.strictEqual(isDynamicClass('Button'), false)
    assert.strictEqual(isDynamicClass('col-12'), false)
  })
})

// --- CSS Path Computation ---

describe('CSS Path Computation', () => {
  test('should generate tag-based path', async () => {
    const { computeCssPath } = await import('../../extension/lib/page/reproduction.js')

    const parent = createElement('form', { id: 'login' })
    const el = createElement('button', { class: 'submit' }, { parent })

    const path = computeCssPath(el)

    assert.ok(path.includes('button'))
  })

  test('should stop at element with ID', async () => {
    const { computeCssPath } = await import('../../extension/lib/page/reproduction.js')

    const root = createElement('div', {})
    const parent = createElement('form', { id: 'myform' }, { parent: root })
    const el = createElement('input', { class: 'field' }, { parent })

    const path = computeCssPath(el)

    assert.ok(path.includes('#myform'))
    // Should not go above the ID element
    assert.ok(!path.includes('div'))
  })

  test('should limit depth to 5 levels', async () => {
    const { computeCssPath } = await import('../../extension/lib/page/reproduction.js')

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
    const { computeCssPath } = await import('../../extension/lib/page/reproduction.js')

    const el = createElement('div', { class: 'container css-abc123 styled-xyz' })

    const path = computeCssPath(el)

    assert.ok(!path.includes('css-abc123'))
    assert.ok(!path.includes('styled-xyz'))
    assert.ok(path.includes('container'))
  })

  test('should include max 2 classes per element', async () => {
    const { computeCssPath } = await import('../../extension/lib/page/reproduction.js')

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
    const { recordEnhancedAction } = await import('../../extension/lib/page/reproduction.js')

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
    const { recordEnhancedAction } = await import('../../extension/lib/page/reproduction.js')

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
    const { recordEnhancedAction } = await import('../../extension/lib/page/reproduction.js')

    const el = createElement('input', { type: 'password', 'data-testid': 'pw' })

    const action = recordEnhancedAction('input', el, { value: 'secret123' })

    assert.strictEqual(action.value, '[redacted]')
  })

  test('should record keypress events', async () => {
    const { recordEnhancedAction } = await import('../../extension/lib/page/reproduction.js')

    const el = createElement('input', { 'data-testid': 'search' })

    const action = recordEnhancedAction('keypress', el, { key: 'Enter' })

    assert.strictEqual(action.type, 'keypress')
    assert.strictEqual(action.key, 'Enter')
  })

  test('should record navigation events', async () => {
    const { recordEnhancedAction } = await import('../../extension/lib/page/reproduction.js')

    const action = recordEnhancedAction('navigate', null, {
      from_url: 'http://localhost:3000/login',
      to_url: 'http://localhost:3000/dashboard'
    })

    assert.strictEqual(action.type, 'navigate')
    assert.strictEqual(action.from_url, 'http://localhost:3000/login')
    assert.strictEqual(action.to_url, 'http://localhost:3000/dashboard')
  })

  test('should record select changes', async () => {
    const { recordEnhancedAction } = await import('../../extension/lib/page/reproduction.js')

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
    const { recordEnhancedAction } = await import('../../extension/lib/page/reproduction.js')

    globalThis.window.location = { href: 'http://localhost:3000/page' }
    const el = createElement('button', {})

    const action = recordEnhancedAction('click', el)

    assert.strictEqual(action.url, 'http://localhost:3000/page')
  })

  test('should buffer up to 50 actions', async () => {
    const { recordEnhancedAction, getEnhancedActionBuffer } = await import('../../extension/lib/page/reproduction.js')

    for (let i = 0; i < 60; i++) {
      const el = createElement('button', { 'data-testid': `btn-${i}` })
      recordEnhancedAction('click', el)
    }

    const buffer = getEnhancedActionBuffer()
    assert.ok(buffer.length <= 50)
  })

  test('should drop oldest actions when buffer is full', async () => {
    const { recordEnhancedAction, getEnhancedActionBuffer, clearEnhancedActionBuffer } =
      await import('../../extension/lib/page/reproduction.js')

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

describe('Playwright Script Generation', () => {
  test('should generate valid Playwright test structure', async () => {
    const { generatePlaywrightScript } = await import('../../extension/lib/page/reproduction.js')

    const actions = [
      { type: 'click', selectors: { testId: 'login-btn' }, url: 'http://localhost:3000/login', timestamp: 1000 }
    ]

    const script = generatePlaywrightScript(actions, { errorMessage: 'Test error' })

    assert.ok(script.includes("import { test, expect } from '@playwright/test'"))
    assert.ok(script.includes('test('))
    assert.ok(script.includes('async ({ page })'))
    assert.ok(script.includes('page.goto'))
  })

  test('should use getByTestId when testId available', async () => {
    const { generatePlaywrightScript } = await import('../../extension/lib/page/reproduction.js')

    const actions = [
      {
        type: 'click',
        selectors: { testId: 'submit', cssPath: 'button.btn' },
        url: 'http://localhost:3000',
        timestamp: 1000
      }
    ]

    const script = generatePlaywrightScript(actions, {})

    assert.ok(script.includes("getByTestId('submit')"))
    assert.ok(!script.includes('button.btn')) // Should prefer testId
  })

  test('should use getByRole when role available and no testId', async () => {
    const { generatePlaywrightScript } = await import('../../extension/lib/page/reproduction.js')

    const actions = [
      {
        type: 'click',
        selectors: { role: { role: 'button', name: 'Submit' }, cssPath: 'button' },
        url: 'http://localhost:3000',
        timestamp: 1000
      }
    ]

    const script = generatePlaywrightScript(actions, {})

    assert.ok(script.includes("getByRole('button', { name: 'Submit' })"))
  })

  test('should use getByLabel when ariaLabel available', async () => {
    const { generatePlaywrightScript } = await import('../../extension/lib/page/reproduction.js')

    const actions = [
      {
        type: 'input',
        selectors: { ariaLabel: 'Email address' },
        value: 'test@test.com',
        url: 'http://localhost:3000',
        timestamp: 1000
      }
    ]

    const script = generatePlaywrightScript(actions, {})

    assert.ok(script.includes("getByLabel('Email address')"))
  })

  test('should use getByText for clickable text', async () => {
    const { generatePlaywrightScript } = await import('../../extension/lib/page/reproduction.js')

    const actions = [{ type: 'click', selectors: { text: 'Next Step' }, url: 'http://localhost:3000', timestamp: 1000 }]

    const script = generatePlaywrightScript(actions, {})

    assert.ok(script.includes("getByText('Next Step')"))
  })

  test('should use locator with id', async () => {
    const { generatePlaywrightScript } = await import('../../extension/lib/page/reproduction.js')

    const actions = [
      {
        type: 'click',
        selectors: { id: 'main-nav', cssPath: 'nav#main-nav' },
        url: 'http://localhost:3000',
        timestamp: 1000
      }
    ]

    const script = generatePlaywrightScript(actions, {})

    assert.ok(script.includes("locator('#main-nav')"))
  })

  test('should fall back to cssPath locator', async () => {
    const { generatePlaywrightScript } = await import('../../extension/lib/page/reproduction.js')

    const actions = [
      { type: 'click', selectors: { cssPath: 'form > button.submit' }, url: 'http://localhost:3000', timestamp: 1000 }
    ]

    const script = generatePlaywrightScript(actions, {})

    assert.ok(script.includes("locator('form > button.submit')"))
  })

  test('should map click action to .click()', async () => {
    const { generatePlaywrightScript } = await import('../../extension/lib/page/reproduction.js')

    const actions = [{ type: 'click', selectors: { testId: 'btn' }, url: 'http://localhost:3000', timestamp: 1000 }]

    const script = generatePlaywrightScript(actions, {})

    assert.ok(script.includes('.click()'))
  })

  test('should map input action to .fill()', async () => {
    const { generatePlaywrightScript } = await import('../../extension/lib/page/reproduction.js')

    const actions = [
      { type: 'input', selectors: { testId: 'name' }, value: 'Alice', url: 'http://localhost:3000', timestamp: 1000 }
    ]

    const script = generatePlaywrightScript(actions, {})

    assert.ok(script.includes(".fill('Alice')"))
  })

  test('should map keypress to keyboard.press()', async () => {
    const { generatePlaywrightScript } = await import('../../extension/lib/page/reproduction.js')

    const actions = [
      {
        type: 'keypress',
        selectors: { testId: 'search' },
        key: 'Enter',
        url: 'http://localhost:3000',
        timestamp: 1000
      }
    ]

    const script = generatePlaywrightScript(actions, {})

    assert.ok(script.includes("keyboard.press('Enter')"))
  })

  test('should map select action to .selectOption()', async () => {
    const { generatePlaywrightScript } = await import('../../extension/lib/page/reproduction.js')

    const actions = [
      {
        type: 'select',
        selectors: { testId: 'country' },
        selected_value: 'us',
        url: 'http://localhost:3000',
        timestamp: 1000
      }
    ]

    const script = generatePlaywrightScript(actions, {})

    assert.ok(script.includes(".selectOption('us')"))
  })

  test('should add waitForURL on navigate actions', async () => {
    const { generatePlaywrightScript } = await import('../../extension/lib/page/reproduction.js')

    const actions = [
      { type: 'click', selectors: { testId: 'login-btn' }, url: 'http://localhost:3000/login', timestamp: 1000 },
      {
        type: 'navigate',
        from_url: 'http://localhost:3000/login',
        to_url: 'http://localhost:3000/dashboard',
        timestamp: 1500
      }
    ]

    const script = generatePlaywrightScript(actions, {})

    assert.ok(script.includes('waitForURL'))
    assert.ok(script.includes('/dashboard'))
  })

  test('should add comment for long pauses (> 2s)', async () => {
    const { generatePlaywrightScript } = await import('../../extension/lib/page/reproduction.js')

    const actions = [
      { type: 'click', selectors: { testId: 'btn1' }, url: 'http://localhost:3000', timestamp: 1000 },
      { type: 'click', selectors: { testId: 'btn2' }, url: 'http://localhost:3000', timestamp: 5000 } // 4s gap
    ]

    const script = generatePlaywrightScript(actions, {})

    assert.ok(script.includes('pause') || script.includes('4'), 'Expected pause comment for 4s gap')
  })

  test('should add scroll as comment only', async () => {
    const { generatePlaywrightScript } = await import('../../extension/lib/page/reproduction.js')

    const actions = [{ type: 'scroll', scroll_y: 500, url: 'http://localhost:3000', timestamp: 1000 }]

    const script = generatePlaywrightScript(actions, {})

    assert.ok(script.includes('//'))
    assert.ok(script.includes('scroll') || script.includes('500'))
  })

  test('should include error context comment at end', async () => {
    const { generatePlaywrightScript } = await import('../../extension/lib/page/reproduction.js')

    const actions = [{ type: 'click', selectors: { testId: 'btn' }, url: 'http://localhost:3000', timestamp: 1000 }]

    const script = generatePlaywrightScript(actions, {
      errorMessage: "Cannot read properties of undefined (reading 'user')"
    })

    assert.ok(script.includes('Cannot read properties of undefined'))
  })

  test('should use base_url override', async () => {
    const { generatePlaywrightScript } = await import('../../extension/lib/page/reproduction.js')

    const actions = [
      { type: 'click', selectors: { testId: 'btn' }, url: 'http://localhost:3000/page', timestamp: 1000 }
    ]

    const script = generatePlaywrightScript(actions, { baseUrl: 'http://localhost:4000' })

    assert.ok(script.includes('localhost:4000'))
    assert.ok(!script.includes('localhost:3000'))
  })
})

// --- Sensitive Data Handling ---

describe('Sensitive Data Handling', () => {
  test('should redact password field values in script', async () => {
    const { generatePlaywrightScript } = await import('../../extension/lib/page/reproduction.js')

    const actions = [
      {
        type: 'input',
        selectors: { testId: 'password' },
        value: '[redacted]',
        input_type: 'password',
        url: 'http://localhost:3000',
        timestamp: 1000
      }
    ]

    const result = generatePlaywrightScript(actions, {})

    assert.ok(result.includes('[user-provided]') || result.includes('[redacted]'))
    assert.ok(!result.includes('secret'))
  })

  test('should include warning for redacted fields', async () => {
    const { generatePlaywrightScript } = await import('../../extension/lib/page/reproduction.js')

    const actions = [
      {
        type: 'input',
        selectors: { testId: 'password' },
        value: '[redacted]',
        input_type: 'password',
        url: 'http://localhost:3000',
        timestamp: 1000
      }
    ]

    const result = generatePlaywrightScript(actions, {})

    // Should have a warning about redacted values in comments or metadata
    assert.ok(
      result.includes('redacted') || result.includes('user-provided') || result.includes('password'),
      'Expected warning about sensitive field'
    )
  })
})

// --- Edge Cases ---

describe('Script Generation Edge Cases', () => {
  test('should handle empty action buffer', async () => {
    const { generatePlaywrightScript } = await import('../../extension/lib/page/reproduction.js')

    const script = generatePlaywrightScript([], {})

    assert.ok(script.includes('test('))
    // Should still be valid Playwright syntax even with no steps
    assert.ok(script.includes("import { test, expect } from '@playwright/test'"))
  })

  test('should handle single action', async () => {
    const { generatePlaywrightScript } = await import('../../extension/lib/page/reproduction.js')

    const actions = [
      { type: 'click', selectors: { testId: 'only-btn' }, url: 'http://localhost:3000', timestamp: 1000 }
    ]

    const script = generatePlaywrightScript(actions, {})

    assert.ok(script.includes("getByTestId('only-btn')"))
    assert.ok(script.includes('.click()'))
  })

  test('should handle actions with no selectors', async () => {
    const { generatePlaywrightScript } = await import('../../extension/lib/page/reproduction.js')

    const actions = [{ type: 'click', selectors: {}, url: 'http://localhost:3000', timestamp: 1000 }]

    const script = generatePlaywrightScript(actions, {})

    // Should not crash, may add a comment about missing selector
    assert.ok(typeof script === 'string')
    assert.ok(script.length > 0)
  })

  test('should respect last_n_actions parameter', async () => {
    const { generatePlaywrightScript } = await import('../../extension/lib/page/reproduction.js')

    const actions = [
      { type: 'click', selectors: { testId: 'btn1' }, url: 'http://localhost:3000', timestamp: 1000 },
      { type: 'click', selectors: { testId: 'btn2' }, url: 'http://localhost:3000', timestamp: 2000 },
      { type: 'click', selectors: { testId: 'btn3' }, url: 'http://localhost:3000', timestamp: 3000 },
      { type: 'click', selectors: { testId: 'btn4' }, url: 'http://localhost:3000', timestamp: 4000 },
      { type: 'click', selectors: { testId: 'btn5' }, url: 'http://localhost:3000', timestamp: 5000 }
    ]

    const script = generatePlaywrightScript(actions, { lastNActions: 2 })

    // Should only include last 2 actions
    assert.ok(!script.includes('btn1'))
    assert.ok(!script.includes('btn2'))
    assert.ok(!script.includes('btn3'))
    assert.ok(script.includes('btn4') || script.includes('btn5'))
  })

  test('should use error message in test name', async () => {
    const { generatePlaywrightScript } = await import('../../extension/lib/page/reproduction.js')

    const actions = [{ type: 'click', selectors: { testId: 'btn' }, url: 'http://localhost:3000', timestamp: 1000 }]

    const script = generatePlaywrightScript(actions, { errorMessage: 'TypeError: foo is not a function' })

    assert.ok(script.includes('foo is not a function') || script.includes('TypeError'))
  })

  test('should cap output at 50KB', async () => {
    const { generatePlaywrightScript } = await import('../../extension/lib/page/reproduction.js')

    // Generate many actions
    const actions = Array.from({ length: 200 }, (_, i) => ({
      type: 'input',
      selectors: { testId: `field-${i}` },
      value: 'x'.repeat(200),
      url: 'http://localhost:3000',
      timestamp: i * 100
    }))

    const script = generatePlaywrightScript(actions, {})

    assert.ok(script.length <= 51200, `Expected <= 50KB, got ${script.length}`)
  })
})

// --- Selector Priority in Generated Script ---

describe('Selector Priority Order', () => {
  test('priority: testId > role > ariaLabel > text > id > cssPath', async () => {
    const { generatePlaywrightScript } = await import('../../extension/lib/page/reproduction.js')

    // All selectors available — should use testId
    const actions = [
      {
        type: 'click',
        selectors: {
          testId: 'my-btn',
          role: { role: 'button', name: 'Click' },
          ariaLabel: 'Click me',
          text: 'Click',
          id: 'btn-1',
          cssPath: 'button.btn'
        },
        url: 'http://localhost:3000',
        timestamp: 1000
      }
    ]

    const script = generatePlaywrightScript(actions, {})

    assert.ok(script.includes("getByTestId('my-btn')"))
  })

  test('should fall through to role when no testId', async () => {
    const { generatePlaywrightScript } = await import('../../extension/lib/page/reproduction.js')

    const actions = [
      {
        type: 'click',
        selectors: {
          role: { role: 'button', name: 'Save' },
          ariaLabel: 'Save changes',
          cssPath: 'button.save'
        },
        url: 'http://localhost:3000',
        timestamp: 1000
      }
    ]

    const script = generatePlaywrightScript(actions, {})

    assert.ok(script.includes("getByRole('button', { name: 'Save' })"))
  })

  test('should fall through to ariaLabel when no testId or role', async () => {
    const { generatePlaywrightScript } = await import('../../extension/lib/page/reproduction.js')

    const actions = [
      {
        type: 'input',
        selectors: {
          ariaLabel: 'Search',
          cssPath: 'input.search'
        },
        value: 'query',
        url: 'http://localhost:3000',
        timestamp: 1000
      }
    ]

    const script = generatePlaywrightScript(actions, {})

    assert.ok(script.includes("getByLabel('Search')"))
  })
})
