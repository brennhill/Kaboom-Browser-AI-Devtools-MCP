// @ts-nocheck
/**
 * @fileoverview websocket-tracking.test.js — Unit tests for WebSocket tracking primitives.
 * Covers message size calculation, payload formatting, truncation, connection
 * tracker stats, adaptive sampling, schema detection, and capture mode state.
 */

import { describe, it, beforeEach } from 'node:test'
import assert from 'node:assert'

import {
  getSize,
  formatPayload,
  truncateWsMessage,
  createConnectionTracker,
  setWebSocketCaptureModeInternal,
  getWebSocketCaptureModeInternal,
  resetCaptureModeForTesting
} from '../../../extension/lib/net/websocket-tracking.js'

function recordSampledIncoming(tracker, data) {
  tracker.recordMessage('incoming', data)
  tracker.recordSampledMessage('incoming', data)
}

// =============================================================================
// getSize()
// =============================================================================

describe('getSize', () => {
  it('returns byte length for ASCII strings', () => {
    assert.strictEqual(getSize('hello'), 5)
  })

  it('returns UTF-8 byte length for multi-byte characters', () => {
    // U+00E9 (e-acute) is 2 bytes in UTF-8
    const size = getSize('\u00e9')
    assert.strictEqual(size, 2)
  })

  it('returns byteLength for ArrayBuffer', () => {
    const buf = new ArrayBuffer(64)
    assert.strictEqual(getSize(buf), 64)
  })

  it('returns size property for Blob-like objects', () => {
    assert.strictEqual(getSize({ size: 4096 }), 4096)
  })

  it('returns 0 for null', () => {
    assert.strictEqual(getSize(null), 0)
  })
})

// =============================================================================
// formatPayload()
// =============================================================================

describe('formatPayload', () => {
  it('returns string data unchanged', () => {
    const json = '{"type":"chat"}'
    assert.strictEqual(formatPayload(json), json)
  })

  it('returns non-JSON text unchanged', () => {
    assert.strictEqual(formatPayload('plain text'), 'plain text')
  })

  it('formats small ArrayBuffer as hex preview', () => {
    const buf = new ArrayBuffer(4)
    const view = new Uint8Array(buf)
    view[0] = 0xde
    view[1] = 0xad
    view[2] = 0xbe
    view[3] = 0xef

    const result = formatPayload(buf)
    assert.ok(result.startsWith('[Binary: 4B]'), `Expected binary prefix, got: ${result}`)
    assert.ok(result.includes('deadbeef'), `Expected hex content, got: ${result}`)
  })

  it('formats large ArrayBuffer with magic bytes only', () => {
    const buf = new ArrayBuffer(512)
    const view = new Uint8Array(buf)
    view[0] = 0x89
    view[1] = 0x50
    view[2] = 0x4e
    view[3] = 0x47

    const result = formatPayload(buf)
    assert.ok(result.startsWith('[Binary: 512B, magic:'), `Expected large binary format, got: ${result}`)
    assert.ok(result.includes('89504e47'), `Expected magic bytes, got: ${result}`)
  })

  it('formats Blob-like objects with size', () => {
    const blob = { size: 2048 }
    const result = formatPayload(blob)
    assert.ok(result.includes('Binary'), `Expected binary indicator, got: ${result}`)
    assert.ok(result.includes('2048'), `Expected size, got: ${result}`)
  })

  it('stringifies null', () => {
    assert.strictEqual(formatPayload(null), 'null')
  })
})

// =============================================================================
// truncateWsMessage()
// =============================================================================

describe('truncateWsMessage', () => {
  it('does not truncate messages within 4KB', () => {
    const msg = '{"short":"value"}'
    const result = truncateWsMessage(msg)
    assert.strictEqual(result.data, msg)
    assert.strictEqual(result.truncated, false)
  })

  it('truncates messages longer than 4KB', () => {
    const msg = 'x'.repeat(5000)
    const result = truncateWsMessage(msg)
    assert.strictEqual(result.data.length, 4096)
    assert.strictEqual(result.truncated, true)
  })

  it('handles exactly 4096-char messages without truncation', () => {
    const msg = 'a'.repeat(4096)
    const result = truncateWsMessage(msg)
    assert.strictEqual(result.data.length, 4096)
    assert.strictEqual(result.truncated, false)
  })
})

