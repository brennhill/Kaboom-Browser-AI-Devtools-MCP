// tutorial_test.go — Unit tests for tutorial context, diagnostics, snippets, and playbooks.

package tutorial

import (
	"encoding/json"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

// fakeTutorialDeps implements Deps.
type fakeTutorialDeps struct {
	trackingEnabled bool
	tabID           int
	tabURL          string
	pilotStatus     any
	extConnected    bool
}

func (f *fakeTutorialDeps) GetTrackingStatus() (bool, int, string) {
	return f.trackingEnabled, f.tabID, f.tabURL
}
func (f *fakeTutorialDeps) GetPilotStatus() any        { return f.pilotStatus }
func (f *fakeTutorialDeps) IsExtensionConnected() bool { return f.extConnected }

// parseResp decodes an MCP tool result into (isError, text).
func parseResp(t *testing.T, resp mcp.JSONRPCResponse) (bool, string) {
	t.Helper()
	var r struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if err := json.Unmarshal(resp.Result, &r); err != nil {
		t.Fatalf("failed to unmarshal result: %v (raw=%s)", err, string(resp.Result))
	}
	text := ""
	if len(r.Content) > 0 {
		text = r.Content[0].Text
	}
	return r.IsError, text
}

func newReq() mcp.JSONRPCRequest { return mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1} }

// ---------------------------------------------------------------------------
// HandleTutorial + TutorialContext / TutorialIssues / TutorialNextSteps
// ---------------------------------------------------------------------------

func TestHandleTutorial(t *testing.T) {
	d := &fakeTutorialDeps{
		extConnected:    true,
		trackingEnabled: true,
		tabID:           5,
		tabURL:          "https://example.com",
		pilotStatus:     map[string]any{"enabled": true, "state": "enabled", "authoritative": true},
	}
	playbooks := map[string]any{"foo": "bar"}

	for _, what := range []string{"tutorial", "examples"} {
		resp := HandleTutorial(d, newReq(), json.RawMessage(`{"what":"`+what+`"}`), playbooks)
		if isErr, text := parseResp(t, resp); isErr {
			t.Fatalf("HandleTutorial(%s) should succeed: %s", what, text)
		}
	}
}

func TestTutorialContext_NilDeps(t *testing.T) {
	ctx := TutorialContext(nil)
	if ctx["extension_connected"] != false {
		t.Error("nil deps should default extension_connected to false")
	}
	if ctx["pilot_enabled"] != true {
		t.Error("nil deps should default pilot_enabled to true")
	}
}

func TestTutorialContext_WithDeps(t *testing.T) {
	d := &fakeTutorialDeps{
		extConnected:    true,
		trackingEnabled: true,
		tabID:           9,
		tabURL:          "https://x.test",
		pilotStatus:     map[string]any{"enabled": false, "state": "explicitly_disabled", "authoritative": true},
	}
	ctx := TutorialContext(d)
	if ctx["extension_connected"] != true {
		t.Error("extension_connected should reflect deps")
	}
	if ctx["pilot_enabled"] != false {
		t.Error("pilot_enabled should reflect status map")
	}
	if ctx["pilot_state"] != "explicitly_disabled" {
		t.Errorf("pilot_state = %v", ctx["pilot_state"])
	}
	if ctx["tracked_tab_id"] != 9 {
		t.Errorf("tracked_tab_id = %v", ctx["tracked_tab_id"])
	}
}

func TestTutorialContext_NonMapPilotStatus(t *testing.T) {
	d := &fakeTutorialDeps{pilotStatus: "not-a-map"}
	ctx := TutorialContext(d)
	// Falls back to defaults for pilot fields.
	if ctx["pilot_enabled"] != true {
		t.Error("non-map pilot status should keep default pilot_enabled")
	}
}

