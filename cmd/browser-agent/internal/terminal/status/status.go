// status.go — Owns synchronized terminal availability and bind diagnostics.
// Docs: docs/features/feature/terminal/index.md

package status

import "sync"

// Snapshot is the immutable terminal health view.
type Snapshot struct {
	Available        bool   `json:"available"`
	Port             int    `json:"port"`
	Error            string `json:"error,omitempty"`
	BlockedByPID     int    `json:"blocked_by_pid,omitempty"`
	BlockedByCommand string `json:"blocked_by_command,omitempty"`
}

// Store owns the terminal server's current availability and last bind failure.
type Store struct {
	mu               sync.RWMutex
	port             int
	wantedPort       int
	available        bool
	errorMessage     string
	blockedByPID     int
	blockedByCommand string
}

// New returns an empty terminal status store.
func New() *Store { return &Store{} }

// SetPort records the listening port. Port zero marks the terminal unavailable.
func (s *Store) SetPort(port int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.port = port
	s.available = port > 0
	if port > 0 {
		s.errorMessage = ""
		s.blockedByPID = 0
		s.blockedByCommand = ""
	}
}

// SetUnavailable records an actionable bind failure.
func (s *Store) SetUnavailable(port int, reason string, blockingPID int, blockingCommand string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.port = 0
	s.available = false
	s.errorMessage = reason
	s.blockedByPID = blockingPID
	s.blockedByCommand = blockingCommand
	s.wantedPort = port
}

// Snapshot returns a consistent immutable view.
func (s *Store) Snapshot() Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	port := s.port
	if !s.available && s.wantedPort > 0 {
		port = s.wantedPort
	}
	return Snapshot{
		Available: s.available, Port: port, Error: s.errorMessage,
		BlockedByPID: s.blockedByPID, BlockedByCommand: s.blockedByCommand,
	}
}

// Port returns the active listening port, or zero when unavailable.
func (s *Store) Port() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.port
}
