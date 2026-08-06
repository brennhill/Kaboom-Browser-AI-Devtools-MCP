// Purpose: Tests for reproduction script generation handler.
// Docs: docs/features/feature/reproduction-scripts/index.md

// reproduction_test.go — Integration tests for reproduction helpers that live in cmd/browser-agent.
// Pure reproduction tests live in internal/reproduction/reproduction_test.go.
package interact

import "testing"

// ============================================
// Selector Parsing for Reproduction (tests parseSelectorForReproduction in tools_interact.go)
// ============================================

func TestParseSelectorForReproduction(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		selector string
		wantKey  string
		wantVal  string
	}{
		{"CSS ID", "#submit-btn", "id", "submit-btn"},
		{"CSS path", "form > input", "cssPath", "form > input"},
		{"text semantic", "text=Submit", "text", "Submit"},
		{"role semantic", "role=button", "role", ""}, // role is a nested map
		{"label semantic", "label=Email", "ariaLabel", "Email"},
		{"aria-label semantic", "aria-label=Close", "ariaLabel", "Close"},
		{"placeholder semantic", "placeholder=Search", "ariaLabel", "Search"},
		{"complex CSS", "div.container > button", "cssPath", "div.container > button"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := ParseSelectorForReproduction(tc.selector)
			if tc.wantKey == "role" {
				// Role is a nested map
				roleData, ok := result["role"]
				if !ok {
					t.Errorf("ParseSelectorForReproduction(%q) missing 'role' key", tc.selector)
				}
				roleMap, ok := roleData.(map[string]any)
				if !ok {
					t.Errorf("ParseSelectorForReproduction(%q) role not a map", tc.selector)
				}
				if roleMap["role"] != "button" {
					t.Errorf("ParseSelectorForReproduction(%q) role.role = %v, want 'button'", tc.selector, roleMap["role"])
				}
			} else {
				val, ok := result[tc.wantKey].(string)
				if !ok || val != tc.wantVal {
					t.Errorf("ParseSelectorForReproduction(%q)[%q] = %q, want %q", tc.selector, tc.wantKey, val, tc.wantVal)
				}
			}
		})
	}
}

func TestDomActionToReproType(t *testing.T) {
	t.Parallel()
	tests := []struct {
		domAction string
		wantType  string
		wantOK    bool
	}{
		{"click", "click", true},
		{"type", "input", true},
		{"select", "select", true},
		{"check", "click", true},
		{"key_press", "keypress", true},
		{"scroll_to", "scroll_element", true},
		{"focus", "focus", true},
		{"get_text", "", false},
		{"get_value", "", false},
	}

	for _, tc := range tests {
		t.Run(tc.domAction, func(t *testing.T) {
			reproType, ok := DOMActionToReproType[tc.domAction]
			if ok != tc.wantOK {
				t.Errorf("domActionToReproType[%q] ok = %v, want %v", tc.domAction, ok, tc.wantOK)
			}
			if ok && reproType != tc.wantType {
				t.Errorf("domActionToReproType[%q] = %q, want %q", tc.domAction, reproType, tc.wantType)
			}
		})
	}
}
