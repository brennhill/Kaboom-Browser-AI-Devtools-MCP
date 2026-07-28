// compare_test.go — Tests security posture comparison and extraction.
// Docs: docs/features/feature/security-hardening/index.md

package diff

import (
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

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
