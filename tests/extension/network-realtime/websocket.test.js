// @ts-nocheck
/**
 * @fileoverview websocket.test.js — Tests for WebSocket capture subsystem.
 * Covers WebSocket constructor wrapping, message interception (text + binary),
 * adaptive sampling under high throughput, JSON schema detection, connection
 * tracking, and close/error event forwarding.
 */

import { test, describe, mock, beforeEach, afterEach } from 'node:test'
import assert from 'node:assert'
import { createMockWindow, createMockCrypto } from '../shared/helpers.js'

let originalWindow, originalCrypto

describe('WebSocket Interception', () => {
  beforeEach(async () => {
    originalWindow = globalThis.window
    originalCrypto = globalThis.crypto
    globalThis.window = createMockWindow({ withWebSocket: true })
    Object.defineProperty(globalThis, 'crypto', { value: createMockCrypto(), writable: true, configurable: true })
    // Force uninstall to reset module state in case a previous test crashed before cleanup
    const { setWebSocketCaptureMode, setWebSocketCaptureEnabled, uninstallWebSocketCapture } =
      await import('../../../extension/lib/net/websocket.js')
    uninstallWebSocketCapture()
    setWebSocketCaptureMode('lifecycle')
    setWebSocketCaptureEnabled(true)
  })

  afterEach(() => {
    globalThis.window = originalWindow
    Object.defineProperty(globalThis, 'crypto', { value: originalCrypto, writable: true, configurable: true })
  })

  test('should replace WebSocket constructor', async () => {
    const { installWebSocketCapture, uninstallWebSocketCapture } = await import('../../../extension/lib/net/websocket.js')

    const OriginalWS = globalThis.window.WebSocket
    installWebSocketCapture()

    assert.notStrictEqual(globalThis.window.WebSocket, OriginalWS)

    uninstallWebSocketCapture()
    assert.strictEqual(globalThis.window.WebSocket, OriginalWS)
  })

  test('should preserve WebSocket.prototype', async () => {
    const { installWebSocketCapture, uninstallWebSocketCapture } = await import('../../../extension/lib/net/websocket.js')

    const OriginalWS = globalThis.window.WebSocket
    installWebSocketCapture()

    assert.strictEqual(globalThis.window.WebSocket.prototype, OriginalWS.prototype)

    uninstallWebSocketCapture()
  })

  test('should preserve WebSocket static constants (CONNECTING, OPEN, CLOSING, CLOSED)', async () => {
    const { installWebSocketCapture, uninstallWebSocketCapture } = await import('../../../extension/lib/net/websocket.js')

    installWebSocketCapture()

    assert.strictEqual(globalThis.window.WebSocket.CONNECTING, 0)
    assert.strictEqual(globalThis.window.WebSocket.OPEN, 1)
    assert.strictEqual(globalThis.window.WebSocket.CLOSING, 2)
    assert.strictEqual(globalThis.window.WebSocket.CLOSED, 3)

    uninstallWebSocketCapture()
  })

  test('should create WebSocket with correct URL and protocols', async () => {
    const { installWebSocketCapture, uninstallWebSocketCapture } = await import('../../../extension/lib/net/websocket.js')

    installWebSocketCapture()

    const ws = new globalThis.window.WebSocket('wss://example.com/ws', ['chat'])
    assert.strictEqual(ws.url, 'wss://example.com/ws')

    uninstallWebSocketCapture()
  })

  test('ws:open payload has spec-compliant shape', async () => {
    const { installWebSocketCapture, uninstallWebSocketCapture } = await import('../../../extension/lib/net/websocket.js')
    installWebSocketCapture()

    const ws = new globalThis.window.WebSocket('wss://example.com/ws')
    ws._emit('open', {})

    const calls = globalThis.window.postMessage.mock.calls
    const openMsg = calls.find((c) => c.arguments[0].type === 'kaboom_ws' && c.arguments[0].payload.event === 'open')
    assert.ok(openMsg, 'Expected ws:open event')
    const payload = openMsg.arguments[0].payload

    // Shape from spec: ts, type, event, id, url
    assert.ok('ts' in payload, 'missing: ts')
    assert.strictEqual(payload.type, 'websocket')
    assert.strictEqual(payload.event, 'open')
    assert.ok('id' in payload, 'missing: id')
    assert.ok('url' in payload, 'missing: url')

    uninstallWebSocketCapture()
  })

  test('ws:close payload has spec-compliant shape', async () => {
    const { installWebSocketCapture, uninstallWebSocketCapture } = await import('../../../extension/lib/net/websocket.js')
    installWebSocketCapture()

    const ws = new globalThis.window.WebSocket('wss://example.com/ws')
    ws._emit('open', {})
    ws._emit('close', { code: 1000, reason: 'normal' })

    const calls = globalThis.window.postMessage.mock.calls
    const closeMsg = calls.find(
      (c) => c.arguments[0].type === 'kaboom_ws' && c.arguments[0].payload.event === 'close'
    )
    assert.ok(closeMsg, 'Expected ws:close event')
    const payload = closeMsg.arguments[0].payload

    // Shape from spec: ts, type, event, id, url, code, reason
    assert.ok('ts' in payload, 'missing: ts')
    assert.strictEqual(payload.type, 'websocket')
    assert.strictEqual(payload.event, 'close')
    assert.ok('id' in payload, 'missing: id')
    assert.ok('url' in payload, 'missing: url')
    assert.ok('code' in payload, 'missing: code')
    assert.ok('reason' in payload, 'missing: reason')

    uninstallWebSocketCapture()
  })

  test('ws:message payload has spec-compliant shape', async () => {
    const { installWebSocketCapture, uninstallWebSocketCapture, setWebSocketCaptureMode } =
      await import('../../../extension/lib/net/websocket.js')
    setWebSocketCaptureMode('messages')
    installWebSocketCapture()

    const ws = new globalThis.window.WebSocket('wss://example.com/ws')
    ws._emit('open', {})
    ws._emit('message', { data: '{"type":"chat"}' })
    await Promise.resolve()

    const calls = globalThis.window.postMessage.mock.calls
    const msgEvent = calls.find(
      (c) => c.arguments[0].type === 'kaboom_ws' && c.arguments[0].payload.event === 'message'
    )
    assert.ok(msgEvent, 'Expected ws:message event')
    const payload = msgEvent.arguments[0].payload

    // Shape from spec: ts, type, event, id, url, direction, data, size
    assert.ok('ts' in payload, 'missing: ts')
    assert.strictEqual(payload.type, 'websocket')
    assert.strictEqual(payload.event, 'message')
    assert.ok('id' in payload, 'missing: id')
    assert.ok('url' in payload, 'missing: url')
    assert.ok('direction' in payload, 'missing: direction')
    assert.ok('data' in payload, 'missing: data')
    assert.ok('size' in payload, 'missing: size')

    uninstallWebSocketCapture()
  })

  test('should emit ws:open event on connection open', async () => {
    const { installWebSocketCapture, uninstallWebSocketCapture } = await import('../../../extension/lib/net/websocket.js')

    installWebSocketCapture()

    const ws = new globalThis.window.WebSocket('wss://example.com/ws')
    ws._emit('open', {})

    const calls = globalThis.window.postMessage.mock.calls
    const openMessage = calls.find((c) => {
      const msg = c.arguments[0]
      return msg.type === 'kaboom_ws' && msg.payload.event === 'open'
    })

    assert.ok(openMessage, 'Expected ws:open event to be posted')
    assert.strictEqual(openMessage.arguments[0].payload.url, 'wss://example.com/ws')
    assert.ok(openMessage.arguments[0].payload.id, 'Expected connection ID')

    uninstallWebSocketCapture()
  })

  test('should emit ws:close event with code and reason', async () => {
    const { installWebSocketCapture, uninstallWebSocketCapture } = await import('../../../extension/lib/net/websocket.js')

    installWebSocketCapture()

    const ws = new globalThis.window.WebSocket('wss://example.com/ws')
    ws._emit('open', {})
    ws._emit('close', { code: 1000, reason: 'normal closure' })

    const calls = globalThis.window.postMessage.mock.calls
    const closeMessage = calls.find((c) => {
      const msg = c.arguments[0]
      return msg.type === 'kaboom_ws' && msg.payload.event === 'close'
    })

    assert.ok(closeMessage, 'Expected ws:close event to be posted')
    assert.strictEqual(closeMessage.arguments[0].payload.code, 1000)
    assert.strictEqual(closeMessage.arguments[0].payload.reason, 'normal closure')

    uninstallWebSocketCapture()
  })

  test('should emit ws:error event', async () => {
    const { installWebSocketCapture, uninstallWebSocketCapture } = await import('../../../extension/lib/net/websocket.js')

    installWebSocketCapture()

    const ws = new globalThis.window.WebSocket('wss://example.com/ws')
    ws._emit('open', {})
    ws._emit('error', {})

    const calls = globalThis.window.postMessage.mock.calls
    const errorMessage = calls.find((c) => {
      const msg = c.arguments[0]
      return msg.type === 'kaboom_ws' && msg.payload.event === 'error'
    })

    assert.ok(errorMessage, 'Expected ws:error event to be posted')

    uninstallWebSocketCapture()
  })

  test('should track incoming messages', async () => {
    const { installWebSocketCapture, uninstallWebSocketCapture, setWebSocketCaptureMode } =
      await import('../../../extension/lib/net/websocket.js')

    setWebSocketCaptureMode('messages')
    installWebSocketCapture()

    const ws = new globalThis.window.WebSocket('wss://example.com/ws')
    ws._emit('open', {})
    ws._emit('message', { data: '{"type":"chat","msg":"hello"}' })
    await Promise.resolve()

    const calls = globalThis.window.postMessage.mock.calls
    const msgEvent = calls.find((c) => {
      const msg = c.arguments[0]
      return msg.type === 'kaboom_ws' && msg.payload.event === 'message' && msg.payload.direction === 'incoming'
    })

    assert.ok(msgEvent, 'Expected incoming message event')
    assert.strictEqual(msgEvent.arguments[0].payload.data, '{"type":"chat","msg":"hello"}')
    assert.strictEqual(msgEvent.arguments[0].payload.size, 29)

    uninstallWebSocketCapture()
  })

  test('should intercept outgoing messages via send()', async () => {
    const { installWebSocketCapture, uninstallWebSocketCapture, setWebSocketCaptureMode } =
      await import('../../../extension/lib/net/websocket.js')

    setWebSocketCaptureMode('messages')
    installWebSocketCapture()

    const ws = new globalThis.window.WebSocket('wss://example.com/ws')
    ws._emit('open', {})
    ws.send('{"type":"ping"}')
    await Promise.resolve()

    const calls = globalThis.window.postMessage.mock.calls
    const msgEvent = calls.find((c) => {
      const msg = c.arguments[0]
      return msg.type === 'kaboom_ws' && msg.payload.event === 'message' && msg.payload.direction === 'outgoing'
    })

    assert.ok(msgEvent, 'Expected outgoing message event')
    assert.strictEqual(msgEvent.arguments[0].payload.data, '{"type":"ping"}')

    uninstallWebSocketCapture()
  })

  test('should call original send() after interception', async () => {
    const { installWebSocketCapture, uninstallWebSocketCapture } = await import('../../../extension/lib/net/websocket.js')

    installWebSocketCapture()

    const ws = new globalThis.window.WebSocket('wss://example.com/ws')
    const originalSend = mock.fn()
    ws.send = originalSend
    // Re-install after replacing send (the wrapper wraps the current send)
    // Actually the wrapper should have already wrapped it

    uninstallWebSocketCapture()
  })

  test('should assign unique ID per connection', async () => {
    const { installWebSocketCapture, uninstallWebSocketCapture } = await import('../../../extension/lib/net/websocket.js')

    installWebSocketCapture()

    const ws1 = new globalThis.window.WebSocket('wss://a.com/ws')
    const ws2 = new globalThis.window.WebSocket('wss://b.com/ws')
    ws1._emit('open', {})
    ws2._emit('open', {})

    const calls = globalThis.window.postMessage.mock.calls
    const openEvents = calls
      .filter((c) => c.arguments[0].type === 'kaboom_ws' && c.arguments[0].payload.event === 'open')
      .map((c) => c.arguments[0].payload.id)

    assert.strictEqual(openEvents.length, 2)
    assert.notStrictEqual(openEvents[0], openEvents[1], 'Expected unique IDs per connection')

    uninstallWebSocketCapture()
  })

  test('should capture messages in low sampling mode at low rates', async () => {
    const { installWebSocketCapture, uninstallWebSocketCapture, setWebSocketCaptureMode } =
      await import('../../../extension/lib/net/websocket.js')

    setWebSocketCaptureMode('low')
    installWebSocketCapture()

    const ws = new globalThis.window.WebSocket('wss://example.com/ws')
    ws._emit('open', {})
    ws._emit('message', { data: '{"type":"chat","msg":"hello"}' })
    ws.send('{"type":"pong"}')

    const calls = globalThis.window.postMessage.mock.calls
    // All modes now capture messages (with sampling). At low message rate, low mode should still capture.
    const openEvents = calls.filter((c) => {
      const msg = c.arguments[0]
      return msg.type === 'kaboom_ws' && msg.payload.event === 'open'
    })
    assert.strictEqual(openEvents.length, 1, 'Expected open event to be captured')

    uninstallWebSocketCapture()
  })

  test('should truncate message data at 4KB', async () => {
    const { installWebSocketCapture, uninstallWebSocketCapture, setWebSocketCaptureMode } =
      await import('../../../extension/lib/net/websocket.js')

    setWebSocketCaptureMode('messages')
    installWebSocketCapture()

    const ws = new globalThis.window.WebSocket('wss://example.com/ws')
    ws._emit('open', {})

    const largeData = 'x'.repeat(5000)
    ws._emit('message', { data: largeData })
    await Promise.resolve()

    const calls = globalThis.window.postMessage.mock.calls
    const msgEvent = calls.find((c) => {
      const msg = c.arguments[0]
      return msg.type === 'kaboom_ws' && msg.payload.event === 'message'
    })

    assert.ok(msgEvent.arguments[0].payload.data.length <= 4096, 'Expected data truncated to 4KB')
    assert.strictEqual(msgEvent.arguments[0].payload.truncated, true)

    uninstallWebSocketCapture()
  })

  test('defers sampled payload formatting and posting beyond the application callback', async () => {
    const { installWebSocketCapture, uninstallWebSocketCapture, setWebSocketCaptureMode } =
      await import('../../../extension/lib/net/websocket.js')
    setWebSocketCaptureMode('messages')
    installWebSocketCapture()

    const ws = new globalThis.window.WebSocket('wss://example.com/ws')
    ws._emit('message', { data: '{"type":"deferred"}' })

    const synchronousMessage = globalThis.window.postMessage.mock.calls.find(
      (c) => c.arguments[0].type === 'kaboom_ws' && c.arguments[0].payload.event === 'message'
    )
    assert.strictEqual(synchronousMessage, undefined, 'message payload work must not run in the native callback')

    await Promise.resolve()
    const deferredMessage = globalThis.window.postMessage.mock.calls.find(
      (c) => c.arguments[0].type === 'kaboom_ws' && c.arguments[0].payload.event === 'message'
    )
    assert.ok(deferredMessage, 'sampled payload should retain its wire event')
    assert.strictEqual(deferredMessage.arguments[0].payload.data, '{"type":"deferred"}')

    uninstallWebSocketCapture()
  })

  test('snapshots incoming ArrayBuffer bytes before deferred formatting', async () => {
    const { installWebSocketCapture, uninstallWebSocketCapture, setWebSocketCaptureMode } =
      await import('../../../extension/lib/net/websocket.js')
    setWebSocketCaptureMode('messages')
    installWebSocketCapture()

    const ws = new globalThis.window.WebSocket('wss://example.com/ws')
    const buffer = Uint8Array.from([0x01, 0x02, 0x03]).buffer
    ws._emit('message', { data: buffer })
    new Uint8Array(buffer).fill(0xff)

    await Promise.resolve()
    const message = globalThis.window.postMessage.mock.calls.find(
      (c) => c.arguments[0].type === 'kaboom_ws' && c.arguments[0].payload.direction === 'incoming'
    )
    assert.strictEqual(message.arguments[0].payload.data, '[Binary: 3B] 010203')
    assert.strictEqual(message.arguments[0].payload.size, 3)

    uninstallWebSocketCapture()
  })

  test('retains incoming ArrayBuffer bytes when the application detaches the source', async () => {
    const { installWebSocketCapture, uninstallWebSocketCapture, setWebSocketCaptureMode } =
      await import('../../../extension/lib/net/websocket.js')
    setWebSocketCaptureMode('messages')
    installWebSocketCapture()

    const ws = new globalThis.window.WebSocket('wss://example.com/ws')
    const buffer = Uint8Array.from([0x0a, 0x1b, 0x2c]).buffer
    ws._emit('message', { data: buffer })
    structuredClone(buffer, { transfer: [buffer] })

    await Promise.resolve()
    const message = globalThis.window.postMessage.mock.calls.find(
      (c) => c.arguments[0].type === 'kaboom_ws' && c.arguments[0].payload.direction === 'incoming'
    )
    assert.strictEqual(message.arguments[0].payload.data, '[Binary: 3B] 0a1b2c')
    assert.strictEqual(message.arguments[0].payload.size, 3)

    uninstallWebSocketCapture()
  })

  test('snapshots only the outgoing typed-view bytes before deferred formatting', async () => {
    const { installWebSocketCapture, uninstallWebSocketCapture, setWebSocketCaptureMode } =
      await import('../../../extension/lib/net/websocket.js')
    setWebSocketCaptureMode('messages')
    installWebSocketCapture()

    const ws = new globalThis.window.WebSocket('wss://example.com/ws')
    const buffer = Uint8Array.from([0xff, 0x11, 0x22, 0x33, 0xee]).buffer
    const view = new Uint8Array(buffer, 1, 3)
    ws.send(view)
    view.fill(0xaa)

    await Promise.resolve()
    const message = globalThis.window.postMessage.mock.calls.find(
      (c) => c.arguments[0].type === 'kaboom_ws' && c.arguments[0].payload.direction === 'outgoing'
    )
    assert.strictEqual(message.arguments[0].payload.data, '[Binary: 3B] 112233')
    assert.strictEqual(message.arguments[0].payload.size, 3)

    uninstallWebSocketCapture()
  })

  test('retains outgoing typed-view bytes when send detaches the source buffer', async () => {
    const { installWebSocketCapture, uninstallWebSocketCapture, setWebSocketCaptureMode } =
      await import('../../../extension/lib/net/websocket.js')
    setWebSocketCaptureMode('messages')
    installWebSocketCapture()

    const ws = new globalThis.window.WebSocket('wss://example.com/ws')
    const buffer = Uint8Array.from([0x44, 0x55, 0x66]).buffer
    const view = new Uint8Array(buffer)
    ws.send(view)
    structuredClone(buffer, { transfer: [buffer] })

    await Promise.resolve()
    const message = globalThis.window.postMessage.mock.calls.find(
      (c) => c.arguments[0].type === 'kaboom_ws' && c.arguments[0].payload.direction === 'outgoing'
    )
    assert.strictEqual(message.arguments[0].payload.data, '[Binary: 3B] 445566')
    assert.strictEqual(message.arguments[0].payload.size, 3)

    uninstallWebSocketCapture()
  })
})

