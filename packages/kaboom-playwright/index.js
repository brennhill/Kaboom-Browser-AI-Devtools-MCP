// @ts-check
/**
 * @fileoverview Playwright fixture for Kaboom CI browser observability.
 * Automatically injects kaboom-ci.js into every page, manages test boundaries,
 * and captures failure context for Playwright reports.
 */

const { test: base } = require('@playwright/test')
const { resolve } = require('path')

const CI_SCRIPT_PATH = resolve(require.resolve('@anthropic/kaboom-ci'))

/**
 * @param {import('@playwright/test').TestInfo} testInfo
 * @returns {string}
 */
function deriveTestId(testInfo) {
  return testInfo.titlePath.join(' > ')
}

/**
 * @param {string} label
 * @param {number | undefined} value
 * @returns {string}
 */
function statLine(label, value) {
  return `${label}: ${value || 0}`
}

/**
 * @param {string[]} lines
 * @param {object[]} logs
 */
function appendErrorLines(lines, logs) {
  lines.push('', '--- Errors ---')
  for (const log of logs.filter((l) => l.level === 'error')) {
    lines.push(`  [${log.source || 'unknown'}] ${log.message || ''}`)
    if (log.stack) lines.push(`    ${log.stack.split('\n')[0]}`)
  }
}

/**
 * @param {string[]} lines
 * @param {object[]} networkBodies
 */
function appendNetworkFailureLines(lines, networkBodies) {
  lines.push('', '--- Network Failures ---')
  for (const body of networkBodies.filter((b) => b.status >= 400)) {
    lines.push(`  ${body.method} ${body.url} → ${body.status}`)
    if (body.responseBody) {
      lines.push(`    ${body.responseBody.slice(0, 200)}`)
    }
  }
}

/**
 * @param {object} snapshot
 * @returns {string}
 */
function formatFailureSummary(snapshot) {
  if (!snapshot) return '=== Kaboom Failure Context ===\nNo snapshot data available.'

  const stats = snapshot.stats || {}
  const logs = Array.isArray(snapshot.logs) ? snapshot.logs : []
  const networkBodies = Array.isArray(snapshot.network_bodies) ? snapshot.network_bodies : []

  const lines = [
    '=== Kaboom Failure Context ===',
    `Captured at: ${snapshot.timestamp || 'unknown'}`,
    '',
    '--- Stats ---',
    statLine('Total logs', stats.total_logs),
    statLine('Errors', stats.error_count),
    statLine('Warnings', stats.warning_count),
    statLine('Network failures', stats.network_failures),
    statLine('WebSocket connections', stats.ws_connections)
  ]

  if ((stats.error_count || 0) > 0) {
    appendErrorLines(lines, logs)
  }

  if ((stats.network_failures || 0) > 0) {
    appendNetworkFailureLines(lines, networkBodies)
  }

  return lines.join('\n')
}

const test = base.extend({
  kaboomPort: [7890, { option: true }],
  kaboomAutoStart: [true, { option: true }],
  kaboomAttachOnFailure: [true, { option: true }],

  kaboom: async ({ kaboomPort, kaboomAttachOnFailure, page }, use, testInfo) => {
    const baseUrl = `http://127.0.0.1:${kaboomPort}`

    // Inject capture script into every page
    await page.addInitScript({ path: CI_SCRIPT_PATH })

    // Mark test start
    const testId = deriveTestId(testInfo)
    await fetch(`${baseUrl}/test-boundary`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ test_id: testId, action: 'start' })
    }).catch(() => {
      /* no-op */
    }) // Server might not be running yet

    const fixture = {
      getSnapshot: async (since) => {
        try {
          const params = new URLSearchParams()
          if (since) params.set('since', since)
          params.set('test_id', testId)
          const res = await fetch(`${baseUrl}/snapshot?${params}`)
          return res.json()
        } catch {
          return { timestamp: new Date().toISOString(), stats: {}, logs: [], network_bodies: [] }
        }
      },
      clearBuffers: async () => {
        try {
          await fetch(`${baseUrl}/clear`, { method: 'POST' })
        } catch {
          // Server unreachable — safe to ignore
        }
      },
      markTest: async (id, action) => {
        try {
          await fetch(`${baseUrl}/test-boundary`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ test_id: id, action })
          })
        } catch {
          // Server unreachable — safe to ignore
        }
      }
    }

    await use(fixture)

    // After test: if failed and attachOnFailure, grab snapshot
    if (testInfo.status !== testInfo.expectedStatus && kaboomAttachOnFailure) {
      try {
        const snapshot = await fixture.getSnapshot()
        await testInfo.attach('kaboom-snapshot', {
          body: JSON.stringify(snapshot, null, 2),
          contentType: 'application/json'
        })
        const summary = formatFailureSummary(snapshot)
        await testInfo.attach('kaboom-summary', {
          body: summary,
          contentType: 'text/plain'
        })
      } catch {
        // Server unreachable — skip attachment
      }
    }

    // Mark test end
    await fixture.markTest(testId, 'end')

    // Clear between tests
    await fixture.clearBuffers()
    return
  }
})

module.exports = { test }
