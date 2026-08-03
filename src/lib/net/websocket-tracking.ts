/**
 * Purpose: WebSocket connection statistics, adaptive message sampling, schema inference, and message formatting primitives.
 * Docs: docs/features/feature/observe/index.md
 */

// websocket-tracking.ts — WebSocket connection tracking, adaptive sampling, and schema detection.

/**
 * @fileoverview WebSocket tracking primitives.
 * Connection statistics, adaptive sampling, schema inference, and message
 * formatting utilities used by the WebSocket capture instrumentation layer.
 */

import { WS_MAX_BODY_SIZE, WS_PREVIEW_LIMIT } from '../constants.js'
import type { WebSocketCaptureMode } from '../../types/capture/websocket.js'

// =============================================================================
// TYPE DEFINITIONS
// =============================================================================

/** WebSocket message data variants */
export type WebSocketMessageData = string | ArrayBuffer | Blob

/** Object with a size property (like Blob) */
export interface SizedObject {
  size: number
}

/** Per-direction message statistics */
interface DirectionStats {
  count: number
  bytes: number
  lastPreview: string | null
  lastAt: number | null
}

/** Connection statistics */
export interface ConnectionStats {
  incoming: DirectionStats
  outgoing: DirectionStats
}

/** Message direction */
export type MessageDirection = 'incoming' | 'outgoing'

/** Truncation result */
export interface TruncationResult {
  data: string
  truncated: boolean
}

/** Sampling info */
export interface SamplingInfo {
  rate: string
  logged: string
  window: string
}

/** Schema info */
export interface SchemaInfo {
  detectedKeys: string[] | null
  consistent: boolean
  variants?: string[]
}

/** Connection tracker interface */
export interface ConnectionTracker {
  id: string
  url: string
  messageCount: number
  _sampleCounter: number
  _messageRate: number
  _messageTimestamps: number[]
  _messageTimestampHead: number
  _schemaKeys: string[]
  _schemaVariants: Map<string, number>
  _schemaConsistent: boolean
  _schemaDetected: boolean
  stats: ConnectionStats
  recordMessage(direction: MessageDirection, data: WebSocketMessageData | null): void
  recordSampledMessage(direction: MessageDirection, data: WebSocketMessageData | null): void
  shouldSample(direction: MessageDirection): boolean
  shouldLogLifecycle(): boolean
  getSamplingInfo(): SamplingInfo
  getMessageRate(): number
  setMessageRate(rate: number): void
  getSchema(): SchemaInfo
  isSchemaChange(data: string | null): boolean
}

// Cached TextEncoder instance to avoid per-call allocation in getSize() hot path
const _textEncoder: TextEncoder | null = typeof TextEncoder !== 'undefined' ? new TextEncoder() : null

// =============================================================================
// CAPTURE MODE STATE
// =============================================================================

let webSocketCaptureMode: WebSocketCaptureMode = 'medium'

/** Set the WebSocket capture mode */
export function setWebSocketCaptureModeInternal(mode: WebSocketCaptureMode): void {
  webSocketCaptureMode = mode
}

/** Get the current WebSocket capture mode */
export function getWebSocketCaptureModeInternal(): WebSocketCaptureMode {
  return webSocketCaptureMode
}

/** Reset capture mode to default (for testing) */
export function resetCaptureModeForTesting(): void {
  webSocketCaptureMode = 'medium'
}

// =============================================================================
// MESSAGE UTILITIES
// =============================================================================

/**
 * Get the byte size of a WebSocket message
 */
// #lizard forgives
export function getSize(data: WebSocketMessageData | SizedObject | null): number {
  if (typeof data === 'string') {
    return _textEncoder ? _textEncoder.encode(data).length : data.length
  }
  if (data instanceof ArrayBuffer) return data.byteLength
  if (data && typeof data === 'object' && 'size' in data) return (data as SizedObject).size
  return 0
}

/**
 * Format a WebSocket payload for logging
 */
