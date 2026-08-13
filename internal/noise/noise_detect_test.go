// noise_detect_test.go — Tests automatic noise-pattern detection.
// Docs: docs/features/feature/noise-filtering/index.md

package noise

import (
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

// ============================================
// Auto-detect: empty inputs
// ============================================

func TestAutoDetect_AllEmptyInputs(t *testing.T) {
	t.Parallel()
	nc := NewNoiseConfig()

	proposals := nc.AutoDetect(nil, nil, nil)
	if len(proposals) != 0 {
		t.Fatalf("expected 0 proposals for nil inputs, got %d", len(proposals))
	}

	proposals = nc.AutoDetect([]types.LogEntry{}, []types.NetworkBody{}, []types.WebSocketEvent{})
	if len(proposals) != 0 {
		t.Fatalf("expected 0 proposals for empty slices, got %d", len(proposals))
	}
}

// ============================================
// detectRepetitiveMessages: below threshold
// ============================================

func TestAutoDetect_RepetitiveMessagesBelowThreshold(t *testing.T) {
	requireDetectorDiscriminates(t)
	t.Parallel()
	nc := NewNoiseConfig()

	// 9 identical messages (threshold is 10)
	entries := make([]types.LogEntry, 9)
	for i := range entries {
		entries[i] = types.LogEntry{
			"level":   "info",
			"message": "almost frequent enough",
			"source":  "http://localhost:3000/app.js",
		}
	}

	proposals := nc.AutoDetect(entries, nil, nil)
	for _, p := range proposals {
		if strings.Contains(p.Rule.MatchSpec.MessageRegex, "almost frequent enough") {
			t.Fatal("should not propose a rule for messages below threshold (< 10)")
		}
	}
}

// ============================================
// detectRepetitiveMessages: exact threshold
// ============================================

func TestAutoDetect_RepetitiveMessagesExactThreshold(t *testing.T) {
	t.Parallel()
	nc := NewNoiseConfig()

	// Exactly 10 identical messages
	entries := make([]types.LogEntry, 10)
	for i := range entries {
		entries[i] = types.LogEntry{
			"level":   "info",
			"message": "exactly at threshold",
			"source":  "http://localhost:3000/app.js",
		}
	}

	proposals := nc.AutoDetect(entries, nil, nil)
	found := false
	for _, p := range proposals {
		if p.Rule.Category == "console" && p.Rule.Classification == "repetitive" {
			found = true
			if p.Confidence < 0.7 || p.Confidence > 0.99 {
				t.Errorf("confidence = %f, want between 0.7 and 0.99", p.Confidence)
			}
			if !p.Rule.AutoDetected {
				t.Error("AutoDetected should be true")
			}
			// Confidence for 10 messages: 0.7 + 10/100 = 0.80
			expected := 0.8
			if p.Confidence < expected-0.01 || p.Confidence > expected+0.01 {
				t.Errorf("confidence = %f, want ~%f for 10 messages", p.Confidence, expected)
			}
		}
	}
	if !found {
		t.Fatal("expected a repetitive console proposal for 10 identical messages")
	}
}

// ============================================
// detectRepetitiveMessages: confidence capping at 0.99
// ============================================

func TestAutoDetect_ConfidenceCappedAt099(t *testing.T) {
	requireDetectorDiscriminates(t)
	t.Parallel()
	nc := NewNoiseConfig()

	// 100 identical messages: 0.7 + 100/100 = 1.7 -> capped at 0.99
	entries := make([]types.LogEntry, 100)
	for i := range entries {
		entries[i] = types.LogEntry{
			"level":   "info",
			"message": "ultra frequent message that will be capped",
			"source":  "http://localhost:3000/app.js",
		}
	}

	proposals := nc.AutoDetect(entries, nil, nil)
	for _, p := range proposals {
		if p.Confidence > 0.99 {
			t.Errorf("confidence = %f, should be capped at 0.99", p.Confidence)
		}
	}
}

// ============================================
// detectRepetitiveMessages: empty message strings ignored
// ============================================

func TestAutoDetect_EmptyMessageStringsIgnored(t *testing.T) {
	requireDetectorDiscriminates(t)
	t.Parallel()
	nc := NewNoiseConfig()

	entries := make([]types.LogEntry, 20)
	for i := range entries {
		entries[i] = types.LogEntry{
			"level":  "info",
			"source": "http://localhost:3000/app.js",
			// no "message" key
		}
	}

	proposals := nc.AutoDetect(entries, nil, nil)
	for _, p := range proposals {
		if p.Rule.MatchSpec.MessageRegex == "" {
			t.Error("should not propose a rule with empty message regex")
		}
	}
}

// ============================================
// detectNodeModuleSources: threshold check (< 2)
// ============================================

func TestAutoDetect_NodeModulesSourceBelowThreshold(t *testing.T) {
	requireDetectorDiscriminates(t)
	t.Parallel()
	nc := NewNoiseConfig()

	// Only 1 entry from node_modules (threshold is 2)
	entries := []types.LogEntry{
		{
			"level":   "warn",
			"message": "single warning",
			"source":  "http://localhost:3000/node_modules/lib/index.js",
		},
	}

	proposals := nc.AutoDetect(entries, nil, nil)
	for _, p := range proposals {
		if strings.Contains(p.Rule.MatchSpec.SourceRegex, "node_modules/lib") {
			t.Fatal("should not propose for node_modules source with only 1 entry (threshold is 2)")
		}
	}
}

// ============================================
// detectNodeModuleSources: non-node_modules source ignored
// ============================================

func TestAutoDetect_NonNodeModulesSourceIgnored(t *testing.T) {
	requireDetectorDiscriminates(t)
	t.Parallel()
	nc := NewNoiseConfig()

	entries := make([]types.LogEntry, 10)
	for i := range entries {
		entries[i] = types.LogEntry{
			"level":   "warn",
			"message": "app warning",
			"source":  "http://localhost:3000/src/app.js",
		}
	}

	proposals := nc.AutoDetect(entries, nil, nil)
	for _, p := range proposals {
		if strings.Contains(p.Rule.MatchSpec.SourceRegex, "src/app.js") {
			t.Fatal("should not propose source rules for non-node_modules paths")
		}
	}
}

// ============================================
// detectNodeModuleSources: empty source field
// ============================================

func TestAutoDetect_EmptySourceFieldIgnored(t *testing.T) {
	requireDetectorDiscriminates(t)
	t.Parallel()
	nc := NewNoiseConfig()

	entries := make([]types.LogEntry, 10)
	for i := range entries {
		entries[i] = types.LogEntry{
			"level":   "warn",
			"message": "warning without source",
			// no "source" key
		}
	}

	proposals := nc.AutoDetect(entries, nil, nil)
	for _, p := range proposals {
		if p.Rule.MatchSpec.SourceRegex != "" && strings.Contains(p.Rule.MatchSpec.SourceRegex, "node_modules") {
			t.Fatal("should not propose source rules when source is empty")
		}
	}
}

// ============================================
// detectInfrastructureURLs: below threshold (< 20)
// ============================================

func TestAutoDetect_InfraURLsBelowThreshold(t *testing.T) {
	requireDetectorDiscriminates(t)
	t.Parallel()
	nc := NewNoiseConfig()

	// 19 requests (threshold is 20)
	bodies := make([]types.NetworkBody, 19)
	for i := range bodies {
		bodies[i] = types.NetworkBody{
			URL:    "http://localhost:3000/health",
			Method: "GET",
			Status: 200,
		}
	}

	proposals := nc.AutoDetect(nil, bodies, nil)
	for _, p := range proposals {
		if strings.Contains(p.Rule.MatchSpec.URLRegex, "health") {
			t.Fatal("should not propose for infrastructure URL below threshold (< 20)")
		}
	}
}

// ============================================
// detectInfrastructureURLs: non-infrastructure path ignored
// ============================================

func TestAutoDetect_NonInfraPathIgnored(t *testing.T) {
	requireDetectorDiscriminates(t)
	t.Parallel()
	nc := NewNoiseConfig()

	// 25 requests to a non-infrastructure path
	bodies := make([]types.NetworkBody, 25)
	for i := range bodies {
		bodies[i] = types.NetworkBody{
			URL:    "http://localhost:3000/api/users",
			Method: "GET",
			Status: 200,
		}
	}

	proposals := nc.AutoDetect(nil, bodies, nil)
	for _, p := range proposals {
		if p.Rule.Category == "network" && strings.Contains(p.Rule.MatchSpec.URLRegex, "api/users") {
			t.Fatal("should not propose for non-infrastructure network paths")
		}
	}
}

// ============================================
// isInfrastructurePath: various patterns
// ============================================

func TestIsInfrastructurePath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		path     string
		expected bool
	}{
		{"/health", true},
		{"/api/health", true},
		{"/ping", true},
		{"/ready", true},
		{"/__debug", true},
		{"/sockjs-node/info", true},
		{"/ws", true},
		{"/api/users", false},
		{"/products/list", false},
		{"/", false},
		{"", false},
	}

	for _, tt := range tests {
		if got := isInfrastructurePath(tt.path); got != tt.expected {
			t.Errorf("isInfrastructurePath(%q) = %v, want %v", tt.path, got, tt.expected)
		}
	}
}

