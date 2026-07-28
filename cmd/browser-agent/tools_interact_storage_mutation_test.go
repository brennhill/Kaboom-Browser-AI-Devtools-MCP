// Purpose: Tests for interact storage mutation actions.
// Docs: docs/features/feature/interact-explore/index.md

// tools_interact_storage_mutation_test.go — TDD coverage for granular storage/cookie mutation actions.
package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

func callInteractStorageAction(t *testing.T, env *interactHelpersTestEnv, argsJSON string) mcp.MCPToolResult {
	t.Helper()
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := env.handler.toolInteract(req, normalizeInteractArgsForAsync(argsJSON))
	return parseToolResult(t, resp)
}

func lastPendingQuery(t *testing.T, env *interactHelpersTestEnv) map[string]any {
	t.Helper()
	pq := env.capture.GetLastPendingQuery()
	if pq == nil {
		t.Fatal("expected pending query to be created")
	}
	var params map[string]any
	if err := json.Unmarshal(pq.Params, &params); err != nil {
		t.Fatalf("failed to parse pending query params: %v", err)
	}
	params["_type"] = pq.Type
	return params
}

func TestInteractStorageMutation_SetStorage_QueuesExecute(t *testing.T) {
	t.Parallel()
	env := newInteractHelpersTestEnv(t)
	env.enablePilot(t)

	result := callInteractStorageAction(t, env, `{"what":"set_storage","storage_type":"localStorage","key":"theme","value":"light"}`)
	if result.IsError {
		t.Fatalf("set_storage should succeed, got error: %s", firstText(result))
	}

	params := lastPendingQuery(t, env)
	if params["_type"] != "execute" {
		t.Fatalf("pending query type = %v, want execute", params["_type"])
	}
	script, _ := params["script"].(string)
	if !strings.Contains(script, "localStorage.setItem") {
		t.Fatalf("script should set localStorage key, got: %q", script)
	}
	if !strings.Contains(script, `"theme"`) || !strings.Contains(script, `"light"`) {
		t.Fatalf("script should include key/value, got: %q", script)
	}
}

func TestInteractStorageMutation_SetStorage_CanonicalWhat_QueuesExecute(t *testing.T) {
	t.Parallel()
	env := newInteractHelpersTestEnv(t)
	env.enablePilot(t)

	result := callInteractStorageAction(t, env, `{"what":"set_storage","storage_type":"localStorage","key":"theme","value":"light"}`)
	if result.IsError {
		t.Fatalf("set_storage via canonical what should succeed, got error: %s", firstText(result))
	}

	params := lastPendingQuery(t, env)
	if params["_type"] != "execute" {
		t.Fatalf("pending query type = %v, want execute", params["_type"])
	}
}

func TestInteractStorageMutation_ClearStorage_QueuesExecute(t *testing.T) {
	t.Parallel()
	env := newInteractHelpersTestEnv(t)
	env.enablePilot(t)

	result := callInteractStorageAction(t, env, `{"what":"clear_storage","storage_type":"sessionStorage"}`)
	if result.IsError {
		t.Fatalf("clear_storage should succeed, got error: %s", firstText(result))
	}

	params := lastPendingQuery(t, env)
	if params["_type"] != "execute" {
		t.Fatalf("pending query type = %v, want execute", params["_type"])
	}
	script, _ := params["script"].(string)
	if !strings.Contains(script, "sessionStorage.clear") {
		t.Fatalf("script should clear sessionStorage, got: %q", script)
	}
}

func TestInteractStorageMutation_DeleteStorage_QueuesExecute(t *testing.T) {
	t.Parallel()
	env := newInteractHelpersTestEnv(t)
	env.enablePilot(t)

	result := callInteractStorageAction(t, env, `{"what":"delete_storage","storage_type":"localStorage","key":"auth_token"}`)
	if result.IsError {
		t.Fatalf("delete_storage should succeed, got error: %s", firstText(result))
	}

	params := lastPendingQuery(t, env)
	if params["_type"] != "execute" {
		t.Fatalf("pending query type = %v, want execute", params["_type"])
	}
	script, _ := params["script"].(string)
	if !strings.Contains(script, "localStorage.removeItem") {
		t.Fatalf("script should remove localStorage key, got: %q", script)
	}
	if !strings.Contains(script, `"auth_token"`) {
		t.Fatalf("script should include key, got: %q", script)
	}
}

