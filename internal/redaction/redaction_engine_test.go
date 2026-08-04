// redaction_engine_test.go — Tests engine robustness, edge cases, and performance.
package redaction

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

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
	if testing.Short() {
		t.Skip("wall-clock SLO runs in the isolated performance lane")
	}
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
	if testing.Short() {
		t.Skip("wall-clock SLO runs in the isolated performance lane")
	}
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
// loadConfig — file not found path
// ============================================

func TestLoadConfig_FileNotFound(t *testing.T) {
	t.Parallel()
	engine := NewRedactionEngine("/nonexistent/path/to/config.json")
	// Should still have built-in patterns and not panic.
	if engine == nil {
		t.Fatal("engine should not be nil for missing config file")
	}
	if len(engine.patterns) == 0 {
		t.Fatal("engine should have built-in patterns even with missing config")
	}
	// Verify builtins still work.
	got := engine.Redact("Bearer my_token_123")
	if !strings.Contains(got, "[REDACTED:bearer-token]") {
		t.Errorf("built-in patterns should work, got %q", got)
	}
}

// ============================================
// loadConfig — invalid JSON path
// ============================================

func TestLoadConfig_InvalidJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(path, []byte(`{not valid json!!!`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	engine := NewRedactionEngine(path)
	if engine == nil {
		t.Fatal("engine should not be nil for invalid JSON config")
	}
	// Only built-in patterns should be loaded.
	got := engine.Redact("AKIAIOSFODNN7EXAMPLE")
	if !strings.Contains(got, "[REDACTED:aws-key]") {
		t.Errorf("built-in patterns should work, got %q", got)
	}
}

// ============================================
// loadConfig — invalid regex in custom patterns (skipped)
// ============================================

func TestLoadConfig_InvalidRegexSkipped(t *testing.T) {
	t.Parallel()
	config := RedactionConfig{
		Patterns: []RedactionPattern{
			{Name: "valid-pattern", Pattern: `CUSTOM_[0-9]+`},
			{Name: "bad-regex", Pattern: `[unclosed`},
		},
	}
	data, _ := json.Marshal(config)
	dir := t.TempDir()
	path := filepath.Join(dir, "redaction.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	engine := NewRedactionEngine(path)
	// Valid pattern should work.
	got := engine.Redact("CUSTOM_42")
	if !strings.Contains(got, "[REDACTED:valid-pattern]") {
		t.Errorf("valid custom pattern should work, got %q", got)
	}
	// Bad regex should not cause any issue.
	safe := engine.Redact("normal text")
	if safe != "normal text" {
		t.Errorf("non-matching text should pass through, got %q", safe)
	}
}

// ============================================
// Redact — empty input
// ============================================

func TestRedact_EmptyInput(t *testing.T) {
	t.Parallel()
	engine := NewRedactionEngine("")
	got := engine.Redact("")
	if got != "" {
		t.Errorf("Redact(\"\") = %q, want empty", got)
	}
}

// ============================================
// RedactJSON — malformed non-text block fields cannot bypass redaction
// ============================================

func TestRedactJSON_NonTextBlockCredentialRedacted(t *testing.T) {
	t.Parallel()
	engine := NewRedactionEngine("")
	result := mcp.MCPToolResult{
		Content: []mcp.MCPContentBlock{
			{Type: "image", Text: "Bearer secret_token_123"},
		},
	}
	data, _ := json.Marshal(result)
	out := engine.RedactJSON(data)
	var parsed mcp.MCPToolResult
	if err := json.Unmarshal(out, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !strings.Contains(parsed.Content[0].Text, "[REDACTED:bearer-token]") {
		t.Errorf("malformed non-text field should be redacted, got %q", parsed.Content[0].Text)
	}
}

// ============================================
// RedactJSON — malformed JSON fallback
// ============================================

func TestRedactJSON_MalformedJSONFallback(t *testing.T) {
	t.Parallel()
	engine := NewRedactionEngine("")
	malformed := json.RawMessage(`{"content": [broken json Bearer secret_abc}`)
	out := engine.RedactJSON(malformed)
	if !strings.Contains(string(out), "[REDACTED:bearer-token]") {
		t.Errorf("fallback should still redact raw string, got %q", string(out))
	}
}

// ============================================
// luhnValid — edge cases for digit doubling (n > 9 branch)
// ============================================

func TestLuhnValid_DoubledDigitExceedsNine(t *testing.T) {
	t.Parallel()
	// Card number where doubled alternate digits exceed 9 triggers n -= 9.
	// Mastercard 5500000000000004 has digits where doubling produces > 9.
	if !luhnValid("5500000000000004") {
		t.Error("expected 5500000000000004 to be Luhn-valid")
	}
	// More explicit: 79927398710 is a well-known Luhn-valid number.
	// Digits: 7 9 9 2 7 3 9 8 7 1 0
	// When doubling from right, position 1 (from right, 0-indexed) = 1*2=2, pos3 = 8*2=16>9 => 16-9=7
	// This exercises the n-=9 branch.
	if !luhnValid("79927398710") {
		// 11 digits is below 13 minimum, this should fail length check
	}
	// Use a 16-digit Luhn-valid number with high alternate digits: 6011111111111117
	if !luhnValid("6011111111111117") {
		t.Error("expected 6011111111111117 (Discover) to be Luhn-valid")
	}
}

func TestLuhnValid_TooShort(t *testing.T) {
	t.Parallel()
	if luhnValid("123456") {
		t.Error("expected 6-digit number to fail length check")
	}
}

func TestLuhnValid_TooLong(t *testing.T) {
	t.Parallel()
	// 20 digits exceeds max of 19
	if luhnValid("12345678901234567890") {
		t.Error("expected 20-digit number to fail length check")
	}
}

func TestLuhnValid_WithSeparators(t *testing.T) {
	t.Parallel()
	// Visa with dashes
	if !luhnValid("4111-1111-1111-1111") {
		t.Error("expected Visa with dashes to be Luhn-valid")
	}
	// Visa with spaces
	if !luhnValid("4111 1111 1111 1111") {
		t.Error("expected Visa with spaces to be Luhn-valid")
	}
}

func TestLuhnValid_InvalidCheckDigit(t *testing.T) {
	t.Parallel()
	// 4111111111111112 is NOT Luhn-valid (wrong check digit)
	if luhnValid("4111111111111112") {
		t.Error("expected 4111111111111112 to fail Luhn check")
	}
}

// ============================================
// NewRedactionEngine — builtin pattern compile error (continue branch)
// ============================================
// NOTE: The built-in patterns are hardcoded and always compile successfully,
// so the `continue` after compile error is unreachable in practice.
// This test verifies that the engine initializes correctly even though
// that branch is technically unreachable with current builtins.

func TestNewRedactionEngine_BuiltinPatternsCompile(t *testing.T) {
	t.Parallel()
	engine := NewRedactionEngine("")
	// Verify all built-in patterns compiled successfully.
	// builtinPatterns has len(builtinPatterns) entries.
	if len(engine.patterns) < len(builtinPatterns) {
		t.Errorf("engine has %d patterns, want at least %d (all builtins)", len(engine.patterns), len(builtinPatterns))
	}
	// Verify each builtin has a name.
	for i, p := range engine.patterns {
		if p.name == "" {
			t.Errorf("pattern %d has empty name", i)
		}
		if p.regex == nil {
			t.Errorf("pattern %d (%s) has nil regex", i, p.name)
		}
		if p.replacement == "" {
			t.Errorf("pattern %d (%s) has empty replacement", i, p.name)
		}
	}
}

// ============================================
// RedactJSON — isError field preserved
// ============================================

func TestRedactJSON_IsErrorPreserved(t *testing.T) {
	t.Parallel()
	engine := NewRedactionEngine("")
	result := mcp.MCPToolResult{
		Content: []mcp.MCPContentBlock{
			{Type: "text", Text: "error: Bearer secret123"},
		},
		IsError: true,
	}
	data, _ := json.Marshal(result)
	out := engine.RedactJSON(data)
	var parsed mcp.MCPToolResult
	if err := json.Unmarshal(out, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !parsed.IsError {
		t.Error("IsError should be preserved after redaction")
	}
	if !strings.Contains(parsed.Content[0].Text, "[REDACTED:bearer-token]") {
		t.Errorf("text should be redacted, got %q", parsed.Content[0].Text)
	}
}

// ============================================
// RedactJSON — empty content blocks
// ============================================

func TestRedactJSON_EmptyContent(t *testing.T) {
	t.Parallel()
	engine := NewRedactionEngine("")
	result := mcp.MCPToolResult{
		Content: []mcp.MCPContentBlock{},
	}
	data, _ := json.Marshal(result)
	out := engine.RedactJSON(data)
	var parsed mcp.MCPToolResult
	if err := json.Unmarshal(out, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(parsed.Content) != 0 {
		t.Errorf("expected empty content, got %d blocks", len(parsed.Content))
	}
}

// ============================================
// Custom config — pattern with custom replacement
// ============================================

func TestLoadConfig_CustomReplacement(t *testing.T) {
	t.Parallel()
	config := RedactionConfig{
		Patterns: []RedactionPattern{
			{Name: "custom", Pattern: `SECRET_[A-Z]+`, Replacement: "***HIDDEN***"},
		},
	}
	data, _ := json.Marshal(config)
	dir := t.TempDir()
	path := filepath.Join(dir, "redaction.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	engine := NewRedactionEngine(path)
	got := engine.Redact("my SECRET_ABC value")
	if got != "my ***HIDDEN*** value" {
		t.Errorf("custom replacement failed, got %q", got)
	}
}

// ============================================
// Custom config — pattern with default replacement (empty Replacement field)
// ============================================

func TestLoadConfig_DefaultReplacement(t *testing.T) {
	t.Parallel()
	config := RedactionConfig{
		Patterns: []RedactionPattern{
			{Name: "mypattern", Pattern: `MYID_[0-9]+`},
		},
	}
	data, _ := json.Marshal(config)
	dir := t.TempDir()
	path := filepath.Join(dir, "redaction.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	engine := NewRedactionEngine(path)
	got := engine.Redact("found MYID_12345")
	if got != "found [REDACTED:mypattern]" {
		t.Errorf("default replacement failed, got %q", got)
	}
}
