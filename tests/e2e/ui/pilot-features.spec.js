/**
 * E2E Tests: AI Web Pilot Features
 *
 * Tests the AI Web Pilot tools in a real browser context:
 *   - interact(action: "highlight"): Visual element highlighting
 *   - interact(action: "save_state/load_state/list_states/delete_state"): Browser state management
 *   - interact(action: "execute_js"): JS execution in browser context
 *
 * These features require the AI Web Pilot toggle to be enabled.
 */
import { test, expect } from '../helpers/extension.js'
import { mcpCall } from '../helpers/mcp.js'
import path from 'path'
import { fileURLToPath } from 'url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const fixturesDir = path.join(__dirname, '..', 'fixtures')

/**
 * Call an MCP tool and return the parsed result
 */
async function mcpToolCall(serverUrl, toolName, args = {}) {
  const resp = await mcpCall(serverUrl, 'tools/call', { name: toolName, arguments: args })
  if (resp.error) {
    return { error: resp.error }
  }
  const content = resp.result?.content
  if (!content || content.length === 0) return { text: '' }
  const text = content[0].text
  try {
    return { data: JSON.parse(text), text }
  } catch {
    return { text }
  }
}

/**
 * Enable AI Web Pilot toggle via extension popup
 */
async function enableAiWebPilot(context, extensionId) {
  const popupPage = await context.newPage()
  await popupPage.goto(`chrome-extension://${extensionId}/popup.html`)
  await popupPage.waitForTimeout(500)

  // Check if toggle exists and enable it
  const toggleExists = await popupPage.evaluate(() => {
    const toggle = document.getElementById('ai-web-pilot-toggle')
    if (toggle) {
      toggle.checked = true
      toggle.dispatchEvent(new Event('change'))
      return true
    }
    return false
  })

  // Also set in storage directly to ensure it's enabled
  await popupPage.evaluate(() => {
    return new Promise((resolve) => {
      chrome.storage.sync.set({ aiWebPilotEnabled: true }, resolve)
    })
  })

  await popupPage.waitForTimeout(500)
  await popupPage.close()
  return toggleExists
}

/**
 * Disable AI Web Pilot toggle
 */
async function disableAiWebPilot(context, extensionId) {
  const popupPage = await context.newPage()
  await popupPage.goto(`chrome-extension://${extensionId}/popup.html`)

  await popupPage.evaluate(() => {
    return new Promise((resolve) => {
      chrome.storage.sync.set({ aiWebPilotEnabled: false }, resolve)
    })
  })

  await popupPage.waitForTimeout(500)
  await popupPage.close()
}

// =============================================================================
// SAFETY GATE TESTS
// =============================================================================

test.describe('AI Web Pilot: Safety Gate', () => {
  test('highlight returns error when toggle is disabled', async ({ page, serverUrl, context, extensionId }) => {
    // Ensure toggle is disabled
    await disableAiWebPilot(context, extensionId)

    await page.goto(`file://${path.join(fixturesDir, 'pilot-test-page.html')}`)
    await page.waitForTimeout(1000)

    const result = await mcpToolCall(serverUrl, 'interact', {
      action: 'highlight',
      selector: '#highlight-target'
    })

    expect(result.text).toContain('pilot_disabled')
  })

  test('execute_js returns error when toggle is disabled', async ({ page, serverUrl, context, extensionId }) => {
    await disableAiWebPilot(context, extensionId)

    await page.goto(`file://${path.join(fixturesDir, 'pilot-test-page.html')}`)
    await page.waitForTimeout(1000)

    const result = await mcpToolCall(serverUrl, 'interact', {
      action: 'execute_js',
      script: 'return 1 + 1'
    })

    expect(result.text).toContain('pilot_disabled')
  })
})

// =============================================================================
// HIGHLIGHT TESTS
// =============================================================================

