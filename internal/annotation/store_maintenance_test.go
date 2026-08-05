// Purpose: Tests for annotation store CRUD operations.
// Docs: docs/features/feature/annotated-screenshots/index.md

// store_maintenance_test.go — Annotation eviction, concurrency, timestamp, and reset tests.
package annotation

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestStore_EvictExpiredEntries(t *testing.T) {
	store := NewStore(10 * time.Minute)
	defer store.Close()

	// Inject expired entries in all three maps
	store.mu.Lock()
	store.sessions[100] = &sessionEntry{
		Session:   &Session{TabID: 100, Timestamp: 1},
		ExpiresAt: time.Now().Add(-1 * time.Second),
	}
	store.details["expired-detail"] = &detailEntry{
		Detail:    Detail{CorrelationID: "expired-detail"},
		ExpiresAt: time.Now().Add(-1 * time.Second),
	}
	store.named["expired-named"] = &namedSessionEntry{
		Session:   &NamedSession{Name: "expired-named"},
		ExpiresAt: time.Now().Add(-1 * time.Second),
	}
	// Also add non-expired entries
	store.sessions[200] = &sessionEntry{
		Session:   &Session{TabID: 200, Timestamp: 2},
		ExpiresAt: time.Now().Add(10 * time.Minute),
	}
	store.details["valid-detail"] = &detailEntry{
		Detail:    Detail{CorrelationID: "valid-detail"},
		ExpiresAt: time.Now().Add(10 * time.Minute),
	}
	store.named["valid-named"] = &namedSessionEntry{
		Session:   &NamedSession{Name: "valid-named"},
		ExpiresAt: time.Now().Add(10 * time.Minute),
	}
	store.mu.Unlock()

	// Manually trigger eviction
	store.evictExpiredEntries()

	// Verify expired entries are gone
	if store.GetSession(100) != nil {
		t.Error("expected expired session to be evicted")
	}
	if _, found := store.GetDetail("expired-detail"); found {
		t.Error("expected expired detail to be evicted")
	}
	if store.GetNamedSession("expired-named") != nil {
		t.Error("expected expired named session to be evicted")
	}

	// Verify valid entries remain with correct data
	validSession := store.GetSession(200)
	if validSession == nil {
		t.Fatal("expected valid session to remain")
	}
	if validSession.TabID != 200 {
		t.Errorf("valid session TabID = %d, want 200", validSession.TabID)
	}
	if validSession.Timestamp != 2 {
		t.Errorf("valid session Timestamp = %d, want 2", validSession.Timestamp)
	}

	validDetail, found := store.GetDetail("valid-detail")
	if !found {
		t.Fatal("expected valid detail to remain")
	}
	if validDetail.CorrelationID != "valid-detail" {
		t.Errorf("valid detail CorrelationID = %q, want 'valid-detail'", validDetail.CorrelationID)
	}

	validNamed := store.GetNamedSession("valid-named")
	if validNamed == nil {
		t.Fatal("expected valid named session to remain")
	}
	if validNamed.Name != "valid-named" {
		t.Errorf("valid named session Name = %q, want 'valid-named'", validNamed.Name)
	}
}

// --- Additional coverage: StoreDetail overwrites existing ---

func TestStore_StoreDetail_Overwrite(t *testing.T) {
	store := NewStore(10 * time.Minute)
	defer store.Close()

	store.StoreDetail("corr1", Detail{
		CorrelationID: "corr1",
		Selector:      "div.old",
	})
	store.StoreDetail("corr1", Detail{
		CorrelationID: "corr1",
		Selector:      "div.new",
	})

	got, found := store.GetDetail("corr1")
	if !found {
		t.Fatal("expected to find detail")
	}
	if got.Selector != "div.new" {
		t.Errorf("expected overwritten selector 'div.new', got %q", got.Selector)
	}
}

// --- Additional coverage: Multiple evictions when storing 2x max ---

