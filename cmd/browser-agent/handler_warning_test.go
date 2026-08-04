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

func releaseCheckerForTag(t *testing.T, tag string) *versioncheck.Checker {
	t.Helper()
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
	return checker
}

func warningHandler(t *testing.T, tag string) *MCPHandler {
	t.Helper()
	handler := NewMCPHandler(nil, version)
	handler.runtime.SetReleaseChecker(releaseCheckerForTag(t, tag))
	return handler
}

func TestMaybeAddUpgradeWarning_NoPending(t *testing.T) {

	handler := warningHandler(t, "")

	resp := mcp.JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      1,
		Result:  mcp.TextResponse("hello"),
	}
	got := handler.maybeAddUpgradeWarning(resp)
	var result mcp.MCPToolResult
	if err := json.Unmarshal(got.Result, &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Content) != 1 || result.Content[0].Text != "hello" {
		t.Fatalf("expected unchanged response, got %+v", result)
	}
}

func TestMaybeAddUpdateAvailableWarning_NoUpdate(t *testing.T) {
	handler := warningHandler(t, "")

	resp := mcp.JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      1,
		Result:  mcp.TextResponse("hello"),
	}
	got := handler.maybeAddUpdateAvailableWarning(resp)
	var result mcp.MCPToolResult
	if err := json.Unmarshal(got.Result, &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Content) != 1 || result.Content[0].Text != "hello" {
		t.Fatalf("expected unchanged response, got %+v", result)
	}
}

func TestMaybeAddUpdateAvailableWarning_NewerAvailable(t *testing.T) {
	t.Parallel()
	handler := warningHandler(t, "v99.0.0")

	resp := mcp.JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      1,
		Result:  mcp.TextResponse("data"),
	}
	got := handler.maybeAddUpdateAvailableWarning(resp)
	var result mcp.MCPToolResult
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
	t.Parallel()
	handler := warningHandler(t, "v99.0.0")
	handler.runtime.SetUpdateLastShown(time.Now())

	resp := mcp.JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      1,
		Result:  mcp.TextResponse("data"),
	}
	got := handler.maybeAddUpdateAvailableWarning(resp)
	var result mcp.MCPToolResult
	if err := json.Unmarshal(got.Result, &result); err != nil {
		t.Fatal(err)
	}
	if result.Content[0].Text != "data" {
		t.Fatalf("expected unchanged response within cooldown, got %q", result.Content[0].Text)
	}
}

func TestUpdateWarningCooldownIsIsolatedPerApplicationRuntime(t *testing.T) {
	t.Parallel()
	first := warningHandler(t, "v99.0.0")
	second := warningHandler(t, "v99.0.0")
	response := mcp.JSONRPCResponse{JSONRPC: "2.0", ID: 1, Result: mcp.TextResponse("data")}

	first.maybeAddUpdateAvailableWarning(response)
	got := second.maybeAddUpdateAvailableWarning(response)

	var result mcp.MCPToolResult
	if err := json.Unmarshal(got.Result, &result); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Content[0].Text, "UPDATE AVAILABLE") {
		t.Fatal("one application runtime suppressed another runtime's warning")
	}
}

func TestMaybeAddUpdateAvailableWarning_SameVersionNoWarning(t *testing.T) {
	t.Parallel()
	handler := warningHandler(t, "v"+version)

	resp := mcp.JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      1,
		Result:  mcp.TextResponse("data"),
	}
	got := handler.maybeAddUpdateAvailableWarning(resp)
	var result mcp.MCPToolResult
	if err := json.Unmarshal(got.Result, &result); err != nil {
		t.Fatal(err)
	}
	if result.Content[0].Text != "data" {
		t.Fatalf("expected unchanged when same version, got %q", result.Content[0].Text)
	}
}

func TestMaybeAddUpgradeWarning_WithPending(t *testing.T) {
	t.Parallel()
	handler := warningHandler(t, "")
	handler.runtime.SetUpgrade(fixedUpgradeInfo{pending: true, version: "0.8.0", detectedAt: time.Now()})

	resp := mcp.JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      1,
		Result:  mcp.TextResponse("data here"),
	}
	got := handler.maybeAddUpgradeWarning(resp)
	var result mcp.MCPToolResult
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