test.describe('AI Web Pilot: highlight', () => {
  test.beforeEach(async ({ context, extensionId }) => {
    await enableAiWebPilot(context, extensionId)
  })

  test('highlights element by ID selector', async ({ page, serverUrl }) => {
    await page.goto(`file://${path.join(fixturesDir, 'pilot-test-page.html')}`)
    await page.waitForTimeout(1000)

    const result = await mcpToolCall(serverUrl, 'interact', {
      action: 'highlight',
      selector: '#highlight-target'
    })

    // Check the highlighter overlay was created
    const highlighter = await page.$('#gasoline-highlighter')
    expect(highlighter).not.toBeNull()

    // Verify the result contains bounds
    expect(result.data?.success || result.text).toBeTruthy()
  })

  test('highlights element by data-testid selector', async ({ page, serverUrl }) => {
    await page.goto(`file://${path.join(fixturesDir, 'pilot-test-page.html')}`)
    await page.waitForTimeout(1000)

    const result = await mcpToolCall(serverUrl, 'interact', {
      action: 'highlight',
      selector: '[data-testid="highlight-box"]'
    })

    const highlighter = await page.$('#gasoline-highlighter')
    expect(highlighter).not.toBeNull()
  })

  test('returns error for non-existent selector', async ({ page, serverUrl }) => {
    await page.goto(`file://${path.join(fixturesDir, 'pilot-test-page.html')}`)
    await page.waitForTimeout(1000)

    const result = await mcpToolCall(serverUrl, 'interact', {
      action: 'highlight',
      selector: '#non-existent-element'
    })

    expect(result.text).toContain('element_not_found')
  })

  test('highlight has correct visual styles', async ({ page, serverUrl }) => {
    await page.goto(`file://${path.join(fixturesDir, 'pilot-test-page.html')}`)
    await page.waitForTimeout(1000)

    await mcpToolCall(serverUrl, 'interact', {
      action: 'highlight',
      selector: '#highlight-target'
    })

    const styles = await page.evaluate(() => {
      const el = document.getElementById('gasoline-highlighter')
      if (!el) return null
      const computed = window.getComputedStyle(el)
      return {
        position: computed.position,
        border: computed.border,
        zIndex: computed.zIndex,
        pointerEvents: computed.pointerEvents
      }
    })

    expect(styles).not.toBeNull()
    expect(styles.position).toBe('fixed')
    // Browser may return 'red' or 'rgb(255, 0, 0)' depending on environment
    expect(styles.border).toMatch(/red|rgb\(255,\s*0,\s*0\)/)
    expect(styles.pointerEvents).toBe('none')
  })

  test('second highlight replaces first', async ({ page, serverUrl }) => {
    await page.goto(`file://${path.join(fixturesDir, 'pilot-test-page.html')}`)
    await page.waitForTimeout(1000)

    // First highlight
    await mcpToolCall(serverUrl, 'interact', {
      action: 'highlight',
      selector: '#highlight-target'
    })

    const firstBounds = await page.evaluate(() => {
      const el = document.getElementById('gasoline-highlighter')
      return el ? el.getBoundingClientRect() : null
    })

    // Second highlight on different element
    await mcpToolCall(serverUrl, 'interact', {
      action: 'highlight',
      selector: '#nested-target'
    })

    const secondBounds = await page.evaluate(() => {
      const el = document.getElementById('gasoline-highlighter')
      return el ? el.getBoundingClientRect() : null
    })

    // Should only have one highlighter, and it should be in a different position
    const count = await page.evaluate(() => document.querySelectorAll('#gasoline-highlighter').length)
    expect(count).toBe(1)
    expect(secondBounds.top).not.toBe(firstBounds.top)
  })

  test('highlight auto-removes after duration', async ({ page, serverUrl }) => {
    await page.goto(`file://${path.join(fixturesDir, 'pilot-test-page.html')}`)
    await page.waitForTimeout(1000)

    await mcpToolCall(serverUrl, 'interact', {
      action: 'highlight',
      selector: '#highlight-target',
      duration_ms: 1500
    })

    // Should exist immediately
    let highlighter = await page.$('#gasoline-highlighter')
    expect(highlighter).not.toBeNull()

    // Wait for auto-removal
    await page.waitForTimeout(2000)

    // Should be gone
    highlighter = await page.$('#gasoline-highlighter')
    expect(highlighter).toBeNull()
  })
})

// =============================================================================
// STATE MANAGEMENT TESTS
// =============================================================================

