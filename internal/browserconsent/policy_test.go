// policy_test.go — Contract for the browser-driving consent gate (kaboom-05ue.2).
package browserconsent

import "testing"

func TestOriginOfNormalizes(t *testing.T) {
	cases := []struct {
		raw, want string
	}{
		{"https://example.com/path?q=secret#frag", "https://example.com"},
		{"https://Example.COM/x", "https://example.com"},
		{"http://example.com:80/x", "http://example.com"},
		{"https://example.com:443/x", "https://example.com"},
		{"http://127.0.0.1:5173/app", "http://127.0.0.1:5173"},
		{"https://sub.example.com/", "https://sub.example.com"},
	}
	for _, tc := range cases {
		got, err := OriginOf(tc.raw)
		if err != nil {
			t.Fatalf("OriginOf(%q) returned error: %v", tc.raw, err)
		}
		if got != tc.want {
			t.Errorf("OriginOf(%q) = %q, want %q", tc.raw, got, tc.want)
		}
	}
}

// A URL's path and query must never survive into the stored origin: consent lists are
// user-visible and logged, and rule 7/13 keep query strings out of both.
func TestOriginOfDropsPathAndQuery(t *testing.T) {
	got, err := OriginOf("https://example.com/reset?token=abc123&email=a@b.c")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "https://example.com" {
		t.Fatalf("origin leaked path or query: %q", got)
	}
}

func TestOriginOfRejectsUnusable(t *testing.T) {
	for _, raw := range []string{"", "   ", "not a url", "about:blank", "chrome://settings", "javascript:alert(1)"} {
		if got, err := OriginOf(raw); err == nil {
			t.Errorf("OriginOf(%q) = %q, want error", raw, got)
		}
	}
}

// The gate must FAIL CLOSED. isMutationAction elsewhere is a hand-maintained allowlist of
// mutating actions, which is fine for evidence capture but wrong here: an action added later
// would default to "not mutating" and skip the gate entirely. Gating is therefore defined by
// an explicit read-only set, so anything new is gated until someone decides otherwise.
func TestIsGatedFailsClosedForUnknownActions(t *testing.T) {
	if !IsGated("some_action_invented_next_year") {
		t.Fatal("an unrecognized action must be gated, not waved through")
	}
	if !IsGated("") {
		t.Fatal("an empty action must be gated")
	}
}

func TestIsGatedLeavesReadOnlyActionsAlone(t *testing.T) {
	readOnly := []string{
		"get_text", "get_value", "get_attribute", "query", "list_interactive",
		"get_readable", "get_markdown", "explore_page", "wait_for", "wait_for_stable",
		"list_states", "clipboard_read",
	}
	for _, action := range readOnly {
		if IsGated(action) {
			t.Errorf("%s reads only and must not require driving consent", action)
		}
	}
}

func TestIsGatedCoversEveryStateChangingAction(t *testing.T) {
	mutating := []string{
		"click", "type", "select", "check", "paste", "key_press", "hardware_click",
		"navigate", "refresh", "back", "forward", "new_tab", "close_tab",
		"execute_js", "set_storage", "clear_storage", "set_cookie", "delete_cookie",
		"fill_form", "fill_form_and_submit", "upload", "batch",
		"screen_recording_start", "clipboard_write", "load_state",
	}
	for _, action := range mutating {
		if !IsGated(action) {
			t.Errorf("%s changes state and must require driving consent", action)
		}
	}
}

func TestDecideDeniesUnlistedOrigin(t *testing.T) {
	p := NewPolicy()
	d := p.Decide("click", "https://bank.example.com/transfer")
	if d.Allowed {
		t.Fatal("an unlisted origin must be denied by default")
	}
	if d.Reason != ReasonNoConsent {
		t.Errorf("Reason = %q, want %q", d.Reason, ReasonNoConsent)
	}
	if d.Origin != "https://bank.example.com" {
		t.Errorf("denial must name the origin, got %q", d.Origin)
	}
}

func TestDecideAllowsConsentedOrigin(t *testing.T) {
	p := NewPolicy()
	if err := p.Allow("https://example.com"); err != nil {
		t.Fatalf("Allow: %v", err)
	}
	if d := p.Decide("click", "https://example.com/page"); !d.Allowed {
		t.Fatalf("consented origin denied: %+v", d)
	}
}

