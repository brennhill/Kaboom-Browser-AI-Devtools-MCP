/**
 * Kaboom CI - Standalone browser capture for CI/CD pipelines
 *
 * Self-contained JavaScript file extracted from inject.js that runs in any
 * browser context without Chrome extension APIs. Replaces window.postMessage
 * with direct HTTP POST to the Kaboom server.
 *
 * Usage:
 *   await page.addInitScript({ path: require.resolve('@anthropic/kaboom-ci') })
 *
 * Configuration (set via window globals before this script runs):
 *   window.__KABOOM_PORT  (default: 7890)
 *   window.__KABOOM_HOST  (default: '127.0.0.1')
 *   window.__KABOOM_TEST_ID - current test identifier for correlation
 */

;(function () {
  'use strict'

  // === Configuration ===
  const KABOOM_HOST = window.__KABOOM_HOST || '127.0.0.1'
  const KABOOM_PORT = window.__KABOOM_PORT || 7890
  const BASE_URL = `http://${KABOOM_HOST}:${KABOOM_PORT}`

  // Guard against double initialization
  if (window.__KABOOM_CI_INITIALIZED) return
  window.__KABOOM_CI_INITIALIZED = true

  // Batch settings (avoid flooding server)
  const BATCH_INTERVAL_MS = 100 // Flush every 100ms
  const MAX_BATCH_SIZE = 50 // Max entries per flush
  const MAX_BATCH_BUFFER = 1000 // Max buffered entries per type (prevents OOM if server is down)

  // === Shared constants (from inject.js) ===
  const MAX_STRING_LENGTH = 10240
  const MAX_RESPONSE_LENGTH = 5120
  const MAX_DEPTH = 10
  const SENSITIVE_HEADERS = [
    'authorization',
    'cookie',
    'set-cookie',
    'x-auth-token',
    'x-api-key',
    'x-csrf-token',
    'proxy-authorization'
  ]

  // === Batching Layer ===
  let logBatch = []
  let wsBatch = []
  let networkBatch = []
  let flushTimer = null

  function scheduleFlush() {
    if (flushTimer) return
    flushTimer = setTimeout(flush, BATCH_INTERVAL_MS)
    return
  }

  function flush() {
    flushTimer = null

    if (logBatch.length > 0) {
      const entries = logBatch.splice(0, MAX_BATCH_SIZE)
      sendToServer('/logs', { entries })
    }
    if (wsBatch.length > 0) {
      const events = wsBatch.splice(0, MAX_BATCH_SIZE)
      sendToServer('/websocket-events', { events })
    }
    if (networkBatch.length > 0) {
      const bodies = networkBatch.splice(0, MAX_BATCH_SIZE)
      sendToServer('/network-bodies', { bodies })
    }

    // If there's still data, schedule another flush
    if (logBatch.length > 0 || wsBatch.length > 0 || networkBatch.length > 0) {
      scheduleFlush()
    }
    return
  }

  function sendToServer(endpoint, data) {
    // Use sendBeacon for reliability (won't be cancelled on page unload)
    var url = `${BASE_URL}${endpoint}`
    var body = JSON.stringify(data)

    if (navigator.sendBeacon) {
      navigator.sendBeacon(url, new Blob([body], { type: 'application/json' }))
    } else {
      // Fallback to fetch (fire-and-forget)
      fetch(url, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body,
        keepalive: true
      }).catch(function () {
        /* no-op */
      }) // Swallow errors — never interfere with the app
    }
    return
  }

  // === Transport Adapter ===
  // Replaces window.postMessage with server POST
  function emit(type, payload) {
    switch (type) {
      case 'kaboom_log':
        if (logBatch.length < MAX_BATCH_BUFFER) logBatch.push(payload)
        break
      case 'kaboom_ws':
        if (wsBatch.length < MAX_BATCH_BUFFER) wsBatch.push(payload)
        break
      case 'kaboom_network_body':
        if (networkBatch.length < MAX_BATCH_BUFFER) networkBatch.push(payload)
        break
      // Enhanced actions and web vitals go to logs
      case 'kaboom_enhanced_action':
      case 'kaboom_web_vitals':
        if (logBatch.length < MAX_BATCH_BUFFER) logBatch.push({ type: type, ...payload })
        break
    }
    scheduleFlush()
    return
  }

  // === safeSerialize (identical to inject.js) ===
  function serializeAtom(value, type) {
    if (type === 'string') {
      return value.length > MAX_STRING_LENGTH ? value.slice(0, MAX_STRING_LENGTH) + '... [truncated]' : value
    }
    if (type === 'number' || type === 'boolean') return value
    if (type === 'function') return '[Function: ' + (value.name || 'anonymous') + ']'
    if (type === 'symbol') return value.toString()
    if (type === 'bigint') return value.toString() + 'n'
    return undefined
  }

  function serializeInstance(value) {
    if (value instanceof Error) {
      return { name: value.name, message: value.message, stack: value.stack }
    }
    if (value instanceof RegExp) return value.toString()
    if (value instanceof Date) return value.toISOString()
    return undefined
  }

  function isDOMNode(value) {
    return (
      (typeof HTMLElement !== 'undefined' && value instanceof HTMLElement) ||
      (typeof Node !== 'undefined' && value instanceof Node)
    )
  }

  function serializeObject(value, depth, seen) {
    if (seen.has(value)) return '[Circular]'
    seen.add(value)
    if (Array.isArray(value)) {
      return value.slice(0, 100).map(function (v) {
        return safeSerialize(v, depth + 1, seen)
      })
    }
    if (isDOMNode(value)) {
      return '[' + value.constructor.name + ': ' + (value.tagName || value.nodeName) + ']'
    }
    var result = {}
    var keys = Object.keys(value).slice(0, 50)
    for (var i = 0; i < keys.length; i++) {
      try {
        result[keys[i]] = safeSerialize(value[keys[i]], depth + 1, seen)
      } catch (_e) {
        result[keys[i]] = '[unserializable]'
      }
    }
    return result
  }

  function safeSerialize(value, depth, seen) {
    if (depth === undefined) depth = 0
    if (seen === undefined) seen = new WeakSet()
    if (value === null) return null
    if (value === undefined) return undefined
    var type = typeof value
    var atom = serializeAtom(value, type)
    if (atom !== undefined) return atom
    if (depth >= MAX_DEPTH) return '[max depth reached]'
    var instance = serializeInstance(value)
    if (instance !== undefined) return instance
    if (type === 'object') return serializeObject(value, depth, seen)
    return String(value)
  }

  // === Console Capture ===
  function installConsoleCapture() {
    var levels = ['log', 'warn', 'error', 'info', 'debug']
    var originals = {}

    for (var level of levels) {
      originals[level] = console[level]

      console[level] = (function (capturedLevel) {
        return function () {
          var args = Array.prototype.slice.call(arguments)

          originals[capturedLevel].apply(console, args)

          emit('kaboom_log', {
            level: capturedLevel,
            message: args
              .map(function (a) {
                if (typeof a === 'string') return a
                var serialized = safeSerialize(a)
                return typeof serialized === 'object' ? JSON.stringify(serialized) : String(serialized)
              })
              .join(' '),
            args: args.map(function (a) {
              return safeSerialize(a)
            }),
            timestamp: new Date().toISOString(),
            url: window.location.href,
            source: 'console'
          })
        }
      })(level)
    }
    return
  }

  // === Exception Capture ===
  function installExceptionCapture() {
    window.addEventListener('error', function (event) {
      emit('kaboom_log', {
        level: 'error',
        message: event.message || 'Unknown error',
        args: [safeSerialize(event.error)],
        timestamp: new Date().toISOString(),
        url: window.location.href,
        source: 'exception',
        stack: event.error ? event.error.stack : undefined,
        filename: event.filename,
        lineno: event.lineno,
        colno: event.colno
      })
    })

    window.addEventListener('unhandledrejection', function (event) {
      var reason = event.reason
      emit('kaboom_log', {
        level: 'error',
        message: (reason && reason.message) || String(reason) || 'Unhandled Promise rejection',
        args: [safeSerialize(reason)],
        timestamp: new Date().toISOString(),
        url: window.location.href,
        source: 'unhandledrejection',
        stack: reason ? reason.stack : undefined
      })
    })
    return
  }

  // === Fetch/XHR Capture ===
  function installNetworkCapture() {
    var originalFetch = window.fetch

    window.fetch = function (input, init) {
      if (!init) init = {}
      var url = typeof input === 'string' ? input : input.url
      var method = init.method || (input.method ? input.method : 'GET')
      var startTime = Date.now()

      // Skip requests to the Kaboom server itself
      if (url && url.indexOf(BASE_URL) === 0) {
        return originalFetch.apply(this, arguments)
      }

      var requestBody = null
      if (init.body) {
        try {
          requestBody = typeof init.body === 'string' ? init.body.slice(0, MAX_RESPONSE_LENGTH) : '[non-string body]'
        } catch (_e) {
          requestBody = '[unreadable]'
        }
      }

      return originalFetch.apply(this, arguments).then(
        function (response) {
          var duration = Date.now() - startTime

          // Capture network body for non-2xx responses
          if (response.status >= 400) {
            var clone = response.clone()
            clone
              .text()
              .then(function (text) {
                emit('kaboom_network_body', {
                  url: url,
                  method: method.toUpperCase(),
                  status: response.status,
                  requestBody: requestBody,
                  responseBody: text.slice(0, MAX_RESPONSE_LENGTH),
                  duration: duration,
                  timestamp: new Date().toISOString(),
                  contentType: response.headers.get('content-type') || '',
                  requestHeaders: filterHeaders(init.headers),
                  responseHeaders: filterHeaders(Object.fromEntries(response.headers.entries()))
                })
              })
              .catch(function () {})
          }

          // Always log errors to console stream
          if (response.status >= 400) {
            emit('kaboom_log', {
              level: response.status >= 500 ? 'error' : 'warn',
              message: method.toUpperCase() + ' ' + url + ' \u2192 ' + response.status,
              timestamp: new Date().toISOString(),
              url: window.location.href,
              source: 'network',
              metadata: { status: response.status, duration: duration }
            })
          }

          return response
        },
        function (error) {
          emit('kaboom_log', {
            level: 'error',
            message: method.toUpperCase() + ' ' + url + ' \u2192 Network Error: ' + error.message,
            timestamp: new Date().toISOString(),
            url: window.location.href,
            source: 'network',
            metadata: {
              error: error.message,
              duration: Date.now() - startTime
            }
          })
          throw error
        }
      )
    }
    return
  }

  // === WebSocket Capture ===
  function installWebSocketCapture() {
    var OriginalWebSocket = window.WebSocket

    window.WebSocket = function (url, protocols) {
      var ws = new OriginalWebSocket(url, protocols)
      var id =
        typeof crypto !== 'undefined' && crypto.randomUUID
          ? crypto.randomUUID()
          : 'ws_' + Date.now() + '_' + Math.random().toString(36).slice(2) // nosemgrep: rules_lgpl_javascript_crypto_rule-node-insecure-random-generator -- non-cryptographic use: generating unique WebSocket connection tracking ID

      emit('kaboom_ws', {
        event: 'connecting',
        id: id,
        url: url,
        ts: new Date().toISOString()
      })

      ws.addEventListener('open', function () {
        emit('kaboom_ws', {
          event: 'open',
          id: id,
          url: url,
          ts: new Date().toISOString()
        })
      })

      ws.addEventListener('message', function (event) {
        var data = typeof event.data === 'string' ? event.data : '[binary]'
        emit('kaboom_ws', {
          event: 'message',
          id: id,
          url: url,
          direction: 'incoming',
          data: data.slice(0, MAX_STRING_LENGTH),
          size: event.data.length || 0,
          ts: new Date().toISOString()
        })
      })

      ws.addEventListener('close', function (event) {
        emit('kaboom_ws', {
          event: 'close',
          id: id,
          url: url,
          code: event.code,
          reason: event.reason,
          ts: new Date().toISOString()
        })
      })

      ws.addEventListener('error', function () {
        emit('kaboom_ws', {
          event: 'error',
          id: id,
          url: url,
          ts: new Date().toISOString()
        })
      })

      // Wrap send
      var originalSend = ws.send.bind(ws)
      ws.send = function (data) {
        var payload = typeof data === 'string' ? data : '[binary]'
        emit('kaboom_ws', {
          event: 'message',
          id: id,
          url: url,
          direction: 'outgoing',
          data: payload.slice(0, MAX_STRING_LENGTH),
          size: data.length || 0,
          ts: new Date().toISOString()
        })
        return originalSend(data)
      }

      return ws
    }

    // Preserve static properties
    window.WebSocket.CONNECTING = OriginalWebSocket.CONNECTING
    window.WebSocket.OPEN = OriginalWebSocket.OPEN
    window.WebSocket.CLOSING = OriginalWebSocket.CLOSING
    window.WebSocket.CLOSED = OriginalWebSocket.CLOSED
    window.WebSocket.prototype = OriginalWebSocket.prototype
    return
  }

  // === Helpers ===
  function filterHeaders(headers) {
    if (!headers) return {}
    var filtered = {}
    var entries
    if (typeof Headers !== 'undefined' && headers instanceof Headers) {
      entries = Array.from(headers.entries())
    } else if (headers && typeof headers === 'object') {
      entries = Object.entries(headers)
    } else {
      return {}
    }
    for (var i = 0; i < entries.length; i++) {
      var key = entries[i][0]

      var value = entries[i][1]
      if (SENSITIVE_HEADERS.indexOf(key.toLowerCase()) !== -1) {
        filtered[key] = '[REDACTED]'
      } else {
        filtered[key] = value
      }
    }
    return filtered
  }

  // === Lifecycle Hooks ===

  // Flush on page unload
  window.addEventListener('beforeunload', function () {
    flush()
  })

  // Signal to server that this page loaded (useful for test correlation)
  sendToServer('/logs', {
    entries: [
      {
        level: 'info',
        message: '[kaboom-ci] Capture initialized',
        timestamp: new Date().toISOString(),
        url: window.location.href,
        source: 'kaboom-ci',
        metadata: {
          testId: window.__KABOOM_TEST_ID || null,
          captureVersion: '5.2.0'
        }
      }
    ]
  })

  // === Install All Captures ===
  installConsoleCapture()
  installExceptionCapture()
  installNetworkCapture()
  installWebSocketCapture()
  return
})()
