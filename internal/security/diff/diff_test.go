// diff_test.go — Tests for snapshot capture and regression/improvement detection.
// Purpose: Tests for security posture diff and regression detection.
// Docs: docs/features/feature/security-hardening/index.md

package diff

import (
	"encoding/json"
	"fmt"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"testing"
	"time"
)

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
	mgr.ttl = time.Millisecond // Very short TTL for testing

	bodies := []types.NetworkBody{{URL: "https://myapp.com/", ContentType: "text/html", ResponseHeaders: map[string]string{"X-Frame-Options": "DENY"}}}
	_, err := mgr.TakeSnapshot("old", bodies)
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(2 * time.Millisecond)

	_, err = mgr.Compare("old", "current", bodies)
	if err == nil {
		t.Error("expected error for expired snapshot")
	}
}

func TestSecurityDiffHeaderRemoved(t *testing.T) {
	t.Parallel()
	mgr := NewManager()

	// Before: has X-Frame-Options
	beforeBodies := []types.NetworkBody{
		{
			URL:         "https://myapp.com/",
			ContentType: "text/html",
			ResponseHeaders: map[string]string{
				"X-Frame-Options":        "DENY",
				"X-Content-Type-Options": "nosniff",
			},
		},
	}

	// After: missing X-Frame-Options
	afterBodies := []types.NetworkBody{
		{
			URL:         "https://myapp.com/",
			ContentType: "text/html",
			ResponseHeaders: map[string]string{
				"X-Content-Type-Options": "nosniff",
			},
		},
	}

	result := mustCompareSnapshots(t, mgr, beforeBodies, afterBodies)

	if result.Verdict != "regressed" {
		t.Errorf("expected 'regressed', got %q", result.Verdict)
	}
	if len(result.Regressions) == 0 {
		t.Fatal("expected regressions")
	}
	found := false
	for _, r := range result.Regressions {
		if r.Header == "X-Frame-Options" && r.Change == "header_removed" {
			found = true
			if r.Severity != "warning" {
				t.Errorf("expected severity 'warning', got %q", r.Severity)
			}
			if r.Category != "headers" {
				t.Errorf("expected category 'headers', got %q", r.Category)
			}
			if r.Recommendation == "" {
				t.Error("expected non-empty recommendation")
			}
		}
	}
	if !found {
		t.Error("expected X-Frame-Options removal regression")
	}
}

func TestSecurityDiffHeaderAdded(t *testing.T) {
	t.Parallel()
	mgr := NewManager()

	// Before: no CSP
	beforeBodies := []types.NetworkBody{
		{
			URL:         "https://myapp.com/",
			ContentType: "text/html",
			ResponseHeaders: map[string]string{
				"X-Frame-Options": "DENY",
			},
		},
	}

	// After: has CSP
	afterBodies := []types.NetworkBody{
		{
			URL:         "https://myapp.com/",
			ContentType: "text/html",
			ResponseHeaders: map[string]string{
				"X-Frame-Options":         "DENY",
				"Content-Security-Policy": "default-src 'self'",
			},
		},
	}

	result := mustCompareSnapshots(t, mgr, beforeBodies, afterBodies)

	if result.Verdict != "improved" {
		t.Errorf("expected 'improved', got %q", result.Verdict)
	}
	if len(result.Improvements) == 0 {
		t.Fatal("expected improvements")
	}
	found := false
	for _, imp := range result.Improvements {
		if imp.Header == "Content-Security-Policy" && imp.Change == "header_added" {
			found = true
			if imp.Category != "headers" {
				t.Errorf("expected category 'headers', got %q", imp.Category)
			}
		}
	}
	if !found {
		t.Error("expected Content-Security-Policy addition improvement")
	}
}