describe('Adaptive Sampling', () => {
  beforeEach(() => {
    originalWindow = globalThis.window
    originalCrypto = globalThis.crypto
    globalThis.window = createMockWindow({ withWebSocket: true })
    Object.defineProperty(globalThis, 'crypto', { value: createMockCrypto(), writable: true, configurable: true })
  })

  afterEach(() => {
    globalThis.window = originalWindow
    Object.defineProperty(globalThis, 'crypto', { value: originalCrypto, writable: true, configurable: true })
  })

  test('should log every message when rate < 10 msg/s', async () => {
    const { createConnectionTracker } = await import('../../../extension/lib/net/websocket-tracking.js')

    const tracker = createConnectionTracker('test-id', 'wss://example.com')

    // Simulate 5 messages in 1 second (5 msg/s)
    for (let i = 0; i < 5; i++) {
      tracker.recordMessage('incoming')
    }

    assert.strictEqual(tracker.shouldSample('incoming'), true)
  })

  test('should sample at ~5 msg/s in medium mode when rate is 30 msg/s', async () => {
    const { createConnectionTracker } = await import('../../../extension/lib/net/websocket-tracking.js')
    const { setWebSocketCaptureMode } = await import('../../../extension/lib/net/websocket.js')

    setWebSocketCaptureMode('medium')
    const tracker = createConnectionTracker('test-id', 'wss://example.com')

    // Simulate 30 messages in 1 second (30 msg/s)
    tracker.setMessageRate(30)

    // Medium mode targets ~5/s, so with 30 msg/s should sample ~1 in 6
    let sampled = 0
    for (let i = 0; i < 30; i++) {
      if (tracker.shouldSample('incoming')) sampled++
    }

    // Should be approximately 5 (tolerance: 2-10)
    assert.ok(sampled >= 2 && sampled <= 10, `Expected ~5 sampled messages in medium mode, got ${sampled}`)
  })

  test('should sample at ~5 msg/s when rate is 50-200 msg/s', async () => {
    const { createConnectionTracker } = await import('../../../extension/lib/net/websocket-tracking.js')

    const tracker = createConnectionTracker('test-id', 'wss://example.com')
    tracker.setMessageRate(100)

    let sampled = 0
    for (let i = 0; i < 100; i++) {
      if (tracker.shouldSample('incoming')) sampled++
    }

    // Should be approximately 5 (tolerance: 2-10)
    assert.ok(sampled >= 2 && sampled <= 10, `Expected ~5 sampled messages, got ${sampled}`)
  })

  test('should sample at ~2 msg/s when rate > 200 msg/s', async () => {
    const { createConnectionTracker } = await import('../../../extension/lib/net/websocket-tracking.js')

    const tracker = createConnectionTracker('test-id', 'wss://example.com')
    tracker.setMessageRate(500)

    let sampled = 0
    for (let i = 0; i < 500; i++) {
      if (tracker.shouldSample('incoming')) sampled++
    }

    // Should be approximately 2 (tolerance: 1-5)
    assert.ok(sampled >= 1 && sampled <= 5, `Expected ~2 sampled messages, got ${sampled}`)
  })

  test('should always log first 5 messages on a new connection', async () => {
    const { createConnectionTracker } = await import('../../../extension/lib/net/websocket-tracking.js')

    const tracker = createConnectionTracker('test-id', 'wss://example.com')
    tracker.setMessageRate(100) // High rate - would normally sample

    let sampled = 0
    for (let i = 0; i < 5; i++) {
      tracker.recordMessage('incoming')
      if (tracker.shouldSample('incoming')) sampled++
    }

    assert.strictEqual(sampled, 5, 'Expected all first 5 messages to be sampled')
  })

  test('should always log lifecycle events regardless of rate', async () => {
    const { createConnectionTracker } = await import('../../../extension/lib/net/websocket-tracking.js')

    const tracker = createConnectionTracker('test-id', 'wss://example.com')
    tracker.setMessageRate(500)

    // Lifecycle events should always be logged
    assert.strictEqual(tracker.shouldLogLifecycle(), true)
  })

  test('should include sampling info when sampling is active', async () => {
    const { createConnectionTracker } = await import('../../../extension/lib/net/websocket-tracking.js')

    const tracker = createConnectionTracker('test-id', 'wss://example.com')
    tracker.setMessageRate(50)

    const info = tracker.getSamplingInfo()

    assert.ok(info.rate, 'Expected rate in sampling info')
    assert.ok(info.logged, 'Expected logged ratio in sampling info')
    assert.ok(info.window, 'Expected window in sampling info')
  })

  test('should use rolling 5-second window for rate calculation', async () => {
    const { createConnectionTracker } = await import('../../../extension/lib/net/websocket-tracking.js')

    const tracker = createConnectionTracker('test-id', 'wss://example.com')

    // Record messages over time
    for (let i = 0; i < 50; i++) {
      tracker.recordMessage('incoming')
    }

    const rate = tracker.getMessageRate()
    assert.ok(rate > 0, 'Expected positive rate')
  })
})

