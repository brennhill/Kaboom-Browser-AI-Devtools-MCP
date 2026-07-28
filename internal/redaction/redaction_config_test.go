// redaction_config_test.go — Tests custom redaction patterns and configuration.
// Docs: docs/features/feature/redaction-patterns/index.md

package redaction

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
