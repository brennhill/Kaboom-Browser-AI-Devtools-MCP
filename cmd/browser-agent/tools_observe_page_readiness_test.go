// tools_observe_page_readiness_test.go — Tests observe pilot and page readiness.
// Docs: docs/features/feature/observe/index.md
package main

import (
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capturefixture"
)

func TestToolsObservePilot_ResponseFields(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callObserveRaw(h, "pilot")
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("pilot should not error, got: %s", result.Content[0].Text)
	}

	// Pilot is a server-side mode, should not get disconnect warning
	text := result.Content[0].Text
	if strings.Contains(text, "Extension is not connected") {
		t.Error("pilot is server-side mode, should NOT get disconnect warning")
	}
}

// ============================================
// observe(what:"page") — page_ready_for_commands Tests
// ============================================

func TestToolsObservePage_PageReadyForCommands_AllConditionsMet(t *testing.T) {
	t.Parallel()
	h, _, cap := makeToolHandler(t)

	// Set up all conditions for page_ready_for_commands=true
	capturefixture.Connect(cap)
	capturefixture.SetPilot(cap, true)
	capturefixture.Track(cap, 42, "https://example.com")
	capturefixture.SetTabStatus(cap, "complete")

	resp := callObserveRaw(h, "page")
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("page should not error, got: %s", result.Content[0].Text)
	}
	data := extractResultJSON(t, result)

	ready, ok := data["page_ready_for_commands"]
	if !ok {
		t.Fatal("response missing 'page_ready_for_commands' field")
	}
	if ready != true {
		t.Errorf("page_ready_for_commands = %v, want true", ready)
	}

	tabStatus, ok := data["tab_status"]
	if !ok {
		t.Fatal("response missing 'tab_status' field")
	}
	if tabStatus != "complete" {
		t.Errorf("tab_status = %v, want 'complete'", tabStatus)
	}
}

func TestToolsObservePage_PageReadyForCommands_ExtensionDisconnected(t *testing.T) {
	t.Parallel()
	h, _, cap := makeToolHandler(t)

	capturefixture.SetPilot(cap, true)
	capturefixture.Track(cap, 42, "https://example.com")
	capturefixture.SetTabStatus(cap, "complete")
	capturefixture.Disconnect(cap)

	resp := callObserveRaw(h, "page")
	result := parseToolResult(t, resp)
	data := extractResultJSON(t, result)

	if data["page_ready_for_commands"] != false {
		t.Errorf("page_ready_for_commands = %v, want false (extension disconnected)", data["page_ready_for_commands"])
	}
}

func TestToolsObservePage_PageReadyForCommands_PilotDisabled(t *testing.T) {
	t.Parallel()
	h, _, cap := makeToolHandler(t)

	capturefixture.Connect(cap)
	capturefixture.SetPilot(cap, false)
	capturefixture.Track(cap, 42, "https://example.com")
	capturefixture.SetTabStatus(cap, "complete")

	resp := callObserveRaw(h, "page")
	result := parseToolResult(t, resp)
	data := extractResultJSON(t, result)

	if data["page_ready_for_commands"] != false {
		t.Errorf("page_ready_for_commands = %v, want false (pilot disabled)", data["page_ready_for_commands"])
	}
}

func TestToolsObservePage_PageReadyForCommands_NoTrackedTab(t *testing.T) {
	t.Parallel()
	h, _, cap := makeToolHandler(t)

	capturefixture.Connect(cap)
	capturefixture.SetPilot(cap, true)
	// No tracked tab set
	capturefixture.SetTabStatus(cap, "complete")

	resp := callObserveRaw(h, "page")
	result := parseToolResult(t, resp)
	data := extractResultJSON(t, result)

	if data["page_ready_for_commands"] != false {
		t.Errorf("page_ready_for_commands = %v, want false (no tracked tab)", data["page_ready_for_commands"])
	}
}

func TestToolsObservePage_PageReadyForCommands_TabLoading(t *testing.T) {
	t.Parallel()
	h, _, cap := makeToolHandler(t)

	capturefixture.Connect(cap)
	capturefixture.SetPilot(cap, true)
	capturefixture.Track(cap, 42, "https://example.com")
	capturefixture.SetTabStatus(cap, "loading")

	resp := callObserveRaw(h, "page")
	result := parseToolResult(t, resp)
	data := extractResultJSON(t, result)

	if data["page_ready_for_commands"] != false {
		t.Errorf("page_ready_for_commands = %v, want false (tab loading)", data["page_ready_for_commands"])
	}
	if data["tab_status"] != "loading" {
		t.Errorf("tab_status = %v, want 'loading'", data["tab_status"])
	}
}

func TestToolsObservePage_DataAgeMs_Present(t *testing.T) {
	t.Parallel()
	h, _, cap := makeToolHandler(t)
	capturefixture.Connect(cap)

	resp := callObserveRaw(h, "page")
	result := parseToolResult(t, resp)
	data := extractResultJSON(t, result)

	meta, ok := data["metadata"].(map[string]any)
	if !ok {
		t.Fatal("metadata should be a map")
	}
	if _, ok := meta["data_age_ms"]; !ok {
		t.Error("metadata missing 'data_age_ms' field")
	}
}