func TestSecurityDiffCookieFlagLost(t *testing.T) {
	t.Parallel()
	mgr := NewManager()

	// Before: cookie has HttpOnly, Secure, SameSite
	beforeBodies := []types.NetworkBody{
		{
			URL:         "https://myapp.com/",
			ContentType: "text/html",
			ResponseHeaders: map[string]string{
				"Set-Cookie": "session=abc; HttpOnly; Secure; SameSite=Strict",
			},
		},
	}

	// After: cookie lost HttpOnly and Secure flags
	afterBodies := []types.NetworkBody{
		{
			URL:         "https://myapp.com/",
			ContentType: "text/html",
			ResponseHeaders: map[string]string{
				"Set-Cookie": "session=abc; SameSite=Strict",
			},
		},
	}

	result := mustCompareSnapshots(t, mgr, beforeBodies, afterBodies)

	if result.Verdict != "regressed" {
		t.Errorf("expected 'regressed', got %q", result.Verdict)
	}

	// Should have regressions for HttpOnly and Secure flag removal
	httpOnlyFound := false
	secureFound := false
	for _, r := range result.Regressions {
		if r.CookieName == "session" && r.Flag == "HttpOnly" && r.Change == "flag_removed" {
			httpOnlyFound = true
			if r.Severity != "warning" {
				t.Errorf("expected severity 'warning' for HttpOnly removal, got %q", r.Severity)
			}
		}
		if r.CookieName == "session" && r.Flag == "Secure" && r.Change == "flag_removed" {
			secureFound = true
		}
	}
	if !httpOnlyFound {
		t.Error("expected HttpOnly flag_removed regression")
	}
	if !secureFound {
		t.Error("expected Secure flag_removed regression")
	}
}

func TestSecurityDiffAuthDropped(t *testing.T) {
	t.Parallel()
	mgr := NewManager()

	// Before: endpoint has auth
	beforeBodies := []types.NetworkBody{
		{
			URL:           "https://api.myapp.com/users",
			Method:        "GET",
			ContentType:   "application/json",
			HasAuthHeader: true,
		},
	}

	// After: same endpoint, no auth
	afterBodies := []types.NetworkBody{
		{
			URL:           "https://api.myapp.com/users",
			Method:        "GET",
			ContentType:   "application/json",
			HasAuthHeader: false,
		},
	}

	result := mustCompareSnapshots(t, mgr, beforeBodies, afterBodies)

	if result.Verdict != "regressed" {
		t.Errorf("expected 'regressed', got %q", result.Verdict)
	}

	found := false
	for _, r := range result.Regressions {
		if r.Change == "auth_removed" && r.Endpoint == "GET https://api.myapp.com/users" {
			found = true
			if r.Severity != "critical" {
				t.Errorf("expected severity 'critical', got %q", r.Severity)
			}
			if r.Category != "auth" {
				t.Errorf("expected category 'auth', got %q", r.Category)
			}
		}
	}
	if !found {
		t.Error("expected auth_removed regression for GET https://api.myapp.com/users")
	}
}

func TestSecurityDiffTransportDowngrade(t *testing.T) {
	t.Parallel()
	mgr := NewManager()

	// Before: HTTPS
	beforeBodies := []types.NetworkBody{
		{
			URL:         "https://myapp.com/api/data",
			Method:      "GET",
			ContentType: "application/json",
		},
	}

	// After: HTTP (downgrade)
	afterBodies := []types.NetworkBody{
		{
			URL:         "http://myapp.com/api/data",
			Method:      "GET",
			ContentType: "application/json",
		},
	}

	result := mustCompareSnapshots(t, mgr, beforeBodies, afterBodies)

	if result.Verdict != "regressed" {
		t.Errorf("expected 'regressed', got %q", result.Verdict)
	}

	found := false
	for _, r := range result.Regressions {
		if r.Change == "transport_downgrade" {
			found = true
			if r.Severity != "high" {
				t.Errorf("expected severity 'high', got %q", r.Severity)
			}
			if r.Category != "transport" {
				t.Errorf("expected category 'transport', got %q", r.Category)
			}
		}
	}
	if !found {
		t.Error("expected transport_downgrade regression")
	}
}

func TestSecurityDiffUnchanged(t *testing.T) {
	t.Parallel()
	mgr := NewManager()

	bodies := []types.NetworkBody{
		{
			URL:         "https://myapp.com/",
			Method:      "GET",
			ContentType: "text/html",
			ResponseHeaders: map[string]string{
				"X-Frame-Options":        "DENY",
				"X-Content-Type-Options": "nosniff",
				"Set-Cookie":             "session=abc; HttpOnly; Secure",
			},
			HasAuthHeader: true,
		},
	}

	_, err := mgr.TakeSnapshot("snap1", bodies)
	if err != nil {
		t.Fatal(err)
	}
	_, err = mgr.TakeSnapshot("snap2", bodies)
	if err != nil {
		t.Fatal(err)
	}

	result, err := mgr.Compare("snap1", "snap2", nil)
	if err != nil {
		t.Fatal(err)
	}

	if result.Verdict != "unchanged" {
		t.Errorf("expected 'unchanged', got %q", result.Verdict)
	}
	if len(result.Regressions) != 0 {
		t.Errorf("expected 0 regressions, got %d", len(result.Regressions))
	}
	if len(result.Improvements) != 0 {
		t.Errorf("expected 0 improvements, got %d", len(result.Improvements))
	}
}

