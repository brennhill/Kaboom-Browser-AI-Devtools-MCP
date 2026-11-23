// resolver_test.go -- Tests for URI resolution, capability normalization,
// playbook-key resolution, and strict rejection of noncanonical names. Pure logic; no I/O.

package resources

import "testing"

func TestCanonicalPlaybookCapability(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"performance canonical", "performance", "performance"},
		{"performance alias rejected", "performance_analysis", ""},
		{"accessibility canonical", "accessibility", "accessibility"},
		{"accessibility alias rejected", "accessibility_audit", ""},
		{"security canonical", "security", "security"},
		{"security alias rejected", "security_audit", ""},
		{"automation canonical", "automation", "automation"},
		{"automation alias browser rejected", "browser_automation", ""},
		{"automation alias interact rejected", "interact", ""},
		{"uppercase trimmed", "  PERFORMANCE  ", "performance"},
		{"mixed case alias rejected", "Security_Audit", ""},
		{"unknown", "banana", ""},
		{"empty", "", ""},
		{"whitespace only", "   ", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := CanonicalPlaybookCapability(tc.in); got != tc.want {
				t.Fatalf("CanonicalPlaybookCapability(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestResolvePlaybookKey(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"bare capability defaults to quick", "performance", "performance/quick"},
		{"bare alias rejected", "interact", ""},
		{"capability and level", "security/full", "security/full"},
		{"alias and level rejected", "browser_automation/full", ""},
		{"leading and trailing slashes trimmed", "/accessibility/quick/", "accessibility/quick"},
		{"uppercase normalized", "PERFORMANCE/QUICK", "performance/quick"},
		{"unknown bare capability", "banana", ""},
		{"unknown capability with level", "banana/full", ""},
		{"empty", "", ""},
		{"slash only", "/", ""},
		{"trailing slash trimmed to bare capability", "performance/", "performance/quick"},
		{"three parts rejected", "performance/quick/extra", ""},
		{"interior empty segment rejected", "performance//quick", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ResolvePlaybookKey(tc.in); got != tc.want {
				t.Fatalf("ResolvePlaybookKey(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestResolveResourceContent(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		uri         string
		wantURI     string
		wantOK      bool
		wantContent string // must equal exactly when non-empty
	}{
		{"capabilities", "kaboom://capabilities", "kaboom://capabilities", true, CapabilityIndex},
		{"guide", "kaboom://guide", "kaboom://guide", true, GuideContent},
		{"quickstart", "kaboom://quickstart", "kaboom://quickstart", true, QuickstartContent},
		{"playbook bare capability", "kaboom://playbook/performance", "kaboom://playbook/performance/quick", true, playbooks()["performance/quick"]},
		{"playbook explicit level", "kaboom://playbook/security/full", "kaboom://playbook/security/full", true, playbooks()["security/full"]},
		{"playbook alias rejected", "kaboom://playbook/interact", "", false, ""},
		{"playbook unknown", "kaboom://playbook/banana", "", false, ""},
		{"demo valid", "kaboom://demo/ws", "kaboom://demo/ws", true, demoScripts()["ws"]},
		{"demo unknown", "kaboom://demo/nope", "", false, ""},
		{"unknown scheme", "kaboom://mystery", "", false, ""},
		{"empty", "", "", false, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			uri, content, ok := ResolveResourceContent(tc.uri)
			if ok != tc.wantOK {
				t.Fatalf("ResolveResourceContent(%q) ok = %v, want %v", tc.uri, ok, tc.wantOK)
			}
			if uri != tc.wantURI {
				t.Fatalf("ResolveResourceContent(%q) uri = %q, want %q", tc.uri, uri, tc.wantURI)
			}
			if tc.wantOK {
				if content != tc.wantContent {
					t.Fatalf("ResolveResourceContent(%q) content mismatch", tc.uri)
				}
				if content == "" {
					t.Fatalf("ResolveResourceContent(%q) returned empty content on success", tc.uri)
				}
			} else if content != "" {
				t.Fatalf("ResolveResourceContent(%q) returned content on failure: %q", tc.uri, content)
			}
		})
	}
}

func TestResourceLookupsCannotMutateLaterResponses(t *testing.T) {
	firstPlaybooks := playbooks()
	firstPlaybooks["performance/quick"] = "poisoned"
	firstDemos := demoScripts()
	firstDemos["ws"] = "poisoned"

	_, playbook, ok := ResolveResourceContent("kaboom://playbook/performance/quick")
	if !ok || playbook == "poisoned" {
		t.Fatalf("playbook lookup retained caller mutation: ok=%v content=%q", ok, playbook)
	}
	_, demo, ok := ResolveResourceContent("kaboom://demo/ws")
	if !ok || demo == "poisoned" {
		t.Fatalf("demo lookup retained caller mutation: ok=%v content=%q", ok, demo)
	}
}

// TestResolveResourceContent_AllPlaybookVariants confirms every advertised
// capability/level combination resolves to non-empty content.
func TestResolveResourceContent_AllPlaybookVariants(t *testing.T) {
	t.Parallel()
	for _, cap := range []string{"performance", "accessibility", "security", "automation"} {
		for _, level := range []string{"quick", "full"} {
			uri := "kaboom://playbook/" + cap + "/" + level
			canonical, content, ok := ResolveResourceContent(uri)
			if !ok {
				t.Fatalf("expected %q to resolve", uri)
			}
			if canonical != uri {
				t.Fatalf("canonical(%q) = %q, want same", uri, canonical)
			}
			if content == "" {
				t.Fatalf("empty content for %q", uri)
			}
		}
	}
}
