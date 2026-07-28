// Purpose: Tests for noise rule matching and log suppression.
// Docs: docs/features/feature/noise-filtering/index.md

package noise

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

// ============================================
// Test Scenario 1: NewNoiseConfig has all built-in rules present
// ============================================

func TestNoiseNewConfigHasBuiltinRules(t *testing.T) {
	t.Parallel()
	nc := NewNoiseConfig()
	rules := nc.ListRules()

	// Should have ~45 built-in rules
	builtinCount := 0
	for _, r := range rules {
		if len(r.ID) >= 8 && r.ID[:8] == "builtin_" {
			builtinCount++
		}
	}
	if builtinCount < 40 {
		t.Errorf("expected at least 40 built-in rules, got %d", builtinCount)
	}

	// Verify specific built-in IDs exist
	expectedIDs := []string{
		"builtin_chrome_extension",
		"builtin_favicon",
		"builtin_sourcemap_404",
		"builtin_hmr_console",
		"builtin_hmr_network",
		"builtin_react_devtools",
		"builtin_cors_preflight",
		"builtin_google_analytics",
		"builtin_segment",
		"builtin_sentry",
		"builtin_service_worker",
		"builtin_passive_listener",
		"builtin_deprecation",
		"builtin_devtools_sourcemap",
		"builtin_ws_hmr",
	}
	ruleMap := make(map[string]bool)
	for _, r := range rules {
		ruleMap[r.ID] = true
	}
	for _, id := range expectedIDs {
		if !ruleMap[id] {
			t.Errorf("missing built-in rule: %s", id)
		}
	}
}

// ============================================
// Test Scenario 2: Chrome extension source -> correctly identified as noise
// ============================================

func TestNoiseConsoleFromChromeExtension(t *testing.T) {
	t.Parallel()
	nc := NewNoiseConfig()

	entry := types.LogEntry{
		"level":   "warn",
		"message": "Some extension warning",
		"source":  "chrome-extension://abcdef123456/content.js",
	}

	if !nc.IsConsoleNoise(entry) {
		t.Error("chrome-extension:// source should be classified as noise")
	}

	// Also test moz-extension://
	entry2 := types.LogEntry{
		"level":   "info",
		"message": "Firefox addon message",
		"source":  "moz-extension://abcdef123456/background.js",
	}

	if !nc.IsConsoleNoise(entry2) {
		t.Error("moz-extension:// source should be classified as noise")
	}
}

// ============================================
// Test Scenario 3: Application error from localhost:3000 -> not noise
// ============================================

func TestNoiseAppErrorNotNoise(t *testing.T) {
	t.Parallel()
	nc := NewNoiseConfig()

	entry := types.LogEntry{
		"level":   "error",
		"message": "TypeError: Cannot read property 'foo' of undefined",
		"source":  "http://localhost:3000/app.js",
	}

	if nc.IsConsoleNoise(entry) {
		t.Error("application error from localhost:3000 should NOT be noise")
	}
}

// ============================================
// Test Scenario 4: Favicon 404 -> noise
// ============================================

func TestNoiseFavicon404(t *testing.T) {
	t.Parallel()
	nc := NewNoiseConfig()

	body := capture.NetworkBody{
		Method: "GET",
		URL:    "http://localhost:3000/favicon.ico",
		Status: 404,
	}

	if !nc.IsNetworkNoise(body) {
		t.Error("favicon.ico request should be classified as noise")
	}
}

// ============================================
// Test Scenario 5: API endpoint 500 -> not noise (real failure)
// ============================================

func TestNoiseAPI500NotNoise(t *testing.T) {
	t.Parallel()
	nc := NewNoiseConfig()

	body := capture.NetworkBody{
		Method: "POST",
		URL:    "http://localhost:3000/api/users",
		Status: 500,
	}

	if nc.IsNetworkNoise(body) {
		t.Error("API 500 error should NOT be noise")
	}
}

// ============================================
// Test Scenario 6: Segment analytics URL -> noise
// ============================================

func TestNoiseAnalyticsURL(t *testing.T) {
	t.Parallel()
	nc := NewNoiseConfig()

	analyticsURLs := []string{
		"https://api.segment.io/v1/track",
		"https://www.google-analytics.com/collect",
		"https://cdn.mxpnl.com/libs/mixpanel.js",
		"https://static.hotjar.com/c/hotjar-123.js",
		"https://api.amplitude.com/2/httpapi",
		"https://plausible.io/api/event",
		"https://us.posthog.com/capture",
	}

	for _, url := range analyticsURLs {
		body := capture.NetworkBody{
			Method: "GET",
			URL:    url,
			Status: 200,
		}
		if !nc.IsNetworkNoise(body) {
			t.Errorf("analytics URL should be noise: %s", url)
		}
	}
}

