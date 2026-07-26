// upload-handler.test.ts — Tests for upload escalation logic.
// Tests verifyFileOnInput, clickFileInput, and escalateToStage4.
//
// Run: node --test extension/background/__tests__/upload-handler.test.js
import { test, describe, mock, beforeEach } from 'node:test'
import assert from 'node:assert'
// ============================================
// Mock Chrome APIs
// ============================================
const mockExecuteScript = mock.fn()
const mockChrome = {
  scripting: {
    executeScript: mockExecuteScript
  }
}
globalThis.chrome = mockChrome
// Mock fetch for daemon calls
const mockFetch = mock.fn()
globalThis.fetch = mockFetch
// ============================================
// Import after mocks are set up
// ============================================
// We import the functions that will be added to upload-handler.ts
// These are tested via their injected script behavior
import { verifyFileOnInput, clickFileInput, escalateToStage4 } from '../upload-handler.js'
// ============================================
// verifyFileOnInput tests
// ============================================
describe('verifyFileOnInput', () => {
  beforeEach(() => {
    mockExecuteScript.mock.resetCalls()
  })
  test('returns has_file: false when files array is empty', async () => {
    mockExecuteScript.mock.mockImplementation(() => Promise.resolve([{ result: { has_file: false } }]))
    const result = await verifyFileOnInput(1, '#file-input')
    assert.strictEqual(result.has_file, false)
  })
  test('returns has_file: true with file_name when file is present', async () => {
    mockExecuteScript.mock.mockImplementation(() =>
      Promise.resolve([{ result: { has_file: true, file_name: 'test.txt' } }])
    )
    const result = await verifyFileOnInput(1, '#file-input')
    assert.strictEqual(result.has_file, true)
    assert.strictEqual(result.file_name, 'test.txt')
  })
})
// ============================================
// clickFileInput tests
// ============================================
describe('clickFileInput', () => {
  beforeEach(() => {
    mockExecuteScript.mock.resetCalls()
  })
  test('returns clicked: true for valid file input', async () => {
    mockExecuteScript.mock.mockImplementation(() => Promise.resolve([{ result: { clicked: true } }]))
    const result = await clickFileInput(1, '#file-input')
    assert.strictEqual(result.clicked, true)
  })
  test('returns clicked: false with error for non-file-input element', async () => {
    mockExecuteScript.mock.mockImplementation(() =>
      Promise.resolve([{ result: { clicked: false, error: 'not_file_input' } }])
    )
    const result = await clickFileInput(1, '#not-a-file')
    assert.strictEqual(result.clicked, false)
    assert.strictEqual(result.error, 'not_file_input')
  })
})
// ============================================
// escalateToStage4 tests
// ============================================
describe('escalateToStage4', () => {
  beforeEach(() => {
    mockExecuteScript.mock.resetCalls()
    mockFetch.mock.resetCalls()
  })
  test('calls /api/os-automation/inject with browser_pid: 0', async () => {
    // Mock: click succeeds
    mockExecuteScript.mock.mockImplementation(() => Promise.resolve([{ result: { clicked: true } }]))
    // Mock: daemon returns success
    mockFetch.mock.mockImplementation(() =>
      Promise.resolve({
        ok: true,
        status: 200,
        json: () => Promise.resolve({ success: true, stage: 4 })
      })
    )
    // We don't need to fully test the end-to-end flow here,
    // just that the daemon is called with browser_pid: 0
    await escalateToStage4(1, '#file-input', '/path/to/file.txt', 'http://localhost:3000')
    // Find the fetch call to os-automation
    const fetchCalls = mockFetch.mock.calls
    const osAutomationCall = fetchCalls.find(
      (call) => typeof call.arguments[0] === 'string' && call.arguments[0].includes('/api/os-automation/inject')
    )
    assert.ok(osAutomationCall, 'Should call /api/os-automation/inject')
    const body = JSON.parse(osAutomationCall.arguments[1].body)
    assert.strictEqual(body.browser_pid, 0, 'Should send browser_pid: 0 for auto-detection')
  })
  test('reports error with OS-specific message when daemon returns 403', async () => {
    // Mock: click succeeds
    mockExecuteScript.mock.mockImplementation(() => Promise.resolve([{ result: { clicked: true } }]))
    // Mock: daemon returns 403 (OS automation disabled)
    mockFetch.mock.mockImplementation(() =>
      Promise.resolve({
        ok: false,
        status: 403,
        json: () =>
          Promise.resolve({
            success: false,
            error: 'OS-level upload automation is disabled. Start server with --enable-os-upload-automation flag.'
          })
      })
    )
    const result = await escalateToStage4(1, '#file-input', '/path/to/file.txt', 'http://localhost:3000')
    assert.ok(result.error, 'Should return an error')
    assert.ok(
      result.error.includes('enable-os-upload-automation') || result.error.includes('disabled'),
      `Error should mention enable-os-upload-automation or disabled, got: ${result.error}`
    )
  })
})
//# sourceMappingURL=upload-handler.test.js.map
