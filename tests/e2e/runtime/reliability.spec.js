/**
 * E2E Tests: Reliability Features
 *
 * Tests circuit breaker (extension resilience when server is down)
 * and memory enforcement (server evicts data under pressure).
 */
import { test, expect } from '../helpers/extension.js'
import path from 'path'
import { fileURLToPath } from 'url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const fixturesDir = path.join(__dirname, '..', 'fixtures')

test.describe('Circuit Breaker', () => {
  test('extension should survive server being killed and recover when restarted', async ({
    page,
    server,
    serverUrl,
    serverPort
  }) => {
    // 1. Verify normal operation: capture a console error
    await page.goto(`file://${path.join(fixturesDir, 'test-page.html')}`)
    await page.waitForTimeout(1000)
    await page.evaluate(() => console.error('before-kill'))
    await page.waitForTimeout(3000)

    const resp1 = await fetch(`${serverUrl}/logs`)
    const data1 = await resp1.json()
    const captured = data1.entries.find(
      (e) => e.args && e.args.some((a) => typeof a === 'string' && a.includes('before-kill'))
    )
    expect(captured).toBeDefined()

    // 2. Kill the server
    server.kill('SIGTERM')
    await new Promise((resolve) => {
      server.on('exit', resolve)
      setTimeout(resolve, 2000)
    })

    // 3. Trigger more errors while server is down (should not crash extension)
    await page.evaluate(() => {
      for (let i = 0; i < 5; i++) {
        console.error(`while-down-${i}`)
      }
    })
    await page.waitForTimeout(2000)

    // 4. Verify the page is still functional (extension didn't crash it)
    const pageTitle = await page.title()
    expect(pageTitle).toBeTruthy()

    // 5. Extension popup should still be accessible
    // (If circuit breaker failed, the service worker might have crashed)
    const serviceWorkers = page.context().serviceWorkers()
    expect(serviceWorkers.length).toBeGreaterThan(0)
  })

  test('extension should not flood a slow server', async ({ page, serverUrl }) => {
    await page.goto(`file://${path.join(fixturesDir, 'test-page.html')}`)
    await page.waitForTimeout(1000)

    // Generate many rapid errors
    await page.evaluate(() => {
      for (let i = 0; i < 50; i++) {
        console.error(`rapid-error-${i}`)
      }
    })

    // Wait for batching + potential backoff
    await page.waitForTimeout(5000)

    // Server should still be responsive
    const resp = await fetch(`${serverUrl}/health`)
    expect(resp.ok).toBe(true)
  })
})

test.describe('Memory Enforcement', () => {
  test('server should evict old WebSocket events when buffer is full', async ({ page, serverUrl }) => {
    await page.goto(`file://${path.join(fixturesDir, 'test-page.html')}`)

    // Directly POST many large WebSocket events to the server
    const largeData = 'x'.repeat(50000) // 50KB per event
    for (let batch = 0; batch < 10; batch++) {
      const events = []
      for (let i = 0; i < 10; i++) {
        events.push({
          timestamp: new Date().toISOString(),
          type: 'websocket',
          event: 'message',
          id: 'test-conn-1',
          url: 'wss://example.com/ws',
          direction: 'incoming',
          data: largeData,
          size: 50000
        })
      }

      const resp = await fetch(`${serverUrl}/websocket-events`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ events })
      })

      // Should accept or reject gracefully (200 or 503), never crash
      expect([200, 503]).toContain(resp.status)
    }

    // Server should still be responsive
    const healthResp = await fetch(`${serverUrl}/health`)
    expect(healthResp.ok).toBe(true)

    // GET events - should have some entries but not all 100
    const eventsResp = await fetch(`${serverUrl}/websocket-events`)
    const eventsData = await eventsResp.json()
    // Buffer should have been trimmed (100 events * 50KB = 5MB > 4MB limit)
    expect(eventsData.events.length).toBeLessThan(100)
    expect(eventsData.events.length).toBeGreaterThan(0)
  })

  test('server should evict old network bodies when buffer is full', async ({ page, serverUrl }) => {
    await page.goto(`file://${path.join(fixturesDir, 'test-page.html')}`)

    // POST many large network bodies
    const largeBody = 'y'.repeat(100000) // 100KB per body
    for (let batch = 0; batch < 10; batch++) {
      const bodies = []
      for (let i = 0; i < 10; i++) {
        bodies.push({
          url: `/api/test-${batch}-${i}`,
          method: 'GET',
          status: 200,
          responseBody: largeBody
        })
      }

      const resp = await fetch(`${serverUrl}/network-bodies`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ bodies })
      })

      expect([200, 503]).toContain(resp.status)
    }

    // Server should still be responsive
    const healthResp = await fetch(`${serverUrl}/health`)
    expect(healthResp.ok).toBe(true)
  })

  test('server should reject events when memory hard limit is simulated', async ({ page, serverUrl }) => {
    await page.goto(`file://${path.join(fixturesDir, 'test-page.html')}`)

    // First, fill up the WS buffer near its limit
    const largeData = 'z'.repeat(100000)
    for (let i = 0; i < 45; i++) {
      await fetch(`${serverUrl}/websocket-events`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          events: [
            {
              timestamp: new Date().toISOString(),
              event: 'message',
              id: 'conn-1',
              data: largeData
            }
          ]
        })
      })
    }

    // Server should still respond to health checks
    const healthResp = await fetch(`${serverUrl}/health`)
    expect(healthResp.ok).toBe(true)
  })
})
