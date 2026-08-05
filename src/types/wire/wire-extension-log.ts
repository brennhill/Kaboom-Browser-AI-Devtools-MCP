// THIS FILE IS GENERATED — do not edit by hand.
// Source: internal/types/wire_log.go
// Generator: scripts/build/generate-wire-types.js

/**
 * @fileoverview Wire type for extension diagnostics — matches internal/types/wire_log.go
 *
 * Canonical TypeScript definition for redacted extension diagnostics sent through sync.
 * Changes here MUST be mirrored in the Go counterpart. Run `make check-wire-drift`.
 */

/**
 * ExtensionLog is one redacted local extension diagnostic sent over /sync.
 */
export interface ExtensionLog {
  readonly timestamp: string
  readonly level: string
  readonly message: string
  readonly source: string
  readonly category?: string
  readonly data?: unknown
}
