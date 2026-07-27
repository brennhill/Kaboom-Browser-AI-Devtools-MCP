// Purpose: Tests for interact DOM query operations.
// Docs: docs/features/feature/interact-explore/index.md

// tools_interact_dom_test.go — Tests for DOM interaction primitives.
//
// Tests verify parameter validation, pilot gating, and queuing behavior
// for the 13 DOM primitive actions.
//
// Run: go test ./cmd/browser-agent -run "TestDOMPrimitive" -v
package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

// ============================================
// Parameter Validation: Missing Selector
// All 11 selector-requiring actions must error when selector is missing
// ============================================

func TestDOMPrimitive_MissingSelector(t *testing.T) {
	env := newInteractTestEnv(t)

	actions := []struct {
		action string
		args   string
	}{
		{"click", `{"what":"click"}`},
		{"type", `{"what":"type","text":"hello"}`},
		{"paste", `{"what":"paste","text":"hello"}`},
		{"select", `{"what":"select","value":"opt1"}`},
		{"check", `{"what":"check"}`},
		{"get_text", `{"what":"get_text"}`},
		{"get_value", `{"what":"get_value"}`},
		{"get_attribute", `{"what":"get_attribute","name":"href"}`},
		{"set_attribute", `{"what":"set_attribute","name":"href","value":"#"}`},
		{"focus", `{"what":"focus"}`},
		{"scroll_to", `{"what":"scroll_to"}`},
		{"wait_for", `{"what":"wait_for"}`},
		{"hover", `{"what":"hover"}`},
	}

	for _, tc := range actions {
		t.Run(tc.action, func(t *testing.T) {
			result, ok := env.callInteract(t, tc.args)
			if !ok {
				t.Fatalf("%s without selector should return result", tc.action)
			}

			if !result.IsError {
				t.Errorf("%s without selector MUST return isError:true", tc.action)
			}

			if len(result.Content) > 0 {
				text := strings.ToLower(result.Content[0].Text)
				if !strings.Contains(text, "selector") {
					t.Errorf("%s error should mention selector\nGot: %s", tc.action, result.Content[0].Text)
				}
			}
		})
	}
}

// ============================================
// Parameter Validation: Action-specific required params
// ============================================

func TestDOMPrimitive_Type_MissingText(t *testing.T) {
	env := newInteractTestEnv(t)

	result, ok := env.callInteract(t, `{"what":"type","selector":"#input"}`)
	if !ok {
		t.Fatal("type without text should return result")
	}

	if !result.IsError {
		t.Error("type without text MUST return isError:true")
	}

	if len(result.Content) > 0 {
		text := strings.ToLower(result.Content[0].Text)
		if !strings.Contains(text, "text") {
			t.Errorf("error should mention text parameter\nGot: %s", result.Content[0].Text)
		}
	}
}

func TestDOMPrimitive_Select_MissingValue(t *testing.T) {
	env := newInteractTestEnv(t)

	result, ok := env.callInteract(t, `{"what":"select","selector":"#dropdown"}`)
	if !ok {
		t.Fatal("select without value should return result")
	}

	if !result.IsError {
		t.Error("select without value MUST return isError:true")
	}

	if len(result.Content) > 0 {
		text := strings.ToLower(result.Content[0].Text)
		if !strings.Contains(text, "value") {
			t.Errorf("error should mention value parameter\nGot: %s", result.Content[0].Text)
		}
	}
}

func TestDOMPrimitive_GetAttribute_MissingName(t *testing.T) {
	env := newInteractTestEnv(t)

	result, ok := env.callInteract(t, `{"what":"get_attribute","selector":"#link"}`)
	if !ok {
		t.Fatal("get_attribute without name should return result")
	}

	if !result.IsError {
		t.Error("get_attribute without name MUST return isError:true")
	}

	if len(result.Content) > 0 {
		text := strings.ToLower(result.Content[0].Text)
		if !strings.Contains(text, "name") {
			t.Errorf("error should mention name parameter\nGot: %s", result.Content[0].Text)
		}
	}
}

func TestDOMPrimitive_SetAttribute_MissingName(t *testing.T) {
	env := newInteractTestEnv(t)

	result, ok := env.callInteract(t, `{"what":"set_attribute","selector":"#link","value":"#"}`)
	if !ok {
		t.Fatal("set_attribute without name should return result")
	}

	if !result.IsError {
		t.Error("set_attribute without name MUST return isError:true")
	}

	if len(result.Content) > 0 {
		text := strings.ToLower(result.Content[0].Text)
		if !strings.Contains(text, "name") {
			t.Errorf("error should mention name parameter\nGot: %s", result.Content[0].Text)
		}
	}
}

