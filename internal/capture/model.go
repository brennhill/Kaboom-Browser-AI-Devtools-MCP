// model.go — Capture-local state types and runtime limits.
// Purpose: Keeps capture-owned state containers and runtime constraints together.
// Why: These types describe capture storage, while wire contracts live in internal/types.
// Docs: docs/features/feature/backend-log-streaming/index.md

package capture

import (
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/circuit"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/performance"
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

type ClientRegistry interface {
	Count() int
	List() any
	Register(cwd string) any
	Get(id string) any
	Unregister(id string) bool
}
