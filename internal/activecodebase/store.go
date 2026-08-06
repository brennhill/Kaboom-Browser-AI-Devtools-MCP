// store.go — Owns synchronized active-codebase workspace state.
// Docs: docs/features/feature/terminal/index.md

package activecodebase

import "sync"

// Store owns the workspace path shared by terminal and configuration features.
type Store struct {
	mu   sync.RWMutex
	path string
}

// New returns an empty workspace store.
func New() *Store { return &Store{} }

// GetActiveCodebase returns the current workspace path.
func (s *Store) GetActiveCodebase() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.path
}

// SetActiveCodebase replaces the current workspace path.
func (s *Store) SetActiveCodebase(path string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.path = path
}
