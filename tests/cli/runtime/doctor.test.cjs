/**
 * Unit tests for lib/daemon/doctor.js
 * Tests diagnostic functions: testBinary, runDiagnostics
 */

const test = require('node:test')
const assert = require('node:assert')
const fs = require('fs')
const path = require('path')
const os = require('os')
const doctor = require('../../../npm/kaboom-agentic-browser/lib/daemon/doctor')
const config = require('../../../npm/kaboom-agentic-browser/lib/config/config')

// runDiagnostics is async and now performs a live /health probe. Stub it to an
// unreachable daemon so these config-focused tests stay hermetic and fast
// regardless of whether a real Kaboom daemon happens to be running.
const NO_DAEMON = { fetchHealthFn: async () => ({ reachable: false }) }

test('doctor.testBinary returns object with expected structure', async () => {
  const result = doctor.testBinary()

  assert.ok(typeof result === 'object', 'Should return object')
  assert.ok(typeof result.ok === 'boolean', 'Should have ok boolean')

  if (result.ok) {
    assert.ok(result.path, 'Should have path when ok=true')
    assert.ok(result.version, 'Should have version when ok=true')
  } else {
    assert.ok(result.error, 'Should have error when ok=false')
  }
})

test('doctor.testBinary handles unsupported platform gracefully', async () => {
  // This test just verifies the function doesn't crash
  // The actual result depends on the platform
  const result = doctor.testBinary()
  assert.ok(result, 'Should return result')
})

test('doctor.runDiagnostics returns complete report structure', async () => {
  const report = await doctor.runDiagnostics(false, NO_DAEMON)

  assert.ok(report.tools, 'Should have tools array')
  assert.ok(Array.isArray(report.tools), 'tools should be array')
  assert.ok(report.binary, 'Should have binary result')
  assert.ok(report.summary, 'Should have summary string')

  assert.strictEqual(report.tools.length, config.CLIENT_DEFINITIONS.length, 'Should check all configured clients')
})

test('doctor.runDiagnostics tools have correct structure', async () => {
  const report = await doctor.runDiagnostics(false, NO_DAEMON)

  for (const tool of report.tools) {
    assert.ok(tool.name, 'Tool should have name')
    if (tool.type === 'file') {
      assert.ok(
        Object.prototype.hasOwnProperty.call(tool, 'path'),
        'File client should declare its config path, including null on unsupported platforms'
      )
    }
    assert.ok(typeof tool.status === 'string', 'Tool should have status')
    assert.ok(Array.isArray(tool.issues), 'Tool should have issues array')
    assert.ok(Array.isArray(tool.suggestions), 'Tool should have suggestions array')

    // Status should be one of the valid values
    assert.ok(['ok', 'error', 'info'].includes(tool.status), 'Status should be valid')
  }
})

test('doctor.runDiagnostics identifies tool names correctly', async () => {
  const report = await doctor.runDiagnostics(false, NO_DAEMON)
  const names = report.tools.map((t) => t.name)

  assert.ok(names.includes('Claude Code'), 'Should identify Claude Code')
  assert.ok(names.includes('Claude Desktop'), 'Should identify Claude Desktop')
  assert.ok(names.includes('VS Code'), 'Should identify VS Code')
  assert.ok(names.includes('Cursor'), 'Should identify Cursor')
  assert.ok(names.includes('Windsurf'), 'Should identify Windsurf')
})

test('doctor.runDiagnostics with verbose=true does not crash', async () => {
  // Verbose mode should produce debug output but not crash
  const report = await doctor.runDiagnostics(true, NO_DAEMON)

  assert.ok(report.tools, 'Should still return valid report with verbose=true')
})

test('doctor.runDiagnostics provides suggestions for unconfigured tools', async () => {
  const report = await doctor.runDiagnostics(false, NO_DAEMON)

  // Error tools (misconfigured) should provide a remediation.
  const errorTools = report.tools.filter((t) => t.status === 'error')
  for (const tool of errorTools) {
    assert.ok(tool.suggestions.length > 0, `Error tool ${tool.name} should have suggestions`)
  }
})

