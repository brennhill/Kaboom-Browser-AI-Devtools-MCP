// Purpose: Tests for annotation store CRUD operations.
// Docs: docs/features/feature/annotated-screenshots/index.md

// store_lifecycle_test.go — Annotation store expiry, wakeup, and close tests.
package annotation

import (
	"testing"
	"time"
)

func TestStore_SessionExpired(t *testing.T) {
	store := NewStore(10 * time.Minute)
	defer store.Close()
	clock := useAnnotationTestClock(store)
	// Override session TTL to something very short for testing
	store.sessionTTL = 50 * time.Millisecond

	store.StoreSession(1, &Session{
		TabID:     1,
		Timestamp: time.Now().UnixMilli(),
		PageURL:   "https://example.com",
	})

	// Should be accessible immediately
	if store.GetSession(1) == nil {
		t.Fatal("Expected session to exist immediately after store")
	}

	clock.Advance(100 * time.Millisecond)

	// Should be nil after expiration
	if store.GetSession(1) != nil {
		t.Error("Expected session to be nil after TTL expiration")
	}

	// GetLatestSession should also return nil
	if store.GetLatestSession() != nil {
		t.Error("Expected GetLatestSession to return nil after TTL expiration")
	}
}

func TestStore_WaitForSession_SpuriousWakeup(t *testing.T) {
	store := NewStore(10 * time.Minute)
	defer store.Close()
	clock := useAnnotationTestClock(store)

	store.MarkDrawStarted()
	clock.Advance(time.Millisecond)

	var result *Session
	var timedOut bool
	done := make(chan struct{})

	go func() {
		result, timedOut = store.WaitForSession(2 * time.Second)
		close(done)
	}()

	// First wake-up: store a session with an old timestamp (before MarkDrawStarted).
	// This is a spurious notification — WaitForSession should NOT return.
	store.mu.Lock()
	store.sessions[99] = &sessionEntry{
		Session:   &Session{TabID: 99, Timestamp: 1},
		ExpiresAt: clock.Now().Add(10 * time.Minute),
	}
	ch := store.sessionNotify
	store.sessionNotify = make(chan struct{})
	store.mu.Unlock()
	close(ch)

	// Second wake-up: store a qualifying session (timestamp after MarkDrawStarted)
	store.StoreSession(42, &Session{
		TabID:     42,
		Timestamp: clock.Now().UnixMilli(),
	})

	select {
	case <-done:
		// Good — waiter returned
	case <-time.After(2 * time.Second):
		t.Fatal("WaitForSession did not return after qualifying session was stored")
	}

	if timedOut {
		t.Error("Expected no timeout")
	}
	if result == nil || result.TabID != 42 {
		t.Errorf("Expected session for tab 42, got %+v", result)
	}
}

// TestStore_WaitForNamedSession_SpuriousWakeup verifies that WaitForNamedSession
// loops correctly when a notify fires for a different session name.
func TestStore_WaitForNamedSession_SpuriousWakeup(t *testing.T) {
	store := NewStore(10 * time.Minute)
	defer store.Close()
	clock := useAnnotationTestClock(store)

	store.MarkDrawStarted()
	clock.Advance(time.Millisecond)

	var result *NamedSession
	var timedOut bool
	done := make(chan struct{})

	go func() {
		result, timedOut = store.WaitForNamedSession("target", 2*time.Second)
		close(done)
	}()

	// Spurious wake: append to a DIFFERENT named session
	store.AppendToNamedSession("other", &Session{
		TabID:     10,
		Timestamp: clock.Now().UnixMilli(),
		PageURL:   "https://other.com",
	})

	// Now store the target named session
	store.AppendToNamedSession("target", &Session{
		TabID:     20,
		Timestamp: clock.Now().UnixMilli(),
		PageURL:   "https://target.com",
	})

	select {
	case <-done:
		// Good
	case <-time.After(2 * time.Second):
		t.Fatal("WaitForNamedSession did not return after target session update")
	}

	if timedOut {
		t.Error("Expected no timeout")
	}
	if result == nil || result.Name != "target" {
		t.Errorf("Expected named session 'target', got %+v", result)
	}
}

func TestStore_Close(t *testing.T) {
	store := NewStore(10 * time.Minute)
	store.Close()
	// Double close should not panic (sync.Once protects)
	store.Close()
	// Store should still work after close (just no background cleanup)
	store.StoreSession(1, &Session{TabID: 1, Timestamp: 1})
	if store.GetSession(1) == nil {
		t.Error("Expected store to still work after Close")
	}
}

func TestStore_ConcurrentClose(t *testing.T) {
	store := NewStore(10 * time.Minute)
	// Concurrent Close calls should not panic
	done := make(chan struct{})
	for i := 0; i < 10; i++ {
		go func() {
			store.Close()
			done <- struct{}{}
		}()
	}
	for i := 0; i < 10; i++ {
		<-done
	}
}

// --- Additional coverage: GetLatestSession skips expired sessions ---

func TestStore_GetLatestSession_SkipsExpired(t *testing.T) {
	store := NewStore(10 * time.Minute)
	defer store.Close()

	// Store a valid session with an older timestamp
	store.StoreSession(1, &Session{
		TabID:     1,
		PageURL:   "https://example.com/valid",
		Timestamp: 1000,
	})

	// Inject an expired session with a newer timestamp
	store.mu.Lock()
	store.sessions[2] = &sessionEntry{
		Session: &Session{
			TabID:     2,
			PageURL:   "https://example.com/expired-but-newer",
			Timestamp: 5000,
		},
		ExpiresAt: time.Now().Add(-1 * time.Second),
	}
	store.mu.Unlock()

	got := store.GetLatestSession()
	if got == nil {
		t.Fatal("expected a session, got nil")
	}
	if got.TabID != 1 {
		t.Errorf("expected tab 1 (expired tab 2 should be skipped), got tab %d", got.TabID)
	}
	if got.PageURL != "https://example.com/valid" {
		t.Errorf("expected page URL 'https://example.com/valid', got %q", got.PageURL)
	}
	if got.Timestamp != 1000 {
		t.Errorf("expected timestamp 1000, got %d", got.Timestamp)
	}
}

