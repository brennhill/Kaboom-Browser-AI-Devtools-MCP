// Purpose: Documents the wsconn package boundary and its externally-held locking contract.
// Why: Makes the "caller owns the mutex" invariant explicit now that the tracker lives outside capture.
// Docs: docs/features/feature/observe/index.md

// doc.go — Package documentation for WebSocket connection tracking.

// Package wsconn tracks live and recently-closed WebSocket connections seen in the
// browser telemetry stream, and projects them into the observe-tool status response.
//
// Concurrency contract: Tracker carries NO lock of its own. It is embedded by value in
// capture.Capture and every method must be called with Capture.mu held — write lock for
// TrackEvent/Clear, read lock for Status/Count. This is the same contract the type had
// before extraction, when it was an unexported-field struct inside package capture.
//
// Bounds: at most maxActiveConns live connections (oldest-first eviction via connOrder)
// and maxClosedConns entries of closed-connection history (single-pass FIFO eviction).
package wsconn
