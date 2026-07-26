// Purpose: Re-exports subsystem type aliases so capture can compose extracted packages without churn.
// Why: Preserves capture package API/readability during modularization into circuit/queries/recording packages.
// Docs: docs/features/feature/backend-log-streaming/index.md

package capture

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/circuit"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/performance"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/recording"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/recording/logdiff"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/recording/playback"
)

// Type aliases for imported packages to avoid qualifying every use.
// These are real type aliases (= syntax), not any forward declarations.
type (
	// Store is the preferred non-stuttering name for the package's primary state container.
	// Backward compatibility: Capture remains available as an alias target.
	Store = Capture
	// Snapshot is the preferred non-stuttering name for CaptureSnapshot.
	// Backward compatibility: CaptureSnapshot remains available as an alias target.
	Snapshot = CaptureSnapshot

	PerformanceSnapshot   = performance.Snapshot         // Alias for convenience (avoid qualifying as performance.Snapshot everywhere)
	PerformanceBaseline   = performance.Baseline         // Alias for convenience
	PerformanceRegression = performance.Regression       // Alias for convenience
	ResourceEntry         = performance.ResourceEntry    // Alias for convenience
	ResourceDiff          = performance.ResourceDiff     // Alias for convenience
	CausalDiffResult      = performance.CausalDiffResult // Alias for convenience
	Recording             = recording.Item               // Alias for convenience (avoid qualifying as recording.Item everywhere)
	RecordingAction       = recording.Action             // Alias for convenience
	PendingQueryResponse  = queries.PendingQueryResponse // Alias for convenience (avoid qualifying as queries.PendingQueryResponse everywhere)
	PendingQuery          = queries.PendingQuery         // Alias for convenience
	CommandResult         = queries.CommandResult        // Alias for convenience (avoid qualifying as queries.CommandResult everywhere)

	// QueryDispatcher subsystem types — moved to internal/queries package.
	QueryDispatcher = queries.QueryDispatcher // Query lifecycle, result storage, async command tracking
	QuerySnapshot   = queries.QuerySnapshot   // Point-in-time view of query state for health reporting

	// Circuit breaker subsystem types — moved to internal/circuit package.
	CircuitBreaker    = circuit.CircuitBreaker    // Rate limiting + circuit breaker state machine
	HealthResponse    = circuit.HealthResponse    // GET /health response
	RateLimitResponse = circuit.RateLimitResponse // 429 response body

	// Recording subsystem types — moved to internal/recording package.
	RecordingManager = recording.Manager     // Recording lifecycle, persistence, and storage quotas
	StorageInfo      = recording.StorageInfo // Recording storage usage info

	// Replay subsystem types — moved to internal/recording/playback package.
	PlaybackSession = playback.Session     // Active playback session state
	PlaybackResult  = playback.Result      // Result of executing a single recorded action
	Coordinates     = playback.Coordinates // X/Y position on the page

	// Log-diff subsystem types — moved to internal/recording/logdiff package.
	LogDiffResult    = logdiff.Result           // Comparison of two recordings
	DiffLogEntry     = logdiff.LogEntry         // Single log entry for diff comparison
	ValueChange      = logdiff.ValueChange      // Field value change between recordings
	ActionComparison = logdiff.ActionComparison // Action counts and types between recordings
)

// NewCircuitBreaker is re-exported from internal/circuit for backward compatibility.
var NewCircuitBreaker = circuit.NewCircuitBreaker

// Lifecycle and debug logger subsystem types are re-exported from:
// - internal/lifecycle (LifecycleObserver, LifecycleEvent, LifecycleListener) — see lifecycle_observer.go
// - internal/debuglog (DebugLogger) — see debug_logger.go

