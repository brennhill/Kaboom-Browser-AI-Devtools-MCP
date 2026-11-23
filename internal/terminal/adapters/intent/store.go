// intent_store.go -- In-memory store for user-initiated intents (e.g., "Find Problems" button).
// Why: Bridges the UI trigger to the AI session — the intent persists until the AI picks it up.
// Docs: docs/features/feature/auto-fix/index.md

package intent

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"sync/atomic"
	"time"
)

const (
	TTL      = 5 * time.Minute
	MaxCount = 3
	// MaxNudges is the number of tool responses to nudge before giving up and discarding.
	MaxNudges    = 3
	ActionQAScan = "qa_scan"
)

// Intent represents a user-initiated action request.
type Intent struct {
	CorrelationID string `json:"correlation_id"`
	PageURL       string `json:"page_url"`
	Action        string `json:"action"`
	CreatedAt     int64  `json:"created_at"`
	NudgeCount    int    `json:"-"`
}

// Store is a thread-safe in-memory store for user intents.
type Store struct {
	mu    sync.Mutex
	items []Intent
	count atomic.Int32 // Fast-path: skip lock when empty
}

// NewStore creates a new intent store.
func NewStore() *Store {
	return &Store{}
}

// Add creates a new intent and returns its correlation ID.
func (s *Store) Add(pageURL, action string) string {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cleanExpiredLocked()

	for len(s.items) >= MaxCount {
		s.items = s.items[1:]
	}

	id := GenerateCorrelationID()
	s.items = append(s.items, Intent{
		CorrelationID: id,
		PageURL:       pageURL,
		Action:        action,
		CreatedAt:     time.Now().Unix(),
	})
	s.syncCountLocked()
	return id
}

// Consume removes and returns the intent with the given correlation ID.
func (s *Store) Consume(correlationID string) *Intent {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cleanExpiredLocked()

	for i, it := range s.items {
		if it.CorrelationID == correlationID {
			s.items = append(s.items[:i], s.items[i+1:]...)
			s.syncCountLocked()
			return &it
		}
	}
	return nil
}

// Pending returns all non-expired intents without consuming them.
func (s *Store) Pending() []Intent {
	if s.count.Load() == 0 {
		return []Intent{}
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cleanExpiredLocked()
	out := make([]Intent, len(s.items))
	copy(out, s.items)
	return out
}

// NudgeAndClean increments the nudge count on all pending intents and removes
// any that have exceeded MaxNudges. Returns true if there are still
// pending intents that should be surfaced to the AI.
func (s *Store) NudgeAndClean() bool {
	if s.count.Load() == 0 {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cleanExpiredLocked()

	n := 0
	for i := range s.items {
		s.items[i].NudgeCount++
		if s.items[i].NudgeCount <= MaxNudges {
			s.items[n] = s.items[i]
			n++
		}
	}
	s.items = s.items[:n]
	s.syncCountLocked()
	return n > 0
}

// ConsumeAll removes and returns all non-expired intents.
func (s *Store) ConsumeAll() []Intent {
	if s.count.Load() == 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]Intent, len(s.items))
	copy(out, s.items)
	s.items = s.items[:0]
	s.count.Store(0)
	return out
}

func (s *Store) cleanExpiredLocked() {
	now := time.Now().Unix()
	cutoff := now - int64(TTL.Seconds())
	n := 0
	for _, it := range s.items {
		if it.CreatedAt >= cutoff {
			s.items[n] = it
			n++
		}
	}
	s.items = s.items[:n]
	s.syncCountLocked()
}

func (s *Store) syncCountLocked() {
	// #nosec G115 -- Add caps this slice at MaxCount (3).
	s.count.Store(int32(len(s.items)))
}

// GenerateCorrelationID creates a unique correlation ID for intents.
func GenerateCorrelationID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "intent_" + hex.EncodeToString([]byte(time.Now().String()[:19]))
	}
	return "intent_" + hex.EncodeToString(b)
}
