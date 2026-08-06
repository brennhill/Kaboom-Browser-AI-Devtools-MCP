/**
 * E2E Test: Extension Popup Connection Status
 *
 * Tests the popup UI reflects the actual server connection state:
 *   background.js polls /health → updates status → popup reads status
 */
import { test, expect } from '../helpers/extension.js'

test.describe('Popup Connection Status', () => {
  test('should show connected status when server is running', async ({ context, extensionId, serverUrl }) => {
    // Give the extension time to detect the server
    await new Promise((r) => setTimeout(r, 3000))

    // Open the popup
    const popupPage = await context.newPage()
    await popupPage.goto(`chrome-extension://${extensionId}/popup.html`)

    // Wait for status to update
    await popupPage.waitForTimeout(2000)

    // Check connection status
    const statusEl = popupPage.locator('#status')
    await expect(statusEl).toHaveText('Connected')
    await expect(statusEl).toHaveClass(/connected/)

    await popupPage.close()
  })

  test('should show disconnected status when server is stopped', async ({
    context,
    extensionId,
    server,
    serverPort
  }) => {
    // Give the extension time to initially connect
    await new Promise((r) => setTimeout(r, 2000))

    // Kill the server
    server.kill('SIGTERM')
    await new Promise((resolve) => {
      server.on('exit', resolve)
      setTimeout(resolve, 2000)
    })

    // Trigger a health re-check by sending setServerUrl (this calls checkConnectionAndUpdate)
    const triggerPage = await context.newPage()
    await triggerPage.goto(`chrome-extension://${extensionId}/options.html`)
    await triggerPage.evaluate((port) => {
      return new Promise((resolve) => {
        chrome.runtime.sendMessage({ type: 'set_server_url', url: `http://127.0.0.1:${port}` }, () => resolve())
      })
    }, serverPort)
    // Wait for the health check to complete (and fail)
    await triggerPage.waitForTimeout(2000)
    await triggerPage.close()

    // Open the popup
    const popupPage = await context.newPage()
    await popupPage.goto(`chrome-extension://${extensionId}/popup.html`)
    await popupPage.waitForTimeout(2000)

    const statusEl = popupPage.locator('#status')
    await expect(statusEl).toHaveText('Disconnected')
    await expect(statusEl).toHaveClass(/disconnected/)

    await popupPage.close()
  })

  test('should display server URL in popup', async ({ context, extensionId, serverUrl, serverPort }) => {
    await new Promise((r) => setTimeout(r, 3000))

    const popupPage = await context.newPage()
    await popupPage.goto(`chrome-extension://${extensionId}/popup.html`)
    await popupPage.waitForTimeout(2000)

    const serverUrlEl = popupPage.locator('#server-url')
    await expect(serverUrlEl).toContainText(`127.0.0.1:${serverPort}`)

    await popupPage.close()
  })

  test('should display entries count', async ({ page, context, extensionId, serverUrl }) => {
    // Generate some log entries first
    const fixturesDir = new URL('./fixtures/', import.meta.url).pathname
    await page.goto(`file://${fixturesDir}test-page.html`)
    await page.waitForTimeout(1000)
    await page.click('#trigger-error')
    await page.waitForTimeout(3000)

    // Open popup and check entries count
    const popupPage = await context.newPage()
    await popupPage.goto(`chrome-extension://${extensionId}/popup.html`)
    await popupPage.waitForTimeout(2000)

    const entriesEl = popupPage.locator('#entries-count')
    const text = await entriesEl.textContent()
    // Should show entries in format "N / max"
    expect(text).toMatch(/\d+ \/ \d+/)

    // The count should be > 0 since we generated an error
    const count = parseInt(text.split('/')[0].trim(), 10)
    expect(count).toBeGreaterThan(0)

    await popupPage.close()
  })

  test('should clear logs via popup button', async ({ page, context, extensionId, serverUrl }) => {
    // Generate entries
    const fixturesDir = new URL('./fixtures/', import.meta.url).pathname
    await page.goto(`file://${fixturesDir}test-page.html`)
    await page.waitForTimeout(1000)
    await page.click('#trigger-error')
    await page.waitForTimeout(3000)

    // Open popup
    const popupPage = await context.newPage()
    await popupPage.goto(`chrome-extension://${extensionId}/popup.html`)
    await popupPage.waitForTimeout(2000)

    // Click clear button
    await popupPage.click('#clear-btn')
    await popupPage.waitForTimeout(2000)

    // Entries should be 0
    const entriesEl = popupPage.locator('#entries-count')
    await expect(entriesEl).toContainText('0 /')

    await popupPage.close()
  })

  test('should toggle WebSocket capture', async ({ context, extensionId }) => {
    const popupPage = await context.newPage()
    await popupPage.goto(`chrome-extension://${extensionId}/popup.html`)
    await popupPage.waitForTimeout(1000)

    const wsToggle = popupPage.locator('#toggle-websocket')
    const wsModeContainer = popupPage.locator('#ws-mode-container')

    // Initially checked (default: ON)
    await expect(wsToggle).toBeChecked()
    // Mode container should be visible
    await expect(wsModeContainer).toBeVisible()

    // Check that mode defaults to medium
    const modeSelect = popupPage.locator('#ws-mode')
    await expect(modeSelect).toHaveValue('medium')

    await popupPage.close()
  })

  test('should show warning when all mode selected', async ({ context, extensionId }) => {
    const popupPage = await context.newPage()
    await popupPage.goto(`chrome-extension://${extensionId}/popup.html`)
    await popupPage.waitForTimeout(1000)

    const wsModeContainer = popupPage.locator('#ws-mode-container')
    const warning = popupPage.locator('#ws-messages-warning')

    // WS capture is ON by default, mode container should be visible
    await expect(wsModeContainer).toBeVisible()

    // Warning should be hidden in medium mode (default)
    await expect(warning).toBeHidden()

    // Switch to all mode
    const modeSelect = popupPage.locator('#ws-mode')
    await modeSelect.selectOption('all')
    await popupPage.waitForTimeout(500)

    // Warning should now be visible
    await expect(warning).toBeVisible()

    await popupPage.close()
  })
})
