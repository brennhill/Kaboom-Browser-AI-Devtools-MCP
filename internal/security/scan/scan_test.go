// scan_test.go — End-to-end tests for security audit analysis and finding generation.
// Purpose: Tests for security audit analysis and finding generation.
// Docs: docs/features/feature/security-hardening/index.md

// These were previously gated behind `//go:build integration` because "NetworkBody
// needs to be imported from capture package". It is imported below, so the tag is
// gone and the scanner's end-to-end tests run by default.
package scan

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
)

// NetworkBody keeps the test tables readable.
type NetworkBody = capture.NetworkBody

// Test fixtures: Stripe-like keys for security scanner tests.
// Constructed via concatenation to avoid GitHub push protection flagging.
var (
	testStripeKey1 = "sk_" + "live_4eC39HqLyjWDarjtT1zdp7dc"
	testStripeKey2 = "sk_" + "live_abcdefghijklmnopqrstuvwx"
	testStripeKey3 = "sk_" + "live_zyxwvutsrqponmlkjihgfedcb"
	testStripeKey4 = "sk_" + "live_abcdefghijklmnopqrstuvwxyz1234567890"
)

// ============================================
// Scanner Construction Tests
// ============================================

func TestNewScanner(t *testing.T) {
	t.Parallel()
	scanner := NewScanner()
	if scanner == nil {
		t.Fatal("NewScanner returned nil")
	}
}

// ============================================
// Empty Input Tests
// ============================================

func TestSecurityScan_EmptyInput(t *testing.T) {
	t.Parallel()
	scanner := NewScanner()
	input := Input{}
	result := scanner.Scan(input)

	if len(result.Findings) != 0 {
		t.Errorf("expected 0 findings for empty input, got %d", len(result.Findings))
	}
	if result.Summary.TotalFindings != 0 {
		t.Errorf("expected TotalFindings=0, got %d", result.Summary.TotalFindings)
	}
	if result.ScannedAt.IsZero() {
		t.Error("ScannedAt should not be zero")
	}
}

func TestSecurityScan_EmptyInput_NoError(t *testing.T) {
	t.Parallel()
	scanner := NewScanner()
	input := Input{
		NetworkBodies:  []NetworkBody{},
		ConsoleEntries: []LogEntry{},
		PageURLs:       []string{},
	}
	result := scanner.Scan(input)

	if len(result.Findings) != 0 {
		t.Errorf("expected 0 findings, got %d", len(result.Findings))
	}
}

// ============================================
// Credential Detection Tests
// ============================================

func TestSecurityScan_APIKeyInURL(t *testing.T) {
	t.Parallel()
	scanner := NewScanner()
	input := Input{
		NetworkBodies: []NetworkBody{
			{
				Method: "GET",
				URL:    "https://api.example.com/data?api_key=sk-proj-abcdefghij1234567890",
				Status: 200,
			},
		},
	}
	result := scanner.Scan(input)

	found := findFinding(result.Findings, "credentials", "critical")
	if found == nil {
		t.Fatal("expected critical credential finding for API key in URL")
	}
	if !strings.Contains(found.Title, "API key") && !strings.Contains(found.Title, "credential") && !strings.Contains(found.Title, "secret") {
		t.Errorf("finding title should mention API key/credential/secret, got: %s", found.Title)
	}
	if found.Location == "" {
		t.Error("finding should have a location")
	}
}

func TestSecurityScan_BearerTokenInResponseBody(t *testing.T) {
	t.Parallel()
	scanner := NewScanner()
	input := Input{
		NetworkBodies: []NetworkBody{
			{
				Method:       "POST",
				URL:          "https://api.example.com/login",
				Status:       200,
				ResponseBody: `{"access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIn0.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c", "token_type": "bearer"}`,
			},
		},
	}
	result := scanner.Scan(input)

	found := findFinding(result.Findings, "credentials", "")
	if found == nil {
		t.Fatal("expected credential finding for JWT in response body")
	}
}

func TestSecurityScan_AWSAccessKey(t *testing.T) {
	t.Parallel()
	scanner := NewScanner()
	input := Input{
		NetworkBodies: []NetworkBody{
			{
				Method:       "GET",
				URL:          "https://api.example.com/config",
				Status:       200,
				ResponseBody: `{"aws_key": "AKIAIOSFODNN7GASLNRQ"}`,
			},
		},
	}
	result := scanner.Scan(input)

	found := findFinding(result.Findings, "credentials", "critical")
	if found == nil {
		t.Fatal("expected critical finding for AWS access key")
	}
}

