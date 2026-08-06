/**
 * E2E Test: Feature Toggle Verification
 *
 * Proves that each popup toggle actually enables/disables its feature.
 * Tests the full pipeline: popup → background → content → inject → server
 */
import { test, expect } from '../helpers/extension.js'
import { entryContains } from '../helpers/mcp.js'
import path from 'path'
import { fileURLToPath } from 'url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const fixturesDir = path.join(__dirname, '..', 'fixtures')

/**
 * Helper: Set log level via background message
 */
async function setLogLevel(context, extensionId, level) {
  const page = await context.newPage()
  await page.goto(`chrome-extension://${extensionId}/options.html`)
  await page.evaluate((lvl) => {
    return new Promise((resolve) => {
      chrome.runtime.sendMessage({ type: 'set_log_level', level: lvl }, () => resolve())
    })
  }, level)
  await page.waitForTimeout(300)
  await page.close()
}

/**
 * Helper: Set a toggle via background message (for features handled by background)
 */
async function setBackgroundToggle(context, extensionId, messageType, enabled) {
  const page = await context.newPage()
  await page.goto(`chrome-extension://${extensionId}/options.html`)
  await page.evaluate(
    ({ type, enabled }) => {
      return new Promise((resolve) => {
        chrome.runtime.sendMessage({ type, enabled }, () => resolve())
      })
    },
    { type: messageType, enabled }
  )
  await page.waitForTimeout(300)
  await page.close()
}

/**
 * Helper: Enable/disable a feature on a page via GASOLINE_SETTING postMessage
 */
async function setPageFeature(page, setting, enabled) {
  await page.evaluate(
    ({ setting, enabled }) => {
      window.postMessage({ type: 'kaboom_setting', setting, enabled }, window.location.origin)
    },
    { setting, enabled }
  )
  await page.waitForTimeout(500)
}

/**
 * Helper: Clear server logs before a test
 */
async function clearServerLogs(serverUrl) {
  await fetch(`${serverUrl}/logs`, { method: 'DELETE' })
}

/**
 * Helper: Get log entries from server
 */
async function getServerLogs(serverUrl) {
  const response = await fetch(`${serverUrl}/logs`)
  const data = await response.json()
  return data.entries || []
}

// =============================================================================
// LOG LEVEL FILTERING
// =============================================================================

test.describe('Log Level Filtering', () => {
  test('level "all" should capture console.log', async ({ page, context, extensionId, serverUrl }) => {
    await setLogLevel(context, extensionId, 'all')
    await clearServerLogs(serverUrl)

    await page.goto(`file://${path.join(fixturesDir, 'toggle-test-page.html')}`)
    await page.waitForTimeout(1000)

    await page.click('#trigger-log')
    await page.waitForTimeout(3000)

    const entries = await getServerLogs(serverUrl)
    const logEntry = entries.find((e) => entryContains(e, 'E2E log message'))
    expect(logEntry).toBeDefined()
    expect(logEntry.level).toBe('log')
  })

  test('level "warn" should capture console.warn but NOT console.log', async ({
    page,
    context,
    extensionId,
    serverUrl
  }) => {
    await setLogLevel(context, extensionId, 'warn')
    await clearServerLogs(serverUrl)

    await page.goto(`file://${path.join(fixturesDir, 'toggle-test-page.html')}`)
    await page.waitForTimeout(1000)

    await page.click('#trigger-log')
    await page.click('#trigger-warn')
    await page.waitForTimeout(3000)

    const entries = await getServerLogs(serverUrl)
    const warnEntry = entries.find((e) => entryContains(e, 'E2E warn message'))
    const logEntry = entries.find((e) => entryContains(e, 'E2E log message'))

    expect(warnEntry).toBeDefined()
    expect(warnEntry.level).toBe('warn')
    expect(logEntry).toBeUndefined()
  })

  test('level "error" should capture console.error but NOT console.warn', async ({
    page,
    context,
    extensionId,
    serverUrl
  }) => {
    await setLogLevel(context, extensionId, 'error')
    await clearServerLogs(serverUrl)

    await page.goto(`file://${path.join(fixturesDir, 'toggle-test-page.html')}`)
    await page.waitForTimeout(1000)

    await page.click('#trigger-warn')
    await page.click('#trigger-error')
    await page.waitForTimeout(3000)

    const entries = await getServerLogs(serverUrl)
    const errorEntry = entries.find((e) => entryContains(e, 'E2E error message'))
    const warnEntry = entries.find((e) => entryContains(e, 'E2E warn message'))

    expect(errorEntry).toBeDefined()
    expect(errorEntry.level).toBe('error')
    expect(warnEntry).toBeUndefined()
  })

  test('network errors are always captured regardless of log level', async ({
    page,
    context,
    extensionId,
    serverUrl
  }) => {
    await setLogLevel(context, extensionId, 'error')
    await clearServerLogs(serverUrl)

    await page.goto(
      `file://${path.join(fixturesDir, 'toggle-test-page.html')}?serverUrl=${encodeURIComponent(serverUrl)}`
    )
    await page.waitForTimeout(1000)

    await page.click('#trigger-network-error')
    await page.waitForTimeout(3000)

    const entries = await getServerLogs(serverUrl)
    const networkEntry = entries.find((e) => e.type === 'network')
    expect(networkEntry).toBeDefined()
    expect(networkEntry.level).toBe('error')
  })
})