func TestTutorialIssues(t *testing.T) {
	tests := []struct {
		name     string
		ctx      map[string]any
		wantCode string
	}{
		{
			name:     "pilot disabled",
			ctx:      map[string]any{"pilot_enabled": false, "pilot_state": "explicitly_disabled"},
			wantCode: "pilot_disabled",
		},
		{
			name:     "extension disconnected",
			ctx:      map[string]any{"pilot_enabled": true, "pilot_state": "enabled", "extension_connected": false},
			wantCode: "extension_disconnected",
		},
		{
			name: "no tracked tab",
			ctx: map[string]any{
				"pilot_enabled": true, "pilot_state": "enabled",
				"extension_connected": true, "tracking_enabled": false,
				"tracked_tab_id": 0, "tracked_tab_url": "",
			},
			wantCode: "no_tracked_tab",
		},
		{
			name: "all good yields no issues",
			ctx: map[string]any{
				"pilot_enabled": true, "pilot_state": "enabled",
				"extension_connected": true, "tracking_enabled": true,
				"tracked_tab_id": 3, "tracked_tab_url": "https://ok.test",
			},
			wantCode: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issues := TutorialIssues(tt.ctx)
			if tt.wantCode == "" {
				if len(issues) != 0 {
					t.Fatalf("expected no issues, got %v", issues)
				}
				return
			}
			if len(issues) == 0 {
				t.Fatalf("expected an issue with code %q", tt.wantCode)
			}
			if issues[0]["code"] != tt.wantCode {
				t.Errorf("code = %v, want %v", issues[0]["code"], tt.wantCode)
			}
		})
	}
}

func TestTutorialNextSteps(t *testing.T) {
	withIssue := map[string]any{"pilot_enabled": false, "pilot_state": "explicitly_disabled"}
	steps := TutorialNextSteps(withIssue)
	if len(steps) == 0 {
		t.Fatal("expected next steps")
	}
	if steps[0] != "Run configure doctor to verify environment status" {
		t.Errorf("unexpected first step for issue context: %q", steps[0])
	}

	healthy := map[string]any{
		"pilot_enabled": true, "pilot_state": "enabled",
		"extension_connected": true, "tracking_enabled": true,
		"tracked_tab_id": 1, "tracked_tab_url": "https://ok.test",
	}
	steps2 := TutorialNextSteps(healthy)
	if steps2[0] != "Run observe errors to inspect current page issues" {
		t.Errorf("unexpected first step for healthy context: %q", steps2[0])
	}
}

// ---------------------------------------------------------------------------
// Tutorial snippet catalog
// ---------------------------------------------------------------------------

func TestTutorialSnippets_NonEmpty(t *testing.T) {
	snippets := TutorialSnippets()
	if len(snippets) == 0 {
		t.Fatal("TutorialSnippets should return a non-empty slice")
	}
}

func TestTutorialSnippets_NoDuplicateGoals(t *testing.T) {
	snippets := TutorialSnippets()
	seen := make(map[string]bool)
	for _, s := range snippets {
		goal, ok := s["goal"].(string)
		if !ok || goal == "" {
			t.Error("snippet missing 'goal' string field")
			continue
		}
		if seen[goal] {
			t.Errorf("duplicate snippet goal: %s", goal)
		}
		seen[goal] = true
	}
}

func TestTutorialSnippets_RequiredFields(t *testing.T) {
	snippets := TutorialSnippets()
	for i, s := range snippets {
		for _, key := range []string{"tool", "goal", "snippet", "arguments"} {
			if _, ok := s[key]; !ok {
				t.Errorf("snippet %d missing required field %q", i, key)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Tutorial playbooks
// ---------------------------------------------------------------------------

func TestTutorialSafeAutomationLoop_NonEmpty(t *testing.T) {
	playbook := TutorialSafeAutomationLoop()
	if len(playbook) == 0 {
		t.Fatal("TutorialSafeAutomationLoop should return a non-empty map")
	}
	if _, ok := playbook["title"]; !ok {
		t.Error("playbook should have a 'title' field")
	}
}

func TestTutorialCSPFallbackPlaybook_NonEmpty(t *testing.T) {
	playbook := TutorialCSPFallbackPlaybook()
	if len(playbook) == 0 {
		t.Fatal("TutorialCSPFallbackPlaybook should return a non-empty map")
	}
}
