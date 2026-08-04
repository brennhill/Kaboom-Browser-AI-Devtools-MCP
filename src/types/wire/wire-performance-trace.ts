// THIS FILE IS GENERATED — do not edit by hand.
// Source: internal/perftrace/wire_trace.go
// Generator: scripts/build/generate-wire-types.js

/**
 * @fileoverview Wire types for Chrome performance trace streaming — matches internal/perftrace/wire_trace.go
 *
 * Canonical TypeScript definitions for the local trace artifact lifecycle.
 * Changes here MUST be mirrored in the Go counterpart. Run `make check-wire-drift`.
 */

export interface WirePerformanceTraceStartRequest {
  readonly tab_id: number
}

export interface WirePerformanceTraceStartResponse {
  readonly trace_id: string
}

export interface WirePerformanceTraceChunkRequest {
  readonly trace_id: string
  readonly sequence: number
  readonly events: readonly unknown[]
}

export interface WirePerformanceTraceFinishRequest {
  readonly trace_id: string
}

export interface WirePerformanceTraceResult {
  readonly trace_id: string
  readonly artifact_path: string
  readonly event_count: number
  readonly chunk_count: number
  readonly bytes: number
}

export interface WirePerformanceTraceAbortRequest {
  readonly trace_id: string
}
