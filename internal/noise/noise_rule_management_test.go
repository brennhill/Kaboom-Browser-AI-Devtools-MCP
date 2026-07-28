// noise_rule_management_test.go — Tests mutable noise rules, filtering, and statistics.
// Docs: docs/features/feature/noise-filtering/index.md

package noise

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

// ============================================
// Test Scenario 10: Adding a custom rule -> new matches are filtered
// ============================================

func TestNoiseAddCustomRule(t *testing.T) {
	t.Parallel()
	nc := NewNoiseConfig()

	entry := types.LogEntry{
		"level":   "info",
		"message": "MyApp: polling for updates",
		"source":  "http://localhost:3000/app.js",
	}

	// Before adding rule, should not be noise
	if nc.IsConsoleNoise(entry) {
		t.Error("entry should not be noise before custom rule added")
	}

	// Add a custom rule
	rule := NoiseRule{
		Category:       "console",
		Classification: "repetitive",
		MatchSpec: NoiseMatchSpec{
			MessageRegex: "MyApp: polling",
		},
	}
	err := nc.AddRules([]NoiseRule{rule})
	if err != nil {
		t.Fatalf("failed to add rule: %v", err)
	}

	// Now it should be noise
	if !nc.IsConsoleNoise(entry) {
		t.Error("entry should be noise after custom rule added")
	}
}

// ============================================
// Test Scenario 11: Removing a custom rule -> entries no longer filtered
// ============================================

func TestNoiseRemoveCustomRule(t *testing.T) {
	t.Parallel()
	nc := NewNoiseConfig()

	rule := NoiseRule{
		Category:       "console",
		Classification: "repetitive",
		MatchSpec: NoiseMatchSpec{
			MessageRegex: "custom pattern to filter",
		},
	}
	err := nc.AddRules([]NoiseRule{rule})
	if err != nil {
		t.Fatalf("failed to add rule: %v", err)
	}

	// Find the added rule ID
	rules := nc.ListRules()
	var addedID string
	for _, r := range rules {
		if r.MatchSpec.MessageRegex == "custom pattern to filter" {
			addedID = r.ID
			break
		}
	}
	if addedID == "" {
		t.Fatal("could not find added rule")
	}

	entry := types.LogEntry{
		"level":   "info",
		"message": "custom pattern to filter here",
		"source":  "http://localhost:3000/app.js",
	}

	if !nc.IsConsoleNoise(entry) {
		t.Error("entry should be noise while rule is active")
	}

	// Remove the rule
	err = nc.RemoveRule(addedID)
	if err != nil {
		t.Fatalf("failed to remove rule: %v", err)
	}

	if nc.IsConsoleNoise(entry) {
		t.Error("entry should NOT be noise after rule is removed")
	}
}

// ============================================
// Test Scenario 12: Cannot remove built-in rules -> they remain after removal attempt
// ============================================

func TestNoiseCannotRemoveBuiltinRules(t *testing.T) {
	t.Parallel()
	nc := NewNoiseConfig()

	err := nc.RemoveRule("builtin_chrome_extension")
	if err == nil {
		t.Error("should return error when trying to remove built-in rule")
	}

	// Verify rule still exists
	rules := nc.ListRules()
	found := false
	for _, r := range rules {
		if r.ID == "builtin_chrome_extension" {
			found = true
			break
		}
	}
	if !found {
		t.Error("built-in rule should still be present after failed removal")
	}
}

// ============================================
// Test Scenario 13: Max rules reached -> additional rules silently dropped
// ============================================

func TestNoiseMaxRulesLimit(t *testing.T) {
	t.Parallel()
	nc := NewNoiseConfig()

	// Get current built-in count
	builtinCount := len(nc.ListRules())

	// Add rules to reach max (100)
	remaining := 100 - builtinCount
	rules := make([]NoiseRule, remaining+5) // Try to add more than allowed
	for i := range rules {
		rules[i] = NoiseRule{
			Category:       "console",
			Classification: "repetitive",
			MatchSpec: NoiseMatchSpec{
				MessageRegex: "pattern_" + string(rune('A'+i%26)),
			},
		}
	}
	_ = nc.AddRules(rules)

	// Total should not exceed 100
	allRules := nc.ListRules()
	if len(allRules) > 100 {
		t.Errorf("expected max 100 rules, got %d", len(allRules))
	}
}

