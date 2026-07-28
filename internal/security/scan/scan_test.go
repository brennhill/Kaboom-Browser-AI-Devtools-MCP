// scan_test.go — Tests security scanner orchestration, filtering, and result safety.
// Docs: docs/features/feature/security-hardening/index.md

// These were previously gated behind `//go:build integration` because "NetworkBody
// needs to be imported from capture package". It is imported below, so the tag is
// gone and the scanner's end-to-end tests run by default.
package scan

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

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
		NetworkBodies:  []types.NetworkBody{},
		ConsoleEntries: []types.LogEntry{},
		PageURLs:       []string{},
	}
	result := scanner.Scan(input)

	if len(result.Findings) != 0 {
		t.Errorf("expected 0 findings, got %d", len(result.Findings))
	}
}

// ============================================
// Summary Tests
// ============================================

func TestSecurityScan_SummaryAccuracy(t *testing.T) {
	t.Parallel()
	scanner := NewScanner()
	input := Input{
		NetworkBodies: []types.NetworkBody{
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
		NetworkBodies: []types.NetworkBody{
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
		NetworkBodies: []types.NetworkBody{
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
		NetworkBodies: []types.NetworkBody{
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
		NetworkBodies: []types.NetworkBody{
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
		NetworkBodies: []types.NetworkBody{
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
	bodies := []types.NetworkBody{
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
	bodies := []types.NetworkBody{
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
		NetworkBodies: []types.NetworkBody{
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
		NetworkBodies: []types.NetworkBody{
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
		NetworkBodies: []types.NetworkBody{
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
		NetworkBodies: []types.NetworkBody{
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
		ConsoleEntries: []types.LogEntry{
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
			NetworkBodies: []types.NetworkBody{
				{
					URL:             urlData,
					RequestBody:     bodyData,
					ResponseBody:    bodyData,
					ContentType:     "application/json",
					Status:          200,
					ResponseHeaders: map[string]string{"Set-Cookie": bodyData},
				},
			},
			ConsoleEntries: []types.LogEntry{
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