// =============================================================================
// Capture mode state
// =============================================================================

describe('Capture mode state', () => {
  beforeEach(() => {
    resetCaptureModeForTesting()
  })

  it('defaults to medium', () => {
    assert.strictEqual(getWebSocketCaptureModeInternal(), 'medium')
  })

  it('can be set and retrieved', () => {
    setWebSocketCaptureModeInternal('all')
    assert.strictEqual(getWebSocketCaptureModeInternal(), 'all')
  })

  it('resets to medium via resetCaptureModeForTesting', () => {
    setWebSocketCaptureModeInternal('low')
    resetCaptureModeForTesting()
    assert.strictEqual(getWebSocketCaptureModeInternal(), 'medium')
  })
})

// =============================================================================
// createConnectionTracker — stats
// =============================================================================

describe('ConnectionTracker stats', () => {
  it('initialises with zero counts', () => {
    const t = createConnectionTracker('c1', 'wss://example.com')
    assert.strictEqual(t.stats.incoming.count, 0)
    assert.strictEqual(t.stats.outgoing.count, 0)
    assert.strictEqual(t.stats.incoming.bytes, 0)
    assert.strictEqual(t.stats.outgoing.bytes, 0)
    assert.strictEqual(t.stats.incoming.lastPreview, null)
    assert.strictEqual(t.stats.incoming.lastAt, null)
  })

  it('tracks incoming message count and bytes', () => {
    const t = createConnectionTracker('c1', 'wss://example.com')
    t.recordMessage('incoming', '{"a":1}') // 7 chars
    t.recordMessage('incoming', '{"b":2}') // 7 chars
    assert.strictEqual(t.stats.incoming.count, 2)
    assert.strictEqual(t.stats.incoming.bytes, 14)
  })

  it('tracks outgoing message count and bytes', () => {
    const t = createConnectionTracker('c1', 'wss://example.com')
    t.recordMessage('outgoing', 'ping')
    assert.strictEqual(t.stats.outgoing.count, 1)
    assert.strictEqual(t.stats.outgoing.bytes, 4)
  })

  it('stores last preview truncated to 200 chars', () => {
    const t = createConnectionTracker('c1', 'wss://example.com')
    const long = 'x'.repeat(300)
    t.recordMessage('incoming', long)
    assert.strictEqual(t.stats.incoming.lastPreview.length, 200)
  })

  it('stores last preview unchanged when short', () => {
    const t = createConnectionTracker('c1', 'wss://example.com')
    t.recordMessage('incoming', 'short')
    assert.strictEqual(t.stats.incoming.lastPreview, 'short')
  })

  it('records lastAt timestamp', () => {
    const t = createConnectionTracker('c1', 'wss://example.com')
    const before = Date.now()
    t.recordMessage('incoming', 'msg')
    assert.ok(t.stats.incoming.lastAt >= before)
    assert.ok(t.stats.incoming.lastAt <= Date.now())
  })

  it('increments messageCount across directions', () => {
    const t = createConnectionTracker('c1', 'wss://example.com')
    t.recordMessage('incoming', 'a')
    t.recordMessage('outgoing', 'b')
    t.recordMessage('incoming', 'c')
    assert.strictEqual(t.messageCount, 3)
  })
})

// =============================================================================
// createConnectionTracker — adaptive sampling
// =============================================================================

