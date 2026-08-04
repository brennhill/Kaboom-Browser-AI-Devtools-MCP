// Purpose: Tests for annotation store CRUD operations.
// Docs: docs/features/feature/annotated-screenshots/index.md

// store_test.go — Tests for the annotation store.
package annotation

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

type annotationTestClock struct {
	now time.Time
}

func (c *annotationTestClock) Now() time.Time { return c.now }

func (c *annotationTestClock) Advance(delta time.Duration) { c.now = c.now.Add(delta) }

func useAnnotationTestClock(store *Store) *annotationTestClock {
	clock := &annotationTestClock{now: time.Unix(100, 0)}
	store.now = clock.Now
	return clock
}

func TestStoreUsesInjectedClockForExpiration(t *testing.T) {
	store := NewStore(time.Minute)
	defer store.Close()
	clock := useAnnotationTestClock(store)

	store.StoreDetail("clocked", Detail{CorrelationID: "clocked"})
	clock.Advance(time.Minute + time.Nanosecond)
	if _, found := store.GetDetail("clocked"); found {
		t.Fatal("detail remained visible after the injected clock passed its TTL")
	}
}

func TestStore_StoreAndGetSession(t *testing.T) {
	store := NewStore(10 * time.Minute)
	defer store.Close()

	session := &Session{
		Annotations: []Annotation{
			{
				ID:             "ann_1",
				Rect:           Rect{X: 100, Y: 200, Width: 150, Height: 50},
				Text:           "make this darker",
				Timestamp:      time.Now().UnixMilli(),
				PageURL:        "https://example.com",
				ElementSummary: "button.primary 'Submit'",
				CorrelationID:  "detail_1",
			},
		},
		ScreenshotPath: "/tmp/draw_test.png",
		PageURL:        "https://example.com",
		TabID:          42,
		Timestamp:      time.Now().UnixMilli(),
	}

	store.StoreSession(42, session)

	got := store.GetSession(42)
	if got == nil {
		t.Fatal("expected session, got nil")
	}
	if len(got.Annotations) != 1 {
		t.Fatalf("expected 1 annotation, got %d", len(got.Annotations))
	}
	if got.Annotations[0].Text != "make this darker" {
		t.Errorf("expected text 'make this darker', got %q", got.Annotations[0].Text)
	}
	if got.Annotations[0].ID != "ann_1" {
		t.Errorf("expected annotation ID 'ann_1', got %q", got.Annotations[0].ID)
	}
	if got.Annotations[0].CorrelationID != "detail_1" {
		t.Errorf("expected correlation ID 'detail_1', got %q", got.Annotations[0].CorrelationID)
	}
	if got.Annotations[0].ElementSummary != "button.primary 'Submit'" {
		t.Errorf("expected element summary, got %q", got.Annotations[0].ElementSummary)
	}
	if got.Annotations[0].PageURL != "https://example.com" {
		t.Errorf("expected page URL 'https://example.com', got %q", got.Annotations[0].PageURL)
	}
	if got.Annotations[0].Rect.X != 100 || got.Annotations[0].Rect.Y != 200 || got.Annotations[0].Rect.Width != 150 || got.Annotations[0].Rect.Height != 50 {
		t.Errorf("expected rect {100 200 150 50}, got %+v", got.Annotations[0].Rect)
	}
	if got.ScreenshotPath != "/tmp/draw_test.png" {
		t.Errorf("expected screenshot path, got %q", got.ScreenshotPath)
	}
	if got.PageURL != "https://example.com" {
		t.Errorf("expected session page URL 'https://example.com', got %q", got.PageURL)
	}
	if got.TabID != 42 {
		t.Errorf("expected tab ID 42, got %d", got.TabID)
	}
}

func TestStore_GetSessionNotFound(t *testing.T) {
	store := NewStore(10 * time.Minute)
	defer store.Close()
	got := store.GetSession(999)
	if got != nil {
		t.Errorf("expected nil for non-existent session, got %+v", got)
	}
}

