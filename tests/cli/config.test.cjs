/**
 * Unit tests for lib/config.js
 * Tests config file utilities: read, write, validate, merge, parse
 */

const test = require('node:test')
const assert = require('node:assert')
const fs = require('fs')
const path = require('path')
const os = require('os')
const config = require('../../npm/kaboom-agentic-browser/lib/config')
const {
  InvalidJSONError,
  FileSizeError,
  ConfigValidationError: _ConfigValidationError
} = require('../../npm/kaboom-agentic-browser/lib/errors')

const testDir = path.join(os.tmpdir(), 'kaboom-cli-test')

function setupTestDir() {
  if (!fs.existsSync(testDir)) {
    fs.mkdirSync(testDir, { recursive: true })
  }
}

function cleanupTestDir() {
  if (fs.existsSync(testDir)) {
    fs.rmSync(testDir, { recursive: true, force: true })
  }
}

test('config.getConfigCandidates returns expected file-client paths', () => {
  const candidates = config.getConfigCandidates()
  const fileClientCount = config.CLIENT_DEFINITIONS.filter((def) => def.type === 'file').length
  assert.strictEqual(candidates.length, fileClientCount, 'Should return one config path per file-based client')
  assert.ok(candidates.some((p) => p.includes('Claude')), 'Should include Claude Desktop path')
  assert.ok(candidates.some((p) => p.includes('.cursor')), 'Should include Cursor path')
  assert.ok(candidates.some((p) => p.includes('.codeium')), 'Should include Windsurf path')
  assert.ok(candidates.some((p) => p.includes('Code')), 'Should include VS Code path')
})

test('config.getToolNameFromPath identifies tool by path', () => {
  assert.strictEqual(config.getToolNameFromPath('/home/user/Library/Application Support/Claude/claude_desktop_config.json'), 'Claude Desktop')
  assert.strictEqual(config.getToolNameFromPath('/home/user/.config/Code/User/mcp.json'), 'VS Code')
  assert.strictEqual(config.getToolNameFromPath('/home/user/.cursor/mcp.json'), 'Cursor')
  assert.strictEqual(config.getToolNameFromPath('/home/user/.codeium/windsurf/mcp_config.json'), 'Windsurf')
  assert.strictEqual(config.getToolNameFromPath('/home/user/.unknown/mcp.json'), 'Unknown')
})

test('config.readConfigFile reads and parses valid JSON', () => {
  setupTestDir()
  const testFile = path.join(testDir, 'valid-config.json')
  const testData = {
    mcpServers: {
      'kaboom-browser-devtools': { command: 'kaboom-agentic-browser', args: [] }
    }
  }
  fs.writeFileSync(testFile, JSON.stringify(testData))

  const result = config.readConfigFile(testFile)
  assert.strictEqual(result.valid, true, 'Should be valid')
  assert.deepStrictEqual(result.data, testData, 'Data should match')
  assert.strictEqual(result.error, null, 'No error should be present')
  assert.ok(result.stats, 'Stats should be present')

  cleanupTestDir()
})

test('config.readConfigFile returns error for non-existent file', () => {
  const result = config.readConfigFile('/nonexistent/file.json')
  assert.strictEqual(result.valid, false, 'Should be invalid')
  assert.strictEqual(result.data, null, 'Data should be null')
  assert.ok(result.error, 'Error should be present')
})

test('config.readConfigFile throws InvalidJSONError for malformed JSON', () => {
  setupTestDir()
  const testFile = path.join(testDir, 'bad-json.json')
  fs.writeFileSync(testFile, '{ invalid json }')

  assert.throws(() => config.readConfigFile(testFile), InvalidJSONError, 'Should throw InvalidJSONError')

  cleanupTestDir()
})

test('config.readConfigFile throws FileSizeError for oversized file', () => {
  setupTestDir()
  const testFile = path.join(testDir, 'large-file.json')
  // Create file larger than 1MB
  const largeData = JSON.stringify({ data: 'x'.repeat(2 * 1024 * 1024) })
  fs.writeFileSync(testFile, largeData)

  assert.throws(() => config.readConfigFile(testFile), FileSizeError, 'Should throw FileSizeError for large files')

  cleanupTestDir()
})

test('config.writeConfigFile writes file atomically', () => {
  setupTestDir()
  const testFile = path.join(testDir, 'atomic-write.json')
  const testData = {
    mcpServers: {
      'kaboom-browser-devtools': { command: 'kaboom-agentic-browser', args: [] }
    }
  }

  const result = config.writeConfigFile(testFile, testData, false)
  assert.strictEqual(result.success, true, 'Write should succeed')
  assert.strictEqual(result.path, testFile, 'Path should be returned')
  assert.ok(fs.existsSync(testFile), 'File should be created')

  // Verify file contents
  const contents = JSON.parse(fs.readFileSync(testFile, 'utf8'))
  assert.deepStrictEqual(contents, testData, 'File contents should match')

  cleanupTestDir()
})