func TestSecurityScan_GitHubToken(t *testing.T) {
	t.Parallel()
	scanner := NewScanner()
	input := Input{
		NetworkBodies: []NetworkBody{
			{
				Method:      "POST",
				URL:         "https://api.example.com/deploy",
				Status:      200,
				RequestBody: `{"token": "ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghij"}`,
			},
		},
	}
	result := scanner.Scan(input)

	found := findFinding(result.Findings, "credentials", "critical")
	if found == nil {
		t.Fatal("expected critical finding for GitHub token")
	}
}

func TestSecurityScan_JWTInURL(t *testing.T) {
	t.Parallel()
	scanner := NewScanner()
	input := Input{
		NetworkBodies: []NetworkBody{
			{
				Method: "GET",
				URL:    "https://api.example.com/verify?token=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U",
				Status: 200,
			},
		},
	}
	result := scanner.Scan(input)

	found := findFinding(result.Findings, "credentials", "")
	if found == nil {
		t.Fatal("expected finding for JWT in URL")
	}
}

func TestSecurityScan_StripeSecretKey(t *testing.T) {
	t.Parallel()
	scanner := NewScanner()
	input := Input{
		NetworkBodies: []NetworkBody{
			{
				Method:       "GET",
				URL:          "https://api.example.com/config",
				Status:       200,
				ResponseBody: `{"stripe_key": "` + testStripeKey1 + `"}`,
			},
		},
	}
	result := scanner.Scan(input)

	found := findFinding(result.Findings, "credentials", "critical")
	if found == nil {
		t.Fatal("expected critical finding for Stripe secret key")
	}
}

func TestSecurityScan_PrivateKeyMaterial(t *testing.T) {
	t.Parallel()
	scanner := NewScanner()
	input := Input{
		NetworkBodies: []NetworkBody{
			{
				Method:       "GET",
				URL:          "https://api.example.com/key",
				Status:       200,
				ResponseBody: "-----BEGIN RSA PRIVATE KEY-----\nMIIE...base64...\n-----END RSA PRIVATE KEY-----",
			},
		},
	}
	result := scanner.Scan(input)

	found := findFinding(result.Findings, "credentials", "critical")
	if found == nil {
		t.Fatal("expected critical finding for private key material")
	}
}

func TestSecurityScan_CredentialInConsoleLog(t *testing.T) {
	t.Parallel()
	scanner := NewScanner()
	input := Input{
		ConsoleEntries: []LogEntry{
			{
				"level":   "log",
				"message": "Auth token: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U",
				"source":  "auth.js:45",
			},
		},
	}
	result := scanner.Scan(input)

	found := findFinding(result.Findings, "credentials", "critical")
	if found == nil {
		t.Fatal("expected critical finding for credential in console log")
	}
}

// ============================================
// False Positive Mitigation Tests
// ============================================

func TestSecurityScan_TestKeyNotFlagged(t *testing.T) {
	t.Parallel()
	scanner := NewScanner()
	input := Input{
		NetworkBodies: []NetworkBody{
			{
				Method: "GET",
				URL:    "https://api.example.com/data?api_key=test_key_for_development_only_1234567890",
				Status: 200,
			},
		},
	}
	result := scanner.Scan(input)

	// Test/dev keys should either not be flagged or be flagged at low severity
	for _, f := range result.Findings {
		if f.Check == "credentials" && f.Severity == "critical" {
			t.Errorf("test/dev keys should not produce critical findings, got: %s", f.Title)
		}
	}
}

// ============================================
// PII Leakage Tests
// ============================================

func TestSecurityScan_EmailInResponseToThirdParty(t *testing.T) {
	t.Parallel()
	scanner := NewScanner()
	input := Input{
		NetworkBodies: []NetworkBody{
			{
				Method:      "POST",
				URL:         "https://analytics.third-party.com/track",
				Status:      200,
				RequestBody: `{"user_email": "john.doe@example.com", "event": "page_view"}`,
			},
		},
		PageURLs: []string{"https://myapp.example.com"},
	}
	result := scanner.Scan(input)

	found := findFinding(result.Findings, "pii", "")
	if found == nil {
		t.Fatal("expected PII finding for email sent to third party")
	}
}