// ============================================
// autoApplyHighConfidence: confidence below 0.9 not applied
// ============================================

func TestAutoDetect_LowConfidenceNotAutoApplied(t *testing.T) {
	t.Parallel()
	nc := NewNoiseConfig()

	// 10 messages -> confidence = 0.7 + 10/100 = 0.80 (< 0.9)
	entries := make([]types.LogEntry, 10)
	for i := range entries {
		entries[i] = types.LogEntry{
			"level":   "info",
			"message": "low confidence test message",
			"source":  "http://localhost:3000/app.js",
		}
	}

	rulesBefore := len(nc.ListRules())
	nc.AutoDetect(entries, nil, nil)
	rulesAfter := len(nc.ListRules())

	if rulesAfter > rulesBefore {
		t.Errorf("low confidence proposals should not be auto-applied, rules before=%d after=%d", rulesBefore, rulesAfter)
	}
}

// ============================================
// autoApplyHighConfidence: at max rules, not applied
// ============================================

func TestAutoDetect_AtMaxRulesNotAutoApplied(t *testing.T) {
	t.Parallel()
	nc := NewNoiseConfig()

	// Fill to max rules
	builtinCount := len(nc.ListRules())
	remaining := maxNoiseRules - builtinCount
	fillRules := make([]NoiseRule, remaining)
	for i := range fillRules {
		fillRules[i] = NoiseRule{
			Category:       "console",
			Classification: "repetitive",
			MatchSpec: NoiseMatchSpec{
				MessageRegex: "fill_" + strings.Repeat("x", i%10),
			},
		}
	}
	_ = nc.AddRules(fillRules)

	// Now at max. High confidence proposals should not be added.
	entries := make([]types.LogEntry, 50)
	for i := range entries {
		entries[i] = types.LogEntry{
			"level":   "info",
			"message": "max rules exceeded test",
			"source":  "http://localhost:3000/app.js",
		}
	}

	rulesBefore := len(nc.ListRules())
	nc.AutoDetect(entries, nil, nil)
	rulesAfter := len(nc.ListRules())

	if rulesAfter > maxNoiseRules {
		t.Errorf("rules should not exceed maxNoiseRules(%d), got %d", maxNoiseRules, rulesAfter)
	}
	if rulesAfter > rulesBefore {
		t.Errorf("should not add rules at max capacity, before=%d after=%d", rulesBefore, rulesAfter)
	}
}

