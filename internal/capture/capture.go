// Purpose: Defines the Capture state container, its constructor, lifecycle and injected dependencies.
// Why: Centralizes all in-memory telemetry state so ingestion/query paths share one coherent source of truth.
// Docs: docs/features/feature/backend-log-streaming/index.md
// Docs: docs/features/feature/request-session-correlation/index.md
//
// Package capture owns the daemon's in-memory browser telemetry, extension state,
// query coordination, bounded event buffers, and recording integration.
package capture

import (
	"sync"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/wsconn"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/performance"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/redaction"
)

// Capture manages all buffered browser state: WebSocket events, network bodies,
// user actions, connections, queries, rate limiting, and performance.
//
// All fields are protected by mu (sync.RWMutex) unless noted otherwise.
// Lock hierarchy: Capture.mu is position 3 (after ClientRegistry, ClientState).
// Release locks before calling external callbacks. Use RLock() for read-only access.
// Sub-struct locks: perf and the buffer stores use parent mu.
//
// Ring buffers (wsEvents, networkBodies, enhancedActions) use entry wrapper structs that
// bundle each datum with its ingestion timestamp, eliminating parallel-array desync risk:
// 1. Each entry carries its own AddedAt timestamp (wsEventEntry, networkBodyEntry, enhancedActionEntry)
// 2. Monotonic counters that survive eviction (wsTotalAdded, networkTotalAdded, actionTotalAdded)
// 3. Memory totals that estimate buffer overhead (wsMemoryTotal, networkBodyMemoryTotal)
//
// Rate limiting uses a sliding 1-second window with circuit breaker (see internal/circuit):
// windowEventCount resets per window. rateLimitStreak tracks consecutive seconds over threshold.
// Circuit opens after 5 consecutive seconds over threshold; closes after 10s below threshold.
// lastBelowThresholdAt tracks when rate dropped below threshold (initialized at startup to prevent false close).
type Capture struct {
	mu sync.RWMutex

	// TTL for read-time filtering (0 = unlimited, no filtering applied).
	// Applied during reads: events older than TTL are skipped.
	TTL time.Duration

	// ============================================
	// Unified Event Buffer Store (ring buffers)
	// ============================================

	buffers BufferStore // ws/network/action buffers + counters + memory totals (protected by Capture.mu).

	// ============================================
	// Timings and Performance Data
	// ============================================

	networkWaterfall NetworkWaterfallBuffer // Ring buffer of browser PerformanceResourceTiming data (configurable capacity, default 1000).
	extensionLogs    ExtensionLogBuffer     // Ring buffer of extension internal logs (max 500). FIFO eviction. No TTL filtering.

	// ============================================
	// WebSocket Connection Tracking
	// ============================================

	wsConnections wsconn.Tracker // Active + closed WS connections, LRU eviction order. Protected by parent mu (no separate lock).

	// ============================================
	// Query Dispatch (Own Locks)
	// ============================================

	queryDispatcher *QueryDispatcher // Pending queries, results, async command tracking — delegates to QueryDispatcher sub-struct (aliased from internal/queries). Has own sync.Mutex + sync.RWMutex — independent of Capture.mu.

	// ============================================
	// Rate Limiting & Circuit Breaker (Own Lock)
	// ============================================

	circuit *CircuitBreaker // Rate limiting + circuit breaker state machine — delegates to internal/circuit. Has own sync.RWMutex — independent of Capture.mu.

	// ============================================
	// Extension State (Protected by parent mu)
	// ============================================

	extensionState ExtensionState // Connection, pilot, tracking, test boundaries. Protected by parent mu (no separate lock).

	// ============================================
	// Debug Logging (Own Lock)
	// ============================================

	debug DebugLogger // Polling activity + HTTP debug circular buffers. Has own sync.Mutex — independent of Capture.mu. Delegates to internal/debuglog.

	// Redaction engine for scrubbing sensitive values from extension debug logs.
	logRedactor *redaction.Engine

	// Recording Management — delegates to RecordingManager sub-struct (aliased from internal/recording).
	recordingManager *RecordingManager // Recording lifecycle, playback, and log-diff. Has own sync.Mutex — independent of Capture.mu.

	// ============================================
	// Composed Sub-Structures
	// ============================================

	perf PerformanceStore // Performance snapshots and baselines. Protected by parent mu (no separate lock).

	// ============================================
	// Multi-Client Support
	// ============================================

	clientRegistry ClientRegistry // Registry of connected MCP clients. HAS OWN LOCK. Lock hierarchy: ClientRegistry.mu is position 1 (outermost), before Capture.mu.

	// ============================================
	// Lifecycle Event Callbacks
	// ============================================

	lifecycle          *LifecycleObserver    // Typed event bus for lifecycle events (circuit breaker, extension state, buffer overflow). Has own lock — independent of Capture.mu. Delegates to internal/lifecycle.
	navigationCallback func()                // Optional callback fired after a navigation action is ingested (called outside lock)
	featuresCallback   func(map[string]bool) // Optional callback fired when extension reports feature usage (called outside lock)

	// ============================================
	// Version Information
	// ============================================

	serverVersion string // Server version (e.g., "5.7.0"), set via SetServerVersion()
}

