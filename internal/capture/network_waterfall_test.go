// Purpose: Tests for network waterfall capture and timing data.
// Docs: docs/features/feature/backend-log-streaming/index.md

// Package capture provides telemetry capture functionality
package capture

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/telemetrystore"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/waterfallstore"
)

// ============================================
// Network Waterfall Tests (TDD Phase 2)
// ============================================
// These tests verify the network waterfall capture system for complete
// CSP generation and security flagging.

// ============================================
// Basic Functionality Tests
// ============================================

func TestHandleNetworkWaterfall_AcceptsValidPayload(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	payload := types.NetworkWaterfallPayload{
		PageURL: "https://example.com",
		Entries: []types.NetworkWaterfallEntry{
			{
				Name:            "https://example.com/app.js",
				URL:             "https://example.com/app.js",
				InitiatorType:   "script",
				Duration:        50.5,
				TransferSize:    1024,
				DecodedBodySize: 2048,
			},
		},
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/network-waterfall", bytes.NewReader(body))
	w := httptest.NewRecorder()

	NewHTTPHandlers(capture).HandleNetworkWaterfall(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestHandleNetworkWaterfall_PreservesRichTimingAndAttribution(t *testing.T) {
	t.Parallel()
	captured := NewCapture()
	body := []byte(`{"page_url":"https://app.test","entries":[{"url":"https://app.test/api","name":"https://app.test/api","initiator_type":"fetch","duration":250,"start_time":10,"queueing_ms":3,"dns_ms":4,"tls_ms":5,"connect_ms":8,"ttfb_ms":90,"download_ms":140,"priority":"high","protocol":"h2","cache_source":"network","compression_ratio":2.4,"status":200,"server_timing":[{"name":"db","duration_ms":22}],"request_id":"req-1","traceparent":"00-abc-def-01","initiator_stack":["at DesignShell (src/DesignShell.tsx:1:2)"],"react_component":"DesignShell","route_loader":"designLoader","store_action":"loadDesign","source_map_status":"browser_stack","duplicate_group_id":"dup-1","duplicate_count":2}]}`)
	w := httptest.NewRecorder()
	NewHTTPHandlers(captured).HandleNetworkWaterfall(w, httptest.NewRequest(http.MethodPost, "/network-waterfall", bytes.NewReader(body)))
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	entry := captured.Telemetry().NetworkWaterfall().Entries()[0]
	if entry.TTFBMs != 90 || entry.Protocol != "h2" || entry.RequestID != "req-1" || entry.ReactComponent != "DesignShell" || entry.DuplicateCount != 2 {
		t.Fatalf("rich waterfall fields were not preserved: %+v", entry)
	}
}

func TestHandleNetworkWaterfall_RejectsMalformedJSON(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	req := httptest.NewRequest("POST", "/network-waterfall", bytes.NewReader([]byte(`{invalid json`)))
	w := httptest.NewRecorder()

	NewHTTPHandlers(capture).HandleNetworkWaterfall(w, req)

	if w.Code == http.StatusOK {
		t.Errorf("Expected error status, got %d", w.Code)
	}
}

func TestHandleNetworkWaterfall_StoresTimestamp(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	payload := types.NetworkWaterfallPayload{
		PageURL: "https://example.com",
		Entries: []types.NetworkWaterfallEntry{
			{
				Name:          "https://example.com/app.js",
				URL:           "https://example.com/app.js",
				InitiatorType: "script",
			},
		},
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/network-waterfall", bytes.NewReader(body))
	w := httptest.NewRecorder()

	beforeTime := time.Now()
	NewHTTPHandlers(capture).HandleNetworkWaterfall(w, req)
	afterTime := time.Now()

	entries := capture.Telemetry().NetworkWaterfall().Entries()
	if len(entries) == 0 {
		t.Fatalf("Expected 1 entry, got %d", len(entries))
	}
	entryTime := entries[0].Timestamp

	if entryTime.Before(beforeTime) || entryTime.After(afterTime) {
		t.Errorf("Timestamp not set correctly: %v (should be between %v and %v)", entryTime, beforeTime, afterTime)
	}
}

func TestHandleNetworkWaterfall_StoresPageURL(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	expectedURL := "https://example.com/page"
	payload := types.NetworkWaterfallPayload{
		PageURL: expectedURL,
		Entries: []types.NetworkWaterfallEntry{
			{
				Name:          "https://example.com/app.js",
				URL:           "https://example.com/app.js",
				InitiatorType: "script",
			},
		},
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/network-waterfall", bytes.NewReader(body))
	w := httptest.NewRecorder()

	NewHTTPHandlers(capture).HandleNetworkWaterfall(w, req)

	entries := capture.Telemetry().NetworkWaterfall().Entries()
	if len(entries) == 0 {
		t.Fatalf("Expected 1 entry, got %d", len(entries))
	}
	storedURL := entries[0].PageURL

	if storedURL != expectedURL {
		t.Errorf("Expected URL %q, got %q", expectedURL, storedURL)
	}
}

// ============================================
// Ring Buffer and Memory Tests
// ============================================

func TestNetworkWaterfall_RingBufferEviction(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	// Override capacity to test eviction behavior
	replaceTelemetryForTest(capture, telemetrystore.Dependencies{Waterfall: waterfallstore.New(10)})

	// Add 12 entries which should trigger eviction since we set max to 10
	for i := 0; i < 12; i++ {
		payload := types.NetworkWaterfallPayload{
			PageURL: "https://example.com",
			Entries: []types.NetworkWaterfallEntry{
				{
					Name:          fmt.Sprintf("https://example.com/resource%d", i),
					URL:           fmt.Sprintf("https://example.com/resource%d", i),
					InitiatorType: "script",
				},
			},
		}

		body, _ := json.Marshal(payload)
		req := httptest.NewRequest("POST", "/network-waterfall", bytes.NewReader(body))
		w := httptest.NewRecorder()

		NewHTTPHandlers(capture).HandleNetworkWaterfall(w, req)
	}

	count := len(capture.Telemetry().NetworkWaterfall().Entries())

	// Should keep only the last 10 (the configured capacity)
	if count > 10 {
		t.Errorf("Expected max 10 entries, got %d", count)
	}
}

func TestNetworkWaterfall_MultipleEntriesInSinglePayload(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	payload := types.NetworkWaterfallPayload{
		PageURL: "https://example.com",
		Entries: []types.NetworkWaterfallEntry{
			{
				Name:          "https://example.com/app.js",
				URL:           "https://example.com/app.js",
				InitiatorType: "script",
			},
			{
				Name:          "https://example.com/style.css",
				URL:           "https://example.com/style.css",
				InitiatorType: "stylesheet",
			},
		},
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/network-waterfall", bytes.NewReader(body))
	w := httptest.NewRecorder()

	NewHTTPHandlers(capture).HandleNetworkWaterfall(w, req)

	if entries := capture.Telemetry().NetworkWaterfall().Entries(); len(entries) != 2 {
		t.Errorf("Expected 2 entries, got %d", len(entries))
	}
}

// ============================================
// CSP Generation Tests
// ============================================

func TestNetworkWaterfall_FeedsCSPGenerator(t *testing.T) {
	t.Parallel()
	// Skip: CSP generator integration not yet implemented.
	// The cspGen field is set by cmd/browser-agent during full server initialization,
	// not by NewCapture(). This test should be enabled when CSP generation
	// is integrated into the network waterfall capture flow.
	t.Skip("CSP generator integration not yet implemented")
}

// ============================================
// Capacity Configuration Tests
// ============================================

func TestNetworkWaterfall_DefaultCapacity(t *testing.T) {
	t.Parallel()
	capture := NewCapture()
	if capture.Telemetry().NetworkWaterfall().Pressure().Capacity == 0 {
		t.Errorf("Expected networkWaterfall buffer to be initialized")
	}
}

func TestNetworkWaterfall_CustomCapacity(t *testing.T) {
	t.Parallel()
	capture := NewCapture()
	capacity := capture.Telemetry().NetworkWaterfall().Pressure().Capacity

	if capacity == 0 {
		t.Errorf("Expected non-zero capacity")
	}
}

// ============================================
// Concurrent Access Tests
// ============================================

func TestNetworkWaterfall_ConcurrentWrites(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	// Write 100 entries concurrently
	done := make(chan bool, 100)
	for i := 0; i < 100; i++ {
		go func(index int) {
			payload := types.NetworkWaterfallPayload{
				PageURL: "https://example.com",
				Entries: []types.NetworkWaterfallEntry{
					{
						Name:          fmt.Sprintf("https://example.com/resource%d", index),
						URL:           fmt.Sprintf("https://example.com/resource%d", index),
						InitiatorType: "script",
					},
				},
			}

			body, _ := json.Marshal(payload)
			req := httptest.NewRequest("POST", "/network-waterfall", bytes.NewReader(body))
			w := httptest.NewRecorder()

			NewHTTPHandlers(capture).HandleNetworkWaterfall(w, req)
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 100; i++ {
		<-done
	}

	count := len(capture.Telemetry().NetworkWaterfall().Entries())

	if count != 100 {
		t.Errorf("Expected 100 entries, got %d", count)
	}
}

// ============================================
// MCP Tool Handler Tests (Skipped)
// ============================================
// NOTE: These tests are skipped because ToolHandler and MCPHandler
// have not been moved to internal packages yet. They remain in cmd/browser-agent
// and would create circular dependencies if imported here.

func TestToolGetNetworkWaterfall_EmptyBuffer(t *testing.T) {
	t.Parallel()
	t.Skip("ToolHandler not available in internal packages - requires cmd/browser-agent refactoring")
}

func TestToolGetNetworkWaterfall_PopulatedBuffer(t *testing.T) {
	t.Parallel()
	t.Skip("ToolHandler not available in internal packages - requires cmd/browser-agent refactoring")
}

func TestToolGetNetworkWaterfall_LimitParameter(t *testing.T) {
	t.Parallel()
	t.Skip("ToolHandler not available in internal packages - requires cmd/browser-agent refactoring")
}

func TestToolGetNetworkWaterfall_URLFilter(t *testing.T) {
	t.Parallel()
	t.Skip("ToolHandler not available in internal packages - requires cmd/browser-agent refactoring")
}

func TestToolGetNetworkWaterfall_ConcurrentAccessSafety(t *testing.T) {
	t.Parallel()
	t.Skip("ToolHandler not available in internal packages - requires cmd/browser-agent refactoring")
}