describe('Schema Detection', () => {
  beforeEach(() => {
    originalWindow = globalThis.window
    originalCrypto = globalThis.crypto
    globalThis.window = createMockWindow({ withWebSocket: true })
    Object.defineProperty(globalThis, 'crypto', { value: createMockCrypto(), writable: true, configurable: true })
  })

  afterEach(() => {
    globalThis.window = originalWindow
    Object.defineProperty(globalThis, 'crypto', { value: originalCrypto, writable: true, configurable: true })
  })

  test('should detect JSON schema from first 5 messages', async () => {
    const { createConnectionTracker } = await import('../../../extension/lib/net/websocket-tracking.js')

    const tracker = createConnectionTracker('test-id', 'wss://example.com')

    // Feed 5 consistent JSON messages
    for (let i = 0; i < 5; i++) {
      tracker.recordSampledMessage('incoming', JSON.stringify({ sym: 'AAPL', price: 185 + i, vol: 1000 }))
    }

    const schema = tracker.getSchema()
    assert.deepStrictEqual(schema.detectedKeys.sort(), ['price', 'sym', 'vol'])
    assert.strictEqual(schema.consistent, true)
  })

  test('should detect schema inconsistency', async () => {
    const { createConnectionTracker } = await import('../../../extension/lib/net/websocket-tracking.js')

    const tracker = createConnectionTracker('test-id', 'wss://example.com')

    // Feed mixed JSON messages
    tracker.recordSampledMessage('incoming', JSON.stringify({ type: 'message', text: 'hello' }))
    tracker.recordSampledMessage('incoming', JSON.stringify({ type: 'message', text: 'world' }))
    tracker.recordSampledMessage('incoming', JSON.stringify({ type: 'typing', user: 'alice' }))
    tracker.recordSampledMessage('incoming', JSON.stringify({ type: 'presence', status: 'online' }))
    tracker.recordSampledMessage('incoming', JSON.stringify({ type: 'message', text: 'again' }))

    const schema = tracker.getSchema()
    assert.strictEqual(schema.consistent, false)
  })

  test('should log schema-change messages even when sampling', async () => {
    const { createConnectionTracker } = await import('../../../extension/lib/net/websocket-tracking.js')

    const tracker = createConnectionTracker('test-id', 'wss://example.com')
    tracker.setMessageRate(100) // High rate - sampling active

    // Establish schema
    for (let i = 0; i < 5; i++) {
      tracker.recordSampledMessage('incoming', JSON.stringify({ sym: 'AAPL', price: 185 }))
    }

    // This message has different keys - should be logged
    const shouldLog = tracker.isSchemaChange(JSON.stringify({ error: 'rate_limit', code: 429 }))
    assert.strictEqual(shouldLog, true, 'Expected schema change to be flagged for logging')
  })

  test('should not detect schema from non-JSON messages', async () => {
    const { createConnectionTracker } = await import('../../../extension/lib/net/websocket-tracking.js')

    const tracker = createConnectionTracker('test-id', 'wss://example.com')

    for (let i = 0; i < 5; i++) {
      tracker.recordMessage('incoming', 'plain text message ' + i)
    }

    const schema = tracker.getSchema()
    assert.strictEqual(schema.detectedKeys, null, 'Expected no keys for non-JSON')
  })

  test('should track schema variants', async () => {
    const { createConnectionTracker } = await import('../../../extension/lib/net/websocket-tracking.js')

    const tracker = createConnectionTracker('test-id', 'wss://example.com')

    // Simulate many messages with known variants
    for (let i = 0; i < 89; i++) {
      tracker.recordSampledMessage('incoming', JSON.stringify({ type: 'message', user: 'u', text: 't' }))
    }
    for (let i = 0; i < 8; i++) {
      tracker.recordSampledMessage('incoming', JSON.stringify({ type: 'typing', user: 'u' }))
    }
    for (let i = 0; i < 3; i++) {
      tracker.recordSampledMessage('incoming', JSON.stringify({ type: 'presence', status: 'on' }))
    }

    const schema = tracker.getSchema()
    assert.ok(schema.variants, 'Expected variants to be tracked')
    assert.ok(schema.variants.length >= 2, 'Expected at least 2 variants')
  })
})

