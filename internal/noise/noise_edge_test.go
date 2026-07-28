// Purpose: Edge-case tests for noise filtering boundary conditions.
// Docs: docs/features/feature/noise-filtering/index.md

// ai_noise_edge_test.go — Edge case and gap-coverage tests for noise filtering.
package noise

import (
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

// ============================================
// DismissNoise: max rules reached
// ============================================

func TestDismissNoise_AtMaxRulesNoOp(t *testing.T) {
	t.Parallel()
	nc := NewNoiseConfig()

	// Fill to maxNoiseRules
	builtinCount := len(nc.ListRules())
	remaining := maxNoiseRules - builtinCount
	fillRules := make([]NoiseRule, remaining)
	for i := range fillRules {
		fillRules[i] = NoiseRule{
			Category:       "console",
			Classification: "repetitive",
			MatchSpec: NoiseMatchSpec{
				MessageRegex: "fill_rule",
			},
		}
	}
	_ = nc.AddRules(fillRules)

	if len(nc.ListRules()) != maxNoiseRules {
		t.Fatalf("expected exactly %d rules, got %d", maxNoiseRules, len(nc.ListRules()))
	}

	// DismissNoise at max should silently return without adding
	nc.DismissNoise("should-not-be-added", "", "at capacity")

	rules := nc.ListRules()
	for _, r := range rules {
		if r.MatchSpec.MessageRegex == "should-not-be-added" {
			t.Fatal("DismissNoise should not add rule when at max capacity")
		}
	}
	if len(rules) != maxNoiseRules {
		t.Fatalf("expected %d rules after dismiss at capacity, got %d", maxNoiseRules, len(rules))
	}
}

// ============================================
// RemoveRule: non-existent rule
// ============================================

func TestRemoveRule_NonExistentRule(t *testing.T) {
	t.Parallel()
	nc := NewNoiseConfig()

	err := nc.RemoveRule("user_9999")
	if err == nil {
		t.Fatal("RemoveRule should return error for non-existent rule")
	}
	if !strings.Contains(err.Error(), "rule not found") {
		t.Errorf("error = %q, want to contain 'rule not found'", err.Error())
	}
}

// ============================================
// RemoveRule: builtin rule error message
// ============================================

func TestRemoveRule_BuiltinRuleErrorMessage(t *testing.T) {
	t.Parallel()
	nc := NewNoiseConfig()

	err := nc.RemoveRule("builtin_favicon")
	if err == nil {
		t.Fatal("RemoveRule should return error for built-in rule")
	}
	if !strings.Contains(err.Error(), "cannot remove built-in rule") {
		t.Errorf("error = %q, want to contain 'cannot remove built-in rule'", err.Error())
	}
	if !strings.Contains(err.Error(), "builtin_favicon") {
		t.Errorf("error = %q, want to contain 'builtin_favicon'", err.Error())
	}
}

// ============================================
// AddRules: validates all patterns in batch before adding any
// ============================================

func TestAddRules_BatchValidationRejectsAll(t *testing.T) {
	t.Parallel()
	nc := NewNoiseConfig()

	rulesBefore := len(nc.ListRules())

	// First rule is valid, second is unsafe
	err := nc.AddRules([]NoiseRule{
		{
			Category:       "console",
			Classification: "repetitive",
			MatchSpec:      NoiseMatchSpec{MessageRegex: "valid"},
		},
		{
			Category:       "console",
			Classification: "repetitive",
			MatchSpec:      NoiseMatchSpec{MessageRegex: "(a+)+"},
		},
	})
	if err == nil {
		t.Fatal("AddRules should reject the entire batch if any pattern is unsafe")
	}

	rulesAfter := len(nc.ListRules())
	if rulesAfter != rulesBefore {
		t.Errorf("no rules should be added on batch validation failure, before=%d after=%d", rulesBefore, rulesAfter)
	}
}

// ============================================
// AddRules: unsafe SourceRegex
// ============================================

func TestAddRules_UnsafeSourceRegex(t *testing.T) {
	t.Parallel()
	nc := NewNoiseConfig()

	err := nc.AddRules([]NoiseRule{
		{
			Category:       "console",
			Classification: "repetitive",
			MatchSpec: NoiseMatchSpec{
				SourceRegex: "(b*)+",
			},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "nested quantifiers") {
		t.Fatalf("expected nested-quantifier error for SourceRegex, got %v", err)
	}
}

// ============================================
// AddRules: unsafe URLRegex
// ============================================

func TestAddRules_UnsafeURLRegex(t *testing.T) {
	t.Parallel()
	nc := NewNoiseConfig()

	err := nc.AddRules([]NoiseRule{
		{
			Category:       "network",
			Classification: "infrastructure",
			MatchSpec: NoiseMatchSpec{
				URLRegex: "(c?)+",
			},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "nested quantifiers") {
		t.Fatalf("expected nested-quantifier error for URLRegex, got %v", err)
	}
}

// ============================================
// validateRegexPattern: all nested quantifier patterns
// ============================================

func TestValidateRegexPattern_NestedQuantifiers(t *testing.T) {
	t.Parallel()

	unsafe := []string{
		"(a+)+",
		"(b*)+",
		"(c?)+",
		"(d{1,2})+",
		"(a+)*",
		"(b*)*",
		"(c?)*",
		"(d{1,2})*",
		"(a+)?",
		"(b*)?",
	}

	for _, p := range unsafe {
		if err := validateRegexPattern(p); err == nil {
			t.Errorf("validateRegexPattern(%q) should reject nested quantifiers", p)
		}
	}

	safe := []string{
		"a+b",
		"foo.*bar",
		`\d{3}-\d{4}`,
		"(abc)+",
		"[a-z]+",
	}

	for _, p := range safe {
		if err := validateRegexPattern(p); err != nil {
			t.Errorf("validateRegexPattern(%q) should accept safe pattern, got %v", p, err)
		}
	}
}

// ============================================
// validateRegexPattern: max length
// ============================================

func TestValidateRegexPattern_MaxLength(t *testing.T) {
	t.Parallel()

	// Exactly 512 characters should pass
	pattern512 := strings.Repeat("a", 512)
	if err := validateRegexPattern(pattern512); err != nil {
		t.Errorf("512-char pattern should be accepted, got %v", err)
	}

	// 513 characters should fail
	pattern513 := strings.Repeat("a", 513)
	err := validateRegexPattern(pattern513)
	if err == nil {
		t.Fatal("513-char pattern should be rejected")
	}
	if !strings.Contains(err.Error(), "maximum length") {
		t.Errorf("error = %q, want to contain 'maximum length'", err.Error())
	}
}

// ============================================
// validateRegexPattern: empty pattern is safe
// ============================================

func TestValidateRegexPattern_EmptyPattern(t *testing.T) {
	t.Parallel()

	if err := validateRegexPattern(""); err != nil {
		t.Errorf("empty pattern should be safe, got %v", err)
	}
}

// ============================================
// IsNetworkNoise: method-only rule (no URL regex)
// ============================================

func TestIsNetworkNoise_MethodOnlyRule(t *testing.T) {
	t.Parallel()
	nc := NewNoiseConfig()

	rule := NoiseRule{
		Category:       "network",
		Classification: "infrastructure",
		MatchSpec: NoiseMatchSpec{
			Method: "HEAD",
		},
	}
	_ = nc.AddRules([]NoiseRule{rule})

	body := types.NetworkBody{
		Method: "HEAD",
		URL:    "http://localhost:3000/any/path",
		Status: 200,
	}
	if !nc.IsNetworkNoise(body) {
		t.Error("HEAD request matching method-only rule should be noise")
	}

	bodyGet := types.NetworkBody{
		Method: "GET",
		URL:    "http://localhost:3000/any/path",
		Status: 200,
	}
	if nc.IsNetworkNoise(bodyGet) {
		t.Error("GET request should not match HEAD-only rule")
	}
}

// ============================================
// IsNetworkNoise: status-range-only rule (no URL, no method)
// ============================================

func TestIsNetworkNoise_StatusRangeOnlyRule(t *testing.T) {
	t.Parallel()
	nc := NewNoiseConfig()

	rule := NoiseRule{
		Category:       "network",
		Classification: "cosmetic",
		MatchSpec: NoiseMatchSpec{
			StatusMin: 300,
			StatusMax: 399,
		},
	}
	_ = nc.AddRules([]NoiseRule{rule})

	body301 := types.NetworkBody{
		Method: "GET",
		URL:    "http://localhost:3000/redirect",
		Status: 301,
	}
	if !nc.IsNetworkNoise(body301) {
		t.Error("301 should match status-range-only rule")
	}

	body200 := types.NetworkBody{
		Method: "GET",
		URL:    "http://localhost:3000/ok",
		Status: 200,
	}
	if nc.IsNetworkNoise(body200) {
		t.Error("200 should not match 300-399 status range rule")
	}
}

// ============================================
// IsConsoleNoise: nil/missing fields in types.LogEntry
// ============================================

func TestIsConsoleNoise_NilFields(t *testing.T) {
	t.Parallel()
	nc := NewNoiseConfig()

	// Empty entry
	if nc.IsConsoleNoise(types.LogEntry{}) {
		t.Error("empty types.LogEntry should not be noise")
	}

	// Only level field
	if nc.IsConsoleNoise(types.LogEntry{"level": "error"}) {
		t.Error("types.LogEntry with only level should not be noise")
	}

	// Non-string type assertions
	if nc.IsConsoleNoise(types.LogEntry{"message": 123, "source": true, "level": nil}) {
		t.Error("types.LogEntry with non-string fields should not be noise")
	}
}

// ============================================
// IsWebSocketNoise: builtin websocket rules
// ============================================

func TestIsWebSocketNoise_BuiltinRules(t *testing.T) {
	t.Parallel()
	nc := NewNoiseConfig()

	noiseURLs := []string{
		"ws://localhost:3000/__vite_hmr",
		"ws://localhost:5173/ws?token=abc",
		"ws://localhost:3000/_next/webpack-hmr",
		"ws://localhost:3000/sockjs-node/websocket",
		"wss://devtools/remote/debugging",
		"ws://localhost/__browser_inspector/ws",
	}

	for _, url := range noiseURLs {
		event := types.WebSocketEvent{URL: url, Event: "message"}
		if !nc.IsWebSocketNoise(event) {
			t.Errorf("expected builtin WS noise for URL: %s", url)
		}
	}

	// Normal WebSocket should not match
	event := types.WebSocketEvent{URL: "wss://api.example.com/live", Event: "message"}
	if nc.IsWebSocketNoise(event) {
		t.Error("normal WebSocket URL should not be noise")
	}
}

// ============================================
// GetStatistics: signal timestamp updates
// ============================================

func TestGetStatistics_SignalTimestamp(t *testing.T) {
	t.Parallel()
	nc := NewNoiseConfig()

	// A non-matching entry should update LastSignalAt
	entry := types.LogEntry{
		"level":   "error",
		"message": "real application error",
		"source":  "http://localhost:3000/app.js",
	}
	nc.IsConsoleNoise(entry)

	stats := nc.GetStatistics()
	if stats.LastSignalAt.IsZero() {
		t.Error("LastSignalAt should be updated after non-noise entry")
	}
	if stats.TotalFiltered != 0 {
		t.Errorf("TotalFiltered = %d, want 0 for non-noise entry", stats.TotalFiltered)
	}
}

// ============================================
// GetStatistics: noise timestamp updates
// ============================================

func TestGetStatistics_NoiseTimestamp(t *testing.T) {
	t.Parallel()
	nc := NewNoiseConfig()

	entry := types.LogEntry{
		"level":   "info",
		"message": "[vite] hot updated",
		"source":  "http://localhost:5173/app.js",
	}
	nc.IsConsoleNoise(entry)

	stats := nc.GetStatistics()
	if stats.LastNoiseAt.IsZero() {
		t.Error("LastNoiseAt should be updated after noise entry")
	}
	if stats.TotalFiltered != 1 {
		t.Errorf("TotalFiltered = %d, want 1", stats.TotalFiltered)
	}
}

// ============================================
// GetStatistics: returns a copy (mutations don't affect internal state)
// ============================================

func TestGetStatistics_ReturnsCopy(t *testing.T) {
	t.Parallel()
	nc := NewNoiseConfig()

	entry := types.LogEntry{
		"level":   "info",
		"message": "[vite] update",
		"source":  "http://localhost:3000/app.js",
	}
	nc.IsConsoleNoise(entry)

	stats := nc.GetStatistics()
	stats.TotalFiltered = 999
	stats.PerRule["hacked"] = 42

	fresh := nc.GetStatistics()
	if fresh.TotalFiltered == 999 {
		t.Error("modifying returned stats should not affect internal state")
	}
	if fresh.PerRule["hacked"] == 42 {
		t.Error("modifying returned PerRule map should not affect internal state")
	}
}

// ============================================
// Network noise: signal recorded for non-matching
// ============================================

func TestIsNetworkNoise_SignalRecorded(t *testing.T) {
	t.Parallel()
	nc := NewNoiseConfig()

	body := types.NetworkBody{
		Method: "POST",
		URL:    "http://localhost:3000/api/orders",
		Status: 200,
	}
	nc.IsNetworkNoise(body)

	stats := nc.GetStatistics()
	if stats.LastSignalAt.IsZero() {
		t.Error("LastSignalAt should be updated after non-noise network entry")
	}
}

// ============================================
// WebSocket noise: signal recorded for non-matching
// ============================================

func TestIsWebSocketNoise_SignalRecorded(t *testing.T) {
	t.Parallel()
	nc := NewNoiseConfig()

	event := types.WebSocketEvent{URL: "wss://api.example.com/live", Event: "message"}
	nc.IsWebSocketNoise(event)

	stats := nc.GetStatistics()
	if stats.LastSignalAt.IsZero() {
		t.Error("LastSignalAt should be updated after non-noise WS event")
	}
}

// ============================================
// ListRules: returns a copy
// ============================================

func TestListRules_ReturnsCopy(t *testing.T) {
	t.Parallel()
	nc := NewNoiseConfig()

	rules := nc.ListRules()
	originalLen := len(rules)
	rules = append(rules, NoiseRule{ID: "injected"})

	fresh := nc.ListRules()
	if len(fresh) != originalLen {
		t.Errorf("ListRules mutation should not affect internal rules, got len=%d want=%d", len(fresh), originalLen)
	}
}

// ============================================
// AddRules: user ID counter increments correctly
// ============================================

func TestAddRules_UserIDCounterIncrement(t *testing.T) {
	t.Parallel()
	nc := NewNoiseConfig()

	// Add 3 rules
	rules := []NoiseRule{
		{Category: "console", Classification: "repetitive", MatchSpec: NoiseMatchSpec{MessageRegex: "one"}},
		{Category: "console", Classification: "repetitive", MatchSpec: NoiseMatchSpec{MessageRegex: "two"}},
		{Category: "console", Classification: "repetitive", MatchSpec: NoiseMatchSpec{MessageRegex: "three"}},
	}
	_ = nc.AddRules(rules)

	allRules := nc.ListRules()
	ids := make(map[string]bool)
	for _, r := range allRules {
		if strings.HasPrefix(r.ID, "user_") {
			ids[r.ID] = true
		}
	}

	if !ids["user_1"] || !ids["user_2"] || !ids["user_3"] {
		t.Errorf("expected user_1, user_2, user_3 IDs, got %v", ids)
	}
}

// ============================================
// AddRules: CreatedAt is populated
// ============================================

func TestAddRules_CreatedAtPopulated(t *testing.T) {
	t.Parallel()
	nc := NewNoiseConfig()

	before := time.Now().Add(-time.Second)
	_ = nc.AddRules([]NoiseRule{
		{Category: "console", Classification: "repetitive", MatchSpec: NoiseMatchSpec{MessageRegex: "ts-test"}},
	})
	after := time.Now().Add(time.Second)

	for _, r := range nc.ListRules() {
		if r.MatchSpec.MessageRegex == "ts-test" {
			if r.CreatedAt.Before(before) || r.CreatedAt.After(after) {
				t.Errorf("CreatedAt = %v, should be between %v and %v", r.CreatedAt, before, after)
			}
			return
		}
	}
	t.Fatal("could not find the added rule")
}

// ============================================
// DismissNoise: ID prefix format
// ============================================

func TestDismissNoise_IDPrefix(t *testing.T) {
	t.Parallel()
	nc := NewNoiseConfig()

	nc.DismissNoise("pattern1", "console", "reason1")
	nc.DismissNoise("pattern2", "network", "reason2")
	nc.DismissNoise("pattern3", "websocket", "reason3")

	rules := nc.ListRules()
	dismissCount := 0
	for _, r := range rules {
		if strings.HasPrefix(r.ID, "dismiss_") {
			dismissCount++
			if r.Classification != "dismissed" {
				t.Errorf("dismiss rule %s has classification %q, want 'dismissed'", r.ID, r.Classification)
			}
		}
	}
	if dismissCount != 3 {
		t.Errorf("expected 3 dismiss rules, got %d", dismissCount)
	}
}
