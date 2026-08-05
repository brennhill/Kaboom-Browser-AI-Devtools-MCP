// owner.go — Owns synchronized runtime client-registry installation.
// Docs: docs/features/feature/request-session-correlation/index.md

package clientstore

import "sync"

// Registry is the minimal runtime client-session boundary used by capture consumers.
type Registry interface {
	Count() int
	List() any
	Register(cwd string) any
	Get(id string) any
	Unregister(id string) bool
}

// Owner synchronizes registry replacement and retrieval. Registry implementations
// remain responsible for synchronizing their own contents.
type Owner struct {
	mu       sync.RWMutex
	registry Registry
}

// New constructs an owner without an installed registry.
func New() *Owner { return &Owner{} }

// Set installs the process-wide client registry during runtime composition.
func (o *Owner) Set(registry Registry) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.registry = registry
}

// Registry returns the currently installed registry.
func (o *Owner) Registry() Registry {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.registry
}
