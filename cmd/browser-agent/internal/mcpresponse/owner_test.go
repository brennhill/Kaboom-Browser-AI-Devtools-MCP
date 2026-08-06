// owner_test.go — Tests MCP warning, version, security, and intent response policy.

package mcpresponse

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/appruntime"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/versioncheck"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/syncruntime"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

type fixedUpgrade struct {
	pending bool
	version string
	at      time.Time
}

func (upgrade fixedUpgrade) UpgradeInfo() (bool, string, time.Time) {
	return upgrade.pending, upgrade.version, upgrade.at
}

func TestOwnerAddsPendingAuditAndUpgradeWarnings(t *testing.T) {
	now := time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC)
	runtime := appruntime.New("0.9.0")
	runtime.SetUpgrade(fixedUpgrade{pending: true, version: "0.9.1", at: now.Add(-time.Minute)})
	pending := true
	owner := New(Config{
		Runtime: runtime, Now: func() time.Time { return now },
		PendingAudit: func() bool { value := pending; pending = false; return value },
	})

	response := owner.Augment(textResponse(), true)
	text := responseText(t, response)
	for _, expected := range []string{"ACTION REQUIRED", "/kaboom/audit", "NOTICE:", "0.9.1"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("response missing %q: %s", expected, text)
		}
	}
	second := owner.Augment(textResponse(), true)
	if strings.Contains(responseText(t, second), "ACTION REQUIRED") {
		t.Fatal("consumed pending intent was repeated")
	}
}

func TestOwnerAddsAvailableReleaseOncePerCooldown(t *testing.T) {
	now := time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC)
	releaseServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"tag_name":"v99.0.0"}`))
	}))
	defer releaseServer.Close()
	checker := versioncheck.New(versioncheck.Options{
		CurrentVersion: "0.9.0", ReleaseURL: releaseServer.URL, HTTPClient: releaseServer.Client(), Now: func() time.Time { return now },
	})
	checker.Check()
	runtime := appruntime.New("0.9.0")
	runtime.SetReleaseChecker(checker)
	owner := New(Config{Runtime: runtime, Now: func() time.Time { return now }})

	if text := responseText(t, owner.addUpdateWarning(textResponse())); !strings.Contains(text, "UPDATE AVAILABLE") {
		t.Fatalf("update warning missing: %s", text)
	}
	if text := responseText(t, owner.addUpdateWarning(textResponse())); strings.Contains(text, "UPDATE AVAILABLE") {
		t.Fatal("update warning ignored cooldown")
	}
}

func TestOwnerWarnsUnknownArgumentsInStableOrder(t *testing.T) {
	var warnings []string
	owner := New(Config{AddWarning: func(value string) { warnings = append(warnings, value) }})
	schemas := []mcp.MCPTool{{Name: "observe", InputSchema: map[string]any{
		"properties": map[string]any{"what": map[string]any{}},
	}}}
	owner.WarnUnknownArguments("observe", json.RawMessage(`{"z":1,"what":"logs","a":2}`), schemas)
	if len(warnings) != 2 || !strings.Contains(warnings[0], "'a'") || !strings.Contains(warnings[1], "'z'") {
		t.Fatalf("warnings = %#v", warnings)
	}
}

func TestOwnerAddsAlteredEnvironmentWarningAndMetadata(t *testing.T) {
	captured := capture.NewCapture()
	captured.Extension().SetSecurityMode(syncruntime.SecurityModeInsecureProxy, []string{"csp_headers"})
	owner := New(Config{})
	owner.SetCapture(captured)

	response := owner.Augment(textResponse(), true)
	if text := responseText(t, response); !strings.Contains(text, "[ALTERED ENVIRONMENT]") {
		t.Fatalf("security warning missing: %s", text)
	}
	var result mcp.MCPToolResult
	if err := json.Unmarshal(response.Result, &result); err != nil {
		t.Fatal(err)
	}
	if result.Metadata["security_mode"] != syncruntime.SecurityModeInsecureProxy {
		t.Fatalf("security_mode = %#v", result.Metadata["security_mode"])
	}
	if productionParity, ok := result.Metadata["production_parity"].(bool); !ok || productionParity {
		t.Fatalf("production_parity = %#v", result.Metadata["production_parity"])
	}
	rewrites, ok := result.Metadata["insecure_rewrites_applied"].([]any)
	if !ok || len(rewrites) != 1 || rewrites[0] != "csp_headers" {
		t.Fatalf("insecure_rewrites_applied = %#v", result.Metadata["insecure_rewrites_applied"])
	}
}

func textResponse() mcp.JSONRPCResponse {
	return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: 1, Result: mcp.TextResponse("base")}
}

func responseText(t *testing.T, response mcp.JSONRPCResponse) string {
	t.Helper()
	var result mcp.MCPToolResult
	if err := json.Unmarshal(response.Result, &result); err != nil {
		t.Fatal(err)
	}
	return result.Content[0].Text
}