func TestSecurityScan_SSNInResponseBody(t *testing.T) {
	t.Parallel()
	scanner := NewScanner()
	input := Input{
		NetworkBodies: []NetworkBody{
			{
				Method:       "GET",
				URL:          "https://api.example.com/user/profile",
				Status:       200,
				ResponseBody: `{"name": "John Doe", "ssn": "123-45-6789"}`,
			},
		},
	}
	result := scanner.Scan(input)

	found := findFinding(result.Findings, "pii", "")
	if found == nil {
		t.Fatal("expected PII finding for SSN in response body")
	}
}

func TestSecurityScan_PhoneNumber(t *testing.T) {
	t.Parallel()
	scanner := NewScanner()
	input := Input{
		NetworkBodies: []NetworkBody{
			{
				Method:       "GET",
				URL:          "https://api.example.com/contacts",
				Status:       200,
				ResponseBody: `{"phone": "+1-555-123-4567", "name": "Jane"}`,
			},
		},
	}
	result := scanner.Scan(input)

	found := findFinding(result.Findings, "pii", "")
	if found == nil {
		t.Fatal("expected PII finding for phone number in response")
	}
}

func TestSecurityScan_CreditCardNumber(t *testing.T) {
	t.Parallel()
	scanner := NewScanner()
	input := Input{
		NetworkBodies: []NetworkBody{
			{
				Method:       "POST",
				URL:          "https://api.example.com/payment",
				Status:       200,
				ResponseBody: `{"card_number": "4532015112830366", "exp": "12/25"}`,
			},
		},
	}
	result := scanner.Scan(input)

	found := findFinding(result.Findings, "pii", "")
	if found == nil {
		t.Fatal("expected PII finding for credit card number")
	}
}

// ============================================
// Security Headers Tests
// ============================================

func TestSecurityScan_MissingHSTS(t *testing.T) {
	t.Parallel()
	scanner := NewScanner()
	input := Input{
		NetworkBodies: []NetworkBody{
			{
				Method:      "GET",
				URL:         "https://app.example.com/",
				Status:      200,
				ContentType: "text/html",
				// No response headers with HSTS
			},
		},
	}
	result := scanner.Scan(input)

	found := findFindingByTitle(result.Findings, "Strict-Transport-Security")
	if found == nil {
		t.Fatal("expected finding for missing HSTS header")
	}
	if found.Severity != "high" && found.Severity != "warning" {
		t.Errorf("missing HSTS should be high/warning severity, got: %s", found.Severity)
	}
}

func TestSecurityScan_MissingCSP(t *testing.T) {
	t.Parallel()
	scanner := NewScanner()
	input := Input{
		NetworkBodies: []NetworkBody{
			{
				Method:      "GET",
				URL:         "https://app.example.com/",
				Status:      200,
				ContentType: "text/html",
			},
		},
	}
	result := scanner.Scan(input)

	found := findFindingByTitle(result.Findings, "Content-Security-Policy")
	if found == nil {
		t.Fatal("expected finding for missing CSP header")
	}
	if found.Severity != "medium" && found.Severity != "warning" {
		t.Errorf("missing CSP should be medium/warning severity, got: %s", found.Severity)
	}
}

func TestSecurityScan_MissingXContentTypeOptions(t *testing.T) {
	t.Parallel()
	scanner := NewScanner()
	input := Input{
		NetworkBodies: []NetworkBody{
			{
				Method:      "GET",
				URL:         "https://app.example.com/page",
				Status:      200,
				ContentType: "text/html",
			},
		},
	}
	result := scanner.Scan(input)

	found := findFindingByTitle(result.Findings, "X-Content-Type-Options")
	if found == nil {
		t.Fatal("expected finding for missing X-Content-Type-Options header")
	}
}

func TestSecurityScan_MissingXFrameOptions(t *testing.T) {
	t.Parallel()
	scanner := NewScanner()
	input := Input{
		NetworkBodies: []NetworkBody{
			{
				Method:      "GET",
				URL:         "https://app.example.com/page",
				Status:      200,
				ContentType: "text/html",
			},
		},
	}
	result := scanner.Scan(input)

	found := findFindingByTitle(result.Findings, "X-Frame-Options")
	if found == nil {
		t.Fatal("expected finding for missing X-Frame-Options header")
	}
}

