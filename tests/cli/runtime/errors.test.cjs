/**
 * Unit tests for lib/config/errors.js
 * Tests custom error classes and their formatting
 */

const test = require('node:test')
const assert = require('node:assert')
const {
  KaboomError,
  PermissionError,
  InvalidJSONError,
  BinaryNotFoundError,
  InvalidEnvFormatError,
  EnvWithoutInstallError,
  ConfigValidationError,
  FileSizeError
} = require('../../../npm/kaboom-agentic-browser/lib/config/errors')

test('KaboomError is base class for all errors', () => {
  const err = new KaboomError('Test message', 'Test recovery')

  assert.ok(err instanceof Error, 'Should extend Error')
  assert.strictEqual(err.name, 'KaboomError')
  assert.strictEqual(err.message, 'Test message')
})

test('KaboomError.format() returns formatted string', () => {
  const err = new KaboomError('Test message', 'Try this fix')
  const formatted = err.format()

  assert.ok(typeof formatted === 'string', 'Should return string')
  assert.ok(formatted.includes('❌'), 'Should include error emoji')
  assert.ok(formatted.includes('Test message'), 'Should include message')
  assert.ok(formatted.includes('Try this fix'), 'Should include recovery')
})

test('PermissionError has correct properties', () => {
  const err = new PermissionError('/path/to/file')

  assert.strictEqual(err.name, 'PermissionError')
  assert.ok(err.message.includes('Permission'), 'Message should mention permission')
  assert.ok(err.recovery, 'Should have recovery suggestion')
  assert.ok(err.format().includes('❌'), 'Formatted should have error emoji')
})

test('InvalidJSONError has correct properties', () => {
  const err = new InvalidJSONError('/path/to/file.json')

  assert.strictEqual(err.name, 'InvalidJSONError')
  assert.ok(err.message.includes('Invalid JSON') || err.message.includes('JSON'), 'Message should mention JSON')
  assert.ok(err.recovery, 'Should have recovery suggestion')
})

test('BinaryNotFoundError has correct properties', () => {
  const err = new BinaryNotFoundError('/path/to/kaboom-agentic-browser')

  assert.strictEqual(err.name, 'BinaryNotFoundError')
  assert.ok(
    err.message.includes('Kaboom') || err.message.includes('binary') || err.message.includes('not found'),
    'Message should mention binary'
  )
  assert.ok(err.recovery, 'Should have recovery suggestion')
})

test('InvalidEnvFormatError has correct properties', () => {
  const err = new InvalidEnvFormatError('INVALID')

  assert.strictEqual(err.name, 'InvalidEnvFormatError')
  assert.ok(err.message.includes('KEY=VALUE') || err.message.includes('format'), 'Message should explain format')
  assert.ok(err.recovery, 'Should have recovery suggestion')
})

test('EnvWithoutInstallError has correct properties', () => {
  const err = new EnvWithoutInstallError()

  assert.strictEqual(err.name, 'EnvWithoutInstallError')
  assert.ok(err.message.includes('--install'), 'Message should mention --install flag')
  assert.ok(err.recovery, 'Should have recovery suggestion')
})

test('ConfigValidationError has correct properties', () => {
  const err = new ConfigValidationError(['mcpServers missing'])

  assert.strictEqual(err.name, 'ConfigValidationError')
  assert.ok(err.message.includes('invalid') || err.message.includes('validation'), 'Message should mention validation')
  assert.ok(err.recovery, 'Should have recovery suggestion')
})

test('FileSizeError has correct properties', () => {
  const err = new FileSizeError('/path/to/large-file.json', 5000000)

  assert.strictEqual(err.name, 'FileSizeError')
  assert.ok(err.message.includes('size') || err.message.includes('large'), 'Message should mention size')
  assert.ok(err.recovery, 'Should have recovery suggestion')
})

test('All errors have recovery properties', () => {
  const errors = [
    new KaboomError('Test', 'Recovery'),
    new PermissionError('/path'),
    new InvalidJSONError('/path'),
    new BinaryNotFoundError('linux-x64'),
    new InvalidEnvFormatError('INVALID'),
    new EnvWithoutInstallError(),
    new ConfigValidationError(['error']),
    new FileSizeError('/path', 5000000)
  ]

  for (const err of errors) {
    assert.ok(err.recovery, `${err.name} should have recovery property`)
  }
})

test('All errors format() methods include emoji', () => {
  const errors = [
    new KaboomError('Test', 'Recovery'),
    new PermissionError('/path'),
    new InvalidJSONError('/path'),
    new BinaryNotFoundError('linux-x64'),
    new InvalidEnvFormatError('INVALID'),
    new EnvWithoutInstallError(),
    new ConfigValidationError(['error']),
    new FileSizeError('/path', 5000000)
  ]

  for (const err of errors) {
    const formatted = err.format()
    assert.ok(formatted.includes('❌'), `${err.name}.format() should include error emoji`)
  }
})

test('Error messages are descriptive and helpful', () => {
  const err = new PermissionError('/home/user/.claude.json')
  const formatted = err.format()

  assert.ok(formatted.includes('Permission'), 'Should mention permission issue')
  assert.ok(formatted.includes('/home/user/.claude'), 'Should include the path')
  assert.ok(formatted.length > 50, 'Should be descriptive (longer message)')
})

test('Error handling preserves stack traces', () => {
  const err = new InvalidJSONError('/path/to/file.json')

  assert.ok(err.stack, 'Should have stack trace')
  assert.ok(err.stack.includes('InvalidJSONError'), 'Stack should include error name')
})

test('Multiple errors can be instantiated independently', () => {
  const err1 = new PermissionError('/path1')
  const err2 = new PermissionError('/path2')

  assert.notStrictEqual(err1, err2, 'Should create separate instances')
  assert.ok(err1.message.includes('/path1'), 'First error should have first path')
  assert.ok(err2.message.includes('/path2'), 'Second error should have second path')
})

test('ConfigValidationError includes validation details', () => {
  const errors = ['mcpServers missing', 'kaboom entry invalid']
  const err = new ConfigValidationError(errors)

  const formatted = err.format()
  assert.ok(formatted.includes('invalid'), 'Should mention validation failure')
  assert.ok(formatted.includes('❌'), 'Should have error emoji')
})

test('FileSizeError shows size information', () => {
  const err = new FileSizeError('/path/file.json', 2097152)

  const formatted = err.format()
  assert.ok(
    formatted.includes('size') ||
      formatted.includes('large') ||
      formatted.includes('2MB') ||
      formatted.includes('2097152'),
    'Should mention size information'
  )
})
