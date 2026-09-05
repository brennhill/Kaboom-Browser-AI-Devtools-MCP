// diff_test.go — Tests security snapshot lifecycle and retention.
// Docs: docs/features/feature/security-hardening/index.md

package diff

import (
	"fmt"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

type diffTestClock struct{ now time.Time }

func (c *diffTestClock) Now() time.Time              { return c.now }
func (c *diffTestClock) Advance(delta time.Duration) { c.now = c.now.Add(delta) }

func useDiffTestClock(mgr *Manager) *diffTestClock {
	clock := &diffTestClock{now: time.Unix(100, 0)}
	mgr.now = clock.Now
	return clock
}

func mustTakeSnapshot(t *testing.T, mgr *Manager, name string, bodies []types.NetworkBody) {
	t.Helper()
	if _, err := mgr.TakeSnapshot(name, bodies); err != nil {
		t.Fatalf("TakeSnapshot(%q) failed: %v", name, err)
	}
}

func mustCompareSnapshots(t *testing.T, mgr *Manager, beforeBodies, afterBodies []types.NetworkBody) *Result {
	t.Helper()
	mustTakeSnapshot(t, mgr, "before", beforeBodies)
	mustTakeSnapshot(t, mgr, "after", afterBodies)

	result, err := mgr.Compare("before", "after", nil)
	if err != nil {
		t.Fatalf("Compare(before, after) failed: %v", err)
	}
	return result
}

func TestSnapshotCapture(t *testing.T) {
	t.Parallel()
	mgr := NewManager()

	bodies := []types.NetworkBody{
		{
			URL:         "https://myapp.com/",
			Method:      "GET",
			ContentType: "text/html",
			ResponseHeaders: map[string]string{
				"X-Frame-Options":           "DENY",
				"X-Content-Type-Options":    "nosniff",
				"Strict-Transport-Security": "max-age=31536000",
				"Set-Cookie":                "session=abc123; HttpOnly; Secure; SameSite=Strict",
			},
			HasAuthHeader: true,
		},
		{
			URL:         "http://api.example.com/data",
			Method:      "POST",
			ContentType: "application/json",
			ResponseHeaders: map[string]string{
				"Content-Type": "application/json",
			},
			HasAuthHeader: false,
		},
	}

	snap, err := mgr.TakeSnapshot("baseline", bodies)
	if err != nil {
		t.Fatal(err)
	}

	if snap.Name != "baseline" {
		t.Errorf("expected name 'baseline', got %q", snap.Name)
	}

	// Check headers were captured for HTML response origin
	origin := "https://myapp.com"
	if snap.Headers[origin] == nil {
		t.Fatal("expected headers for https://myapp.com")
	}
	if snap.Headers[origin]["X-Frame-Options"] != "DENY" {
		t.Errorf("expected X-Frame-Options 'DENY', got %q", snap.Headers[origin]["X-Frame-Options"])
	}
	if snap.Headers[origin]["X-Content-Type-Options"] != "nosniff" {
		t.Errorf("expected X-Content-Type-Options 'nosniff', got %q", snap.Headers[origin]["X-Content-Type-Options"])
	}

	// Check cookies were captured
	if len(snap.Cookies[origin]) == 0 {
		t.Fatal("expected cookies for https://myapp.com")
	}
	cookie := snap.Cookies[origin][0]
	if cookie.Name != "session" {
		t.Errorf("expected cookie name 'session', got %q", cookie.Name)
	}
	if !cookie.HttpOnly {
		t.Error("expected HttpOnly flag on cookie")
	}
	if !cookie.Secure {
		t.Error("expected Secure flag on cookie")
	}
	if cookie.SameSite != "strict" {
		t.Errorf("expected SameSite 'strict', got %q", cookie.SameSite)
	}

	// Check auth was captured
	if !snap.Auth["GET https://myapp.com/"] {
		t.Error("expected auth=true for GET https://myapp.com/")
	}
	if snap.Auth["POST http://api.example.com/data"] {
		t.Error("expected auth=false for POST http://api.example.com/data")
	}

	// Check transport was captured
	if snap.Transport[origin] != "https" {
		t.Errorf("expected transport 'https' for %s, got %q", origin, snap.Transport[origin])
	}
	if snap.Transport["http://api.example.com"] != "http" {
		t.Errorf("expected transport 'http' for http://api.example.com, got %q", snap.Transport["http://api.example.com"])
	}
}

func TestSnapshotNameValidation(t *testing.T) {
	t.Parallel()
	mgr := NewManager()
	bodies := []types.NetworkBody{{URL: "https://myapp.com/", ContentType: "text/html", ResponseHeaders: map[string]string{"X-Frame-Options": "DENY"}}}

	// Empty name
	_, err := mgr.TakeSnapshot("", bodies)
	if err == nil {
		t.Error("expected error for empty name")
	}

	// Reserved name "current"
	_, err = mgr.TakeSnapshot("current", bodies)
	if err == nil {
		t.Error("expected error for reserved name 'current'")
	}

	// Too long name (>50 chars)
	longName := "abcdefghijklmnopqrstuvwxyz0123456789abcdefghijklmnop" // 52 chars
	_, err = mgr.TakeSnapshot(longName, bodies)
	if err == nil {
		t.Error("expected error for name exceeding 50 chars")
	}

	// Valid name should work
	_, err = mgr.TakeSnapshot("valid-name", bodies)
	if err != nil {
		t.Errorf("unexpected error for valid name: %v", err)
	}
}

func TestSnapshotMaxCount(t *testing.T) {
	t.Parallel()
	mgr := NewManager()
	bodies := []types.NetworkBody{{URL: "https://myapp.com/", ContentType: "text/html", ResponseHeaders: map[string]string{"X-Frame-Options": "DENY"}}}

	// Create 5 snapshots (max)
	for i := 1; i <= 5; i++ {
		name := fmt.Sprintf("snap%d", i)
		_, err := mgr.TakeSnapshot(name, bodies)
		if err != nil {
			t.Fatalf("failed to create snapshot %d: %v", i, err)
		}
	}

	// Verify all 5 exist
	list := mgr.ListSnapshots()
	if len(list) != 5 {
		t.Fatalf("expected 5 snapshots, got %d", len(list))
	}

	// Create 6th snapshot — should evict oldest (snap1)
	_, err := mgr.TakeSnapshot("snap6", bodies)
	if err != nil {
		t.Fatal(err)
	}

	list = mgr.ListSnapshots()
	if len(list) != 5 {
		t.Fatalf("expected 5 snapshots after eviction, got %d", len(list))
	}

	// snap1 should be evicted
	for _, entry := range list {
		if entry.Name == "snap1" {
			t.Error("snap1 should have been evicted")
		}
	}

	// snap6 should exist
	found := false
	for _, entry := range list {
		if entry.Name == "snap6" {
			found = true
			break
		}
	}
	if !found {
		t.Error("snap6 should exist after eviction")
	}
}

func TestSnapshotTTL(t *testing.T) {
	t.Parallel()
	mgr := NewManager()
	clock := useDiffTestClock(mgr)
	mgr.ttl = time.Millisecond // Very short TTL for testing

	bodies := []types.NetworkBody{{URL: "https://myapp.com/", ContentType: "text/html", ResponseHeaders: map[string]string{"X-Frame-Options": "DENY"}}}
	_, err := mgr.TakeSnapshot("old", bodies)
	if err != nil {
		t.Fatal(err)
	}

	clock.Advance(2 * time.Millisecond)

	_, err = mgr.Compare("old", "current", bodies)
	if err == nil {
		t.Error("expected error for expired snapshot")
	}
}

func TestSecurityDiffListSnapshots(t *testing.T) {
	t.Parallel()
	mgr := NewManager()
	bodies := []types.NetworkBody{{URL: "https://myapp.com/", ContentType: "text/html", ResponseHeaders: map[string]string{"X-Frame-Options": "DENY"}}}

	_, _ = mgr.TakeSnapshot("alpha", bodies)
	_, _ = mgr.TakeSnapshot("beta", bodies)

	list := mgr.ListSnapshots()
	if len(list) != 2 {
		t.Fatalf("expected 2 snapshots in list, got %d", len(list))
	}

	// Verify names are present
	names := make(map[string]bool)
	for _, entry := range list {
		names[entry.Name] = true
		if entry.TakenAt == "" {
			t.Errorf("expected non-empty TakenAt for %s", entry.Name)
		}
		if entry.Age == "" {
			t.Errorf("expected non-empty Age for %s", entry.Name)
		}
	}
	if !names["alpha"] {
		t.Error("expected 'alpha' in list")
	}
	if !names["beta"] {
		t.Error("expected 'beta' in list")
	}
}

func TestSecurityDiffCompareAgainstCurrent(t *testing.T) {
	t.Parallel()
	mgr := NewManager()

	// Baseline snapshot with auth
	baselineBodies := []types.NetworkBody{
		{
			URL:           "https://api.myapp.com/users",
			Method:        "GET",
			ContentType:   "application/json",
			HasAuthHeader: true,
		},
	}

	_, err := mgr.TakeSnapshot("baseline", baselineBodies)
	if err != nil {
		t.Fatal(err)
	}

	// Current bodies: auth dropped
	currentBodies := []types.NetworkBody{
		{
			URL:           "https://api.myapp.com/users",
			Method:        "GET",
			ContentType:   "application/json",
			HasAuthHeader: false,
		},
	}

	// compare_to empty string uses currentBodies
	result, err := mgr.Compare("baseline", "", currentBodies)
	if err != nil {
		t.Fatal(err)
	}

	if result.Verdict != "regressed" {
		t.Errorf("expected 'regressed', got %q", result.Verdict)
	}

	found := false
	for _, r := range result.Regressions {
		if r.Change == "auth_removed" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected auth_removed regression when comparing against current")
	}

	// Also test with compare_to = "current"
	result2, err := mgr.Compare("baseline", "current", currentBodies)
	if err != nil {
		t.Fatal(err)
	}
	if result2.Verdict != "regressed" {
		t.Errorf("expected 'regressed' with 'current', got %q", result2.Verdict)
	}
}

func TestSecurityDiffLRUEviction(t *testing.T) {
	t.Parallel()
	mgr := NewManager()
	bodies := []types.NetworkBody{
		{URL: "https://app.com/", ContentType: "text/html", Status: 200,
			ResponseHeaders: map[string]string{"X-Frame-Options": "DENY"}},
	}

	// Fill to max (5 snapshots) + 1 to trigger eviction
	for i := 0; i < 6; i++ {
		name := fmt.Sprintf("snap%d", i)
		_, err := mgr.TakeSnapshot(name, bodies)
		if err != nil {
			t.Fatalf("TakeSnapshot(%q) failed: %v", name, err)
		}
	}

	// First snapshot should be evicted
	list := mgr.ListSnapshots()
	for _, s := range list {
		if s.Name == "snap0" {
			t.Error("snap0 should have been evicted by LRU")
		}
	}
}

func TestSecurityDiffCompareWithCurrent(t *testing.T) {
	t.Parallel()
	mgr := NewManager()
	bodies := []types.NetworkBody{
		{URL: "https://app.com/", ContentType: "text/html", Status: 200,
			ResponseHeaders: map[string]string{
				"X-Frame-Options":           "DENY",
				"Strict-Transport-Security": "max-age=31536000",
				"X-Content-Type-Options":    "nosniff",
				"Content-Security-Policy":   "default-src 'self'",
				"Referrer-Policy":           "strict-origin",
				"Permissions-Policy":        "camera=()",
			},
			HasAuthHeader: true},
	}

	mustTakeSnapshot(t, mgr, "baseline", bodies)

	// Compare baseline vs "current" with all headers removed
	currentBodies := []types.NetworkBody{
		{URL: "https://app.com/", ContentType: "text/html", Status: 200,
			ResponseHeaders: map[string]string{},
			HasAuthHeader:   false},
	}

	result, err := mgr.Compare("baseline", "current", currentBodies)
	if err != nil {
		t.Fatalf("Compare failed: %v", err)
	}

	if len(result.Regressions) == 0 {
		t.Error("expected regressions when headers removed")
	}
	// Verify all header recommendation paths are exercised
	if result.Summary.ByCategory["headers"] == 0 {
		t.Error("expected header regressions")
	}
}

func TestSecurityDiffSnapshotOverwrite(t *testing.T) {
	t.Parallel()
	mgr := NewManager()
	bodies := []types.NetworkBody{
		{URL: "https://app.com/", ContentType: "text/html", Status: 200,
			ResponseHeaders: map[string]string{"X-Frame-Options": "DENY"}},
	}

	mustTakeSnapshot(t, mgr, "same", bodies)
	mustTakeSnapshot(t, mgr, "same", bodies)

	list := mgr.ListSnapshots()
	count := 0
	for _, s := range list {
		if s.Name == "same" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected 1 snapshot named 'same', got %d", count)
	}
}

// requireSnapshotEntry returns the named entry from ListSnapshots, failing the
// test when it is absent. Callers rely on it so an empty listing can never be
// mistaken for a satisfied expectation.
func requireSnapshotEntry(t *testing.T, mgr *Manager, name string) SnapshotListEntry {
	t.Helper()
	list := mgr.ListSnapshots()
	for _, entry := range list {
		if entry.Name == name {
			return entry
		}
	}
	t.Fatalf("snapshot %q missing from ListSnapshots (%d entries returned)", name, len(list))
	return SnapshotListEntry{}
}

func TestSecurityDiffExpiredSnapshot(t *testing.T) {
	t.Parallel()
	mgr := NewManager()
	clock := useDiffTestClock(mgr)
	mgr.ttl = 1 * time.Millisecond // Very short TTL

	bodies := []types.NetworkBody{
		{URL: "https://app.com/", ContentType: "text/html", Status: 200,
			ResponseHeaders: map[string]string{"X-Frame-Options": "DENY"}},
	}

	mustTakeSnapshot(t, mgr, "old", bodies)

	// Discriminating control: before the TTL elapses the same entry must report
	// Expired=false. This proves ListSnapshots derives expiry from the clock
	// rather than reporting a constant — or nothing at all.
	if fresh := requireSnapshotEntry(t, mgr, "old"); fresh.Expired {
		t.Fatalf("control: snapshot %q must not be expired before its %v TTL elapses", "old", mgr.ttl)
	}

	clock.Advance(5 * time.Millisecond)

	if aged := requireSnapshotEntry(t, mgr, "old"); !aged.Expired {
		t.Error("expected expired=true for old snapshot")
	}
}