describe('ConnectionTracker adaptive sampling', () => {
  beforeEach(() => {
    resetCaptureModeForTesting() // medium
  })

  it('always samples first 5 messages', () => {
    const t = createConnectionTracker('c1', 'wss://example.com')
    t.setMessageRate(100)
    for (let i = 0; i < 5; i++) {
      t.recordMessage('incoming', 'x')
      assert.strictEqual(t.shouldSample('incoming'), true, `message ${i + 1} should be sampled`)
    }
  })

  it('samples every message in all mode', () => {
    setWebSocketCaptureModeInternal('all')
    const t = createConnectionTracker('c1', 'wss://example.com')
    t.setMessageRate(500)
    // Skip past the first-5 guarantee
    for (let i = 0; i < 10; i++) t.recordMessage('incoming', 'x')

    let sampled = 0
    for (let i = 0; i < 100; i++) {
      if (t.shouldSample('incoming')) sampled++
    }
    assert.strictEqual(sampled, 100)
  })

  it('downsamples in medium mode at high rate', () => {
    const t = createConnectionTracker('c1', 'wss://example.com')
    t.setMessageRate(50)
    // Burn through first-5 guarantee
    for (let i = 0; i < 6; i++) t.recordMessage('incoming', 'x')

    let sampled = 0
    for (let i = 0; i < 50; i++) {
      if (t.shouldSample('incoming')) sampled++
    }
    // medium targets 5/s; with 50 msg/s should sample ~1 in 10 => ~5 out of 50
    assert.ok(sampled >= 2 && sampled <= 15, `Expected ~5 sampled, got ${sampled}`)
  })

  it('shouldLogLifecycle always returns true', () => {
    const t = createConnectionTracker('c1', 'wss://example.com')
    assert.strictEqual(t.shouldLogLifecycle(), true)
  })

  it('getSamplingInfo returns rate, logged, and window', () => {
    const t = createConnectionTracker('c1', 'wss://example.com')
    t.setMessageRate(50)
    const info = t.getSamplingInfo()
    assert.strictEqual(info.rate, '50/s')
    assert.strictEqual(info.window, '5s')
    assert.ok(info.logged.length > 0, 'Expected logged ratio')
  })

  it('getMessageRate returns 0-ish for no messages', () => {
    const t = createConnectionTracker('c1', 'wss://example.com')
    const rate = t.getMessageRate()
    assert.strictEqual(rate, 0)
  })

  it('getMessageRate reflects recorded messages', () => {
    const t = createConnectionTracker('c1', 'wss://example.com')
    for (let i = 0; i < 10; i++) t.recordMessage('incoming', 'x')
    assert.ok(t.getMessageRate() > 0, 'Expected positive rate after messages')
  })

  it('keeps sustained frame accounting amortized O(1) and within the 0.1ms budget', () => {
    const t = createConnectionTracker('c1', 'wss://example.com')
    const originalNow = Date.now
    let now = 1_000_000
    Date.now = () => now++
    try {
      const started = process.hrtime.bigint()
      for (let i = 0; i < 100_000; i++) t.recordMessage('incoming', 'x')
      const elapsedMs = Number(process.hrtime.bigint() - started) / 1_000_000
      assert.ok(elapsedMs / 100_000 < 0.1, `Expected <0.1ms/frame, got ${elapsedMs / 100_000}ms`)
      assert.ok(t._messageTimestamps.length < 11_000, 'expired timestamps should be compacted instead of growing forever')
    } finally {
      Date.now = originalNow
    }
  })
})

// =============================================================================
// createConnectionTracker — schema detection
// =============================================================================