// ============================================
// Test Scenario 14: Reset -> only built-ins remain
// ============================================

func TestNoiseReset(t *testing.T) {
	t.Parallel()
	nc := NewNoiseConfig()

	// Add custom rules
	rule := NoiseRule{
		Category:       "console",
		Classification: "repetitive",
		MatchSpec: NoiseMatchSpec{
			MessageRegex: "custom noise",
		},
	}
	_ = nc.AddRules([]NoiseRule{rule})

	// Reset
	nc.Reset()

	// Only built-ins should remain
	rules := nc.ListRules()
	for _, r := range rules {
		if len(r.ID) < 8 || r.ID[:8] != "builtin_" {
			t.Errorf("non-built-in rule survived reset: %s", r.ID)
		}
	}

	// Verify built-ins are still there
	if len(rules) < 40 {
		t.Errorf("expected at least 40 built-in rules after reset, got %d", len(rules))
	}
}

// ============================================
// Test Scenario 18: dismiss_noise defaults to console category
// ============================================

func TestNoiseDismissDefaultsToConsole(t *testing.T) {
	t.Parallel()
	nc := NewNoiseConfig()

	nc.DismissNoise("some pattern", "", "annoying message")

	rules := nc.ListRules()
	found := false
	for _, r := range rules {
		if len(r.ID) >= 8 && r.ID[:8] == "dismiss_" {
			if r.Category != "console" {
				t.Errorf("dismiss_noise should default to console category, got %s", r.Category)
			}
			if r.Classification != "dismissed" {
				t.Errorf("dismiss_noise should have 'dismissed' classification, got %s", r.Classification)
			}
			found = true
			break
		}
	}
	if !found {
		t.Error("dismiss_noise should create a dismiss_ prefixed rule")
	}
}

// ============================================
// Test Scenario 19: dismiss_noise with network category sets URL pattern
// ============================================

func TestNoiseDismissNetworkCategory(t *testing.T) {
	t.Parallel()
	nc := NewNoiseConfig()

	nc.DismissNoise("/api/health", "network", "health check noise")

	body := types.NetworkBody{
		Method: "GET",
		URL:    "http://localhost:3000/api/health",
		Status: 200,
	}

	if !nc.IsNetworkNoise(body) {
		t.Error("dismissed network pattern should match as noise")
	}
}

// ============================================
// Test Scenario 20: Statistics track filtered counts per rule
// ============================================

func TestNoiseStatistics(t *testing.T) {
	t.Parallel()
	nc := NewNoiseConfig()

	// Trigger a few matches
	entry := types.LogEntry{
		"level":   "info",
		"message": "[vite] hot updated module",
		"source":  "http://localhost:5173/app.js",
	}

	for i := 0; i < 5; i++ {
		nc.IsConsoleNoise(entry)
	}

	stats := nc.GetStatistics()

	// Should have counted matches for the HMR rule
	if stats.TotalFiltered < 5 {
		t.Errorf("expected at least 5 filtered entries, got %d", stats.TotalFiltered)
	}

	// Check per-rule count
	if stats.PerRule["builtin_hmr_console"] < 5 {
		t.Errorf("expected at least 5 matches for builtin_hmr_console, got %d", stats.PerRule["builtin_hmr_console"])
	}
}

// ============================================
// Test Scenario 21: Invalid regex pattern -> rule skipped, no panic
// ============================================

