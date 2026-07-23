// playbooks_coverage_test.go — Contract tests for resource URI resolution, playbook lookup,
// and the interact-failure recovery catalog.

package playbooks

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// CanonicalPlaybookCapability
// ---------------------------------------------------------------------------

func TestCanonicalPlaybookCapability_MapsAliasesAndRejectsUnknown(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   string
		want string
	}{
		{"performance", "performance"},
		{"performance_analysis", "performance"},
		{"accessibility", "accessibility"},
		{"accessibility_audit", "accessibility"},
		{"security", "security"},
		{"security_audit", "security"},
		{"automation", "automation"},
		{"browser_automation", "automation"},
		{"interact", "automation"},
		{"  SECURITY_AUDIT  ", "security"}, // case + surrounding whitespace are normalized
		{"a11y", ""},
		{"perf", ""},
		{"", ""},
		{"performance/quick", ""}, // a full key is not a capability
	}
	for _, tt := range tests {
		if got := CanonicalPlaybookCapability(tt.in); got != tt.want {
			t.Errorf("CanonicalPlaybookCapability(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// ResolvePlaybookKey
// ---------------------------------------------------------------------------

func TestResolvePlaybookKey_DefaultsBareCapabilityToQuick(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{"bare capability defaults to quick", "performance", "performance/quick"},
		{"bare alias defaults to quick", "browser_automation", "automation/quick"},
		{"explicit level preserved", "security/full", "security/full"},
		{"alias with level", "accessibility_audit/full", "accessibility/full"},
		{"surrounding slashes stripped", "/security/full/", "security/full"},
		{"trailing slash collapses to bare capability", "performance/", "performance/quick"},
		{"uppercase normalized", "SECURITY/FULL", "security/full"},
		{"inner whitespace trimmed on both parts", " performance /  full ", "performance/full"},
		{"unknown level is passed through unvalidated", "security/deep", "security/deep"},
		{"unknown capability rejected", "seo/quick", ""},
		{"bare unknown capability rejected", "seo", ""},
		{"three segments rejected", "security/full/extra", ""},
		{"empty rejected", "", ""},
		{"only slashes rejected", "///", ""},
		{"whitespace-only level rejected", "performance/ /", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ResolvePlaybookKey(tt.in); got != tt.want {
				t.Errorf("ResolvePlaybookKey(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// ResolvePlaybookKey does not check the key exists — resolution to a missing
// playbook must be caught by ResolveResourceContent, not silently served.
func TestResolvePlaybookKey_UnknownLevelDoesNotResolveToContent(t *testing.T) {
	t.Parallel()

	key := ResolvePlaybookKey("security/deep")
	if key != "security/deep" {
		t.Fatalf("ResolvePlaybookKey = %q, want security/deep", key)
	}
	if _, _, ok := ResolveResourceContent("kaboom://playbook/security/deep"); ok {
		t.Error("kaboom://playbook/security/deep must not resolve — there is no such playbook")
	}
}

// ---------------------------------------------------------------------------
// ResolveResourceContent
// ---------------------------------------------------------------------------

func TestResolveResourceContent_StaticResourcesRouteToTheirOwnContent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		uri  string
		want string
	}{
		{"kaboom://capabilities", CapabilityIndex},
		{"kaboom://guide", GuideContent},
		{"kaboom://quickstart", QuickstartContent},
	}
	for _, tt := range tests {
		gotURI, gotText, ok := ResolveResourceContent(tt.uri)
		if !ok {
			t.Errorf("ResolveResourceContent(%q) not ok", tt.uri)
			continue
		}
		if gotURI != tt.uri {
			t.Errorf("canonical uri = %q, want %q", gotURI, tt.uri)
		}
		if gotText != tt.want {
			t.Errorf("ResolveResourceContent(%q) returned the wrong document", tt.uri)
		}
	}
}

func TestResolveResourceContent_RewritesPlaybookURIToCanonicalForm(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in      string
		wantURI string
		wantKey string
	}{
		{"kaboom://playbook/performance", "kaboom://playbook/performance/quick", "performance/quick"},
		{"kaboom://playbook/browser_automation", "kaboom://playbook/automation/quick", "automation/quick"},
		{"kaboom://playbook/security_audit/full", "kaboom://playbook/security/full", "security/full"},
		{"kaboom://playbook/ACCESSIBILITY/Quick", "kaboom://playbook/accessibility/quick", "accessibility/quick"},
	}
	for _, tt := range tests {
		gotURI, gotText, ok := ResolveResourceContent(tt.in)
		if !ok {
			t.Errorf("ResolveResourceContent(%q) not ok", tt.in)
			continue
		}
		// Clients cache by the returned URI, so alias -> canonical rewriting is load-bearing.
		if gotURI != tt.wantURI {
			t.Errorf("ResolveResourceContent(%q) uri = %q, want %q", tt.in, gotURI, tt.wantURI)
		}
		if gotText != Playbooks[tt.wantKey] {
			t.Errorf("ResolveResourceContent(%q) served content that is not Playbooks[%q]", tt.in, tt.wantKey)
		}
	}
}

func TestResolveResourceContent_UnknownURIsReturnEmptyAndFalse(t *testing.T) {
	t.Parallel()

	unknown := []string{
		"",
		"kaboom://",
		"kaboom://unknown",
		"kaboom://playbook/",
		"kaboom://playbook/seo/quick",
		"kaboom://playbook/security/deep",
		"kaboom://demo/does-not-exist",
		"https://kaboom.dev/playbook/security/quick",
		"KABOOM://GUIDE", // scheme matching is exact, not case-insensitive
	}
	for _, uri := range unknown {
		gotURI, gotText, ok := ResolveResourceContent(uri)
		if ok {
			t.Errorf("ResolveResourceContent(%q) = ok, want not ok", uri)
		}
		if gotURI != "" || gotText != "" {
			t.Errorf("ResolveResourceContent(%q) = (%q, %q), want empty strings on failure", uri, gotURI, gotText)
		}
	}
}

// Demo names are looked up verbatim, unlike playbook keys which are lowercased.
func TestResolveResourceContent_DemoNamesAreExactMatchOnly(t *testing.T) {
	t.Parallel()

	uri, text, ok := ResolveResourceContent("kaboom://demo/ws")
	if !ok {
		t.Fatal("kaboom://demo/ws should resolve")
	}
	if uri != "kaboom://demo/ws" {
		t.Errorf("demo uri = %q, want kaboom://demo/ws (demos are not canonicalized)", uri)
	}
	if text != DemoScripts["ws"] {
		t.Error("demo content should be DemoScripts[\"ws\"]")
	}
	if _, _, ok := ResolveResourceContent("kaboom://demo/WS"); ok {
		t.Error("kaboom://demo/WS must not resolve — demo lookup is case-sensitive")
	}
}

func TestDemoScripts_ExactNameSet(t *testing.T) {
	t.Parallel()

	want := map[string]bool{"ws": true, "annotations": true, "recording": true, "dependencies": true}
	for name := range DemoScripts {
		if !want[name] {
			t.Errorf("unexpected demo %q — add it to the kaboom://demo resource listing before shipping", name)
		}
		delete(want, name)
	}
	for name := range want {
		t.Errorf("demo %q disappeared — kaboom://demo/%s is a published resource URI", name, name)
	}
}

// ---------------------------------------------------------------------------
// Playbooks catalog
// ---------------------------------------------------------------------------

func TestPlaybooks_ExactPublishedKeySet(t *testing.T) {
	t.Parallel()

	want := map[string]bool{
		"performance/quick": true, "performance/full": true,
		"accessibility/quick": true, "accessibility/full": true,
		"security/quick": true, "security/full": true,
		"automation/quick": true, "automation/full": true,
	}
	for key := range Playbooks {
		if !want[key] {
			t.Errorf("unexpected playbook key %q — list it in CapabilityIndex before shipping", key)
		}
		delete(want, key)
	}
	for key := range want {
		t.Errorf("playbook %q missing — kaboom://playbook/%s is advertised in CapabilityIndex", key, key)
	}
}

// The capability index is what agents read to decide which playbook to fetch;
// every URI it advertises must actually resolve.
func TestCapabilityIndex_EveryAdvertisedPlaybookURIResolves(t *testing.T) {
	t.Parallel()

	const prefix = "- kaboom://playbook/"
	found := 0
	for _, line := range strings.Split(CapabilityIndex, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		found++
		uri := strings.TrimPrefix(line, "- ")
		if _, _, ok := ResolveResourceContent(uri); !ok {
			t.Errorf("CapabilityIndex advertises %q but it does not resolve", uri)
		}
	}
	if found != len(Playbooks) {
		t.Errorf("CapabilityIndex lists %d playbook URIs, but %d playbooks exist", found, len(Playbooks))
	}
}

func TestMergePlaybookSets_PanicsOnDuplicateKey(t *testing.T) {
	t.Parallel()

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("mergePlaybookSets must panic on a duplicate key, not silently drop a playbook")
		}
		msg, _ := r.(string)
		if msg != "duplicate playbook key: security/quick" {
			t.Errorf("panic = %v, want \"duplicate playbook key: security/quick\"", r)
		}
	}()

	mergePlaybookSets(
		map[string]string{"security/quick": "a"},
		map[string]string{"security/quick": "b"},
	)
}

func TestMergePlaybookSets_UnionsDisjointSets(t *testing.T) {
	t.Parallel()

	merged := mergePlaybookSets(
		map[string]string{"x/quick": "X"},
		map[string]string{"y/full": "Y"},
		map[string]string{},
	)
	if len(merged) != 2 {
		t.Fatalf("merged size = %d, want 2", len(merged))
	}
	if merged["x/quick"] != "X" || merged["y/full"] != "Y" {
		t.Errorf("merged = %v, want values preserved", merged)
	}
}

// ---------------------------------------------------------------------------
// Interact failure playbooks
// ---------------------------------------------------------------------------

func TestInteractFailurePlaybooks_EveryEntryIsActionable(t *testing.T) {
	t.Parallel()

	wantCodes := map[string]bool{
		"element_not_found": true, "ambiguous_target": true, "stale_element_id": true,
		"scope_not_found": true, "blocked_by_overlay": true,
	}
	for code, pb := range InteractFailurePlaybooks {
		if !wantCodes[code] {
			t.Errorf("unexpected failure code %q — quickstart must document it too", code)
		}
		delete(wantCodes, code)

		// Agents match on the detection signal, so it must name its own code.
		if !strings.HasPrefix(pb.DetectionSignal, "error="+code) {
			t.Errorf("%s: DetectionSignal = %q, want prefix %q", code, pb.DetectionSignal, "error="+code)
		}
		if len(pb.OrderedRecoverySteps) < 2 {
			t.Errorf("%s: only %d recovery steps, a playbook needs an ordered sequence", code, len(pb.OrderedRecoverySteps))
		}
		for i, step := range pb.OrderedRecoverySteps {
			if strings.TrimSpace(step) == "" {
				t.Errorf("%s: recovery step %d is blank", code, i)
			}
		}
		if strings.TrimSpace(pb.StopAndReportCondition) == "" {
			t.Errorf("%s: StopAndReportCondition is empty — recovery would loop forever", code)
		}
		if strings.TrimSpace(pb.RetrySuggestion) == "" {
			t.Errorf("%s: RetrySuggestion is empty", code)
		}
	}
	for code := range wantCodes {
		t.Errorf("failure playbook %q missing — error responses for it would carry no recovery plan", code)
	}
}

func TestNormalizeInteractFailureCode_MatchesExactAndEmbeddedCodes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{"exact code", "element_not_found", "element_not_found"},
		{"uppercase", "STALE_ELEMENT_ID", "stale_element_id"},
		{"padded", "  scope_not_found\n", "scope_not_found"},
		{"embedded in a sentence", "click failed: blocked_by_overlay (modal on top)", "blocked_by_overlay"},
		{"embedded with error= prefix", "error=ambiguous_target", "ambiguous_target"},
		{"unknown code", "network_error", ""},
		{"empty", "", ""},
		{"whitespace only", "   ", ""},
		{"substring of a code does not match", "element_not", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NormalizeInteractFailureCode(tt.in); got != tt.want {
				t.Errorf("NormalizeInteractFailureCode(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestLookupInteractFailurePlaybook_ReturnsCanonicalCodeAndItsPlaybook(t *testing.T) {
	t.Parallel()

	code, pb, ok := LookupInteractFailurePlaybook("Error=BLOCKED_BY_OVERLAY while clicking")
	if !ok {
		t.Fatal("lookup should succeed for an embedded, upper-cased code")
	}
	if code != "blocked_by_overlay" {
		t.Errorf("code = %q, want blocked_by_overlay", code)
	}
	// The returned playbook must be the one keyed by the canonical code, not a neighbour.
	if pb.RetrySuggestion != InteractFailurePlaybooks["blocked_by_overlay"].RetrySuggestion {
		t.Errorf("returned playbook does not match InteractFailurePlaybooks[%q]", code)
	}
	if pb.OrderedRecoverySteps[0] != `Run interact({what:"dismiss_top_overlay"}) to close the topmost modal/dialog.` {
		t.Errorf("first recovery step = %q", pb.OrderedRecoverySteps[0])
	}
}

func TestLookupInteractFailurePlaybook_UnknownCodeReturnsZeroValue(t *testing.T) {
	t.Parallel()

	for _, raw := range []string{"", "   ", "timeout", "not_a_code"} {
		code, pb, ok := LookupInteractFailurePlaybook(raw)
		if ok {
			t.Errorf("LookupInteractFailurePlaybook(%q) = ok, want not ok", raw)
		}
		if code != "" {
			t.Errorf("LookupInteractFailurePlaybook(%q) code = %q, want empty", raw, code)
		}
		if pb.DetectionSignal != "" || pb.OrderedRecoverySteps != nil || pb.StopAndReportCondition != "" || pb.RetrySuggestion != "" {
			t.Errorf("LookupInteractFailurePlaybook(%q) returned a non-zero playbook: %+v", raw, pb)
		}
	}
}

func TestTutorialFailureRecoveryPlaybooks_SnakeCaseSerializationShape(t *testing.T) {
	t.Parallel()

	out := TutorialFailureRecoveryPlaybooks()
	if len(out) != len(InteractFailurePlaybooks) {
		t.Fatalf("got %d entries, want %d (one per failure code)", len(out), len(InteractFailurePlaybooks))
	}

	entry, ok := out["element_not_found"].(map[string]any)
	if !ok {
		t.Fatalf("out[element_not_found] = %T, want map[string]any", out["element_not_found"])
	}

	// The JSON key is retry_guidance while the struct field is RetrySuggestion;
	// renaming one without the other silently drops the field from tutorial output.
	wantKeys := []string{"detection_signal", "ordered_recovery_steps", "stop_and_report_condition", "retry_guidance"}
	if len(entry) != len(wantKeys) {
		t.Errorf("entry has %d keys %v, want exactly %v", len(entry), entry, wantKeys)
	}
	for _, k := range wantKeys {
		if _, ok := entry[k]; !ok {
			t.Errorf("entry missing key %q", k)
		}
	}

	src := InteractFailurePlaybooks["element_not_found"]
	if entry["detection_signal"] != src.DetectionSignal {
		t.Errorf("detection_signal = %v, want %q", entry["detection_signal"], src.DetectionSignal)
	}
	if entry["retry_guidance"] != src.RetrySuggestion {
		t.Errorf("retry_guidance = %v, want RetrySuggestion %q", entry["retry_guidance"], src.RetrySuggestion)
	}
	if entry["stop_and_report_condition"] != src.StopAndReportCondition {
		t.Errorf("stop_and_report_condition mismatch")
	}
	steps, ok := entry["ordered_recovery_steps"].([]string)
	if !ok {
		t.Fatalf("ordered_recovery_steps = %T, want []string", entry["ordered_recovery_steps"])
	}
	if len(steps) != len(src.OrderedRecoverySteps) || steps[0] != src.OrderedRecoverySteps[0] {
		t.Errorf("ordered_recovery_steps = %v, want %v (order is meaningful)", steps, src.OrderedRecoverySteps)
	}
}
