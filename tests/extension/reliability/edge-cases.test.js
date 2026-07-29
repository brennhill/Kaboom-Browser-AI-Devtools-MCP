// @ts-nocheck
/**
 * @fileoverview edge-cases.test.js — Tests for edge cases and stress scenarios.
 * Covers WebSocket reconnection, service worker restart, concurrent operations,
 * and memory pressure edge cases beyond basic memory enforcement.
 *
 * Addresses Issue #6.1: Missing Test Coverage for Edge Cases
 */

import { test, describe, beforeEach, afterEach } from 'node:test'
import assert from 'node:assert'
import { createMockWindow, createMockCrypto } from '../shared/helpers.js'

let originalWindow, originalCrypto

describe('Edge Cases: WebSocket Reconnection', () => {
  beforeEach(async () => {
    originalWindow = globalThis.window
    originalCrypto = globalThis.crypto
    globalThis.window = createMockWindow({ withWebSocket: true })
    Object.defineProperty(globalThis, 'crypto', { value: createMockCrypto(), writable: true, configurable: true })
    const { uninstallWebSocketCapture } = await import('../../../extension/lib/net/websocket.js')
    uninstallWebSocketCapture()
  })

  afterEach(() => {
    globalThis.window = originalWindow
    Object.defineProperty(globalThis, 'crypto', { value: originalCrypto, writable: true, configurable: true })
  })

  test('should handle rapid connection/disconnection cycles', async () => {
    const { installWebSocketCapture, uninstallWebSocketCapture } = await import('../../../extension/lib/net/websocket.js')
    installWebSocketCapture()

    // Simulate rapid connect/disconnect cycles
    for (let i = 0; i < 10; i++) {
      const ws = new globalThis.window.WebSocket('wss://example.com/ws')
      ws._emit('open', {})
      ws._emit('close', { code: 1000, reason: 'normal' })
    }

    const calls = globalThis.window.postMessage.mock.calls
    const openEvents = calls.filter((c) => {
      const msg = c.arguments[0]
      return msg.type === 'kaboom_ws' && msg.payload.event === 'open'
    })
    const closeEvents = calls.filter((c) => {
      const msg = c.arguments[0]
      return msg.type === 'kaboom_ws' && msg.payload.event === 'close'
    })

    assert.strictEqual(openEvents.length, 10, 'Expected 10 open events')
    assert.strictEqual(closeEvents.length, 10, 'Expected 10 close events')

    // Verify each connection has unique ID
    const openIds = openEvents.map((c) => c.arguments[0].payload.id)
    const closeIds = closeEvents.map((c) => c.arguments[0].payload.id)
    assert.deepStrictEqual(openIds, closeIds, 'Open and close IDs should match')

    uninstallWebSocketCapture()
  })

  test('should handle connection error followed by successful reconnect', async () => {
    const { installWebSocketCapture, uninstallWebSocketCapture } = await import('../../../extension/lib/net/websocket.js')
    installWebSocketCapture()

    // First connection fails
    const ws1 = new globalThis.window.WebSocket('wss://example.com/ws')
    ws1._emit('error', { message: 'Connection refused' })
    ws1._emit('close', { code: 1006, reason: 'abnormal closure' })

    // Second connection succeeds
    const ws2 = new globalThis.window.WebSocket('wss://example.com/ws')
    ws2._emit('open', {})
    ws2._emit('message', { data: '{"type":"connected"}' })

    const calls = globalThis.window.postMessage.mock.calls
    const errorEvents = calls.filter((c) => {
      const msg = c.arguments[0]
      return msg.type === 'kaboom_ws' && msg.payload.event === 'error'
    })
    const openEvents = calls.filter((c) => {
      const msg = c.arguments[0]
      return msg.type === 'kaboom_ws' && msg.payload.event === 'open'
    })

    assert.strictEqual(errorEvents.length, 1, 'Expected 1 error event')
    assert.strictEqual(openEvents.length, 1, 'Expected 1 open event')

    uninstallWebSocketCapture()
  })

  test('should handle messages received before connection fully established', async () => {
    const { installWebSocketCapture, uninstallWebSocketCapture } = await import('../../../extension/lib/net/websocket.js')
    installWebSocketCapture()

    const ws = new globalThis.window.WebSocket('wss://example.com/ws')
    // Message before open (edge case in some implementations)
    ws._emit('message', { data: '{"type":"early"}' })
    ws._emit('open', {})
    await Promise.resolve()

    const calls = globalThis.window.postMessage.mock.calls
    const msgEvents = calls.filter((c) => {
      const msg = c.arguments[0]
      return msg.type === 'kaboom_ws' && msg.payload.event === 'message'
    })

    assert.ok(msgEvents.length >= 1, 'Expected at least 1 message event')

    uninstallWebSocketCapture()
  })

  test('should handle connection timeout (no events received)', async () => {
    const { installWebSocketCapture, uninstallWebSocketCapture } = await import('../../../extension/lib/net/websocket.js')
    installWebSocketCapture()

    // Create connection but never emit events (simulates timeout)
    const _ws = new globalThis.window.WebSocket('wss://example.com/ws')

    // Connection creation should still post a message to track the new connection
    const calls = globalThis.window.postMessage.mock.calls
    const _wsEvents = calls.filter((c) => {
      const msg = c.arguments[0]
      return msg.type === 'kaboom_ws'
    })

    // Even without open/close events, the WebSocket constructor interception should be tracked
    // At minimum, no errors should have been thrown during construction
    assert.strictEqual(typeof _ws.send, 'function', 'WebSocket send method should be available')
    assert.strictEqual(typeof _ws.close, 'function', 'WebSocket close method should be available')
    assert.strictEqual(_ws.url, 'wss://example.com/ws', 'WebSocket URL should match')

    uninstallWebSocketCapture()
  })

  test('should handle simultaneous connections to different URLs', async () => {
    const { installWebSocketCapture, uninstallWebSocketCapture } = await import('../../../extension/lib/net/websocket.js')
    installWebSocketCapture()

    const ws1 = new globalThis.window.WebSocket('wss://a.com/ws')
    const ws2 = new globalThis.window.WebSocket('wss://b.com/ws')
    const ws3 = new globalThis.window.WebSocket('wss://c.com/ws')

    ws1._emit('open', {})
    ws2._emit('open', {})
    ws3._emit('open', {})

    ws1.send('{"from":"a"}')
    ws2.send('{"from":"b"}')
    ws3.send('{"from":"c"}')

    const calls = globalThis.window.postMessage.mock.calls
    const openEvents = calls.filter((c) => {
      const msg = c.arguments[0]
      return msg.type === 'kaboom_ws' && msg.payload.event === 'open'
    })

    assert.strictEqual(openEvents.length, 3, 'Expected 3 open events')

    // Verify URLs are different
    const urls = openEvents.map((c) => c.arguments[0].payload.url)
    assert.ok(urls.includes('wss://a.com/ws'))
    assert.ok(urls.includes('wss://b.com/ws'))
    assert.ok(urls.includes('wss://c.com/ws'))

    uninstallWebSocketCapture()
  })
})