func TestSecurityScan_HeadersWithPresent(t *testing.T) {
	t.Parallel()
	scanner := NewScanner()
	input := Input{
		NetworkBodies: []NetworkBody{
			{
				Method:      "GET",
				URL:         "https://app.example.com/page",
				Status:      200,
				ContentType: "text/html",
				ResponseHeaders: map[string]string{
					"Strict-Transport-Security": "max-age=31536000; includeSubDomains",
					"X-Content-Type-Options":    "nosniff",
					"X-Frame-Options":           "DENY",
					"Content-Security-Policy":   "default-src 'self'",
					"Referrer-Policy":           "strict-origin",
					"Permissions-Policy":        "camera=(), microphone=()",
				},
			},
		},
	}
	result := scanner.Scan(input)

	// Should not have any header findings for these specific headers
	for _, f := range result.Findings {
		if f.Check == "headers" {
			t.Errorf("expected no header findings when all headers present, got: %s", f.Title)
		}
	}
}

func TestSecurityScan_LocalhostSkipsHSTS(t *testing.T) {
	t.Parallel()
	scanner := NewScanner()
	input := Input{
		NetworkBodies: []NetworkBody{
			{
				Method:      "GET",
				URL:         "http://localhost:3000/",
				Status:      200,
				ContentType: "text/html",
			},
		},
	}
	result := scanner.Scan(input)

	// HSTS check should skip localhost
	for _, f := range result.Findings {
		if f.Check == "headers" && strings.Contains(f.Title, "Strict-Transport-Security") {
			t.Error("HSTS check should skip localhost URLs")
		}
	}
}

// ============================================
// Cookie Security Tests
// ============================================

func TestSecurityScan_CookieMissingHttpOnly(t *testing.T) {
	t.Parallel()
	scanner := NewScanner()
	input := Input{
		NetworkBodies: []NetworkBody{
			{
				Method:      "POST",
				URL:         "https://app.example.com/login",
				Status:      200,
				ContentType: "application/json",
				ResponseHeaders: map[string]string{
					"Set-Cookie": "session_id=abc123; Path=/; Secure; SameSite=Lax",
				},
			},
		},
	}
	result := scanner.Scan(input)

	found := findFinding(result.Findings, "cookies", "")
	if found == nil {
		t.Fatal("expected finding for session cookie missing HttpOnly")
	}
	if !strings.Contains(strings.ToLower(found.Title), "httponly") {
		t.Errorf("finding title should mention HttpOnly, got: %s", found.Title)
	}
}

func TestSecurityScan_CookieMissingSecure(t *testing.T) {
	t.Parallel()
	scanner := NewScanner()
	input := Input{
		NetworkBodies: []NetworkBody{
			{
				Method:      "POST",
				URL:         "https://app.example.com/login",
				Status:      200,
				ContentType: "application/json",
				ResponseHeaders: map[string]string{
					"Set-Cookie": "auth_token=xyz789; Path=/; HttpOnly; SameSite=Strict",
				},
			},
		},
	}
	result := scanner.Scan(input)

	found := findFindingByTitle(result.Findings, "Secure")
	if found == nil {
		// Also check lowercase
		found = findFinding(result.Findings, "cookies", "")
		if found == nil {
			t.Fatal("expected finding for cookie missing Secure flag on HTTPS")
		}
	}
}

func TestSecurityScan_CookieMissingSameSite(t *testing.T) {
	t.Parallel()
	scanner := NewScanner()
	input := Input{
		NetworkBodies: []NetworkBody{
			{
				Method:      "POST",
				URL:         "https://app.example.com/login",
				Status:      200,
				ContentType: "application/json",
				ResponseHeaders: map[string]string{
					"Set-Cookie": "session=abc123; Path=/; HttpOnly; Secure",
				},
			},
		},
	}
	result := scanner.Scan(input)

	found := findFinding(result.Findings, "cookies", "")
	if found == nil {
		t.Fatal("expected finding for cookie missing SameSite")
	}
}

func TestSecurityScan_SecureCookieNoFindings(t *testing.T) {
	t.Parallel()
	scanner := NewScanner()
	input := Input{
		NetworkBodies: []NetworkBody{
			{
				Method:      "POST",
				URL:         "https://app.example.com/login",
				Status:      200,
				ContentType: "application/json",
				ResponseHeaders: map[string]string{
					"Set-Cookie": "session_id=abc123; Path=/; HttpOnly; Secure; SameSite=Lax",
				},
			},
		},
	}
	result := scanner.Scan(input)

	for _, f := range result.Findings {
		if f.Check == "cookies" {
			t.Errorf("expected no cookie findings for properly secured cookie, got: %s", f.Title)
		}
	}
}