func TestNoiseInvalidRegexNoPanic(t *testing.T) {
	t.Parallel()
	nc := NewNoiseConfig()

	// This should not panic
	rule := NoiseRule{
		Category:       "console",
		Classification: "repetitive",
		MatchSpec: NoiseMatchSpec{
			MessageRegex: "[invalid regex(",
		},
	}
	err := nc.AddRules([]NoiseRule{rule})
	if err != nil {
		t.Fatalf("adding rule with invalid regex should not error, got: %v", err)
	}

	// The rule should exist but never match
	entry := types.LogEntry{
		"level":   "info",
		"message": "[invalid regex(",
		"source":  "http://localhost:3000/app.js",
	}

	// Should not panic and should not match
	if nc.IsConsoleNoise(entry) {
		t.Error("invalid regex should never match")
	}
}

// ============================================
// Test Scenario 22: Concurrent read/write -> no race conditions
// ============================================

func TestNoiseConcurrentAccess(t *testing.T) {
	t.Parallel()
	nc := NewNoiseConfig()

	var wg sync.WaitGroup

	// Concurrent readers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				entry := types.LogEntry{
					"level":   "info",
					"message": "[vite] update",
					"source":  "http://localhost:3000/app.js",
				}
				nc.IsConsoleNoise(entry)

				body := types.NetworkBody{
					Method: "GET",
					URL:    "http://localhost:3000/favicon.ico",
					Status: 404,
				}
				nc.IsNetworkNoise(body)

				wsEvent := types.WebSocketEvent{
					URL: "ws://localhost:3000/ws",
				}
				nc.IsWebSocketNoise(wsEvent)
			}
		}()
	}

	// Concurrent writers
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				rule := NoiseRule{
					Category:       "console",
					Classification: "repetitive",
					MatchSpec: NoiseMatchSpec{
						MessageRegex: "concurrent_test",
					},
				}
				_ = nc.AddRules([]NoiseRule{rule})
			}
		}(i)
	}

	wg.Wait()
	// If we get here without a race condition panic, the test passes
}

func TestNoiseRulePrefix(t *testing.T) {
	t.Parallel()
	nc := NewNoiseConfig()

	// User rule should get "user_" prefix
	rule := NoiseRule{
		Category:       "console",
		Classification: "repetitive",
		MatchSpec: NoiseMatchSpec{
			MessageRegex: "test prefix",
		},
	}
	_ = nc.AddRules([]NoiseRule{rule})

	rules := nc.ListRules()
	found := false
	for _, r := range rules {
		if r.MatchSpec.MessageRegex == "test prefix" {
			if len(r.ID) < 5 || r.ID[:5] != "user_" {
				t.Errorf("user rule should have 'user_' prefix, got ID: %s", r.ID)
			}
			found = true
			break
		}
	}
	if !found {
		t.Error("could not find added user rule")
	}
}

func TestNoiseMatchSpecMethodFilter(t *testing.T) {
	t.Parallel()
	nc := NewNoiseConfig()

	// Add rule that only matches GET requests
	rule := NoiseRule{
		Category:       "network",
		Classification: "infrastructure",
		MatchSpec: NoiseMatchSpec{
			URLRegex: "/internal/",
			Method:   "GET",
		},
	}
	_ = nc.AddRules([]NoiseRule{rule})

	getBody := types.NetworkBody{
		Method: "GET",
		URL:    "http://localhost:3000/internal/status",
		Status: 200,
	}
	if !nc.IsNetworkNoise(getBody) {
		t.Error("GET to /internal/ should be noise")
	}

	postBody := types.NetworkBody{
		Method: "POST",
		URL:    "http://localhost:3000/internal/status",
		Status: 200,
	}
	if nc.IsNetworkNoise(postBody) {
		t.Error("POST to /internal/ should NOT be noise (method filter is GET)")
	}
}

