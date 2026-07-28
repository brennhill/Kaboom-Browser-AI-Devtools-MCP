// @ts-nocheck
/**
 * @fileoverview On-demand accessibility, page-load deferral, and memory-pressure tests.
 */

import { test, describe, mock, beforeEach, afterEach } from 'node:test'
import assert from 'node:assert'
import { createMockDocument, createMockWindow } from './on-demand-fixture.js'

let originalDocument
let originalWindow


describe('Accessibility Audit Execution', () => {
  beforeEach(() => {
    originalDocument = globalThis.document
    originalWindow = globalThis.window
    globalThis.document = createMockDocument()
    globalThis.window = createMockWindow()
  })

  afterEach(() => {
    globalThis.document = originalDocument
    globalThis.window = originalWindow
  })

  test('should wait for axe-core to appear on window', async () => {
    const { runAxeAudit } = await import('../../extension/lib/analysis/dom-queries.js')

    globalThis.window.axe = null

    // Simulate content script injecting axe-core after 50ms
    // (loadAxeCore polls window.axe every 100ms with a 5s timeout)
    setTimeout(() => {
      globalThis.window.axe = {
        run: mock.fn(() =>
          Promise.resolve({
            violations: [],
            passes: [],
            incomplete: [],
            inapplicable: []
          })
        )
      }
    }, 50)

    const _result = await runAxeAudit({})

    assert.ok(globalThis.window.axe, 'axe-core should be loaded on window')
    assert.ok(globalThis.window.axe.run.mock.calls.length > 0, 'axe.run should have been called')
  })

  test('should reuse axe-core if already loaded', async () => {
    const { runAxeAudit } = await import('../../extension/lib/analysis/dom-queries.js')

    globalThis.window.axe = {
      run: mock.fn(() =>
        Promise.resolve({
          violations: [],
          passes: [],
          incomplete: [],
          inapplicable: []
        })
      )
    }

    await runAxeAudit({})

    // Should NOT have created a new script element
    assert.strictEqual(globalThis.document.createElement.mock.calls.length, 0)
    assert.strictEqual(globalThis.window.axe.run.mock.calls.length, 1)
  })

  test('should pass scope as context to axe.run', async () => {
    const { runAxeAudit } = await import('../../extension/lib/analysis/dom-queries.js')

    globalThis.window.axe = {
      run: mock.fn(() =>
        Promise.resolve({
          violations: [],
          passes: [],
          incomplete: [],
          inapplicable: []
        })
      )
    }

    await runAxeAudit({ scope: '#main-content' })

    const [context] = globalThis.window.axe.run.mock.calls[0].arguments
    assert.deepStrictEqual(context, { include: ['#main-content'] })
  })

  test('should pass tags as runOnly config', async () => {
    const { runAxeAudit } = await import('../../extension/lib/analysis/dom-queries.js')

    globalThis.window.axe = {
      run: mock.fn(() =>
        Promise.resolve({
          violations: [],
          passes: [],
          incomplete: [],
          inapplicable: []
        })
      )
    }

    await runAxeAudit({ tags: ['wcag2a', 'wcag2aa'] })

    const [, config] = globalThis.window.axe.run.mock.calls[0].arguments
    assert.deepStrictEqual(config.runOnly, ['wcag2a', 'wcag2aa'])
  })

  test('should include passes when include_passes is true', async () => {
    const { runAxeAudit } = await import('../../extension/lib/analysis/dom-queries.js')

    globalThis.window.axe = {
      run: mock.fn(() =>
        Promise.resolve({
          violations: [],
          passes: [{ id: 'button-name' }],
          incomplete: [],
          inapplicable: []
        })
      )
    }

    await runAxeAudit({ include_passes: true })

    const [, config] = globalThis.window.axe.run.mock.calls[0].arguments
    assert.ok(config.resultTypes.includes('passes'))
  })

  test('should format violations with selector, html, and fix suggestion', async () => {
    const { formatAxeResults } = await import('../../extension/lib/analysis/dom-queries.js')

    const axeResult = {
      violations: [
        {
          id: 'color-contrast',
          impact: 'serious',
          description: 'Elements must have sufficient color contrast',
          helpUrl: 'https://dequeuniversity.com/rules/axe/4.8/color-contrast',
          tags: ['wcag2aa', 'cat.color'],
          nodes: [
            {
              target: ['#signup-form > label:nth-child(2)'],
              html: '<label class="form-label subtle">Email address</label>',
              failureSummary: 'Element has insufficient color contrast of 2.8:1'
            }
          ]
        }
      ],
      passes: [],
      incomplete: [],
      inapplicable: []
    }

    const formatted = formatAxeResults(axeResult)

    assert.strictEqual(formatted.violations[0].id, 'color-contrast')
    assert.strictEqual(formatted.violations[0].impact, 'serious')
    assert.strictEqual(formatted.violations[0].nodes[0].selector, '#signup-form > label:nth-child(2)')
    assert.ok(formatted.violations[0].nodes[0].html.includes('form-label'))
  })

  test('should limit nodes per violation to 10', async () => {
    const { formatAxeResults } = await import('../../extension/lib/analysis/dom-queries.js')

    const nodes = Array.from({ length: 20 }, (_, i) => ({
      target: [`#node-${i}`],
      html: `<div id="node-${i}">Node ${i}</div>`,
      failureSummary: 'Failure'
    }))

    const axeResult = {
      violations: [
        {
          id: 'test-rule',
          impact: 'minor',
          description: 'Test',
          helpUrl: 'http://test.com',
          tags: [],
          nodes
        }
      ],
      passes: [],
      incomplete: [],
      inapplicable: []
    }

    const formatted = formatAxeResults(axeResult)

    assert.ok(formatted.violations[0].nodes.length <= 10)
    assert.strictEqual(formatted.violations[0].nodeCount, 20)
  })

  test('should truncate HTML snippets to 200 chars', async () => {
    const { formatAxeResults } = await import('../../extension/lib/analysis/dom-queries.js')

    const longHtml = '<div class="' + 'x'.repeat(300) + '">content</div>'
    const axeResult = {
      violations: [
        {
          id: 'test-rule',
          impact: 'minor',
          description: 'Test',
          helpUrl: 'http://test.com',
          tags: [],
          nodes: [{ target: ['div'], html: longHtml, failureSummary: 'Failure' }]
        }
      ],
      passes: [],
      incomplete: [],
      inapplicable: []
    }

    const formatted = formatAxeResults(axeResult)

    assert.ok(formatted.violations[0].nodes[0].html.length <= 200)
  })

  test('should include summary counts', async () => {
    const { formatAxeResults } = await import('../../extension/lib/analysis/dom-queries.js')

    const axeResult = {
      violations: [
        { id: 'v1', nodes: [] },
        { id: 'v2', nodes: [] }
      ],
      passes: Array(52).fill({ id: 'p', nodes: [] }),
      incomplete: [{ id: 'i1', nodes: [] }],
      inapplicable: Array(31).fill({ id: 'ia', nodes: [] })
    }

    const formatted = formatAxeResults(axeResult)

    assert.strictEqual(formatted.summary.violations, 2)
    assert.strictEqual(formatted.summary.passes, 52)
    assert.strictEqual(formatted.summary.incomplete, 1)
    assert.strictEqual(formatted.summary.inapplicable, 31)
  })

  test('should timeout after 30 seconds', async () => {
    const { runAxeAuditWithTimeout } = await import('../../extension/lib/analysis/dom-queries.js')

    globalThis.window.axe = {
      run: mock.fn(() => new Promise(() => {})) // Never resolves
    }

    const result = await runAxeAuditWithTimeout({}, 50) // 50ms timeout for testing

    assert.ok(result.error, 'Expected timeout error')
    assert.ok(result.error.includes('timeout'), 'Expected timeout message')
  })

  test('should extract WCAG tags', async () => {
    const { formatAxeResults } = await import('../../extension/lib/analysis/dom-queries.js')

    const axeResult = {
      violations: [
        {
          id: 'color-contrast',
          impact: 'serious',
          description: 'Test',
          helpUrl: 'http://test.com',
          tags: ['wcag2aa', 'cat.color', 'wcag143'],
          nodes: []
        }
      ],
      passes: [],
      incomplete: [],
      inapplicable: []
    }

    const formatted = formatAxeResults(axeResult)

    // Should extract WCAG-specific tags
    assert.ok(formatted.violations[0].wcag, 'Expected wcag field')
    assert.ok(formatted.violations[0].wcag.includes('wcag2aa'))
  })
})