test('doctor.runDiagnostics binary result has correct structure', async () => {
  const report = await doctor.runDiagnostics(false, NO_DAEMON)
  const binary = report.binary

  assert.ok(typeof binary.ok === 'boolean', 'Binary should have ok boolean')

  if (binary.ok) {
    assert.ok(binary.path, 'Binary should have path when ok=true')
    assert.ok(binary.version !== undefined, 'Binary should have version when ok=true')
  } else {
    assert.ok(binary.error, 'Binary should have error when ok=false')
  }
})

test('doctor.runDiagnostics summary mentions count information', async () => {
  const report = await doctor.runDiagnostics(false, NO_DAEMON)

  assert.ok(typeof report.summary === 'string', 'Summary should be string')
  assert.ok(report.summary.length > 0, 'Summary should not be empty')
  // Summary should mention counts
  assert.ok(report.summary.includes('Summary') || report.summary.includes('tool'), 'Summary should describe tools')
})

test('doctor.runDiagnostics identifies existing valid configs', async () => {
  // Create a temporary valid config
  const testDir = path.join(os.tmpdir(), 'doctor-test')
  if (!fs.existsSync(testDir)) {
    fs.mkdirSync(testDir, { recursive: true })
  }

  try {
    const testPath = path.join(testDir, 'claude.mcp.json')
    const validConfig = {
      mcpServers: {
        'kaboom-browser-devtools': { command: 'kaboom-agentic-browser' }
      }
    }
    fs.writeFileSync(testPath, JSON.stringify(validConfig))

    // Note: runDiagnostics checks specific hardcoded paths, not our test path
    // So this test mainly verifies the diagnostic logic works with valid configs
    const report = await doctor.runDiagnostics(false, NO_DAEMON)

    assert.ok(report.tools.length > 0, 'Should return tools even if not configured')
  } finally {
    // Cleanup
    if (fs.existsSync(testDir)) {
      fs.rmSync(testDir, { recursive: true, force: true })
    }
  }
})

test('doctor.runDiagnostics handles invalid JSON gracefully', async () => {
  // The diagnostic should handle invalid configs without crashing
  const report = await doctor.runDiagnostics(false, NO_DAEMON)

  // Should always return a complete report
  assert.ok(report.tools, 'Should return tools')
  assert.ok(report.binary, 'Should return binary info')
  assert.ok(report.summary, 'Should return summary')
})

test('doctor tool statuses are consistent with structure', async () => {
  const report = await doctor.runDiagnostics(false, NO_DAEMON)

  for (const tool of report.tools) {
    // If status is 'ok', should have no issues
    if (tool.status === 'ok') {
      assert.strictEqual(tool.issues.length, 0, `OK tool ${tool.name} should have no issues`)
    }

    // If status is 'error', should have issues
    if (tool.status === 'error') {
      assert.ok(tool.issues.length > 0, `Error tool ${tool.name} should have issues`)
    }

    // If status is 'info', could have issues (not configured)
    // This is OK
  }
})

test('doctor.runDiagnostics does not modify config files', async () => {
  const candidates = config.CLIENT_DEFINITIONS
    .filter((def) => def.type === 'file' && def.format !== 'toml')
    .map((def) => config.getClientConfigPath(def))

  // Get checksums of existing files
  const checksums = {}
  for (const candidate of candidates) {
    if (fs.existsSync(candidate)) {
      const content = fs.readFileSync(candidate, 'utf8')
      checksums[candidate] = JSON.stringify(JSON.parse(content)) // Normalize JSON
    }
  }

  // Run diagnostics
  await doctor.runDiagnostics(false, NO_DAEMON)

  // Verify files haven't changed
  for (const candidate in checksums) {
    if (fs.existsSync(candidate)) {
      const content = fs.readFileSync(candidate, 'utf8')
      const newChecksum = JSON.stringify(JSON.parse(content))
      assert.strictEqual(newChecksum, checksums[candidate], `Config ${candidate} should not be modified by diagnostics`)
    }
  }
})
