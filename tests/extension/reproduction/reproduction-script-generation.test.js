// @ts-nocheck
/**
 * @fileoverview Playwright generation, redaction, edge-case, and selector-priority tests.
 */

import { test, describe } from 'node:test'
import assert from 'node:assert'

describe('Playwright Script Generation', () => {
  test('should generate valid Playwright test structure', async () => {
    const { generatePlaywrightScript } = await import('../../../extension/lib/page/reproduction.js')

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
    const { generatePlaywrightScript } = await import('../../../extension/lib/page/reproduction.js')

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
    const { generatePlaywrightScript } = await import('../../../extension/lib/page/reproduction.js')

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
    const { generatePlaywrightScript } = await import('../../../extension/lib/page/reproduction.js')

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
    const { generatePlaywrightScript } = await import('../../../extension/lib/page/reproduction.js')

    const actions = [{ type: 'click', selectors: { text: 'Next Step' }, url: 'http://localhost:3000', timestamp: 1000 }]

    const script = generatePlaywrightScript(actions, {})

    assert.ok(script.includes("getByText('Next Step')"))
  })

  test('should use locator with id', async () => {
    const { generatePlaywrightScript } = await import('../../../extension/lib/page/reproduction.js')

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
    const { generatePlaywrightScript } = await import('../../../extension/lib/page/reproduction.js')

    const actions = [
      { type: 'click', selectors: { cssPath: 'form > button.submit' }, url: 'http://localhost:3000', timestamp: 1000 }
    ]

    const script = generatePlaywrightScript(actions, {})

    assert.ok(script.includes("locator('form > button.submit')"))
  })

  test('should map click action to .click()', async () => {
    const { generatePlaywrightScript } = await import('../../../extension/lib/page/reproduction.js')

    const actions = [{ type: 'click', selectors: { testId: 'btn' }, url: 'http://localhost:3000', timestamp: 1000 }]

    const script = generatePlaywrightScript(actions, {})

    assert.ok(script.includes('.click()'))
  })

  test('should map input action to .fill()', async () => {
    const { generatePlaywrightScript } = await import('../../../extension/lib/page/reproduction.js')

    const actions = [
      { type: 'input', selectors: { testId: 'name' }, value: 'Alice', url: 'http://localhost:3000', timestamp: 1000 }
    ]

    const script = generatePlaywrightScript(actions, {})

    assert.ok(script.includes(".fill('Alice')"))
  })

  test('should map keypress to keyboard.press()', async () => {
    const { generatePlaywrightScript } = await import('../../../extension/lib/page/reproduction.js')

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
    const { generatePlaywrightScript } = await import('../../../extension/lib/page/reproduction.js')

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
    const { generatePlaywrightScript } = await import('../../../extension/lib/page/reproduction.js')

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
    const { generatePlaywrightScript } = await import('../../../extension/lib/page/reproduction.js')

    const actions = [
      { type: 'click', selectors: { testId: 'btn1' }, url: 'http://localhost:3000', timestamp: 1000 },
      { type: 'click', selectors: { testId: 'btn2' }, url: 'http://localhost:3000', timestamp: 5000 } // 4s gap
    ]

    const script = generatePlaywrightScript(actions, {})

    assert.ok(script.includes('pause') || script.includes('4'), 'Expected pause comment for 4s gap')
  })

  test('should add scroll as comment only', async () => {
    const { generatePlaywrightScript } = await import('../../../extension/lib/page/reproduction.js')

    const actions = [{ type: 'scroll', scroll_y: 500, url: 'http://localhost:3000', timestamp: 1000 }]

    const script = generatePlaywrightScript(actions, {})

    assert.ok(script.includes('//'))
    assert.ok(script.includes('scroll') || script.includes('500'))
  })

  test('should include error context comment at end', async () => {
    const { generatePlaywrightScript } = await import('../../../extension/lib/page/reproduction.js')

    const actions = [{ type: 'click', selectors: { testId: 'btn' }, url: 'http://localhost:3000', timestamp: 1000 }]

    const script = generatePlaywrightScript(actions, {
      errorMessage: "Cannot read properties of undefined (reading 'user')"
    })

    assert.ok(script.includes('Cannot read properties of undefined'))
  })

  test('should use base_url override', async () => {
    const { generatePlaywrightScript } = await import('../../../extension/lib/page/reproduction.js')

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
    const { generatePlaywrightScript } = await import('../../../extension/lib/page/reproduction.js')

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
    const { generatePlaywrightScript } = await import('../../../extension/lib/page/reproduction.js')

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
    const { generatePlaywrightScript } = await import('../../../extension/lib/page/reproduction.js')

    const script = generatePlaywrightScript([], {})

    assert.ok(script.includes('test('))
    // Should still be valid Playwright syntax even with no steps
    assert.ok(script.includes("import { test, expect } from '@playwright/test'"))
  })

  test('should handle single action', async () => {
    const { generatePlaywrightScript } = await import('../../../extension/lib/page/reproduction.js')

    const actions = [
      { type: 'click', selectors: { testId: 'only-btn' }, url: 'http://localhost:3000', timestamp: 1000 }
    ]

    const script = generatePlaywrightScript(actions, {})

    assert.ok(script.includes("getByTestId('only-btn')"))
    assert.ok(script.includes('.click()'))
  })

  test('should handle actions with no selectors', async () => {
    const { generatePlaywrightScript } = await import('../../../extension/lib/page/reproduction.js')

    const actions = [{ type: 'click', selectors: {}, url: 'http://localhost:3000', timestamp: 1000 }]

    const script = generatePlaywrightScript(actions, {})

    // Should not crash, may add a comment about missing selector
    assert.ok(typeof script === 'string')
    assert.ok(script.length > 0)
  })

  test('should respect last_n_actions parameter', async () => {
    const { generatePlaywrightScript } = await import('../../../extension/lib/page/reproduction.js')

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
    const { generatePlaywrightScript } = await import('../../../extension/lib/page/reproduction.js')

    const actions = [{ type: 'click', selectors: { testId: 'btn' }, url: 'http://localhost:3000', timestamp: 1000 }]

    const script = generatePlaywrightScript(actions, { errorMessage: 'TypeError: foo is not a function' })

    assert.ok(script.includes('foo is not a function') || script.includes('TypeError'))
  })

  test('should cap output at 50KB', async () => {
    const { generatePlaywrightScript } = await import('../../../extension/lib/page/reproduction.js')

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
    const { generatePlaywrightScript } = await import('../../../extension/lib/page/reproduction.js')

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
    const { generatePlaywrightScript } = await import('../../../extension/lib/page/reproduction.js')

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
    const { generatePlaywrightScript } = await import('../../../extension/lib/page/reproduction.js')

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
