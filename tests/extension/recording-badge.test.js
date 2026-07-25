// @ts-nocheck
import { beforeEach, describe, mock, test } from 'node:test'
import assert from 'node:assert'

import { startRecordingBadgeTimer, stopRecordingBadgeTimer } from '../../extension/background/recording/badge.js'

function createChromeMock() {
  return {
    action: {
      setBadgeText: mock.fn(),
      setBadgeBackgroundColor: mock.fn()
    }
  }
}

describe('recording badge lifecycle', () => {
  beforeEach(() => {
    mock.reset()
    globalThis.chrome = createChromeMock()
    stopRecordingBadgeTimer()
  })

  test('startRecordingBadgeTimer sets background and immediate elapsed text', () => {
    startRecordingBadgeTimer(Date.now() - 65_000)

    assert.strictEqual(globalThis.chrome.action.setBadgeBackgroundColor.mock.calls.length, 1)
    assert.deepStrictEqual(globalThis.chrome.action.setBadgeBackgroundColor.mock.calls[0].arguments[0], {
      color: '#dc2626'
    })

    const textCalls = globalThis.chrome.action.setBadgeText.mock.calls
    assert.ok(textCalls.length >= 1)
    const latestText = textCalls[textCalls.length - 1].arguments[0].text
    assert.strictEqual(typeof latestText, 'string')
    assert.ok(latestText.length > 0)
  })

  test('startRecordingBadgeTimer supports short second-format labels', () => {
    startRecordingBadgeTimer(Date.now() - 5_000)
    const textCalls = globalThis.chrome.action.setBadgeText.mock.calls
    const latestText = textCalls[textCalls.length - 1].arguments[0].text
    assert.ok(latestText.endsWith('s'))
  })

  test('startRecordingBadgeTimer gracefully handles zero epoch input', () => {
    startRecordingBadgeTimer(0)
    const textCalls = globalThis.chrome.action.setBadgeText.mock.calls
    if (textCalls.length > 0) {
      const latestText = textCalls[textCalls.length - 1].arguments[0].text
      assert.strictEqual(typeof latestText, 'string')
    }
    stopRecordingBadgeTimer()
  })

  test('stopRecordingBadgeTimer clears badge text', () => {
    startRecordingBadgeTimer(Date.now() - 5_000)
    stopRecordingBadgeTimer()
    const lastCall = globalThis.chrome.action.setBadgeText.mock.calls.at(-1)
    assert.deepStrictEqual(lastCall.arguments[0], { text: '' })
  })

  test('badge timer is best-effort when chrome.action methods throw', () => {
    globalThis.chrome.action.setBadgeBackgroundColor = mock.fn(() => {
      throw new Error('bg failure')
    })
    globalThis.chrome.action.setBadgeText = mock.fn(() => {
      throw new Error('text failure')
    })

    assert.doesNotThrow(() => startRecordingBadgeTimer(Date.now()))
    assert.doesNotThrow(() => stopRecordingBadgeTimer())
  })

  test('no-op when action API is unavailable', () => {
    globalThis.chrome = {}
    assert.doesNotThrow(() => startRecordingBadgeTimer(Date.now()))
    assert.doesNotThrow(() => stopRecordingBadgeTimer())
  })
})
