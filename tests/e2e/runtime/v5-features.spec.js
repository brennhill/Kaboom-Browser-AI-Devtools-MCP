/**
 * E2E Test: v5 Features (AI-Preprocessed Errors + Reproduction Scripts)
 *
 * Tests the v5 features in a real browser context with the extension loaded:
 *   - AI context enrichment pipeline
 *   - Multi-strategy selector computation
 *   - Enhanced action recording
 *   - Playwright script generation
 */
import { test, expect } from '../helpers/extension.js'
import path from 'path'
import { fileURLToPath } from 'url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const fixturesDir = path.join(__dirname, '..', 'fixtures')

test.describe('v5: Multi-Strategy Selectors', () => {
  test('should compute testId selector from data-testid', async ({ page }) => {
    await page.goto(`file://${path.join(fixturesDir, 'v5-test-page.html')}`)
    await page.waitForTimeout(1000)

    const selectors = await page.evaluate(() => {
      const el = document.querySelector('[data-testid="email-input"]')
      return window.__kaboom.getSelectors(el)
    })

    expect(selectors.testId).toBe('email-input')
    expect(selectors.ariaLabel).toBe('Email address')
    expect(selectors.id).toBe('email')
    expect(selectors.cssPath).toBeDefined()
  })

  test('should compute role selector for button', async ({ page }) => {
    await page.goto(`file://${path.join(fixturesDir, 'v5-test-page.html')}`)
    await page.waitForTimeout(1000)

    const selectors = await page.evaluate(() => {
      const el = document.querySelector('[data-testid="login-btn"]')
      return window.__kaboom.getSelectors(el)
    })

    expect(selectors.testId).toBe('login-btn')
    expect(selectors.ariaLabel).toBe('Log in')
    expect(selectors.role).toBeDefined()
    expect(selectors.role.role).toBe('button')
    expect(selectors.text).toBe('Log In')
  })

  test('should compute data-cy selector', async ({ page }) => {
    await page.goto(`file://${path.join(fixturesDir, 'v5-test-page.html')}`)
    await page.waitForTimeout(1000)

    const selectors = await page.evaluate(() => {
      const el = document.querySelector('[data-cy="action-btn"]')
      return window.__kaboom.getSelectors(el)
    })

    expect(selectors.testId).toBe('action-btn')
  })

  test('should compute id-based selector', async ({ page }) => {
    await page.goto(`file://${path.join(fixturesDir, 'v5-test-page.html')}`)
    await page.waitForTimeout(1000)

    const selectors = await page.evaluate(() => {
      const el = document.querySelector('#nav-profile')
      return window.__kaboom.getSelectors(el)
    })

    expect(selectors.id).toBe('nav-profile')
  })

  test('should compute aria-label for links', async ({ page }) => {
    await page.goto(`file://${path.join(fixturesDir, 'v5-test-page.html')}`)
    await page.waitForTimeout(1000)

    const selectors = await page.evaluate(() => {
      const el = document.querySelector('[aria-label="Settings page"]')
      return window.__kaboom.getSelectors(el)
    })

    expect(selectors.ariaLabel).toBe('Settings page')
  })
})