// ============================================
// isConsoleCoveredLocked: source coverage for message
// ============================================

func TestAutoDetect_SourceCoverageForMessage(t *testing.T) {
	t.Parallel()
	nc := NewNoiseConfig()

	// The built-in chrome extension rule covers sources with chrome-extension://
	// Create 15+ entries with a unique message but coming from chrome-extension source
	entries := make([]types.LogEntry, 15)
	for i := range entries {
		entries[i] = types.LogEntry{
			"level":   "warn",
			"message": "unique message from covered source 12345",
			"source":  "chrome-extension://abcdef/content.js",
		}
	}

	proposals := nc.AutoDetect(entries, nil, nil)

	// The source is covered by builtin_chrome_extension, so no proposal should be made
	for _, p := range proposals {
		if strings.Contains(p.Rule.MatchSpec.MessageRegex, "unique message from covered source") {
			t.Error("should not propose for messages whose source is already covered")
		}
	}
}

// ============================================
// detectInfrastructureURLs: all infrastructure patterns
// ============================================

func TestAutoDetect_AllInfraPatterns(t *testing.T) {
	t.Parallel()

	// Only test paths NOT already covered by built-in rules.
	// /sockjs-node and /ws are covered by builtin_hmr_network and builtin_ws_hmr.
	// /__internal patterns may be covered by builtin_next_internal (/_next/).
	infraPaths := []string{"/health", "/ping", "/ready"}

	for _, path := range infraPaths {
		nc := NewNoiseConfig()

		bodies := make([]types.NetworkBody, 25)
		for i := range bodies {
			bodies[i] = types.NetworkBody{
				URL:    "http://localhost:3000" + path,
				Method: "GET",
				Status: 200,
			}
		}

		proposals := nc.AutoDetect(nil, bodies, nil)
		found := false
		for _, p := range proposals {
			if p.Rule.Category == "network" && p.Rule.Classification == "infrastructure" {
				found = true
				if p.Confidence != 0.8 {
					t.Errorf("infrastructure confidence for %s = %f, want 0.8", path, p.Confidence)
				}
				if !p.Rule.AutoDetected {
					t.Errorf("AutoDetected should be true for %s", path)
				}
			}
		}
		if !found {
			t.Errorf("expected infrastructure proposal for path %s", path)
		}
	}
}

