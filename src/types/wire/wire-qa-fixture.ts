// THIS FILE IS GENERATED — do not edit by hand.
// Source: internal/qafixture/wire_fixture.go
// Generator: scripts/build/generate-wire-types.js

/**
 * @fileoverview Wire types for deterministic QA fixtures — matches internal/qafixture/wire_fixture.go
 *
 * Canonical TypeScript definitions for the versioned QA fixture command payload.
 * Changes here MUST be mirrored in the Go counterpart. Run `make check-wire-drift`.
 */

export interface WireQAFixture {
  readonly version: number
  readonly target?: WireQATarget
  readonly viewport?: WireQAViewport
  readonly locale?: string
  readonly permissions?: readonly string[]
  readonly network?: WireQANetwork
  readonly cookies?: readonly WireQACookie[]
  readonly local_storage?: Readonly<Record<string, string>>
  readonly session_storage?: Readonly<Record<string, string>>
  readonly feature_flags?: Readonly<Record<string, boolean>>
  readonly seed_data?: Readonly<Record<string, unknown>>
  readonly user_state?: string
  readonly auth_role?: string
  readonly setup_timeout_ms?: number
}

export interface WireQATarget {
  readonly url?: string
}

export interface WireQAViewport {
  readonly width?: number
  readonly height?: number
}

export interface WireQANetwork {
  readonly profile?: string
}

export interface WireQACookie {
  readonly name: string
  readonly value: string
  readonly domain?: string
  readonly path?: string
  readonly secure?: boolean
  readonly http_only?: boolean
  readonly same_site?: string
}
