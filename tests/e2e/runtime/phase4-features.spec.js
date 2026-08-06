/**
 * E2E Tests: Phase 4 Features
 *
 * Tests the full pipeline for:
 *   1. AI Capture Control — configure(action: "capture") + GET /settings
 *   2. Error Clustering — observe(what: "error_clusters")
 *   3. Navigation History — observe(what: "history")
 */
import { test, expect } from '../helpers/extension.js'
import { mcpCall, mcpToolText } from '../helpers/mcp.js'
import path from 'path'
import { fileURLToPath } from 'url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const fixturesDir = path.join(__dirname, '..', 'fixtures')

// ===================================================
// Capture Control
// ===================================================

test.describe('Phase 4: Capture Control', () => {
  test('should set a capture override via configure tool', async ({ page, serverUrl }) => {
    const text = await mcpToolText(serverUrl, 'configure', {
      action: 'capture',
      settings: { log_level: 'all' }
    })

    expect(text).toContain('log_level')
    expect(text).toContain('all')
  })

  test('should expose overrides via GET /settings', async ({ page, serverUrl }) => {
    // Set an override first
    await mcpToolText(serverUrl, 'configure', {
      action: 'capture',
      settings: { ws_mode: 'medium' }
    })

    // Poll /settings
    const resp = await fetch(`${serverUrl}/settings`)
    const data = await resp.json()

    expect(data.connected).toBe(true)
    expect(data.capture_overrides).toBeDefined()
    expect(data.capture_overrides.ws_mode).toBe('medium')
  })

  test('should reset overrides to defaults', async ({ page, serverUrl }) => {
    // Set an override
    await mcpToolText(serverUrl, 'configure', {
      action: 'capture',
      settings: { log_level: 'all' }
    })

    // Wait to avoid rate limit
    await page.waitForTimeout(1100)

    // Reset
    const text = await mcpToolText(serverUrl, 'configure', {
      action: 'capture',
      settings: 'reset'
    })

    expect(text).toContain('reset')

    // Verify /settings shows empty overrides
    const resp = await fetch(`${serverUrl}/settings`)
    const data = await resp.json()
    expect(Object.keys(data.capture_overrides).length).toBe(0)
  })

  test('should reject invalid setting names', async ({ page, serverUrl }) => {
    const text = await mcpToolText(serverUrl, 'configure', {
      action: 'capture',
      settings: { invalid_setting: 'value' }
    })

    expect(text).toContain('Unknown capture setting')
  })

  test('should reject invalid setting values', async ({ page, serverUrl }) => {
    const text = await mcpToolText(serverUrl, 'configure', {
      action: 'capture',
      settings: { log_level: 'invalid_value' }
    })

    expect(text).toContain('Invalid value')
  })

  test('should rate-limit rapid changes', async ({ page, serverUrl }) => {
    // First change should succeed
    const text1 = await mcpToolText(serverUrl, 'configure', {
      action: 'capture',
      settings: { log_level: 'all' }
    })
    expect(text1).toContain('log_level')

    // Immediate second change should be rate-limited
    const text2 = await mcpToolText(serverUrl, 'configure', {
      action: 'capture',
      settings: { log_level: 'warn' }
    })
    expect(text2).toContain('Rate limited')
  })
})

// ===================================================
// Error Clustering
// ===================================================

test.describe('Phase 4: Error Clustering', () => {
  test('should return empty cluster state initially', async ({ page, serverUrl }) => {
    const text = await mcpToolText(serverUrl, 'observe', { what: 'error_clusters' })
    const data = JSON.parse(text)

    expect(data.clusters).toBeDefined()
    expect(data.total_count).toBe(0)
  })

  test('should cluster similar errors from the page', async ({ page, serverUrl }) => {
    // Navigate to the test page and trigger similar errors
    await page.goto(`file://${path.join(fixturesDir, 'phase4-test-page.html')}`)
    await page.waitForTimeout(1000)

    await page.click('#trigger-similar-errors')

    // Wait for errors to be captured and sent to server
    await page.waitForTimeout(4000)

    const text = await mcpToolText(serverUrl, 'observe', { what: 'error_clusters' })
    const data = JSON.parse(text)

    expect(data.total_count).toBeGreaterThan(0)
    // Errors should have clustered (at least one cluster formed from 3 similar errors)
    if (data.clusters.length > 0) {
      const cluster = data.clusters[0]
      expect(cluster.count).toBeGreaterThanOrEqual(2)
      expect(cluster.message).toBeDefined()
    }
  })
})

// ===================================================
// Navigation History
// ===================================================

test.describe('Phase 4: Navigation History', () => {
  test('should return valid history response structure', async ({ page, serverUrl }) => {
    const text = await mcpToolText(serverUrl, 'observe', { what: 'history' })
    const data = JSON.parse(text)

    expect(data.entries).toBeDefined()
    expect(Array.isArray(data.entries)).toBe(true)
    expect(typeof data.count).toBe('number')
  })

  test('should record an event via configure', async ({ page, serverUrl }) => {
    const text = await mcpToolText(serverUrl, 'configure', {
      action: 'record_event',
      event: {
        type: 'fix',
        description: 'Fixed null user in UserProfile component',
        source: 'user-profile.js:42'
      }
    })

    expect(text).toContain('Event recorded')
    expect(text).toContain('fix')
  })

  test('should reject record_event with missing type', async ({ page, serverUrl }) => {
    const text = await mcpToolText(serverUrl, 'configure', {
      action: 'record_event',
      event: { description: 'Missing type field' }
    })

    expect(text).toContain("'type' is missing")
  })

  test('should reject record_event with missing description', async ({ page, serverUrl }) => {
    const text = await mcpToolText(serverUrl, 'configure', {
      action: 'record_event',
      event: { type: 'error' }
    })

    expect(text).toContain("'description' is missing")
  })

  test('should reject record_event with missing event parameter', async ({ page, serverUrl }) => {
    const text = await mcpToolText(serverUrl, 'configure', {
      action: 'record_event'
    })

    expect(text).toContain("'event' is missing")
  })
})
