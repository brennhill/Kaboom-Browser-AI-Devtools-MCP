// Purpose: Implements background TTL eviction for annotation sessions/details.
// Why: Ensures memory usage stays bounded without coupling cleanup logic to read/write paths.
// Docs: docs/features/feature/annotated-screenshots/index.md

package annotation

import "time"

// cleanupLoop periodically removes expired sessions and detail entries.
func (s *Store) cleanupLoop() {
	ticker := time.NewTicker(2 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-s.done:
			return
		case <-ticker.C:
			s.evictExpiredEntries()
		}
	}
}

// evictExpiredEntries removes all expired details, sessions, and named sessions.
func (s *Store) evictExpiredEntries() {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	for id, entry := range s.details {
		if now.After(entry.ExpiresAt) {
			delete(s.details, id)
		}
	}
	for tabID, entry := range s.sessions {
		if now.After(entry.ExpiresAt) {
			delete(s.sessions, tabID)
		}
	}
	for name, entry := range s.named {
		if now.After(entry.ExpiresAt) {
			delete(s.named, name)
		}
	}
}
