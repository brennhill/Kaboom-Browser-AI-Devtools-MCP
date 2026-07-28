// noise_builtin_matching_test.go — Tests built-in browser-noise classification.
// Docs: docs/features/feature/noise-filtering/index.md

package noise

import (
	"testing"

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

	body := types.NetworkBody{
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

	body := types.NetworkBody{
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
		body := types.NetworkBody{
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

	body := types.NetworkBody{
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
	body401 := types.NetworkBody{
		Method: "GET",
		URL:    "http://localhost:3000/api/protected",
		Status: 401,
	}
	if nc.IsNetworkNoise(body401) {
		t.Error("401 response should NEVER be classified as noise")
	}

	// 403 should never be noise
	body403 := types.NetworkBody{
		Method: "GET",
		URL:    "http://localhost:3000/api/admin",
		Status: 403,
	}
	if nc.IsNetworkNoise(body403) {
		t.Error("403 response should NEVER be classified as noise")
	}

	// 200 to the same pattern should still be noise
	body200 := types.NetworkBody{
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

	body := types.NetworkBody{
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
		body := types.NetworkBody{
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

	event := types.WebSocketEvent{
		URL:   "ws://localhost:3000/sockjs-node/websocket",
		Event: "message",
		Data:  "heartbeat",
	}

	if !nc.IsWebSocketNoise(event) {
		t.Error("sockjs-node WebSocket event should be noise")
	}

	// Normal WebSocket should not be noise
	event2 := types.WebSocketEvent{
		URL:   "ws://localhost:3000/api/live",
		Event: "message",
		Data:  "user data",
	}

	if nc.IsWebSocketNoise(event2) {
		t.Error("normal WebSocket event should not be noise")
	}
}
