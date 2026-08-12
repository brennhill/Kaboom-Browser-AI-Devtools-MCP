// Purpose: Tests for annotation store CRUD operations.
// Docs: docs/features/feature/annotated-screenshots/index.md

// store_lifecycle_test.go — Annotation store expiry, wakeup, and close tests.
package annotation

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
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

func TestLoadDrawSessionRejectsTraversal(t *testing.T) {
	t.Parallel()
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	response := LoadDrawSession(NewStore(time.Minute), req, json.RawMessage(`{"file":"../secret.json"}`), t.TempDir(), nil)
	var result mcp.MCPToolResult
	if err := json.Unmarshal(response.Result, &result); err != nil {
		t.Fatal(err)
	}
	if !result.IsError {
		t.Fatal("expected path traversal to fail")
	}
}

func TestDrawSessionHistoryFiltersSortsAndReportsDirectoryFailures(t *testing.T) {
	t.Parallel()
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	if result := decodeDrawResult(t, ListDrawHistory(req, "", errors.New("unavailable"), 0)); !result.IsError {
		t.Fatal("directory resolution error was ignored")
	}
	if result := decodeDrawResult(t, ListDrawHistory(req, filepath.Join(t.TempDir(), "missing"), nil, 0)); !result.IsError {
		t.Fatal("missing directory error was ignored")
	}
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "draw-session-dir.json"), 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"ignored.json", "draw-session-old.json", "draw-session-new.json"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(`{}`), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	old := time.Now().Add(-time.Hour)
	_ = os.Chtimes(filepath.Join(dir, "draw-session-old.json"), old, old)
	result := decodeDrawResult(t, ListDrawHistory(req, dir, nil, 0))
	if result.IsError || !strings.Contains(result.Content[0].Text, `"count":2`) ||
		strings.Index(result.Content[0].Text, "draw-session-new") > strings.Index(result.Content[0].Text, "draw-session-old") {
		t.Fatalf("history = %+v", result)
	}
}

func TestLoadDrawSessionHydratesCanonicalStores(t *testing.T) {
	t.Parallel()
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	dir := t.TempDir()
	store := NewStore(time.Minute)
	t.Cleanup(store.Close)
	for _, args := range []json.RawMessage{nil, json.RawMessage(`not-json`), json.RawMessage(`{"file":"missing.json"}`)} {
		if result := decodeDrawResult(t, LoadDrawSession(store, req, args, dir, nil)); !result.IsError {
			t.Fatalf("LoadDrawSession(%s) accepted invalid input", args)
		}
	}
	if result := decodeDrawResult(t, LoadDrawSession(store, req, json.RawMessage(`{"file":"session.json"}`), dir, errors.New("blocked"))); !result.IsError {
		t.Fatal("directory error was ignored")
	}
	if err := os.WriteFile(filepath.Join(dir, "broken.json"), []byte(`not-json`), 0o600); err != nil {
		t.Fatal(err)
	}
	if result := decodeDrawResult(t, LoadDrawSession(store, req, json.RawMessage(`{"file":"broken.json"}`), dir, nil)); !result.IsError {
		t.Fatal("corrupt session was accepted")
	}
	payload := `{"tab_id":7,"page_url":"https://example.test","timestamp":1234,"annot_session_name":"review","annotations":[],"element_details":{"corr":{"tag_name":"button"}}}`
	if err := os.WriteFile(filepath.Join(dir, "draw-session-good.json"), []byte(payload), 0o600); err != nil {
		t.Fatal(err)
	}
	result := decodeDrawResult(t, LoadDrawSession(store, req, json.RawMessage(`{"file":"draw-session-good.json"}`), dir, nil))
	if result.IsError || !strings.Contains(result.Content[0].Text, `"annot_session":"review"`) {
		t.Fatalf("loaded session = %+v", result)
	}
	if session := store.GetSession(7); session == nil || session.Timestamp == 0 {
		t.Fatalf("hydrated tab session = %#v", session)
	}
	if _, ok := store.GetDetail("corr"); !ok {
		t.Fatal("element detail was not hydrated")
	}
	// Loading the same persisted page must not duplicate it in the named session.
	_ = LoadDrawSession(store, req, json.RawMessage(`{"file":"draw-session-good.json"}`), dir, nil)
	if named := store.GetNamedSession("review"); named == nil || len(named.Pages) != 1 {
		t.Fatalf("named session = %#v", named)
	}
}