func TestStore_SessionOverwrite(t *testing.T) {
	store := NewStore(10 * time.Minute)
	defer store.Close()

	session1 := &Session{
		Annotations: []Annotation{{Text: "first"}},
		TabID:       42,
		Timestamp:   100,
	}
	session2 := &Session{
		Annotations: []Annotation{{Text: "second"}, {Text: "third"}},
		TabID:       42,
		Timestamp:   200,
	}

	store.StoreSession(42, session1)
	store.StoreSession(42, session2)

	got := store.GetSession(42)
	if got == nil {
		t.Fatal("expected session, got nil")
	}
	if len(got.Annotations) != 2 {
		t.Fatalf("expected 2 annotations after overwrite, got %d", len(got.Annotations))
	}
	if got.Annotations[0].Text != "second" {
		t.Errorf("expected text 'second', got %q", got.Annotations[0].Text)
	}
	if got.Annotations[1].Text != "third" {
		t.Errorf("expected text 'third', got %q", got.Annotations[1].Text)
	}
	if got.Timestamp != 200 {
		t.Errorf("expected timestamp 200 after overwrite, got %d", got.Timestamp)
	}
}

func TestStore_GetLatestSession(t *testing.T) {
	store := NewStore(10 * time.Minute)
	defer store.Close()

	store.StoreSession(1, &Session{TabID: 1, Timestamp: 100, Annotations: []Annotation{{Text: "tab1"}}})
	store.StoreSession(2, &Session{TabID: 2, Timestamp: 300, Annotations: []Annotation{{Text: "tab2"}}})
	store.StoreSession(3, &Session{TabID: 3, Timestamp: 200, Annotations: []Annotation{{Text: "tab3"}}})

	latest := store.GetLatestSession()
	if latest == nil {
		t.Fatal("expected latest session, got nil")
	}
	if latest.TabID != 2 {
		t.Errorf("expected latest tab 2, got %d", latest.TabID)
	}
	if latest.Timestamp != 300 {
		t.Errorf("expected latest timestamp 300, got %d", latest.Timestamp)
	}
	if len(latest.Annotations) != 1 || latest.Annotations[0].Text != "tab2" {
		t.Errorf("expected annotation text 'tab2', got %+v", latest.Annotations)
	}
}

func TestStore_GetLatestSessionEmpty(t *testing.T) {
	store := NewStore(10 * time.Minute)
	defer store.Close()
	latest := store.GetLatestSession()
	if latest != nil {
		t.Errorf("expected nil for empty store, got %+v", latest)
	}
}

func TestStore_StoreAndGetDetail(t *testing.T) {
	store := NewStore(10 * time.Minute)
	defer store.Close()

	detail := Detail{
		CorrelationID:  "detail_1",
		Selector:       "button.primary",
		Tag:            "button",
		TextContent:    "Submit",
		Classes:        []string{"primary", "rounded"},
		ID:             "submit-btn",
		ComputedStyles: map[string]string{"background-color": "rgb(59, 130, 246)"},
		ParentSelector: "form.checkout > div.actions",
		BoundingRect:   Rect{X: 100, Y: 200, Width: 150, Height: 50},
	}

	store.StoreDetail("detail_1", detail)

	got, found := store.GetDetail("detail_1")
	if !found {
		t.Fatal("expected to find detail")
	}
	if got.CorrelationID != "detail_1" {
		t.Errorf("expected correlation ID 'detail_1', got %q", got.CorrelationID)
	}
	if got.Selector != "button.primary" {
		t.Errorf("expected selector 'button.primary', got %q", got.Selector)
	}
	if got.Tag != "button" {
		t.Errorf("expected tag 'button', got %q", got.Tag)
	}
	if got.TextContent != "Submit" {
		t.Errorf("expected text content 'Submit', got %q", got.TextContent)
	}
	if len(got.Classes) != 2 || got.Classes[0] != "primary" || got.Classes[1] != "rounded" {
		t.Errorf("expected classes [primary rounded], got %v", got.Classes)
	}
	if got.ID != "submit-btn" {
		t.Errorf("expected ID 'submit-btn', got %q", got.ID)
	}
	if got.ComputedStyles["background-color"] != "rgb(59, 130, 246)" {
		t.Errorf("expected background-color 'rgb(59, 130, 246)', got %q", got.ComputedStyles["background-color"])
	}
	if got.ParentSelector != "form.checkout > div.actions" {
		t.Errorf("expected parent selector 'form.checkout > div.actions', got %q", got.ParentSelector)
	}
	if got.BoundingRect.X != 100 || got.BoundingRect.Y != 200 || got.BoundingRect.Width != 150 || got.BoundingRect.Height != 50 {
		t.Errorf("expected bounding rect {100 200 150 50}, got %+v", got.BoundingRect)
	}
}

