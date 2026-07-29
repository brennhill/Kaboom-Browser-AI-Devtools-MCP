// @ts-nocheck
/**
 * @fileoverview request-audit.test.js — Tests for the shared popup/hover audit trigger helper.
 */

import { beforeEach, describe, mock, test } from 'node:test'
import assert from 'node:assert'

describe('requestAudit', () => {
  beforeEach(() => {
    mock.reset()
    globalThis.chrome = {
      runtime: {
        sendMessage: mock.fn((message) => Promise.resolve({ success: true, type: message?.type }))
      }
    }
  })

  test('opens the terminal panel before requesting the audit bridge', async () => {
    const { requestAudit } = await import('../../../extension/lib/tabs/request-audit.js')

    await requestAudit('https://tracked.example/')

    const sentMessages = chrome.runtime.sendMessage.mock.calls.map((call) => call.arguments[0])
    assert.deepStrictEqual(
      sentMessages.map((message) => message.type),
      ['open_terminal_panel', 'qa_scan_requested']
    )
    assert.strictEqual(sentMessages[1].page_url, 'https://tracked.example/')
  })
})
