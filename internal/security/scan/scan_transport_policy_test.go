// scan_transport_policy_test.go — Tests headers, cookies, and transport-policy findings.
// Docs: docs/features/feature/security-hardening/index.md

package scan

import (
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

// ============================================
// Security Headers Tests
// ============================================

func TestSecurityScan_MissingHSTS(t *testing.T) {
	t.Parallel()
	scanner := NewScanner()
	input := Input{
		NetworkBodies: []types.NetworkBody{
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
		NetworkBodies: []types.NetworkBody{
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
		NetworkBodies: []types.NetworkBody{
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
		NetworkBodies: []types.NetworkBody{
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
		NetworkBodies: []types.NetworkBody{
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
		NetworkBodies: []types.NetworkBody{
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
		NetworkBodies: []types.NetworkBody{
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
		NetworkBodies: []types.NetworkBody{
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
		NetworkBodies: []types.NetworkBody{
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
		NetworkBodies: []types.NetworkBody{
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
		NetworkBodies: []types.NetworkBody{
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
		NetworkBodies: []types.NetworkBody{
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
		NetworkBodies: []types.NetworkBody{
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
		NetworkBodies: []types.NetworkBody{
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