func TestSecurityDiffListSnapshots(t *testing.T) {
	t.Parallel()
	mgr := NewManager()
	bodies := []types.NetworkBody{{URL: "https://myapp.com/", ContentType: "text/html", ResponseHeaders: map[string]string{"X-Frame-Options": "DENY"}}}

	_, _ = mgr.TakeSnapshot("alpha", bodies)
	time.Sleep(time.Millisecond)
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

func TestSecurityDiffHandleDiffSecurity(t *testing.T) {
	t.Parallel()
	mgr := NewManager()
	bodies := []types.NetworkBody{
		{
			URL:         "https://myapp.com/",
			Method:      "GET",
			ContentType: "text/html",
			ResponseHeaders: map[string]string{
				"X-Frame-Options": "DENY",
			},
			HasAuthHeader: true,
		},
	}

	// Test snapshot action
	snapshotParams, _ := json.Marshal(map[string]string{
		"action": "snapshot",
		"name":   "test-snap",
	})
	result, err := mgr.HandleDiffSecurity(json.RawMessage(snapshotParams), bodies)
	if err != nil {
		t.Fatal(err)
	}
	snap, ok := result.(*Snapshot)
	if !ok {
		t.Fatalf("expected *Snapshot, got %T", result)
	}
	if snap.Name != "test-snap" {
		t.Errorf("expected name 'test-snap', got %q", snap.Name)
	}

	// Test list action
	listParams, _ := json.Marshal(map[string]string{
		"action": "list",
	})
	listResult, err := mgr.HandleDiffSecurity(json.RawMessage(listParams), bodies)
	if err != nil {
		t.Fatal(err)
	}
	entries, ok := listResult.([]SnapshotListEntry)
	if !ok {
		t.Fatalf("expected []SnapshotListEntry, got %T", listResult)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	// Take another snapshot for compare
	snap2Params, _ := json.Marshal(map[string]string{
		"action": "snapshot",
		"name":   "test-snap2",
	})
	_, err = mgr.HandleDiffSecurity(json.RawMessage(snap2Params), bodies)
	if err != nil {
		t.Fatal(err)
	}

	// Test compare action
	compareParams, _ := json.Marshal(map[string]string{
		"action":       "compare",
		"compare_from": "test-snap",
		"compare_to":   "test-snap2",
	})
	compareResult, err := mgr.HandleDiffSecurity(json.RawMessage(compareParams), bodies)
	if err != nil {
		t.Fatal(err)
	}
	diffResult, ok := compareResult.(*Result)
	if !ok {
		t.Fatalf("expected *Result, got %T", compareResult)
	}
	if diffResult.Verdict != "unchanged" {
		t.Errorf("expected 'unchanged' verdict, got %q", diffResult.Verdict)
	}
}

func TestSummary(t *testing.T) {
	t.Parallel()
	mgr := NewManager()

	// Before: multiple headers, auth, HTTPS
	beforeBodies := []types.NetworkBody{
		{
			URL:         "https://myapp.com/",
			Method:      "GET",
			ContentType: "text/html",
			ResponseHeaders: map[string]string{
				"X-Frame-Options":         "DENY",
				"X-Content-Type-Options":  "nosniff",
				"Content-Security-Policy": "default-src 'self'",
			},
			HasAuthHeader: true,
		},
	}

	// After: all headers removed, auth dropped
	afterBodies := []types.NetworkBody{
		{
			URL:             "https://myapp.com/",
			Method:          "GET",
			ContentType:     "text/html",
			ResponseHeaders: map[string]string{},
			HasAuthHeader:   false,
		},
	}

	result := mustCompareSnapshots(t, mgr, beforeBodies, afterBodies)

	if result.Summary.TotalRegressions == 0 {
		t.Error("expected non-zero total regressions")
	}
	if result.Summary.BySeverity == nil {
		t.Error("expected non-nil BySeverity map")
	}
	if result.Summary.ByCategory == nil {
		t.Error("expected non-nil ByCategory map")
	}
	if result.Summary.ByCategory["headers"] == 0 {
		t.Error("expected headers category in summary")
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

func TestBuildEphemeralSnapshotCookiesAndTransport(t *testing.T) {
	t.Parallel()
	mgr := NewManager()

	// Baseline with cookies (HttpOnly, Secure, SameSite) and auth
	mustTakeSnapshot(t, mgr, "before", []types.NetworkBody{
		{URL: "https://app.com/api", ContentType: "application/json", Status: 200,
			Method: "POST",
			ResponseHeaders: map[string]string{
				"Set-Cookie": "session=abc; HttpOnly; Secure; SameSite=Strict",
			},
			HasAuthHeader: true},
	})

	// Current: same origin, cookie flags stripped, auth dropped
	currentBodies := []types.NetworkBody{
		{URL: "https://app.com/api", ContentType: "application/json", Status: 200,
			Method: "POST",
			ResponseHeaders: map[string]string{
				"Set-Cookie": "session=abc",
			},
			HasAuthHeader: false},
	}

	result, err := mgr.Compare("before", "current", currentBodies)
	if err != nil {
		t.Fatalf("Compare failed: %v", err)
	}

	foundCookie := false
	foundAuth := false
	for _, r := range result.Regressions {
		if r.Category == "cookies" {
			foundCookie = true
		}
		if r.Category == "auth" {
			foundAuth = true
		}
	}
	if !foundCookie {
		t.Error("expected cookie regressions (HttpOnly/Secure/SameSite removed)")
	}
	if !foundAuth {
		t.Error("expected auth regressions (auth header dropped)")
	}
}

func TestHandleDiffSecurityInvalidAction(t *testing.T) {
	t.Parallel()
	mgr := NewManager()
	params := []byte(`{"action":"invalid"}`)
	_, err := mgr.HandleDiffSecurity(params, nil)
	if err == nil {
		t.Error("expected error for invalid action")
	}
}

func TestHandleDiffSecurityInvalidJSON(t *testing.T) {
	t.Parallel()
	mgr := NewManager()
	params := []byte(`{invalid}`)
	_, err := mgr.HandleDiffSecurity(params, nil)
	if err == nil {
		t.Error("expected error for invalid JSON params")
	}
}

func TestExtractSnapshotHelpers(t *testing.T) {
	t.Parallel()
	// extractSnapshotOrigin with invalid URL
	got := extractSnapshotOrigin("://invalid")
	if got != "://invalid" {
		t.Errorf("expected raw URL back for invalid, got %q", got)
	}
	// extractSnapshotOrigin with valid URL
	got = extractSnapshotOrigin("https://example.com:8080/path")
	if got != "https://example.com:8080" {
		t.Errorf("expected https://example.com:8080, got %q", got)
	}

	// extractScheme with invalid URL
	got = extractScheme("://bad")
	if got != "" {
		t.Errorf("expected empty for invalid URL, got %q", got)
	}

	// extractHostFromOrigin with invalid URL
	got = extractHostFromOrigin("://bad")
	if got != "://bad" {
		t.Errorf("expected raw input for invalid URL, got %q", got)
	}

	// headerRemovedRecommendation for all known headers
	headers := []string{"X-Frame-Options", "Strict-Transport-Security", "X-Content-Type-Options",
		"Content-Security-Policy", "Referrer-Policy", "Permissions-Policy", "Unknown-Header"}
	for _, h := range headers {
		rec := headerRemovedRecommendation(h)
		if rec == "" {
			t.Errorf("expected non-empty recommendation for %s", h)
		}
	}
}

func TestSecurityDiffExpiredSnapshot(t *testing.T) {
	t.Parallel()
	mgr := NewManager()
	mgr.ttl = 1 * time.Millisecond // Very short TTL

	bodies := []types.NetworkBody{
		{URL: "https://app.com/", ContentType: "text/html", Status: 200,
			ResponseHeaders: map[string]string{"X-Frame-Options": "DENY"}},
	}

	mustTakeSnapshot(t, mgr, "old", bodies)
	time.Sleep(5 * time.Millisecond) // Wait for expiry

	list := mgr.ListSnapshots()
	for _, s := range list {
		if s.Name == "old" && !s.Expired {
			t.Error("expected expired=true for old snapshot")
		}
	}
}