// Consent is per-origin. A grant on one host must not extend to another, and must not
// extend to a subdomain: evil.example.com is a different security principal.
func TestConsentDoesNotSpreadAcrossOrigins(t *testing.T) {
	p := NewPolicy()
	_ = p.Allow("https://example.com")
	for _, raw := range []string{
		"https://evil.example.com/x",
		"http://example.com/x",
		"https://example.com.attacker.test/x",
		"https://example.com:8443/x",
	} {
		if d := p.Decide("click", raw); d.Allowed {
			t.Errorf("consent for https://example.com must not cover %s", raw)
		}
	}
}

func TestReadOnlyActionNeedsNoConsent(t *testing.T) {
	p := NewPolicy()
	d := p.Decide("get_text", "https://never-consented.example/x")
	if !d.Allowed {
		t.Fatalf("read-only action must not be gated: %+v", d)
	}
	if d.Reason != ReasonNotGated {
		t.Errorf("Reason = %q, want %q", d.Reason, ReasonNotGated)
	}
}

func TestLocalhostAllowedByDefaultButRevocable(t *testing.T) {
	p := NewPolicy()
	for _, raw := range []string{"http://localhost:5173/app", "http://127.0.0.1:3000/", "http://[::1]:8080/"} {
		if d := p.Decide("click", raw); !d.Allowed {
			t.Errorf("local development origin %s must work without ceremony: %+v", raw, d)
		} else if d.Reason != ReasonLocalhost {
			t.Errorf("Reason for %s = %q, want %q", raw, d.Reason, ReasonLocalhost)
		}
	}
	p.SetAllowLocalhost(false)
	if d := p.Decide("click", "http://localhost:5173/app"); d.Allowed {
		t.Fatal("the localhost default must be revocable")
	}
}

func TestSessionConsentIsSeparateFromPersistent(t *testing.T) {
	p := NewPolicy()
	_ = p.AllowForSession("https://staging.example.com")
	if d := p.Decide("click", "https://staging.example.com/x"); !d.Allowed {
		t.Fatal("session consent must permit driving")
	}
	if got := p.List(); len(got) != 0 {
		t.Errorf("session consent must not appear in the persistent list, got %v", got)
	}
	p.ClearSession()
	if d := p.Decide("click", "https://staging.example.com/x"); d.Allowed {
		t.Fatal("session consent must not survive ClearSession")
	}
}

func TestRevokeRemovesConsent(t *testing.T) {
	p := NewPolicy()
	_ = p.Allow("https://example.com")
	if err := p.Revoke("https://example.com"); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	if d := p.Decide("click", "https://example.com/x"); d.Allowed {
		t.Fatal("revoked origin must be denied")
	}
}

// An unusable URL must be denied, never waved through. A gated action whose target cannot
// be identified is exactly the case where proceeding is least safe.
func TestDecideDeniesUnresolvableTarget(t *testing.T) {
	p := NewPolicy()
	_ = p.Allow("https://example.com")
	for _, raw := range []string{"", "about:blank", "chrome://settings"} {
		d := p.Decide("click", raw)
		if d.Allowed {
			t.Errorf("unresolvable target %q must be denied", raw)
		}
		if d.Reason != ReasonUnresolvableTarget {
			t.Errorf("Reason for %q = %q, want %q", raw, d.Reason, ReasonUnresolvableTarget)
		}
	}
}

func TestAllowRejectsGarbage(t *testing.T) {
	p := NewPolicy()
	for _, raw := range []string{"", "not a url", "chrome://settings"} {
		if err := p.Allow(raw); err == nil {
			t.Errorf("Allow(%q) must reject an unusable origin", raw)
		}
	}
}

func TestListIsSortedAndCopied(t *testing.T) {
	p := NewPolicy()
	_ = p.Allow("https://b.example.com")
	_ = p.Allow("https://a.example.com")
	got := p.List()
	if len(got) != 2 || got[0] != "https://a.example.com" || got[1] != "https://b.example.com" {
		t.Fatalf("List() = %v, want sorted origins", got)
	}
	got[0] = "mutated"
	if again := p.List(); again[0] == "mutated" {
		t.Fatal("List() must return a copy the caller cannot mutate into the policy")
	}
}
