// Purpose: Coverage-expansion tests for security edge cases and branch paths.
// Docs: docs/features/feature/security-hardening/index.md

// security_coverage_part2_test.go — Targeted coverage tests for uncovered security paths (part 2).
// Covers: checkPII integration, checkSecurityHeaders, shouldSkipHSTS,
// diffSingleCookieFlag, flagAbsentValue, looksLikeCreditCard,
// scanConsoleForCredentials, getEntryString, scanURLForGenericSecrets,
// checkTransport, checkCookies.
package security

import (
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
)

// ============================================
// checkPII — full integration
// ============================================

func TestCheckPII_RequestBodyToThirdParty(t *testing.T) {
	t.Parallel()
	s := NewSecurityScanner()
	bodies := []capture.NetworkBody{
		{
			URL:         "https://analytics.external.com/collect",
			Method:      "POST",
			RequestBody: `{"email":"user@example.com","ssn":"123-45-6789"}`,
		},
	}
	pageURLs := []string{"https://myapp.com"}

	findings := s.checkPII(bodies, pageURLs)
	if len(findings) < 2 {
		t.Errorf("expected at least 2 PII findings (email + SSN), got %d", len(findings))
	}
}

func TestCheckPII_ResponseBody(t *testing.T) {
	t.Parallel()
	s := NewSecurityScanner()
	bodies := []capture.NetworkBody{
		{
			URL:          "https://api.myapp.com/users/1",
			Method:       "GET",
			ResponseBody: `{"email":"admin@myapp.com"}`,
		},
	}

	findings := s.checkPII(bodies, []string{"https://myapp.com"})
	if len(findings) == 0 {
		t.Fatal("expected PII finding for email in response body")
	}
}

// ============================================
// checkSecurityHeaders
// ============================================

func TestCheckSecurityHeaders_MissingHeaders(t *testing.T) {
	t.Parallel()
	s := NewSecurityScanner()
	bodies := []capture.NetworkBody{
		{
			URL:             "https://example.com/page",
			ContentType:     "text/html",
			ResponseHeaders: map[string]string{},
		},
	}

	findings := s.checkSecurityHeaders(bodies)
	if len(findings) == 0 {
		t.Fatal("expected findings for missing security headers")
	}
}

func TestCheckSecurityHeaders_SkipsHSTSForLocalhost(t *testing.T) {
	t.Parallel()

	if !shouldSkipHSTS("Strict-Transport-Security", capture.NetworkBody{URL: "http://localhost:3000"}) {
		t.Error("should skip HSTS for localhost")
	}
	if !shouldSkipHSTS("Strict-Transport-Security", capture.NetworkBody{URL: "http://example.com"}) {
		t.Error("should skip HSTS for non-HTTPS")
	}
	if shouldSkipHSTS("Strict-Transport-Security", capture.NetworkBody{URL: "https://example.com"}) {
		t.Error("should not skip HSTS for HTTPS non-localhost")
	}
	if shouldSkipHSTS("X-Frame-Options", capture.NetworkBody{URL: "http://localhost:3000"}) {
		t.Error("should not skip non-HSTS header")
	}
}

// ============================================
// diffSingleCookieFlag — All branches
// ============================================

func TestDiffSingleCookieFlag_FlagRemoved(t *testing.T) {
	t.Parallel()
	spec := cookieFlagSpec{
		flagName:   "HttpOnly",
		fromActive: true,
		toActive:   false,
		lostMsg:    "lost HttpOnly",
	}
	change := diffSingleCookieFlag("https://example.com", "session_id", spec)
	if change == nil {
		t.Fatal("expected non-nil change for flag removal")
	}
	if change.Change != "flag_removed" {
		t.Errorf("change = %q, want flag_removed", change.Change)
	}
	if change.Before != "present" {
		t.Errorf("before = %q, want present", change.Before)
	}
	if change.After != "absent" {
		t.Errorf("after = %q, want absent", change.After)
	}
}

func TestDiffSingleCookieFlag_FlagAdded(t *testing.T) {
	t.Parallel()
	spec := cookieFlagSpec{
		flagName:   "Secure",
		fromActive: false,
		toActive:   true,
		gainedMsg:  "gained Secure",
	}
	change := diffSingleCookieFlag("https://example.com", "session_id", spec)
	if change == nil {
		t.Fatal("expected non-nil change for flag addition")
	}
	if change.Change != "flag_added" {
		t.Errorf("change = %q, want flag_added", change.Change)
	}
	if change.After != "present" {
		t.Errorf("after = %q, want present", change.After)
	}
}

