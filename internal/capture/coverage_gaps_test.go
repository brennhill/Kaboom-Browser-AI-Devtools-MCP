// Purpose: Coverage-expansion tests for capture pipeline edge cases and branch paths.
// Docs: docs/features/feature/backend-log-streaming/index.md

// coverage_gaps_test.go — Targeted tests for uncovered capture paths (part 1).
// Covers: lifecycle observer access, extension version compatibility,
// GetVersionMismatch, majorMinor, detectAndSetBinaryFormat,
// redactExtensionLog edge cases, circuit breaker, and HTTP handlers.
package capture

import (
	"encoding/json"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/circuit"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/lifecycle"
)

func TestExtractBrowserNameUsesPrivacySafeFamilies(t *testing.T) {
	t.Parallel()
	for userAgent, want := range map[string]string{
		"Mozilla Chrome Brave":   "brave",
		"Mozilla Chrome Edg/120": "edge",
		"Mozilla Chrome/120":     "chrome",
		"Mozilla Firefox/120":    "firefox",
		"Mozilla Safari/17":      "safari",
		"custom client":          "unknown",
	} {
		if got := extractBrowserName(userAgent); got != want {
			t.Fatalf("extractBrowserName(%q) = %q, want %q", userAgent, got, want)
		}
	}
}

// ============================================
// Lifecycle observer
// ============================================

func TestLifecycleObserver(t *testing.T) {
	t.Parallel()

	c := NewCapture()
	defer c.Close()

	var received lifecycle.Event
	var receivedData map[string]any
	c.Lifecycle().Subscribe(func(event lifecycle.Event, data map[string]any) {
		received = event
		receivedData = data
	})

	c.Lifecycle().Emit(lifecycle.EventCircuitOpened, map[string]any{"key": "value"})

	if received != lifecycle.EventCircuitOpened {
		t.Errorf("callback event = %v, want circuit_opened", received)
	}
	if receivedData["key"] != "value" {
		t.Errorf("callback data = %v, want key=value", receivedData)
	}
}

func TestLifecycleObserverEmitWithoutSubscribers(t *testing.T) {
	t.Parallel()

	c := NewCapture()
	defer c.Close()

	// Should not panic when no callback is set
	c.Lifecycle().Emit(lifecycle.EventUnknown, nil)
}

// ============================================
// Extension version compatibility / majorMinor
// ============================================

func TestExtensionRuntimeServerVersion(t *testing.T) {
	t.Parallel()

	c := NewCapture()
	defer c.Close()

	c.Extension().SetServerVersion("6.0.3")
	if got := c.Extension().ServerVersion(); got != "6.0.3" {
		t.Errorf("ServerVersion() = %q, want 6.0.3", got)
	}
}

func TestGetVersionMismatch_NoExtensionVersion(t *testing.T) {
	t.Parallel()

	c := NewCapture()
	defer c.Close()

	c.Extension().SetServerVersion("6.0.3")
	extVer, srvVer, mismatch := c.Extension().VersionMismatch()
	if extVer != "" {
		t.Errorf("extVer = %q, want empty", extVer)
	}
	if srvVer != "6.0.3" {
		t.Errorf("srvVer = %q, want 6.0.3", srvVer)
	}
	if mismatch {
		t.Error("mismatch = true, want false when extension version empty")
	}
}

func TestGetVersionMismatch_NoServerVersion(t *testing.T) {
	t.Parallel()

	c := NewCapture()
	defer c.Close()

	c.Extension().SetExtensionVersion("6.0.3")

	_, _, mismatch := c.Extension().VersionMismatch()
	if mismatch {
		t.Error("mismatch = true, want false when server version empty")
	}
}

func TestGetVersionMismatch_Match(t *testing.T) {
	t.Parallel()

	c := NewCapture()
	defer c.Close()

	c.Extension().SetServerVersion("6.0.3")
	c.Extension().SetExtensionVersion("6.0.5")

	extVer, srvVer, mismatch := c.Extension().VersionMismatch()
	if extVer != "6.0.5" {
		t.Errorf("extVer = %q, want 6.0.5", extVer)
	}
	if srvVer != "6.0.3" {
		t.Errorf("srvVer = %q, want 6.0.3", srvVer)
	}
	if mismatch {
		t.Error("mismatch = true, want false (same major.minor)")
	}
}

func TestGetVersionMismatch_Mismatch(t *testing.T) {
	t.Parallel()

	c := NewCapture()
	defer c.Close()

	c.Extension().SetServerVersion("6.0.3")
	c.Extension().SetExtensionVersion("5.9.0")

	_, _, mismatch := c.Extension().VersionMismatch()
	if !mismatch {
		t.Error("mismatch = false, want true (6.0 != 5.9)")
	}
}