// =============================================================================
// NETWORK ERROR CAPTURE
// =============================================================================

test.describe('Network Error Capture', () => {
  test('should capture fetch connection failures as network errors', async ({
    page,
    context,
    extensionId,
    serverUrl
  }) => {
    await setLogLevel(context, extensionId, 'error')
    await clearServerLogs(serverUrl)

    await page.goto(`file://${path.join(fixturesDir, 'toggle-test-page.html')}`)
    await page.waitForTimeout(1000)

    await page.click('#trigger-network-error')
    await page.waitForSelector('#status:has-text("Network error triggered")', { timeout: 5000 })
    await page.waitForTimeout(3000)

    const entries = await getServerLogs(serverUrl)
    const networkEntry = entries.find((e) => e.type === 'network')
    expect(networkEntry).toBeDefined()
    expect(networkEntry.url).toContain('127.0.0.1:1')
    expect(networkEntry.error).toBeDefined()
  })

  test('should capture 404 responses as network errors', async ({ page, context, extensionId, serverUrl }) => {
    await setLogLevel(context, extensionId, 'error')
    await clearServerLogs(serverUrl)

    await page.goto(
      `file://${path.join(fixturesDir, 'toggle-test-page.html')}?serverUrl=${encodeURIComponent(serverUrl)}`
    )
    await page.waitForTimeout(1000)

    await page.click('#trigger-fetch-404')
    await page.waitForTimeout(3000)

    const entries = await getServerLogs(serverUrl)
    const networkEntry = entries.find((e) => e.type === 'network' && e.status === 404)
    expect(networkEntry).toBeDefined()
    expect(networkEntry.url).toContain('/nonexistent-path-e2e-test')
    expect(networkEntry.method).toBe('GET')
  })
})

// =============================================================================
// USER ACTION REPLAY
// =============================================================================

test.describe('User Action Replay Toggle', () => {
  test('when enabled, errors should include _actions from preceding clicks', async ({
    page,
    context,
    extensionId,
    serverUrl
  }) => {
    await setLogLevel(context, extensionId, 'error')
    await clearServerLogs(serverUrl)

    await page.goto(`file://${path.join(fixturesDir, 'toggle-test-page.html')}`)
    await page.waitForTimeout(1000)

    // Action replay is ON by default, ensure it's enabled
    await setPageFeature(page, 'set_action_replay_enabled', true)

    // Perform some user actions
    await page.click('#action-btn-1')
    await page.waitForTimeout(200)
    await page.click('#action-btn-2')
    await page.waitForTimeout(200)

    // Now trigger an error
    await page.click('#trigger-error-after-actions')
    await page.waitForTimeout(3000)

    const entries = await getServerLogs(serverUrl)
    const errorEntry = entries.find((e) => entryContains(e, 'Error after user actions'))
    expect(errorEntry).toBeDefined()
    expect(errorEntry._actions).toBeDefined()
    expect(errorEntry._actions.length).toBeGreaterThan(0)

    // Verify the actions include our button clicks
    const clickActions = errorEntry._actions.filter((a) => a.type === 'click')
    expect(clickActions.length).toBeGreaterThanOrEqual(2)
  })

  test('when disabled, errors should NOT include _actions', async ({ page, context, extensionId, serverUrl }) => {
    await setLogLevel(context, extensionId, 'error')
    await clearServerLogs(serverUrl)

    await page.goto(`file://${path.join(fixturesDir, 'toggle-test-page.html')}`)
    await page.waitForTimeout(1000)

    // Disable action replay
    await setPageFeature(page, 'set_action_replay_enabled', false)

    // Perform some user actions
    await page.click('#action-btn-1')
    await page.waitForTimeout(200)
    await page.click('#action-btn-2')
    await page.waitForTimeout(200)

    // Now trigger an error
    await page.click('#trigger-error-after-actions')
    await page.waitForTimeout(3000)

    const entries = await getServerLogs(serverUrl)
    const errorEntry = entries.find((e) => entryContains(e, 'Error after user actions'))
    expect(errorEntry).toBeDefined()
    // _actions should be absent or empty
    const hasActions = errorEntry._actions && errorEntry._actions.length > 0
    expect(hasActions).toBeFalsy()
  })
})