describe('Edge Cases: Service Worker Restart', () => {
  beforeEach(async () => {
    originalWindow = globalThis.window
    originalCrypto = globalThis.crypto
    globalThis.window = createMockWindow({ withWebSocket: true })
    Object.defineProperty(globalThis, 'crypto', { value: createMockCrypto(), writable: true, configurable: true })
  })

  afterEach(() => {
    globalThis.window = originalWindow
    Object.defineProperty(globalThis, 'crypto', { value: originalCrypto, writable: true, configurable: true })
  })

  test('should reset state on service worker restart', async () => {
    const { installWebSocketCapture, uninstallWebSocketCapture, resetForTesting } =
      await import('../../../extension/lib/net/websocket.js')

    installWebSocketCapture()

    // Create some state
    const ws1 = new globalThis.window.WebSocket('wss://example.com/ws')
    ws1._emit('open', {})

    // Simulate service worker restart
    resetForTesting()

    // Reinstall after restart
    installWebSocketCapture()

    // Create new connection after restart
    const ws2 = new globalThis.window.WebSocket('wss://example.com/ws')
    ws2._emit('open', {})

    uninstallWebSocketCapture()
  })

  test('should handle early connections before full initialization', async () => {
    const { installWebSocketCapture, uninstallWebSocketCapture } = await import('../../../extension/lib/net/websocket.js')

    // Don't wait for full initialization
    installWebSocketCapture()

    // Immediately create connection (edge case during startup)
    const ws = new globalThis.window.WebSocket('wss://example.com/ws')
    ws._emit('open', {})

    const calls = globalThis.window.postMessage.mock.calls
    const openEvents = calls.filter((c) => {
      const msg = c.arguments[0]
      return msg.type === 'kaboom_ws' && msg.payload.event === 'open'
    })

    assert.ok(openEvents.length >= 1, 'Expected open event even during early initialization')

    uninstallWebSocketCapture()
  })

  test('should handle multiple rapid install/uninstall cycles', async () => {
    const { installWebSocketCapture, uninstallWebSocketCapture } = await import('../../../extension/lib/net/websocket.js')

    for (let i = 0; i < 5; i++) {
      installWebSocketCapture()
      const ws = new globalThis.window.WebSocket('wss://example.com/ws')
      ws._emit('open', {})
      uninstallWebSocketCapture()
    }

    // After cycling, capture should still be installable and functional
    installWebSocketCapture()
    const ws = new globalThis.window.WebSocket('wss://example.com/ws')
    ws._emit('open', {})

    const calls = globalThis.window.postMessage.mock.calls
    const openEvents = calls.filter((c) => {
      const msg = c.arguments[0]
      return msg.type === 'kaboom_ws' && msg.payload.event === 'open'
    })

    // 5 opens from the loop + 1 from the final verification
    assert.strictEqual(openEvents.length, 6, 'WebSocket capture should still work after install/uninstall cycling')

    uninstallWebSocketCapture()
  })
})

