// owner.go — Owns lazy annotation-store creation and deterministic shutdown.
// Docs: docs/features/feature/annotated-screenshots/index.md

package runtime

import (
	"sync"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/annotation"
)

// Owner serializes annotation store creation and shutdown.
type Owner struct {
	mu    sync.Mutex
	ttl   time.Duration
	store *annotation.Store
}

// New returns an annotation lifecycle owner.
func New(ttl time.Duration) *Owner { return &Owner{ttl: ttl} }

// Store returns the live store, creating it on first use or after shutdown.
func (o *Owner) Store() *annotation.Store {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.store == nil {
		o.store = annotation.NewStore(o.ttl)
	}
	return o.store
}

// Close detaches and closes the current store. It is safe to call repeatedly.
func (o *Owner) Close() {
	o.mu.Lock()
	store := o.store
	o.store = nil
	o.mu.Unlock()
	if store != nil {
		store.Close()
	}
}