// ============================================
// Test Scenario 7: OPTIONS 204 preflight -> noise
// ============================================

func TestNoiseCORSPreflight(t *testing.T) {
	t.Parallel()
	nc := NewNoiseConfig()

	body := capture.NetworkBody{
		Method: "OPTIONS",
		URL:    "http://localhost:3000/api/users",
		Status: 204,
	}

	if !nc.IsNetworkNoise(body) {
		t.Error("OPTIONS 204 preflight should be noise")
	}

	// OPTIONS 200 should also be noise
	body.Status = 200
	if !nc.IsNetworkNoise(body) {
		t.Error("OPTIONS 200 preflight should be noise")
	}

	// OPTIONS 403 should NOT be noise (auth issue)
	body.Status = 403
	if nc.IsNetworkNoise(body) {
		t.Error("OPTIONS 403 should NOT be noise (auth invariant)")
	}
}

// ============================================
// Test Scenario 8: [vite] hot update message -> noise
// ============================================

func TestNoiseHMRMessages(t *testing.T) {
	t.Parallel()
	nc := NewNoiseConfig()

	hmrMessages := []string{
		"[vite] hot updated: /src/App.tsx",
		"[HMR] Waiting for update signal from WDS...",
		"[webpack] Building...",
		"[next] Fast Refresh - Full reload",
	}

	for _, msg := range hmrMessages {
		entry := types.LogEntry{
			"level":   "info",
			"message": msg,
			"source":  "http://localhost:3000/app.js",
		}
		if !nc.IsConsoleNoise(entry) {
			t.Errorf("HMR message should be noise: %s", msg)
		}
	}
}

// ============================================
// Test Scenario 9: React DevTools download prompt -> noise
// ============================================