func TestDiffSingleCookieFlag_SameSiteRemoved(t *testing.T) {
	t.Parallel()
	spec := cookieFlagSpec{
		flagName:   "SameSite",
		fromActive: true,
		toActive:   false,
		fromVal:    "Lax",
		lostMsg:    "lost SameSite",
	}
	change := diffSingleCookieFlag("https://example.com", "session_id", spec)
	if change == nil {
		t.Fatal("expected non-nil change")
	}
	if change.Before != "Lax" {
		t.Errorf("before = %q, want Lax (SameSite uses actual value)", change.Before)
	}
}

func TestDiffSingleCookieFlag_SameSiteAdded(t *testing.T) {
	t.Parallel()
	spec := cookieFlagSpec{
		flagName:   "SameSite",
		fromActive: false,
		toActive:   true,
		toVal:      "Strict",
		gainedMsg:  "gained SameSite",
	}
	change := diffSingleCookieFlag("https://example.com", "session_id", spec)
	if change == nil {
		t.Fatal("expected non-nil change")
	}
	if change.After != "Strict" {
		t.Errorf("after = %q, want Strict (SameSite uses actual value)", change.After)
	}
}

func TestDiffSingleCookieFlag_NoChange(t *testing.T) {
	t.Parallel()
	spec := cookieFlagSpec{
		flagName:   "HttpOnly",
		fromActive: true,
		toActive:   true,
	}
	change := diffSingleCookieFlag("https://example.com", "session_id", spec)
	if change != nil {
		t.Error("expected nil change when flag unchanged")
	}
}

// ============================================
// flagAbsentValue
// ============================================

func TestFlagAbsentValue(t *testing.T) {
	t.Parallel()
	if got := flagAbsentValue("SameSite", ""); got != "" {
		t.Errorf("flagAbsentValue(SameSite, '') = %q, want empty", got)
	}
	if got := flagAbsentValue("HttpOnly", ""); got != "absent" {
		t.Errorf("flagAbsentValue(HttpOnly, '') = %q, want absent", got)
	}
	if got := flagAbsentValue("Secure", "fallback"); got != "absent" {
		t.Errorf("flagAbsentValue(Secure, 'fallback') = %q, want absent", got)
	}
}

// ============================================
// looksLikeCreditCard — Luhn algorithm
// ============================================

func TestLooksLikeCreditCard_ValidNumbers(t *testing.T) {
	t.Parallel()
	valid := []string{
		"4111111111111111", // Visa
		"5500000000000004", // Mastercard
		"378282246310005",  // Amex
	}
	for _, num := range valid {
		if !looksLikeCreditCard(num) {
			t.Errorf("looksLikeCreditCard(%q) = false, want true", num)
		}
	}
}

func TestLooksLikeCreditCard_InvalidNumbers(t *testing.T) {
	t.Parallel()
	invalid := []string{
		"1234567890123456",     // Fails Luhn
		"1234",                 // Too short
		"12345678901234567890", // Too long
	}
	for _, num := range invalid {
		if looksLikeCreditCard(num) {
			t.Errorf("looksLikeCreditCard(%q) = true, want false", num)
		}
	}
}

func TestLooksLikeCreditCard_NonDigit(t *testing.T) {
	t.Parallel()
	if looksLikeCreditCard("411111111111111a") {
		t.Error("expected false for string with non-digit character")
	}
}

// ============================================
// scanConsoleForCredentials — edge cases
// ============================================

func TestScanConsoleForCredentials_EmptyMessage(t *testing.T) {
	t.Parallel()
	s := NewSecurityScanner()
	entry := LogEntry{"source": "console", "message": ""}
	findings := s.scanConsoleForCredentials(entry)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings for empty message, got %d", len(findings))
	}
}

func TestScanConsoleForCredentials_NilMessage(t *testing.T) {
	t.Parallel()
	s := NewSecurityScanner()
	entry := LogEntry{"source": "console"}
	findings := s.scanConsoleForCredentials(entry)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings for nil message, got %d", len(findings))
	}
}

