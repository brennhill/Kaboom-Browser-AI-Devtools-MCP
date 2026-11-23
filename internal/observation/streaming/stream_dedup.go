// Purpose: Deduplicates streaming notifications by tracking recently-seen message keys.
// Why: Separates dedup state management from emission, filtering, and rate limiting.
package streaming

import "time"

// IsDuplicate checks if a dedup key was seen within the dedup window.
func (s *StreamState) IsDuplicate(key string, now time.Time) bool {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	if lastSeen, ok := s.SeenMessages[key]; ok {
		if now.Sub(lastSeen) < DedupWindow {
			return true
		}
	}
	return false
}

// RecordDedupKey records that a message with this key was sent.
func (s *StreamState) RecordDedupKey(key string, now time.Time) {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	s.SeenMessages[key] = now

	for k, t := range s.SeenMessages {
		if now.Sub(t) > DedupWindow {
			delete(s.SeenMessages, k)
		}
	}
}

// isDuplicateLocked checks if this alert was recently emitted.
// Caller must hold s.Mu.
func (s *StreamState) isDuplicateLocked(dedupKey string, now time.Time) bool {
	if lastSeen, ok := s.SeenMessages[dedupKey]; ok {
		return now.Sub(lastSeen) < DedupWindow
	}
	return false
}

// recordEmissionLocked updates emission state and prunes stale dedup entries.
// Caller must hold s.Mu.
func (s *StreamState) recordEmissionLocked(dedupKey string, now time.Time) {
	s.LastNotified = now
	s.checkRateResetLocked(now)
	s.NotifyCount++
	s.SeenMessages[dedupKey] = now

	for k, t := range s.SeenMessages {
		if now.Sub(t) > DedupWindow {
			delete(s.SeenMessages, k)
		}
	}
}