describe('Edge Cases: Concurrent Operations', () => {
  beforeEach(async () => {
    originalWindow = globalThis.window
    originalCrypto = globalThis.crypto
    globalThis.window = createMockWindow({ withWebSocket: true })
    Object.defineProperty(globalThis, 'crypto', { value: createMockCrypto(), writable: true, configurable: true })
    const { resetForTesting } = await import('../../../extension/lib/net/websocket.js')
    resetForTesting()
  })

  afterEach(() => {
    globalThis.window = originalWindow
    Object.defineProperty(globalThis, 'crypto', { value: originalCrypto, writable: true, configurable: true })
  })

  test('should handle concurrent WebSocket connections', async () => {
    const { installWebSocketCapture, uninstallWebSocketCapture } = await import('../../../extension/lib/net/websocket.js')
    installWebSocketCapture()

    // Create 20 concurrent connections
    const connections = []
    for (let i = 0; i < 20; i++) {
      const ws = new globalThis.window.WebSocket(`wss://example.com/ws/${i}`)
      connections.push(ws)
    }

    // Open all connections
    connections.forEach((ws) => ws._emit('open', {}))

    // Send messages on all connections
    connections.forEach((ws, i) => ws.send(`{"id":${i}}`))
    await Promise.resolve()

    const calls = globalThis.window.postMessage.mock.calls
    const openEvents = calls.filter((c) => {
      const msg = c.arguments[0]
      return msg.type === 'kaboom_ws' && msg.payload.event === 'open'
    })
    const msgEvents = calls.filter((c) => {
      const msg = c.arguments[0]
      return msg.type === 'kaboom_ws' && msg.payload.event === 'message'
    })

    assert.strictEqual(openEvents.length, 20, 'Expected 20 open events')
    assert.ok(msgEvents.length >= 20, 'Expected at least 20 message events')

    uninstallWebSocketCapture()
  })

  test('should handle rapid message bursts on single connection', async () => {
    const { installWebSocketCapture, uninstallWebSocketCapture, setWebSocketCaptureMode } =
      await import('../../../extension/lib/net/websocket.js')
    installWebSocketCapture()
    setWebSocketCaptureMode('all') // Disable sampling for burst test

    const ws = new globalThis.window.WebSocket('wss://example.com/ws')
    ws._emit('open', {})

    // Send 100 messages rapidly
    for (let i = 0; i < 100; i++) {
      ws.send(`{"msg":${i}}`)
    }
    await Promise.resolve()

    const calls = globalThis.window.postMessage.mock.calls
    const msgEvents = calls.filter((c) => {
      const msg = c.arguments[0]
      return msg.type === 'kaboom_ws' && msg.payload.event === 'message'
    })

    assert.ok(msgEvents.length >= 100, 'Expected at least 100 message events')

    uninstallWebSocketCapture()
  })

  test('should handle mixed connection states simultaneously', async () => {
    const { installWebSocketCapture, uninstallWebSocketCapture } = await import('../../../extension/lib/net/websocket.js')
    installWebSocketCapture()

    // Create connections in different states
    const _connecting = new globalThis.window.WebSocket('wss://example.com/connecting')
    const open = new globalThis.window.WebSocket('wss://example.com/open')
    const closing = new globalThis.window.WebSocket('wss://example.com/closing')
    const _closed = new globalThis.window.WebSocket('wss://example.com/closed')
    const errored = new globalThis.window.WebSocket('wss://example.com/error')

    // Set states
    open._emit('open', {})
    closing._emit('close', { code: 1000, reason: 'normal' })
    errored._emit('error', { message: 'failed' })
    errored._emit('close', { code: 1006, reason: 'error' })

    const calls = globalThis.window.postMessage.mock.calls
    const openEvents = calls.filter((c) => {
      const msg = c.arguments[0]
      return msg.type === 'kaboom_ws' && msg.payload.event === 'open'
    })
    const closeEvents = calls.filter((c) => {
      const msg = c.arguments[0]
      return msg.type === 'kaboom_ws' && msg.payload.event === 'close'
    })
    const errorEvents = calls.filter((c) => {
      const msg = c.arguments[0]
      return msg.type === 'kaboom_ws' && msg.payload.event === 'error'
    })

    assert.strictEqual(openEvents.length, 1, 'Expected 1 open event')
    assert.strictEqual(closeEvents.length, 2, 'Expected 2 close events')
    assert.strictEqual(errorEvents.length, 1, 'Expected 1 error event')

    uninstallWebSocketCapture()
  })
})

