// model.go — Capture's public compatibility model, local state types, and limits.
// Purpose: Keeps the package API and runtime constraints in one stable boundary.
// Why: Aliases, local model types, and their capacity constants change together when
// capture delegates another responsibility to a focused subsystem.
// Docs: docs/features/feature/backend-log-streaming/index.md

package capture

import (
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/circuit"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/performance"
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
)

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
