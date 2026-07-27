// Purpose: Tests for handler warning message injection.
// Docs: docs/features/feature/mcp-persistent-server/index.md

// handler_warning_test.go — Tests for upgrade/update warning injection into MCP tool responses.
package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/versioncheck"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

func useReleaseChecker(t *testing.T, tag string) {
	t.Helper()
	original := releaseChecker
	checker := versioncheck.New(versioncheck.Options{CurrentVersion: version})
	if tag != "" {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"tag_name":"` + tag + `"}`))
		}))
		checker = versioncheck.New(versioncheck.Options{
			CurrentVersion: version, ReleaseURL: server.URL, HTTPClient: server.Client(),
		})
		checker.Check()
		server.Close()
	}
	releaseChecker = checker
	t.Cleanup(func() { releaseChecker = original })
}

func TestMaybeAddUpgradeWarning_NoPending(t *testing.T) {

	// No upgrade state set — response should pass through unchanged.
	orig := binaryUpgradeState
	binaryUpgradeState = nil
	defer func() { binaryUpgradeState = orig }()

	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      1,
		Result:  mcp.TextResponse("hello"),
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
	useReleaseChecker(t, "")

	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      1,
		Result:  mcp.TextResponse("hello"),
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
	useReleaseChecker(t, "v99.0.0")

	origLastNotify := updateNotifyLastShown
	updateNotifyLastShown = time.Time{} // reset cooldown

	defer func() {
		updateNotifyLastShown = origLastNotify
	}()

	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      1,
		Result:  mcp.TextResponse("data"),
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
	useReleaseChecker(t, "v99.0.0")

	// Set last shown to now — should suppress the warning
	origLastNotify := updateNotifyLastShown
	updateNotifyLastShown = time.Now()

	defer func() {
		updateNotifyLastShown = origLastNotify
	}()

	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      1,
		Result:  mcp.TextResponse("data"),
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
	useReleaseChecker(t, "v"+version)

	origLastNotify := updateNotifyLastShown
	updateNotifyLastShown = time.Time{}

	defer func() {
		updateNotifyLastShown = origLastNotify
	}()

	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      1,
		Result:  mcp.TextResponse("data"),
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

	binaryUpgradeState = fixedUpgradeInfo{pending: true, version: "0.8.0", detectedAt: time.Now()}

	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      1,
		Result:  mcp.TextResponse("data here"),
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

type fixedUpgradeInfo struct {
	pending    bool
	version    string
	detectedAt time.Time
}

func (f fixedUpgradeInfo) UpgradeInfo() (bool, string, time.Time) {
	return f.pending, f.version, f.detectedAt
}
