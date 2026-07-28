// Purpose: Smoke tests for data_age_ms metadata across observe modes (Stream 6, PR #329).
// Why: Validates that data_age_ms is present, numeric, and correct in handler-level responses.
// Docs: docs/features/feature/mcp-persistent-server/index.md

// tools_observe_data_age_smoke_test.go — Smoke tests for observe data_age_ms metadata.
package main

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"strings"
	"testing"
	"time"
)

// ============================================
// Smoke Tests: data_age_ms metadata
// ============================================

func TestSmoke_ObservePage_DataAgeMs_IsNumeric(t *testing.T) {
	t.Parallel()
	h, _, cap := makeToolHandler(t)
	cap.SimulateExtensionConnectForTest()
	cap.SetPilotEnabled(true)
	cap.SetTrackingStatusForTest(42, "https://example.com")
	cap.SetTabStatusForTest("complete")

	resp := callObserveRaw(h, "page")
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("page should not error, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	meta, ok := data["metadata"].(map[string]any)
	if !ok {
		t.Fatal("metadata should be a map")
	}

	ageRaw, exists := meta["data_age_ms"]
	if !exists {
		t.Fatal("metadata missing 'data_age_ms' field")
	}

	ageMs, ok := ageRaw.(float64) // JSON numbers are float64
	if !ok {
		t.Fatalf("data_age_ms should be numeric (float64), got %T", ageRaw)
	}
	if ageMs < 0 && ageMs != -1 {
		t.Errorf("data_age_ms = %v, want >= 0 or -1 sentinel", ageMs)
	}
}

func TestSmoke_ObserveErrors_DataAgeMs_IsNumeric(t *testing.T) {
	t.Parallel()
	h, server, _ := makeToolHandler(t)

	ts := time.Now().UTC().Format(time.RFC3339)
	server.logs.SeedEntries([]types.LogEntry{
		types.LogEntry{
			"level": "error", "message": "Test error for data_age_ms", "ts": ts,
		},
	}, nil)

	resp := callObserveRaw(h, "errors")
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("errors should not error, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	meta, ok := data["metadata"].(map[string]any)
	if !ok {
		t.Fatal("metadata should be a map")
	}

	ageRaw, exists := meta["data_age_ms"]
	if !exists {
		t.Fatal("metadata missing 'data_age_ms' field")
	}

	ageMs, ok := ageRaw.(float64)
	if !ok {
		t.Fatalf("data_age_ms should be numeric (float64), got %T", ageRaw)
	}
	// Data was just populated — should NOT be -1 sentinel.
	if ageMs == -1 {
		t.Error("data_age_ms should not be -1 sentinel when data is populated")
	}
	if ageMs < 0 {
		t.Errorf("data_age_ms = %v, want >= 0 for populated data", ageMs)
	}
}

func TestSmoke_ObserveNetworkBodies_DataAgeMs_Present(t *testing.T) {
	t.Parallel()
	h, _, cap := makeToolHandler(t)

	cap.AddNetworkBodiesForTest([]types.NetworkBody{
		{
			URL:       "https://api.example.com/smoke",
			Method:    "GET",
			Status:    200,
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		},
	})

	resp := callObserveRaw(h, "network_bodies")
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("network_bodies should not error, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	meta, ok := data["metadata"].(map[string]any)
	if !ok {
		t.Fatal("metadata should be a map")
	}

	ageRaw, exists := meta["data_age_ms"]
	if !exists {
		t.Fatal("metadata missing 'data_age_ms' field for network_bodies")
	}

	ageMs, ok := ageRaw.(float64)
	if !ok {
		t.Fatalf("data_age_ms should be numeric (float64), got %T", ageRaw)
	}
	if ageMs < 0 {
		t.Errorf("data_age_ms = %v, want >= 0 for populated network_bodies", ageMs)
	}
}

func TestSmoke_ObserveActions_DataAgeMs_Present(t *testing.T) {
	t.Parallel()
	h, _, cap := makeToolHandler(t)

	cap.AddEnhancedActionsForTest([]types.EnhancedAction{
		{Type: "click", Timestamp: time.Now().UnixMilli(), URL: "https://example.com/smoke"},
	})

	resp := callObserveRaw(h, "actions")
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("actions should not error, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	meta, ok := data["metadata"].(map[string]any)
	if !ok {
		t.Fatal("metadata should be a map")
	}

	ageRaw, exists := meta["data_age_ms"]
	if !exists {
		t.Fatal("metadata missing 'data_age_ms' field for actions")
	}

	ageMs, ok := ageRaw.(float64)
	if !ok {
		t.Fatalf("data_age_ms should be numeric (float64), got %T", ageRaw)
	}
	if ageMs < 0 {
		t.Errorf("data_age_ms = %v, want >= 0 for populated actions", ageMs)
	}
}

func TestSmoke_ObserveErrors_DataAgeMs_NoData_Sentinel(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	// Empty buffer — no errors added
	resp := callObserveRaw(h, "errors")
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("errors with empty buffer should not error, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	meta, ok := data["metadata"].(map[string]any)
	if !ok {
		t.Fatal("metadata should be a map")
	}

	ageRaw, exists := meta["data_age_ms"]
	if !exists {
		t.Fatal("metadata missing 'data_age_ms' field")
	}

	ageMs, ok := ageRaw.(float64)
	if !ok {
		t.Fatalf("data_age_ms should be numeric (float64), got %T", ageRaw)
	}
	if ageMs != -1 {
		t.Errorf("data_age_ms = %v, want -1 sentinel when no data exists", ageMs)
	}
}

func TestSmoke_ObservePage_DataAgeMs_RecentValue(t *testing.T) {
	t.Parallel()
	h, _, cap := makeToolHandler(t)
	cap.SimulateExtensionConnectForTest()
	cap.SetPilotEnabled(true)
	cap.SetTrackingStatusForTest(42, "https://example.com")
	cap.SetTabStatusForTest("complete")

	resp := callObserveRaw(h, "page")
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("page should not error, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	meta, ok := data["metadata"].(map[string]any)
	if !ok {
		t.Fatal("metadata should be a map")
	}

	ageRaw, exists := meta["data_age_ms"]
	if !exists {
		t.Fatal("metadata missing 'data_age_ms' field")
	}

	ageMs, ok := ageRaw.(float64)
	if !ok {
		t.Fatalf("data_age_ms should be numeric (float64), got %T", ageRaw)
	}

	// Data was just set up — should be less than 5000ms.
	// The -1 sentinel is acceptable if page has no timestamped data, but
	// if positive, it must be recent.
	if ageMs >= 0 && ageMs > 5000 {
		t.Errorf("data_age_ms = %v, want < 5000 for recently created data", ageMs)
	}

	// Verify snake_case throughout
	assertSnakeCaseFields(t, string(resp.Result))

	// Verify no camelCase data_age field leaks
	text := result.Content[0].Text
	if strings.Contains(text, "dataAge") && !strings.Contains(text, "data_age") {
		t.Error("found camelCase 'dataAge' — all fields should use snake_case")
	}
}

// ============================================
// Smoke Tests: is_active tab focus metadata
// ============================================

func TestSmoke_ObservePage_IsActive_PresentWhenKnown(t *testing.T) {
	t.Parallel()
	h, _, cap := makeToolHandler(t)
	cap.SimulateExtensionConnectForTest()
	cap.SetPilotEnabled(true)
	cap.SetTrackingStatusForTest(42, "https://example.com")
	cap.SetTabStatusForTest("complete")
	cap.SetTrackedTabActiveForTest(true)

	resp := callObserveRaw(h, "page")
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("page should not error, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	isActive, ok := data["is_active"]
	if !ok {
		t.Fatal("is_active should be present when tab active state is known")
	}
	if isActive != true {
		t.Errorf("is_active = %v, want true", isActive)
	}
}

func TestSmoke_ObservePage_IsActive_FalseWhenInactive(t *testing.T) {
	t.Parallel()
	h, _, cap := makeToolHandler(t)
	cap.SimulateExtensionConnectForTest()
	cap.SetPilotEnabled(true)
	cap.SetTrackingStatusForTest(42, "https://example.com")
	cap.SetTabStatusForTest("complete")
	cap.SetTrackedTabActiveForTest(false)

	resp := callObserveRaw(h, "page")
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("page should not error, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	isActive, ok := data["is_active"]
	if !ok {
		t.Fatal("is_active should be present when tab active state is known")
	}
	if isActive != false {
		t.Errorf("is_active = %v, want false", isActive)
	}
}

func TestSmoke_ObservePage_IsActive_AbsentWhenUnknown(t *testing.T) {
	t.Parallel()
	h, _, cap := makeToolHandler(t)
	cap.SimulateExtensionConnectForTest()
	cap.SetPilotEnabled(true)
	cap.SetTrackingStatusForTest(42, "https://example.com")
	cap.SetTabStatusForTest("complete")
	// Do NOT call SetTrackedTabActiveForTest — state is unknown

	resp := callObserveRaw(h, "page")
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("page should not error, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	if _, ok := data["is_active"]; ok {
		t.Error("is_active should NOT be present when tab active state is unknown")
	}
}
