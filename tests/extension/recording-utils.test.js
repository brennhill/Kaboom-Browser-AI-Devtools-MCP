// @ts-nocheck
import { describe, test } from 'node:test'
import assert from 'node:assert'

import { buildScreenRecordingSlug, buildRecordingToastLabel } from '../../extension/background/recording/utils.js'

describe('recording url helpers', () => {
  test('buildScreenRecordingSlug normalizes hostnames', () => {
    assert.strictEqual(buildScreenRecordingSlug('https://www.Example-Site.com/path?q=1'), 'example-site-com')
    assert.strictEqual(buildScreenRecordingSlug('https://localhost:3000/abc'), 'localhost')
  })

  test('buildScreenRecordingSlug falls back on invalid urls', () => {
    assert.strictEqual(buildScreenRecordingSlug('not-a-url'), 'recording')
    assert.strictEqual(buildScreenRecordingSlug(undefined), 'recording')
    assert.strictEqual(buildScreenRecordingSlug('https://___/x'), 'recording')
  })

  test('buildRecordingToastLabel produces clipped host+path label', () => {
    assert.strictEqual(buildRecordingToastLabel('https://www.docs.example.com/'), 'Recording docs.example.com')
    assert.strictEqual(
      buildRecordingToastLabel('https://example.com/very/long/path/that/keeps/going/and/going/for/a/while'),
      'Recording example.com/very/long/path/that/keeps/g...'
    )
  })

  test('buildRecordingToastLabel falls back on invalid urls', () => {
    assert.strictEqual(buildRecordingToastLabel('::bad::url'), 'Recording started')
    assert.strictEqual(buildRecordingToastLabel(undefined), 'Recording started')
  })
})