func TestStore_MultipleEvictions(t *testing.T) {
	store := NewStore(10 * time.Minute)
	defer store.Close()

	total := MaxSessions * 2
	for i := 1; i <= total; i++ {
		store.StoreSession(i, &Session{
			TabID:     i,
			Timestamp: int64(i),
		})
	}

	store.mu.RLock()
	count := len(store.sessions)
	store.mu.RUnlock()

	if count != MaxSessions {
		t.Errorf("expected %d sessions after multiple evictions, got %d", MaxSessions, count)
	}

	// The oldest half should be evicted
	for i := 1; i <= MaxSessions; i++ {
		if store.GetSession(i) != nil {
			t.Errorf("expected tab %d to be evicted", i)
		}
	}

	// The newest MaxSessions should remain
	for i := MaxSessions + 1; i <= total; i++ {
		if store.GetSession(i) == nil {
			t.Errorf("expected tab %d to still exist", i)
		}
	}
}

// --- Additional coverage: AppendToNamedSession updates TTL ---

func TestStore_AppendToNamedSession_UpdatesTTL(t *testing.T) {
	store := NewStore(10 * time.Minute)
	defer store.Close()
	clock := useAnnotationTestClock(store)

	store.AppendToNamedSession("ttl-test", &Session{TabID: 1})

	store.mu.RLock()
	firstExpiry := store.named["ttl-test"].ExpiresAt
	store.mu.RUnlock()

	clock.Advance(2 * time.Millisecond)

	store.AppendToNamedSession("ttl-test", &Session{TabID: 2})

	store.mu.RLock()
	secondExpiry := store.named["ttl-test"].ExpiresAt
	store.mu.RUnlock()

	if !secondExpiry.After(firstExpiry) {
		t.Error("expected TTL to be refreshed after append")
	}
}

// --- Additional coverage: Concurrent read/write with named sessions ---

func TestStore_ConcurrentNamedSessions(t *testing.T) {
	store := NewStore(10 * time.Minute)
	defer store.Close()

	const goroutines = 20
	const iterations = 50
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for g := 0; g < goroutines; g++ {
		go func(id int) {
			defer wg.Done()
			sessionName := fmt.Sprintf("concurrent-%d", id)
			for i := 0; i < iterations; i++ {
				store.AppendToNamedSession(sessionName, &Session{
					TabID:     id*iterations + i,
					Timestamp: int64(id*iterations + i),
				})
				store.GetNamedSession(sessionName)
				store.ListNamedSessions()
			}
		}(g)
	}
	wg.Wait()
	// If we get here without panic or data race, the test passes
}

// --- Additional coverage: GetSession with expired entry injected directly ---

func TestStore_GetSession_Expired_Direct(t *testing.T) {
	store := NewStore(10 * time.Minute)
	defer store.Close()

	store.mu.Lock()
	store.sessions[5] = &sessionEntry{
		Session:   &Session{TabID: 5, PageURL: "https://expired.com", Timestamp: 1},
		ExpiresAt: time.Now().Add(-1 * time.Second),
	}
	store.mu.Unlock()

	got := store.GetSession(5)
	if got != nil {
		t.Error("expected nil for expired session")
	}
}

// --- Additional coverage: GetLatestSession with multiple tabs ---

func TestStore_GetLatestSession_MultipleTabs(t *testing.T) {
	store := NewStore(10 * time.Minute)
	defer store.Close()

	for i := 0; i < 10; i++ {
		store.StoreSession(i, &Session{
			TabID:     i,
			Timestamp: int64(i * 100),
		})
	}

	got := store.GetLatestSession()
	if got == nil {
		t.Fatal("expected session")
	}
	if got.TabID != 9 {
		t.Errorf("expected tab 9 (highest timestamp), got tab %d", got.TabID)
	}
}

// --- Additional coverage: All expired in GetLatestSession returns nil ---

func TestStore_GetLatestSession_AllExpired(t *testing.T) {
	store := NewStore(10 * time.Minute)
	defer store.Close()

	store.mu.Lock()
	for i := 0; i < 5; i++ {
		store.sessions[i] = &sessionEntry{
			Session:   &Session{TabID: i, Timestamp: int64(i * 100)},
			ExpiresAt: time.Now().Add(-1 * time.Second),
		}
	}
	store.mu.Unlock()

	got := store.GetLatestSession()
	if got != nil {
		t.Error("expected nil when all sessions are expired")
	}
}