// ============================================
// Pilot Disabled: All 13 DOM actions check pilot
// ============================================

func TestDOMPrimitive_AllActions_PilotDisabled(t *testing.T) {
	env := newInteractTestEnv(t)

	// All DOM actions with valid params — pilot is disabled by default
	actions := []struct {
		action string
		args   string
	}{
		{"click", `{"what":"click","selector":"#btn"}`},
		{"type", `{"what":"type","selector":"#input","text":"hello"}`},
		{"paste", `{"what":"paste","selector":"#editor","text":"hello"}`},
		{"select", `{"what":"select","selector":"#dropdown","value":"opt1"}`},
		{"check", `{"what":"check","selector":"#checkbox"}`},
		{"get_text", `{"what":"get_text","selector":"#el"}`},
		{"get_value", `{"what":"get_value","selector":"#input"}`},
		{"get_attribute", `{"what":"get_attribute","selector":"#link","name":"href"}`},
		{"set_attribute", `{"what":"set_attribute","selector":"#el","name":"class","value":"active"}`},
		{"focus", `{"what":"focus","selector":"#input"}`},
		{"scroll_to", `{"what":"scroll_to","selector":"#section"}`},
		{"wait_for", `{"what":"wait_for","selector":"#loading"}`},
		{"key_press", `{"what":"key_press","selector":"#input","text":"Enter"}`},
		{"open_composer", `{"what":"open_composer"}`},
		{"submit_active_composer", `{"what":"submit_active_composer"}`},
		{"confirm_top_dialog", `{"what":"confirm_top_dialog"}`},
		{"dismiss_top_overlay", `{"what":"dismiss_top_overlay"}`},
		{"list_interactive", `{"what":"list_interactive"}`},
		{"hover", `{"what":"hover","selector":"#el"}`},
		{"activate_tab", `{"what":"activate_tab"}`},
	}

	for _, tc := range actions {
		t.Run(tc.action, func(t *testing.T) {
			result, ok := env.callInteract(t, tc.args)
			if !ok {
				t.Fatalf("%s should return result", tc.action)
			}

			if !result.IsError {
				t.Errorf("%s with pilot disabled should return isError:true", tc.action)
			}

			if len(result.Content) > 0 {
				text := strings.ToLower(result.Content[0].Text)
				if !strings.Contains(text, "pilot") {
					t.Errorf("%s error should mention pilot\nGot: %s", tc.action, result.Content[0].Text)
				}
			}
		})
	}
}

// ============================================
// Queued Response: Valid params + pilot enabled returns correlation_id
// NOTE: Skipped — requires pilot enabled which needs extension connection.
// Covered by shell UAT.
// ============================================

func TestDOMPrimitive_Click_ReturnsCorrelationID(t *testing.T) {
	t.Skip("Skipped: requires pilot enabled; covered by shell UAT")
}

func TestDOMPrimitive_ListInteractive_ReturnsCorrelationID(t *testing.T) {
	t.Skip("Skipped: requires pilot enabled; covered by shell UAT")
}

// ============================================
// list_interactive: Does NOT require selector
// ============================================

func TestDOMPrimitive_ListInteractive_NoSelectorNeeded(t *testing.T) {
	env := newInteractTestEnv(t)

	// list_interactive without selector should NOT get a "selector missing" error
	// It should reach the pilot check instead
	result, ok := env.callInteract(t, `{"what":"list_interactive"}`)
	if !ok {
		t.Fatal("list_interactive should return result")
	}

	if len(result.Content) > 0 {
		text := strings.ToLower(result.Content[0].Text)
		if strings.Contains(text, "selector") {
			t.Errorf("list_interactive should NOT require selector\nGot: %s", result.Content[0].Text)
		}
	}
}

// ============================================
// key_press: No selector required (#321)
// ============================================

func TestDOMPrimitive_KeyPress_NoSelectorRequired(t *testing.T) {
	env := newInteractTestEnv(t)

	// key_press without selector should NOT get a "selector missing" error.
	// It should reach the pilot check instead (pilot is disabled in test env).
	result, ok := env.callInteract(t, `{"what":"key_press","text":"Escape"}`)
	if !ok {
		t.Fatal("key_press without selector should return result")
	}

	if len(result.Content) > 0 {
		text := strings.ToLower(result.Content[0].Text)
		if strings.Contains(text, "selector") {
			t.Errorf("key_press should NOT require selector (#321)\nGot: %s", result.Content[0].Text)
		}
	}
}