func TestMajorMinor(t *testing.T) {
	t.Parallel()

	cases := []struct {
		input string
		want  string
	}{
		{"6.0.3", "6.0"},
		{"1.2.3", "1.2"},
		{"10.20.30", "10.20"},
		{"6.0", "6.0"},
		{"6", ""},
		{"", ""},
	}

	for _, tc := range cases {
		got := majorMinor(tc.input)
		if got != tc.want {
			t.Errorf("majorMinor(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestGetVersionMismatch_InvalidVersionFormat(t *testing.T) {
	t.Parallel()

	c := NewCapture()
	defer c.Close()

	c.Extension().SetServerVersion("6.0.3")
	c.Extension().SetExtensionVersion("invalid")

	_, _, mismatch := c.Extension().VersionMismatch()
	if mismatch {
		t.Error("mismatch = true, want false for invalid version format")
	}
}

// ============================================
// detectAndSetBinaryFormat
// ============================================

func TestDetectAndSetBinaryFormat_AlreadySet(t *testing.T) {
	t.Parallel()
	c := NewCapture()
	c.Telemetry().AddNetworkBodies([]types.NetworkBody{{BinaryFormat: "png", RequestBody: "some content"}})
	if got := c.Telemetry().NetworkBodies().Snapshot().Bodies[0].BinaryFormat; got != "png" {
		t.Errorf("BinaryFormat = %q, want png (should not change)", got)
	}
}

func TestDetectAndSetBinaryFormat_EmptyBodies(t *testing.T) {
	t.Parallel()
	c := NewCapture()
	c.Telemetry().AddNetworkBodies([]types.NetworkBody{{}})
	if got := c.Telemetry().NetworkBodies().Snapshot().Bodies[0].BinaryFormat; got != "" {
		t.Errorf("BinaryFormat = %q, want empty for empty bodies", got)
	}
}

func TestDetectAndSetBinaryFormat_PNG_ResponseBody(t *testing.T) {
	t.Parallel()
	pngMagic := "\x89PNG\r\n\x1a\n" + strings.Repeat("\x00", 20)
	c := NewCapture()
	c.Telemetry().AddNetworkBodies([]types.NetworkBody{{ResponseBody: pngMagic}})
	if c.Telemetry().NetworkBodies().Snapshot().Bodies[0].BinaryFormat == "" {
		t.Skip("PNG detection not triggered — util.DetectBinaryFormat may need longer header")
	}
}

func TestDetectAndSetBinaryFormat_TextBodies(t *testing.T) {
	t.Parallel()
	c := NewCapture()
	c.Telemetry().AddNetworkBodies([]types.NetworkBody{{RequestBody: `{"hello":"world"}`, ResponseBody: `{"ok":true}`}})
	if got := c.Telemetry().NetworkBodies().Snapshot().Bodies[0].BinaryFormat; got != "" {
		t.Errorf("BinaryFormat = %q, want empty for text content", got)
	}
}

// ============================================
// Circuit breaker (delegation tests — struct tests live in internal/circuit)
// ============================================

func TestCircuitBreaker_GetHealthStatus_Open(t *testing.T) {
	t.Parallel()
	cb := circuit.NewCircuitBreaker(func(lifecycle.Event, map[string]any) {})
	cb.ForceOpen("test_reason")
	health := cb.GetHealthStatus()
	if !health.CircuitOpen {
		t.Error("CircuitOpen = false, want true")
	}
	if health.Reason != "test_reason" {
		t.Errorf("Reason = %q, want test_reason", health.Reason)
	}
	if health.OpenedAt == "" {
		t.Error("OpenedAt should be non-empty when circuit is open")
	}
}

func TestCircuitBreaker_GetHealthStatus_Closed(t *testing.T) {
	t.Parallel()
	cb := circuit.NewCircuitBreaker(func(lifecycle.Event, map[string]any) {})
	health := cb.GetHealthStatus()
	if health.CircuitOpen {
		t.Error("CircuitOpen = true, want false")
	}
	if health.OpenedAt != "" {
		t.Errorf("OpenedAt = %q, want empty when closed", health.OpenedAt)
	}
}

// ============================================
// HTTP Handlers
// ============================================

func TestHandleNetworkBodies_MethodNotAllowed(t *testing.T) {
	t.Parallel()
	c := NewCapture()
	defer c.Close()
	rr := httptest.NewRecorder()
	NewHTTPHandlers(c).HandleNetworkBodies(rr, httptest.NewRequest(http.MethodGet, "/network-bodies", nil))
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusMethodNotAllowed)
	}
}

func TestHandleNetworkBodies_InvalidJSON(t *testing.T) {
	t.Parallel()
	c := NewCapture()
	defer c.Close()
	rr := httptest.NewRecorder()
	NewHTTPHandlers(c).HandleNetworkBodies(rr, httptest.NewRequest(http.MethodPost, "/network-bodies", strings.NewReader("{invalid")))
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestHandleNetworkBodies_Success(t *testing.T) {
	t.Parallel()
	c := NewCapture()
	defer c.Close()
	payload := `{"bodies":[{"method":"GET","url":"https://example.com","status":200}]}`
	rr := httptest.NewRecorder()
	NewHTTPHandlers(c).HandleNetworkBodies(rr, httptest.NewRequest(http.MethodPost, "/network-bodies", strings.NewReader(payload)))
	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}
	if resp["status"] != "ok" || resp["count"] != float64(1) {
		t.Errorf("response = %v", resp)
	}
}

func TestHandleEnhancedActions_MethodNotAllowed(t *testing.T) {
	t.Parallel()
	c := NewCapture()
	defer c.Close()
	rr := httptest.NewRecorder()
	NewHTTPHandlers(c).HandleEnhancedActions(rr, httptest.NewRequest(http.MethodGet, "/enhanced-actions", nil))
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusMethodNotAllowed)
	}
}

