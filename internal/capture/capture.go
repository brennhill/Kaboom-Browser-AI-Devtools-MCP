// Purpose: Defines the Capture state container, its constructor, lifecycle and injected dependencies.
// Why: Centralizes all in-memory telemetry state so ingestion/query paths share one coherent source of truth.
// Docs: docs/features/feature/backend-log-streaming/index.md
// Docs: docs/features/feature/request-session-correlation/index.md
//
// Package capture owns the daemon's in-memory browser telemetry, extension state,
// query coordination, bounded event buffers, and recording integration.
package capture

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/clientstore"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/featureusage"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/logstore"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/perfstore"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/telemetrystore"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/circuit"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/lifecycle"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/recording"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/redaction"
)

// Capture manages all buffered browser state: WebSocket events, network bodies,
// user actions, connections, queries, rate limiting, and performance.
//
// Capture is a composition root. Each owner below synchronizes its own state;
// Capture itself has no shared mutex or independently mutable state.
//
// Independently synchronized telemetry owners retain each datum with its
// ingestion timestamp, eliminating parallel-array desynchronization. Their
// snapshots expose monotonic totals, bounded-retention pressure, and memory
// accounting without leaking mutable storage.
//
// Rate limiting uses a sliding 1-second window with circuit breaker (see internal/circuit):
// windowEventCount resets per window. rateLimitStreak tracks consecutive seconds over threshold.
// Circuit opens after 5 consecutive seconds over threshold; closes after 10s below threshold.
// lastBelowThresholdAt tracks when rate dropped below threshold (initialized at startup to prevent false close).
type Capture struct {
	// ============================================
	// Unified Telemetry Store (Own Lock)
	// ============================================

	telemetry *telemetrystore.Store // Event buffers, WebSocket connections, and navigation callback.

	// ============================================
	// Timings and Performance Data
	// ============================================

	extensionLogs *logstore.Extension // Bounded extension logs. Own lock, redaction, and retention.

	// ============================================
	// Query Dispatch (Own Locks)
	// ============================================

	queryDispatcher *queries.QueryDispatcher // Pending queries, results, async command tracking. Has own sync.Mutex + sync.RWMutex.

	// ============================================
	// Rate Limiting & Circuit Breaker (Own Lock)
	// ============================================

	circuit *circuit.CircuitBreaker // Rate limiting + circuit breaker state machine. Has own sync.RWMutex.

	// ============================================
	// Extension Runtime (Own Lock)
	// ============================================

	extension *ExtensionRuntime // Connection, pilot, tracking, CSP, and test boundaries. Independently synchronized.

	// ============================================
	// Diagnostic Logging (Own Lock)
	// ============================================

	diagnosticLogs *logstore.Diagnostic // Redacted polling + HTTP diagnostics. Independently synchronized.

	// recording.Recording Management — delegates to recording.RecordingManager sub-struct (aliased from internal/recording).
	recordingManager *recording.RecordingManager // recording.Recording lifecycle, playback, and log-diff. Has own sync.Mutex.

	// ============================================
	// Composed Sub-Structures
	// ============================================

	perf *perfstore.Store // Performance snapshots and action correlations. Own lock and retention.

	// ============================================
	// Multi-Client Support
	// ============================================

	clients *clientstore.Owner // Runtime client-registry wiring. Registry contents own their own lock.

	// ============================================
	// Lifecycle Event Callbacks
	// ============================================

	lifecycle    *lifecycle.Observer    // Typed event bus for lifecycle events (circuit breaker, extension state, buffer overflow). Has its own lock.
	featureUsage *featureusage.Observer // Optional extension feature-usage consumer. Independently synchronized.
}

// NewCapture creates a fully initialized Capture with all subcomponents wired.
//
// Invariants:
// - queryDispatcher/circuit/debug/recordingManager are non-nil in returned instance.
// - extension runtime sets and command-reconciliation maps start initialized.
func NewCapture() *Capture {
	logRedactor := redaction.NewRedactionEngine("")
	c := &Capture{
		extensionLogs:    logstore.NewExtension(logRedactor.Redact),
		extension:        newExtensionRuntime(),
		perf:             perfstore.New(),
		diagnosticLogs:   logstore.NewDiagnostic(logRedactor.Redact),
		recordingManager: recording.NewRecordingManager(),
		lifecycle:        lifecycle.NewObserver(),
		featureUsage:     featureusage.New(),
		clients:          clientstore.New(),
	}
	c.queryDispatcher = queries.NewQueryDispatcher()
	c.circuit = circuit.NewCircuitBreaker(c.lifecycle.Emit)
	c.telemetry = telemetrystore.New(telemetrystore.Dependencies{ActiveTestIDs: c.extension.GetActiveTestIDs})

	return c
}

// Queries returns the canonical independently synchronized query dispatcher.
func (c *Capture) Queries() *queries.QueryDispatcher {
	return c.queryDispatcher
}

// Recordings returns the canonical independently synchronized recording manager.
func (c *Capture) Recordings() *recording.RecordingManager {
	return c.recordingManager
}

// Circuit returns the canonical independently synchronized circuit breaker.
func (c *Capture) Circuit() *circuit.CircuitBreaker {
	return c.circuit
}

// Lifecycle returns the canonical independently synchronized lifecycle observer.
func (c *Capture) Lifecycle() *lifecycle.Observer {
	return c.lifecycle
}

// FeatureUsage returns the canonical independently synchronized usage observer.
func (c *Capture) FeatureUsage() *featureusage.Observer {
	return c.featureUsage
}

// Clients returns the canonical client-registry owner.
func (c *Capture) Clients() *clientstore.Owner {
	return c.clients
}

// Extension returns the canonical independently synchronized extension runtime.
func (c *Capture) Extension() *ExtensionRuntime {
	return c.extension
}

// Telemetry returns the canonical independently synchronized event store.
func (c *Capture) Telemetry() *telemetrystore.Store {
	return c.telemetry
}

// Performance returns the canonical independently synchronized performance owner.
func (c *Capture) Performance() *perfstore.Store { return c.perf }

// ExtensionLogs returns the independently synchronized extension-log owner.
func (c *Capture) ExtensionLogs() *logstore.Extension { return c.extensionLogs }

// DiagnosticLogs returns the canonical redacted diagnostic-log owner.
func (c *Capture) DiagnosticLogs() *logstore.Diagnostic { return c.diagnosticLogs }

// Close shuts down capture-owned background goroutines.
//
// Failure semantics:
// - Idempotent for query cleanup lifecycle; no panic on repeated calls.
// - Does not clear in-memory buffers.
func (c *Capture) Close() {
	if c.queryDispatcher != nil {
		c.queryDispatcher.Close()
	}
}
