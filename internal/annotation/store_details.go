// Purpose: Manages detailed annotation payload storage with TTL and bounded capacity.
// Why: Keeps potentially large DOM/style detail handling independent from session orchestration.
// Docs: docs/features/feature/annotated-screenshots/index.md

package annotation

import "time"

// StoreDetail saves element detail with TTL expiration.
// Evicts oldest entries if the detail map exceeds MaxDetails.
func (s *Store) StoreDetail(correlationID string, detail Detail) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.details[correlationID] = &detailEntry{
		Detail:    detail,
		ExpiresAt: s.now().Add(s.detailTTL),
	}
	if len(s.details) > MaxDetails {
		s.evictOldestDetailLocked()
	}
}

// GetDetail retrieves element detail if not expired.
func (s *Store) GetDetail(correlationID string) (*Detail, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entry, ok := s.details[correlationID]
	if !ok {
		return nil, false
	}
	if s.now().After(entry.ExpiresAt) {
		return nil, false
	}
	return &entry.Detail, true
}

// FindAnnotationTimestamp searches all sessions (anonymous and named) for an
// annotation with the given correlationID. Returns the annotation's timestamp
// in milliseconds, or 0 if not found.
func (s *Store) FindAnnotationTimestamp(correlationID string) int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	now := s.now()

	// Search all anonymous sessions
	for _, entry := range s.sessions {
		if now.After(entry.ExpiresAt) {
			continue
		}
		for _, ann := range entry.Session.Annotations {
			if ann.CorrelationID == correlationID {
				return ann.Timestamp
			}
		}
	}

	// Search all named sessions
	for _, entry := range s.named {
		if now.After(entry.ExpiresAt) {
			continue
		}
		for _, page := range entry.Session.Pages {
			for _, ann := range page.Annotations {
				if ann.CorrelationID == correlationID {
					return ann.Timestamp
				}
			}
		}
	}

	return 0
}

// evictOldestDetailLocked removes the detail entry closest to expiration.
// Must be called with s.mu held.
func (s *Store) evictOldestDetailLocked() {
	var oldestKey string
	var oldestExpiry time.Time
	first := true
	for id, entry := range s.details {
		if first || entry.ExpiresAt.Before(oldestExpiry) {
			oldestExpiry = entry.ExpiresAt
			oldestKey = id
			first = false
		}
	}
	if !first {
		delete(s.details, oldestKey)
	}
}