describe('Page Load Deferral', () => {
  beforeEach(() => {
    originalDocument = globalThis.document
    originalWindow = globalThis.window
    globalThis.document = createMockDocument()
    globalThis.window = createMockWindow()
  })

  afterEach(() => {
    globalThis.document = originalDocument
    globalThis.window = originalWindow
  })

  test('should defer intercepts while page is loading', async () => {
    const { shouldDeferIntercepts } = await import('../../extension/inject/observers.js')

    globalThis.document.readyState = 'loading'
    assert.strictEqual(shouldDeferIntercepts(), true)
  })

  test('should not defer if page already loaded', async () => {
    const { shouldDeferIntercepts } = await import('../../extension/inject/observers.js')

    globalThis.document.readyState = 'complete'
    assert.strictEqual(shouldDeferIntercepts(), false)
  })
})

describe('Memory Pressure Detection', () => {
  beforeEach(() => {
    originalWindow = globalThis.window
    globalThis.window = createMockWindow()
  })

  afterEach(() => {
    globalThis.window = originalWindow
  })

  test('should reduce buffers at soft limit (20MB)', async () => {
    const { checkMemoryPressure } = await import('../../extension/inject/observers.js')

    const state = {
      wsBufferCapacity: 500,
      networkBufferCapacity: 100,
      memoryUsageMB: 25 // Above 20MB soft limit
    }

    const result = checkMemoryPressure(state)

    assert.ok(result.wsBufferCapacity < 500, 'Expected WS buffer reduced')
    assert.ok(result.networkBufferCapacity < 100, 'Expected network buffer reduced')
  })

  test('should disable network bodies at hard limit (50MB)', async () => {
    const { checkMemoryPressure } = await import('../../extension/inject/observers.js')

    const state = {
      wsBufferCapacity: 500,
      networkBufferCapacity: 100,
      networkBodiesEnabled: true,
      memoryUsageMB: 55 // Above 50MB hard limit
    }

    const result = checkMemoryPressure(state)

    assert.strictEqual(result.networkBodiesEnabled, false)
  })

  test('should not modify state when under soft limit', async () => {
    const { checkMemoryPressure } = await import('../../extension/inject/observers.js')

    const state = {
      wsBufferCapacity: 500,
      networkBufferCapacity: 100,
      networkBodiesEnabled: true,
      memoryUsageMB: 15 // Under 20MB soft limit
    }

    const result = checkMemoryPressure(state)

    assert.strictEqual(result.wsBufferCapacity, 500)
    assert.strictEqual(result.networkBufferCapacity, 100)
    assert.strictEqual(result.networkBodiesEnabled, true)
  })
})