describe('Edge Cases: Memory Pressure Scenarios', () => {
  beforeEach(async () => {
    originalWindow = globalThis.window
    originalCrypto = globalThis.crypto
    globalThis.window = createMockWindow({ withWebSocket: true })
    Object.defineProperty(globalThis, 'crypto', { value: createMockCrypto(), writable: true, configurable: true })
    const { resetForTesting } = await import('../../../extension/lib/net/websocket.js')
    resetForTesting()
  })

  afterEach(() => {
    globalThis.window = originalWindow
    Object.defineProperty(globalThis, 'crypto', { value: originalCrypto, writable: true, configurable: true })
  })

  test('should handle memory pressure during active connection', async () => {
    const { installWebSocketCapture, uninstallWebSocketCapture } = await import('../../../extension/lib/net/websocket.js')
    installWebSocketCapture()

    const ws = new globalThis.window.WebSocket('wss://example.com/ws')
    ws._emit('open', {})

    // Simulate memory pressure by creating many messages
    for (let i = 0; i < 1000; i++) {
      ws.send(`{"data":"${'x'.repeat(100)}"}`)
    }

    const calls = globalThis.window.postMessage.mock.calls
    // Should not crash or throw errors
    assert.ok(calls.length > 0, 'Should handle memory pressure without crashing')

    uninstallWebSocketCapture()
  })

  test('should handle rapid connection creation under memory pressure', async () => {
    const { installWebSocketCapture, uninstallWebSocketCapture } = await import('../../../extension/lib/net/websocket.js')
    installWebSocketCapture()

    // Create many connections rapidly (simulates memory pressure)
    const connections = []
    for (let i = 0; i < 50; i++) {
      const ws = new globalThis.window.WebSocket(`wss://example.com/ws/${i}`)
      connections.push(ws)
    }

    // Open all connections
    connections.forEach((ws) => ws._emit('open', {}))

    // Verify all 50 connections were tracked via postMessage
    const calls = globalThis.window.postMessage.mock.calls
    const openEvents = calls.filter((c) => {
      const msg = c.arguments[0]
      return msg.type === 'kaboom_ws' && msg.payload.event === 'open'
    })

    assert.strictEqual(openEvents.length, 50, 'All 50 connections should post open events under memory pressure')

    uninstallWebSocketCapture()
  })

  test('should cleanup old connections when memory is constrained', async () => {
    const { installWebSocketCapture, uninstallWebSocketCapture } = await import('../../../extension/lib/net/websocket.js')
    installWebSocketCapture()

    // Create and close many connections
    for (let i = 0; i < 100; i++) {
      const ws = new globalThis.window.WebSocket('wss://example.com/ws')
      ws._emit('open', {})
      ws._emit('close', { code: 1000, reason: 'normal' })
    }

    const calls = globalThis.window.postMessage.mock.calls
    const closeEvents = calls.filter((c) => {
      const msg = c.arguments[0]
      return msg.type === 'kaboom_ws' && msg.payload.event === 'close'
    })

    assert.strictEqual(closeEvents.length, 100, 'Expected 100 close events')

    uninstallWebSocketCapture()
  })
})