func TestNoiseMatchSpecStatusRange(t *testing.T) {
	t.Parallel()
	nc := NewNoiseConfig()

	// Add rule that only matches 4xx status on .map files
	rule := NoiseRule{
		Category:       "network",
		Classification: "cosmetic",
		MatchSpec: NoiseMatchSpec{
			URLRegex:  "\\.css\\.map$",
			StatusMin: 400,
			StatusMax: 499,
		},
	}
	_ = nc.AddRules([]NoiseRule{rule})

	body404 := types.NetworkBody{
		Method: "GET",
		URL:    "http://localhost:3000/styles.css.map",
		Status: 404,
	}
	if !nc.IsNetworkNoise(body404) {
		t.Error(".css.map 404 should be noise")
	}

	body200 := types.NetworkBody{
		Method: "GET",
		URL:    "http://localhost:3000/styles.css.map",
		Status: 200,
	}
	if nc.IsNetworkNoise(body200) {
		t.Error(".css.map 200 should NOT be noise (status range is 4xx)")
	}
}

func TestNoiseMatchSpecLevelFilter(t *testing.T) {
	t.Parallel()
	nc := NewNoiseConfig()

	// Add rule that only matches warn-level messages
	rule := NoiseRule{
		Category:       "console",
		Classification: "cosmetic",
		MatchSpec: NoiseMatchSpec{
			MessageRegex: "experimental feature",
			Level:        "warn",
		},
	}
	_ = nc.AddRules([]NoiseRule{rule})

	warnEntry := types.LogEntry{
		"level":   "warn",
		"message": "Using experimental feature X",
		"source":  "http://localhost:3000/app.js",
	}
	if !nc.IsConsoleNoise(warnEntry) {
		t.Error("warn-level experimental feature message should be noise")
	}

	errorEntry := types.LogEntry{
		"level":   "error",
		"message": "Using experimental feature X",
		"source":  "http://localhost:3000/app.js",
	}
	if nc.IsConsoleNoise(errorEntry) {
		t.Error("error-level should NOT be noise (level filter is warn)")
	}
}

// ============================================
// Coverage Gap Tests
// ============================================

func TestDismissNoise_WebSocketCategory(t *testing.T) {
	t.Parallel()
	nc := NewNoiseConfig()

	nc.DismissNoise("wss://example\\.com/socket", "websocket", "noisy socket")

	rules := nc.ListRules()
	var found bool
	for _, r := range rules {
		if r.Category == "websocket" && r.MatchSpec.URLRegex == "wss://example\\.com/socket" {
			found = true
			if r.Classification != "dismissed" {
				t.Errorf("Expected classification 'dismissed', got %q", r.Classification)
			}
			if r.Reason != "noisy socket" {
				t.Errorf("Expected reason 'noisy socket', got %q", r.Reason)
			}
			break
		}
	}
	if !found {
		t.Error("Expected a websocket dismiss rule to be created")
	}

	// Verify the rule actually matches websocket events
	event := types.WebSocketEvent{
		URL: "wss://example.com/socket",
	}
	if !nc.IsWebSocketNoise(event) {
		t.Error("Dismissed websocket URL should be noise")
	}
}

func TestIsCoveredLocked_LevelMismatch(t *testing.T) {
	t.Parallel()
	nc := NewNoiseConfig()

	// Add a rule with a message regex (but the rule also has Level set).
	// isConsoleCoveredLocked does NOT check level — it only checks messageRegex.
	// So a message matching the regex is "covered" regardless of level.
	rule := NoiseRule{
		Category:       "console",
		Classification: "cosmetic",
		MatchSpec: NoiseMatchSpec{
			MessageRegex: "experimental feature",
			Level:        "warn",
		},
	}
	_ = nc.AddRules([]NoiseRule{rule})

	// isConsoleCoveredLocked is called inside AutoDetect to prevent duplicate proposals.
	// Even though the rule has Level=warn, the coverage check matches by regex alone.
	entries := []types.LogEntry{
		{"message": "experimental feature", "level": "error", "source": "app.js"},
	}

	// Feed enough entries to trigger frequency detection
	manyEntries := make([]types.LogEntry, 15)
	for i := range manyEntries {
		manyEntries[i] = entries[0]
	}

	proposals := nc.AutoDetect(manyEntries, nil, nil)

	// The message is already covered by the existing rule (messageRegex match),
	// so no new proposal should be generated for it
	for _, p := range proposals {
		if p.Rule.MatchSpec.MessageRegex == "experimental feature" ||
			p.Rule.MatchSpec.MessageRegex == "experimental\\ feature" {
			t.Error("Should not propose a rule for an already-covered message")
		}
	}
}