// --- Additional coverage: WaitForNamedSession returns named session ---

func TestStore_WaitForNamedSession_Returns(t *testing.T) {
	store := NewStore(10 * time.Minute)
	defer store.Close()
	clock := useAnnotationTestClock(store)

	store.MarkDrawStarted()
	clock.Advance(time.Millisecond)

	result := make(chan struct {
		session  *NamedSession
		timedOut bool
	}, 1)
	go func() {
		session, timedOut := store.WaitForNamedSession("wait-test", 2*time.Second)
		result <- struct {
			session  *NamedSession
			timedOut bool
		}{session: session, timedOut: timedOut}
	}()
	store.AppendToNamedSession("wait-test", &Session{
		TabID:       1,
		PageURL:     "https://example.com/waited",
		Timestamp:   clock.Now().UnixMilli(),
		Annotations: []Annotation{{Text: "waited"}},
	})
	waited := <-result
	ns, timedOut := waited.session, waited.timedOut
	if timedOut {
		t.Fatal("expected named session but got timeout")
	}
	if ns == nil {
		t.Fatal("expected named session, got nil")
	}
	if ns.Name != "wait-test" {
		t.Errorf("expected name 'wait-test', got %q", ns.Name)
	}
	if len(ns.Pages) != 1 {
		t.Fatalf("expected 1 page, got %d", len(ns.Pages))
	}
	if ns.Pages[0].PageURL != "https://example.com/waited" {
		t.Errorf("expected page URL 'https://example.com/waited', got %q", ns.Pages[0].PageURL)
	}
	if len(ns.Pages[0].Annotations) != 1 || ns.Pages[0].Annotations[0].Text != "waited" {
		t.Errorf("expected annotation text 'waited', got %+v", ns.Pages[0].Annotations)
	}
}

// --- Additional coverage: Close unblocks WaitForSession with timedOut=false ---

func TestStore_Close_UnblocksWait(t *testing.T) {
	store := NewStore(10 * time.Minute)

	store.MarkDrawStarted()

	result := make(chan struct {
		session  *Session
		timedOut bool
	}, 1)
	go func() {
		session, timedOut := store.WaitForSession(5 * time.Second)
		result <- struct {
			session  *Session
			timedOut bool
		}{session: session, timedOut: timedOut}
	}()
	store.Close()
	waited := <-result
	session, timedOut := waited.session, waited.timedOut
	if session != nil {
		t.Error("expected nil session after close")
	}
	// When store is closed, waitForCondition returns (nil, false)
	if timedOut {
		t.Error("expected timedOut=false after close (done channel)")
	}
}

func TestStore_DetailEvictionCap(t *testing.T) {
	store := NewStore(10 * time.Minute)
	defer store.Close()

	// Store MaxDetails + 10 entries
	for i := 0; i < MaxDetails+10; i++ {
		store.StoreDetail(fmt.Sprintf("detail-%d", i), Detail{
			CorrelationID: fmt.Sprintf("detail-%d", i),
			Selector:      fmt.Sprintf("div.item-%d", i),
		})
	}

	// Count should never exceed MaxDetails + 1 (at most one over before eviction)
	store.mu.RLock()
	count := len(store.details)
	store.mu.RUnlock()

	if count > MaxDetails+1 {
		t.Errorf("expected detail count <= %d, got %d", MaxDetails+1, count)
	}

	// The latest entries should still be retrievable
	_, found := store.GetDetail(fmt.Sprintf("detail-%d", MaxDetails+9))
	if !found {
		t.Error("expected latest detail entry to exist")
	}
}