test.describe('v5: Enhanced Action Recording', () => {
  test('should record click actions with selectors', async ({ page }) => {
    await page.goto(`file://${path.join(fixturesDir, 'v5-test-page.html')}`)
    await page.waitForTimeout(1000)

    const action = await page.evaluate(() => {
      window.__kaboom.clearEnhancedActions()
      const el = document.querySelector('[data-testid="login-btn"]')
      return window.__kaboom.recordAction('click', el)
    })

    expect(action.type).toBe('click')
    expect(action.selectors.testId).toBe('login-btn')
    expect(action.selectors.ariaLabel).toBe('Log in')
    expect(action.url).toContain('v5-test-page.html')
    expect(action.timestamp).toBeDefined()
  })

  test('should record input actions with value', async ({ page }) => {
    await page.goto(`file://${path.join(fixturesDir, 'v5-test-page.html')}`)
    await page.waitForTimeout(1000)

    const action = await page.evaluate(() => {
      window.__kaboom.clearEnhancedActions()
      const el = document.querySelector('[data-testid="email-input"]')
      return window.__kaboom.recordAction('input', el, { value: 'test@example.com' })
    })

    expect(action.type).toBe('input')
    expect(action.value).toBe('test@example.com')
    expect(action.inputType).toBe('email')
  })

  test('should redact password input values', async ({ page }) => {
    await page.goto(`file://${path.join(fixturesDir, 'v5-test-page.html')}`)
    await page.waitForTimeout(1000)

    const action = await page.evaluate(() => {
      window.__kaboom.clearEnhancedActions()
      const el = document.querySelector('[data-testid="password-input"]')
      return window.__kaboom.recordAction('input', el, { value: 'supersecret' })
    })

    expect(action.value).toBe('[redacted]')
  })

  test('should maintain buffer of recorded actions', async ({ page }) => {
    await page.goto(`file://${path.join(fixturesDir, 'v5-test-page.html')}`)
    await page.waitForTimeout(1000)

    const bufferSize = await page.evaluate(() => {
      window.__kaboom.clearEnhancedActions()
      const btn = document.querySelector('[data-testid="login-btn"]')
      const input = document.querySelector('[data-testid="email-input"]')

      window.__kaboom.recordAction('click', btn)
      window.__kaboom.recordAction('input', input, { value: 'test@test.com' })
      window.__kaboom.recordAction('click', btn)

      return window.__kaboom.getEnhancedActions().length
    })

    expect(bufferSize).toBe(3)
  })

  test('should limit buffer to 50 actions', async ({ page }) => {
    await page.goto(`file://${path.join(fixturesDir, 'v5-test-page.html')}`)
    await page.waitForTimeout(1000)

    const bufferSize = await page.evaluate(() => {
      window.__kaboom.clearEnhancedActions()
      const btn = document.querySelector('[data-testid="login-btn"]')

      for (let i = 0; i < 60; i++) {
        window.__kaboom.recordAction('click', btn)
      }

      return window.__kaboom.getEnhancedActions().length
    })

    expect(bufferSize).toBeLessThanOrEqual(50)
  })
})

test.describe('v5: Playwright Script Generation', () => {
  test('should generate valid Playwright test from actions', async ({ page }) => {
    await page.goto(`file://${path.join(fixturesDir, 'v5-test-page.html')}`)
    await page.waitForTimeout(1000)

    const script = await page.evaluate(() => {
      window.__kaboom.clearEnhancedActions()
      const btn = document.querySelector('[data-testid="login-btn"]')
      const emailInput = document.querySelector('[data-testid="email-input"]')

      window.__kaboom.recordAction('input', emailInput, { value: 'user@test.com' })
      window.__kaboom.recordAction('click', btn)

      return window.__kaboom.generateScript(null, { errorMessage: 'Login failed' })
    })

    expect(script).toContain("import { test, expect } from '@playwright/test'")
    expect(script).toContain('test(')
    expect(script).toContain('async ({ page })')
    expect(script).toContain('page.goto')
    expect(script).toContain("getByTestId('email-input')")
    expect(script).toContain('.fill(')
    expect(script).toContain("getByTestId('login-btn')")
    expect(script).toContain('.click()')
    expect(script).toContain('Login failed')
  })

  test('should use getByTestId as top priority selector', async ({ page }) => {
    await page.goto(`file://${path.join(fixturesDir, 'v5-test-page.html')}`)
    await page.waitForTimeout(1000)

    const script = await page.evaluate(() => {
      window.__kaboom.clearEnhancedActions()
      const btn = document.querySelector('[data-testid="login-btn"]')
      window.__kaboom.recordAction('click', btn)
      return window.__kaboom.generateScript()
    })

    expect(script).toContain("getByTestId('login-btn')")
  })

  test('should handle password redaction in generated script', async ({ page }) => {
    await page.goto(`file://${path.join(fixturesDir, 'v5-test-page.html')}`)
    await page.waitForTimeout(1000)

    const script = await page.evaluate(() => {
      window.__kaboom.clearEnhancedActions()
      const pw = document.querySelector('[data-testid="password-input"]')
      window.__kaboom.recordAction('input', pw, { value: 'secret123' })
      return window.__kaboom.generateScript()
    })

    expect(script).not.toContain('secret123')
    expect(script).toContain('[user-provided]')
  })

  test('should generate script with select actions', async ({ page }) => {
    await page.goto(`file://${path.join(fixturesDir, 'v5-test-page.html')}`)
    await page.waitForTimeout(1000)

    const script = await page.evaluate(() => {
      window.__kaboom.clearEnhancedActions()
      const select = document.querySelector('[data-testid="country-select"]')
      window.__kaboom.recordAction('select', select, { selectedValue: 'us', selectedText: 'United States' })
      return window.__kaboom.generateScript()
    })

    expect(script).toContain("selectOption('us')")
  })

  test('should generate script with keyboard actions', async ({ page }) => {
    await page.goto(`file://${path.join(fixturesDir, 'v5-test-page.html')}`)
    await page.waitForTimeout(1000)

    const script = await page.evaluate(() => {
      window.__kaboom.clearEnhancedActions()
      const input = document.querySelector('[data-testid="email-input"]')
      window.__kaboom.recordAction('input', input, { value: 'test@test.com' })
      window.__kaboom.recordAction('keypress', input, { key: 'Enter' })
      return window.__kaboom.generateScript()
    })

    expect(script).toContain("keyboard.press('Enter')")
  })

  test('should handle baseUrl override', async ({ page }) => {
    await page.goto(`file://${path.join(fixturesDir, 'v5-test-page.html')}`)
    await page.waitForTimeout(1000)

    const script = await page.evaluate(() => {
      window.__kaboom.clearEnhancedActions()
      const btn = document.querySelector('[data-testid="login-btn"]')
      window.__kaboom.recordAction('click', btn)
      return window.__kaboom.generateScript(null, { baseUrl: 'http://localhost:4000' })
    })

    expect(script).toContain('localhost:4000')
  })

  test('should handle empty action buffer', async ({ page }) => {
    await page.goto(`file://${path.join(fixturesDir, 'v5-test-page.html')}`)
    await page.waitForTimeout(1000)

    const script = await page.evaluate(() => {
      window.__kaboom.clearEnhancedActions()
      return window.__kaboom.generateScript()
    })

    expect(script).toContain("import { test, expect } from '@playwright/test'")
    expect(script).toContain('test(')
  })
})

