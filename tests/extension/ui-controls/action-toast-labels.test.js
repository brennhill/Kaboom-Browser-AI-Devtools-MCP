// @ts-nocheck
import { beforeEach, describe, mock, test } from 'node:test'
import assert from 'node:assert'

describe('actionToast label normalization', () => {
  beforeEach(() => {
    globalThis.chrome = {
      tabs: {
        sendMessage: mock.fn(() => Promise.resolve())
      }
    }
  })

  test('maps wait_for_stable to reader-friendly copy', async () => {
    const { actionToast } = await import('../../../extension/background/commands/helpers.js')
    actionToast(42, 'wait_for_stable', undefined, 'trying')

    assert.strictEqual(globalThis.chrome.tabs.sendMessage.mock.calls.length, 1)
    const [, message] = globalThis.chrome.tabs.sendMessage.mock.calls[0].arguments
    assert.strictEqual(message.type, 'kaboom_action_toast')
    assert.strictEqual(message.text, 'Waiting for page to stabilize...')
  })

  test('humanizes unmapped snake_case action labels', async () => {
    const { actionToast } = await import('../../../extension/background/commands/helpers.js')
    actionToast(42, 'switch_tab')

    assert.strictEqual(globalThis.chrome.tabs.sendMessage.mock.calls.length, 1)
    const [, message] = globalThis.chrome.tabs.sendMessage.mock.calls[0].arguments
    assert.strictEqual(message.text, 'Switch tab')
  })

  test('uses progressive label for scroll_to in trying state', async () => {
    const { actionToast } = await import('../../../extension/background/commands/helpers.js')
    actionToast(42, 'scroll_to', '#checkout-button', 'trying')

    assert.strictEqual(globalThis.chrome.tabs.sendMessage.mock.calls.length, 1)
    const [, message] = globalThis.chrome.tabs.sendMessage.mock.calls[0].arguments
    assert.strictEqual(message.text, 'Scrolling to')
    assert.strictEqual(message.detail, '#checkout-button')
  })

  test('infers wait_for target from detail in trying state', async () => {
    const { actionToast } = await import('../../../extension/background/commands/helpers.js')
    actionToast(42, 'wait_for', '#result-panel', 'trying')

    assert.strictEqual(globalThis.chrome.tabs.sendMessage.mock.calls.length, 1)
    const [, message] = globalThis.chrome.tabs.sendMessage.mock.calls[0].arguments
    assert.strictEqual(message.text, 'Waiting for #result-panel')
    assert.strictEqual(message.detail, undefined)
  })

  test('falls back to generic waiting copy when wait_for has no target detail', async () => {
    const { actionToast } = await import('../../../extension/background/commands/helpers.js')
    actionToast(42, 'wait_for', 'page', 'trying')

    assert.strictEqual(globalThis.chrome.tabs.sendMessage.mock.calls.length, 1)
    const [, message] = globalThis.chrome.tabs.sendMessage.mock.calls[0].arguments
    assert.strictEqual(message.text, 'Waiting for condition...')
    assert.strictEqual(message.detail, undefined)
  })

  test('preserves non-enum labels as-is', async () => {
    const { actionToast } = await import('../../../extension/background/commands/helpers.js')
    actionToast(42, 'Navigate to docs')

    assert.strictEqual(globalThis.chrome.tabs.sendMessage.mock.calls.length, 1)
    const [, message] = globalThis.chrome.tabs.sendMessage.mock.calls[0].arguments
    assert.strictEqual(message.text, 'Navigate to docs')
  })
})
