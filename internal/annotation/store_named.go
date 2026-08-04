// Purpose: Manages named multi-page annotation sessions and related waiter completion.
// Why: Keeps cross-page session behavior isolated from anonymous per-tab session flow.
// Docs: docs/features/feature/annotated-screenshots/index.md

package annotation

import (
	"encoding/json"
	"time"
)

// GetNamedSessionSinceDraw returns the named session if updated after MarkDrawStarted, or nil.
func (s *Store) GetNamedSessionSinceDraw(name string) *NamedSession {
	s.mu.RLock()
	sinceTs := s.lastDrawStartedAt
	s.mu.RUnlock()
	ns := s.GetNamedSession(name)
	if ns != nil && ns.UpdatedAt > sinceTs {
		return ns
	}
	return nil
}

// AppendToNamedSession adds a page to a named multi-page session.
// Creates the session if it doesn't exist. Also fires the session notify and completes async waiters.
func (s *Store) AppendToNamedSession(name string, session *Session) {
	type namedSessionPlan struct {
		toComplete []waiter
		completeFn func(string, json.RawMessage)
		nsCopy     NamedSession
		ch         chan struct{}
	}
	plan := func() namedSessionPlan {
		s.mu.Lock()
		defer s.mu.Unlock()
		entry, ok := s.named[name]
		if !ok {
			entry = &namedSessionEntry{
				Session: &NamedSession{
					Name: name,
				},
			}
			s.named[name] = entry
		}
		entry.Session.Pages = append(entry.Session.Pages, session)
		now := s.now()
		entry.Session.UpdatedAt = now.UnixMilli()
		entry.ExpiresAt = now.Add(s.sessionTTL)
		// Evict oldest named sessions if over cap.
		if len(s.named) > MaxNamedSessions {
			s.evictOldestNamedSessionLocked()
		}
		// Collect and clear matching named waiters.
		var toComplete []waiter
		remaining := s.waiters[:0]
		for _, w := range s.waiters {
			if w.AnnotSessionName == name {
				toComplete = append(toComplete, w)
			} else {
				remaining = append(remaining, w)
			}
		}
		s.waiters = remaining

		// Snapshot the session for completing waiters.
		nsCopy := *entry.Session
		nsCopy.Pages = make([]*Session, len(entry.Session.Pages))
		copy(nsCopy.Pages, entry.Session.Pages)

		// Notify blocking waiters.
		ch := s.sessionNotify
		s.sessionNotify = make(chan struct{})
		return namedSessionPlan{
			toComplete: toComplete,
			completeFn: s.completeCommand,
			nsCopy:     nsCopy,
			ch:         ch,
		}
	}()
	close(plan.ch)

	// Complete async waiters outside the lock.
	if plan.completeFn != nil {
		for _, w := range plan.toComplete {
			result := BuildNamedSessionResult(&plan.nsCopy, w.URLFilter)
			plan.completeFn(w.CorrelationID, result)
		}
	}
}

// GetNamedSession returns a snapshot of the named multi-page session if not expired.
// Returns a shallow copy with its own Pages slice to avoid data races with concurrent
// AppendToNamedSession calls that modify the internal Pages slice under write lock.
func (s *Store) GetNamedSession(name string) *NamedSession {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entry, ok := s.named[name]
	if !ok {
		return nil
	}
	if s.now().After(entry.ExpiresAt) {
		return nil
	}
	// Return a copy so callers can safely iterate Pages without holding the lock.
	copied := *entry.Session
	copied.Pages = make([]*Session, len(entry.Session.Pages))
	copy(copied.Pages, entry.Session.Pages)
	return &copied
}

// ListNamedSessions returns the names of all non-expired named sessions.
func (s *Store) ListNamedSessions() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	now := s.now()
	names := make([]string, 0, len(s.named))
	for name, entry := range s.named {
		if !now.After(entry.ExpiresAt) {
			names = append(names, name)
		}
	}
	return names
}

// ClearNamedSession removes a named session.
func (s *Store) ClearNamedSession(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.named, name)
}

// evictOldestNamedSessionLocked removes the named session with the oldest UpdatedAt.
// Must be called with s.mu held.
func (s *Store) evictOldestNamedSessionLocked() {
	var oldestKey string
	var oldestTime int64 = 1<<63 - 1
	for name, entry := range s.named {
		if entry.Session.UpdatedAt < oldestTime {
			oldestTime = entry.Session.UpdatedAt
			oldestKey = name
		}
	}
	delete(s.named, oldestKey)
}

// WaitForNamedSession blocks until the named session has been updated since sinceTs,
// or until timeout expires. Returns (session, timedOut).
// Loops on wake-ups to handle spurious notifications from unrelated sessions.
func (s *Store) WaitForNamedSession(name string, timeout time.Duration) (*NamedSession, bool) {
	s.mu.RLock()
	sinceTs := s.lastDrawStartedAt
	s.mu.RUnlock()

	checker := func() any {
		if ns := s.GetNamedSession(name); ns != nil && ns.UpdatedAt > sinceTs {
			return ns
		}
		return nil
	}

	result, timedOut := s.waitForCondition(timeout, checker)
	if result != nil {
		return result.(*NamedSession), false
	}
	return nil, timedOut
}
