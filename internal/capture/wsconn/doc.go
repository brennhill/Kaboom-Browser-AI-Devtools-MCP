// Purpose: Documents the wsconn package boundary and its externally held locking contract.
// Why: The telemetry owner serializes tracker mutations and snapshots.
// Docs: docs/features/feature/observe/index.md

// doc.go — Package documentation for WebSocket connection tracking.

// Package wsconn tracks live and recently-closed WebSocket connections seen in the
// browser telemetry stream, and projects them into the observe-tool status response.
//
// Concurrency contract: Tracker carries no lock of its own. TelemetryStore embeds it
// by value and calls every method while holding the telemetry owner's mutex.
//
// Bounds: at most maxActiveConns live connections (oldest-first eviction via connOrder)
// and maxClosedConns entries of closed-connection history (single-pass FIFO eviction).
package wsconn