func TestStore_DetailNotFound(t *testing.T) {
	store := NewStore(10 * time.Minute)
	defer store.Close()
	_, found := store.GetDetail("nonexistent")
	if found {
		t.Error("expected not found for non-existent detail")
	}
}

func TestStore_DetailExpired(t *testing.T) {
	// Use very short TTL
	store := NewStore(1 * time.Millisecond)
	defer store.Close()
	clock := useAnnotationTestClock(store)

	store.StoreDetail("expire_test", Detail{Selector: "div.test"})

	clock.Advance(5 * time.Millisecond)

	_, found := store.GetDetail("expire_test")
	if found {
		t.Error("expected detail to be expired")
	}
}

func TestStore_ZeroAnnotations(t *testing.T) {
	store := NewStore(10 * time.Minute)
	defer store.Close()

	session := &Session{
		Annotations:    []Annotation{},
		ScreenshotPath: "/tmp/empty.png",
		TabID:          42,
		Timestamp:      time.Now().UnixMilli(),
	}
	store.StoreSession(42, session)

	got := store.GetSession(42)
	if got == nil {
		t.Fatal("expected session even with 0 annotations")
	}
	if len(got.Annotations) != 0 {
		t.Errorf("expected 0 annotations, got %d", len(got.Annotations))
	}
	if got.ScreenshotPath != "/tmp/empty.png" {
		t.Errorf("expected screenshot path '/tmp/empty.png', got %q", got.ScreenshotPath)
	}
	if got.TabID != 42 {
		t.Errorf("expected tab ID 42, got %d", got.TabID)
	}
}

func TestStore_ConcurrentAccess(t *testing.T) {
	store := NewStore(10 * time.Minute)
	defer store.Close()

	var wg sync.WaitGroup
	// Concurrent session writes
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(tabID int) {
			defer wg.Done()
			store.StoreSession(tabID, &Session{
				TabID:       tabID,
				Timestamp:   time.Now().UnixMilli(),
				Annotations: []Annotation{{Text: fmt.Sprintf("tab%d", tabID)}},
			})
		}(i)
	}
	// Concurrent detail writes
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			store.StoreDetail(fmt.Sprintf("detail_%d", id), Detail{
				Selector: fmt.Sprintf("div.item-%d", id),
			})
		}(i)
	}
	// Concurrent reads
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(tabID int) {
			defer wg.Done()
			store.GetSession(tabID)
			store.GetDetail(fmt.Sprintf("detail_%d", tabID))
			store.GetLatestSession()
		}(i)
	}
	wg.Wait()

	// Verify all sessions were stored (writes all complete before reads in the WaitGroup)
	found := 0
	for i := 0; i < 50; i++ {
		s := store.GetSession(i)
		if s != nil {
			found++
			if s.TabID != i {
				t.Errorf("session %d: TabID = %d, want %d", i, s.TabID, i)
			}
			if len(s.Annotations) != 1 {
				t.Errorf("session %d: annotation count = %d, want 1", i, len(s.Annotations))
			}
		}
	}
	if found != 50 {
		t.Errorf("Expected all 50 sessions to be stored after concurrent access, got %d", found)
	}

	// Verify all details were stored
	detailFound := 0
	for i := 0; i < 50; i++ {
		d, ok := store.GetDetail(fmt.Sprintf("detail_%d", i))
		if ok {
			detailFound++
			expectedSelector := fmt.Sprintf("div.item-%d", i)
			if d.Selector != expectedSelector {
				t.Errorf("detail_%d: selector = %q, want %q", i, d.Selector, expectedSelector)
			}
		}
	}
	if detailFound != 50 {
		t.Errorf("Expected all 50 details to be stored, got %d", detailFound)
	}
}

