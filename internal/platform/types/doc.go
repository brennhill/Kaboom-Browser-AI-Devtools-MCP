// Purpose: Implement doc.go internal behavior used by MCP runtime features.
// Why: Maintains stable server behavior across tool and transport paths.
// Docs: docs/features/feature/normalized-event-schema/index.md

// doc.go — Package documentation for foundational cross-cutting types.

// Package types provides the foundational, zero-dependency types for Kaboom.
//
// This package contains all cross-cutting type definitions needed by multiple packages:
//   - Protocol types (JSON-RPC, MCP protocol)
//   - Network and browser telemetry types (WebSocket, HTTP, actions)
//   - Alert types (immediate alerts, CI results, performance regressions)
//   - Logging types (server logs, extension logs, polling logs)
//   - Security types (threat flags)
//   - Buffer management types (cursors, clear counts)
//
// Design Principle: Zero Dependencies
// This package imports only the Go standard library. It is safe to import from
// any other package without creating circular dependencies. All other packages
// should import from types for canonical type definitions.
//
// Architecture Layer: Foundation
// types is the foundation layer in a 4-layer architecture:
//
//	Layer 1: types (zero deps) ← YOU ARE HERE
//	Layer 2: Domain packages (capture, ai, security, session, etc.)
//	Layer 3: Composite packages (analysis, observation)
//	Layer 4: Wiring (cmd/browser-agent main, integration tests)
//
// This layering ensures dependency flows only downward, preventing circular imports.
package types