// ============================================
// Insecure Transport Tests
// ============================================

func TestSecurityScan_HTTPLoginEndpoint(t *testing.T) {
	t.Parallel()
	scanner := NewScanner()
	input := Input{
		NetworkBodies: []NetworkBody{
			{
				Method: "POST",
				URL:    "http://api.example.com/auth/login",
				Status: 200,
			},
		},
	}
	result := scanner.Scan(input)

	found := findFinding(result.Findings, "transport", "")
	if found == nil {
		t.Fatal("expected transport finding for HTTP login endpoint")
	}
}

func TestSecurityScan_HTTPLocalhostNotFlagged(t *testing.T) {
	t.Parallel()
	scanner := NewScanner()
	input := Input{
		NetworkBodies: []NetworkBody{
			{
				Method: "POST",
				URL:    "http://localhost:3000/api/login",
				Status: 200,
			},
		},
	}
	result := scanner.Scan(input)

	for _, f := range result.Findings {
		if f.Check == "transport" {
			t.Errorf("localhost HTTP should not be flagged, got: %s", f.Title)
		}
	}
}

func TestSecurityScan_HTTP127NotFlagged(t *testing.T) {
	t.Parallel()
	scanner := NewScanner()
	input := Input{
		NetworkBodies: []NetworkBody{
			{
				Method: "GET",
				URL:    "http://127.0.0.1:8080/api/data",
				Status: 200,
			},
		},
	}
	result := scanner.Scan(input)

	for _, f := range result.Findings {
		if f.Check == "transport" {
			t.Errorf("127.0.0.1 HTTP should not be flagged, got: %s", f.Title)
		}
	}
}

func TestSecurityScan_MixedContent(t *testing.T) {
	t.Parallel()
	scanner := NewScanner()
	input := Input{
		NetworkBodies: []NetworkBody{
			{
				Method:      "GET",
				URL:         "http://cdn.example.com/script.js",
				Status:      200,
				ContentType: "application/javascript",
			},
		},
		PageURLs: []string{"https://app.example.com/dashboard"},
	}
	result := scanner.Scan(input)

	found := findFinding(result.Findings, "transport", "")
	if found == nil {
		t.Fatal("expected transport finding for mixed content")
	}
}

// ============================================
// Evidence Redaction Tests
// ============================================

func TestSecurityScan_EvidenceRedacted(t *testing.T) {
	t.Parallel()
	scanner := NewScanner()
	secretValue := "sk-proj-abcdefghijklmnopqrstuvwxyz1234567890"
	input := Input{
		NetworkBodies: []NetworkBody{
			{
				Method: "GET",
				URL:    "https://api.example.com/data?api_key=" + secretValue,
				Status: 200,
			},
		},
	}
	result := scanner.Scan(input)

	if len(result.Findings) == 0 {
		t.Fatal("expected at least one finding")
	}

	for _, f := range result.Findings {
		if f.Check == "credentials" && f.Evidence != "" {
			// Evidence should NOT contain the full secret
			if strings.Contains(f.Evidence, secretValue) {
				t.Error("evidence should be redacted, but contains the full secret")
			}
			// Evidence should show some prefix
			if !strings.Contains(f.Evidence, "sk-p") && !strings.Contains(f.Evidence, "***") && !strings.Contains(f.Evidence, "...") {
				t.Errorf("evidence should show partial value with masking, got: %s", f.Evidence)
			}
		}
	}
}

// ============================================
// Summary Tests
// ============================================

func TestSecurityScan_SummaryAccuracy(t *testing.T) {
	t.Parallel()
	scanner := NewScanner()
	input := Input{
		NetworkBodies: []NetworkBody{
			{
				Method: "GET",
				URL:    "https://api.example.com/data?api_key=" + testStripeKey2,
				Status: 200,
			},
			{
				Method: "POST",
				URL:    "http://api.example.com/login",
				Status: 200,
			},
		},
	}
	result := scanner.Scan(input)

	if result.Summary.TotalFindings != len(result.Findings) {
		t.Errorf("summary total (%d) should match findings length (%d)",
			result.Summary.TotalFindings, len(result.Findings))
	}

	// Check that BySeverity sums match total
	severitySum := 0
	for _, count := range result.Summary.BySeverity {
		severitySum += count
	}
	if severitySum != result.Summary.TotalFindings {
		t.Errorf("severity sum (%d) should match total (%d)", severitySum, result.Summary.TotalFindings)
	}

	// Check that ByCheck sums match total
	checkSum := 0
	for _, count := range result.Summary.ByCheck {
		checkSum += count
	}
	if checkSum != result.Summary.TotalFindings {
		t.Errorf("check sum (%d) should match total (%d)", checkSum, result.Summary.TotalFindings)
	}

	if result.Summary.URLsScanned < 1 {
		t.Error("URLsScanned should be at least 1")
	}
}

