/**
 * E2E Test: MCP over HTTP (POST /mcp)
 *
 * Tests the full pipeline via the HTTP-accessible MCP endpoint:
 *   Browser extension captures → Server stores → POST /mcp retrieves
 *
 * Covers: tools/list, observe (errors, logs, network_bodies, websocket_events,
 *         websocket_status, performance), configure (clear)
 */
import { test, expect } from '../helpers/extension.js'
import { mcpCall, mcpToolText } from '../helpers/mcp.js'
import path from 'path'
import { fileURLToPath } from 'url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const fixturesDir = path.join(__dirname, '..', 'fixtures')

test.describe('MCP HTTP Endpoint', () => {
  test('should respond to tools/list with all available tools', async ({ page, serverUrl }) => {
    const resp = await mcpCall(serverUrl, 'tools/list')

    expect(resp.jsonrpc).toBe('2.0')
    expect(resp.error).toBeUndefined()

    const toolNames = resp.result.tools.map((t) => t.name)

    // 4 composite tools
    expect(toolNames).toContain('observe')
    expect(toolNames).toContain('generate')
    expect(toolNames).toContain('configure')
    expect(toolNames).toContain('interact')
  })

  test('should return error for unknown method', async ({ page, serverUrl }) => {
    const resp = await mcpCall(serverUrl, 'unknown/method')

    expect(resp.error).toBeDefined()
    expect(resp.error.code).toBe(-32601)
  })

  test('should return error for unknown tool', async ({ page, serverUrl }) => {
    const resp = await mcpCall(serverUrl, 'tools/call', {
      name: 'nonexistent_tool',
      arguments: {}
    })

    expect(resp.error).toBeDefined()
  })
})

test.describe('MCP: Browser Logs', () => {
  test('should capture console.error via MCP', async ({ page, serverUrl }) => {
    await page.goto(`file://${path.join(fixturesDir, 'mcp-test-page.html')}`)
    await page.waitForTimeout(1000)

    // Trigger error
    await page.click('#trigger-error')
    await page.waitForTimeout(3000)

    // Query via MCP - returns JSON array as text when entries exist
    const text = await mcpToolText(serverUrl, 'observe', { what: 'errors' })

    // Should be a JSON array of error entries (not the "no errors" message)
    const errors = JSON.parse(text)
    expect(Array.isArray(errors)).toBe(true)
    expect(errors.length).toBeGreaterThan(0)

    const mcpError = errors.find((e) => e.args?.some((a) => typeof a === 'string' && a.includes('MCP test error')))
    expect(mcpError).toBeDefined()
  })

  test('should return empty message after clearing', async ({ page, serverUrl }) => {
    // Clear first
    await mcpCall(serverUrl, 'tools/call', { name: 'configure', arguments: { action: 'clear' } })

    const text = await mcpToolText(serverUrl, 'observe', { what: 'logs' })
    expect(text).toBe('No browser logs found')
  })
})

test.describe('MCP: WebSocket Events', () => {
  test('should return empty message when no WebSocket connections', async ({ page, serverUrl }) => {
    const text = await mcpToolText(serverUrl, 'observe', { what: 'websocket_events' })
    expect(text).toBe('No WebSocket events captured')
  })

  test('should return empty status when no WebSocket connections', async ({ page, serverUrl }) => {
    const text = await mcpToolText(serverUrl, 'observe', { what: 'websocket_status' })
    // WebSocket status returns JSON with empty connections array
    const data = JSON.parse(text)
    expect(data.connections).toBeDefined()
    expect(data.connections.length).toBe(0)
  })
})

test.describe('MCP: Network Bodies', () => {
  test('should return empty message when nothing captured', async ({ page, serverUrl }) => {
    const text = await mcpToolText(serverUrl, 'observe', { what: 'network_bodies' })
    expect(text).toBe('No network bodies captured')
  })
})

test.describe('MCP: Performance', () => {
  test('should return no data message when no snapshots captured', async ({ page, serverUrl }) => {
    const text = await mcpToolText(serverUrl, 'observe', { what: 'performance' })
    expect(text).toContain('No performance snapshot available')
  })
})
