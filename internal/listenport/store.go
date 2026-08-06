// store.go — Owns synchronized daemon HTTP listen-port state.
// Docs: docs/features/feature/mcp-persistent-server/index.md

package listenport

import (
	"sync/atomic"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/serverdefaults"
)

// Store owns the positive port used by HTTP URL and route composition.
type Store struct{ port atomic.Int64 }

// New returns a store initialized to the canonical daemon port.
func New() *Store {
	store := &Store{}
	store.port.Store(serverdefaults.Port)
	return store
}

// Set records a positive active listener port.
func (s *Store) Set(port int) {
	if port > 0 {
		s.port.Store(int64(port))
	}
}

// Get returns the active listener port.
func (s *Store) Get() int {
	port := int(s.port.Load())
	if port <= 0 {
		return serverdefaults.Port
	}
	return port
}