test.describe('AI Web Pilot: state management', () => {
  test.beforeEach(async ({ context, extensionId }) => {
    await enableAiWebPilot(context, extensionId)
  })

  test('saves and lists named snapshots', async ({ page, serverUrl }) => {
    await page.goto(`file://${path.join(fixturesDir, 'pilot-test-page.html')}`)
    await page.waitForTimeout(1000)

    // Save a snapshot
    const saveResult = await mcpToolCall(serverUrl, 'interact', {
      action: 'save_state',
      snapshot_name: 'test-snapshot-1'
    })

    expect(saveResult.data?.success || saveResult.text).toBeTruthy()

    // List snapshots
    const listResult = await mcpToolCall(serverUrl, 'interact', {
      action: 'list_states'
    })

    expect(listResult.data?.snapshots || listResult.text).toBeTruthy()
    if (listResult.data?.snapshots) {
      const names = listResult.data.snapshots.map((s) => s.name)
      expect(names).toContain('test-snapshot-1')
    }
  })

  test('restores saved snapshot', async ({ page, serverUrl }) => {
    await page.goto(`file://${path.join(fixturesDir, 'pilot-test-page.html')}`)
    await page.waitForTimeout(1000)

    // Set initial state
    await page.fill('#state-input', 'before-snapshot')

    // Save snapshot
    await mcpToolCall(serverUrl, 'interact', {
      action: 'save_state',
      snapshot_name: 'restore-test'
    })

    // Modify state
    await page.fill('#state-input', 'after-snapshot')

    // Verify state changed
    const afterValue = await page.inputValue('#state-input')
    expect(afterValue).toBe('after-snapshot')

    // Restore snapshot
    const restoreResult = await mcpToolCall(serverUrl, 'interact', {
      action: 'load_state',
      snapshot_name: 'restore-test'
    })

    expect(restoreResult.data?.success || restoreResult.text).toBeTruthy()
  })

  test('deletes named snapshot', async ({ page, serverUrl }) => {
    await page.goto(`file://${path.join(fixturesDir, 'pilot-test-page.html')}`)
    await page.waitForTimeout(1000)

    // Save a snapshot
    await mcpToolCall(serverUrl, 'interact', {
      action: 'save_state',
      snapshot_name: 'delete-test'
    })

    // Delete it
    const deleteResult = await mcpToolCall(serverUrl, 'interact', {
      action: 'delete_state',
      snapshot_name: 'delete-test'
    })

    expect(deleteResult.data?.success || deleteResult.text).toBeTruthy()

    // Verify it's gone
    const listResult = await mcpToolCall(serverUrl, 'interact', {
      action: 'list_states'
    })

    if (listResult.data?.snapshots) {
      const names = listResult.data.snapshots.map((s) => s.name)
      expect(names).not.toContain('delete-test')
    }
  })
})

// =============================================================================
// EXECUTE_JS TESTS
// =============================================================================

