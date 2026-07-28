// unit_test.go — Unit tests for the individual security checks and credential scanner.
// Purpose: Unit tests for security logic.
// Docs: docs/features/feature/security-hardening/index.md

package scan

import (
	"encoding/json"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"strings"
	"testing"
)

func TestCheckCookies_FlagsMissingSessionCookieSecurityAttributes(t *testing.T) {
	t.Parallel()
	scanner := NewScanner()

	bodies := []types.NetworkBody{
		{
			URL: "https://app.example.com/dashboard",
			ResponseHeaders: map[string]string{
				"Set-Cookie": "session_token=abc123",
			},
		},
	}

	findings := scanner.checkCookies(bodies)

	var hasHTTPOnly, hasSecure, hasSameSite bool
	for _, f := range findings {
		if strings.Contains(f.Title, "HttpOnly") {
			hasHTTPOnly = true
		}
		if strings.Contains(f.Title, "Secure flag") {
			hasSecure = true
		}
		if strings.Contains(f.Title, "SameSite") {
			hasSameSite = true
		}
	}

	if !hasHTTPOnly || !hasSecure || !hasSameSite {
		t.Fatalf("expected HttpOnly/Secure/SameSite findings, got: %+v", findings)
	}
}

func TestCheckSecurityHeaders_SkipsHSTSOnLocalhost(t *testing.T) {
	t.Parallel()
	scanner := NewScanner()

	bodies := []types.NetworkBody{
		{
			URL:         "https://localhost:3000/",
			ContentType: "text/html; charset=utf-8",
			ResponseHeaders: map[string]string{
				"Content-Type": "text/html; charset=utf-8",
			},
		},
	}

	findings := scanner.checkSecurityHeaders(bodies)
	for _, f := range findings {
		if strings.Contains(f.Title, "Strict-Transport-Security") {
			t.Fatalf("localhost response should not require HSTS, got finding: %+v", f)
		}
	}
}

func TestCheckTransport_HTTPJSOnHTTPSPageIncludesCriticalMixedContent(t *testing.T) {
	t.Parallel()
	scanner := NewScanner()

	bodies := []types.NetworkBody{
		{
			Method:      "GET",
			URL:         "http://cdn.example.com/app.js",
			ContentType: "application/javascript",
		},
	}
	findings := scanner.checkTransport(bodies, []string{"https://app.example.com"})

	var hasCriticalMixedContent bool
	for _, f := range findings {
		if strings.Contains(f.Title, "Mixed content") && f.Severity == "critical" {
			hasCriticalMixedContent = true
		}
	}

	if !hasCriticalMixedContent {
		t.Fatalf("expected critical mixed-content finding, got: %+v", findings)
	}
}

func TestScanForPII_ThirdPartyEscalatesSeverity(t *testing.T) {
	t.Parallel()
	scanner := NewScanner()

	content := `{"email":"person@example.com","ssn":"123-45-6789","phone":"+1-555-123-4567"}`
	findings := scanner.scanForPII(content, "https://analytics.example.net/collect", "request body", true)

	var hasCriticalSSN, hasEmailWarning, hasPhoneWarning bool
	for _, f := range findings {
		if strings.Contains(f.Title, "SSN pattern") && f.Severity == "critical" {
			hasCriticalSSN = true
		}
		if strings.Contains(f.Title, "Email address") && f.Severity == "warning" {
			hasEmailWarning = true
		}
		if strings.Contains(f.Title, "Phone number") && f.Severity == "warning" {
			hasPhoneWarning = true
		}
		if strings.Contains(f.Evidence, "123-45-6789") {
			t.Fatalf("PII evidence should be redacted, got %q", f.Evidence)
		}
	}

	if !hasCriticalSSN || !hasEmailWarning || !hasPhoneWarning {
		t.Fatalf("expected SSN/email/phone findings with third-party severities, got: %+v", findings)
	}
}

func TestHelperFunctions_FilterAndParsing(t *testing.T) {
	t.Parallel()

	filteredBodies := filterBodiesByURL([]types.NetworkBody{
		{URL: "https://api.example.com/users"},
		{URL: "https://cdn.example.com/app.js"},
	}, "api.example.com")
	if len(filteredBodies) != 1 || !strings.Contains(filteredBodies[0].URL, "api.example.com") {
		t.Fatalf("filterBodiesByURL result = %+v", filteredBodies)
	}

	findings := []Finding{
		{Severity: "info", Check: "headers"},
		{Severity: "warning", Check: "cookies"},
		{Severity: "critical", Check: "credentials"},
	}
	criticalOnly := filterBySeverity(findings, "high")
	if len(criticalOnly) != 1 || criticalOnly[0].Severity != "critical" {
		t.Fatalf("filterBySeverity(high) = %+v", criticalOnly)
	}
	if len(filterBySeverity(findings, "not-a-severity")) != len(findings) {
		t.Fatal("unknown severity should return original findings")
	}

	fields := detectPIIFields(`{"email":"a@b.com","phone":"+1 555 123 4567","ssn":"123-45-6789"}`)
	if len(fields) != 3 {
		t.Fatalf("detectPIIFields len = %d, want 3 (email, phone, ssn)", len(fields))
	}

	if !looksLikeCreditCard("4111111111111111") {
		t.Fatal("looksLikeCreditCard should accept valid Luhn test number")
	}
	if looksLikeCreditCard("4111111111111112") {
		t.Fatal("looksLikeCreditCard should reject invalid Luhn number")
	}
	if looksLikeCreditCard("4111x11111111111") {
		t.Fatal("looksLikeCreditCard should reject non-digit input")
	}

	entry := LogEntry{"message": "hello"}
	if getEntryString(entry, "message") != "hello" {
		t.Fatal("getEntryString should return string values")
	}
	if getEntryString(LogEntry{"message": 123}, "message") != "" {
		t.Fatal("getEntryString should return empty for non-string values")
	}
}