func TestStore_TakeWaiter_RemovesAndReturnsSessionName(t *testing.T) {
	store := NewStore(10 * time.Minute)
	defer store.Close()

	store.RegisterWaiter("ann_1", "qa", "")
	store.RegisterWaiter("ann_2", "", "")

	sessionName, urlFilter, found := store.TakeWaiter("ann_1")
	if !found {
		t.Fatal("expected waiter ann_1 to be found")
	}
	if sessionName != "qa" {
		t.Fatalf("sessionName = %q, want qa", sessionName)
	}
	if urlFilter != "" {
		t.Fatalf("urlFilter = %q, want empty", urlFilter)
	}

	_, _, found = store.TakeWaiter("ann_1")
	if found {
		t.Fatal("expected waiter ann_1 to be removed after first take")
	}

	store.mu.RLock()
	remaining := len(store.waiters)
	remainingID := ""
	if remaining > 0 {
		remainingID = store.waiters[0].CorrelationID
	}
	store.mu.RUnlock()
	if remaining != 1 {
		t.Fatalf("remaining waiters = %d, want 1", remaining)
	}
	if remainingID != "ann_2" {
		t.Fatalf("remaining waiter = %q, want ann_2", remainingID)
	}
}

func TestStore_SessionTTL_Is2Hours(t *testing.T) {
	store := NewStore(10 * time.Minute)
	defer store.Close()

	// Verify the default TTL is 2 hours
	if store.sessionTTL != 2*time.Hour {
		t.Fatalf("expected sessionTTL = 2h, got %v", store.sessionTTL)
	}

	// Session stored with a manually backdated expiry at 90 minutes should still be live
	store.StoreSession(1, &Session{
		TabID:     1,
		Timestamp: time.Now().UnixMilli(),
		PageURL:   "https://example.com",
	})

	// Simulate 90 minutes elapsed by adjusting the session entry's ExpiresAt
	store.mu.Lock()
	entry := store.sessions[1]
	entry.ExpiresAt = time.Now().Add(30 * time.Minute) // 120min TTL - 90min elapsed = 30min remaining
	store.mu.Unlock()

	got := store.GetSession(1)
	if got == nil {
		t.Error("expected session to still be retrievable after 90 minutes (within 2h TTL)")
	}
}

func TestStore_FindAnnotationTimestamp_Anonymous(t *testing.T) {
	store := NewStore(10 * time.Minute)
	defer store.Close()

	store.StoreSession(1, &Session{
		TabID: 1, Timestamp: 1000, PageURL: "https://a.com",
		Annotations: []Annotation{
			{ID: "a1", CorrelationID: "corr_1", Timestamp: 111},
			{ID: "a2", CorrelationID: "corr_2", Timestamp: 222},
		},
	})
	store.StoreSession(2, &Session{
		TabID: 2, Timestamp: 2000, PageURL: "https://b.com",
		Annotations: []Annotation{
			{ID: "b1", CorrelationID: "corr_3", Timestamp: 333},
		},
	})

	// Found in first session
	if ts := store.FindAnnotationTimestamp("corr_1"); ts != 111 {
		t.Errorf("expected 111, got %d", ts)
	}
	// Found in second session
	if ts := store.FindAnnotationTimestamp("corr_3"); ts != 333 {
		t.Errorf("expected 333, got %d", ts)
	}
	// Not found
	if ts := store.FindAnnotationTimestamp("nonexistent"); ts != 0 {
		t.Errorf("expected 0, got %d", ts)
	}
}

func TestStore_FindAnnotationTimestamp_Named(t *testing.T) {
	store := NewStore(10 * time.Minute)
	defer store.Close()

	store.AppendToNamedSession("review", &Session{
		TabID: 3, PageURL: "https://c.com", Timestamp: 3000,
		Annotations: []Annotation{
			{ID: "n1", CorrelationID: "named_corr_1", Timestamp: 444},
		},
	})

	if ts := store.FindAnnotationTimestamp("named_corr_1"); ts != 444 {
		t.Errorf("expected 444, got %d", ts)
	}
	// Not found in named
	if ts := store.FindAnnotationTimestamp("wrong_id"); ts != 0 {
		t.Errorf("expected 0, got %d", ts)
	}
}

