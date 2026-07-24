// @ts-nocheck
/**
 * @fileoverview brand-metadata.test.js — Verifies shared Kaboom URLs and user-facing helper copy.
 */

import { describe, test } from 'node:test'
import assert from 'node:assert'

describe('brand metadata helpers', () => {
  test('exports Kaboom repo/docs URLs and shared daemon guidance', async () => {
    const brand = await import('../../extension/lib/brand.js')

    assert.strictEqual(brand.KABOOM_DOCS_URL, 'https://gokaboom.dev/docs')
    assert.strictEqual(brand.KABOOM_REPOSITORY_URL, 'https://github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP')
    assert.strictEqual(brand.KABOOM_LOG_PREFIX, '[KaBOOM!]')
    assert.strictEqual(brand.KABOOM_RECORDING_LOG_PREFIX, '[KaBOOM! REC]')
    assert.strictEqual(brand.getTrackedTabLostToastDetail(), 'Re-enable in KaBOOM! popup')
    assert.match(brand.getDaemonStartHint(), /KaBOOM! daemon running/)
    assert.match(brand.getDaemonStartHint(), /npx kaboom-agentic-browser/)
    assert.doesNotMatch(brand.getDaemonStartHint(), /Gasoline daemon|STRUM daemon|gasoline-agentic-browser|strum-agentic-browser/)
  })
})