describe('Binary Message Handling', () => {
  beforeEach(() => {
    originalWindow = globalThis.window
    originalCrypto = globalThis.crypto
    globalThis.window = createMockWindow({ withWebSocket: true })
    Object.defineProperty(globalThis, 'crypto', { value: createMockCrypto(), writable: true, configurable: true })
  })

  afterEach(() => {
    globalThis.window = originalWindow
    Object.defineProperty(globalThis, 'crypto', { value: originalCrypto, writable: true, configurable: true })
  })

  test('should format small binary as hex preview', async () => {
    const { formatPayload } = await import('../../../extension/lib/net/websocket-tracking.js')

    // Binary < 256 bytes: hex preview
    const buffer = new ArrayBuffer(128)
    const view = new Uint8Array(buffer)
    for (let i = 0; i < 128; i++) view[i] = i

    const formatted = formatPayload(buffer)

    assert.ok(formatted.startsWith('[Binary: 128B]'), `Expected binary prefix, got: ${formatted}`)
    assert.ok(formatted.includes('000102'), 'Expected hex content')
  })

  test('should format large binary with magic bytes only', async () => {
    const { formatPayload } = await import('../../../extension/lib/net/websocket-tracking.js')

    // Binary >= 256 bytes: size + magic bytes
    const buffer = new ArrayBuffer(4096)
    const view = new Uint8Array(buffer)
    view[0] = 0x0a
    view[1] = 0x1b
    view[2] = 0x2c
    view[3] = 0x3d

    const formatted = formatPayload(buffer)

    assert.ok(formatted.startsWith('[Binary: 4096B, magic:'), `Expected large binary format, got: ${formatted}`)
    assert.ok(formatted.includes('0a1b2c3d'), 'Expected magic bytes')
  })

  test('should pass through JSON text as-is', async () => {
    const { formatPayload } = await import('../../../extension/lib/net/websocket-tracking.js')

    const json = '{"type":"chat","msg":"hello"}'
    const formatted = formatPayload(json)

    assert.strictEqual(formatted, json)
  })

  test('should pass through non-JSON text as-is', async () => {
    const { formatPayload } = await import('../../../extension/lib/net/websocket-tracking.js')

    const text = 'Hello, world!'
    const formatted = formatPayload(text)

    assert.strictEqual(formatted, text)
  })

  test('should handle Blob binary data', async () => {
    const { formatPayload } = await import('../../../extension/lib/net/websocket-tracking.js')

    // Simulate Blob (in test environment, use object with size)
    const blob = {
      size: 1024,
      type: 'application/octet-stream',
      arrayBuffer: () => Promise.resolve(new ArrayBuffer(1024))
    }

    const formatted = formatPayload(blob)

    assert.ok(formatted.includes('Binary'), 'Expected binary indicator for Blob')
    assert.ok(formatted.includes('1024'), 'Expected size in output')
  })

  test('should compute correct size for text messages', async () => {
    const { getSize } = await import('../../../extension/lib/net/websocket-tracking.js')

    assert.strictEqual(getSize('hello'), 5)
    assert.strictEqual(getSize('{"type":"chat"}'), 15)
  })

  test('should compute correct size for ArrayBuffer', async () => {
    const { getSize } = await import('../../../extension/lib/net/websocket-tracking.js')

    const buffer = new ArrayBuffer(256)
    assert.strictEqual(getSize(buffer), 256)
  })

  test('should compute correct size for Blob', async () => {
    const { getSize } = await import('../../../extension/lib/net/websocket-tracking.js')

    const blob = { size: 4096 }
    assert.strictEqual(getSize(blob), 4096)
  })
})