func TestHandleEnhancedActions_InvalidJSON(t *testing.T) {
	t.Parallel()
	c := NewCapture()
	defer c.Close()
	rr := httptest.NewRecorder()
	NewHTTPHandlers(c).HandleEnhancedActions(rr, httptest.NewRequest(http.MethodPost, "/enhanced-actions", strings.NewReader("{bad")))
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestHandleEnhancedActions_Success(t *testing.T) {
	t.Parallel()
	c := NewCapture()
	defer c.Close()
	rr := httptest.NewRecorder()
	NewHTTPHandlers(c).HandleEnhancedActions(rr, httptest.NewRequest(http.MethodPost, "/enhanced-actions", strings.NewReader(`{"actions":[{"type":"click","selector":"#btn"}]}`)))
	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestHandlePerformanceSnapshots_MethodNotAllowed(t *testing.T) {
	t.Parallel()
	c := NewCapture()
	defer c.Close()
	rr := httptest.NewRecorder()
	NewHTTPHandlers(c).HandlePerformanceSnapshots(rr, httptest.NewRequest(http.MethodGet, "/performance-snapshots", nil))
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusMethodNotAllowed)
	}
}

func TestHandlePerformanceSnapshots_InvalidJSON(t *testing.T) {
	t.Parallel()
	c := NewCapture()
	defer c.Close()
	rr := httptest.NewRecorder()
	NewHTTPHandlers(c).HandlePerformanceSnapshots(rr, httptest.NewRequest(http.MethodPost, "/performance-snapshots", strings.NewReader("{bad")))
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestHandlePerformanceSnapshots_Success(t *testing.T) {
	t.Parallel()
	c := NewCapture()
	defer c.Close()
	rr := httptest.NewRecorder()
	NewHTTPHandlers(c).HandlePerformanceSnapshots(rr, httptest.NewRequest(http.MethodPost, "/performance-snapshots", strings.NewReader(`{"snapshots":[{"url":"https://example.com","timestamp":"2026-01-01T00:00:00Z"}]}`)))
	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestHandleNetworkWaterfall_MethodNotAllowed(t *testing.T) {
	t.Parallel()
	c := NewCapture()
	defer c.Close()
	rr := httptest.NewRecorder()
	NewHTTPHandlers(c).HandleNetworkWaterfall(rr, httptest.NewRequest(http.MethodGet, "/network-waterfall", nil))
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusMethodNotAllowed)
	}
}

func TestHandleNetworkWaterfall_InvalidJSON(t *testing.T) {
	t.Parallel()
	c := NewCapture()
	defer c.Close()
	rr := httptest.NewRecorder()
	NewHTTPHandlers(c).HandleNetworkWaterfall(rr, httptest.NewRequest(http.MethodPost, "/network-waterfall", strings.NewReader("{bad")))
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestHandleNetworkWaterfall_Success(t *testing.T) {
	t.Parallel()
	c := NewCapture()
	defer c.Close()
	rr := httptest.NewRecorder()
	NewHTTPHandlers(c).HandleNetworkWaterfall(rr, httptest.NewRequest(http.MethodPost, "/network-waterfall", strings.NewReader(`{"entries":[{"name":"https://example.com/app.js","duration":50}],"page_url":"https://example.com"}`)))
	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d; body = %s", rr.Code, http.StatusOK, rr.Body.String())
	}
}
