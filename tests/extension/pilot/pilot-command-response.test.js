// @ts-nocheck
/**
 * Purpose: Proves pilot commands never turn missing content-script responses into success.
 */
import { beforeEach, describe, mock, test } from 'node:test'
import assert from 'node:assert'

const sendMessage = mock.fn()
const diagnosticLog = mock.fn()

globalThis.chrome = {
  runtime: { getManifest: () => ({ version: '0.9.0' }) },
  tabs: {
    query: mock.fn(async () => [{ id: 17, url: 'https://example.test/' }]),
    sendMessage
  },
  scripting: { executeScript: mock.fn() }
}
globalThis.__KABOOM_DEBUG_LOG__ = diagnosticLog

const { handlePilotCommand } = await import('../../../extension/background/commands/interact.js')
const { resetPilotCacheForTesting } = await import('../../../extension/background/runtime-state/pilot-state.js')

describe('pilot command response contract', () => {
  beforeEach(() => {
    mock.reset()
    diagnosticLog.mock.resetCalls()
    globalThis.__KABOOM_DEBUG_LOG__ = diagnosticLog
    resetPilotCacheForTesting(true)
  })

  for (const missingResponse of [undefined, false]) {
    test(`rejects ${String(missingResponse)} instead of reporting success`, async () => {
      sendMessage.mock.mockImplementationOnce(async () => missingResponse)

      const result = await handlePilotCommand('kaboom_manage_state', { action: 'capture' }, 17)

      assert.deepStrictEqual(result, {
        success: false,
        error: 'pilot_command_no_response',
        message: 'The content script returned no terminal response for kaboom_manage_state.'
      })
      assert.strictEqual(diagnosticLog.mock.calls.length, 1)
      assert.deepStrictEqual(diagnosticLog.mock.calls[0].arguments, [
        'error',
        'Pilot command returned no terminal response',
        { command: 'kaboom_manage_state', tab_id: 17, response_type: String(missingResponse) }
      ])
    })
  }

  test('preserves a valid typed response exactly', async () => {
    const response = { success: true, state: { local_storage: {} } }
    sendMessage.mock.mockImplementationOnce(async () => response)

    const result = await handlePilotCommand('kaboom_manage_state', { action: 'capture' }, 17)

    assert.strictEqual(result, response)
    assert.strictEqual(diagnosticLog.mock.calls.length, 0)
  })
})