func TestDOMPrimitive_IntentActions_NoSelectorNeeded(t *testing.T) {
	env := newInteractTestEnv(t)

	actions := []string{
		"open_composer",
		"submit_active_composer",
		"confirm_top_dialog",
		"dismiss_top_overlay",
		"activate_tab",
	}

	for _, action := range actions {
		t.Run(action, func(t *testing.T) {
			result, ok := env.callInteract(t, `{"what":"`+action+`"}`)
			if !ok {
				t.Fatalf("%s should return result", action)
			}

			if len(result.Content) > 0 {
				text := strings.ToLower(result.Content[0].Text)
				if strings.Contains(text, "selector") {
					t.Errorf("%s should NOT require selector\nGot: %s", action, result.Content[0].Text)
				}
			}
		})
	}
}

// ============================================
// Parameter Validation: paste action
// ============================================

func TestDOMPrimitive_Paste_MissingText(t *testing.T) {
	env := newInteractTestEnv(t)

	result, ok := env.callInteract(t, `{"what":"paste","selector":"#editor"}`)
	if !ok {
		t.Fatal("paste without text should return result")
	}

	if !result.IsError {
		t.Error("paste without text MUST return isError:true")
	}

	if len(result.Content) > 0 {
		text := strings.ToLower(result.Content[0].Text)
		if !strings.Contains(text, "text") {
			t.Errorf("error should mention text parameter\nGot: %s", result.Content[0].Text)
		}
	}
}

func TestDOMPrimitive_Paste_MissingSelector(t *testing.T) {
	env := newInteractTestEnv(t)

	result, ok := env.callInteract(t, `{"what":"paste","text":"hello"}`)
	if !ok {
		t.Fatal("paste without selector should return result")
	}

	if !result.IsError {
		t.Error("paste without selector MUST return isError:true")
	}

	if len(result.Content) > 0 {
		text := strings.ToLower(result.Content[0].Text)
		if !strings.Contains(text, "selector") {
			t.Errorf("error should mention selector\nGot: %s", result.Content[0].Text)
		}
	}
}

func TestDOMPrimitive_Paste_PilotDisabled(t *testing.T) {
	env := newInteractTestEnv(t)

	result, ok := env.callInteract(t, `{"what":"paste","selector":"#editor","text":"hello"}`)
	if !ok {
		t.Fatal("paste should return result")
	}

	if !result.IsError {
		t.Error("paste with pilot disabled should return isError:true")
	}

	if len(result.Content) > 0 {
		text := strings.ToLower(result.Content[0].Text)
		if !strings.Contains(text, "pilot") {
			t.Errorf("paste error should mention pilot\nGot: %s", result.Content[0].Text)
		}
	}
}

// ============================================
// Safety Net: All DOM actions don't panic
// ============================================

// ============================================
// wait_for: Enhanced condition validation (#371)
// ============================================

func TestDOMPrimitive_WaitFor_TextOnly_NoSelectorRequired(t *testing.T) {
	env := newInteractTestEnv(t)

	// wait_for with text (no selector) should NOT get a "selector missing" error.
	// It should reach the pilot check instead.
	result, ok := env.callInteract(t, `{"what":"wait_for","text":"Welcome"}`)
	if !ok {
		t.Fatal("wait_for with text should return result")
	}

	if len(result.Content) > 0 {
		text := strings.ToLower(result.Content[0].Text)
		if strings.Contains(text, "selector") {
			t.Errorf("wait_for with text should NOT require selector\nGot: %s", result.Content[0].Text)
		}
	}
}

func TestDOMPrimitive_WaitFor_URLContainsOnly_NoSelectorRequired(t *testing.T) {
	env := newInteractTestEnv(t)

	// wait_for with url_contains (no selector) should NOT get a "selector missing" error.
	result, ok := env.callInteract(t, `{"what":"wait_for","url_contains":"/dashboard"}`)
	if !ok {
		t.Fatal("wait_for with url_contains should return result")
	}

	if len(result.Content) > 0 {
		text := strings.ToLower(result.Content[0].Text)
		if strings.Contains(text, "selector") {
			t.Errorf("wait_for with url_contains should NOT require selector\nGot: %s", result.Content[0].Text)
		}
	}
}

