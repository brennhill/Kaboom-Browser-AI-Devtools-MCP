/**
 * Purpose: Installs early WebSocket, fetch, and XHR shims before page scripts run and buffers pre-inject events.
 * Why: Prevents loss of startup network activity that occurs before the main inject capture bundle is initialized.
 * Docs: docs/features/feature/backend-log-streaming/index.md
 */

// early-patch.ts — Lightweight WebSocket, fetch, and XHR patches.
// Runs in MAIN world at document_start before any page scripts.
// Saves originals and buffers connections/bodies for handoff to inject.bundled.js.
// Must be self-contained: no imports, no chrome.* APIs (MAIN world).

// The helpers below sit at module scope rather than inside the IIFE. They are still
// self-contained — one file, no imports — and hoisting them keeps the install sequence
// readable instead of burying it under two generators.

/**
 * Assign a global that the page may have made read-only.
 *
 * Duplicated from src/lib/safe-global-patch.ts on purpose: this file runs in
 * the MAIN world at document_start and is required to be self-contained (see
 * the header). Keep the two in sync.
 *
 * Hardened pages define fetch/WebSocket as non-writable, so a plain assignment
 * throws "Cannot assign to read only property 'fetch' of object '#<Window>'".
 * Unguarded, that throw escaped and aborted every patch after it.
 */
function safeAssignGlobal<T extends object, K extends keyof T>(target: T, key: K, value: T[K]): boolean {
  try {
    // eslint-disable-next-line security/detect-object-injection -- key is a keyof T supplied by our own call sites
    target[key] = value
    // eslint-disable-next-line security/detect-object-injection -- same key, read back to confirm the write landed
    if (target[key] === value) return true
  } catch {
    // EXPECTED_ABSENCE: page-owned access can normally throw for detached,
    // cross-origin, or hostile objects; logging it would misleadingly blame Kaboom for page behavior.
    // Non-writable — try defineProperty below.
  }
  try {
    Object.defineProperty(target, key, { value, writable: true, configurable: true })
    // eslint-disable-next-line security/detect-object-injection -- same key, read back to confirm the write landed
    return target[key] === value
  } catch {
    // EXPECTED_ABSENCE: a non-configurable page-owned global is normal;
    // logging would mislabel the documented no-capture fallback as failure.
    return false
  }
}

/**
 * xorshift128 seeded by an FNV-1a hash of the seed string.
 *
 * Deterministic across runs and machines — the point of the exercise — and cheap enough
 * that replacing Math.random costs the page nothing measurable.
 */
function makeSeededRandom(seed: string): () => number {
  const lanes: number[] = []
  let hash = 2166136261 >>> 0
  for (let lane = 0; lane < 4; lane++) {
    const material = seed + ':' + lane
    for (let i = 0; i < material.length; i++) {
      hash ^= material.charCodeAt(i)
      hash = Math.imul(hash, 16777619) >>> 0
    }
    // A zero lane makes xorshift128 emit zeros forever, so every value the page reads
    // would be 0 and the "seeded" run would be silently degenerate.
    lanes.push(hash === 0 ? 0x9e3779b9 : hash)
  }
  let [x, y, z, w] = lanes as [number, number, number, number]
  return function next(): number {
    const t = (x ^ (x << 11)) >>> 0
    x = y
    y = z
    z = w
    w = (w ^ (w >>> 19) ^ (t ^ (t >>> 8))) >>> 0
    return w / 4294967296
  }
}

/**
 * Replace Math.random and crypto.getRandomValues with a seeded generator.
 *
 * Returns whether the replacement actually landed. A hardened page can make either
 * non-writable, and reporting a seed that never took effect would put a claim in the
 * emitted test that the recording never honoured.
 */
