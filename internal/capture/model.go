// model.go — Capture-local state types and runtime limits.
// Purpose: Keeps capture-owned state containers and runtime constraints together.
// Why: These types describe capture storage, while wire contracts live in internal/types.
// Docs: docs/features/feature/backend-log-streaming/index.md

package capture

import (
	"sync"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/circuit"
)

const (
	MaxWSEvents        = 500
	MaxNetworkBodies   = 100
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

const (
	extensionDisconnectThreshold = 10 * time.Second
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

type ClientRegistry interface {
	Count() int
	List() any
	Register(cwd string) any
	Get(id string) any
	Unregister(id string) bool
}

// ClientRegistryOwner synchronizes replacement and retrieval of the runtime registry.
// The registry implementation owns synchronization for its own contents.
type ClientRegistryOwner struct {
	mu       sync.RWMutex
	registry ClientRegistry
}

func newClientRegistryOwner() *ClientRegistryOwner {
	return &ClientRegistryOwner{}
}

// Set installs the process-wide client registry during runtime composition.
func (o *ClientRegistryOwner) Set(registry ClientRegistry) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.registry = registry
}

// Registry returns the currently installed registry.
func (o *ClientRegistryOwner) Registry() ClientRegistry {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.registry
}