func TestStore_SessionEvictionCap(t *testing.T) {
	store := NewStore(10 * time.Minute)
	defer store.Close()

	// Store more sessions than MaxSessions (100)
	for i := 1; i <= 110; i++ {
		store.StoreSession(i, &Session{
			TabID:       i,
			Timestamp:   int64(i),
			Annotations: []Annotation{{Text: fmt.Sprintf("session_%d", i)}},
		})
	}

	// Count surviving sessions (should be <= MaxSessions)
	count := 0
	for i := 1; i <= 110; i++ {
		if store.GetSession(i) != nil {
			count++
		}
	}
	if count > 100 {
		t.Errorf("Expected at most 100 sessions after eviction, got %d", count)
	}
	// The newest sessions should survive with correct data
	newest := store.GetSession(110)
	if newest == nil {
		t.Fatal("Expected newest session (110) to survive eviction")
	}
	if newest.TabID != 110 {
		t.Errorf("newest session TabID = %d, want 110", newest.TabID)
	}
	if newest.Timestamp != 110 {
		t.Errorf("newest session Timestamp = %d, want 110", newest.Timestamp)
	}
	if len(newest.Annotations) != 1 || newest.Annotations[0].Text != "session_110" {
		t.Errorf("newest session annotation = %+v, want text 'session_110'", newest.Annotations)
	}
}

func TestStore_MarkDrawStarted(t *testing.T) {
	store := NewStore(10 * time.Minute)
	defer store.Close()

	before := time.Now().UnixMilli()
	store.MarkDrawStarted()
	after := time.Now().UnixMilli()

	store.mu.RLock()
	ts := store.lastDrawStartedAt
	store.mu.RUnlock()

	if ts < before || ts > after {
		t.Errorf("expected lastDrawStartedAt between %d and %d, got %d", before, after, ts)
	}
}

func TestStore_WaitForSession_ImmediateReturn(t *testing.T) {
	store := NewStore(10 * time.Minute)
	defer store.Close()
	clock := useAnnotationTestClock(store)

	// Mark draw started, then store a session with a newer timestamp
	store.MarkDrawStarted()
	clock.Advance(time.Millisecond)
	store.StoreSession(1, &Session{
		TabID:       1,
		Timestamp:   clock.Now().UnixMilli(),
		Annotations: []Annotation{{Text: "immediate"}},
	})

	session, timedOut := store.WaitForSession(100 * time.Millisecond)
	if timedOut {
		t.Fatal("expected immediate return, got timeout")
	}
	if session == nil {
		t.Fatal("expected session, got nil")
	}
	if session.TabID != 1 {
		t.Errorf("expected TabID 1, got %d", session.TabID)
	}
	if len(session.Annotations) != 1 {
		t.Fatalf("expected 1 annotation, got %d", len(session.Annotations))
	}
	if session.Annotations[0].Text != "immediate" {
		t.Errorf("expected text 'immediate', got %q", session.Annotations[0].Text)
	}
}