func decodeDrawResult(t *testing.T, response mcp.JSONRPCResponse) mcp.MCPToolResult {
	t.Helper()
	var result mcp.MCPToolResult
	if err := json.Unmarshal(response.Result, &result); err != nil {
		t.Fatal(err)
	}
	return result
}

// draw_history returned every session it could find — 4051 of them on a real
// machine, 1,084,063 bytes — and leaned on the 100KB response clamp to cut it
// down. That is a safety net doing a paginator's job: the clamp keeps whatever
// bytes came first, which has no relationship to what the caller wanted, and
// the truncation used to leave the body unparseable.
func TestListDrawHistoryAppliesADefaultPageSize(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < drawHistoryDefaultLimit+40; i++ {
		name := filepath.Join(dir, "draw-session-"+strconv.Itoa(i)+"-1700000000000.json")
		if err := os.WriteFile(name, []byte(`{"annotations":[]}`), 0o600); err != nil {
			t.Fatalf("seed session: %v", err)
		}
	}

	resp := ListDrawHistory(mcp.JSONRPCRequest{}, dir, nil, 0)
	payload := drawHistoryPayload(t, resp)

	sessions, _ := payload["sessions"].([]any)
	if len(sessions) != drawHistoryDefaultLimit {
		t.Fatalf("returned %d sessions, want the default page of %d", len(sessions), drawHistoryDefaultLimit)
	}
	// The caller must be able to tell the listing was cut, from the data rather
	// than from prose it may never parse.
	if total, _ := payload["total"].(float64); int(total) != drawHistoryDefaultLimit+40 {
		t.Errorf("total = %v, want every session counted even when not returned", payload["total"])
	}
	if count, _ := payload["count"].(float64); int(count) != drawHistoryDefaultLimit {
		t.Errorf("count = %v, want the number actually returned", payload["count"])
	}
	if truncated, _ := payload["truncated"].(bool); !truncated {
		t.Error("truncated must be true when sessions were withheld")
	}
}

func TestListDrawHistoryHonoursAnExplicitLimit(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 12; i++ {
		name := filepath.Join(dir, "draw-session-"+strconv.Itoa(i)+"-1700000000000.json")
		if err := os.WriteFile(name, []byte(`{"annotations":[]}`), 0o600); err != nil {
			t.Fatalf("seed session: %v", err)
		}
	}

	resp := ListDrawHistory(mcp.JSONRPCRequest{}, dir, nil, 5)
	payload := drawHistoryPayload(t, resp)
	sessions, _ := payload["sessions"].([]any)
	if len(sessions) != 5 {
		t.Fatalf("returned %d sessions, want the requested 5", len(sessions))
	}
	if truncated, _ := payload["truncated"].(bool); !truncated {
		t.Error("truncated must be true when a limit withheld sessions")
	}
}

// A listing that fits must not claim it was cut.
func TestListDrawHistoryDoesNotClaimTruncationWhenComplete(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "draw-session-1-1700000000000.json"), []byte(`{}`), 0o600); err != nil {
		t.Fatalf("seed session: %v", err)
	}
	payload := drawHistoryPayload(t, ListDrawHistory(mcp.JSONRPCRequest{}, dir, nil, 0))
	if truncated, ok := payload["truncated"].(bool); ok && truncated {
		t.Error("a complete listing must not report truncated")
	}
}

func drawHistoryPayload(t *testing.T, resp mcp.JSONRPCResponse) map[string]any {
	t.Helper()
	raw, err := json.Marshal(resp.Result)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	var result mcp.MCPToolResult
	if err := json.Unmarshal(raw, &result); err != nil || len(result.Content) == 0 {
		t.Fatalf("unmarshal tool result: %v", err)
	}
	text := result.Content[0].Text
	start := strings.Index(text, "{")
	if start < 0 {
		t.Fatalf("no JSON body in %.120q", text)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(text[start:]), &payload); err != nil {
		t.Fatalf("parse payload: %v", err)
	}
	return payload
}
