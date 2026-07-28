// scan_sensitive_data_test.go — Tests credential, PII, and evidence-redaction findings.
// Docs: docs/features/feature/security-hardening/index.md

package scan

import (
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

// ============================================
// Credential Detection Tests
// ============================================

func TestSecurityScan_APIKeyInURL(t *testing.T) {
	t.Parallel()
	scanner := NewScanner()
	input := Input{
		NetworkBodies: []types.NetworkBody{
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
		NetworkBodies: []types.NetworkBody{
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
		NetworkBodies: []types.NetworkBody{
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
		NetworkBodies: []types.NetworkBody{
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
		NetworkBodies: []types.NetworkBody{
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
		NetworkBodies: []types.NetworkBody{
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
		NetworkBodies: []types.NetworkBody{
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
		ConsoleEntries: []types.LogEntry{
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
		NetworkBodies: []types.NetworkBody{
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
		NetworkBodies: []types.NetworkBody{
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
		NetworkBodies: []types.NetworkBody{
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
		NetworkBodies: []types.NetworkBody{
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
		NetworkBodies: []types.NetworkBody{
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
// Evidence Redaction Tests
// ============================================

func TestSecurityScan_EvidenceRedacted(t *testing.T) {
	t.Parallel()
	scanner := NewScanner()
	secretValue := "sk-proj-abcdefghijklmnopqrstuvwxyz1234567890"
	input := Input{
		NetworkBodies: []types.NetworkBody{
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