func TestNoiseReactDevTools(t *testing.T) {
	t.Parallel()
	nc := NewNoiseConfig()

	entry := types.LogEntry{
		"level":   "info",
		"message": "Download the React DevTools for a better development experience: https://reactjs.org/link/react-devtools",
		"source":  "http://localhost:3000/bundle.js",
	}

	if !nc.IsConsoleNoise(entry) {
		t.Error("React DevTools download prompt should be noise")
	}
}

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

	body := capture.NetworkBody{
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

				body := capture.NetworkBody{
					Method: "GET",
					URL:    "http://localhost:3000/favicon.ico",
					Status: 404,
				}
				nc.IsNetworkNoise(body)

				wsEvent := capture.WebSocketEvent{
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

// ============================================
// Test Scenario 23: Auth-related entries (401 response) -> never filtered
// ============================================

func TestNoiseAuthNeverFiltered(t *testing.T) {
	t.Parallel()
	nc := NewNoiseConfig()

	// Even if a rule matches the URL, 401/403 should never be filtered
	rule := NoiseRule{
		Category:       "network",
		Classification: "infrastructure",
		MatchSpec: NoiseMatchSpec{
			URLRegex: "/api/",
		},
	}
	_ = nc.AddRules([]NoiseRule{rule})

	// 401 should never be noise
	body401 := capture.NetworkBody{
		Method: "GET",
		URL:    "http://localhost:3000/api/protected",
		Status: 401,
	}
	if nc.IsNetworkNoise(body401) {
		t.Error("401 response should NEVER be classified as noise")
	}

	// 403 should never be noise
	body403 := capture.NetworkBody{
		Method: "GET",
		URL:    "http://localhost:3000/api/admin",
		Status: 403,
	}
	if nc.IsNetworkNoise(body403) {
		t.Error("403 response should NEVER be classified as noise")
	}

	// 200 to the same pattern should still be noise
	body200 := capture.NetworkBody{
		Method: "GET",
		URL:    "http://localhost:3000/api/data",
		Status: 200,
	}
	if !nc.IsNetworkNoise(body200) {
		t.Error("200 response matching rule should be noise")
	}
}

// ============================================
// Additional edge case tests
// ============================================

func TestNoiseSourceMap404(t *testing.T) {
	t.Parallel()
	nc := NewNoiseConfig()

	body := capture.NetworkBody{
		Method: "GET",
		URL:    "http://localhost:3000/assets/app.js.map",
		Status: 404,
	}

	if !nc.IsNetworkNoise(body) {
		t.Error("source map 404 should be noise")
	}

	// Source map 200 should NOT be noise
	body.Status = 200
	if nc.IsNetworkNoise(body) {
		t.Error("source map 200 should NOT be noise")
	}
}

func TestNoiseHMRNetworkURLs(t *testing.T) {
	t.Parallel()
	nc := NewNoiseConfig()

	hmrURLs := []string{
		"http://localhost:3000/__vite_ping",
		"http://localhost:3000/hot-update.js",
		"http://localhost:3000/_next/webpack-hmr",
		"ws://localhost:3000/__webpack_hmr",
	}

	for _, url := range hmrURLs {
		body := capture.NetworkBody{
			Method: "GET",
			URL:    url,
			Status: 200,
		}
		if !nc.IsNetworkNoise(body) {
			t.Errorf("HMR network URL should be noise: %s", url)
		}
	}
}

func TestNoiseServiceWorkerMessages(t *testing.T) {
	t.Parallel()
	nc := NewNoiseConfig()

	messages := []string{
		"Service Worker registration successful",
		"ServiceWorker: activated",
		"Service worker installed",
	}

	for _, msg := range messages {
		entry := types.LogEntry{
			"level":   "info",
			"message": msg,
			"source":  "http://localhost:3000/sw.js",
		}
		if !nc.IsConsoleNoise(entry) {
			t.Errorf("service worker message should be noise: %s", msg)
		}
	}
}

func TestNoisePassiveEventListener(t *testing.T) {
	t.Parallel()
	nc := NewNoiseConfig()

	entry := types.LogEntry{
		"level":   "warn",
		"message": "Added non-passive event listener to a scroll-blocking 'touchstart' event",
		"source":  "http://localhost:3000/vendor.js",
	}

	if !nc.IsConsoleNoise(entry) {
		t.Error("passive event listener warning should be noise")
	}
}

func TestNoiseDeprecationWarning(t *testing.T) {
	t.Parallel()
	nc := NewNoiseConfig()

	entry := types.LogEntry{
		"level":   "warn",
		"message": "[Deprecation] SharedArrayBuffer will require cross-origin isolation",
		"source":  "http://localhost:3000/bundle.js",
	}

	if !nc.IsConsoleNoise(entry) {
		t.Error("[Deprecation] warning should be noise")
	}
}

func TestNoiseWebSocketEvent(t *testing.T) {
	t.Parallel()
	nc := NewNoiseConfig()

	// Add a websocket noise rule
	rule := NoiseRule{
		Category:       "websocket",
		Classification: "framework",
		MatchSpec: NoiseMatchSpec{
			URLRegex: "sockjs-node",
		},
	}
	_ = nc.AddRules([]NoiseRule{rule})

	event := capture.WebSocketEvent{
		URL:   "ws://localhost:3000/sockjs-node/websocket",
		Event: "message",
		Data:  "heartbeat",
	}

	if !nc.IsWebSocketNoise(event) {
		t.Error("sockjs-node WebSocket event should be noise")
	}

	// Normal WebSocket should not be noise
	event2 := capture.WebSocketEvent{
		URL:   "ws://localhost:3000/api/live",
		Event: "message",
		Data:  "user data",
	}

	if nc.IsWebSocketNoise(event2) {
		t.Error("normal WebSocket event should not be noise")
	}
}

func TestNoiseAutoDetectNetworkFrequency(t *testing.T) {
	t.Parallel()
	nc := NewNoiseConfig()

	// Create 25 network requests to /health endpoint
	var bodies []capture.NetworkBody
	for i := 0; i < 25; i++ {
		bodies = append(bodies, capture.NetworkBody{
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

	getBody := capture.NetworkBody{
		Method: "GET",
		URL:    "http://localhost:3000/internal/status",
		Status: 200,
	}
	if !nc.IsNetworkNoise(getBody) {
		t.Error("GET to /internal/ should be noise")
	}

	postBody := capture.NetworkBody{
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

	body404 := capture.NetworkBody{
		Method: "GET",
		URL:    "http://localhost:3000/styles.css.map",
		Status: 404,
	}
	if !nc.IsNetworkNoise(body404) {
		t.Error(".css.map 404 should be noise")
	}

	body200 := capture.NetworkBody{
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
	event := capture.WebSocketEvent{
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
	bodies := make([]capture.NetworkBody, 25)
	for i := range bodies {
		bodies[i] = capture.NetworkBody{
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
	bodies := make([]capture.NetworkBody, 25)
	for i := range bodies {
		bodies[i] = capture.NetworkBody{
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