// ============================================
// detectNodeModuleSources: confidence is always 0.75
// ============================================

func TestAutoDetect_NodeModulesConfidence(t *testing.T) {
	t.Parallel()
	nc := NewNoiseConfig()

	entries := make([]types.LogEntry, 5)
	for i := range entries {
		entries[i] = types.LogEntry{
			"level":   "warn",
			"message": "lib warning " + string(rune('A'+i)),
			"source":  "http://localhost:3000/node_modules/unique-test-lib/dist/index.js",
		}
	}

	proposals := nc.AutoDetect(entries, nil, nil)
	for _, p := range proposals {
		if p.Rule.Classification == "extension" && p.Rule.AutoDetected {
			if p.Confidence != 0.75 {
				t.Errorf("node_modules source confidence = %f, want 0.75", p.Confidence)
			}
			if p.Rule.Category != "console" {
				t.Errorf("node_modules rule category = %q, want console", p.Rule.Category)
			}
		}
	}
}

// ============================================
// Auto-detect proposal reason strings
// ============================================

func TestAutoDetect_ReasonStrings(t *testing.T) {
	t.Parallel()
	nc := NewNoiseConfig()

	// Repetitive messages
	entries := make([]types.LogEntry, 15)
	for i := range entries {
		entries[i] = types.LogEntry{
			"level":   "info",
			"message": "reason test repetitive msg",
			"source":  "http://localhost:3000/app.js",
		}
	}

	proposals := nc.AutoDetect(entries, nil, nil)
	for _, p := range proposals {
		if p.Rule.Classification == "repetitive" {
			if !strings.Contains(p.Reason, "message repeated 15 times") {
				t.Errorf("reason = %q, want to contain 'message repeated 15 times'", p.Reason)
			}
		}
	}
}