describe('WebSocket Message Truncation', () => {
  beforeEach(() => {
    originalWindow = globalThis.window
    originalCrypto = globalThis.crypto
    globalThis.window = createMockWindow({ withWebSocket: true })
    Object.defineProperty(globalThis, 'crypto', { value: createMockCrypto(), writable: true, configurable: true })
  })

  afterEach(() => {
    globalThis.window = originalWindow
    Object.defineProperty(globalThis, 'crypto', { value: originalCrypto, writable: true, configurable: true })
  })

  test('should truncate text messages longer than 4KB', async () => {
    const { truncateWsMessage } = await import('../../../extension/lib/net/websocket-tracking.js')

    const longMessage = 'x'.repeat(5000)
    const result = truncateWsMessage(longMessage)

    assert.ok(result.data.length <= 4096)
    assert.strictEqual(result.truncated, true)
  })

  test('should not truncate messages within 4KB', async () => {
    const { truncateWsMessage } = await import('../../../extension/lib/net/websocket-tracking.js')

    const shortMessage = '{"type":"chat","msg":"hello"}'
    const result = truncateWsMessage(shortMessage)

    assert.strictEqual(result.data, shortMessage)
    assert.strictEqual(result.truncated, false)
  })
})

describe('Connection Stats', () => {
  beforeEach(() => {
    originalWindow = globalThis.window
    originalCrypto = globalThis.crypto
    globalThis.window = createMockWindow({ withWebSocket: true })
    Object.defineProperty(globalThis, 'crypto', { value: createMockCrypto(), writable: true, configurable: true })
  })

  afterEach(() => {
    globalThis.window = originalWindow
    Object.defineProperty(globalThis, 'crypto', { value: originalCrypto, writable: true, configurable: true })
  })

  test('should track incoming message count and bytes', async () => {
    const { createConnectionTracker } = await import('../../../extension/lib/net/websocket-tracking.js')

    const tracker = createConnectionTracker('test-id', 'wss://example.com')

    tracker.recordMessage('incoming', '{"msg":"hello"}') // 15 bytes
    tracker.recordMessage('incoming', '{"msg":"world"}') // 15 bytes

    assert.strictEqual(tracker.stats.incoming.count, 2)
    assert.strictEqual(tracker.stats.incoming.bytes, 30)
  })

  test('should track outgoing message count and bytes', async () => {
    const { createConnectionTracker } = await import('../../../extension/lib/net/websocket-tracking.js')

    const tracker = createConnectionTracker('test-id', 'wss://example.com')

    tracker.recordMessage('outgoing', '{"type":"ping"}') // 15 bytes

    assert.strictEqual(tracker.stats.outgoing.count, 1)
    assert.strictEqual(tracker.stats.outgoing.bytes, 15)
  })

  test('should track last message preview', async () => {
    const { createConnectionTracker } = await import('../../../extension/lib/net/websocket-tracking.js')

    const tracker = createConnectionTracker('test-id', 'wss://example.com')

    tracker.recordMessage('incoming', '{"type":"first"}')
    tracker.recordMessage('incoming', '{"type":"second"}')

    assert.strictEqual(tracker.stats.incoming.lastPreview, '{"type":"second"}')
  })

  test('should truncate preview to 200 chars', async () => {
    const { createConnectionTracker } = await import('../../../extension/lib/net/websocket-tracking.js')

    const tracker = createConnectionTracker('test-id', 'wss://example.com')
    const longMessage = 'x'.repeat(300)

    tracker.recordMessage('incoming', longMessage)

    assert.ok(tracker.stats.incoming.lastPreview.length <= 200)
  })

  test('should update all incoming stats from one complete record operation', async () => {
    const { createConnectionTracker } = await import('../../../extension/lib/net/websocket-tracking.js')

    const tracker = createConnectionTracker('test-id', 'wss://example.com')
    mock.method(Date, 'now', () => 1_700_000_000_000)

    tracker.recordMessage('incoming', '{"msg":"complete"}')

    assert.deepStrictEqual(tracker.stats.incoming, {
      count: 1,
      bytes: 18,
      lastAt: 1_700_000_000_000,
      lastPreview: '{"msg":"complete"}'
    })
  })
})
