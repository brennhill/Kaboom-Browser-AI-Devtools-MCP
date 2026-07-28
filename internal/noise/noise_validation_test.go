// Purpose: Tests for noise rule validation and rejection of invalid patterns.
// Docs: docs/features/feature/noise-filtering/index.md

// ai_noise_validation_test.go — Tests for noise JSON serialization, Reset, regex validation, and built-in rules.
package noise

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

// ============================================
// Noise JSON serialization: snake_case fields
// ============================================

func TestNoiseRule_JSONSnakeCase(t *testing.T) {
	t.Parallel()

	rule := NoiseRule{
		ID:             "test_1",
		Category:       "console",
		Classification: "repetitive",
		MatchSpec: NoiseMatchSpec{
			MessageRegex: "test",
			SourceRegex:  "source",
			URLRegex:     "url",
			Method:       "GET",
			StatusMin:    400,
			StatusMax:    499,
			Level:        "error",
		},
		AutoDetected: true,
		CreatedAt:    time.Now(),
		Reason:       "test reason",
	}

	data, err := json.Marshal(rule)
	if err != nil {
		t.Fatalf("json.Marshal error = %v", err)
	}

	jsonStr := string(data)

	// Verify snake_case field names
	expectedFields := []string{
		`"id"`, `"category"`, `"classification"`, `"match_spec"`,
		`"message_regex"`, `"source_regex"`, `"url_regex"`, `"method"`,
		`"status_min"`, `"status_max"`, `"level"`, `"auto_detected"`,
		`"created_at"`, `"reason"`,
	}
	for _, field := range expectedFields {
		if !strings.Contains(jsonStr, field) {
			t.Errorf("JSON output missing snake_case field %s, got: %s", field, jsonStr)
		}
	}
}

// ============================================
// NoiseStatistics JSON: snake_case fields
// ============================================

func TestNoiseStatistics_JSONSnakeCase(t *testing.T) {
	t.Parallel()

	stats := NoiseStatistics{
		TotalFiltered: 42,
		PerRule:       map[string]int{"rule_1": 10},
		LastSignalAt:  time.Now(),
		LastNoiseAt:   time.Now(),
	}

	data, err := json.Marshal(stats)
	if err != nil {
		t.Fatalf("json.Marshal error = %v", err)
	}

	jsonStr := string(data)
	expectedFields := []string{`"total_filtered"`, `"per_rule"`, `"last_signal_at"`, `"last_noise_at"`}
	for _, field := range expectedFields {
		if !strings.Contains(jsonStr, field) {
			t.Errorf("JSON missing snake_case field %s, got: %s", field, jsonStr)
		}
	}
}

// ============================================
// PersistedNoiseData JSON: snake_case fields
// ============================================

func TestPersistedNoiseData_JSONSnakeCase(t *testing.T) {
	t.Parallel()

	persisted := PersistedNoiseData{
		Version:    1,
		NextUserID: 5,
		Rules:      []NoiseRule{},
		Statistics: NoiseStatistics{PerRule: map[string]int{}},
	}

	data, err := json.Marshal(persisted)
	if err != nil {
		t.Fatalf("json.Marshal error = %v", err)
	}

	jsonStr := string(data)
	expectedFields := []string{`"version"`, `"next_user_id"`, `"rules"`}
	for _, field := range expectedFields {
		if !strings.Contains(jsonStr, field) {
			t.Errorf("JSON missing snake_case field %s, got: %s", field, jsonStr)
		}
	}
}

// ============================================
// Reset: statistics are cleared
// ============================================

func TestReset_StatisticsCleared(t *testing.T) {
	t.Parallel()
	nc := NewNoiseConfig()

	// Generate some statistics
	entry := types.LogEntry{"level": "info", "message": "[vite] hot updated", "source": "http://localhost:3000/app.js"}
	for i := 0; i < 5; i++ {
		nc.IsConsoleNoise(entry)
	}

	stats := nc.GetStatistics()
	if stats.TotalFiltered == 0 {
		t.Fatal("expected non-zero TotalFiltered before reset")
	}

	nc.Reset()

	resetStats := nc.GetStatistics()
	if resetStats.TotalFiltered != 0 {
		t.Errorf("TotalFiltered after reset = %d, want 0", resetStats.TotalFiltered)
	}
	if len(resetStats.PerRule) != 0 {
		t.Errorf("PerRule after reset = %v, want empty", resetStats.PerRule)
	}
}

// ============================================
// Reset: userIDCounter is reset to 0
// ============================================

