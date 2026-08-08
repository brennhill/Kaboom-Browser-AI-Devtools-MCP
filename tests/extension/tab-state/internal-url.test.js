// internal-url.test.js — Canonical internal/restricted page predicates.
// Docs: docs/features/feature/tab-tracking-ux/index.md
import { describe, test } from 'node:test'
import assert from 'node:assert'

const { isInternalUrl, isInternalTab } = await import('../../../extension/lib/tabs/internal-url.js')

describe('isInternalUrl', () => {
  test('classifies browser-internal schemes as internal', () => {
    for (const url of [
      'chrome://extensions',
      'chrome-extension://abc/panel.html',
      'about:blank',
      'edge://settings',
      'brave://settings',
      'devtools://devtools/bundled/inspector.html'
    ]) {
      assert.strictEqual(isInternalUrl(url), true, `${url} must be internal`)
    }
  })

  test('classifies web pages as scriptable and a missing URL as internal', () => {
    assert.strictEqual(isInternalUrl('https://app.example.com/fixture'), false)
    assert.strictEqual(isInternalUrl('http://127.0.0.1:7890/tests/interact.html'), false)
    assert.strictEqual(isInternalUrl(undefined), true)
    assert.strictEqual(isInternalUrl(''), true)
  })
})

describe('isInternalTab', () => {
  test('treats a committed web page with no in-flight navigation as scriptable', () => {
    assert.strictEqual(isInternalTab({ url: 'https://app.example.com/fixture' }), false)
    assert.strictEqual(isInternalTab({ url: 'https://app.example.com/fixture', pendingUrl: '' }), false)
    assert.strictEqual(
      isInternalTab({ url: 'https://app.example.com/fixture', pendingUrl: 'https://app.example.com/next' }),
      false
    )
  })

  test('treats a web page navigating to an internal page as internal', () => {
    // The navigation race behind the performance-trace failure: Chrome still
    // reports the previous document in `url` while the restricted destination
    // is only visible through `pendingUrl`.
    assert.strictEqual(
      isInternalTab({
        url: 'https://app.example.com/fixture',
        pendingUrl: 'chrome-extension://other-extension/panel.html'
      }),
      true
    )
    assert.strictEqual(isInternalTab({ url: 'https://app.example.com/fixture', pendingUrl: 'chrome://settings' }), true)
  })

  test('treats a committed internal page as internal regardless of destination', () => {
    assert.strictEqual(isInternalTab({ url: 'chrome://extensions', pendingUrl: 'https://app.example.com' }), true)
  })

  test('fails closed for a missing tab or missing URL', () => {
    assert.strictEqual(isInternalTab(null), true)
    assert.strictEqual(isInternalTab(undefined), true)
    assert.strictEqual(isInternalTab({}), true)
  })
})
