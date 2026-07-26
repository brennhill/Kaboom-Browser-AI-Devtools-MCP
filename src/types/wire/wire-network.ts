// THIS FILE IS GENERATED — do not edit by hand.
// Source: internal/types/wire_network.go
// Generator: scripts/generate-wire-types.js

/**
 * @fileoverview Wire types for network telemetry — matches internal/types/wire_network.go
 *
 * Canonical TypeScript definitions for NetworkBody and NetworkWaterfall HTTP payloads.
 * Changes here MUST be mirrored in the Go counterpart. Run `make check-wire-drift`.
 */

/**
 * WireNetworkBody is the JSON shape for captured network request/response bodies.
 */
export interface WireNetworkBody {
  readonly method: string
  readonly url: string
  readonly status: number
  readonly request_body?: string
  readonly response_body?: string
  readonly content_type?: string
  readonly duration?: number
  readonly request_truncated?: boolean
  readonly response_truncated?: boolean
  readonly tab_id?: number
  // server-only: ts — server-side timestamp
  // server-only: response_headers, has_auth_header, binary_format, format_confidence, test_ids
}

/**
 * WireNetworkWaterfallEntry is the JSON shape for a single PerformanceResourceTiming entry.
 */
export interface WireNetworkWaterfallEntry {
  readonly name: string
  readonly url: string
  readonly initiator_type: string
  readonly duration: number
  readonly start_time: number
  readonly fetch_start?: number
  readonly response_end?: number
  readonly transfer_size: number
  readonly decoded_body_size: number
  readonly encoded_body_size: number
  readonly page_url?: string
}

/**
 * WireNetworkWaterfallPayload is the top-level shape POSTed to /network-waterfall.
 */
export interface WireNetworkWaterfallPayload {
  readonly entries: readonly WireNetworkWaterfallEntry[]
  readonly page_url: string
}