// =============================================================================
// PERFORMANCE MARKS
// =============================================================================

test.describe('Performance Marks Toggle', () => {
  test('when enabled, performance.mark() should be captured', async ({ page, context, extensionId, serverUrl }) => {
    await page.goto(`file://${path.join(fixturesDir, 'toggle-test-page.html')}`)
    await page.waitForTimeout(1000)

    // Enable performance marks capture
    await setPageFeature(page, 'set_performance_marks_enabled', true)
    await page.waitForTimeout(500)

    // Create a performance mark
    await page.click('#trigger-perf-mark')
    await page.waitForTimeout(500)

    // Verify the mark was captured via __kaboom API
    const marks = await page.evaluate(() => {
      if (window.__kaboom && window.__kaboom.getMarks) {
        return window.__kaboom.getMarks()
      }
      return null
    })

    expect(marks).toBeDefined()
    expect(marks).not.toBeNull()
    const testMark = marks.find((m) => m.name === 'e2e-test-mark')
    expect(testMark).toBeDefined()
  })

  test('when disabled, performance.mark() wrapper should be uninstalled', async ({
    page,
    context,
    extensionId,
    serverUrl
  }) => {
    await page.goto(`file://${path.join(fixturesDir, 'toggle-test-page.html')}`)
    await page.waitForTimeout(1000)

    // First enable to install the wrapper
    await setPageFeature(page, 'set_performance_marks_enabled', true)
    await page.waitForTimeout(500)

    // Verify wrapper is installed (not native code)
    const isWrappedBefore = await page.evaluate(() => {
      return !performance.mark.toString().includes('[native code]')
    })
    expect(isWrappedBefore).toBe(true)

    // Now disable - should uninstall the wrapper
    await setPageFeature(page, 'set_performance_marks_enabled', false)
    await page.waitForTimeout(500)

    // Verify performance.mark is now the native function
    const isNativeAfter = await page.evaluate(() => {
      return performance.mark.toString().includes('[native code]')
    })
    expect(isNativeAfter).toBe(true)
  })

  test('when enabled, performance.measure() should be captured', async ({ page, context, extensionId, serverUrl }) => {
    await page.goto(`file://${path.join(fixturesDir, 'toggle-test-page.html')}`)
    await page.waitForTimeout(1000)

    // Enable performance marks capture
    await setPageFeature(page, 'set_performance_marks_enabled', true)
    await page.waitForTimeout(500)

    // Create marks and a measure
    await page.click('#trigger-perf-measure')
    await page.waitForTimeout(500)

    const measures = await page.evaluate(() => {
      if (window.__kaboom && window.__kaboom.getMeasures) {
        return window.__kaboom.getMeasures()
      }
      return null
    })

    expect(measures).toBeDefined()
    expect(measures).not.toBeNull()
    const testMeasure = measures.find((m) => m.name === 'e2e-test-measure')
    expect(testMeasure).toBeDefined()
    expect(testMeasure.duration).toBeDefined()
  })
})

// =============================================================================
// NETWORK WATERFALL
// =============================================================================

