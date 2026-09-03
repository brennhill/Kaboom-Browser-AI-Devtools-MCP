// tools_configure_consent_test.go — configure(mode='consent') round trip (kaboom-05ue.2).
package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/browserconsent"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

func consentText(t *testing.T, resp mcp.JSONRPCResponse) string {
	t.Helper()
	raw, err := json.Marshal(resp.Result)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(raw)
}

// The gate is only a boundary if there is a supported way through it. Without this the
// product refuses to drive every real site and offers no remedy.
func TestConsentModeGrantsThenDrivingIsAllowed(t *testing.T) {
	policy := browserconsent.NewPolicy()
	if d := policy.Decide("click", "https://example.com/x"); d.Allowed {
		t.Fatal("precondition: origin must start unconsented")
	}

	resp := handleConfigureConsent(policy, mcp.JSONRPCRequest{},
		json.RawMessage(`{"action":"allow","origin":"https://example.com/some/path?token=secret"}`))
	body := consentText(t, resp)
	if strings.Contains(body, "token=secret") || strings.Contains(body, "/some/path") {
		t.Errorf("consent response leaked the URL path or query: %s", body)
	}

	if d := policy.Decide("click", "https://example.com/x"); !d.Allowed {
		t.Fatalf("driving must be allowed after a grant: %+v", d)
	}
}

func TestConsentModeListsAndRevokes(t *testing.T) {
	policy := browserconsent.NewPolicy()
	_ = policy.Allow("https://a.example.com")

	listed := consentText(t, handleConfigureConsent(policy, mcp.JSONRPCRequest{}, json.RawMessage(`{"action":"list"}`)))
	if !strings.Contains(listed, "https://a.example.com") {
		t.Errorf("list must report consented origins: %s", listed)
	}

	handleConfigureConsent(policy, mcp.JSONRPCRequest{}, json.RawMessage(`{"action":"revoke","origin":"https://a.example.com"}`))
	if d := policy.Decide("click", "https://a.example.com/x"); d.Allowed {
		t.Fatal("revoke must remove consent")
	}
}

func TestConsentModeSessionScopeIsSeparate(t *testing.T) {
	policy := browserconsent.NewPolicy()
	handleConfigureConsent(policy, mcp.JSONRPCRequest{},
		json.RawMessage(`{"action":"allow_session","origin":"https://staging.example.com"}`))
	if d := policy.Decide("click", "https://staging.example.com/x"); !d.Allowed {
		t.Fatal("session grant must permit driving")
	}
	listed := consentText(t, handleConfigureConsent(policy, mcp.JSONRPCRequest{}, json.RawMessage(`{"action":"list"}`)))
	if strings.Contains(listed, "staging.example.com") {
		t.Errorf("session grants must not appear in the persistent list: %s", listed)
	}
}

func TestConsentModeRejectsBadInput(t *testing.T) {
	policy := browserconsent.NewPolicy()
	for _, args := range []string{
		`{"action":"allow"}`,
		`{"action":"allow","origin":"chrome://settings"}`,
		`{"action":"teleport"}`,
	} {
		body := consentText(t, handleConfigureConsent(policy, mcp.JSONRPCRequest{}, json.RawMessage(args)))
		if !strings.Contains(body, "error_code") {
			t.Errorf("args %s must be refused, got %s", args, body)
		}
	}
}

func TestConsentModeLocalhostToggle(t *testing.T) {
	policy := browserconsent.NewPolicy()
	handleConfigureConsent(policy, mcp.JSONRPCRequest{}, json.RawMessage(`{"action":"deny_localhost"}`))
	if d := policy.Decide("click", "http://localhost:5173/x"); d.Allowed {
		t.Fatal("deny_localhost must revoke the local development default")
	}
	handleConfigureConsent(policy, mcp.JSONRPCRequest{}, json.RawMessage(`{"action":"allow_localhost"}`))
	if d := policy.Decide("click", "http://localhost:5173/x"); !d.Allowed {
		t.Fatal("allow_localhost must restore it")
	}
}