// ============================================
// Auto-detect with node_modules source reason
// ============================================

func TestAutoDetect_NodeModulesReasonString(t *testing.T) {
	t.Parallel()
	nc := NewNoiseConfig()

	entries := make([]types.LogEntry, 3)
	for i := range entries {
		entries[i] = types.LogEntry{
			"level":   "warn",
			"message": "node_modules reason test " + string(rune('A'+i)),
			"source":  "http://localhost:3000/node_modules/reason-lib/index.js",
		}
	}

	proposals := nc.AutoDetect(entries, nil, nil)
	for _, p := range proposals {
		if p.Rule.Classification == "extension" && p.Rule.AutoDetected {
			if !strings.Contains(p.Reason, "node_modules source with 3 entries") {
				t.Errorf("reason = %q, want to contain 'node_modules source with 3 entries'", p.Reason)
			}
		}
	}
}

// ============================================
// Auto-detect with network infrastructure reason
// ============================================

func TestAutoDetect_InfraReasonString(t *testing.T) {
	t.Parallel()
	nc := NewNoiseConfig()

	bodies := make([]types.NetworkBody, 30)
	for i := range bodies {
		bodies[i] = types.NetworkBody{
			URL:    "http://localhost:3000/ping",
			Method: "GET",
			Status: 200,
		}
	}

	proposals := nc.AutoDetect(nil, bodies, nil)
	for _, p := range proposals {
		if p.Rule.Category == "network" && p.Rule.Classification == "infrastructure" {
			if !strings.Contains(p.Reason, "infrastructure path hit 30 times") {
				t.Errorf("reason = %q, want to contain 'infrastructure path hit 30 times'", p.Reason)
			}
		}
	}
}

// ============================================
// Auto-detect: network bodies with empty URL path
// ============================================

func TestAutoDetect_EmptyURLPathIgnored(t *testing.T) {
	requireDetectorDiscriminates(t)
	t.Parallel()
	nc := NewNoiseConfig()

	bodies := make([]types.NetworkBody, 25)
	for i := range bodies {
		bodies[i] = types.NetworkBody{
			URL:    "", // empty URL
			Method: "GET",
			Status: 200,
		}
	}

	proposals := nc.AutoDetect(nil, bodies, nil)
	for _, p := range proposals {
		if p.Rule.Category == "network" {
			t.Errorf("should not propose rules for empty URL paths, got: %+v", p)
		}
	}
}

// ============================================
// Test Scenario 15: Auto-detect with 15 identical messages -> proposed rule with confidence > 0.7
// ============================================