func TestInteractStorageMutation_SetCookie_QueuesExecute(t *testing.T) {
	t.Parallel()
	env := newInteractHelpersTestEnv(t)
	env.enablePilot(t)

	result := callInteractStorageAction(t, env, `{"what":"set_cookie","name":"debug","value":"true","domain":".example.com","path":"/"}`)
	if result.IsError {
		t.Fatalf("set_cookie should succeed, got error: %s", firstText(result))
	}

	params := lastPendingQuery(t, env)
	if params["_type"] != "execute" {
		t.Fatalf("pending query type = %v, want execute", params["_type"])
	}
	script, _ := params["script"].(string)
	if !strings.Contains(script, "document.cookie") || !strings.Contains(script, "debug=true") {
		t.Fatalf("script should set cookie, got: %q", script)
	}
	if !strings.Contains(script, "domain=.example.com") || !strings.Contains(script, "path=/") {
		t.Fatalf("script should include domain/path attributes, got: %q", script)
	}
}

func TestInteractStorageMutation_DeleteCookie_QueuesExecute(t *testing.T) {
	t.Parallel()
	env := newInteractHelpersTestEnv(t)
	env.enablePilot(t)

	result := callInteractStorageAction(t, env, `{"what":"delete_cookie","name":"_ga","domain":".example.com","path":"/"}`)
	if result.IsError {
		t.Fatalf("delete_cookie should succeed, got error: %s", firstText(result))
	}

	params := lastPendingQuery(t, env)
	if params["_type"] != "execute" {
		t.Fatalf("pending query type = %v, want execute", params["_type"])
	}
	script, _ := params["script"].(string)
	if !strings.Contains(script, "document.cookie") || !strings.Contains(script, "_ga=; expires=Thu, 01 Jan 1970") {
		t.Fatalf("script should delete cookie, got: %q", script)
	}
	if !strings.Contains(script, "domain=.example.com") || !strings.Contains(script, "path=/") {
		t.Fatalf("script should include domain/path attributes, got: %q", script)
	}
}

func TestInteractStorageMutation_DeleteCookie_CanonicalWhat_QueuesExecute(t *testing.T) {
	t.Parallel()
	env := newInteractHelpersTestEnv(t)
	env.enablePilot(t)

	result := callInteractStorageAction(t, env, `{"what":"delete_cookie","name":"_ga","domain":".example.com","path":"/"}`)
	if result.IsError {
		t.Fatalf("delete_cookie via canonical what should succeed, got error: %s", firstText(result))
	}

	params := lastPendingQuery(t, env)
	if params["_type"] != "execute" {
		t.Fatalf("pending query type = %v, want execute", params["_type"])
	}
}

func TestInteractStorageMutation_SetStorage_MissingKey(t *testing.T) {
	t.Parallel()
	env := newInteractHelpersTestEnv(t)
	env.enablePilot(t)

	result := callInteractStorageAction(t, env, `{"what":"set_storage","storage_type":"localStorage","value":"light"}`)
	if !result.IsError {
		t.Fatal("set_storage without key should return error")
	}
	text := firstText(result)
	if !strings.Contains(text, "missing_param") || !strings.Contains(text, "key") {
		t.Fatalf("expected missing_param error for key, got: %s", text)
	}
}

func TestInteractStorageMutation_DeleteStorage_MissingKey(t *testing.T) {
	t.Parallel()
	env := newInteractHelpersTestEnv(t)
	env.enablePilot(t)

	result := callInteractStorageAction(t, env, `{"what":"delete_storage","storage_type":"localStorage"}`)
	if !result.IsError {
		t.Fatal("delete_storage without key should return error")
	}
	text := firstText(result)
	if !strings.Contains(text, "missing_param") || !strings.Contains(text, "key") {
		t.Fatalf("expected missing_param error for key, got: %s", text)
	}
}

func TestInteractStorageMutation_DeleteCookie_MissingName(t *testing.T) {
	t.Parallel()
	env := newInteractHelpersTestEnv(t)
	env.enablePilot(t)

	result := callInteractStorageAction(t, env, `{"what":"delete_cookie"}`)
	if !result.IsError {
		t.Fatal("delete_cookie without name should return error")
	}
	text := firstText(result)
	if !strings.Contains(text, "missing_param") || !strings.Contains(text, "name") {
		t.Fatalf("expected missing_param error for name, got: %s", text)
	}
}

func TestInteractStorageMutation_SetStorage_InvalidStorageType(t *testing.T) {
	t.Parallel()
	env := newInteractHelpersTestEnv(t)
	env.enablePilot(t)

	result := callInteractStorageAction(t, env, `{"what":"set_storage","storage_type":"cookieStorage","key":"theme","value":"light"}`)
	if !result.IsError {
		t.Fatal("set_storage with invalid storage_type should return error")
	}
	text := firstText(result)
	if !strings.Contains(text, "invalid_param") || !strings.Contains(text, "storage_type") {
		t.Fatalf("expected invalid_param error for storage_type, got: %s", text)
	}
}
