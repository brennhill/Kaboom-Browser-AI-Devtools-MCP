// Purpose: Tests for interact navigation in insecure proxy mode.
// Docs: docs/features/feature/interact-explore/index.md

package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capturefixture"
)

func TestHandleBrowserActionNavigate_InsecureSchemeRequiresSecurityMode(t *testing.T) {
	t.Parallel()
	env := newInteractTestEnv(t)
	capturefixture.SetPilot(env.capture, true)

	result, ok := env.callInteract(t, `{"what":"navigate","url":"kaboom-insecure://https://example.com"}`)
	if !ok {
		t.Fatal("navigate should return result")
	}
	if !result.IsError {
		t.Fatal("navigate with kaboom-insecure scheme should fail when security mode is normal")
	}
	if text := firstText(result); !strings.Contains(strings.ToLower(text), "security_mode") {
		t.Fatalf("error should mention security_mode enablement, got: %s", text)
	}

	if pq := lastPendingQuerySnapshot(env.capture.Queries()); pq != nil {
		t.Fatalf("no pending query should be queued on validation failure, got type=%s", pq.Type)
	}
}

func TestHandleBrowserActionNavigate_RewritesKaboomInsecureURL(t *testing.T) {
	t.Parallel()
	env := newInteractTestEnv(t)
	capturefixture.SetPilot(env.capture, true)

	enable := callConfigureRaw(env.handler, `{"what":"security_mode","mode":"insecure_proxy","confirm":true}`)
	enableResult := parseToolResult(t, enable)
	if enableResult.IsError {
		t.Fatalf("security mode enable failed: %s", firstText(enableResult))
	}

	result, ok := env.callInteract(t, `{"what":"navigate","url":"kaboom-insecure://https://example.com/path?q=1#frag"}`)
	if !ok {
		t.Fatal("navigate should return result")
	}
	if result.IsError {
		t.Fatalf("navigate should succeed with insecure mode enabled, got: %s", firstText(result))
	}

	pq := lastPendingQuerySnapshot(env.capture.Queries())
	if pq == nil {
		t.Fatal("navigate should create pending query")
	}
	if pq.Type != "browser_action" {
		t.Fatalf("pending query type = %q, want browser_action", pq.Type)
	}

	var params map[string]any
	if err := json.Unmarshal(pq.Params, &params); err != nil {
		t.Fatalf("unmarshal params: %v", err)
	}
	if got, _ := params["action"].(string); got != "navigate" {
		t.Fatalf("action = %q, want navigate", got)
	}
	urlValue, _ := params["url"].(string)
	if !strings.Contains(urlValue, "http://127.0.0.1:7890/insecure-proxy?target=") {
		t.Fatalf("rewritten url = %q, want insecure proxy url", urlValue)
	}
	if !strings.Contains(urlValue, "https%3A%2F%2Fexample.com%2Fpath%3Fq%3D1%23frag") {
		t.Fatalf("rewritten url missing encoded target, got: %q", urlValue)
	}
}

func TestHandleBrowserActionNewTab_RewritesKaboomInsecureURL(t *testing.T) {
	t.Parallel()
	env := newInteractTestEnv(t)
	capturefixture.SetPilot(env.capture, true)

	enable := callConfigureRaw(env.handler, `{"what":"security_mode","mode":"insecure_proxy","confirm":true}`)
	if parseToolResult(t, enable).IsError {
		t.Fatalf("security mode enable failed: %s", firstText(parseToolResult(t, enable)))
	}

	result, ok := env.callInteract(t, `{"what":"new_tab","url":"kaboom-insecure://https://example.org"}`)
	if !ok {
		t.Fatal("new_tab should return result")
	}
	if result.IsError {
		t.Fatalf("new_tab should succeed with insecure mode enabled, got: %s", firstText(result))
	}

	pq := lastPendingQuerySnapshot(env.capture.Queries())
	if pq == nil {
		t.Fatal("new_tab should create pending query")
	}

	var params map[string]any
	if err := json.Unmarshal(pq.Params, &params); err != nil {
		t.Fatalf("unmarshal params: %v", err)
	}
	urlValue, _ := params["url"].(string)
	if !strings.Contains(urlValue, "http://127.0.0.1:7890/insecure-proxy?target=https%3A%2F%2Fexample.org") {
		t.Fatalf("new_tab rewritten url = %q, want insecure proxy URL", urlValue)
	}
}
