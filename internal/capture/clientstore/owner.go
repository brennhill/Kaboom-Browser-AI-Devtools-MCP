// owner.go — Owns synchronized runtime client-registry installation.
// Docs: docs/features/feature/request-session-correlation/index.md

package clientstore

import (
	"sync"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/session/clientreg"
)

// Owner synchronizes registry replacement and retrieval. Registry implementations
// remain responsible for synchronizing their own contents.
type Owner struct {
	mu       sync.RWMutex
	registry *clientreg.ClientRegistry
}

// New constructs an owner without an installed registry.
func New() *Owner { return &Owner{} }

// Set installs the process-wide client registry during runtime composition.
func (o *Owner) Set(registry *clientreg.ClientRegistry) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.registry = registry
}

// Registry returns the currently installed registry.
func (o *Owner) Registry() *clientreg.ClientRegistry {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.registry
}