test('config.writeConfigFile with dryRun=true does not write', () => {
  setupTestDir()
  const testFile = path.join(testDir, 'dry-run.json')
  const testData = {
    mcpServers: {
      'kaboom-browser-devtools': { command: 'kaboom-agentic-browser', args: [] }
    }
  }

  const result = config.writeConfigFile(testFile, testData, true)
  assert.strictEqual(result.success, true, 'Dry-run should succeed')
  assert.strictEqual(!fs.existsSync(testFile), true, 'File should not be created')

  cleanupTestDir()
})

test('config.validateMCPConfig accepts valid config', () => {
  const validConfig = {
    mcpServers: {
      'kaboom-browser-devtools': { command: 'kaboom-agentic-browser' }
    }
  }

  const errors = config.validateMCPConfig(validConfig)
  assert.strictEqual(errors.length, 0, 'Valid config should have no errors')
})

test('config.validateMCPConfig rejects config without mcpServers', () => {
  const invalidConfig = { otherKey: 'value' }
  const errors = config.validateMCPConfig(invalidConfig)
  assert.ok(errors.length > 0, 'Should have errors')
  assert.ok(errors[0].includes('mcpServers'), 'Error should mention mcpServers')
})

test('config.validateMCPConfig rejects non-object mcpServers', () => {
  const invalidConfig = { mcpServers: ['array', 'not', 'object'] }
  const errors = config.validateMCPConfig(invalidConfig)
  assert.ok(errors.length > 0, 'Should have errors')
  assert.ok(errors[0].includes('object'), 'Error should mention object')
})

test('config.mergeKaboomConfig preserves existing entries', () => {
  const existing = {
    mcpServers: {
      other: { command: 'other-tool' }
    }
  }

  const kaboom = { command: 'kaboom-agentic-browser', args: [] }
  const merged = config.mergeKaboomConfig(existing, kaboom, {})

  assert.ok(merged.mcpServers['kaboom-browser-devtools'], 'kaboom entry should be added')
  assert.ok(merged.mcpServers.other, 'other entry should be preserved')
  assert.strictEqual(merged.mcpServers.other.command, 'other-tool', 'other entry unchanged')
})

test('config.mergeKaboomConfig removes legacy entries and adds env vars', () => {
  const existing = { mcpServers: {} }
  existing.mcpServers.gasoline = { command: 'gasoline-mcp', args: [] }
  existing.mcpServers['strum-browser-devtools'] = { command: 'strum-agentic-browser', args: [] }
  const kaboom = { command: 'kaboom-agentic-browser', args: [] }
  const envVars = { DEBUG: '1', API_KEY: 'secret' }

  const merged = config.mergeKaboomConfig(existing, kaboom, envVars)
  assert.deepStrictEqual(merged.mcpServers['kaboom-browser-devtools'].env, envVars, 'Env vars should be added')
  assert.strictEqual(merged.mcpServers.gasoline, undefined, 'legacy gasoline entry should be removed')
  assert.strictEqual(merged.mcpServers['strum-browser-devtools'], undefined, 'legacy strum entry should be removed')
})

test('config.mergeKaboomConfig without env vars does not add empty env object', () => {
  const existing = { mcpServers: {} }
  const kaboom = { command: 'kaboom-agentic-browser', args: [] }

  const merged = config.mergeKaboomConfig(existing, kaboom, {})
  assert.strictEqual(merged.mcpServers['kaboom-browser-devtools'].env, undefined, 'Empty env should not be added')
})

test('config.parseEnvVar parses valid KEY=VALUE', () => {
  const result = config.parseEnvVar('DEBUG=1')
  assert.strictEqual(result.key, 'DEBUG', 'Key should be extracted')
  assert.strictEqual(result.value, '1', 'Value should be extracted')
})

test('config.parseEnvVar parses complex values', () => {
  const result = config.parseEnvVar('API_URL=http://localhost:7890')
  assert.strictEqual(result.key, 'API_URL', 'Key should be extracted')
  assert.strictEqual(result.value, 'http://localhost:7890', 'Complex value should be extracted')
})

test('config.parseEnvVar splits on the first equals sign only', () => {
  const result = config.parseEnvVar('TOKEN=abc=def')
  assert.strictEqual(result.key, 'TOKEN', 'Key should be extracted')
  assert.strictEqual(result.value, 'abc=def', 'Value containing "=" should be kept intact')

  const b64 = config.parseEnvVar('SECRET=dGVzdA==')
  assert.strictEqual(b64.value, 'dGVzdA==', 'Base64 padding should survive')
})

test('config.parseEnvVar rejects invalid format', () => {
  assert.throws(() => config.parseEnvVar('INVALID'), /InvalidEnvFormatError/, 'Should throw for missing equals')

  assert.throws(() => config.parseEnvVar('=value'), /InvalidEnvFormatError/, 'Should throw for missing key')

  assert.throws(() => config.parseEnvVar('KEY='), /InvalidEnvFormatError/, 'Should throw for missing value')
})
