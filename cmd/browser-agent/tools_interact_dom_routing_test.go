// tools_interact_dom_routing_test.go — Tests DOM primitive parameter routing.
// Docs: docs/features/feature/interact-explore/index.md

package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capturefixture"
)

// ============================================
// interact DOM primitives — Parameter Validation
// ============================================

func TestToolsInteractDOMPrimitives_MissingSelector(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	actions := []string{"click", "type", "select", "check", "get_text", "get_value",
		"get_attribute", "set_attribute", "focus", "scroll_to", "wait_for"}

	for _, action := range actions {
		t.Run(action, func(t *testing.T) {
			resp := callInteractRaw(h, `{"what":"`+action+`"}`)
			result := parseToolResult(t, resp)
			if !result.IsError {
				t.Fatalf("%s without selector should return isError:true", action)
			}
			if !strings.Contains(result.Content[0].Text, "selector") {
				t.Errorf("%s error should mention 'selector', got: %s", action, result.Content[0].Text)
			}
		})
	}
}

func TestToolsInteractDOMPrimitives_IntentActions_NoSelector(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	actions := []string{
		"open_composer",
		"submit_active_composer",
		"confirm_top_dialog",
		"dismiss_top_overlay",
	}

	for _, action := range actions {
		t.Run(action, func(t *testing.T) {
			resp := callInteractRaw(h, `{"what":"`+action+`"}`)
			result := parseToolResult(t, resp)
			if !result.IsError {
				t.Fatalf("%s without selector should still error while pilot is disabled", action)
			}
			if strings.Contains(strings.ToLower(result.Content[0].Text), "selector") {
				t.Errorf("%s should not fail with selector-missing guidance: %s", action, result.Content[0].Text)
			}
		})
	}
}

func TestToolsInteractDOMPrimitives_ActionSpecificParams(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	// Actions that require specific params beyond selector
	cases := []struct {
		action  string
		args    string
		missing string
	}{
		{"type", `{"what":"type","selector":"input"}`, "text"},
		{"select", `{"what":"select","selector":"select"}`, "value"},
		{"get_attribute", `{"what":"get_attribute","selector":"div"}`, "name"},
		{"set_attribute", `{"what":"set_attribute","selector":"div"}`, "name"},
	}

	for _, tc := range cases {
		t.Run(tc.action+"_missing_"+tc.missing, func(t *testing.T) {
			resp := callInteractRaw(h, tc.args)
			result := parseToolResult(t, resp)
			if !result.IsError {
				t.Fatalf("%s without %s should return isError:true", tc.action, tc.missing)
			}
			if !strings.Contains(result.Content[0].Text, tc.missing) {
				t.Errorf("error should mention missing %q param, got: %s", tc.missing, result.Content[0].Text)
			}
		})
	}
}

func TestToolsInteractDOMPrimitives_SuccessWithPilot(t *testing.T) {
	t.Parallel()

	cases := []struct {
		action string
		args   string
	}{
		{"click", `{"what":"click","selector":"#btn"}`},
		{"type", `{"what":"type","selector":"input","text":"hello"}`},
		{"select", `{"what":"select","selector":"select","value":"opt1"}`},
		{"check", `{"what":"check","selector":"input[type=checkbox]"}`},
		{"get_text", `{"what":"get_text","selector":"div"}`},
		{"get_value", `{"what":"get_value","selector":"input"}`},
		{"get_attribute", `{"what":"get_attribute","selector":"a","name":"href"}`},
		{"set_attribute", `{"what":"set_attribute","selector":"div","name":"data-test","value":"1"}`},
		{"focus", `{"what":"focus","selector":"input"}`},
		{"scroll_to", `{"what":"scroll_to","selector":"#footer"}`},
		{"wait_for", `{"what":"wait_for","selector":"#spinner"}`},
		{"key_press", `{"what":"key_press","selector":"input","text":"Enter"}`},
		{"open_composer", `{"what":"open_composer"}`},
		{"submit_active_composer", `{"what":"submit_active_composer"}`},
		{"confirm_top_dialog", `{"what":"confirm_top_dialog"}`},
		{"dismiss_top_overlay", `{"what":"dismiss_top_overlay"}`},
	}

	for _, tc := range cases {
		t.Run(tc.action, func(t *testing.T) {
			h, _, cap := makeToolHandler(t)
			capturefixture.SetPilot(cap, true)
			mockConnectedTrackedTab(t, cap)

			resp := callInteractRaw(h, tc.args)
			result := parseToolResult(t, resp)
			if result.IsError {
				t.Fatalf("%s should succeed with pilot enabled, got: %s", tc.action, result.Content[0].Text)
			}

			data := extractResultJSON(t, result)
			if data["status"] != "queued" {
				t.Errorf("status = %v, want 'queued'", data["status"])
			}
			corr, _ := data["correlation_id"].(string)
			if !strings.HasPrefix(corr, "dom_") {
				t.Errorf("correlation_id should start with 'dom_', got: %s", corr)
			}

			pq := lastPendingQuerySnapshot(cap.Queries())
			if pq == nil {
				t.Fatalf("expected pending query for %s", tc.action)
			}
			if !strings.Contains(string(pq.Params), `"action":"`+tc.action+`"`) {
				t.Errorf("pending query params should include canonical action=%q, got: %s", tc.action, string(pq.Params))
			}
			assertSnakeCaseFields(t, string(resp.Result))
		})
	}
}

