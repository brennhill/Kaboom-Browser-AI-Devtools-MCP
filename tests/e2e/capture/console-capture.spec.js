/**
 * E2E Test: Console Error Capture Flow
 *
 * Tests the full pipeline:
 *   Page console.error → inject.js captures → content.js bridges →
 *   background.js batches → POST /logs → GET /logs reads
 */
import { test, expect } from '../helpers/extension.js'
import { entryContains } from '../helpers/mcp.js'
import path from 'path'
import { fileURLToPath } from 'url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const fixturesDir = path.join(__dirname, '..', 'fixtures')

test.describe('Console Error Capture', () => {
  test('should capture console.error and deliver to server', async ({ page, serverUrl }) => {
    // Navigate to test page
    await page.goto(`file://${path.join(fixturesDir, 'test-page.html')}`)

    // Wait for inject.js to be loaded (extension content script runs)
    await page.waitForTimeout(1000)

    // Trigger a console.error
    await page.click('#trigger-error')

    // Wait for the batch to be sent (background.js batches on a timer)
    await page.waitForTimeout(3000)

    // Query the server for captured logs
    const response = await fetch(`${serverUrl}/logs`)
    const data = await response.json()

    // Verify the error was captured
    expect(data.entries).toBeDefined()
    expect(data.entries.length).toBeGreaterThan(0)

    const errorEntry = data.entries.find(
      (e) => e.level === 'error' && e.type === 'console' && entryContains(e, 'Test error message')
    )
    expect(errorEntry).toBeDefined()
    expect(errorEntry.args).toBeDefined()
  })

  test('should capture uncaught exceptions', async ({ page, serverUrl }) => {
    await page.goto(`file://${path.join(fixturesDir, 'test-page.html')}`)
    await page.waitForTimeout(1000)

    // Trigger uncaught exception
    await page.click('#trigger-uncaught')

    // Wait for batch delivery
    await page.waitForTimeout(3000)

    const response = await fetch(`${serverUrl}/logs`)
    const data = await response.json()

    expect(data.entries).toBeDefined()
    const exceptionEntry = data.entries.find(
      (e) => e.level === 'error' && e.type === 'exception' && entryContains(e, 'Uncaught test exception')
    )
    expect(exceptionEntry).toBeDefined()
    expect(exceptionEntry.message).toContain('Uncaught test exception')
  })

  test('should capture unhandled promise rejections', async ({ page, serverUrl }) => {
    await page.goto(`file://${path.join(fixturesDir, 'test-page.html')}`)
    await page.waitForTimeout(1000)

    // Trigger unhandled rejection
    await page.click('#trigger-rejection')

    // Wait for batch delivery
    await page.waitForTimeout(3000)

    const response = await fetch(`${serverUrl}/logs`)
    const data = await response.json()

    expect(data.entries).toBeDefined()
    const rejectionEntry = data.entries.find(
      (e) => e.level === 'error' && e.type === 'exception' && entryContains(e, 'Unhandled rejection test')
    )
    expect(rejectionEntry).toBeDefined()
  })

  test('should not capture console.warn at error log level', async ({ page, serverUrl }) => {
    await page.goto(`file://${path.join(fixturesDir, 'test-page.html')}`)
    await page.waitForTimeout(1000)

    // Trigger a warning (should not be captured at default 'error' level)
    await page.click('#trigger-warn')

    // Wait for potential batch delivery
    await page.waitForTimeout(3000)

    const response = await fetch(`${serverUrl}/logs`)
    const data = await response.json()

    // Warnings should not appear at 'error' log level
    const warnEntry = (data.entries || []).find((e) => e.level === 'warn' && entryContains(e, 'Test warning message'))
    expect(warnEntry).toBeUndefined()
  })

  test('should include source location in captured exceptions', async ({ page, serverUrl }) => {
    await page.goto(`file://${path.join(fixturesDir, 'test-page.html')}`)
    await page.waitForTimeout(1000)

    await page.click('#trigger-uncaught')
    await page.waitForTimeout(3000)

    const response = await fetch(`${serverUrl}/logs`)
    const data = await response.json()

    const exceptionEntry = data.entries?.find(
      (e) => e.level === 'error' && e.type === 'exception' && entryContains(e, 'Uncaught test exception')
    )
    expect(exceptionEntry).toBeDefined()
    // Exception entries include filename and line numbers
    expect(exceptionEntry.filename).toBeDefined()
    expect(exceptionEntry.lineno).toBeDefined()
  })

  test('should clear logs via DELETE endpoint', async ({ page, serverUrl }) => {
    await page.goto(`file://${path.join(fixturesDir, 'test-page.html')}`)
    await page.waitForTimeout(1000)

    // Generate an error
    await page.click('#trigger-error')
    await page.waitForTimeout(3000)

    // Verify error exists
    let response = await fetch(`${serverUrl}/logs`)
    let data = await response.json()
    expect(data.entries.length).toBeGreaterThan(0)

    // Clear logs
    await fetch(`${serverUrl}/logs`, { method: 'DELETE' })

    // Verify logs are cleared
    response = await fetch(`${serverUrl}/logs`)
    data = await response.json()
    expect(data.entries.length).toBe(0)
  })
})
