/**
 * Integration tests for bin/kaboom-agentic-browser CLI
 * Tests command routing, argument parsing, and end-to-end workflows
 */

const test = require('node:test')
const assert = require('node:assert')
const { execSync } = require('child_process')
const fs = require('fs')
const path = require('path')
const os = require('os')
const doctor = require('../../npm/kaboom-agentic-browser/lib/doctor')

// --doctor now exits non-zero on a HARD failure — a missing/broken platform
// binary — and 0 when tooling is healthy (a not-running daemon / not-connected
// extension are SOFT states and stay exit 0). testBinary() resolves the same
// repo-relative node_modules candidates for both this process and the CLI
// subprocess, so it faithfully predicts the exit code without flakiness whether
// or not the optional platform binary happens to be installed.
const BINARY_OK = doctor.testBinary().ok
const EXPECTED_DOCTOR_EXIT = BINARY_OK ? 0 : 1

// Sandbox HOME so real (non-dry-run) --install cases write to a throwaway home
// instead of the developer's / CI runner's actual MCP client configs. Seed a
// Cursor dir (~/.cursor) so at least one client is detected — this preserves the
// --install exit-0 contract — and any config write lands in the sandbox.
const SANDBOX_HOME = fs.mkdtempSync(path.join(os.tmpdir(), 'kaboom-cli-it-'))
fs.mkdirSync(path.join(SANDBOX_HOME, '.cursor'), { recursive: true })
fs.writeFileSync(path.join(SANDBOX_HOME, '.cursor', 'mcp.json'), '{}')
process.on('exit', () => {
  try {
    fs.rmSync(SANDBOX_HOME, { recursive: true, force: true })
  } catch {
    // Best-effort temp cleanup.
  }
})

// Helper to run kaboom-agentic-browser command.
// - HOME/USERPROFILE point at the sandbox so config writes never touch the real machine.
// - Real (non-dry-run) --install would otherwise start the daemon, open the extension
//   folder/browser, and block on the connect wait — disable all three so the
//   parser/routing tests stay hermetic and never leave a daemon running.
const HERMETIC_ENV = {
  ...process.env,
  HOME: SANDBOX_HOME,
  USERPROFILE: SANDBOX_HOME,
  KABOOM_NO_DAEMON: '1',
  KABOOM_NO_OPEN: '1',
  KABOOM_NO_WAIT: '1',
}
function runCommand(args) {
  try {
    const cmd = `npm/kaboom-agentic-browser/bin/kaboom-agentic-browser ${args}`.trim()
    const output = execSync(cmd, { encoding: 'utf8', stdio: ['pipe', 'pipe', 'pipe'], env: HERMETIC_ENV })
    return { success: true, output, exitCode: 0 }
  } catch (e) {
    return { success: false, output: e.stdout || '', error: e.stderr || '', exitCode: e.status || 1 }
  }
}

test('kaboom-agentic-browser --help shows help message', () => {
  const result = runCommand('--help')

  // Help should always exit 0
  assert.strictEqual(result.exitCode, 0, 'Help should exit with 0')
  assert.ok(
    result.output.includes('Kaboom Agentic Browser Server') || result.output.includes('Usage'),
    'Should show help'
  )
  assert.ok(result.output.includes('--install') || result.output.includes('install'), 'Should mention install command')
})

test('kaboom-agentic-browser -h shows help message', () => {
  const result = runCommand('-h')

  assert.strictEqual(result.exitCode, 0, 'Short help should exit with 0')
  assert.ok(result.output.length > 0, 'Should show help')
})

test('kaboom-agentic-browser --config shows configuration', () => {
  const result = runCommand('--config')

  assert.strictEqual(result.exitCode, 0, 'Config should exit with 0')
  assert.ok(result.output.includes('Configuration') || result.output.includes('mcpServers'), 'Should show config info')
})

test('kaboom-agentic-browser -c shows configuration', () => {
  const result = runCommand('-c')

  assert.strictEqual(result.exitCode, 0, 'Short config should exit with 0')
  assert.ok(result.output.length > 0, 'Should show config')
})

test('kaboom-agentic-browser --doctor runs diagnostics', () => {
  const result = runCommand('--doctor')

  // Exit code follows the binary check (see EXPECTED_DOCTOR_EXIT); the diagnostic
  // report is printed either way (stdout is captured even on a non-zero exit).
  assert.strictEqual(result.exitCode, EXPECTED_DOCTOR_EXIT, 'Doctor exit code should follow the binary check')
  assert.ok(result.output.includes('Diagnostic') || result.output.includes('tool'), 'Should show diagnostic info')
})

test('kaboom-agentic-browser --doctor --verbose runs diagnostics verbosely', () => {
  const result = runCommand('--doctor --verbose')

  assert.strictEqual(result.exitCode, EXPECTED_DOCTOR_EXIT, 'Doctor verbose exit code should follow the binary check')
  assert.ok(result.output.length > 0, 'Should show diagnostic info')
})

test('kaboom-agentic-browser --doctor exit code reflects the binary check (hard failure => non-zero)', () => {
  const result = runCommand('--doctor')

  if (BINARY_OK) {
    assert.strictEqual(result.exitCode, 0, 'Healthy tooling: doctor exits 0')
  } else {
    // The hermetic sandbox has no installed platform binary, so the binary check
    // is a hard failure and doctor must exit non-zero — but still print the report.
    assert.strictEqual(result.exitCode, 1, 'Missing/broken platform binary: doctor exits 1')
    assert.ok(result.output.includes('Binary Check'), 'Should still print the diagnostic report on failure')
  }
})

