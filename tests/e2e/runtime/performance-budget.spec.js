/**
 * E2E Test: Performance Budget Monitor
 *
 * Tests the full pipeline:
 *   Page load → inject.js captures performance data → content.js bridges →
 *   background.js POSTs → /performance-snapshot stores → GET reads
 */
import { test, expect } from '../helpers/extension.js'
import path from 'path'
import { fileURLToPath } from 'url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const fixturesDir = path.join(__dirname, '..', 'fixtures')

test.describe('Performance Budget Monitor', () => {
  test('should capture performance snapshot after page load', async ({ page, serverUrl }) => {
    // Navigate to test page
    await page.goto(`file://${path.join(fixturesDir, 'test-page.html')}`)

    // Wait for load event + 2s delay + background POST buffer
    await page.waitForTimeout(4000)

    // Query the server for captured performance snapshot
    const response = await fetch(`${serverUrl}/performance-snapshot`)
    const data = await response.json()

    // Verify a snapshot was captured
    expect(data.snapshot).not.toBeNull()
    expect(data.snapshot.url).toBeDefined()
    expect(data.snapshot.timestamp).toBeDefined()
    expect(data.snapshot.timing).toBeDefined()
    expect(data.snapshot.network).toBeDefined()
    expect(data.snapshot.longTasks).toBeDefined()
  })

  test('should include navigation timing in snapshot', async ({ page, serverUrl }) => {
    await page.goto(`file://${path.join(fixturesDir, 'test-page.html')}`)
    await page.waitForTimeout(4000)

    const response = await fetch(`${serverUrl}/performance-snapshot`)
    const data = await response.json()

    expect(data.snapshot).not.toBeNull()

    // Navigation timing fields should be present and non-negative
    expect(data.snapshot.timing.domContentLoaded).toBeGreaterThanOrEqual(0)
    expect(data.snapshot.timing.load).toBeGreaterThanOrEqual(0)
    expect(data.snapshot.timing.timeToFirstByte).toBeGreaterThanOrEqual(0)
    expect(data.snapshot.timing.domInteractive).toBeGreaterThanOrEqual(0)
  })

  test('should include network summary in snapshot', async ({ page, serverUrl }) => {
    await page.goto(`file://${path.join(fixturesDir, 'test-page.html')}`)
    await page.waitForTimeout(4000)

    const response = await fetch(`${serverUrl}/performance-snapshot`)
    const data = await response.json()

    expect(data.snapshot).not.toBeNull()

    // Network summary should have valid structure
    expect(data.snapshot.network.requestCount).toBeGreaterThanOrEqual(0)
    expect(data.snapshot.network.transferSize).toBeGreaterThanOrEqual(0)
    expect(data.snapshot.network.decodedSize).toBeGreaterThanOrEqual(0)
    expect(data.snapshot.network.byType).toBeDefined()
    expect(data.snapshot.network.slowestRequests).toBeDefined()
    expect(Array.isArray(data.snapshot.network.slowestRequests)).toBe(true)
  })

  test('should build baseline from multiple page loads', async ({ page, serverUrl }) => {
    // Load the page twice to build baseline
    await page.goto(`file://${path.join(fixturesDir, 'test-page.html')}`)
    await page.waitForTimeout(4000)

    // Navigate away and back to trigger another snapshot
    await page.goto('about:blank')
    await page.waitForTimeout(500)
    await page.goto(`file://${path.join(fixturesDir, 'test-page.html')}`)
    await page.waitForTimeout(4000)

    // Query the server - baseline should exist after 2 loads
    const response = await fetch(`${serverUrl}/performance-snapshot`)
    const data = await response.json()

    expect(data.snapshot).not.toBeNull()
    expect(data.baseline).not.toBeNull()
    expect(data.baseline.sampleCount).toBeGreaterThanOrEqual(2)
    expect(data.baseline.timing).toBeDefined()
    expect(data.baseline.network).toBeDefined()
  })

  test('should clear snapshots via DELETE endpoint', async ({ page, serverUrl }) => {
    await page.goto(`file://${path.join(fixturesDir, 'test-page.html')}`)
    await page.waitForTimeout(4000)

    // Verify snapshot exists
    let response = await fetch(`${serverUrl}/performance-snapshot`)
    let data = await response.json()
    expect(data.snapshot).not.toBeNull()

    // Clear snapshots
    await fetch(`${serverUrl}/performance-snapshot`, { method: 'DELETE' })

    // Verify cleared
    response = await fetch(`${serverUrl}/performance-snapshot`)
    data = await response.json()
    expect(data.snapshot).toBeNull()
  })

  test('should include long task metrics', async ({ page, serverUrl }) => {
    await page.goto(`file://${path.join(fixturesDir, 'test-page.html')}`)
    await page.waitForTimeout(4000)

    const response = await fetch(`${serverUrl}/performance-snapshot`)
    const data = await response.json()

    expect(data.snapshot).not.toBeNull()

    // Long task metrics should be present with valid structure
    expect(data.snapshot.longTasks).toBeDefined()
    expect(typeof data.snapshot.longTasks.count).toBe('number')
    expect(typeof data.snapshot.longTasks.totalBlockingTime).toBe('number')
  })

  test('should have valid timestamp in ISO format', async ({ page, serverUrl }) => {
    await page.goto(`file://${path.join(fixturesDir, 'test-page.html')}`)
    await page.waitForTimeout(4000)

    const response = await fetch(`${serverUrl}/performance-snapshot`)
    const data = await response.json()

    expect(data.snapshot).not.toBeNull()
    expect(data.snapshot.timestamp).toContain('T')
    // Should be parseable as a date
    const date = new Date(data.snapshot.timestamp)
    expect(date.getTime()).not.toBeNaN()
  })
})