describe('ConnectionTracker schema detection', () => {
  it('returns null keys when no JSON messages received', () => {
    const t = createConnectionTracker('c1', 'wss://example.com')
    const schema = t.getSchema()
    assert.strictEqual(schema.detectedKeys, null)
    assert.strictEqual(schema.consistent, true)
  })

  it('detects consistent schema from 5 identical-shape messages', () => {
    const t = createConnectionTracker('c1', 'wss://example.com')
    for (let i = 0; i < 5; i++) {
      recordSampledIncoming(t, JSON.stringify({ sym: 'AAPL', price: 100 + i }))
    }
    const schema = t.getSchema()
    assert.deepStrictEqual(schema.detectedKeys, ['price', 'sym'])
    assert.strictEqual(schema.consistent, true)
  })

  it('does not parse schema until a frame has passed sampling', () => {
    const t = createConnectionTracker('c1', 'wss://example.com')
    t.recordMessage('incoming', '{"expensive":true}')
    assert.strictEqual(t.getSchema().detectedKeys, null)
    t.recordSampledMessage('incoming', '{"expensive":true}')
    assert.deepStrictEqual(t.getSchema().detectedKeys, ['expensive'])
  })

  it('detects inconsistent schema from mixed-shape messages', () => {
    const t = createConnectionTracker('c1', 'wss://example.com')
    recordSampledIncoming(t, JSON.stringify({ type: 'msg', text: 'hi' }))
    recordSampledIncoming(t, JSON.stringify({ type: 'err', code: 500 }))
    const schema = t.getSchema()
    assert.strictEqual(schema.consistent, false)
  })

  it('ignores outgoing messages for schema detection', () => {
    const t = createConnectionTracker('c1', 'wss://example.com')
    for (let i = 0; i < 5; i++) {
      t.recordMessage('outgoing', JSON.stringify({ action: 'ping' }))
    }
    const schema = t.getSchema()
    assert.strictEqual(schema.detectedKeys, null)
  })

  it('ignores non-JSON incoming messages', () => {
    const t = createConnectionTracker('c1', 'wss://example.com')
    for (let i = 0; i < 5; i++) {
      t.recordMessage('incoming', 'plain text ' + i)
    }
    assert.strictEqual(t.getSchema().detectedKeys, null)
  })

  it('ignores JSON array messages', () => {
    const t = createConnectionTracker('c1', 'wss://example.com')
    for (let i = 0; i < 5; i++) {
      t.recordMessage('incoming', JSON.stringify([1, 2, 3]))
    }
    assert.strictEqual(t.getSchema().detectedKeys, null)
  })

  it('tracks variants after bootstrap phase', () => {
    const t = createConnectionTracker('c1', 'wss://example.com')
    // Bootstrap with consistent messages
    for (let i = 0; i < 5; i++) {
      recordSampledIncoming(t, JSON.stringify({ type: 'msg', text: 'hi' }))
    }
    // Post-bootstrap variant
    for (let i = 0; i < 3; i++) {
      recordSampledIncoming(t, JSON.stringify({ type: 'err', code: 500 }))
    }
    const schema = t.getSchema()
    assert.ok(schema.variants, 'Expected variants to be tracked')
    assert.ok(schema.variants.length >= 2, `Expected at least 2 variants, got ${schema.variants.length}`)
  })

  it('isSchemaChange returns false before schema detected', () => {
    const t = createConnectionTracker('c1', 'wss://example.com')
    assert.strictEqual(t.isSchemaChange('{"new":"keys"}'), false)
  })

  it('isSchemaChange returns true for new key set after detection', () => {
    const t = createConnectionTracker('c1', 'wss://example.com')
    for (let i = 0; i < 5; i++) {
      recordSampledIncoming(t, JSON.stringify({ sym: 'AAPL', price: 100 }))
    }
    assert.strictEqual(t.isSchemaChange(JSON.stringify({ error: 'rate_limit', code: 429 })), true)
  })

  it('isSchemaChange returns false for known key set', () => {
    const t = createConnectionTracker('c1', 'wss://example.com')
    for (let i = 0; i < 5; i++) {
      recordSampledIncoming(t, JSON.stringify({ sym: 'AAPL', price: 100 }))
    }
    assert.strictEqual(t.isSchemaChange(JSON.stringify({ sym: 'GOOG', price: 200 })), false)
  })

  it('isSchemaChange returns false for non-JSON data', () => {
    const t = createConnectionTracker('c1', 'wss://example.com')
    for (let i = 0; i < 5; i++) {
      recordSampledIncoming(t, JSON.stringify({ sym: 'AAPL', price: 100 }))
    }
    assert.strictEqual(t.isSchemaChange('not json'), false)
  })

  it('isSchemaChange returns false for null', () => {
    const t = createConnectionTracker('c1', 'wss://example.com')
    for (let i = 0; i < 5; i++) {
      recordSampledIncoming(t, JSON.stringify({ sym: 'AAPL', price: 100 }))
    }
    assert.strictEqual(t.isSchemaChange(null), false)
  })
})