func TestStore_FindAnnotationTimestamp_SkipsExpired(t *testing.T) {
	store := NewStore(10 * time.Minute)
	defer store.Close()

	store.StoreSession(1, &Session{
		TabID: 1, Timestamp: 1000, PageURL: "https://expired.com",
		Annotations: []Annotation{
			{ID: "e1", CorrelationID: "exp_corr", Timestamp: 555},
		},
	})

	// Expire the session
	store.mu.Lock()
	store.sessions[1].ExpiresAt = time.Now().Add(-1 * time.Second)
	store.mu.Unlock()

	if ts := store.FindAnnotationTimestamp("exp_corr"); ts != 0 {
		t.Errorf("expected 0 for expired session, got %d", ts)
	}

	// Named session expired
	store.AppendToNamedSession("old", &Session{
		TabID: 4, PageURL: "https://old.com", Timestamp: 4000,
		Annotations: []Annotation{
			{ID: "n_exp", CorrelationID: "named_exp_corr", Timestamp: 666},
		},
	})
	store.mu.Lock()
	store.named["old"].ExpiresAt = time.Now().Add(-1 * time.Second)
	store.mu.Unlock()

	if ts := store.FindAnnotationTimestamp("named_exp_corr"); ts != 0 {
		t.Errorf("expected 0 for expired named session, got %d", ts)
	}
}

func TestStore_ClearAll_ClearsAllAnnotationState(t *testing.T) {
	store := NewStore(10 * time.Minute)
	defer store.Close()

	now := time.Now().UnixMilli()
	store.StoreSession(1, &Session{
		TabID:     1,
		Timestamp: now,
		Annotations: []Annotation{
			{ID: "ann_1", CorrelationID: "detail_1", Timestamp: now},
		},
	})
	store.StoreDetail("detail_1", Detail{CorrelationID: "detail_1", Selector: "#target"})
	store.AppendToNamedSession("qa", &Session{
		TabID:     1,
		Timestamp: now,
		Annotations: []Annotation{
			{ID: "ann_named", CorrelationID: "detail_named", Timestamp: now},
		},
	})
	store.RegisterWaiter("ann_wait", "", "")

	counts := store.ClearAll()
	if counts.Sessions != 1 {
		t.Fatalf("cleared sessions = %d, want 1", counts.Sessions)
	}
	if counts.Details != 1 {
		t.Fatalf("cleared details = %d, want 1", counts.Details)
	}
	if counts.NamedSessions != 1 {
		t.Fatalf("cleared named sessions = %d, want 1", counts.NamedSessions)
	}
	if counts.Waiters != 1 {
		t.Fatalf("cleared waiters = %d, want 1", counts.Waiters)
	}

	if got := store.GetLatestSession(); got != nil {
		t.Fatalf("expected latest session cleared, got %+v", got)
	}
	if got := store.GetNamedSession("qa"); got != nil {
		t.Fatalf("expected named session cleared, got %+v", got)
	}
	if _, found := store.GetDetail("detail_1"); found {
		t.Fatal("expected detail cleared")
	}
	if _, _, found := store.TakeWaiter("ann_wait"); found {
		t.Fatal("expected waiter cleared")
	}
}

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
	clock := useAnnotationTestClock(store)

	store.MarkDrawStarted()
	clock.Advance(time.Millisecond)

	result := make(chan struct {
		session  *NamedSession
		timedOut bool
	}, 1)
	go func() {
		session, timedOut := store.WaitForNamedSession("qa", 2*time.Second)
		result <- struct {
			session  *NamedSession
			timedOut bool
		}{session: session, timedOut: timedOut}
	}()
	store.AppendToNamedSession("qa", &Session{
		TabID:       1,
		Timestamp:   clock.Now().UnixMilli(),
		Annotations: []Annotation{{Text: "waited"}},
	})

	waited := <-result
	ns, timedOut := waited.session, waited.timedOut

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
	clock := useAnnotationTestClock(store)

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
		clock.Advance(time.Millisecond)
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
