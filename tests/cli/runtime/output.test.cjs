/**
 * Unit tests for lib/output.js
 * Tests output formatters: success, error, warning, info, install/doctor/uninstall result formatters
 */

const test = require('node:test')
const assert = require('node:assert')
const output = require('../../../npm/kaboom-agentic-browser/lib/output')

test('output.success returns formatted success message', () => {
  const result = output.success('Kaboom installed')

  assert.ok(typeof result === 'string', 'Should return string')
  assert.ok(result.includes('✅'), 'Should include success emoji')
  assert.ok(result.includes('Kaboom installed'), 'Should include message')
})

test('output.success with details includes details', () => {
  const result = output.success('Install complete', 'Updated 2 configs')

  assert.ok(result.includes('Install complete'), 'Should include message')
  assert.ok(result.includes('Updated 2 configs'), 'Should include details')
})

test('output.error returns formatted error message', () => {
  const result = output.error('Installation failed')

  assert.ok(typeof result === 'string', 'Should return string')
  assert.ok(result.includes('❌'), 'Should include error emoji')
  assert.ok(result.includes('Installation failed'), 'Should include message')
})

test('output.error with recovery includes recovery suggestions', () => {
  const result = output.error('Permission denied', 'Run with sudo or check file permissions')

  assert.ok(result.includes('Permission denied'), 'Should include message')
  assert.ok(result.includes('sudo'), 'Should include recovery suggestion')
})

test('output.warning returns formatted warning message', () => {
  const result = output.warning('Config not found')

  assert.ok(typeof result === 'string', 'Should return string')
  assert.ok(result.includes('⚠️'), 'Should include warning emoji')
  assert.ok(result.includes('Config not found'), 'Should include message')
})

test('output.info returns formatted info message', () => {
  const result = output.info('4 tools available')

  assert.ok(typeof result === 'string', 'Should return string')
  assert.ok(result.includes('ℹ️'), 'Should include info emoji')
  assert.ok(result.includes('4 tools available'), 'Should include message')
})

test('output.installResult formats successful installation', () => {
  const result = output.installResult({
    updated: [{ name: 'Claude Desktop', path: '/path' }],
    total: 4,
    errors: [],
    notFound: []
  })

  assert.ok(typeof result === 'string', 'Should return string')
  assert.ok(result.includes('✅'), 'Should include success indicator')
  assert.ok(result.includes('Claude Desktop'), 'Should mention tool')
})

test('output.installResult shows errors when present', () => {
  const result = output.installResult({
    updated: [],
    total: 4,
    errors: [{ name: 'Claude Desktop', message: 'Permission denied' }],
    notFound: []
  })

  assert.ok(result.includes('❌'), 'Should include error indicator')
  assert.ok(result.includes('Permission denied'), 'Should mention error')
})

test('output.installResult handles partial installations', () => {
  const result = output.installResult({
    updated: [
      { name: 'Claude Desktop', path: '/path1' },
      { name: 'VSCode', path: '/path2' }
    ],
    total: 4,
    errors: [{ name: 'Cursor', message: 'Config not found' }],
    notFound: ['Codeium']
  })

  assert.ok(typeof result === 'string', 'Should return string')
  assert.ok(result.includes('Claude Desktop'), 'Should show updated tool')
})

test('output.diagnosticReport formats diagnostic results', () => {
  const report = {
    tools: [
      {
        name: 'Claude Desktop',
        path: '/path/to/.claude.json',
        status: 'ok',
        issues: [],
        suggestions: []
      },
      {
        name: 'VSCode',
        path: '/path/to/vscode.mcp.json',
        status: 'error',
        issues: ['Config invalid'],
        suggestions: ['Fix JSON syntax']
      }
    ],
    binary: { ok: true, version: '1.0.0', path: '/path/to/binary' },
    summary: 'Summary: 1 tool ready, 1 needs repair'
  }

  const result = output.diagnosticReport(report)

  assert.ok(typeof result === 'string', 'Should return string')
  assert.ok(result.includes('Claude Desktop'), 'Should mention tools')
  assert.ok(result.includes('Configured and ready'), 'Should show ok status')
  assert.ok(result.includes('Issue:'), 'Should show issues')
  assert.doesNotMatch(result, /Gasoline/)
})

