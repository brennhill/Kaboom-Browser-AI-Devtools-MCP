// network_recording_state.go — In-memory state container for network recording lifecycle.
// Why: Keeps concurrency-safe start/stop/status state transitions separate from tool handler orchestration.

package configurehandler

import (
	"sync"
	"time"
)

// RecordingSnapshot holds the captured state from a recording session.
type RecordingSnapshot struct {
	Active    bool
	StartTime time.Time
	Domain    string
	Method    string
}

// NetworkRecordingState tracks an active network recording session.
type NetworkRecordingState struct {
	mu        sync.Mutex
	active    bool
	startTime time.Time
	domain    string // optional domain filter
	method    string // optional HTTP method filter
}

// TryStart atomically checks if recording is inactive and starts it.
// Returns (startTime, true) on success, or (zero, false) if already active.
func (s *NetworkRecordingState) TryStart(domain, method string) (time.Time, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active {
		return time.Time{}, false
	}
	s.active = true
	s.startTime = time.Now()
	s.domain = domain
	s.method = method
	return s.startTime, true
}

// Stop atomically stops recording and returns a snapshot of the session state.
// Returns (snapshot, true) if recording was active, or (zero, false) if not.
func (s *NetworkRecordingState) Stop() (RecordingSnapshot, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.active {
		return RecordingSnapshot{}, false
	}
	snap := RecordingSnapshot{
		Active:    true,
		StartTime: s.startTime,
		Domain:    s.domain,
		Method:    s.method,
	}
	s.active = false
	s.startTime = time.Time{}
	s.domain = ""
	s.method = ""
	return snap, true
}

// Info returns a snapshot of the current recording state.
func (s *NetworkRecordingState) Info() RecordingSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	return RecordingSnapshot{
		Active:    s.active,
		StartTime: s.startTime,
		Domain:    s.domain,
		Method:    s.method,
	}
}
