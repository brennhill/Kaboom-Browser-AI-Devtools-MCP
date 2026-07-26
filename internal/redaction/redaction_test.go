// Purpose: Tests for PII and credential redaction correctness.
// Docs: docs/features/feature/redaction-patterns/index.md

// These were gated behind `//go:build integration` with a note saying raceEnabled
// "needs to be exported or defined here". It is defined here — race_enabled_test.go
// and race_disabled_test.go declare it as a build-tag-selected constant in this same
// package, so the stated blocker was already solved. The tag is gone and these tests
// run by default.
package redaction

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ============================================
// Built-in Pattern Tests
// ============================================

func TestRedactBearerToken(t *testing.T) {
	t.Parallel()
	engine := NewRedactionEngine("")
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "standard bearer token",
			input: `Authorization: Bearer eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.payload.sig`,
			want:  `Authorization: [REDACTED:bearer-token]`,
		},
		{
			name:  "bearer in JSON",
			input: `{"token": "Bearer abc123def456-._~+/="}`,
			want:  `{"token": "[REDACTED:bearer-token]"}`,
		},
		{
			name:  "no bearer keyword",
			input: `Authorization: Basic dXNlcjpwYXNz`,
			want:  `Authorization: [REDACTED:basic-auth]`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := engine.Redact(tt.input)
			if got != tt.want {
				t.Errorf("Redact() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRedactAWSKeys(t *testing.T) {
	t.Parallel()
	engine := NewRedactionEngine("")
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "AWS access key ID",
			input: `aws_access_key_id = AKIAIOSFODNN7EXAMPLE`,
			want:  `aws_access_key_id = [REDACTED:aws-key]`,
		},
		{
			name:  "AWS key in environment variable",
			input: `AWS_ACCESS_KEY_ID=AKIAI44QH8DHBEXAMPLE`,
			want:  `AWS_ACCESS_KEY_ID=[REDACTED:aws-key]`,
		},
		{
			name:  "not an AWS key (too short)",
			input: `AKIA1234`,
			want:  `AKIA1234`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := engine.Redact(tt.input)
			if got != tt.want {
				t.Errorf("Redact() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRedactJWT(t *testing.T) {
	t.Parallel()
	engine := NewRedactionEngine("")
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "standard JWT",
			input: `token: eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U`,
			want:  `token: [REDACTED:jwt]`,
		},
		{
			name:  "JWT with URL-safe base64",
			input: `eyJhbGciOiJSUzI1NiJ9.eyJpc3MiOiJqb2UiLCJleHAiOjEzMDA4MTkzODB9.abc_def-ghi`,
			want:  `[REDACTED:jwt]`,
		},
		{
			name:  "not a JWT (missing parts)",
			input: `eyJhbGciOiJIUzI1NiJ9.notavalidjwt`,
			want:  `eyJhbGciOiJIUzI1NiJ9.notavalidjwt`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := engine.Redact(tt.input)
			if got != tt.want {
				t.Errorf("Redact() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRedactGitHubPAT(t *testing.T) {
	t.Parallel()
	engine := NewRedactionEngine("")
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "GitHub personal access token (classic)",
			input: `GITHUB_TOKEN=ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghij`,
			want:  `GITHUB_TOKEN=[REDACTED:github-pat]`,
		},
		{
			name:  "GitHub fine-grained PAT",
			input: `token: github_pat_1234567890ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghij1234567890abcdefghijklmnopq`,
			want:  `token: [REDACTED:github-pat]`,
		},
		{
			name:  "not a GitHub PAT",
			input: `ghp_short`,
			want:  `ghp_short`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := engine.Redact(tt.input)
			if got != tt.want {
				t.Errorf("Redact() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRedactPrivateKey(t *testing.T) {
	t.Parallel()
	engine := NewRedactionEngine("")

	input := `Here is my key:
-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEA0Z3VS5JJcds3xfn/yGmDq2sNDG8K
-----END RSA PRIVATE KEY-----
done`

	got := engine.Redact(input)
	if !strings.Contains(got, "[REDACTED:private-key]") {
		t.Errorf("Expected private key to be redacted, got: %q", got)
	}
	if strings.Contains(got, "MIIEpAIBAAKCAQEA") {
		t.Errorf("Private key content should not be present in output")
	}
}

func TestRedactCreditCard(t *testing.T) {
	t.Parallel()
	engine := NewRedactionEngine("")
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "visa card with spaces",
			input: `card: 4111 1111 1111 1111`,
			want:  `card: [REDACTED:credit-card]`,
		},
		{
			name:  "visa card with dashes",
			input: `card: 4111-1111-1111-1111`,
			want:  `card: [REDACTED:credit-card]`,
		},
		{
			name:  "visa card no separators",
			input: `card: 4111111111111111`,
			want:  `card: [REDACTED:credit-card]`,
		},
		{
			name:  "not a valid card (fails Luhn)",
			input: `number: 1234567890123456`,
			want:  `number: 1234567890123456`,
		},
		{
			name:  "too short",
			input: `4111 1111 1111`,
			want:  `4111 1111 1111`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := engine.Redact(tt.input)
			if got != tt.want {
				t.Errorf("Redact() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRedactSSN(t *testing.T) {
	t.Parallel()
	engine := NewRedactionEngine("")
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "standard SSN",
			input: `ssn: 123-45-6789`,
			want:  `ssn: [REDACTED:ssn]`,
		},
		{
			name:  "SSN in text",
			input: `Patient SSN is 078-05-1120 on file`,
			want:  `Patient SSN is [REDACTED:ssn] on file`,
		},
		{
			name:  "not an SSN (no dashes)",
			input: `123456789`,
			want:  `123456789`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := engine.Redact(tt.input)
			if got != tt.want {
				t.Errorf("Redact() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRedactAPIKey(t *testing.T) {
	t.Parallel()
	engine := NewRedactionEngine("")
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "api_key in header",
			input: `api_key: sk-1234567890abcdef`,
			want:  `[REDACTED:api-key]`,
		},
		{
			name:  "API-Key format",
			input: `API-Key = my_secret_key_value`,
			want:  `[REDACTED:api-key]`,
		},
		{
			name:  "secret_key assignment",
			input: `secret_key=super_secret_123`,
			want:  `[REDACTED:api-key]`,
		},
		{
			name:  "apikey single word",
			input: `apikey: abcdef123456`,
			want:  `[REDACTED:api-key]`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := engine.Redact(tt.input)
			if got != tt.want {
				t.Errorf("Redact() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRedactBasicAuth(t *testing.T) {
	t.Parallel()
	engine := NewRedactionEngine("")
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "basic auth header",
			input: `Authorization: Basic dXNlcjpwYXNzd29yZA==`,
			want:  `Authorization: [REDACTED:basic-auth]`,
		},
		{
			name:  "basic auth in JSON",
			input: `{"auth": "Basic YWRtaW46c2VjcmV0"}`,
			want:  `{"auth": "[REDACTED:basic-auth]"}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := engine.Redact(tt.input)
			if got != tt.want {
				t.Errorf("Redact() = %q, want %q", got, tt.want)
			}
		})
	}
}

// ============================================
// Custom Pattern Tests
// ============================================

func TestRedactCustomPatterns(t *testing.T) {
	t.Parallel()
	// Create a temp config file
	config := RedactionConfig{
		Patterns: []RedactionPattern{
			{Name: "internal-id", Pattern: `CUST-[0-9]{8}`},
			{Name: "medical-record", Pattern: `MRN[0-9]{7,10}`},
		},
	}
	configJSON, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "redaction.json")
	if err := os.WriteFile(configPath, configJSON, 0600); err != nil {
		t.Fatal(err)
	}

	engine := NewRedactionEngine(configPath)

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "customer ID",
			input: `Customer: CUST-12345678`,
			want:  `Customer: [REDACTED:internal-id]`,
		},
		{
			name:  "medical record number",
			input: `Record: MRN1234567`,
			want:  `Record: [REDACTED:medical-record]`,
		},
		{
			name:  "no match",
			input: `Normal text without patterns`,
			want:  `Normal text without patterns`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := engine.Redact(tt.input)
			if got != tt.want {
				t.Errorf("Redact() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRedactCustomPatternWithReplacement(t *testing.T) {
	t.Parallel()
	config := RedactionConfig{
		Patterns: []RedactionPattern{
			{Name: "custom", Pattern: `SECRET-[A-Z]+`, Replacement: "[HIDDEN]"},
		},
	}
	configJSON, _ := json.Marshal(config)
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "redaction.json")
	os.WriteFile(configPath, configJSON, 0600)

	engine := NewRedactionEngine(configPath)
	got := engine.Redact("Value: SECRET-ABCDEF")
	want := "Value: [HIDDEN]"
	if got != want {
		t.Errorf("Redact() = %q, want %q", got, want)
	}
}

// ============================================
// Config Loading Tests
// ============================================

func TestRedactionEngineNoConfig(t *testing.T) {
	t.Parallel()
	// Empty config path should still work with built-in patterns
	engine := NewRedactionEngine("")
	if engine == nil {
		t.Fatal("NewRedactionEngine returned nil with empty config")
	}
	// Built-in patterns should work
	got := engine.Redact("Bearer abc123token")
	if !strings.Contains(got, "[REDACTED:bearer-token]") {
		t.Errorf("Built-in patterns should work without config, got: %q", got)
	}
}

func TestRedactionEngineInvalidConfigPath(t *testing.T) {
	t.Parallel()
	// Non-existent config path should not crash, just use built-ins
	engine := NewRedactionEngine("/nonexistent/path/config.json")
	if engine == nil {
		t.Fatal("NewRedactionEngine returned nil with invalid path")
	}
	// Built-in patterns still work
	got := engine.Redact("Bearer token123value")
	if !strings.Contains(got, "[REDACTED:bearer-token]") {
		t.Errorf("Built-in patterns should work with invalid config path, got: %q", got)
	}
}

func TestRedactionEngineInvalidJSON(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "bad.json")
	os.WriteFile(configPath, []byte(`not valid json`), 0600)

	// Should not panic, just use built-ins
	engine := NewRedactionEngine(configPath)
	if engine == nil {
		t.Fatal("NewRedactionEngine returned nil with invalid JSON")
	}
}

func TestRedactionEngineInvalidRegex(t *testing.T) {
	t.Parallel()
	// RE2 doesn't support backreferences, lookahead, etc.
	config := RedactionConfig{
		Patterns: []RedactionPattern{
			{Name: "valid", Pattern: `[0-9]+`},
			{Name: "invalid-backref", Pattern: `(abc)\1`}, // backreference - invalid in RE2
		},
	}
	configJSON, _ := json.Marshal(config)
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "redaction.json")
	os.WriteFile(configPath, configJSON, 0600)

	// Should not panic; valid patterns still work, invalid ones are skipped
	engine := NewRedactionEngine(configPath)
	got := engine.Redact("test 12345 value")
	if !strings.Contains(got, "[REDACTED:valid]") {
		t.Errorf("Valid patterns should still work when invalid ones are skipped, got: %q", got)
	}
}

// ============================================
// Edge Cases
// ============================================

func TestRedactEmptyInput(t *testing.T) {
	t.Parallel()
	engine := NewRedactionEngine("")
	got := engine.Redact("")
	if got != "" {
		t.Errorf("Redact empty string should return empty, got: %q", got)
	}
}

func TestRedactNoMatch(t *testing.T) {
	t.Parallel()
	engine := NewRedactionEngine("")
	input := "This is a normal log message with no sensitive data"
	got := engine.Redact(input)
	if got != input {
		t.Errorf("Non-matching content should pass through unchanged, got: %q", got)
	}
}

func TestRedactMultipleMatchesSameLine(t *testing.T) {
	t.Parallel()
	engine := NewRedactionEngine("")
	input := `token1: Bearer abc123 and token2: Bearer def456`
	got := engine.Redact(input)
	count := strings.Count(got, "[REDACTED:bearer-token]")
	if count != 2 {
		t.Errorf("Expected 2 redactions, got %d in: %q", count, got)
	}
}

func TestRedactMultiplePatternTypes(t *testing.T) {
	t.Parallel()
	engine := NewRedactionEngine("")
	input := `Auth: Bearer mytoken123 SSN: 123-45-6789`
	got := engine.Redact(input)
	if !strings.Contains(got, "[REDACTED:bearer-token]") {
		t.Errorf("Expected bearer-token redaction in: %q", got)
	}
	if !strings.Contains(got, "[REDACTED:ssn]") {
		t.Errorf("Expected ssn redaction in: %q", got)
	}
}

func TestRedactBinaryContent(t *testing.T) {
	t.Parallel()
	engine := NewRedactionEngine("")
	// Binary-like content should pass through without issues
	input := string([]byte{0x00, 0x01, 0x02, 0xFF, 0xFE})
	got := engine.Redact(input)
	if got != input {
		t.Errorf("Binary content should pass through unchanged")
	}
}

func TestRedactLargeInput(t *testing.T) {
	t.Parallel()
	engine := NewRedactionEngine("")
	// 100KB of text with a few secrets embedded
	builder := strings.Builder{}
	for i := 0; i < 2000; i++ {
		builder.WriteString("This is a normal log line number ")
		builder.WriteString(strings.Repeat("x", 50))
		builder.WriteString("\n")
	}
	// Insert secrets at various positions
	base := builder.String()
	input := base[:len(base)/4] + "Bearer secret_token_123" + base[len(base)/4:len(base)/2] + "123-45-6789 " + base[len(base)/2:]

	got := engine.Redact(input)
	if !strings.Contains(got, "[REDACTED:bearer-token]") {
		t.Errorf("Should redact bearer token in large input")
	}
	if !strings.Contains(got, "[REDACTED:ssn]") {
		t.Errorf("Should redact SSN in large input")
	}
}

// ============================================
// Redaction Format Tests
// ============================================

func TestRedactFormat(t *testing.T) {
	t.Parallel()
	engine := NewRedactionEngine("")
	tests := []struct {
		name    string
		input   string
		pattern string // expected pattern name in redaction
	}{
		{"bearer format", "Bearer token123abc", "bearer-token"},
		{"ssn format", "123-45-6789", "ssn"},
		{"aws format", "AKIAIOSFODNN7EXAMPLE", "aws-key"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := engine.Redact(tt.input)
			expected := "[REDACTED:" + tt.pattern + "]"
			if !strings.Contains(got, expected) {
				t.Errorf("Expected format %q in output %q", expected, got)
			}
		})
	}
}

// ============================================
// Thread Safety Tests
// ============================================

func TestRedactConcurrent(t *testing.T) {
	t.Parallel()
	engine := NewRedactionEngine("")
	done := make(chan bool, 100)

	for i := 0; i < 100; i++ {
		go func() {
			result := engine.Redact("Bearer my_secret_token_123")
			if !strings.Contains(result, "[REDACTED:bearer-token]") {
				t.Errorf("Concurrent redaction failed: %q", result)
			}
			done <- true
		}()
	}

	for i := 0; i < 100; i++ {
		<-done
	}
}

// ============================================
// Performance Benchmark
// ============================================

func BenchmarkRedactSmallInput(b *testing.B) {
	engine := NewRedactionEngine("")
	// ~5KB input with a few secrets
	input := strings.Repeat("Normal log line with some text. ", 150) +
		"Bearer secret123token " +
		strings.Repeat("More text here. ", 10) +
		"SSN: 123-45-6789"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		engine.Redact(input)
	}
}

func TestRedactPerformanceSmall(t *testing.T) {
	t.Parallel()
	if raceEnabled {
		t.Skip("Performance SLO test skipped under race detector")
	}
	engine := NewRedactionEngine("")
	// ~5KB input
	input := strings.Repeat("Normal log line with some text data. ", 140) +
		"Bearer secret_token_value " +
		"SSN: 123-45-6789"

	if len(input) < 4000 || len(input) > 6000 {
		t.Fatalf("Test input should be ~5KB, got %d bytes", len(input))
	}

	start := time.Now()
	iterations := 100
	for i := 0; i < iterations; i++ {
		engine.Redact(input)
	}
	elapsed := time.Since(start)
	avgMs := float64(elapsed.Milliseconds()) / float64(iterations)

	if avgMs > 2.0 {
		t.Errorf("Average redaction time %.2fms exceeds 2ms SLO for ~5KB input", avgMs)
	}
}

func TestRedactPerformanceLarge(t *testing.T) {
	t.Parallel()
	if raceEnabled {
		t.Skip("Performance SLO test skipped under race detector")
	}
	engine := NewRedactionEngine("")
	// ~100KB input
	input := strings.Repeat("Large response body with various content repeated many times. ", 1700) +
		"Bearer hidden_secret " +
		"AKIAIOSFODNN7EXAMPLE " +
		"123-45-6789"

	if len(input) < 90000 {
		t.Fatalf("Test input should be ~100KB, got %d bytes", len(input))
	}

	start := time.Now()
	iterations := 10
	for i := 0; i < iterations; i++ {
		engine.Redact(input)
	}
	elapsed := time.Since(start)
	avgMs := float64(elapsed.Milliseconds()) / float64(iterations)

	if avgMs > 30.0 {
		t.Errorf("Average redaction time %.2fms exceeds 30ms for ~100KB input", avgMs)
	}
}

// ============================================
// MCP Response Integration Tests
// ============================================

func TestRedactMCPToolResult(t *testing.T) {
	t.Parallel()
	engine := NewRedactionEngine("")

	// Simulate an MCP tool result with sensitive content
	result := MCPToolResult{
		Content: []MCPContentBlock{
			{Type: "text", Text: `{"headers": {"Authorization": "Bearer secret123abc"}, "body": "SSN: 123-45-6789"}`},
		},
	}
	resultJSON, _ := json.Marshal(result)

	redacted := engine.RedactJSON(resultJSON)

	var redactedResult MCPToolResult
	if err := json.Unmarshal(redacted, &redactedResult); err != nil {
		t.Fatalf("Redacted JSON should be valid: %v", err)
	}

	text := redactedResult.Content[0].Text
	if strings.Contains(text, "secret123abc") {
		t.Errorf("Bearer token should be redacted from MCP result, got: %s", text)
	}
	if strings.Contains(text, "123-45-6789") {
		t.Errorf("SSN should be redacted from MCP result, got: %s", text)
	}
	if !strings.Contains(text, "[REDACTED:bearer-token]") {
		t.Errorf("Expected bearer-token redaction marker in: %s", text)
	}
	if !strings.Contains(text, "[REDACTED:ssn]") {
		t.Errorf("Expected ssn redaction marker in: %s", text)
	}
}

func TestRedactJSONPreservesStructure(t *testing.T) {
	t.Parallel()
	engine := NewRedactionEngine("")

	// Non-sensitive JSON should pass through structurally intact
	input := `{"content":[{"type":"text","text":"Hello world, no secrets here"}]}`
	got := engine.RedactJSON(json.RawMessage(input))

	var result MCPToolResult
	if err := json.Unmarshal(got, &result); err != nil {
		t.Fatalf("Output should be valid JSON: %v", err)
	}
	if result.Content[0].Text != "Hello world, no secrets here" {
		t.Errorf("Non-sensitive content should be unchanged, got: %s", result.Content[0].Text)
	}
}

func TestRedactJSONMultipleContentBlocks(t *testing.T) {
	t.Parallel()
	engine := NewRedactionEngine("")

	result := MCPToolResult{
		Content: []MCPContentBlock{
			{Type: "text", Text: "Bearer token_one"},
			{Type: "text", Text: "SSN: 999-88-7777"},
			{Type: "text", Text: "No secrets here"},
		},
	}
	resultJSON, _ := json.Marshal(result)

	redacted := engine.RedactJSON(resultJSON)
	var redactedResult MCPToolResult
	if err := json.Unmarshal(redacted, &redactedResult); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	if !strings.Contains(redactedResult.Content[0].Text, "[REDACTED:bearer-token]") {
		t.Errorf("First block should be redacted: %s", redactedResult.Content[0].Text)
	}
	if !strings.Contains(redactedResult.Content[1].Text, "[REDACTED:ssn]") {
		t.Errorf("Second block should be redacted: %s", redactedResult.Content[1].Text)
	}
	if redactedResult.Content[2].Text != "No secrets here" {
		t.Errorf("Third block should be unchanged: %s", redactedResult.Content[2].Text)
	}
}

// ============================================
// Luhn Validation Tests
// ============================================

func TestLuhnValidation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input string
		valid bool
	}{
		{"valid visa", "4111111111111111", true},
		{"valid mastercard", "5500000000000004", true},
		{"valid amex (15-digit, Luhn valid)", "378282246310005", true}, // Luhn validates 13-19 digits
		{"invalid number", "1234567890123456", false},
		{"all zeros", "0000000000000000", true}, // Luhn valid but unlikely real
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := luhnValid(tt.input)
			if got != tt.valid {
				t.Errorf("luhnValid(%q) = %v, want %v", tt.input, got, tt.valid)
			}
		})
	}
}

// ============================================
// Session Cookie / Token Tests
// ============================================

func TestRedactSessionCookie(t *testing.T) {
	t.Parallel()
	engine := NewRedactionEngine("")
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "session cookie",
			input: `Cookie: session=abcdef1234567890ABCDEF`,
			want:  `Cookie: [REDACTED:session-cookie]`,
		},
		{
			name:  "sid value",
			input: `sid=A1B2C3D4E5F6G7H8I9J0K1L2M3N4O5P6`,
			want:  `[REDACTED:session-cookie]`,
		},
		{
			name:  "token assignment",
			input: `token = eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9abcdef`,
			want:  `[REDACTED:session-cookie]`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := engine.Redact(tt.input)
			if got != tt.want {
				t.Errorf("Redact() = %q, want %q", got, tt.want)
			}
		})
	}
}