test('output.diagnosticReport includes binary version', () => {
  const report = {
    tools: [],
    binary: { ok: true, version: '1.2.3' },
    summary: 'Summary: 0 tools ready'
  }

  const result = output.diagnosticReport(report)

  assert.ok(result.includes('1.2.3'), 'Should include binary version')
  assert.match(result, /Kaboom/)
})

test('output.uninstallResult formats uninstall results', () => {
  const result = output.uninstallResult({
    removed: [{ name: 'Claude Desktop', path: '/path' }],
    notConfigured: ['VSCode'],
    errors: [],
    success: true
  })

  assert.ok(typeof result === 'string', 'Should return string')
  assert.ok(result.includes('Claude Desktop'), 'Should mention removed tool')
  assert.ok(result.includes('✅'), 'Should include success emoji')
})

test('output.uninstallResult shows not configured tools', () => {
  const result = output.uninstallResult({
    removed: [],
    notConfigured: ['VSCode', 'Cursor'],
    errors: [],
    success: false
  })

  assert.ok(result.includes('VSCode'), 'Should mention unconfigured tool')
  assert.ok(result.includes('Cursor'), 'Should mention unconfigured tool')
  assert.doesNotMatch(result, /Gasoline/)
})

test('output.uninstallResult shows errors', () => {
  const result = output.uninstallResult({
    removed: [],
    notConfigured: [],
    errors: ['Claude Desktop: Permission denied'],
    success: false
  })

  assert.ok(result.includes('Permission denied'), 'Should mention error')
})

test('output formatters produce consistent strings', () => {
  // Calling same formatter with same input should produce same output
  const success1 = output.success('Test')
  const success2 = output.success('Test')

  assert.strictEqual(success1, success2, 'Same input should produce same output')
})

test('output all formatters return non-empty strings', () => {
  assert.ok(output.success('test').length > 0, 'success should not be empty')
  assert.ok(output.error('test').length > 0, 'error should not be empty')
  assert.ok(output.warning('test').length > 0, 'warning should not be empty')
  assert.ok(output.info('test').length > 0, 'info should not be empty')
})

test('output installResult handles various input combinations', () => {
  const result1 = output.installResult({ updated: [], total: 4, errors: [], notFound: [] })
  const result2 = output.installResult({
    updated: [
      { name: 'T1', path: 'P1' },
      { name: 'T2', path: 'P2' },
      { name: 'T3', path: 'P3' },
      { name: 'T4', path: 'P4' }
    ],
    total: 4,
    errors: [],
    notFound: []
  })
  const result3 = output.installResult({
    updated: [
      { name: 'T1', path: 'P1' },
      { name: 'T2', path: 'P2' }
    ],
    total: 4,
    errors: [{ name: 'E1', message: 'Error' }],
    notFound: ['T']
  })

  assert.ok(result1.length >= 0, 'Should handle zero updates')
  assert.ok(result2.length > 0, 'Should handle full installation')
  assert.ok(result3.length > 0, 'Should handle partial installation')
})

test('output formatters include emoji markers', () => {
  const success = output.success('test')
  const error = output.error('test')
  const warning = output.warning('test')
  const info = output.info('test')

  // Each should have its emoji
  assert.ok(success.includes('✅'), 'success should have ✅')
  assert.ok(error.includes('❌'), 'error should have ❌')
  assert.ok(warning.includes('⚠️'), 'warning should have ⚠️')
  assert.ok(info.includes('ℹ️'), 'info should have ℹ️')
})

test('output.diagnosticReport handles empty tools array', () => {
  const report = {
    tools: [],
    binary: { ok: true },
    summary: 'Empty'
  }

  const result = output.diagnosticReport(report)
  assert.ok(typeof result === 'string', 'Should handle empty tools')
})

test('output.uninstallResult handles all empty', () => {
  const result = output.uninstallResult({
    removed: [],
    notConfigured: [],
    errors: [],
    success: false
  })

  assert.ok(typeof result === 'string', 'Should handle all empty')
})