func TestCredentialScanner_URLPatterns(t *testing.T) {
	t.Parallel()
	scanner := NewScanner()

	apiFindings := scanner.scanURLForCredentials(types.NetworkBody{
		Method: "GET",
		URL:    "https://api.example.com/data?api_key=live_secret_1234567890abcdef",
	})
	if len(apiFindings) == 0 || apiFindings[0].Check != "credentials" || apiFindings[0].Severity != "critical" {
		t.Fatalf("expected critical API-key finding, got %+v", apiFindings)
	}

	genericFindings := scanner.scanURLForCredentials(types.NetworkBody{
		Method: "GET",
		URL:    "https://api.example.com/data?password=superSecretValue123456",
	})
	if len(genericFindings) == 0 || genericFindings[0].Severity != "critical" {
		t.Fatalf("expected credential URL finding for password param, got %+v", genericFindings)
	}

	jwtFindings := scanner.scanURLForCredentials(types.NetworkBody{
		Method: "GET",
		URL:    "https://api.example.com/verify?token=eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.signature1234567890",
	})
	if len(jwtFindings) == 0 {
		t.Fatalf("expected JWT URL finding, got %+v", jwtFindings)
	}

	awsFindings := scanner.scanURLForCredentials(types.NetworkBody{
		Method: "GET",
		URL:    "https://api.example.com/config?access_key=AKIA1234567890ABCDEF",
	})
	if len(awsFindings) == 0 {
		t.Fatalf("expected AWS key URL finding, got %+v", awsFindings)
	}
}

func TestCredentialScanner_BodyPatternsAndTestKeyHandling(t *testing.T) {
	t.Parallel()
	scanner := NewScanner()

	body := `{
		"aws":"AKIA1234567890ABCDEF",
		"github":"ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdef123456",
		"stripe":"stripe_example_placeholder",
		"private":"-----BEGIN RSA PRIVATE KEY-----",
		"jwt":"eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.signature1234567890",
		"apiKey":"prod_secret_key_987654321"
	}`
	findings := scanner.scanBodyForCredentials(body, "https://api.example.com/login", "response body")
	if len(findings) < 5 {
		t.Fatalf("expected multiple credential findings, got %+v", findings)
	}

	for _, f := range findings {
		if f.Evidence == "" {
			t.Fatalf("expected redacted evidence for finding %+v", f)
		}
		if strings.Contains(f.Evidence, "AKIA1234567890ABCDEF") {
			t.Fatalf("evidence should be redacted, got %q", f.Evidence)
		}
	}

	// Test/dev key should be ignored.
	testKeyBody := `{"apiKey":"test_key_for_dev_only_123456789"}`
	testFindings := scanner.scanBodyForCredentials(testKeyBody, "https://api.example.com/login", "request body")
	for _, f := range testFindings {
		if strings.Contains(strings.ToLower(f.Title), "api key") && strings.Contains(strings.ToLower(f.Evidence), "test_key") {
			t.Fatalf("test keys should not be flagged as credentials: %+v", testFindings)
		}
	}
}

func TestCredentialScanner_ConsolePatterns(t *testing.T) {
	t.Parallel()
	scanner := NewScanner()

	bearerAndJWT := LogEntry{
		"message": "Authorization: Bearer token_abcd1234efgh5678ijkl9012 with jwt eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.signature1234567890",
		"source":  "auth.js:10",
	}
	findings := scanner.scanConsoleForCredentials(bearerAndJWT)
	if len(findings) != 1 || !strings.Contains(findings[0].Title, "Bearer token") {
		t.Fatalf("expected single bearer-token finding (no JWT double-count), got %+v", findings)
	}

	jwtOnly := LogEntry{
		"message": "jwt: eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.signature1234567890",
		"source":  "auth.js:20",
	}
	jwtFindings := scanner.scanConsoleForCredentials(jwtOnly)
	if len(jwtFindings) != 1 || !strings.Contains(jwtFindings[0].Title, "JWT token") {
		t.Fatalf("expected JWT console finding, got %+v", jwtFindings)
	}
}

// First/third-party URL classification now lives in internal/security/httpsec
// and is covered by httpsec/url_test.go.
func TestSecurityHelpers_isTestKey(t *testing.T) {
	t.Parallel()

	if !isTestKey("my_TEST_token_123") {
		t.Fatal("isTestKey should detect test/dev indicators")
	}
	if isTestKey("production_abc987654321") {
		t.Fatal("isTestKey should not classify production-like tokens as test keys")
	}
}

func TestHandleSecurityAudit_CredentialsOnlyPath(t *testing.T) {
	t.Parallel()
	scanner := NewScanner()

	params := json.RawMessage(`{"checks":["credentials"]}`)
	bodies := []types.NetworkBody{
		{
			Method: "GET",
			URL:    "https://api.example.com/data?api_key=live_secret_1234567890abcdef",
		},
	}
	console := []LogEntry{
		{
			"message": "Bearer token_abcdefghijklmnopqrstuvwxyz123456",
			"source":  "console",
		},
	}

	result, err := scanner.HandleSecurityAudit(params, bodies, console, nil, nil)
	if err != nil {
		t.Fatalf("HandleSecurityAudit returned error: %v", err)
	}

	scanResult, ok := result.(Result)
	if !ok {
		t.Fatalf("expected Result, got %T", result)
	}
	if len(scanResult.Findings) == 0 {
		t.Fatal("expected credential findings")
	}
	for _, f := range scanResult.Findings {
		if f.Check != "credentials" {
			t.Fatalf("credentials-only audit should only return credentials findings, got %+v", scanResult.Findings)
		}
	}
}