func TestStore_WaitForSession_BlocksAndReturns(t *testing.T) {
	store := NewStore(10 * time.Minute)
	defer store.Close()
	clock := useAnnotationTestClock(store)

	store.MarkDrawStarted()
	clock.Advance(time.Millisecond)

	result := make(chan struct {
		session  *Session
		timedOut bool
	}, 1)
	go func() {
		session, timedOut := store.WaitForSession(2 * time.Second)
		result <- struct {
			session  *Session
			timedOut bool
		}{session: session, timedOut: timedOut}
	}()
	store.StoreSession(1, &Session{
		TabID:       1,
		Timestamp:   clock.Now().UnixMilli(),
		Annotations: []Annotation{{Text: "delayed"}},
	})
	waited := <-result
	session, timedOut := waited.session, waited.timedOut

	if timedOut {
		t.Fatal("expected session, got timeout")
	}
	if session == nil {
		t.Fatal("expected session, got nil")
	}
	if session.TabID != 1 {
		t.Errorf("expected TabID 1, got %d", session.TabID)
	}
	if len(session.Annotations) != 1 {
		t.Fatalf("expected 1 annotation, got %d", len(session.Annotations))
	}
	if session.Annotations[0].Text != "delayed" {
		t.Errorf("expected text 'delayed', got %q", session.Annotations[0].Text)
	}
}

func TestStore_WaitForSession_Timeout(t *testing.T) {
	store := NewStore(10 * time.Minute)
	defer store.Close()

	store.MarkDrawStarted()

	start := time.Now()
	session, timedOut := store.WaitForSession(50 * time.Millisecond)
	elapsed := time.Since(start)

	if !timedOut {
		t.Error("expected timeout")
	}
	if session != nil {
		t.Errorf("expected nil session on timeout, got %+v", session)
	}
	if elapsed < 40*time.Millisecond {
		t.Error("expected to have waited at least 40ms")
	}
}

func TestStore_WaitForSession_SkipsStaleSession(t *testing.T) {
	store := NewStore(10 * time.Minute)
	defer store.Close()
	clock := useAnnotationTestClock(store)

	// Store an old session BEFORE marking draw started
	store.StoreSession(1, &Session{
		TabID:       1,
		Timestamp:   clock.Now().UnixMilli() - 5000,
		Annotations: []Annotation{{Text: "stale"}},
	})

	store.MarkDrawStarted()

	// The stale session should not be returned — it's from before draw started
	session, timedOut := store.WaitForSession(50 * time.Millisecond)

	if !timedOut {
		t.Error("expected timeout since only stale session exists")
	}
	if session != nil {
		t.Errorf("expected nil (stale session should be skipped), got %+v", session)
	}
}

func TestStore_WaitForSession_NoDrawStarted(t *testing.T) {
	store := NewStore(10 * time.Minute)
	defer store.Close()

	// Without MarkDrawStarted, lastDrawStartedAt is 0 — any session qualifies
	store.StoreSession(1, &Session{
		TabID:       1,
		Timestamp:   time.Now().UnixMilli(),
		Annotations: []Annotation{{Text: "any"}},
	})

	session, timedOut := store.WaitForSession(50 * time.Millisecond)
	if timedOut {
		t.Fatal("expected immediate return, got timeout")
	}
	if session == nil {
		t.Fatal("expected session, got nil")
	}
	if session.TabID != 1 {
		t.Errorf("expected TabID 1, got %d", session.TabID)
	}
	if len(session.Annotations) != 1 || session.Annotations[0].Text != "any" {
		t.Errorf("expected annotation text 'any', got %+v", session.Annotations)
	}
}

func TestStore_WaitForSession_CloseUnblocks(t *testing.T) {
	store := NewStore(10 * time.Minute)

	store.MarkDrawStarted()

	result := make(chan *Session, 1)
	go func() {
		session, _ := store.WaitForSession(5 * time.Second)
		result <- session
	}()
	store.Close()
	session := <-result

	if session != nil {
		t.Error("expected nil session after close")
	}
}