// ============================================
// URL Filter Tests
// ============================================

func TestSecurityScan_URLFilter(t *testing.T) {
	t.Parallel()
	scanner := NewScanner()
	input := Input{
		NetworkBodies: []NetworkBody{
			{
				Method: "GET",
				URL:    "https://api.example.com/data?api_key=" + testStripeKey2,
				Status: 200,
			},
			{
				Method: "GET",
				URL:    "https://other.example.com/data?api_key=" + testStripeKey3,
				Status: 200,
			},
		},
		URLFilter: "api.example.com",
	}
	result := scanner.Scan(input)

	// All findings should be for the filtered URL
	for _, f := range result.Findings {
		if f.Check == "credentials" && !strings.Contains(f.Location, "api.example.com") {
			t.Errorf("with URL filter, findings should only be for filtered URL, got location: %s", f.Location)
		}
	}
}

// ============================================
// Check Selection Tests
// ============================================

func TestSecurityScan_CheckSelection(t *testing.T) {
	t.Parallel()
	scanner := NewScanner()
	input := Input{
		NetworkBodies: []NetworkBody{
			{
				Method: "GET",
				URL:    "https://api.example.com/data?api_key=" + testStripeKey2,
				Status: 200,
			},
			{
				Method: "POST",
				URL:    "http://api.example.com/login",
				Status: 200,
			},
		},
		Checks: []string{"transport"}, // Only run transport checks
	}
	result := scanner.Scan(input)

	for _, f := range result.Findings {
		if f.Check != "transport" {
			t.Errorf("with checks=[transport], should only get transport findings, got: %s", f.Check)
		}
	}
}

// ============================================
// Severity Filter Tests
// ============================================

func TestSecurityScan_SeverityFilter(t *testing.T) {
	t.Parallel()
	scanner := NewScanner()
	input := Input{
		NetworkBodies: []NetworkBody{
			{
				Method: "GET",
				URL:    "https://api.example.com/data?api_key=" + testStripeKey2,
				Status: 200,
			},
			{
				Method:      "GET",
				URL:         "https://app.example.com/",
				Status:      200,
				ContentType: "text/html",
			},
		},
		SeverityMin: "critical",
	}
	result := scanner.Scan(input)

	for _, f := range result.Findings {
		if f.Severity != "critical" {
			t.Errorf("with severity_min=critical, should only get critical findings, got: %s (%s)", f.Severity, f.Title)
		}
	}
}

// ============================================
// Auth Pattern Tests
// ============================================

func TestSecurityScan_MissingAuth(t *testing.T) {
	t.Parallel()
	scanner := NewScanner()
	input := Input{
		NetworkBodies: []NetworkBody{
			{
				Method:        "GET",
				URL:           "https://api.example.com/users/profile",
				Status:        200,
				ResponseBody:  `{"email": "user@example.com", "name": "John Doe", "phone": "+15551234567"}`,
				HasAuthHeader: false,
			},
		},
	}
	result := scanner.Scan(input)

	found := findFinding(result.Findings, "auth", "")
	if found == nil {
		t.Fatal("expected auth finding for endpoint returning PII without auth")
	}
}

func TestSecurityScan_WithAuthNoFinding(t *testing.T) {
	t.Parallel()
	scanner := NewScanner()
	input := Input{
		NetworkBodies: []NetworkBody{
			{
				Method:        "GET",
				URL:           "https://api.example.com/users/profile",
				Status:        200,
				ResponseBody:  `{"email": "user@example.com", "name": "John Doe"}`,
				HasAuthHeader: true,
			},
		},
	}
	result := scanner.Scan(input)

	// Should not flag endpoints that have auth
	for _, f := range result.Findings {
		if f.Check == "auth" {
			t.Errorf("should not flag authenticated endpoints, got: %s", f.Title)
		}
	}
}

