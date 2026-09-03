// consent_guard_test.go — Contract for the driving-consent guard (kaboom-05ue.2).
package toolguard

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

func newGuards() *Guards { return New(nil, context.Background(), time.Second) }

// errorPayload digs the structured error out of a tool response.
func errorPayload(t *testing.T, resp mcp.JSONRPCResponse) map[string]any {
	t.Helper()
	raw, err := json.Marshal(resp.Result)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	var envelope struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil || len(envelope.Content) == 0 {
		t.Fatalf("unexpected result shape: %s", raw)
	}
	// The text carries a human-readable line followed by the structured JSON line.
	for _, line := range strings.Split(envelope.Content[0].Text, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "{") {
			continue
		}
		var payload map[string]any
		if err := json.Unmarshal([]byte(trimmed), &payload); err == nil {
			return payload
		}
	}
	t.Fatalf("no structured error line found in: %s", envelope.Content[0].Text)
	return nil
}

func TestConsentGuardBlocksUnconsentedDriving(t *testing.T) {
	g := newGuards()
	resp, blocked := g.RequireDrivingConsent(mcp.JSONRPCRequest{}, "click", "https://bank.example.com/transfer")
	if !blocked {
		t.Fatal("driving an unconsented origin must be blocked")
	}
	payload := errorPayload(t, resp)
	if payload["error_code"] != mcp.ErrOriginNotConsented {
		t.Errorf("error_code = %v, want %v", payload["error_code"], mcp.ErrOriginNotConsented)
	}
	msg, _ := payload["message"].(string)
	if !strings.Contains(msg, "https://bank.example.com") {
		t.Errorf("refusal must name the origin so the user can grant it; got %q", msg)
	}
	// The refusal must not carry the path or query of the page being acted on (rules 7/13).
	if strings.Contains(msg, "/transfer") {
		t.Errorf("refusal leaked the URL path: %q", msg)
	}
}

// Retrying without a grant produces the same refusal, so the guard must not tell the agent
// to retry — that is how a bounded failure turns into a burned retry budget.
func TestConsentRefusalIsNotRetryable(t *testing.T) {
	g := newGuards()
	resp, _ := g.RequireDrivingConsent(mcp.JSONRPCRequest{}, "click", "https://example.com/x")
	if retryable, ok := errorPayload(t, resp)["retryable"].(bool); ok && retryable {
		t.Fatal("a consent refusal must not be advertised as retryable")
	}
}

func TestConsentGuardAllowsAfterGrant(t *testing.T) {
	g := newGuards()
	if err := g.Consent().Allow("https://example.com"); err != nil {
		t.Fatalf("Allow: %v", err)
	}
	if _, blocked := g.RequireDrivingConsent(mcp.JSONRPCRequest{}, "click", "https://example.com/page"); blocked {
		t.Fatal("a consented origin must be allowed to drive")
	}
}

func TestConsentGuardIgnoresReadOnlyActions(t *testing.T) {
	g := newGuards()
	if _, blocked := g.RequireDrivingConsent(mcp.JSONRPCRequest{}, "get_text", "https://never-granted.example/x"); blocked {
		t.Fatal("read-only actions must not require driving consent")
	}
}

func TestConsentGuardRefusesUnresolvableTarget(t *testing.T) {
	g := newGuards()
	resp, blocked := g.RequireDrivingConsent(mcp.JSONRPCRequest{}, "click", "chrome://settings")
	if !blocked {
		t.Fatal("an action whose target cannot be identified must be refused, not defaulted")
	}
	if code := errorPayload(t, resp)["error_code"]; code != mcp.ErrOriginNotConsented {
		t.Errorf("error_code = %v, want %v", code, mcp.ErrOriginNotConsented)
	}
}

// An action nobody has classified must be gated. This is the property that keeps the gate
// meaningful as the interact surface grows.
func TestConsentGuardGatesUnknownActions(t *testing.T) {
	g := newGuards()
	if _, blocked := g.RequireDrivingConsent(mcp.JSONRPCRequest{}, "teleport_user_funds", "https://example.com/x"); !blocked {
		t.Fatal("an unrecognized action must be gated by default")
	}
}

func TestConsentGuardAllowsLocalDevelopment(t *testing.T) {
	g := newGuards()
	if _, blocked := g.RequireDrivingConsent(mcp.JSONRPCRequest{}, "click", "http://localhost:5173/app"); blocked {
		t.Fatal("local development must not need a consent grant")
	}
}