func TestIsSourceCoveredLocked_RegexMatch(t *testing.T) {
	t.Parallel()
	nc := NewNoiseConfig()

	// Add a rule with sourceRegex matching node_modules paths
	rule := NoiseRule{
		Category:       "console",
		Classification: "extension",
		MatchSpec: NoiseMatchSpec{
			SourceRegex: `node_modules/react`,
		},
	}
	_ = nc.AddRules([]NoiseRule{rule})

	// Create entries from the covered source (node_modules is required for source analysis)
	entries := make([]types.LogEntry, 5)
	for i := range entries {
		entries[i] = types.LogEntry{
			"message": fmt.Sprintf("react warning %d", i),
			"source":  "http://localhost:3000/node_modules/react/cjs/react.development.js",
			"level":   "warn",
		}
	}

	proposals := nc.AutoDetect(entries, nil, nil)

	// The source is already covered, so no proposal should be generated for it
	for _, p := range proposals {
		if strings.Contains(p.Rule.MatchSpec.SourceRegex, "node_modules/react") {
			t.Error("Should not propose a rule for an already-covered source")
		}
	}
}

func TestIsURLCoveredLocked_RegexMatch(t *testing.T) {
	t.Parallel()
	nc := NewNoiseConfig()

	// Add a rule with URLRegex matching health endpoint
	rule := NoiseRule{
		Category:       "network",
		Classification: "infrastructure",
		MatchSpec: NoiseMatchSpec{
			URLRegex: `/health`,
		},
	}
	_ = nc.AddRules([]NoiseRule{rule})

	// Create enough network bodies to trigger frequency detection (>= 20)
	bodies := make([]types.NetworkBody, 25)
	for i := range bodies {
		bodies[i] = types.NetworkBody{
			URL:    "http://localhost:3000/health",
			Method: "GET",
			Status: 200,
		}
	}

	proposals := nc.AutoDetect(nil, bodies, nil)

	// The URL is already covered, so no proposal should be generated for it
	for _, p := range proposals {
		if strings.Contains(p.Rule.MatchSpec.URLRegex, "health") {
			t.Error("Should not propose a rule for an already-covered URL path")
		}
	}
}

func TestIsURLCoveredLocked_StatusMinMaxRange(t *testing.T) {
	t.Parallel()
	nc := NewNoiseConfig()

	// The built-in sourcemap rule has URLRegex=`\.map(\?|$)` with StatusMin=400, StatusMax=499
	// Verify the URL coverage check works regardless of status range (isURLCoveredLocked
	// only checks urlRegex, not status ranges)

	// Create enough .map requests to trigger frequency detection
	bodies := make([]types.NetworkBody, 25)
	for i := range bodies {
		bodies[i] = types.NetworkBody{
			URL:    "http://localhost:3000/__webpack_hmr",
			Method: "GET",
			Status: 200,
		}
	}

	// Add a network rule covering /__webpack_hmr
	rule := NoiseRule{
		Category:       "network",
		Classification: "infrastructure",
		MatchSpec: NoiseMatchSpec{
			URLRegex:  `__webpack_hmr`,
			StatusMin: 200,
			StatusMax: 299,
		},
	}
	_ = nc.AddRules([]NoiseRule{rule})

	proposals := nc.AutoDetect(nil, bodies, nil)

	for _, p := range proposals {
		if strings.Contains(p.Rule.MatchSpec.URLRegex, "__webpack_hmr") {
			t.Error("Should not propose a rule for a URL already covered by urlRegex")
		}
	}
}