function installSeededRandomness(seed: string): boolean {
  const key = String(seed)
  if (window.__KABOOM_RANDOM_SEED_ACTIVE__ && window.__KABOOM_RANDOM_SEED__ === key) return true

  const next = makeSeededRandom(key)
  const mathOK = safeAssignGlobal(Math, 'random', next)

  let cryptoOK = true
  if (window.crypto && typeof window.crypto.getRandomValues === 'function') {
    const seededBytes = function <T extends ArrayBufferView | null>(array: T): T {
      if (array && ArrayBuffer.isView(array)) {
        const bytes = new Uint8Array(array.buffer, array.byteOffset, array.byteLength)
        for (let i = 0; i < bytes.length; i++) {
          // eslint-disable-next-line security/detect-object-injection -- i is a loop counter bounded by bytes.length
          bytes[i] = Math.floor(next() * 256)
        }
      }
      return array
    }
    cryptoOK = safeAssignGlobal(window.crypto, 'getRandomValues', seededBytes)
  }

  window.__KABOOM_RANDOM_SEED__ = key
  window.__KABOOM_RANDOM_SEED_ACTIVE__ = mathOK && cryptoOK
  return window.__KABOOM_RANDOM_SEED_ACTIVE__
}

;(function () {
  'use strict'

  if (typeof window === 'undefined') return

  // Cloaked domains — bail out before patching any globals.
  // Must be sync (MAIN world, no chrome APIs). Manifest exclude_matches is
  // the primary guard; this is a defense-in-depth fallback.
  const CLOAKED_DOMAINS = ['cloudflare.com']
  const host = location.hostname
  if (CLOAKED_DOMAINS.some((d) => host === d || host.endsWith('.' + d))) return

  // Guard: only install once (extension reloads, multiple frames)
  if (window.__KABOOM_ORIGINAL_WS__ || window.__KABOOM_ORIGINAL_FETCH__ || window.__KABOOM_EARLY_BODIES__) return

  // =========================================================================
  // SEEDED RANDOMNESS (opt-in, off unless a session pins it)
  //
  // A recorded session that calls Math.random cannot be replayed: the ids, the shuffles and
  // the sampled A/B arms differ on every run, so the generated test asserts on values that
  // will never come back. Seeding has to happen here because document_start in the MAIN
  // world is the last moment before page scripts can capture the originals.
  // =========================================================================

  // Published for the extension's pinning path, which cannot carry its own generator: two
  // independent Math.random replacements in one page would disagree with each other.
  window.__KABOOM_SEED_RANDOM__ = installSeededRandomness
  // The seed may already be here — the pinning snippet and this file both run before page
  // scripts, and their order is not guaranteed. Handling both orders is why the seed and
  // the installer are separate globals.
  if (typeof window.__KABOOM_RANDOM_SEED__ === 'string') installSeededRandomness(window.__KABOOM_RANDOM_SEED__)

  // =========================================================================
  // SHARED: Early body buffer (used by fetch + XHR patches)
  // =========================================================================

  const EARLY_BODY_MAX = 50
  const BODY_SIZE_CAP = 16384 // 16KB per body
  const BODY_READ_TIMEOUT = 5000 // 5s timeout for async body read
  // Only capture text/json-like responses
  const TEXT_CONTENT_RE = /^(text\/|application\/json|application\/.*\+json|application\/xml|application\/.*\+xml)/i

  const earlyBodies: EarlyNetworkBody[] = []
  window.__KABOOM_EARLY_BODIES__ = earlyBodies

  /** Push a body entry with FIFO eviction */
  function pushBody(entry: EarlyNetworkBody): void {
    earlyBodies.push(entry)
    if (earlyBodies.length > EARLY_BODY_MAX) {
      earlyBodies.shift()
    }
  }

  // =========================================================================
  // WEBSOCKET PATCH (existing)
  // =========================================================================

  if (window.WebSocket) {
    const OriginalWS = window.WebSocket

    // Store original for inject script to retrieve
    window.__KABOOM_ORIGINAL_WS__ = OriginalWS

    // Buffer for early connections
    const earlyConnections: EarlyWsConnection[] = []
    window.__KABOOM_EARLY_WS__ = earlyConnections

    // Thin wrapper: creates real WebSocket + buffers lifecycle events
    function EarlyWebSocket(this: unknown, url: string | URL, protocols?: string | string[]): WebSocket {
      const ws = protocols !== undefined ? new OriginalWS(url, protocols) : new OriginalWS(url)
      const conn: EarlyWsConnection = { ws, url: url.toString(), createdAt: Date.now(), events: [] }

      ws.addEventListener('open', () => {
        conn.events.push({ type: 'open', ts: Date.now() })
      })
      ws.addEventListener('close', (e: CloseEvent) => {
        conn.events.push({ type: 'close', ts: Date.now(), code: e.code, reason: e.reason })
      })
      ws.addEventListener('error', () => {
        conn.events.push({ type: 'error', ts: Date.now() })
      })

      earlyConnections.push(conn)

      // Cap buffer to bound memory
      if (earlyConnections.length > 50) {
        earlyConnections.shift()
      }

      return ws
    }

    // Preserve prototype chain: instanceof WebSocket still works
    EarlyWebSocket.prototype = OriginalWS.prototype

    // Preserve static constants
    Object.defineProperty(EarlyWebSocket, 'CONNECTING', { value: 0, writable: false })
    Object.defineProperty(EarlyWebSocket, 'OPEN', { value: 1, writable: false })
    Object.defineProperty(EarlyWebSocket, 'CLOSING', { value: 2, writable: false })
    Object.defineProperty(EarlyWebSocket, 'CLOSED', { value: 3, writable: false })

    if (!safeAssignGlobal(window, 'WebSocket', EarlyWebSocket as unknown as typeof WebSocket)) {
      // Read-only WebSocket: leave the page's own alone and skip early WS capture.
      // Mirror the fetch path — drop the stashes so inject's Phase 2 does not adopt
      // a shim we never installed, and so the buffer is not left dangling.
      delete window.__KABOOM_ORIGINAL_WS__
      delete window.__KABOOM_EARLY_WS__
    }
  }

  // =========================================================================
  // FETCH PATCH
  // =========================================================================

  if (typeof window.fetch === 'function') {
    const OriginalFetch = window.fetch

    // Store original for Phase 2 adoption
    window.__KABOOM_ORIGINAL_FETCH__ = OriginalFetch

    const patchedFetch = function (input: RequestInfo | URL, init?: RequestInit): Promise<Response> {
      // Determine URL and method
      let url = ''
      let method = 'GET'
      if (typeof input === 'string') {
        url = input
      } else if (input instanceof URL) {
        url = input.toString()
      } else if (input && typeof (input as Request).url === 'string') {
        url = (input as Request).url
        method = (input as Request).method || 'GET'
      }
      if (init?.method) {
        method = init.method
      }

      // Call original fetch — return the original promise unchanged
      const responsePromise = OriginalFetch.call(window, input, init)

      // Async body read in microtask — does NOT block the fetch return
      responsePromise
        .then((response: Response) => {
          try {
            const contentType = response.headers?.get?.('content-type') || ''
            // Only capture text/json responses
            if (!TEXT_CONTENT_RE.test(contentType)) return
            // Clone to avoid consuming the body
            const cloned = response.clone()
            const status = response.status

            // Race body read against timeout
            Promise.race([
              cloned.text(),
              new Promise<string>((resolve) => {
                setTimeout(() => resolve('[Skipped: body read timeout]'), BODY_READ_TIMEOUT)
              })
            ])
              .then((body: string) => {
                const truncated = body.length > BODY_SIZE_CAP ? body.slice(0, BODY_SIZE_CAP) : body
                pushBody({
                  url,
                  method: method.toUpperCase(),
                  status,
                  content_type: contentType,
                  response_body: truncated,
                  timestamp: Date.now()
                })
              })
              .catch(() => {
                // EXPECTED_ABSENCE: unreadable cloned bodies are normal page
                // behavior; logging them would misleadingly attribute page data limits to Kaboom.
              })
          } catch {
            // EXPECTED_ABSENCE: hostile response accessors are normal page
            // behavior; logging them would misleadingly blame Kaboom for page code.
          }
        })
        .catch(() => {
          // EXPECTED_ABSENCE: rejected page fetches are normal application behavior;
          // logging them would misleadingly duplicate the page's own network failure.
        })

      return responsePromise
    }

    if (!safeAssignGlobal(window, 'fetch', patchedFetch as typeof window.fetch)) {
      // Read-only fetch: leave the page's own alone and skip early fetch capture.
      // Everything below (XHR, buffering) still installs.
      delete window.__KABOOM_ORIGINAL_FETCH__
    }
  }

  // =========================================================================
  // XHR PATCH
  // =========================================================================

  if (typeof XMLHttpRequest !== 'undefined') {
    const OriginalOpen = XMLHttpRequest.prototype.open
    const OriginalSend = XMLHttpRequest.prototype.send

    // Store originals for Phase 2 adoption
    window.__KABOOM_ORIGINAL_XHR_OPEN__ = OriginalOpen
    window.__KABOOM_ORIGINAL_XHR_SEND__ = OriginalSend

    XMLHttpRequest.prototype.open = function (method: string, url: string | URL, ...rest: unknown[]) {
      ;(this as XMLHttpRequest & { __kaboomEarlyMethod: string }).__kaboomEarlyMethod = method
      ;(this as XMLHttpRequest & { __kaboomEarlyUrl: string }).__kaboomEarlyUrl =
        typeof url === 'string' ? url : url.toString()
      return OriginalOpen.apply(this, [method, url, ...rest] as Parameters<typeof XMLHttpRequest.prototype.open>)
    }

    XMLHttpRequest.prototype.send = function (body?: Document | XMLHttpRequestBodyInit | null) {
      const xhrUrl: string = (this as XMLHttpRequest & { __kaboomEarlyUrl?: string }).__kaboomEarlyUrl || ''
      const xhrMethod: string = (this as XMLHttpRequest & { __kaboomEarlyMethod?: string }).__kaboomEarlyMethod || 'GET'

      this.addEventListener('load', function (this: XMLHttpRequest) {
        try {
          const contentType = this.getResponseHeader('content-type') || ''
          // Only capture text/json responses
          if (!TEXT_CONTENT_RE.test(contentType)) return

          const responseType: string = this.responseType
          // Skip non-text response types (blob, arraybuffer, document)
          if (responseType && responseType !== '' && responseType !== 'text' && responseType !== 'json') return

          let responseBody: string | null = null
          try {
            responseBody = this.responseText
          } catch {
            // EXPECTED_ABSENCE: inaccessible responseText is normal for opaque/non-text XHR;
            // logging would mislabel the expected skipped capture as failure.
            return
          }
          if (responseBody === null) return

          const truncated = responseBody.length > BODY_SIZE_CAP ? responseBody.slice(0, BODY_SIZE_CAP) : responseBody

          pushBody({
            url: xhrUrl,
            method: xhrMethod.toUpperCase(),
            status: this.status,
            content_type: contentType,
            response_body: truncated,
            timestamp: Date.now()
          })
        } catch {
          // EXPECTED_ABSENCE: page-owned access can normally throw for detached,
          // cross-origin, or hostile objects; logging it would misleadingly blame Kaboom for page behavior.
          /* silent — early body capture must not affect page */
        }
      })

      return OriginalSend.call(this, body as XMLHttpRequestBodyInit | null | undefined)
    }
  }

  // =========================================================================
  // SELF-CLEANUP: If Phase 2 never adopts early patches (e.g., CSP blocks
  // inject bundle), restore originals and free buffers after 30 seconds.
  // Bounds worst-case memory leak to ~800KB for the 30-second window.
  // =========================================================================

  setTimeout(() => {
    // Phase 2 deletes __KABOOM_EARLY_BODIES__ on adoption — if it still
    // exists, Phase 2 never ran and we must clean up.
    if (window.__KABOOM_EARLY_BODIES__) {
      delete window.__KABOOM_EARLY_BODIES__

      // Restore fetch
      if (window.__KABOOM_ORIGINAL_FETCH__) {
        safeAssignGlobal(window, 'fetch', window.__KABOOM_ORIGINAL_FETCH__)
        delete window.__KABOOM_ORIGINAL_FETCH__
      }

      // Restore XHR
      if (window.__KABOOM_ORIGINAL_XHR_OPEN__) {
        XMLHttpRequest.prototype.open = window.__KABOOM_ORIGINAL_XHR_OPEN__
        delete window.__KABOOM_ORIGINAL_XHR_OPEN__
      }
      if (window.__KABOOM_ORIGINAL_XHR_SEND__) {
        XMLHttpRequest.prototype.send = window.__KABOOM_ORIGINAL_XHR_SEND__
        delete window.__KABOOM_ORIGINAL_XHR_SEND__
      }

      // Restore WebSocket through the same guard as the install and the fetch
      // restore: a plain assignment throws on a non-configurable read-only global,
      // and that throw here would abort the rest of the cleanup and leak the buffer.
      if (window.__KABOOM_ORIGINAL_WS__) {
        safeAssignGlobal(window, 'WebSocket', window.__KABOOM_ORIGINAL_WS__)
        delete window.__KABOOM_ORIGINAL_WS__
      }
      delete window.__KABOOM_EARLY_WS__
    }
  }, 30_000)
})()