export function formatPayload(data: WebSocketMessageData | null): string {
  if (typeof data === 'string') return data

  if (data instanceof ArrayBuffer) {
    const bytes = new Uint8Array(data)
    if (data.byteLength < 256) {
      // Small binary: hex preview
      let hex = ''
      for (let i = 0; i < bytes.length; i++) {
        const byte = bytes[i]
        if (byte !== undefined) {
          hex += byte.toString(16).padStart(2, '0')
        }
      }
      return `[Binary: ${data.byteLength}B] ${hex}`
    } else {
      // Large binary: size + magic bytes (first 4 bytes)
      let magic = ''
      for (let i = 0; i < Math.min(4, bytes.length); i++) {
        const byte = bytes[i]
        if (byte !== undefined) {
          magic += byte.toString(16).padStart(2, '0')
        }
      }
      return `[Binary: ${data.byteLength}B, magic:${magic}]`
    }
  }

  // Blob or Blob-like
  if (data && typeof data === 'object' && 'size' in data) {
    return `[Binary: ${(data as SizedObject).size}B]`
  }

  return String(data)
}

/**
 * Truncate a WebSocket message to the size limit
 */
// #lizard forgives
export function truncateWsMessage(message: string): TruncationResult {
  if (typeof message === 'string' && message.length > WS_MAX_BODY_SIZE) {
    return { data: message.slice(0, WS_MAX_BODY_SIZE), truncated: true }
  }
  return { data: message, truncated: false }
}

// =============================================================================
// CONNECTION TRACKER
// =============================================================================

/**
 * Create a connection tracker for adaptive sampling and schema detection
 */