// --- Additional coverage: GetNamedSession returns nil for expired ---

func TestStore_GetNamedSession_Expired(t *testing.T) {
	store := NewStore(10 * time.Minute)
	defer store.Close()

	// Inject an expired named session
	store.mu.Lock()
	store.named["old-session"] = &namedSessionEntry{
		Session: &NamedSession{
			Name:  "old-session",
			Pages: []*Session{{TabID: 1, PageURL: "https://old.com"}},
		},
		ExpiresAt: time.Now().Add(-1 * time.Second),
	}
	store.mu.Unlock()

	ns := store.GetNamedSession("old-session")
	if ns != nil {
		t.Error("expected nil for expired named session")
	}
}

// --- Additional coverage: GetNamedSession returns a copy, not reference ---

func TestStore_GetNamedSession_ReturnsCopy(t *testing.T) {
	store := NewStore(10 * time.Minute)
	defer store.Close()

	store.AppendToNamedSession("copytest", &Session{
		TabID:   1,
		PageURL: "https://example.com/page1",
	})

	copy1 := store.GetNamedSession("copytest")
	if copy1 == nil {
		t.Fatal("expected named session")
	}
	if copy1.Name != "copytest" {
		t.Errorf("expected name 'copytest', got %q", copy1.Name)
	}
	if len(copy1.Pages) != 1 {
		t.Fatalf("expected 1 page, got %d", len(copy1.Pages))
	}
	if copy1.Pages[0].PageURL != "https://example.com/page1" {
		t.Errorf("expected page URL 'https://example.com/page1', got %q", copy1.Pages[0].PageURL)
	}

	// Mutate the returned copy's Pages slice
	copy1.Pages = append(copy1.Pages, &Session{
		TabID:   99,
		PageURL: "https://mutated.com",
	})

	// Get again and verify internal state was not affected
	copy2 := store.GetNamedSession("copytest")
	if copy2 == nil {
		t.Fatal("expected named session on second get")
	}
	if len(copy2.Pages) != 1 {
		t.Errorf("expected internal state to have 1 page, got %d (copy mutation leaked)", len(copy2.Pages))
	}
}

// --- Additional coverage: ListNamedSessions excludes expired ---

func TestStore_ListNamedSessions_ExcludesExpired(t *testing.T) {
	store := NewStore(10 * time.Minute)
	defer store.Close()

	store.AppendToNamedSession("active1", &Session{TabID: 1})
	store.AppendToNamedSession("active2", &Session{TabID: 2})

	// Inject an expired named session
	store.mu.Lock()
	store.named["expired-one"] = &namedSessionEntry{
		Session:   &NamedSession{Name: "expired-one"},
		ExpiresAt: time.Now().Add(-1 * time.Second),
	}
	store.mu.Unlock()

	names := store.ListNamedSessions()
	nameSet := make(map[string]bool)
	for _, n := range names {
		nameSet[n] = true
	}

	if !nameSet["active1"] {
		t.Error("expected 'active1' in list")
	}
	if !nameSet["active2"] {
		t.Error("expected 'active2' in list")
	}
	if nameSet["expired-one"] {
		t.Error("expected 'expired-one' to be excluded (expired)")
	}
	if len(names) != 2 {
		t.Errorf("expected 2 names, got %d", len(names))
	}
}

// --- Additional coverage: ListNamedSessions on empty store ---

func TestStore_ListNamedSessions_Empty(t *testing.T) {
	store := NewStore(10 * time.Minute)
	defer store.Close()

	names := store.ListNamedSessions()
	if names == nil {
		t.Error("expected non-nil empty slice")
	}
	if len(names) != 0 {
		t.Errorf("expected 0 names, got %d", len(names))
	}
}

// --- Additional coverage: ClearNamedSession on nonexistent does not panic ---

func TestStore_ClearNamedSession_Nonexistent(t *testing.T) {
	store := NewStore(10 * time.Minute)
	defer store.Close()

	// Should not panic
	store.ClearNamedSession("nonexistent")

	// Verify still works
	store.AppendToNamedSession("after-clear", &Session{TabID: 1})
	if store.GetNamedSession("after-clear") == nil {
		t.Error("expected store to work after clearing nonexistent session")
	}
}

// --- Additional coverage: WaitForSession ignores stale pre-mark session ---

func TestStore_WaitForSession_IgnoresStaleSession(t *testing.T) {
	store := NewStore(10 * time.Minute)
	defer store.Close()
	clock := useAnnotationTestClock(store)

	// Store a session BEFORE MarkDrawStarted
	store.StoreSession(1, &Session{
		TabID:     1,
		Timestamp: clock.Now().UnixMilli(),
	})

	clock.Advance(time.Millisecond)
	store.MarkDrawStarted()

	// WaitForSession should NOT return the stale session
	session, timedOut := store.WaitForSession(50 * time.Millisecond)
	if session != nil {
		t.Error("expected nil session (stale session before MarkDrawStarted)")
	}
	if !timedOut {
		t.Error("expected timeout since no new session was stored")
	}
}

// --- Additional coverage: evictExpiredEntries cleans all three maps ---
