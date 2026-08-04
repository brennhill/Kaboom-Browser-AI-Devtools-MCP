// Purpose: Manages anonymous per-tab annotation sessions and draw-time waiting behavior.
// Why: Isolates session lifecycle and waiter completion from detail/named-session concerns.
// Docs: docs/features/feature/annotated-screenshots/index.md

package annotation

import (
	"encoding/json"
	"time"
)

// GetLatestSessionSinceDraw returns the latest session newer than MarkDrawStarted, or nil.
func (s *Store) GetLatestSessionSinceDraw() *Session {
	s.mu.RLock()
	sinceTs := s.lastDrawStartedAt
	s.mu.RUnlock()
	return s.getSessionSince(sinceTs)
}

// StoreSession saves an annotation session, overwriting any previous session for the tab.
// Notifies any goroutines blocked in WaitForSession and completes async waiters.
func (s *Store) StoreSession(tabID int, session *Session) {
	type sessionStorePlan struct {
		toComplete []waiter
		completeFn func(string, json.RawMessage)
		ch         chan struct{}
	}
	plan := func() sessionStorePlan {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.sessions[tabID] = &sessionEntry{
			Session:   session,
			ExpiresAt: s.now().Add(s.sessionTTL),
		}
		// Evict oldest sessions if over cap.
		if len(s.sessions) > MaxSessions {
			s.evictOldestSessionLocked()
		}
		// Collect and clear anonymous waiters (sessionName == "").
		var toComplete []waiter
		remaining := s.waiters[:0]
		for _, w := range s.waiters {
			if w.AnnotSessionName == "" {
				toComplete = append(toComplete, w)
			} else {
				remaining = append(remaining, w)
			}
		}
		s.waiters = remaining
		// Notify blocking waiters: close current channel, create a fresh one.
		ch := s.sessionNotify
		s.sessionNotify = make(chan struct{})
		return sessionStorePlan{
			toComplete: toComplete,
			completeFn: s.completeCommand,
			ch:         ch,
		}
	}()
	close(plan.ch)

	// Complete async waiters outside the lock.
	if plan.completeFn != nil {
		for _, w := range plan.toComplete {
			result := BuildSessionResult(session, w.URLFilter)
			plan.completeFn(w.CorrelationID, result)
		}
	}
}

// MarkDrawStarted records the current time so WaitForSession can skip stale sessions.
func (s *Store) MarkDrawStarted() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastDrawStartedAt = s.now().UnixMilli()
}

// WaitForSession blocks until a session newer than the last MarkDrawStarted arrives,
// or until timeout expires. Returns (session, timedOut).
// Loops on wake-ups to handle spurious notifications from unrelated sessions.
func (s *Store) WaitForSession(timeout time.Duration) (*Session, bool) {
	checker := func() any {
		// T8 fix: re-read lastDrawStartedAt under the lock each iteration.
		// If the user triggers a new draw cycle (updating lastDrawStartedAt),
		// the checker must use the fresh threshold to avoid returning a session
		// from a previous draw cycle.
		s.mu.RLock()
		sinceTs := s.lastDrawStartedAt
		s.mu.RUnlock()
		if session := s.getSessionSince(sinceTs); session != nil {
			return session
		}
		return nil
	}

	result, timedOut := s.waitForCondition(timeout, checker)
	if result != nil {
		return result.(*Session), false
	}
	return nil, timedOut
}

// getSessionSince returns the latest session with Timestamp > sinceTs.
func (s *Store) getSessionSince(sinceTs int64) *Session {
	s.mu.RLock()
	defer s.mu.RUnlock()
	now := s.now()
	var latest *sessionEntry
	for _, entry := range s.sessions {
		if now.After(entry.ExpiresAt) {
			continue
		}
		if entry.Session.Timestamp <= sinceTs {
			continue
		}
		if latest == nil || entry.Session.Timestamp > latest.Session.Timestamp {
			latest = entry
		}
	}
	if latest == nil {
		return nil
	}
	return latest.Session
}

// GetSession returns the latest annotation session for a tab.
func (s *Store) GetSession(tabID int) *Session {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entry := s.sessions[tabID]
	if entry == nil {
		return nil
	}
	if s.now().After(entry.ExpiresAt) {
		return nil
	}
	return entry.Session
}

// GetLatestSession returns the most recent annotation session across all tabs.
func (s *Store) GetLatestSession() *Session {
	s.mu.RLock()
	defer s.mu.RUnlock()
	now := s.now()
	var latest *sessionEntry
	for _, entry := range s.sessions {
		if now.After(entry.ExpiresAt) {
			continue
		}
		if latest == nil || entry.Session.Timestamp > latest.Session.Timestamp {
			latest = entry
		}
	}
	if latest == nil {
		return nil
	}
	return latest.Session
}

// evictOldestSessionLocked removes the session with the oldest timestamp.
// Must be called with s.mu held.
func (s *Store) evictOldestSessionLocked() {
	var oldestKey int
	var oldestTime int64 = 1<<63 - 1
	for tabID, entry := range s.sessions {
		if entry.Session.Timestamp < oldestTime {
			oldestTime = entry.Session.Timestamp
			oldestKey = tabID
		}
	}
	delete(s.sessions, oldestKey)
}