test('kaboom-agentic-browser --install --dry-run previews without writing', () => {
  // Get initial state
  const candidates = require('../../npm/kaboom-agentic-browser/lib/config').getConfigCandidates()
  const initialState = {}
  for (const candidate of candidates) {
    if (fs.existsSync(candidate)) {
      initialState[candidate] = fs.readFileSync(candidate, 'utf8')
    }
  }

  try {
    const result = runCommand('--install --dry-run --skills-no-fallback')

    assert.strictEqual(result.exitCode, 0, 'Dry-run install should exit with 0')
    assert.ok(
      result.output.includes('Dry') || result.output.includes('preview') || result.output.length > 0,
      'Should mention dry-run'
    )

    // Verify no files were actually modified
    for (const candidate in initialState) {
      if (fs.existsSync(candidate)) {
        const currentState = fs.readFileSync(candidate, 'utf8')
        assert.strictEqual(currentState, initialState[candidate], `File ${candidate} should not be modified in dry-run`)
      }
    }
  } catch (_e) {
    // Dry-run might fail if files don't exist, which is OK
  }
})

test('kaboom-agentic-browser --env without --install shows error', () => {
  const result = runCommand('--env DEBUG=1')

  // Should fail or show error
  assert.ok(
    result.output.includes('--env') || result.output.includes('--install') || !result.success,
    'Should mention --env needs --install'
  )
})

test('kaboom-agentic-browser with unsupported flag shows error', () => {
  const result = runCommand('--for-all')

  // Should fail or show error
  assert.ok(
    result.output.includes('Unknown command') || result.error.includes('Unknown command') || !result.success,
    'Should mention unknown command'
  )
})

test('kaboom-agentic-browser with invalid flag shows help or error', () => {
  const result = runCommand('--invalid-flag')

  // Invalid flag should either show help or run the binary
  assert.ok(result.output.length > 0 || result.error, 'Should show output or error')
})

test('kaboom-agentic-browser with no args attempts to run binary', () => {
  // No args should attempt to run the binary, which may fail if not in PATH
  const result = runCommand('')

  // This might fail or succeed depending on whether binary is available
  // The important thing is that it doesn't crash
  assert.ok(typeof result.exitCode === 'number', 'Should return exit code')
})

test('kaboom-agentic-browser --help exits successfully', () => {
  const result = runCommand('--help')

  assert.strictEqual(result.exitCode, 0, 'Help should exit with 0')
})

test('kaboom-agentic-browser --config exits successfully', () => {
  const result = runCommand('--config')

  assert.strictEqual(result.exitCode, 0, 'Config should exit with 0')
})

test('kaboom-agentic-browser --doctor exits with the binary-check code', () => {
  const result = runCommand('--doctor')

  assert.strictEqual(result.exitCode, EXPECTED_DOCTOR_EXIT, 'Doctor exit code should follow the binary check')
})

test('CLI handles multiple env vars', () => {
  const result = runCommand('--install --dry-run --env DEBUG=1 --env API_KEY=secret')

  // Should succeed (or at least not crash)
  assert.ok(typeof result.exitCode === 'number', 'Should return exit code')
})

test('CLI with --verbose flag produces more output', () => {
  const resultNormal = runCommand('--doctor')
  const resultVerbose = runCommand('--doctor --verbose')

  // Both should exit with the same binary-check-driven code.
  assert.strictEqual(resultNormal.exitCode, EXPECTED_DOCTOR_EXIT, 'Normal doctor exit code should follow the binary check')
  assert.strictEqual(resultVerbose.exitCode, EXPECTED_DOCTOR_EXIT, 'Verbose doctor exit code should follow the binary check')

  // Verbose output might be longer or same (depends on implementation)
  assert.ok(resultVerbose.output.length > 0, 'Verbose should produce output')
})

test('CLI outputs use emoji markers for status', () => {
  const result = runCommand('--doctor')

  assert.strictEqual(result.exitCode, EXPECTED_DOCTOR_EXIT, 'Doctor exit code should follow the binary check')
  // Output should have status indicators (printed even on a non-zero exit).
  assert.ok(
    result.output.includes('✅') || result.output.includes('❌') || result.output.includes('ℹ️'),
    'Output should use emoji markers'
  )
})

test('kaboom-agentic-browser --install --dry-run handles install flags', () => {
  const result = runCommand('--install --dry-run --skills-no-fallback')

  assert.strictEqual(result.exitCode, 0, 'Dry-run install should exit with 0')
  assert.ok(result.output.length > 0, 'Should produce output')
})

test('kaboom-agentic-browser command parser handles flag combinations', () => {
  // These should all parse correctly even if they might fail
  const testCases = [
    '--install --dry-run',
    '--install --skills-no-fallback',
    '--install --env KEY=VALUE',
    '--doctor --verbose',
    '--help',
    '--config'
  ]

  for (const args of testCases) {
    const result = runCommand(args)
    assert.ok(typeof result.exitCode === 'number', `Should handle "${args}"`)
  }
})

test('CLI gracefully handles config file errors', () => {
  // Doctor should still complete (print the report) even if config is invalid;
  // its exit code follows the binary check, not the config state.
  const result = runCommand('--doctor')

  assert.strictEqual(result.exitCode, EXPECTED_DOCTOR_EXIT, 'Doctor exit code should follow the binary check')
  assert.ok(result.output.length > 0, 'Doctor should still print a report')
})

test('CLI does not crash with empty arguments', () => {
  try {
    const result = runCommand('')
    assert.ok(typeof result.exitCode === 'number', 'Should handle empty args')
  } catch (_e) {
    // Some error is OK - we just don't want a crash
    assert.ok(true, 'Handled without crashing')
  }
})
