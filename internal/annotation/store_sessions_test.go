// store_sessions_test.go — Regression tests for WaitForSession stale timestamp (T8).
// Purpose: Validates that WaitForSession re-reads lastDrawStartedAt on each check,
// so a second MarkDrawStarted mid-wait invalidates sessions from the first draw cycle.
// Docs: docs/features/feature/annotated-screenshots/index.md

package annotation

import (
	"testing"
	"time"
)

// TestWaitForSession_ReReadsDrawTimestamp_T8 is the regression test for T8.
// Scenario:
//  1. MarkDrawStarted (first draw cycle)
//  2. Start WaitForSession in a goroutine
//  3. Store a session from the first draw cycle
//  4. MarkDrawStarted again (second draw cycle) — invalidates the first session
//  5. Verify WaitForSession does NOT return the first session
//  6. Store a session from the second draw cycle
//  7. Verify WaitForSession returns the second session
func TestWaitForSession_ReReadsDrawTimestamp_T8(t *testing.T) {
	store := NewStore(10 * time.Minute)
	defer store.Close()
	clock := useAnnotationTestClock(store)

	// Step 1: first draw cycle
	store.MarkDrawStarted()
	clock.Advance(time.Millisecond)

	// Store a session from the first draw cycle.
	firstSession := &Session{
		Annotations: []Annotation{{ID: "first", Text: "first cycle"}},
		PageURL:     "https://example.com/page1",
		TabID:       1,
		Timestamp:   clock.Now().UnixMilli(),
	}
	store.StoreSession(1, firstSession)

	// Step 4: second draw cycle — advances the threshold past firstSession.
	clock.Advance(time.Millisecond)
	store.MarkDrawStarted()
	clock.Advance(time.Millisecond)

	// Step 2: start WaitForSession. Because lastDrawStartedAt is now after
	// firstSession.Timestamp, the checker must NOT return firstSession.
	result := make(chan struct {
		session  *Session
		timedOut bool
	}, 1)

	go func() {
		session, timedOut := store.WaitForSession(500 * time.Millisecond)
		result <- struct {
			session  *Session
			timedOut bool
		}{session: session, timedOut: timedOut}
	}()

	// Step 6: store a session from the second draw cycle.
	secondSession := &Session{
		Annotations: []Annotation{{ID: "second", Text: "second cycle"}},
		PageURL:     "https://example.com/page2",
		TabID:       2,
		Timestamp:   clock.Now().UnixMilli(),
	}
	store.StoreSession(2, secondSession)

	// Step 7: WaitForSession should now return the second session.
	var waited struct {
		session  *Session
		timedOut bool
	}
	select {
	case waited = <-result:
	case <-time.After(2 * time.Second):
		t.Fatal("WaitForSession did not return within timeout")
	}

	if waited.timedOut {
		t.Fatal("WaitForSession timed out unexpectedly")
	}
	if waited.session == nil {
		t.Fatal("WaitForSession returned nil session")
	}
	if waited.session.Annotations[0].ID != "second" {
		t.Errorf("expected session from second draw cycle (id=second), got id=%s",
			waited.session.Annotations[0].ID)
	}
}

// TestWaitForSession_SingleDrawCycle_StillWorks verifies the basic case:
// a single draw cycle where WaitForSession returns the session correctly.
func TestWaitForSession_SingleDrawCycle_StillWorks(t *testing.T) {
	store := NewStore(10 * time.Minute)
	defer store.Close()
	clock := useAnnotationTestClock(store)

	store.MarkDrawStarted()
	clock.Advance(time.Millisecond)

	done := make(chan struct{})
	var result *Session
	var timedOut bool

	go func() {
		result, timedOut = store.WaitForSession(500 * time.Millisecond)
		close(done)
	}()

	session := &Session{
		Annotations: []Annotation{{ID: "a1", Text: "test"}},
		PageURL:     "https://example.com",
		TabID:       1,
		Timestamp:   clock.Now().UnixMilli(),
	}
	store.StoreSession(1, session)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("WaitForSession did not return within timeout")
	}

	if timedOut {
		t.Fatal("WaitForSession timed out unexpectedly")
	}
	if result == nil {
		t.Fatal("WaitForSession returned nil")
	}
	if result.Annotations[0].ID != "a1" {
		t.Errorf("expected annotation id 'a1', got %q", result.Annotations[0].ID)
	}
}