func TestNoiseAutoDetectFrequency(t *testing.T) {
	t.Parallel()
	nc := NewNoiseConfig()

	// Create entries with 15 identical messages
	var entries []types.LogEntry
	for i := 0; i < 15; i++ {
		entries = append(entries, types.LogEntry{
			"level":   "info",
			"message": "Repeated polling message from my app",
			"source":  "http://localhost:3000/app.js",
		})
	}

	proposals := nc.AutoDetect(entries, nil, nil)

	// Should have at least one proposal
	if len(proposals) == 0 {
		t.Fatal("expected at least one auto-detected proposal")
	}

	// Check confidence > 0.7
	found := false
	for _, p := range proposals {
		if p.Confidence > 0.7 {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected at least one proposal with confidence > 0.7")
	}
}

// ============================================
// Test Scenario 16: Auto-detect doesn't duplicate existing rules
// ============================================

func TestNoiseAutoDetectNoDuplicates(t *testing.T) {
	requireDetectorDiscriminates(t)
	t.Parallel()
	nc := NewNoiseConfig()

	// The built-in chrome extension rule already covers this
	var entries []types.LogEntry
	for i := 0; i < 20; i++ {
		entries = append(entries, types.LogEntry{
			"level":   "warn",
			"message": "Extension warning",
			"source":  "chrome-extension://abc123/script.js",
		})
	}

	proposals := nc.AutoDetect(entries, nil, nil)

	// Should NOT propose a rule that duplicates the chrome extension built-in
	for _, p := range proposals {
		if p.Rule.MatchSpec.SourceRegex != "" && p.Rule.MatchSpec.SourceRegex == "chrome-extension://" {
			t.Error("auto-detect should not duplicate existing rules")
		}
	}
}

// ============================================
// Test Scenario 17: High-confidence auto-detected rules are automatically applied
// ============================================

func TestNoiseAutoDetectHighConfidenceApplied(t *testing.T) {
	t.Parallel()
	nc := NewNoiseConfig()

	// Create 50 identical messages (should yield high confidence: 0.7 + 50/100 = 1.2, capped at 0.99)
	var entries []types.LogEntry
	for i := 0; i < 50; i++ {
		entries = append(entries, types.LogEntry{
			"level":   "info",
			"message": "Unique auto-apply test message xyz",
			"source":  "http://localhost:3000/worker.js",
		})
	}

	beforeCount := len(nc.ListRules())
	proposals := nc.AutoDetect(entries, nil, nil)

	// High confidence proposals (>= 0.9) should be auto-applied
	highConfCount := 0
	for _, p := range proposals {
		if p.Confidence >= 0.9 {
			highConfCount++
		}
	}

	afterCount := len(nc.ListRules())
	if highConfCount > 0 && afterCount <= beforeCount {
		t.Error("high-confidence proposals should be automatically applied as rules")
	}
}

func TestNoiseAutoDetectNetworkFrequency(t *testing.T) {
	t.Parallel()
	nc := NewNoiseConfig()

	// Create 25 network requests to /health endpoint
	var bodies []types.NetworkBody
	for i := 0; i < 25; i++ {
		bodies = append(bodies, types.NetworkBody{
			Method: "GET",
			URL:    "http://localhost:3000/health",
			Status: 200,
		})
	}

	proposals := nc.AutoDetect(nil, bodies, nil)

	// Should propose a rule for /health
	found := false
	for _, p := range proposals {
		if p.Rule.Category == "network" && p.Confidence >= 0.8 {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected network frequency proposal for /health endpoint with confidence >= 0.8")
	}
}

func TestNoiseAutoDetectSourceAnalysis(t *testing.T) {
	t.Parallel()
	nc := NewNoiseConfig()

	// Create entries from node_modules
	var entries []types.LogEntry
	for i := 0; i < 5; i++ {
		entries = append(entries, types.LogEntry{
			"level":   "warn",
			"message": "Some lib warning " + string(rune('A'+i)),
			"source":  "http://localhost:3000/node_modules/some-lib/index.js",
		})
	}

	proposals := nc.AutoDetect(entries, nil, nil)

	// Should detect node_modules source
	found := false
	for _, p := range proposals {
		if p.Rule.Category == "console" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected auto-detect to flag node_modules source entries")
	}
}

// requireDetectorDiscriminates is the positive half of an "is ignored" or
// "below threshold" assertion.
//
// Those tests assert only that no proposal of some shape came back, which a
// detector returning nothing at all satisfies. Verified: with AutoDetect
// stubbed to return nil, ten of them still passed while only five failed. This
// runs the same detector over input it must react to, so a case that expects
// silence proves the detector was listening.
func requireDetectorDiscriminates(t *testing.T) {
	t.Helper()
	nc := NewNoiseConfig()
	entries := make([]types.LogEntry, 5)
	for i := range entries {
		entries[i] = types.LogEntry{
			"level":   "warn",
			"message": "discriminator warning " + string(rune('A'+i)),
			"source":  "http://localhost:3000/node_modules/discriminator-lib/dist/index.js",
		}
	}
	if len(nc.AutoDetect(entries, nil, nil)) == 0 {
		t.Fatal("the detector produced no proposal for input it must react to, so an " +
			"'ignored' result proves nothing about the rule under test")
	}
}