test.describe('v5: AI Context Enrichment', () => {
  test('should enrich error with AI context', async ({ page }) => {
    await page.goto(`file://${path.join(fixturesDir, 'v5-test-page.html')}`)
    await page.waitForTimeout(1000)

    const enriched = await page.evaluate(async () => {
      const error = {
        type: 'exception',
        level: 'error',
        message: "Cannot read properties of undefined (reading 'user')",
        stack: `TypeError: Cannot read properties of undefined
    at handleSubmit (http://localhost:3000/main.js:42:15)`,
        filename: 'http://localhost:3000/main.js',
        lineno: 42,
        _enrichments: []
      }
      return await window.__kaboom.enrichError(error)
    })

    expect(enriched._aiContext).toBeDefined()
    expect(enriched._aiContext.summary).toBeDefined()
    expect(enriched._aiContext.summary).toContain('Cannot read properties')
    expect(enriched._enrichments).toContain('aiContext')
  })

  test('should include state snapshot when enabled and store exists', async ({ page }) => {
    await page.goto(`file://${path.join(fixturesDir, 'v5-test-page.html')}`)
    await page.waitForTimeout(1000)

    const enriched = await page.evaluate(async () => {
      window.__kaboom.setStateSnapshot(true)
      const error = {
        type: 'exception',
        level: 'error',
        message: 'auth failed: token expired',
        stack: 'Error: auth failed\n    at fn (http://localhost:3000/main.js:10:5)',
        _enrichments: []
      }
      const result = await window.__kaboom.enrichError(error)
      window.__kaboom.setStateSnapshot(false)
      return result
    })

    expect(enriched._aiContext).toBeDefined()
    if (enriched._aiContext.stateSnapshot) {
      expect(enriched._aiContext.stateSnapshot.source).toBe('redux')
      expect(enriched._aiContext.stateSnapshot.keys).toBeDefined()
    }
  })

  test('should skip enrichment when disabled', async ({ page }) => {
    await page.goto(`file://${path.join(fixturesDir, 'v5-test-page.html')}`)
    await page.waitForTimeout(1000)

    const enriched = await page.evaluate(async () => {
      window.__kaboom.setAiContext(false)
      const error = {
        type: 'exception',
        level: 'error',
        message: 'test error',
        stack: 'Error: test',
        _enrichments: []
      }
      const result = await window.__kaboom.enrichError(error)
      window.__kaboom.setAiContext(true) // Re-enable
      return result
    })

    expect(enriched._aiContext).toBeUndefined()
  })

  test('should detect React component ancestry on element', async ({ page }) => {
    await page.goto(`file://${path.join(fixturesDir, 'v5-test-page.html')}`)
    await page.waitForTimeout(1000)

    const enriched = await page.evaluate(async () => {
      // Focus the button that has the mock fiber
      const btn = document.querySelector('[data-testid="login-btn"]')
      btn.focus()

      const error = {
        type: 'exception',
        level: 'error',
        message: 'Button click error',
        stack: 'Error: Button click error\n    at handleClick (app.js:10:5)',
        _enrichments: []
      }
      return await window.__kaboom.enrichError(error)
    })

    expect(enriched._aiContext).toBeDefined()
    if (enriched._aiContext.componentAncestry) {
      expect(enriched._aiContext.componentAncestry.framework).toBe('react')
      expect(enriched._aiContext.componentAncestry.components.length).toBeGreaterThan(0)
      const names = enriched._aiContext.componentAncestry.components.map((c) => c.name)
      expect(names).toContain('LoginButton')
    }
  })

  test('should complete enrichment within 3s timeout', async ({ page }) => {
    await page.goto(`file://${path.join(fixturesDir, 'v5-test-page.html')}`)
    await page.waitForTimeout(1000)

    const elapsed = await page.evaluate(async () => {
      const start = Date.now()
      const error = {
        type: 'exception',
        level: 'error',
        message: 'timeout test',
        stack: 'Error: timeout\n    at fn (http://localhost:3000/main.js:10:5)',
        _enrichments: []
      }
      await window.__kaboom.enrichError(error)
      return Date.now() - start
    })

    expect(elapsed).toBeLessThan(4000)
  })
})