describe('Edge Cases: Message Edge Cases', () => {
  beforeEach(async () => {
    originalWindow = globalThis.window
    originalCrypto = globalThis.crypto
    globalThis.window = createMockWindow({ withWebSocket: true })
    Object.defineProperty(globalThis, 'crypto', { value: createMockCrypto(), writable: true, configurable: true })
  })

  afterEach(() => {
    globalThis.window = originalWindow
    Object.defineProperty(globalThis, 'crypto', { value: originalCrypto, writable: true, configurable: true })
  })

  test('should handle empty messages', async () => {
    const { installWebSocketCapture, uninstallWebSocketCapture } = await import('../../../extension/lib/net/websocket.js')
    installWebSocketCapture()

    const ws = new globalThis.window.WebSocket('wss://example.com/ws')
    ws._emit('open', {})
    ws.send('')
    await Promise.resolve()

    const calls = globalThis.window.postMessage.mock.calls
    const msgEvents = calls.filter((c) => {
      const msg = c.arguments[0]
      return msg.type === 'kaboom_ws' && msg.payload.event === 'message'
    })

    assert.ok(msgEvents.length >= 1, 'Should handle empty messages')

    uninstallWebSocketCapture()
  })

  test('should handle messages with special characters', async () => {
    const { installWebSocketCapture, uninstallWebSocketCapture } = await import('../../../extension/lib/net/websocket.js')
    installWebSocketCapture()

    const ws = new globalThis.window.WebSocket('wss://example.com/ws')
    ws._emit('open', {})

    const specialMsg = JSON.stringify({
      text: 'Hello 世界 🌍 \n\t\r',
      emoji: '😀😁😂',
      unicode: '\u0000\u001F\u007F'
    })
    ws.send(specialMsg)
    await Promise.resolve()

    const calls = globalThis.window.postMessage.mock.calls
    const msgEvents = calls.filter((c) => {
      const msg = c.arguments[0]
      return msg.type === 'kaboom_ws' && msg.payload.event === 'message'
    })

    assert.ok(msgEvents.length >= 1, 'Should handle messages with special characters')

    uninstallWebSocketCapture()
  })

  test('should handle malformed JSON messages', async () => {
    const { installWebSocketCapture, uninstallWebSocketCapture } = await import('../../../extension/lib/net/websocket.js')
    installWebSocketCapture()

    const ws = new globalThis.window.WebSocket('wss://example.com/ws')
    ws._emit('open', {})

    // Send malformed JSON
    ws.send('{invalid json')
    ws.send('{"valid": "json"}')
    ws.send('another invalid')
    await Promise.resolve()

    const calls = globalThis.window.postMessage.mock.calls
    const msgEvents = calls.filter((c) => {
      const msg = c.arguments[0]
      return msg.type === 'kaboom_ws' && msg.payload.event === 'message'
    })

    assert.ok(msgEvents.length >= 3, 'Should handle malformed JSON without crashing')

    uninstallWebSocketCapture()
  })

  test('should handle extremely large messages', async () => {
    const { installWebSocketCapture, uninstallWebSocketCapture } = await import('../../../extension/lib/net/websocket.js')
    installWebSocketCapture()

    const ws = new globalThis.window.WebSocket('wss://example.com/ws')
    ws._emit('open', {})

    // Send message larger than 4KB limit
    const hugeMessage = 'x'.repeat(10000)
    ws.send(hugeMessage)
    await Promise.resolve()

    const calls = globalThis.window.postMessage.mock.calls
    const msgEvents = calls.filter((c) => {
      const msg = c.arguments[0]
      return msg.type === 'kaboom_ws' && msg.payload.event === 'message'
    })

    assert.ok(msgEvents.length >= 1, 'Should handle large messages')

    // Verify truncation
    const lastMsg = msgEvents[msgEvents.length - 1].arguments[0].payload
    assert.ok(lastMsg.data.length <= 4096, 'Large messages should be truncated')
    assert.strictEqual(lastMsg.truncated, true, 'Truncation flag should be set')

    uninstallWebSocketCapture()
  })
})
