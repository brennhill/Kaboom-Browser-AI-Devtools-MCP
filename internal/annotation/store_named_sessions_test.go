// Purpose: Tests for annotation store CRUD operations.
// Docs: docs/features/feature/annotated-screenshots/index.md

// store_named_sessions_test.go — Named annotation-session storage and waiting tests.
package annotation

import (
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestStore_NamedSession_AppendAndGet(t *testing.T) {
	store := NewStore(10 * time.Minute)
	defer store.Close()

	page1 := &Session{
		TabID:       1,
		Timestamp:   100,
		PageURL:     "https://example.com/login",
		Annotations: []Annotation{{Text: "fix button"}},
	}
	page2 := &Session{
		TabID:       1,
		Timestamp:   200,
		PageURL:     "https://example.com/dashboard",
		Annotations: []Annotation{{Text: "wrong color"}, {Text: "misaligned"}},
	}

	store.AppendToNamedSession("qa-review", page1)
	store.AppendToNamedSession("qa-review", page2)

	ns := store.GetNamedSession("qa-review")
	if ns == nil {
		t.Fatal("expected named session")
	}
	if ns.Name != "qa-review" {
		t.Errorf("expected name 'qa-review', got %q", ns.Name)
	}
	if len(ns.Pages) != 2 {
		t.Fatalf("expected 2 pages, got %d", len(ns.Pages))
	}
	if ns.Pages[0].PageURL != "https://example.com/login" {
		t.Errorf("expected first page URL, got %q", ns.Pages[0].PageURL)
	}
	if ns.Pages[0].Timestamp != 100 {
		t.Errorf("expected first page timestamp 100, got %d", ns.Pages[0].Timestamp)
	}
	if len(ns.Pages[0].Annotations) != 1 || ns.Pages[0].Annotations[0].Text != "fix button" {
		t.Errorf("expected first page annotation 'fix button', got %+v", ns.Pages[0].Annotations)
	}
	if ns.Pages[1].PageURL != "https://example.com/dashboard" {
		t.Errorf("expected second page URL, got %q", ns.Pages[1].PageURL)
	}
	if ns.Pages[1].Timestamp != 200 {
		t.Errorf("expected second page timestamp 200, got %d", ns.Pages[1].Timestamp)
	}
	if len(ns.Pages[1].Annotations) != 2 {
		t.Errorf("expected 2 annotations on page 2, got %d", len(ns.Pages[1].Annotations))
	}
	if ns.Pages[1].Annotations[0].Text != "wrong color" {
		t.Errorf("expected annotation 'wrong color', got %q", ns.Pages[1].Annotations[0].Text)
	}
	if ns.Pages[1].Annotations[1].Text != "misaligned" {
		t.Errorf("expected annotation 'misaligned', got %q", ns.Pages[1].Annotations[1].Text)
	}
}

func TestStore_NamedSession_NotFound(t *testing.T) {
	store := NewStore(10 * time.Minute)
	defer store.Close()

	ns := store.GetNamedSession("nonexistent")
	if ns != nil {
		t.Errorf("expected nil for non-existent named session, got %+v", ns)
	}
}

func TestStore_NamedSession_ListSessions(t *testing.T) {
	store := NewStore(10 * time.Minute)
	defer store.Close()

	store.AppendToNamedSession("review-1", &Session{TabID: 1, Timestamp: 100})
	store.AppendToNamedSession("review-2", &Session{TabID: 1, Timestamp: 200})

	names := store.ListNamedSessions()
	if len(names) != 2 {
		t.Fatalf("expected 2 named sessions, got %d", len(names))
	}
}

func TestStore_NamedSession_Clear(t *testing.T) {
	store := NewStore(10 * time.Minute)
	defer store.Close()

	store.AppendToNamedSession("qa", &Session{TabID: 1, Timestamp: 100})
	store.ClearNamedSession("qa")

	ns := store.GetNamedSession("qa")
	if ns != nil {
		t.Errorf("expected nil after clear, got %+v", ns)
	}
}

func TestStore_NamedSession_WaitBlocks(t *testing.T) {
	store := NewStore(10 * time.Minute)
	defer store.Close()

	store.MarkDrawStarted()

	go func() {
		time.Sleep(50 * time.Millisecond)
		store.AppendToNamedSession("qa", &Session{
			TabID:       1,
			Timestamp:   time.Now().UnixMilli(),
			Annotations: []Annotation{{Text: "waited"}},
		})
	}()

	start := time.Now()
	ns, timedOut := store.WaitForNamedSession("qa", 2*time.Second)
	elapsed := time.Since(start)

	if timedOut {
		t.Fatal("expected session, got timeout")
	}
	if ns == nil {
		t.Fatal("expected named session")
	}
	if ns.Name != "qa" {
		t.Errorf("expected name 'qa', got %q", ns.Name)
	}
	if len(ns.Pages) != 1 {
		t.Fatalf("expected 1 page, got %d", len(ns.Pages))
	}
	if len(ns.Pages[0].Annotations) != 1 || ns.Pages[0].Annotations[0].Text != "waited" {
		t.Errorf("expected annotation 'waited', got %+v", ns.Pages[0].Annotations)
	}
	if elapsed < 30*time.Millisecond {
		t.Error("expected to have blocked")
	}
}

func TestStore_NamedSession_WaitTimeout(t *testing.T) {
	store := NewStore(10 * time.Minute)
	defer store.Close()

	store.MarkDrawStarted()

	_, timedOut := store.WaitForNamedSession("qa", 50*time.Millisecond)
	if !timedOut {
		t.Error("expected timeout")
	}
}

func TestStore_NamedSession_EvictionCap(t *testing.T) {
	store := NewStore(10 * time.Minute)
	defer store.Close()

	// Fill up to MaxNamedSessions + 1 (51 total)
	for i := 0; i < 51; i++ {
		name := "session_" + strings.Repeat("0", 3-len(strconv.Itoa(i))) + strconv.Itoa(i) // zero-padded
		store.AppendToNamedSession(name, &Session{
			TabID:     i + 1,
			Timestamp: int64(1000 + i),
			Annotations: []Annotation{
				{Text: "annotation for " + name},
			},
		})
		// Small sleep to ensure UpdatedAt ordering
		time.Sleep(time.Millisecond)
	}

	names := store.ListNamedSessions()
	if len(names) != 50 {
		t.Fatalf("expected 50 named sessions after eviction, got %d", len(names))
	}

	// The first session (session_000) should have been evicted (oldest UpdatedAt)
	evicted := store.GetNamedSession("session_000")
	if evicted != nil {
		t.Error("expected session_000 to be evicted, but it still exists")
	}

	// The most recent session should still exist with correct data
	latest := store.GetNamedSession("session_050")
	if latest == nil {
		t.Fatal("expected session_050 to exist after eviction")
	}
	if latest.Name != "session_050" {
		t.Errorf("latest session name = %q, want 'session_050'", latest.Name)
	}
	if len(latest.Pages) != 1 {
		t.Errorf("expected 1 page in latest session, got %d", len(latest.Pages))
	}
	if len(latest.Pages) > 0 && len(latest.Pages[0].Annotations) > 0 {
		if latest.Pages[0].Annotations[0].Text != "annotation for session_050" {
			t.Errorf("latest annotation text = %q, want 'annotation for session_050'", latest.Pages[0].Annotations[0].Text)
		}
	}
}

// TestStore_WaitForSession_SpuriousWakeup verifies that WaitForSession
// loops correctly when a notify fires for an unrelated session (different tab).