func TestToolsInteractDOMPrimitive_ScopeRectQueues(t *testing.T) {
	t.Parallel()
	h, _, cap := makeToolHandler(t)
	capturefixture.SetPilot(cap, true)
	mockConnectedTrackedTab(t, cap)

	resp := callInteractRaw(h, `{"what":"click","selector":".compose","scope_rect":{"x":120,"y":240,"width":300,"height":180}}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("click with scope_rect should queue, got error: %s", result.Content[0].Text)
	}

	pq := lastPendingQuerySnapshot(cap.Queries())
	if pq == nil {
		t.Fatal("expected pending query for click")
	}

	var payload map[string]any
	if err := json.Unmarshal(pq.Params, &payload); err != nil {
		t.Fatalf("failed to parse pending query params: %v", err)
	}
	if got, _ := payload["action"].(string); got != "click" {
		t.Fatalf("pending action = %v, want click", payload["action"])
	}
	scopeRect, ok := payload["scope_rect"].(map[string]any)
	if !ok {
		t.Fatalf("scope_rect missing from normalized payload: %s", string(pq.Params))
	}
	for _, k := range []string{"x", "y", "width", "height"} {
		if _, exists := scopeRect[k]; !exists {
			t.Fatalf("scope_rect missing key %q in payload: %s", k, string(pq.Params))
		}
	}
}

// ============================================
// interact(action:"list_interactive") — near_x/near_y/near_radius → scope_rect conversion (#448)
// ============================================

func TestToolsInteractDOMPrimitive_NearParamsConvertToScopeRect(t *testing.T) {
	t.Parallel()
	h, _, cap := makeToolHandler(t)
	capturefixture.SetPilot(cap, true)
	mockConnectedTrackedTab(t, cap)

	resp := callInteractRaw(h, `{"what":"list_interactive","near_x":500,"near_y":300,"near_radius":150}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("list_interactive with near params should queue, got error: %s", result.Content[0].Text)
	}

	pq := lastPendingQuerySnapshot(cap.Queries())
	if pq == nil {
		t.Fatal("expected pending query for list_interactive")
	}

	var payload map[string]any
	if err := json.Unmarshal(pq.Params, &payload); err != nil {
		t.Fatalf("failed to parse pending query params: %v", err)
	}
	scopeRect, ok := payload["scope_rect"].(map[string]any)
	if !ok {
		t.Fatalf("scope_rect missing from payload — near_x/near_y/near_radius should convert to scope_rect: %s", string(pq.Params))
	}
	// near_x=500, near_y=300, near_radius=150 → scope_rect={x:350, y:150, width:300, height:300}
	if x, _ := scopeRect["x"].(float64); x != 350 {
		t.Errorf("scope_rect.x = %v, want 350", x)
	}
	if y, _ := scopeRect["y"].(float64); y != 150 {
		t.Errorf("scope_rect.y = %v, want 150", y)
	}
	if w, _ := scopeRect["width"].(float64); w != 300 {
		t.Errorf("scope_rect.width = %v, want 300", w)
	}
	if h, _ := scopeRect["height"].(float64); h != 300 {
		t.Errorf("scope_rect.height = %v, want 300", h)
	}
}

func TestToolsInteractDOMPrimitive_NearParamsDoNotOverrideScopeRect(t *testing.T) {
	t.Parallel()
	h, _, cap := makeToolHandler(t)
	capturefixture.SetPilot(cap, true)
	mockConnectedTrackedTab(t, cap)

	// Explicit scope_rect takes precedence over near params
	resp := callInteractRaw(h, `{"what":"list_interactive","near_x":500,"near_y":300,"near_radius":150,"scope_rect":{"x":0,"y":0,"width":100,"height":100}}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content[0].Text)
	}

	pq := lastPendingQuerySnapshot(cap.Queries())
	if pq == nil {
		t.Fatal("expected pending query")
	}

	var payload map[string]any
	if err := json.Unmarshal(pq.Params, &payload); err != nil {
		t.Fatalf("failed to parse: %v", err)
	}
	scopeRect := payload["scope_rect"].(map[string]any)
	if x, _ := scopeRect["x"].(float64); x != 0 {
		t.Errorf("explicit scope_rect.x should be preserved, got %v", x)
	}
}