func TestReset_UserIDCounterResetToZero(t *testing.T) {
	t.Parallel()
	nc := NewNoiseConfig()

	_ = nc.AddRules([]NoiseRule{
		{Category: "console", Classification: "repetitive", MatchSpec: NoiseMatchSpec{MessageRegex: "pre-reset"}},
	})

	nc.Reset()

	_ = nc.AddRules([]NoiseRule{
		{Category: "console", Classification: "repetitive", MatchSpec: NoiseMatchSpec{MessageRegex: "post-reset"}},
	})

	for _, r := range nc.ListRules() {
		if r.MatchSpec.MessageRegex == "post-reset" {
			if r.ID != "user_1" {
				t.Errorf("post-reset user ID = %q, want 'user_1'", r.ID)
			}
			return
		}
	}
	t.Fatal("could not find post-reset rule")
}

// ============================================
// isRuleRegexValid: comprehensive
// ============================================

func TestIsRuleRegexValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		rule  NoiseRule
		valid bool
	}{
		{
			name:  "all_empty",
			rule:  NoiseRule{},
			valid: true,
		},
		{
			name:  "valid_message_regex",
			rule:  NoiseRule{MatchSpec: NoiseMatchSpec{MessageRegex: "foo.*bar"}},
			valid: true,
		},
		{
			name:  "valid_source_regex",
			rule:  NoiseRule{MatchSpec: NoiseMatchSpec{SourceRegex: `chrome-extension://`}},
			valid: true,
		},
		{
			name:  "valid_url_regex",
			rule:  NoiseRule{MatchSpec: NoiseMatchSpec{URLRegex: `\.map(\?|$)`}},
			valid: true,
		},
		{
			name:  "invalid_message_regex",
			rule:  NoiseRule{MatchSpec: NoiseMatchSpec{MessageRegex: "[invalid("}},
			valid: false,
		},
		{
			name:  "invalid_source_regex",
			rule:  NoiseRule{MatchSpec: NoiseMatchSpec{SourceRegex: "[invalid("}},
			valid: false,
		},
		{
			name:  "invalid_url_regex",
			rule:  NoiseRule{MatchSpec: NoiseMatchSpec{URLRegex: "[invalid("}},
			valid: false,
		},
		{
			name: "mixed_valid_and_empty",
			rule: NoiseRule{MatchSpec: NoiseMatchSpec{
				MessageRegex: "valid",
				SourceRegex:  "",
				URLRegex:     "also-valid",
			}},
			valid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isRuleRegexValid(tt.rule); got != tt.valid {
				t.Errorf("isRuleRegexValid() = %v, want %v", got, tt.valid)
			}
		})
	}
}

// ============================================
// Built-in rules: all have required fields
// ============================================

func TestBuiltinRules_AllHaveRequiredFields(t *testing.T) {
	t.Parallel()

	rules := builtinRules()
	for _, r := range rules {
		if r.ID == "" {
			t.Error("built-in rule missing ID")
		}
		if !strings.HasPrefix(r.ID, "builtin_") {
			t.Errorf("built-in rule ID %q does not start with 'builtin_'", r.ID)
		}
		if r.Category == "" {
			t.Errorf("built-in rule %s missing Category", r.ID)
		}
		if r.Classification == "" {
			t.Errorf("built-in rule %s missing Classification", r.ID)
		}
		if r.CreatedAt.IsZero() {
			t.Errorf("built-in rule %s has zero CreatedAt", r.ID)
		}
		// At least one match spec field should be set
		spec := r.MatchSpec
		if spec.MessageRegex == "" && spec.SourceRegex == "" && spec.URLRegex == "" && spec.Method == "" && spec.StatusMin == 0 {
			t.Errorf("built-in rule %s has no match spec fields set", r.ID)
		}
	}
}

// ============================================
// Built-in rules: unique IDs
// ============================================

func TestBuiltinRules_UniqueIDs(t *testing.T) {
	t.Parallel()

	rules := builtinRules()
	seen := make(map[string]bool)
	for _, r := range rules {
		if seen[r.ID] {
			t.Errorf("duplicate built-in rule ID: %s", r.ID)
		}
		seen[r.ID] = true
	}
}

// ============================================
// Built-in rules: all categories are valid
// ============================================

func TestBuiltinRules_ValidCategories(t *testing.T) {
	t.Parallel()

	validCategories := map[string]bool{
		"console":   true,
		"network":   true,
		"websocket": true,
	}

	rules := builtinRules()
	for _, r := range rules {
		if !validCategories[r.Category] {
			t.Errorf("built-in rule %s has invalid category %q", r.ID, r.Category)
		}
	}
}

// ============================================
// Built-in rules: all regexes compile
// ============================================

func TestBuiltinRules_AllRegexesCompile(t *testing.T) {
	t.Parallel()

	rules := builtinRules()
	for _, r := range rules {
		if !isRuleRegexValid(r) {
			t.Errorf("built-in rule %s has invalid regex", r.ID)
		}
	}
}