// ============================================
// MCP Tool Handler Tests
// ============================================

func TestHandleSecurityAudit_EmptyParams(t *testing.T) {
	t.Parallel()
	scanner := NewScanner()
	params := json.RawMessage(`{}`)
	result, err := scanner.HandleSecurityAudit(params, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("HandleSecurityAudit with empty params should not error, got: %v", err)
	}
	if result == nil {
		t.Fatal("HandleSecurityAudit should return a result")
	}
}

func TestHandleSecurityAudit_WithChecksParam(t *testing.T) {
	t.Parallel()
	scanner := NewScanner()
	params := json.RawMessage(`{"checks": ["credentials", "transport"]}`)
	bodies := []NetworkBody{
		{
			Method: "GET",
			URL:    "http://api.example.com/data?api_key=" + testStripeKey2,
			Status: 200,
		},
	}
	result, err := scanner.HandleSecurityAudit(params, bodies, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have findings from both credential and transport checks
	resultMap, ok := result.(Result)
	if !ok {
		t.Fatal("result should be Result")
	}
	if len(resultMap.Findings) == 0 {
		t.Error("expected findings")
	}
}

func TestHandleSecurityAudit_URLFilter(t *testing.T) {
	t.Parallel()
	scanner := NewScanner()
	params := json.RawMessage(`{"url": "api.example.com"}`)
	bodies := []NetworkBody{
		{
			Method: "GET",
			URL:    "https://api.example.com/data?api_key=" + testStripeKey2,
			Status: 200,
		},
		{
			Method: "GET",
			URL:    "https://other.com/data?api_key=" + testStripeKey3,
			Status: 200,
		},
	}
	result, err := scanner.HandleSecurityAudit(params, bodies, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resultMap := result.(Result)
	for _, f := range resultMap.Findings {
		if f.Check == "credentials" && !strings.Contains(f.Location, "api.example.com") {
			t.Errorf("URL filter should limit findings, got location: %s", f.Location)
		}
	}
}

// ============================================
// Concurrent Safety Tests
// ============================================

func TestScanner_ConcurrentSafe(t *testing.T) {
	t.Parallel()
	scanner := NewScanner()
	input := Input{
		NetworkBodies: []NetworkBody{
			{
				Method: "GET",
				URL:    "https://api.example.com/data?api_key=" + testStripeKey2,
				Status: 200,
			},
		},
	}

	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			result := scanner.Scan(input)
			if len(result.Findings) == 0 {
				t.Error("expected findings in concurrent scan")
			}
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

// ============================================
// JSON Serialization Tests
// ============================================

func TestResult_JSONSerialization(t *testing.T) {
	t.Parallel()
	scanner := NewScanner()
	input := Input{
		NetworkBodies: []NetworkBody{
			{
				Method: "GET",
				URL:    "https://api.example.com/data?api_key=" + testStripeKey2,
				Status: 200,
			},
		},
	}
	result := scanner.Scan(input)

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("failed to marshal result: %v", err)
	}

	var decoded Result
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	if decoded.Summary.TotalFindings != result.Summary.TotalFindings {
		t.Errorf("round-trip mismatch: total findings %d vs %d",
			decoded.Summary.TotalFindings, result.Summary.TotalFindings)
	}
}

// ============================================
// Redaction Helper Tests
// ============================================

func TestRedactSecret(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		input  string
		expect string // Check that it contains prefix and masking
	}{
		{
			name:  "short secret",
			input: "abcdefgh",
		},
		{
			name:  "long secret",
			input: testStripeKey4,
		},
		{
			name:  "JWT",
			input: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := redactSecret(tt.input)
			// Should not equal the original
			if result == tt.input {
				t.Error("redacted value should differ from original")
			}
			// Should contain some visible prefix
			if len(tt.input) > 6 && !strings.HasPrefix(result, tt.input[:6]) {
				t.Errorf("redacted value should start with first 6 chars, got: %s", result)
			}
			// Should contain masking indicator
			if !strings.Contains(result, "***") && !strings.Contains(result, "...") {
				t.Errorf("redacted value should contain masking, got: %s", result)
			}
		})
	}
}

// ============================================
// Edge Cases
// ============================================