// NewCapture creates a fully initialized Capture with all subcomponents wired.
//
// Invariants:
// - queryDispatcher/circuit/debug/recordingManager are non-nil in returned instance.
// - extensionState.activeTestIDs and extensionState.missingInProgressByCorr start as initialized maps.
func NewCapture() *Capture {
	c := &Capture{
		buffers: newBufferStore(),
		networkWaterfall: NetworkWaterfallBuffer{
			entries:  make([]NetworkWaterfallEntry, 0, DefaultNetworkWaterfallCapacity),
			capacity: DefaultNetworkWaterfallCapacity,
		},
		extensionLogs: ExtensionLogBuffer{
			logs: make([]ExtensionLog, 0, MaxExtensionLogs),
		},
		wsConnections: wsconn.NewTracker(),
		extensionState: ExtensionState{
			activeTestIDs:           make(map[string]bool),
			missingInProgressByCorr: make(map[string]int),
			pilotSource:             PilotSourceAssumedStartup,
			securityMode:            SecurityModeNormal,
		},
		perf: PerformanceStore{
			snapshots:       make(map[string]performance.Snapshot),
			snapshotOrder:   make([]string, 0),
			baselines:       make(map[string]performance.Baseline),
			baselineOrder:   make([]string, 0),
			beforeSnapshots: make(map[string]performance.Snapshot),
		},
		debug:            NewDebugLogger(),
		recordingManager: NewRecordingManager(),

		logRedactor: redaction.NewRedactionEngine(""),
		lifecycle:   NewLifecycleObserver(),
	}
	c.queryDispatcher = NewQueryDispatcher()
	c.circuit = NewCircuitBreaker(c.lifecycle.EmitFunc())

	// Note: clientRegistry is initialized by capture.New() in capture package
	// to avoid circular import (those packages import capture for NetworkBody, WebSocketEvent, etc.)
	return c
}

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

// SetNavigationCallback sets a callback function that fires after a navigation
// action is ingested. The callback is invoked outside of the Capture lock in a
// separate goroutine (via util.SafeGo) so it is safe to call Capture methods.
// Used for automatic noise detection after page navigations.
func (c *Capture) SetNavigationCallback(cb func()) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.navigationCallback = cb
}

// SetFeaturesCallback sets a callback for extension feature usage reports.
// Called from HandleSync when features_used is present. Invoked outside Capture lock.
func (c *Capture) SetFeaturesCallback(cb func(map[string]bool)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.featuresCallback = cb
}

// SubscribeLifecycle registers a typed lifecycle event listener and returns a
// subscription ID for later removal via UnsubscribeLifecycle.
// Thread-safe; the observer has its own lock independent of Capture.mu.
func (c *Capture) SubscribeLifecycle(fn LifecycleListener) int {
	return c.lifecycle.Subscribe(fn)
}

// UnsubscribeLifecycle removes a lifecycle listener by its subscription ID.
// No-op if the ID is not found.
func (c *Capture) UnsubscribeLifecycle(id int) {
	c.lifecycle.Unsubscribe(id)
}

// SetLifecycleCallback registers a string-based lifecycle callback.
// Backward-compatible: wraps the callback as a LifecycleListener on the observer.
// Note: does NOT clear previous listeners. Callers that need exclusive ownership
// should use SubscribeLifecycle/UnsubscribeLifecycle directly.
func (c *Capture) SetLifecycleCallback(cb func(event string, data map[string]any)) {
	c.lifecycle.Subscribe(func(event LifecycleEvent, data map[string]any) {
		cb(event.String(), data)
	})
}

// AddLifecycleCallback appends a string-based lifecycle callback.
// Backward-compatible: wraps the callback as a LifecycleListener on the observer.
// Deprecated: prefer SubscribeLifecycle for new code.
func (c *Capture) AddLifecycleCallback(cb func(event string, data map[string]any)) {
	c.lifecycle.Subscribe(func(event LifecycleEvent, data map[string]any) {
		cb(event.String(), data)
	})
}

// emitLifecycleEvent dispatches a lifecycle event via the observer.
// Backward-compatible bridge: converts string event name to typed event.
//
// Failure semantics:
// - No listeners is a silent no-op.
// - Individual listener panics are recovered (error isolation).
func (c *Capture) emitLifecycleEvent(event string, data map[string]any) {
	c.lifecycle.EmitString(event, data)
}

// SetServerVersion sets server version for compatibility checking.
// Called once at startup with version from main.go.
func (c *Capture) SetServerVersion(v string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.serverVersion = v
}

// GetServerVersion returns server version.
func (c *Capture) GetServerVersion() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.serverVersion
}

// SetClientRegistry wires the client registry used by /clients endpoints.
// Safe to call at startup before serving requests.
func (c *Capture) SetClientRegistry(reg ClientRegistry) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.clientRegistry = reg
}
