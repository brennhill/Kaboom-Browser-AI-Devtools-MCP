// reproselector_test.go — Tests reproduction selector parsing and DOM action type mapping.
// Docs: docs/features/feature/reproduction-scripts/index.md
package actionlog

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
			result := parseSelectorForReproduction(tc.selector)
			if tc.wantKey == "role" {
				// Role is a nested map
				roleData, ok := result["role"]
				if !ok {
					t.Errorf("parseSelectorForReproduction(%q) missing 'role' key", tc.selector)
				}
				roleMap, ok := roleData.(map[string]any)
				if !ok {
					t.Errorf("parseSelectorForReproduction(%q) role not a map", tc.selector)
				}
				if roleMap["role"] != "button" {
					t.Errorf("parseSelectorForReproduction(%q) role.role = %v, want 'button'", tc.selector, roleMap["role"])
				}
			} else {
				val, ok := result[tc.wantKey].(string)
				if !ok || val != tc.wantVal {
					t.Errorf("parseSelectorForReproduction(%q)[%q] = %q, want %q", tc.selector, tc.wantKey, val, tc.wantVal)
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
			reproType, ok := reproTypeForDOMAction(tc.domAction)
			if ok != tc.wantOK {
				t.Errorf("reproTypeForDOMAction(%q) ok = %v, want %v", tc.domAction, ok, tc.wantOK)
			}
			if ok && reproType != tc.wantType {
				t.Errorf("reproTypeForDOMAction(%q) = %q, want %q", tc.domAction, reproType, tc.wantType)
			}
		})
	}
}

func TestParseSelectorForReproduction_TextSelector(t *testing.T) {
	t.Parallel()
	result := parseSelectorForReproduction("text=Submit")
	if result["text"] != "Submit" {
		t.Errorf("text=Submit should parse to text:Submit, got %v", result)
	}
}

func TestParseSelectorForReproduction_RoleSelector(t *testing.T) {
	t.Parallel()
	result := parseSelectorForReproduction("role=button")
	roleMap, ok := result["role"].(map[string]any)
	if !ok || roleMap["role"] != "button" {
		t.Errorf("role=button should parse to role:{role:button}, got %v", result)
	}
}

func TestParseSelectorForReproduction_LabelSelector(t *testing.T) {
	t.Parallel()
	result := parseSelectorForReproduction("label=Name")
	if result["ariaLabel"] != "Name" {
		t.Errorf("label=Name should parse to ariaLabel:Name, got %v", result)
	}
}

func TestParseSelectorForReproduction_AriaLabelSelector(t *testing.T) {
	t.Parallel()
	result := parseSelectorForReproduction("aria-label=Close")
	if result["ariaLabel"] != "Close" {
		t.Errorf("aria-label=Close should parse to ariaLabel:Close, got %v", result)
	}
}

func TestParseSelectorForReproduction_PlaceholderSelector(t *testing.T) {
	t.Parallel()
	result := parseSelectorForReproduction("placeholder=Email")
	if result["ariaLabel"] != "Email" {
		t.Errorf("placeholder=Email should parse to ariaLabel:Email, got %v", result)
	}
}

func TestParseSelectorForReproduction_UnknownPrefixToCSS(t *testing.T) {
	t.Parallel()
	result := parseSelectorForReproduction("data-testid=main")
	if result["cssPath"] != "data-testid=main" {
		t.Errorf("unknown prefix should fall back to cssPath, got %v", result)
	}
}

func TestParseSelectorForReproduction_IDSelector(t *testing.T) {
	t.Parallel()
	result := parseSelectorForReproduction("#main")
	if result["id"] != "main" {
		t.Errorf("#main should parse to id:main, got %v", result)
	}
}

func TestParseSelectorForReproduction_ComplexCSSSelector(t *testing.T) {
	t.Parallel()
	result := parseSelectorForReproduction("#main .btn")
	if result["cssPath"] != "#main .btn" {
		t.Errorf("complex CSS should parse to cssPath, got %v", result)
	}
}

func TestParseSelectorForReproduction_PlainCSSSelector(t *testing.T) {
	t.Parallel()
	result := parseSelectorForReproduction("div.container")
	if result["cssPath"] != "div.container" {
		t.Errorf("plain CSS should parse to cssPath, got %v", result)
	}
}

func TestReproTypeForDOMAction_Mappings(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"click":     "click",
		"type":      "input",
		"select":    "select",
		"check":     "click",
		"key_press": "keypress",
		"scroll_to": "scroll_element",
		"focus":     "focus",
		"hover":     "hover",
	}
	for action, expected := range cases {
		if got, _ := reproTypeForDOMAction(action); got != expected {
			t.Errorf("reproTypeForDOMAction(%q) = %q, want %q", action, got, expected)
		}
	}
}