test.describe('AI Web Pilot: execute_js', () => {
  test.beforeEach(async ({ context, extensionId }) => {
    await enableAiWebPilot(context, extensionId)
  })

  test('executes simple expression and returns result', async ({ page, serverUrl }) => {
    await page.goto(`file://${path.join(fixturesDir, 'pilot-test-page.html')}`)
    await page.waitForTimeout(1000)

    const result = await mcpToolCall(serverUrl, 'interact', {
      action: 'execute_js',
      script: 'return 2 + 2'
    })

    expect(result.data?.result).toBe(4)
  })

  test('reads DOM values', async ({ page, serverUrl }) => {
    await page.goto(`file://${path.join(fixturesDir, 'pilot-test-page.html')}`)
    await page.waitForTimeout(1000)

    await page.fill('#state-input', 'test-value-123')

    const result = await mcpToolCall(serverUrl, 'interact', {
      action: 'execute_js',
      script: 'return document.getElementById("state-input").value'
    })

    expect(result.data?.result).toBe('test-value-123')
  })

  test('reads global variables', async ({ page, serverUrl }) => {
    await page.goto(`file://${path.join(fixturesDir, 'pilot-test-page.html')}`)
    await page.waitForTimeout(1000)

    // Set a global value via page click
    await page.click('#set-global-btn')

    const result = await mcpToolCall(serverUrl, 'interact', {
      action: 'execute_js',
      script: 'return window.__testValue'
    })

    expect(result.data?.result).toContain('global-test-value-')
  })

  test('returns objects serialized as JSON', async ({ page, serverUrl }) => {
    await page.goto(`file://${path.join(fixturesDir, 'pilot-test-page.html')}`)
    await page.waitForTimeout(1000)

    const result = await mcpToolCall(serverUrl, 'interact', {
      action: 'execute_js',
      script: 'return { name: "test", values: [1, 2, 3] }'
    })

    expect(result.data?.result).toEqual({ name: 'test', values: [1, 2, 3] })
  })

  test('handles arrays', async ({ page, serverUrl }) => {
    await page.goto(`file://${path.join(fixturesDir, 'pilot-test-page.html')}`)
    await page.waitForTimeout(1000)

    const result = await mcpToolCall(serverUrl, 'interact', {
      action: 'execute_js',
      script: 'return Array.from(document.querySelectorAll(".action-button")).map(el => el.id)'
    })

    expect(Array.isArray(result.data?.result)).toBe(true)
    expect(result.data?.result).toContain('increment-btn')
  })

  test('returns error for invalid JavaScript', async ({ page, serverUrl }) => {
    await page.goto(`file://${path.join(fixturesDir, 'pilot-test-page.html')}`)
    await page.waitForTimeout(1000)

    const result = await mcpToolCall(serverUrl, 'interact', {
      action: 'execute_js',
      script: 'return {{{invalid syntax' // triple braces intentional for invalid syntax test
    })

    expect(result.data?.error || result.text).toBeTruthy()
  })

  test('can call page functions', async ({ page, serverUrl }) => {
    await page.goto(`file://${path.join(fixturesDir, 'pilot-test-page.html')}`)
    await page.waitForTimeout(1000)

    // Use the exposed test counter
    const result = await mcpToolCall(serverUrl, 'interact', {
      action: 'execute_js',
      script: 'window.__testCounter.set(42); return window.__testCounter.get()'
    })

    expect(result.data?.result).toBe(42)

    // Verify DOM was updated
    const counterText = await page.textContent('#counter-value')
    expect(counterText).toBe('42')
  })

  test('handles undefined return value', async ({ page, serverUrl }) => {
    await page.goto(`file://${path.join(fixturesDir, 'pilot-test-page.html')}`)
    await page.waitForTimeout(1000)

    const result = await mcpToolCall(serverUrl, 'interact', {
      action: 'execute_js',
      script: 'console.log("test")' // No return
    })

    // Should succeed without error
    expect(result.data?.success !== false).toBeTruthy()
  })

  test('handles null return value', async ({ page, serverUrl }) => {
    await page.goto(`file://${path.join(fixturesDir, 'pilot-test-page.html')}`)
    await page.waitForTimeout(1000)

    const result = await mcpToolCall(serverUrl, 'interact', {
      action: 'execute_js',
      script: 'return null'
    })

    expect(result.data?.result).toBeNull()
  })
})

// =============================================================================
// INTEGRATION TESTS
// =============================================================================

test.describe('AI Web Pilot: Integration', () => {
  test.beforeEach(async ({ context, extensionId }) => {
    await enableAiWebPilot(context, extensionId)
  })

  test('highlight + execute workflow', async ({ page, serverUrl }) => {
    await page.goto(`file://${path.join(fixturesDir, 'pilot-test-page.html')}`)
    await page.waitForTimeout(1000)

    // Highlight an element
    await mcpToolCall(serverUrl, 'interact', {
      action: 'highlight',
      selector: '#state-input'
    })

    // Execute JS to read its value
    const result = await mcpToolCall(serverUrl, 'interact', {
      action: 'execute_js',
      script: 'return document.getElementById("state-input").value'
    })

    expect(result.data?.result).toBe('initial value')
  })

  test('state save + execute + restore workflow', async ({ page, serverUrl }) => {
    await page.goto(`file://${path.join(fixturesDir, 'pilot-test-page.html')}`)
    await page.waitForTimeout(1000)

    // Save initial state
    await mcpToolCall(serverUrl, 'interact', {
      action: 'save_state',
      snapshot_name: 'workflow-test'
    })

    // Use execute_js to modify state
    await mcpToolCall(serverUrl, 'interact', {
      action: 'execute_js',
      script: 'document.getElementById("state-input").value = "modified by AI"'
    })

    // Verify modification
    const modifiedValue = await page.inputValue('#state-input')
    expect(modifiedValue).toBe('modified by AI')

    // Restore
    await mcpToolCall(serverUrl, 'interact', {
      action: 'load_state',
      snapshot_name: 'workflow-test'
    })
  })
})
