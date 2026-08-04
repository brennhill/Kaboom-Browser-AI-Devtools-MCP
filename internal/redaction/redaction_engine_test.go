// redaction_engine_test.go — Tests redaction engine robustness and performance.
// Docs: docs/features/feature/redaction-patterns/index.md

// These were gated behind `//go:build integration` with a note saying raceEnabled
// "needs to be exported or defined here". It is defined here — race_enabled_test.go
// and race_disabled_test.go declare it as a build-tag-selected constant in this same
// package, so the stated blocker was already solved. The tag is gone and these tests
// run by default.
package redaction

import (
	"strings"
	"testing"
	"time"
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