// #lizard forgives
export function createConnectionTracker(id: string, url: string): ConnectionTracker {
  const tracker: ConnectionTracker = {
    id,
    url,
    messageCount: 0,
    _sampleCounter: 0,
    _messageRate: 0,
    _messageTimestamps: [],
    _messageTimestampHead: 0,
    _schemaKeys: [],
    _schemaVariants: new Map(),
    _schemaConsistent: true,
    _schemaDetected: false,

    stats: {
      incoming: { count: 0, bytes: 0, lastPreview: null, lastAt: null },
      outgoing: { count: 0, bytes: 0, lastPreview: null, lastAt: null }
    },

    /**
     * Record bounded message statistics before making the sampling decision.
     *
     * WEBSOCKET PAYLOAD SCHEMA INFERENCE LOGIC:
     *
     * This method implements a three-phase schema detection strategy to identify the
     * shape of JSON messages flowing over a WebSocket connection. Understanding the
     * schema is crucial for debugging: it reveals whether messages are uniform (good
     * for testing) or polymorphic (suggests different message types or errors).
     *
     * PHASE 1: BOOTSTRAP DETECTION (messages 1-5)
     *   Purpose: Quickly infer the "canonical" schema from the first JSON messages.
     *   Strategy:
     *     - Extract sorted object keys from each incoming JSON message
     *     - Stop after 5 messages (samples are enough to detect schema; balance between
     *       coverage and memory/CPU cost)
     *     - Compute consistency: if all 5 messages have identical key sets, mark as
     *       consistent=true
     *     - Store key strings as comma-separated sorted lists (e.g., "id,status,timestamp")
     *   Why 5: Statistically sufficient for most API patterns. First message might be
     *     special (connection ACK). By message 5, the pattern is clear.
     *   Early exit: If not JSON or message is array, skip (only track object schemas).
     *
     * PHASE 2: CONSISTENCY CHECKING (after first 2 messages)
     *   Trigger: Once _schemaKeys.length >= 2, begin checking if all keys match the first.
     *   Result: Sets _schemaConsistent = boolean indicating if messages have uniform schema.
     *   Why check early: Detect schema changes immediately without waiting for all 5 messages.
     *   Performance: O(n) single pass over _schemaKeys array; no redundant comparisons.
     *
     * PHASE 3: VARIANT TRACKING (messages 6+)
     *   Purpose: After bootstrap, track schema variants without resetting detection.
     *   Strategy:
     *     - Continue parsing incoming JSON messages after _schemaDetected = true
     *     - Build variants Map: key -> count (e.g., "id,status" -> 5 occurrences)
     *     - Memory bound: Cap Map at 50 entries. Only add new variants if under cap;
     *       always increment existing keys (ensures frequent patterns stay tracked).
     *     - This bounds memory to ~50KB even on long-lived connections.
     *   Why variants matter: Detects polymorphic message types (e.g., "id,status,data"
     *     vs "id,error,code"). Useful for debugging API versioning issues.
     *   Why cap variants: Long-running connections might emit hundreds of unique schemas.
     *     Capping prevents unbounded growth while keeping the 50 most frequent variants.
     *
     * SAMPLING RATE DECISION:
     *   The schema info (keys, consistency, variants) flows to getSchema() which returns:
     *     - detectedKeys: union of all seen keys (for understanding message structure)
     *     - consistent: boolean (true if all bootstrap messages matched)
     *     - variants: array of key strings (top variants seen after bootstrap)
     *   MCP observe handler uses this to emit SchemaInfo in WebSocket capture events,
     *   helping users understand payload patterns without logging every message.
     *
     * MESSAGE RATE TRACKING:
     *   Maintains an indexed queue for the last 5 seconds (sliding window). This powers
     *   shouldSample() which implements adaptive sampling: high-frequency connections
     *   (>200 msg/s) sample at 1-in-100; low-frequency (<2 msg/s) capture all messages.
     *   This ensures detailed visibility on slow links without bloating on high-volume.
     */
    recordMessage(direction: MessageDirection, data: WebSocketMessageData | null): void {
      this.messageCount++
      const size = data ? (typeof data === 'string' ? data.length : getSize(data)) : 0
      const now = Date.now()

      this.stats[direction].count++
      this.stats[direction].bytes += size
      this.stats[direction].lastAt = now

      if (data && typeof data === 'string') {
        this.stats[direction].lastPreview = data.length > WS_PREVIEW_LIMIT ? data.slice(0, WS_PREVIEW_LIMIT) : data
      }

      // Advance an indexed queue instead of filtering/copying the entire window.
      this._messageTimestamps.push(now)
      const cutoff = now - 5000
      while (
        this._messageTimestampHead < this._messageTimestamps.length &&
        (this._messageTimestamps[this._messageTimestampHead] ?? now) < cutoff
      ) {
        this._messageTimestampHead++
      }

      // Compact occasionally so long-lived connections remain memory bounded.
      // Each timestamp is copied at most once, keeping total accounting amortized O(1).
      if (this._messageTimestampHead >= 1024 && this._messageTimestampHead * 2 >= this._messageTimestamps.length) {
        this._messageTimestamps = this._messageTimestamps.slice(this._messageTimestampHead)
        this._messageTimestampHead = 0
      }
    },

    /**
     * Enrich a sampled message after the application callback has returned.
     */
    recordSampledMessage(direction: MessageDirection, data: WebSocketMessageData | null): void {
      // Schema detection from first 5 sampled incoming JSON messages
      if (direction === 'incoming' && data && typeof data === 'string' && this._schemaKeys.length < 5) {
        try {
          const parsed: unknown = JSON.parse(data)
          if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
            const keys = Object.keys(parsed as object).sort()
            const keyStr = keys.join(',')
            this._schemaKeys.push(keyStr)

            // Track variants
            this._schemaVariants.set(keyStr, (this._schemaVariants.get(keyStr) || 0) + 1)

            // Check consistency after 2+ messages
            if (this._schemaKeys.length >= 2) {
              const first = this._schemaKeys[0]
              this._schemaConsistent = this._schemaKeys.every((k) => k === first)
            }

            if (this._schemaKeys.length >= 5) {
              this._schemaDetected = true
            }
          }
        } catch {
          // EXPECTED_ABSENCE: page-owned access can normally throw for detached,
          // cross-origin, or hostile objects; logging it would misleadingly blame Kaboom for page behavior.
          // Not JSON, no schema
        }
      }

      // Track variants for messages beyond the first 5 (cap at 50 to bound memory)
      if (direction === 'incoming' && data && typeof data === 'string' && this._schemaDetected) {
        try {
          const parsed: unknown = JSON.parse(data)
          if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
            const keys = Object.keys(parsed as object).sort()
            const keyStr = keys.join(',')
            // Only add new variants if under cap; always increment existing
            if (this._schemaVariants.has(keyStr) || this._schemaVariants.size < 50) {
              this._schemaVariants.set(keyStr, (this._schemaVariants.get(keyStr) || 0) + 1)
            }
          }
        } catch {
          // EXPECTED_ABSENCE: page-owned access can normally throw for detached,
          // cross-origin, or hostile objects; logging it would misleadingly blame Kaboom for page behavior.
          // Not JSON
        }
      }
    },

    /**
     * Determine if a message should be sampled (logged)
     */
    shouldSample(_direction: MessageDirection): boolean {
      this._sampleCounter++

      // 'all' mode: no sampling
      if (webSocketCaptureMode === 'all') return true

      // Always log first 5 messages on a connection
      if (this.messageCount > 0 && this.messageCount <= 5) return true

      const rate = this._messageRate || this.getMessageRate()

      // Mode-based target caps:
      // 'high': ~10 msg/s, 'medium': ~5 msg/s, 'low': ~2 msg/s
      const targetRate = webSocketCaptureMode === 'high' ? 10 : webSocketCaptureMode === 'medium' ? 5 : 2

      if (rate <= targetRate) return true

      const n = Math.max(1, Math.round(rate / targetRate))
      return this._sampleCounter % n === 0
    },

    /**
     * Lifecycle events should always be logged
     */
    shouldLogLifecycle(): boolean {
      return true
    },

    /**
     * Get sampling info
     */
    getSamplingInfo(): SamplingInfo {
      const rate = this._messageRate || this.getMessageRate()
      let targetRate = rate
      if (rate >= 10 && rate < 50) targetRate = 10
      else if (rate >= 50 && rate < 200) targetRate = 5
      else if (rate >= 200) targetRate = 2

      return {
        rate: `${rate}/s`,
        logged: `${targetRate}/${Math.round(rate)}`,
        window: '5s'
      }
    },

    /**
     * Get the current message rate (messages per second)
     */
    getMessageRate(): number {
      const activeCount = this._messageTimestamps.length - this._messageTimestampHead
      if (activeCount < 2) return activeCount
      const lastTime = this._messageTimestamps[this._messageTimestamps.length - 1]
      const firstTime = this._messageTimestamps[this._messageTimestampHead]
      if (lastTime === undefined || firstTime === undefined) return activeCount
      const window = (lastTime - firstTime) / 1000
      return window > 0 ? activeCount / window : activeCount
    },

    /**
     * Set the message rate manually (for testing)
     */
    setMessageRate(rate: number): void {
      this._messageRate = rate
    },

    /**
     * Get the detected schema info
     */
    getSchema(): SchemaInfo {
      if (this._schemaKeys.length === 0) {
        return { detectedKeys: null, consistent: true }
      }

      // Get union of all detected keys
      const allKeys = new Set<string>()
      for (const keyStr of this._schemaKeys) {
        for (const k of keyStr.split(',')) {
          if (k) allKeys.add(k)
        }
      }

      // Build variants list
      const variants: string[] = []
      for (const [keyStr, count] of this._schemaVariants) {
        if (count > 0) variants.push(keyStr)
      }

      return {
        detectedKeys: allKeys.size > 0 ? Array.from(allKeys).sort() : null,
        consistent: this._schemaConsistent,
        variants: variants.length > 1 ? variants : undefined
      }
    },

    /**
     * Check if a message represents a schema change
     */
    isSchemaChange(data: string | null): boolean {
      if (!this._schemaDetected || !data || typeof data !== 'string') return false
      try {
        const parsed: unknown = JSON.parse(data)
        if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) return false
        const keys = Object.keys(parsed as object)
          .sort()
          .join(',')
        // It's a change if none of the first 5 schemas match
        return !this._schemaKeys.includes(keys)
      } catch {
        // EXPECTED_ABSENCE: non-serializable page payloads are expected here; logging would mislabel a skipped comparison.
        return false
      }
    }
  }

  return tracker
}