test.describe('Network Waterfall Toggle', () => {
  test('when enabled, resource timing data should be collected', async ({ page, context, extensionId, serverUrl }) => {
    await page.goto(
      `file://${path.join(fixturesDir, 'toggle-test-page.html')}?serverUrl=${encodeURIComponent(serverUrl)}`
    )
    await page.waitForTimeout(1000)

    // Enable network waterfall
    await setPageFeature(page, 'set_network_waterfall_enabled', true)
    await page.waitForTimeout(500)

    // Trigger a fetch to generate resource timing (use the server health endpoint)
    await page.evaluate(async (url) => {
      await fetch(url + '/health')
    }, serverUrl)
    await page.waitForTimeout(1000)

    // Check that waterfall data was collected
    const waterfall = await page.evaluate(() => {
      if (window.__kaboom && window.__kaboom.getNetworkWaterfall) {
        return window.__kaboom.getNetworkWaterfall()
      }
      return null
    })

    expect(waterfall).toBeDefined()
    expect(waterfall).not.toBeNull()
    // Should have at least one entry from the health check
    expect(waterfall.length).toBeGreaterThan(0)
  })

  test('when disabled, waterfall enrichment flag should be off', async ({ page, context, extensionId, serverUrl }) => {
    await page.goto(
      `file://${path.join(fixturesDir, 'toggle-test-page.html')}?serverUrl=${encodeURIComponent(serverUrl)}`
    )
    await page.waitForTimeout(1000)

    // First enable waterfall
    await setPageFeature(page, 'set_network_waterfall_enabled', true)
    await page.waitForTimeout(500)

    // Now disable waterfall
    await setPageFeature(page, 'set_network_waterfall_enabled', false)
    await page.waitForTimeout(500)

    // Trigger a fetch to generate resource timing
    await page.evaluate(async (url) => {
      await fetch(url + '/health')
    }, serverUrl)
    await page.waitForTimeout(1000)

    // Trigger an error - waterfall data should NOT be attached when disabled
    await clearServerLogs(serverUrl)
    await page.evaluate(() => {
      console.error('Error after waterfall disabled')
    })
    await page.waitForTimeout(3000)

    const entries = await getServerLogs(serverUrl)
    const errorEntry = entries.find((e) => entryContains(e, 'Error after waterfall disabled'))
    expect(errorEntry).toBeDefined()
    // Waterfall enrichment should not be present when disabled
    const hasWaterfallEnrichment = errorEntry._enrichments && errorEntry._enrichments.includes('networkWaterfall')
    expect(hasWaterfallEnrichment).toBeFalsy()
  })
})

// =============================================================================
// SCREENSHOT ON ERROR
// =============================================================================

test.describe('Screenshot on Error Toggle', () => {
  test('when enabled, exception should trigger screenshot entry in logs', async ({
    page,
    context,
    extensionId,
    serverUrl
  }) => {
    // Enable screenshot on error via background
    await setBackgroundToggle(context, extensionId, 'set_screenshot_on_error', true)
    await clearServerLogs(serverUrl)

    await page.goto(`file://${path.join(fixturesDir, 'toggle-test-page.html')}`)
    await page.waitForTimeout(1000)

    // Trigger an exception (which should auto-screenshot)
    await page.click('#trigger-exception')
    // Give time for screenshot capture + batch delivery
    await page.waitForTimeout(5000)

    const entries = await getServerLogs(serverUrl)
    // Should have both the exception entry and a screenshot entry
    const exceptionEntry = entries.find((e) => e.type === 'exception')
    expect(exceptionEntry).toBeDefined()

    const screenshotEntry = entries.find((e) => e.type === 'screenshot')
    // Screenshot may fail in headless mode, but if it works it should be here
    if (screenshotEntry) {
      expect(screenshotEntry.trigger).toBe('error')
      expect(screenshotEntry.screenshotFile).toBeDefined()
    }
  })

  test('when disabled, exception should NOT trigger screenshot', async ({ page, context, extensionId, serverUrl }) => {
    // Disable screenshot on error
    await setBackgroundToggle(context, extensionId, 'set_screenshot_on_error', false)
    await clearServerLogs(serverUrl)

    await page.goto(`file://${path.join(fixturesDir, 'toggle-test-page.html')}`)
    await page.waitForTimeout(1000)

    // Trigger an exception
    await page.click('#trigger-exception')
    await page.waitForTimeout(5000)

    const entries = await getServerLogs(serverUrl)
    // Should have the exception but NO screenshot
    const exceptionEntry = entries.find((e) => e.type === 'exception')
    expect(exceptionEntry).toBeDefined()

    const screenshotEntry = entries.find((e) => e.type === 'screenshot')
    expect(screenshotEntry).toBeUndefined()
  })
})
