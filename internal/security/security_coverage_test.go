// Purpose: Coverage-expansion tests for security edge cases and branch paths.
// Docs: docs/features/feature/security-hardening/index.md

// security_coverage_test.go — Targeted coverage tests for uncovered security paths (part 1).
// Covers: formatDuration, redactSecret, networkFlagDescription, networkFlagRemediation,
// extractOrigin, scanForCreditCard, scanForSSN,
// scanForEmailPII, scanForPhonePII, thirdPartySeverity, checkAuthPatterns.
package security

import (
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

// ============================================
// formatDuration — All time range branches
// ============================================

func TestFormatDuration_SubSecond(t *testing.T) {
	t.Parallel()
	got := formatDuration(500 * time.Millisecond)
	if got != "0.5s" {
		t.Errorf("formatDuration(500ms) = %q, want 0.5s", got)
	}
}

func TestFormatDuration_Seconds(t *testing.T) {
	t.Parallel()
	got := formatDuration(30 * time.Second)
	if got != "30s" {
		t.Errorf("formatDuration(30s) = %q, want 30s", got)
	}
}

func TestFormatDuration_MinutesOnly(t *testing.T) {
	t.Parallel()
	got := formatDuration(5 * time.Minute)
	if got != "5m" {
		t.Errorf("formatDuration(5m) = %q, want 5m", got)
	}
}

func TestFormatDuration_MinutesAndSeconds(t *testing.T) {
	t.Parallel()
	got := formatDuration(5*time.Minute + 30*time.Second)
	if got != "5m30s" {
		t.Errorf("formatDuration(5m30s) = %q, want 5m30s", got)
	}
}

func TestFormatDuration_HoursOnly(t *testing.T) {
	t.Parallel()
	got := formatDuration(2 * time.Hour)
	if got != "2h" {
		t.Errorf("formatDuration(2h) = %q, want 2h", got)
	}
}

func TestFormatDuration_HoursAndMinutes(t *testing.T) {
	t.Parallel()
	got := formatDuration(2*time.Hour + 15*time.Minute)
	if got != "2h15m" {
		t.Errorf("formatDuration(2h15m) = %q, want 2h15m", got)
	}
}

func TestFormatDuration_ExactSecondBoundary(t *testing.T) {
	t.Parallel()
	got := formatDuration(1 * time.Second)
	if got != "1s" {
		t.Errorf("formatDuration(1s) = %q, want 1s", got)
	}
}

func TestFormatDuration_JustUnderMinute(t *testing.T) {
	t.Parallel()
	got := formatDuration(59 * time.Second)
	if got != "59s" {
		t.Errorf("formatDuration(59s) = %q, want 59s", got)
	}
}

// ============================================
// redactSecret — All length branches
// ============================================

func TestRedactSecret_VeryShort(t *testing.T) {
	t.Parallel()
	cases := []struct {
		input string
		want  string
	}{
		{"ab", "ab***"},      // len <= 3
		{"abc", "abc***"},    // len <= 3
		{"abcd", "abc***"},   // len <= 6, > 3
		{"abcdef", "abc***"}, // len <= 6
	}
	for _, tc := range cases {
		got := redactSecret(tc.input)
		if got != tc.want {
			t.Errorf("redactSecret(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestRedactSecret_Medium(t *testing.T) {
	t.Parallel()
	// len > 6 && <= 10
	got := redactSecret("abcdefgh") // 8 chars
	if got != "abcdef***" {
		t.Errorf("redactSecret(8 chars) = %q, want abcdef***", got)
	}
	got = redactSecret("1234567890") // 10 chars
	if got != "123456***" {
		t.Errorf("redactSecret(10 chars) = %q, want 123456***", got)
	}
}

func TestRedactSecret_Long(t *testing.T) {
	t.Parallel()
	// len > 10
	got := redactSecret("abcdefghijklmno") // 15 chars
	if got != "abcdef***mno" {
		t.Errorf("redactSecret(15 chars) = %q, want abcdef***mno", got)
	}
}

// ============================================
// networkFlagDescription / networkFlagRemediation — All branches
// ============================================

func TestNetworkFlagDescription_AllTypes(t *testing.T) {
	t.Parallel()
	types := []string{
		"suspicious_tld", "non_standard_port", "mixed_content",
		"ip_address_origin", "potential_typosquatting", "unknown_type",
	}
	for _, typ := range types {
		got := networkFlagDescription(typ)
		if got == "" {
			t.Errorf("networkFlagDescription(%q) returned empty string", typ)
		}
	}
}

func TestNetworkFlagRemediation_AllTypes(t *testing.T) {
	t.Parallel()
	types := []string{
		"suspicious_tld", "non_standard_port", "mixed_content",
		"ip_address_origin", "potential_typosquatting", "unknown_type",
	}
	for _, typ := range types {
		got := networkFlagRemediation(typ)
		if got == "" {
			t.Errorf("networkFlagRemediation(%q) returned empty string", typ)
		}
	}
}

// ============================================
// extractOrigin — Edge cases
// ============================================

func TestExtractOrigin_DataURL(t *testing.T) {
	t.Parallel()
	got := util.ExtractOrigin("data:text/html,<h1>Hello</h1>")
	if got != "" {
		t.Errorf("extractOrigin(data:...) = %q, want empty", got)
	}
}

func TestExtractOrigin_BlobURL(t *testing.T) {
	t.Parallel()
	got := util.ExtractOrigin("blob:https://example.com/uuid-here")
	if got != "https://example.com" {
		t.Errorf("extractOrigin(blob:...) = %q, want https://example.com", got)
	}
}

func TestExtractOrigin_NoScheme(t *testing.T) {
	t.Parallel()
	got := util.ExtractOrigin("example.com/path")
	if got != "" {
		t.Errorf("extractOrigin(no scheme) = %q, want empty", got)
	}
}

func TestExtractOrigin_NoHost(t *testing.T) {
	t.Parallel()
	got := util.ExtractOrigin("file:///path/to/file")
	if got != "" {
		t.Errorf("extractOrigin(file:///) = %q, want empty", got)
	}
}

// ============================================
// checkPII — scanForCreditCard with Luhn validation
// ============================================

func TestScanForCreditCard_ValidLuhn(t *testing.T) {
	t.Parallel()
	content := "Payment with card 4111111111111111 processed"
	finding := scanForCreditCard(content, "https://pay.example.com", "request body")
	if finding == nil {
		t.Fatal("expected finding for valid credit card number")
	}
	if finding.Check != "pii" {
		t.Errorf("check = %q, want pii", finding.Check)
	}
	if finding.Severity != "critical" {
		t.Errorf("severity = %q, want critical", finding.Severity)
	}
}

func TestScanForCreditCard_InvalidLuhn(t *testing.T) {
	t.Parallel()
	content := "Not a card: 1234567890123456"
	finding := scanForCreditCard(content, "https://example.com", "request body")
	if finding != nil {
		t.Error("expected nil finding for invalid Luhn number")
	}
}

func TestScanForCreditCard_TooShort(t *testing.T) {
	t.Parallel()
	content := "Short: 1234 5678 9012"
	finding := scanForCreditCard(content, "https://example.com", "request body")
	if finding != nil {
		t.Error("expected nil finding for too-short number")
	}
}

func TestScanForCreditCard_NoMatch(t *testing.T) {
	t.Parallel()
	content := "No credit card numbers here at all"
	finding := scanForCreditCard(content, "https://example.com", "request body")
	if finding == nil {
		return // correct: no match
	}
	t.Error("expected nil finding for content without CC pattern")
}

// ============================================
// scanForSSN — third party severity
// ============================================

func TestScanForSSN_ThirdParty(t *testing.T) {
	t.Parallel()
	content := "SSN is 123-45-6789 in this request"
	finding := scanForSSN(content, "https://third-party.com/api", "request body", true)
	if finding == nil {
		t.Fatal("expected SSN finding")
	}
	if finding.Severity != "critical" {
		t.Errorf("severity = %q, want critical for third-party SSN", finding.Severity)
	}
}

func TestScanForSSN_FirstParty(t *testing.T) {
	t.Parallel()
	content := "SSN is 123-45-6789"
	finding := scanForSSN(content, "https://example.com/api", "response body", false)
	if finding == nil {
		t.Fatal("expected SSN finding")
	}
	if finding.Severity != "high" {
		t.Errorf("severity = %q, want high for first-party SSN", finding.Severity)
	}
}

func TestScanForSSN_NoMatch(t *testing.T) {
	t.Parallel()
	finding := scanForSSN("no SSNs here", "https://example.com", "request body", false)
	if finding != nil {
		t.Error("expected nil finding for content without SSN")
	}
}

// ============================================
// scanForEmailPII
// ============================================

func TestScanForEmailPII_Match(t *testing.T) {
	t.Parallel()
	finding := scanForEmailPII("contact user@example.com for info", "https://api.com/data", "response body", false)
	if finding == nil {
		t.Fatal("expected email PII finding")
	}
	if finding.Severity != "info" {
		t.Errorf("severity = %q, want info for first-party email", finding.Severity)
	}
}

func TestScanForEmailPII_ThirdParty(t *testing.T) {
	t.Parallel()
	finding := scanForEmailPII("email: user@example.com", "https://third.com/api", "request body", true)
	if finding == nil {
		t.Fatal("expected email PII finding")
	}
	if finding.Severity != "warning" {
		t.Errorf("severity = %q, want warning for third-party email", finding.Severity)
	}
}

func TestScanForEmailPII_NoMatch(t *testing.T) {
	t.Parallel()
	finding := scanForEmailPII("no emails here", "https://example.com", "request body", false)
	if finding != nil {
		t.Error("expected nil finding")
	}
}

// ============================================
// scanForPhonePII
// ============================================

func TestScanForPhonePII_Match(t *testing.T) {
	t.Parallel()
	finding := scanForPhonePII("Call us at (555) 123-4567", "https://api.com", "response body", false)
	if finding == nil {
		t.Fatal("expected phone PII finding")
	}
}

func TestScanForPhonePII_ThirdParty(t *testing.T) {
	t.Parallel()
	finding := scanForPhonePII("Phone: 555-123-4567", "https://third.com", "request body", true)
	if finding == nil {
		t.Fatal("expected phone PII finding")
	}
	if finding.Severity != "warning" {
		t.Errorf("severity = %q, want warning for third-party phone", finding.Severity)
	}
}

func TestScanForPhonePII_ShortNumber(t *testing.T) {
	t.Parallel()
	finding := scanForPhonePII("Number: 123-456", "https://example.com", "response body", false)
	if finding != nil {
		t.Error("expected nil finding for short phone number")
	}
}

func TestScanForPhonePII_NoMatch(t *testing.T) {
	t.Parallel()
	finding := scanForPhonePII("no phones here", "https://example.com", "request body", false)
	if finding != nil {
		t.Error("expected nil finding")
	}
}

// ============================================
// thirdPartySeverity
// ============================================

func TestThirdPartySeverity(t *testing.T) {
	t.Parallel()
	if got := thirdPartySeverity(true); got != "warning" {
		t.Errorf("thirdPartySeverity(true) = %q, want warning", got)
	}
	if got := thirdPartySeverity(false); got != "info" {
		t.Errorf("thirdPartySeverity(false) = %q, want info", got)
	}
}

// ============================================
// checkAuthPatterns — responses with/without auth
// ============================================

func TestCheckAuthPatterns_PIIWithoutAuth(t *testing.T) {
	t.Parallel()
	s := NewSecurityScanner()
	bodies := []capture.NetworkBody{
		{
			URL:           "https://api.example.com/users",
			Method:        "GET",
			HasAuthHeader: false,
			ResponseBody:  `{"email":"user@example.com","phone":"555-123-4567"}`,
		},
	}

	findings := s.checkAuthPatterns(bodies)
	if len(findings) == 0 {
		t.Fatal("expected findings for PII without auth")
	}
	if findings[0].Check != "auth" {
		t.Errorf("check = %q, want auth", findings[0].Check)
	}
}

func TestCheckAuthPatterns_PIIWithAuth(t *testing.T) {
	t.Parallel()
	s := NewSecurityScanner()
	bodies := []capture.NetworkBody{
		{
			URL:           "https://api.example.com/users",
			Method:        "GET",
			HasAuthHeader: true,
			ResponseBody:  `{"email":"user@example.com"}`,
		},
	}

	findings := s.checkAuthPatterns(bodies)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings when auth header present, got %d", len(findings))
	}
}

func TestCheckAuthPatterns_NoPII(t *testing.T) {
	t.Parallel()
	s := NewSecurityScanner()
	bodies := []capture.NetworkBody{
		{
			URL:           "https://api.example.com/status",
			Method:        "GET",
			HasAuthHeader: false,
			ResponseBody:  `{"status":"ok"}`,
		},
	}

	findings := s.checkAuthPatterns(bodies)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings for response without PII, got %d", len(findings))
	}
}

func TestCheckAuthPatterns_EmptyResponse(t *testing.T) {
	t.Parallel()
	s := NewSecurityScanner()
	bodies := []capture.NetworkBody{
		{
			URL:           "https://api.example.com/ping",
			Method:        "GET",
			HasAuthHeader: false,
			ResponseBody:  "",
		},
	}

	findings := s.checkAuthPatterns(bodies)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings for empty response, got %d", len(findings))
	}
}