func TestSecurityScan_VeryLongURL(t *testing.T) {
	t.Parallel()
	scanner := NewScanner()
	longURL := "https://api.example.com/data?" + strings.Repeat("x", 10000)
	input := Input{
		NetworkBodies: []NetworkBody{
			{
				Method: "GET",
				URL:    longURL,
				Status: 200,
			},
		},
	}
	// Should not panic
	result := scanner.Scan(input)
	_ = result
}

func TestSecurityScan_InvalidURLFormat(t *testing.T) {
	t.Parallel()
	scanner := NewScanner()
	input := Input{
		NetworkBodies: []NetworkBody{
			{
				Method: "GET",
				URL:    "not-a-valid-url",
				Status: 200,
			},
		},
	}
	// Should not panic
	result := scanner.Scan(input)
	_ = result
}

func TestSecurityScan_NilConsoleEntryFields(t *testing.T) {
	t.Parallel()
	scanner := NewScanner()
	input := Input{
		ConsoleEntries: []LogEntry{
			{}, // Empty entry
			{"level": nil, "message": nil},
		},
	}
	// Should not panic
	result := scanner.Scan(input)
	_ = result
}

// ============================================
// Test Helpers
// ============================================

func findFinding(findings []Finding, check, severity string) *Finding {
	for i, f := range findings {
		if f.Check == check {
			if severity == "" || f.Severity == severity {
				return &findings[i]
			}
		}
	}
	return nil
}

func findFindingByTitle(findings []Finding, titleSubstr string) *Finding {
	for i, f := range findings {
		if strings.Contains(f.Title, titleSubstr) {
			return &findings[i]
		}
	}
	return nil
}

// FuzzSecurityPatterns exercises all security scanner regex patterns with
// arbitrary inputs to ensure no panics, hangs, or invalid output structures.
func FuzzSecurityPatterns(f *testing.F) {
	// Seed corpus: strings that exercise each regex pattern category
	seeds := []string{
		// AWS key pattern
		"AKIA1234567890ABCDEF",
		// GitHub token
		"ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghij",
		// Stripe key (constructed to avoid push protection)
		"sk_" + "test_" + "abcdefghijklmnopqrstuvwx",
		// JWT
		"eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxIn0.signature_here",
		// Private key header
		"-----BEGIN RSA PRIVATE KEY-----",
		// API key in URL
		"https://api.example.com/v1?api_key=supersecretvalue123",
		// Bearer token
		"Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9",
		// API key in JSON body
		`{"apiKey": "my_secret_api_key_value_here"}`,
		// Generic secret in URL
		"https://example.com/callback?secret=longvaluehere123",
		// Email PII
		"user@example.com",
		// Phone PII
		"+1 (555) 123-4567",
		// SSN PII
		"123-45-6789",
		// Credit card PII
		"4111 1111 1111 1111",
		// PII field name in JSON
		`{"email": "test@test.com", "ssn": "000-00-0000"}`,
		// Empty and edge cases
		"",
		"\x00\xff\xfe",
		strings.Repeat("a", 100000),
		strings.Repeat("?&key=", 1000),
		`{"` + strings.Repeat(`"`, 5000) + `}`,
	}

	for _, s := range seeds {
		f.Add(s, s)
	}

	scanner := NewScanner()

	f.Fuzz(func(t *testing.T, urlData, bodyData string) {
		// Exercise credential + PII patterns via network bodies
		input := Input{
			NetworkBodies: []NetworkBody{
				{
					URL:             urlData,
					RequestBody:     bodyData,
					ResponseBody:    bodyData,
					ContentType:     "application/json",
					Status:          200,
					ResponseHeaders: map[string]string{"Set-Cookie": bodyData},
				},
			},
			ConsoleEntries: []LogEntry{
				{"level": "error", "msg": bodyData},
			},
			PageURLs: []string{urlData},
		}

		// Must not panic
		result := scanner.Scan(input)

		// Result must be structurally valid
		if result.Summary.TotalFindings < 0 {
			t.Error("Negative finding count")
		}
		if result.Summary.TotalFindings != len(result.Findings) {
			t.Errorf("Summary count %d != findings length %d",
				result.Summary.TotalFindings, len(result.Findings))
		}

		// All findings must have required fields
		for _, finding := range result.Findings {
			if finding.Check == "" {
				t.Error("Finding with empty check")
			}
			if finding.Severity == "" {
				t.Error("Finding with empty severity")
			}
		}

		// Must serialize without error
		if _, err := json.Marshal(result); err != nil {
			t.Errorf("Result not serializable: %v", err)
		}
	})
}