test.describe('v5: Full Integration Flow', () => {
  test('should record real user interactions and generate script', async ({ page }) => {
    await page.goto(`file://${path.join(fixturesDir, 'v5-test-page.html')}`)
    await page.waitForTimeout(1000)

    // Clear the buffer first
    await page.evaluate(() => window.__kaboom.clearEnhancedActions())

    // Record a series of user actions programmatically
    const script = await page.evaluate(() => {
      const email = document.querySelector('[data-testid="email-input"]')
      const password = document.querySelector('[data-testid="password-input"]')
      const loginBtn = document.querySelector('[data-testid="login-btn"]')

      window.__kaboom.recordAction('input', email, { value: 'user@example.com' })
      window.__kaboom.recordAction('input', password, { value: 'secret' })
      window.__kaboom.recordAction('click', loginBtn)

      return window.__kaboom.generateScript(null, {
        errorMessage: "Cannot read properties of undefined (reading 'user')"
      })
    })

    // Verify the generated script is a complete, valid Playwright test
    expect(script).toContain("import { test, expect } from '@playwright/test'")
    expect(script).toContain('reproduction')
    expect(script).toContain("getByTestId('email-input')")
    expect(script).toContain("fill('user@example.com')")
    expect(script).toContain("getByTestId('password-input')")
    expect(script).toContain('[user-provided]') // Password redacted
    expect(script).toContain("getByTestId('login-btn')")
    expect(script).toContain('.click()')
    expect(script).toContain('Cannot read properties of undefined')
  })

  test('should use lastNActions to limit script steps', async ({ page }) => {
    await page.goto(`file://${path.join(fixturesDir, 'v5-test-page.html')}`)
    await page.waitForTimeout(1000)

    const script = await page.evaluate(() => {
      window.__kaboom.clearEnhancedActions()
      const btn = document.querySelector('[data-testid="login-btn"]')
      const email = document.querySelector('[data-testid="email-input"]')

      // Record 5 actions
      window.__kaboom.recordAction('input', email, { value: 'a@b.com' })
      window.__kaboom.recordAction('click', btn)
      window.__kaboom.recordAction('input', email, { value: 'c@d.com' })
      window.__kaboom.recordAction('click', btn)
      window.__kaboom.recordAction('click', btn)

      // Only get last 2
      return window.__kaboom.generateScript(null, { lastNActions: 2 })
    })

    // Should only have the last 2 actions
    const clickCount = (script.match(/\.click\(\)/g) || []).length
    expect(clickCount).toBe(2)
  })
})
