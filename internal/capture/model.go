// model.go — Capture's public compatibility model, local state types, and limits.
// Purpose: Keeps the package API and runtime constraints in one stable boundary.
// Why: Aliases, local model types, and their capacity constants change together when
// capture delegates another responsibility to a focused subsystem.
// Docs: docs/features/feature/backend-log-streaming/index.md

package capture

import (
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/wsconn"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/circuit"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/debuglog"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/lifecycle"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/performance"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/recording"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/recording/logdiff"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/recording/playback"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

// ============================================
// Canonical wire types — declared in internal/types
// ============================================

// WebSocketEvent is an alias to canonical definition in internal/types/network.go
type WebSocketEvent = types.WebSocketEvent

// SamplingInfo is an alias to canonical definition in internal/types/network.go
type SamplingInfo = types.SamplingInfo

// WebSocketEventFilter is an alias to canonical definition in internal/types/network.go
type WebSocketEventFilter = types.WebSocketEventFilter

// WebSocketStatusFilter is an alias to canonical definition in internal/types/network.go
type WebSocketStatusFilter = types.WebSocketStatusFilter

// WebSocketStatusResponse is an alias to canonical definition in internal/types/network.go
type WebSocketStatusResponse = types.WebSocketStatusResponse

// WebSocketConnection is an alias to canonical definition in internal/types/network.go
type WebSocketConnection = types.WebSocketConnection

// WebSocketClosedConnection is an alias to canonical definition in internal/types/network.go
type WebSocketClosedConnection = types.WebSocketClosedConnection

// WebSocketMessageRate is an alias to canonical definition in internal/types/network.go
type WebSocketMessageRate = types.WebSocketMessageRate

// WebSocketDirectionStats is an alias to canonical definition in internal/types/network.go
type WebSocketDirectionStats = types.WebSocketDirectionStats

// WebSocketLastMessage is an alias to canonical definition in internal/types/network.go
type WebSocketLastMessage = types.WebSocketLastMessage

// WebSocketMessagePreview is an alias to canonical definition in internal/types/network.go
type WebSocketMessagePreview = types.WebSocketMessagePreview

// WebSocketSchema is an alias to canonical definition in internal/types/network.go
type WebSocketSchema = types.WebSocketSchema

// WebSocketSamplingStatus is an alias to canonical definition in internal/types/network.go
type WebSocketSamplingStatus = types.WebSocketSamplingStatus

// NetworkWaterfallEntry is an alias to canonical definition in internal/types/network.go
type NetworkWaterfallEntry = types.NetworkWaterfallEntry

// NetworkWaterfallPayload is an alias to canonical definition in internal/types/network.go
type NetworkWaterfallPayload = types.NetworkWaterfallPayload

// NetworkBody is an alias to canonical definition in internal/types/network.go
type NetworkBody = types.NetworkBody

// NetworkBodyFilter is an alias to canonical definition in internal/types/network.go
type NetworkBodyFilter = types.NetworkBodyFilter

// EnhancedAction is an alias to canonical definition in internal/types/network.go
type EnhancedAction = types.EnhancedAction

// EnhancedActionFilter is an alias to canonical definition in internal/types/network.go
type EnhancedActionFilter = types.EnhancedActionFilter

// ExtensionLog is an alias to canonical definition in internal/types/log.go
type ExtensionLog = types.ExtensionLog

// PollingLogEntry is an alias to canonical definition in internal/types/log.go
type PollingLogEntry = types.PollingLogEntry

// HTTPDebugEntry is an alias to canonical definition in internal/types/log.go
type HTTPDebugEntry = types.HTTPDebugEntry

// Type aliases for imported packages to avoid qualifying every use.
// These are real type aliases (= syntax), not any forward declarations.
type (
	// Store is the preferred non-stuttering name for the package's primary state container.
	// Backward compatibility: Capture remains available as an alias target.
	Store = Capture
	// Snapshot is the preferred non-stuttering name for CaptureSnapshot.
	// Backward compatibility: CaptureSnapshot remains available as an alias target.
	Snapshot = CaptureSnapshot

	ResourceEntry        = performance.ResourceEntry    // Alias for convenience
	ResourceDiff         = performance.ResourceDiff     // Alias for convenience
	CausalDiffResult     = performance.CausalDiffResult // Alias for convenience
	PendingQueryResponse = queries.PendingQueryResponse // Alias for convenience (avoid qualifying as queries.PendingQueryResponse everywhere)
	PendingQuery         = queries.PendingQuery         // Alias for convenience
	CommandResult        = queries.CommandResult        // Alias for convenience (avoid qualifying as queries.CommandResult everywhere)

	// QueryDispatcher subsystem types — moved to internal/queries package.
	QueryDispatcher = queries.QueryDispatcher // Query lifecycle, result storage, async command tracking
	QuerySnapshot   = queries.QuerySnapshot   // Point-in-time view of query state for health reporting

	// Circuit breaker subsystem types — moved to internal/circuit package.
	CircuitBreaker    = circuit.CircuitBreaker    // Rate limiting + circuit breaker state machine
	HealthResponse    = circuit.HealthResponse    // GET /health response
	RateLimitResponse = circuit.RateLimitResponse // 429 response body

	// WebSocket connection tracking — moved to internal/capture/wsconn package.
	WSConnectionTracker = wsconn.Tracker // Active + closed WS connections, LRU eviction order. Guarded by Capture.mu.

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

// Debug logger subsystem types — moved to internal/debuglog package.

// DebugLogger is an alias to the canonical type in internal/debuglog.
type DebugLogger = debuglog.Logger

// NewDebugLogger re-exports debuglog.NewLogger for backward compatibility.
var NewDebugLogger = debuglog.NewLogger

const debugLogSize = debuglog.LogSize

const (
	queryResultTTL = queries.QueryResultTTL // Re-export for queries_lifecycle_test.go
)

// Constants re-exported as unexported for capture-package test compatibility.
const (
	recordingStorageMax   = recording.RecordingStorageMax
	recordingWarningLevel = recording.RecordingWarningLevel
)

// Function re-exports for capture-package test compatibility.
var (
	validateRecordingID    = recording.ValidateRecordingID
	calculateRecordingSize = recording.CalculateRecordingSize
)

// Type aliases for backward compatibility — all types now live in internal/lifecycle.
type (
	LifecycleEvent    = lifecycle.Event
	LifecycleListener = lifecycle.Listener
	LifecycleObserver = lifecycle.Observer
)

// Event constant aliases for backward compatibility.
const (
	EventUnknown               = lifecycle.EventUnknown
	EventCircuitOpened         = lifecycle.EventCircuitOpened
	EventCircuitClosed         = lifecycle.EventCircuitClosed
	EventExtensionConnected    = lifecycle.EventExtensionConnected
	EventExtensionDisconnected = lifecycle.EventExtensionDisconnected
	EventBufferEviction        = lifecycle.EventBufferEviction
	EventRateLimitTriggered    = lifecycle.EventRateLimitTriggered
	EventCommandStateDesync    = lifecycle.EventCommandStateDesync
	EventSyncSnapshot          = lifecycle.EventSyncSnapshot
)

// NewLifecycleObserver re-exports lifecycle.NewObserver for backward compatibility.
var NewLifecycleObserver = lifecycle.NewObserver

// ParseLifecycleEvent re-exports lifecycle.ParseEvent for backward compatibility.
var ParseLifecycleEvent = lifecycle.ParseEvent

const (
	MaxWSEvents        = 500
	MaxNetworkBodies   = 100
	MaxExtensionLogs   = 500
	MaxEnhancedActions = 1000

	RateLimitThreshold = circuit.RateLimitThreshold

	DefaultNetworkWaterfallCapacity = 1000
	MinNetworkWaterfallCapacity     = 100
	MaxNetworkWaterfallCapacity     = 10000

	defaultWSLimit       = 50
	defaultBodyLimit     = 20
	maxExtensionPostBody = 5 << 20
	maxRequestBodySize   = 8192
	maxResponseBodySize  = 16384
	wsBufferMemoryLimit  = 4 * 1024 * 1024
	nbBufferMemoryLimit  = 8 * 1024 * 1024
)

const ExtensionReadinessTimeout = 5 * time.Second

const extensionReadinessPollInterval = 200 * time.Millisecond

var (
	extensionDisconnectThreshold = 10 * time.Second
	readinessGatePollInterval    = 100 * time.Millisecond
)

type SecurityFlag struct {
	Type      string    `json:"type"`
	Severity  string    `json:"severity"`
	Origin    string    `json:"origin"`
	Message   string    `json:"message"`
	Resource  string    `json:"resource"`
	PageURL   string    `json:"page_url"`
	Timestamp time.Time `json:"timestamp"`
}

type PerformanceStore struct {
	snapshots       map[string]performance.PerformanceSnapshot
	snapshotOrder   []string
	baselines       map[string]performance.PerformanceBaseline
	baselineOrder   []string
	beforeSnapshots map[string]performance.PerformanceSnapshot
}

type NetworkWaterfallBuffer struct {
	entries  []NetworkWaterfallEntry
	capacity int
}

type ExtensionLogBuffer struct {
	logs []ExtensionLog
}

type ClientRegistry interface {
	Count() int
	List() any
	Register(cwd string) any
	Get(id string) any
	Unregister(id string) bool
}