func TestDOMPrimitive_WaitFor_MutualExclusivity(t *testing.T) {
	env := newInteractTestEnv(t)

	cases := []struct {
		name string
		args string
	}{
		{"selector_and_text", `{"what":"wait_for","selector":"#el","text":"hello"}`},
		{"selector_and_url_contains", `{"what":"wait_for","selector":"#el","url_contains":"/page"}`},
		{"text_and_url_contains", `{"what":"wait_for","text":"hello","url_contains":"/page"}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, ok := env.callInteract(t, tc.args)
			if !ok {
				t.Fatalf("wait_for %s should return result", tc.name)
			}

			if !result.IsError {
				t.Errorf("wait_for %s MUST return isError:true", tc.name)
			}

			if len(result.Content) > 0 {
				text := strings.ToLower(result.Content[0].Text)
				if !strings.Contains(text, "mutually exclusive") {
					t.Errorf("wait_for %s error should mention 'mutually exclusive'\nGot: %s", tc.name, result.Content[0].Text)
				}
			}
		})
	}
}

func TestDOMPrimitive_WaitFor_AbsentRequiresSelector(t *testing.T) {
	env := newInteractTestEnv(t)

	result, ok := env.callInteract(t, `{"what":"wait_for","absent":true}`)
	if !ok {
		t.Fatal("wait_for with absent but no selector should return result")
	}

	if !result.IsError {
		t.Error("wait_for with absent but no selector MUST return isError:true")
	}

	if len(result.Content) > 0 {
		text := strings.ToLower(result.Content[0].Text)
		if !strings.Contains(text, "selector") {
			t.Errorf("error should mention selector requirement\nGot: %s", result.Content[0].Text)
		}
	}
}

func TestDOMPrimitive_WaitFor_AbsentWithSelector_PassesValidation(t *testing.T) {
	env := newInteractTestEnv(t)

	// wait_for with absent + selector should pass validation and reach the pilot check
	result, ok := env.callInteract(t, `{"what":"wait_for","selector":"#spinner","absent":true}`)
	if !ok {
		t.Fatal("wait_for with absent+selector should return result")
	}

	if len(result.Content) > 0 {
		text := strings.ToLower(result.Content[0].Text)
		// Should NOT fail on condition validation — should reach pilot check instead
		if strings.Contains(text, "mutually exclusive") || strings.Contains(text, "requires") {
			t.Errorf("wait_for with absent+selector should pass validation\nGot: %s", result.Content[0].Text)
		}
	}
}

func TestDOMPrimitive_WaitFor_NoCondition(t *testing.T) {
	env := newInteractTestEnv(t)

	result, ok := env.callInteract(t, `{"what":"wait_for"}`)
	if !ok {
		t.Fatal("wait_for with no conditions should return result")
	}

	if !result.IsError {
		t.Error("wait_for with no conditions MUST return isError:true")
	}

	if len(result.Content) > 0 {
		text := strings.ToLower(result.Content[0].Text)
		if !strings.Contains(text, "condition") && !strings.Contains(text, "selector") {
			t.Errorf("error should mention missing condition\nGot: %s", result.Content[0].Text)
		}
	}
}

func TestDOMPrimitive_AllActions_NoPanic(t *testing.T) {
	env := newInteractTestEnv(t)

	allActions := []struct {
		action string
		args   string
	}{
		{"click", `{"what":"click","selector":"#btn"}`},
		{"type", `{"what":"type","selector":"#input","text":"hello"}`},
		{"paste", `{"what":"paste","selector":"#editor","text":"hello"}`},
		{"select", `{"what":"select","selector":"#dropdown","value":"opt1"}`},
		{"check", `{"what":"check","selector":"#checkbox"}`},
		{"get_text", `{"what":"get_text","selector":"#el"}`},
		{"get_value", `{"what":"get_value","selector":"#input"}`},
		{"get_attribute", `{"what":"get_attribute","selector":"#link","name":"href"}`},
		{"set_attribute", `{"what":"set_attribute","selector":"#el","name":"class","value":"active"}`},
		{"focus", `{"what":"focus","selector":"#input"}`},
		{"scroll_to", `{"what":"scroll_to","selector":"#section"}`},
		{"wait_for", `{"what":"wait_for","selector":"#loading"}`},
		{"key_press", `{"what":"key_press","selector":"#input","text":"Enter"}`},
		{"open_composer", `{"what":"open_composer"}`},
		{"submit_active_composer", `{"what":"submit_active_composer"}`},
		{"confirm_top_dialog", `{"what":"confirm_top_dialog"}`},
		{"dismiss_top_overlay", `{"what":"dismiss_top_overlay"}`},
		{"list_interactive", `{"what":"list_interactive"}`},
		{"hover", `{"what":"hover","selector":"#el"}`},
		{"activate_tab", `{"what":"activate_tab"}`},
	}

	for _, tc := range allActions {
		t.Run(tc.action, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("interact(%s) PANICKED: %v", tc.action, r)
				}
			}()

			args := json.RawMessage(tc.args)
			req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
			resp := env.handler.toolInteract(req, args)

			if resp.Result == nil && resp.Error == nil {
				t.Errorf("interact(%s) returned nil response", tc.action)
			}
		})
	}
}
