// Purpose: Documents the capture package, its concurrency model, and its file layout.
// Why: Maintains stable server behavior across tool and transport paths.
// Docs: docs/features/feature/backend-log-streaming/index.md

// doc.go — Package documentation for browser telemetry capture and buffering.

// Package capture provides real-time browser telemetry capture and buffering.
//
// Core functionality includes:
//   - WebSocket event capture with connection lifecycle tracking
//   - Network request/response body capture with binary format detection
//   - User action capture (clicks, inputs, navigation) with multi-strategy selectors
//   - Performance timing data (PerformanceResourceTiming API)
//   - Console log capture with structured filtering
//   - Recording/playback for test generation and debugging
//
// The Capture type maintains ring buffers with configurable capacity, memory-based
// eviction, and TTL filtering. All methods are thread-safe using a single mutex.
//
// Memory management enforces soft/hard/critical limits to prevent unbounded growth:
//   - Soft (50MB): Evict 25% of oldest entries
//   - Hard (100MB): Evict 50% of oldest entries
//   - Critical (150MB): Clear all buffers, enter minimal mode
//
// # Layout
//
// Capture is a single state container guarded by one RWMutex, so every file declaring a
// *Capture method necessarily lives in this package. Files are grouped by the subsystem
// whose state they touch, not by the kind of code they contain:
//
//	doc.go, types.go, aliases.go, constants.go     declarations and re-exports
//	capture.go                                     Capture struct, constructor, lifecycle
//	buffer_store.go, buffer_clear.go               event ring buffers, eviction, memory accounting
//	accessors.go, store_views.go                   read-only projections of buffered state
//	websocket.go, network.go, enhanced_actions.go  per-signal ingestion and query paths
//	extension_state.go, sync.go, sync_state.go     extension connection/pilot state and the /sync loop
//	extension_logs.go, debug.go                    extension log buffer and debug/redaction paths
//	handlers.go, helpers.go, status.go, settings.go  HTTP entry points and shared request plumbing
//	query_dispatcher.go, recording_manager.go, lifecycle_observer.go, performance_store.go
//	                                               thin facades over extracted subsystems
//
// Subsystems owning state independent of Capture.mu live in their own packages and are
// re-exported from aliases.go: internal/capture/wsconn, internal/queries, internal/circuit,
// internal/recording, internal/lifecycle, internal/debuglog.
package capture