func TestScanConsoleForCredentials_NonStringMessage(t *testing.T) {
	t.Parallel()
	s := NewSecurityScanner()
	entry := LogEntry{"source": "console", "message": 42}
	findings := s.scanConsoleForCredentials(entry)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings for non-string message, got %d", len(findings))
	}
}

func TestScanConsoleForCredentials_LongMessage(t *testing.T) {
	t.Parallel()
	s := NewSecurityScanner()
	longMsg := "Bearer " + strings.Repeat("A", 11000)
	entry := LogEntry{"source": "console", "message": longMsg}
	findings := s.scanConsoleForCredentials(entry)
	if len(findings) == 0 {
		t.Error("expected Bearer token finding even with long message")
	}
}

// ============================================
// getEntryString
// ============================================

func TestGetEntryString_AllBranches(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		entry LogEntry
		key   string
		want  string
	}{
		{"missing key", LogEntry{}, "message", ""},
		{"nil value", LogEntry{"message": nil}, "message", ""},
		{"non-string value", LogEntry{"message": 42}, "message", ""},
		{"valid string", LogEntry{"message": "hello"}, "message", "hello"},
	}
	for _, tc := range cases {
		got := getEntryString(tc.entry, tc.key)
		if got != tc.want {
			t.Errorf("getEntryString(%s) = %q, want %q", tc.name, got, tc.want)
		}
	}
}

// ============================================
// scanURLForGenericSecrets — dedup avoidance
// ============================================

func TestScanURLForGenericSecrets_SkipsWhenAPIKeyMatches(t *testing.T) {
	t.Parallel()
	s := NewSecurityScanner()
	url := "https://api.example.com/data?api_key=abcdefghijklmnop"
	findings := s.scanURLForGenericSecrets(url)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings (apiKey pattern takes precedence), got %d", len(findings))
	}
}

func TestScanURLForGenericSecrets_GenericSecretParam(t *testing.T) {
	t.Parallel()
	s := NewSecurityScanner()
	url := "https://api.example.com/data?mysecret=longsecretvaluethatisnotapikey"
	findings := s.scanURLForGenericSecrets(url)
	if len(findings) == 0 {
		t.Fatal("expected finding for generic secret parameter")
	}
}

// ============================================
// checkTransport — mixed content with JS
// ============================================

func TestCheckTransport_MixedContentJS(t *testing.T) {
	t.Parallel()
	s := NewSecurityScanner()
	bodies := []capture.NetworkBody{
		{
			URL:         "http://evil.example.com/inject.js",
			Method:      "GET",
			ContentType: "application/javascript",
		},
	}
	pageURLs := []string{"https://myapp.com"}

	findings := s.checkTransport(bodies, pageURLs)

	if len(findings) < 2 {
		t.Fatalf("expected at least 2 findings, got %d", len(findings))
	}

	hasCritical := false
	for _, f := range findings {
		if f.Severity == "critical" {
			hasCritical = true
		}
	}
	if !hasCritical {
		t.Error("expected critical severity for mixed content JavaScript")
	}
}

func TestCheckTransport_LocalhostSkipped(t *testing.T) {
	t.Parallel()
	s := NewSecurityScanner()
	bodies := []capture.NetworkBody{
		{URL: "http://localhost:3000/api", Method: "GET"},
	}

	findings := s.checkTransport(bodies, []string{"https://myapp.com"})
	if len(findings) != 0 {
		t.Errorf("expected 0 findings for localhost, got %d", len(findings))
	}
}

// ============================================
// checkCookies — missing flags
// ============================================

func TestCheckCookies_SessionCookieMissingFlags(t *testing.T) {
	t.Parallel()
	s := NewSecurityScanner()
	bodies := []capture.NetworkBody{
		{
			URL: "https://example.com/login",
			ResponseHeaders: map[string]string{
				"Set-Cookie": "session_id=abc123; Path=/",
			},
		},
	}

	findings := s.checkCookies(bodies)
	if len(findings) < 2 {
		t.Errorf("expected at least 2 findings for session cookie missing flags, got %d", len(findings))
	}
}

func TestCheckCookies_NilHeaders(t *testing.T) {
	t.Parallel()
	s := NewSecurityScanner()
	bodies := []capture.NetworkBody{
		{URL: "https://example.com", ResponseHeaders: nil},
	}

	findings := s.checkCookies(bodies)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings for nil headers, got %d", len(findings))
	}
}
