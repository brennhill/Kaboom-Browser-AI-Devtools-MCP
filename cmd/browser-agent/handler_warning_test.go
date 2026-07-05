// Purpose: Tests for handler warning message injection.
// Docs: docs/features/feature/mcp-persistent-server/index.md

// handler_warning_test.go — Tests for upgrade/update warning injection into MCP tool responses.
package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestMaybeAddUpgradeWarning_NoPending(t *testing.T) {

	// No upgrade state set — response should pass through unchanged.
	orig := binaryUpgradeState
	binaryUpgradeState = nil
	defer func() { binaryUpgradeState = orig }()

	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      1,
		Result:  mcpTextResponse("hello"),
	}
	got := maybeAddUpgradeWarning(resp)
	var result MCPToolResult
	if err := json.Unmarshal(got.Result, &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Content) != 1 || result.Content[0].Text != "hello" {
		t.Fatalf("expected unchanged response, got %+v", result)
	}
}

func TestMaybeAddUpdateAvailableWarning_NoUpdate(t *testing.T) {
	// Not parallel: modifies package-level versionCheckMu-protected state
	versionCheckMu.Lock()
	origVer := availableVersion
	availableVersion = ""
	versionCheckMu.Unlock()
	defer func() {
		versionCheckMu.Lock()
		availableVersion = origVer
		versionCheckMu.Unlock()
	}()

	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      1,
		Result:  mcpTextResponse("hello"),
	}
	got := maybeAddUpdateAvailableWarning(resp)
	var result MCPToolResult
	if err := json.Unmarshal(got.Result, &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Content) != 1 || result.Content[0].Text != "hello" {
		t.Fatalf("expected unchanged response, got %+v", result)
	}
}

func TestMaybeAddUpdateAvailableWarning_NewerAvailable(t *testing.T) {
	// Not parallel: modifies package-level state
	versionCheckMu.Lock()
	origVer := availableVersion
	availableVersion = "99.0.0"
	versionCheckMu.Unlock()

	origLastNotify := updateNotifyLastShown
	updateNotifyLastShown = time.Time{} // reset cooldown

	defer func() {
		versionCheckMu.Lock()
		availableVersion = origVer
		versionCheckMu.Unlock()
		updateNotifyLastShown = origLastNotify
	}()

	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      1,
		Result:  mcpTextResponse("data"),
	}
	got := maybeAddUpdateAvailableWarning(resp)
	var result MCPToolResult
	if err := json.Unmarshal(got.Result, &result); err != nil {
		t.Fatal(err)
	}
	text := result.Content[0].Text
	if !strings.Contains(text, "UPDATE AVAILABLE") || !strings.Contains(text, "99.0.0") {
		t.Fatalf("expected update notice, got %q", text)
	}
	if !strings.Contains(text, "npm install -g kaboom-agentic-browser@latest") {
		t.Fatalf("expected install command, got %q", text)
	}
}

func TestMaybeAddUpdateAvailableWarning_DailyCooldown(t *testing.T) {
	// Not parallel: modifies package-level state
	versionCheckMu.Lock()
	origVer := availableVersion
	availableVersion = "99.0.0"
	versionCheckMu.Unlock()

	// Set last shown to now — should suppress the warning
	origLastNotify := updateNotifyLastShown
	updateNotifyLastShown = time.Now()

	defer func() {
		versionCheckMu.Lock()
		availableVersion = origVer
		versionCheckMu.Unlock()
		updateNotifyLastShown = origLastNotify
	}()

	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      1,
		Result:  mcpTextResponse("data"),
	}
	got := maybeAddUpdateAvailableWarning(resp)
	var result MCPToolResult
	if err := json.Unmarshal(got.Result, &result); err != nil {
		t.Fatal(err)
	}
	if result.Content[0].Text != "data" {
		t.Fatalf("expected unchanged response within cooldown, got %q", result.Content[0].Text)
	}
}

func TestMaybeAddUpdateAvailableWarning_SameVersionNoWarning(t *testing.T) {
	// Not parallel: modifies package-level state
	versionCheckMu.Lock()
	origVer := availableVersion
	availableVersion = version // same as current
	versionCheckMu.Unlock()

	origLastNotify := updateNotifyLastShown
	updateNotifyLastShown = time.Time{}

	defer func() {
		versionCheckMu.Lock()
		availableVersion = origVer
		versionCheckMu.Unlock()
		updateNotifyLastShown = origLastNotify
	}()

	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      1,
		Result:  mcpTextResponse("data"),
	}
	got := maybeAddUpdateAvailableWarning(resp)
	var result MCPToolResult
	if err := json.Unmarshal(got.Result, &result); err != nil {
		t.Fatal(err)
	}
	if result.Content[0].Text != "data" {
		t.Fatalf("expected unchanged when same version, got %q", result.Content[0].Text)
	}
}

func TestMaybeAddUpgradeWarning_WithPending(t *testing.T) {
	// Not parallel: modifies package-level binaryUpgradeState
	orig := binaryUpgradeState
	defer func() { binaryUpgradeState = orig }()

	state := &BinaryWatcherState{}
	state.mu.Lock()
	state.upgradePending = true
	state.detectedVersion = "0.8.0"
	state.detectedAt = time.Now()
	state.mu.Unlock()
	binaryUpgradeState = state

	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      1,
		Result:  mcpTextResponse("data here"),
	}
	got := maybeAddUpgradeWarning(resp)
	var result MCPToolResult
	if err := json.Unmarshal(got.Result, &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Content) < 1 {
		t.Fatal("expected content")
	}
	text := result.Content[0].Text
	if !strings.Contains(text, "NOTICE:") || !strings.Contains(text, "0.8.0") {
		t.Fatalf("expected upgrade notice, got %q", text)
	}
	if !strings.Contains(text, "data here") {
		t.Fatalf("expected original content preserved, got %q", text)
	}
}
