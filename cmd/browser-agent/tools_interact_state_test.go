// Purpose: Tests for interact state save/load actions.
// Docs: docs/features/feature/interact-explore/index.md

// tools_interact_state_test.go — Tests for state management handlers with form, storage, and cookie capture.
// Verifies that state_capture and state_restore status fields are always present and unambiguous.
package main

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	act "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/interact"
)

// extractResponseData parses the JSON payload from an MCPToolResult's text content.
func extractResponseData(t *testing.T, resp mcp.JSONRPCResponse) map[string]any {
	t.Helper()
	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal MCPToolResult: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error response: %s", result.Content[0].Text)
	}
	text := result.Content[0].Text
	idx := strings.Index(text, "{")
	if idx < 0 {
		t.Fatalf("no JSON in response text: %s", text)
	}
	var data map[string]any
	if err := json.Unmarshal([]byte(text[idx:]), &data); err != nil {
		t.Fatalf("parse response JSON: %v\nraw: %s", err, text[idx:])
	}
	return data
}

// simulateExtensionConnection sends a /sync request to mark the extension as connected.
func simulateExtensionConnection(t *testing.T, env *interactHelpersTestEnv) {
	t.Helper()
	httpReq := httptest.NewRequest("POST", "/sync", strings.NewReader(`{"ext_session_id":"test"}`))
	httpReq.Header.Set("X-Kaboom-Client", "test-client")
	capture.NewSyncHandler(env.capture).HandleSync(httptest.NewRecorder(), httpReq)
}

func requireSessionStore(t *testing.T, env *interactHelpersTestEnv) {
	t.Helper()
	if env.handler.sessionStoreImpl == nil {
		t.Skip("session store unavailable in this test environment")
	}
}

func TestStateHandlers_RejectLegacyNameParam(t *testing.T) {
	t.Parallel()
	for name, call := range map[string]func(*ToolHandler, mcp.JSONRPCRequest) mcp.JSONRPCResponse{
		"save": func(h *ToolHandler, req mcp.JSONRPCRequest) mcp.JSONRPCResponse {
			return h.stateInteractHandler.HandleStateSave(req, json.RawMessage(`{"name":"old"}`))
		},
		"load": func(h *ToolHandler, req mcp.JSONRPCRequest) mcp.JSONRPCResponse {
			return h.stateInteractHandler.HandleStateLoad(req, json.RawMessage(`{"name":"old"}`))
		},
		"delete": func(h *ToolHandler, req mcp.JSONRPCRequest) mcp.JSONRPCResponse {
			return h.stateInteractHandler.HandleStateDelete(req, json.RawMessage(`{"name":"old"}`))
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			env := newInteractHelpersTestEnv(t)
			req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), ClientID: "test-client"}
			resp := call(env.handler, req)
			if !strings.Contains(string(resp.Result), mcp.ErrMissingParam) {
				t.Fatalf("expected %s, got %s", mcp.ErrMissingParam, string(resp.Result))
			}
		})
	}
}

// ============================================
// captureState — explicit status per scenario
// ============================================

func TestCaptureState_Status_PilotDisabled(t *testing.T) {
	t.Parallel()
	env := newInteractHelpersTestEnv(t)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	result := env.handler.stateInteractHandler.CaptureState(req)
	if result.Status != act.StateCaptureStatusPilotDisabled {
		t.Errorf("status = %q, want %q", result.Status, act.StateCaptureStatusPilotDisabled)
	}
	if result.Data != nil {
		t.Error("Data should be nil when pilot is disabled")
	}
}

func TestCaptureState_Status_ExtensionDisconnected(t *testing.T) {
	t.Parallel()
	env := newInteractHelpersTestEnv(t)
	env.enablePilot(t)
	env.capture.Extension().SimulateExtensionDisconnectForTest()

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	result := env.handler.stateInteractHandler.CaptureState(req)
	if result.Status != act.StateCaptureStatusExtensionDisconnected {
		t.Errorf("status = %q, want %q", result.Status, act.StateCaptureStatusExtensionDisconnected)
	}
	if result.Data != nil {
		t.Error("Data should be nil when extension is disconnected")
	}
}

// ============================================
// save_state — state_capture field always present
// ============================================

func TestSaveState_StateCapture_Captured(t *testing.T) {
	t.Parallel()
	env := newInteractHelpersTestEnv(t)
	env.enablePilot(t)
	simulateExtensionConnection(t, env)
	requireSessionStore(t, env)

	go func() {
		time.Sleep(50 * time.Millisecond)
		queries := env.capture.Queries().GetPendingQueries()
		for _, q := range queries {
			if q.Type == "execute" && strings.HasPrefix(q.CorrelationID, "state_capture_") {
				result, _ := json.Marshal(map[string]any{
					"success": true,
					"result": map[string]any{
						"form_values":     map[string]any{"username": "john", "remember": true},
						"scroll_position": map[string]any{"x": 0.0, "y": 150.0},
					},
				})
				env.capture.Queries().ApplyCommandResult(q.CorrelationID, "complete", result, "")
				return
			}
		}
	}()

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), ClientID: "test-client"}
	resp := env.handler.stateInteractHandler.HandleStateSave(req, json.RawMessage(`{"snapshot_name":"form_test"}`))
	data := extractResponseData(t, resp)

	if data["state_capture"] != "captured" {
		t.Errorf("state_capture = %v, want \"captured\"", data["state_capture"])
	}
	if data["status"] != "saved" {
		t.Errorf("status = %v, want \"saved\"", data["status"])
	}

	// Verify form_values actually persisted by loading the state
	resp2 := env.handler.stateInteractHandler.HandleStateLoad(req, json.RawMessage(`{"snapshot_name":"form_test"}`))
	loadData := extractResponseData(t, resp2)
	state, _ := loadData["state"].(map[string]any)
	if _, ok := state["form_values"]; !ok {
		t.Error("form_values should be present in persisted state")
	}
	if _, ok := state["scroll_position"]; !ok {
		t.Error("scroll_position should be present in persisted state")
	}
}

func TestSaveState_CapturesStorage(t *testing.T) {
	t.Parallel()
	env := newInteractHelpersTestEnv(t)
	env.enablePilot(t)
	simulateExtensionConnection(t, env)
	requireSessionStore(t, env)

	go func() {
		time.Sleep(50 * time.Millisecond)
		queries := env.capture.Queries().GetPendingQueries()
		for _, q := range queries {
			if q.Type == "execute" && strings.HasPrefix(q.CorrelationID, "state_capture_") {
				result, _ := json.Marshal(map[string]any{
					"success": true,
					"result": map[string]any{
						"form_values":     map[string]any{"email": "test@test.com"},
						"scroll_position": map[string]any{"x": 0.0, "y": 0.0},
						"local_storage":   map[string]any{"theme": "dark", "lang": "en"},
						"session_storage": map[string]any{"cart_id": "abc123"},
						"cookies":         map[string]any{"_ga": "GA1.2.123", "prefs": "compact"},
					},
				})
				env.capture.Queries().ApplyCommandResult(q.CorrelationID, "complete", result, "")
				return
			}
		}
	}()

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), ClientID: "test-client"}
	resp := env.handler.stateInteractHandler.HandleStateSave(req, json.RawMessage(`{"snapshot_name":"storage_test"}`))
	data := extractResponseData(t, resp)

	if data["state_capture"] != "captured" {
		t.Errorf("state_capture = %v, want \"captured\"", data["state_capture"])
	}

	// Load and verify all storage fields persisted
	resp2 := env.handler.stateInteractHandler.HandleStateLoad(req, json.RawMessage(`{"snapshot_name":"storage_test"}`))
	loadData := extractResponseData(t, resp2)
	state, _ := loadData["state"].(map[string]any)

	ls, ok := state["local_storage"].(map[string]any)
	if !ok {
		t.Fatal("local_storage should be present in persisted state")
	}
	if ls["theme"] != "dark" || ls["lang"] != "en" {
		t.Errorf("local_storage = %v, want theme=dark, lang=en", ls)
	}

	ss, ok := state["session_storage"].(map[string]any)
	if !ok {
		t.Fatal("session_storage should be present in persisted state")
	}
	if ss["cart_id"] != "abc123" {
		t.Errorf("session_storage = %v, want cart_id=abc123", ss)
	}

	cookies, ok := state["cookies"].(map[string]any)
	if !ok {
		t.Fatal("cookies should be present in persisted state")
	}
	if cookies["prefs"] != "compact" {
		t.Errorf("cookies = %v, want prefs=compact", cookies)
	}
}

func TestSaveState_ServerRedaction_RedactsSensitiveFormValues(t *testing.T) {
	t.Parallel()
	env := newInteractHelpersTestEnv(t)
	env.enablePilot(t)
	simulateExtensionConnection(t, env)
	requireSessionStore(t, env)

	rawPassword := "plain-password-value"
	rawToken := "short-token-value"
	rawAPIKey := "dev-api-key"
	rawTokenLike := "sk-" + "1234567890abcdef1234567890abcdef"

	go func() {
		time.Sleep(50 * time.Millisecond)
		queries := env.capture.Queries().GetPendingQueries()
		for _, q := range queries {
			if q.Type == "execute" && strings.HasPrefix(q.CorrelationID, "state_capture_") {
				result, _ := json.Marshal(map[string]any{
					"success": true,
					"result": map[string]any{
						"form_values": map[string]any{
							"user_password": rawPassword,
							"authToken":     rawToken,
							"apiKeyInput":   rawAPIKey,
							"notes":         rawTokenLike,
							"display_name":  "alice",
						},
					},
				})
				env.capture.Queries().ApplyCommandResult(q.CorrelationID, "complete", result, "")
				return
			}
		}
	}()

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), ClientID: "test-client"}
	resp := env.handler.stateInteractHandler.HandleStateSave(req, json.RawMessage(`{"snapshot_name":"redaction_server_side"}`))
	data := extractResponseData(t, resp)
	if data["status"] != "saved" {
		t.Fatalf("status = %v, want \"saved\"", data["status"])
	}

	savedRaw, err := env.handler.sessionStoreImpl.Load(act.StateNamespace, "redaction_server_side")
	if err != nil {
		t.Fatalf("load persisted state: %v", err)
	}
	savedJSON := string(savedRaw)
	for _, leak := range []string{rawPassword, rawToken, rawAPIKey, rawTokenLike} {
		if strings.Contains(savedJSON, leak) {
			t.Fatalf("persisted state leaked sensitive value %q: %s", leak, savedJSON)
		}
	}

	var saved map[string]any
	if err := json.Unmarshal(savedRaw, &saved); err != nil {
		t.Fatalf("unmarshal persisted state: %v", err)
	}
	formValues, ok := saved["form_values"].(map[string]any)
	if !ok {
		t.Fatalf("form_values type = %T, want map[string]any", saved["form_values"])
	}
	for _, key := range []string{"user_password", "authToken", "apiKeyInput", "notes"} {
		v, _ := formValues[key].(string)
		if !strings.Contains(v, "[REDACTED") {
			t.Errorf("form_values[%q] should be redacted, got %q", key, v)
		}
	}
	if formValues["display_name"] != "alice" {
		t.Errorf("display_name should be preserved, got %v", formValues["display_name"])
	}
}

func TestSaveState_ServerRedaction_RedactsLegacyFormValueShapes(t *testing.T) {
	t.Parallel()
	env := newInteractHelpersTestEnv(t)
	env.enablePilot(t)
	simulateExtensionConnection(t, env)
	requireSessionStore(t, env)

	rawLegacySecret := "legacy-raw-secret"
	rawLegacyToken := "sk-" + "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

	go func() {
		time.Sleep(50 * time.Millisecond)
		queries := env.capture.Queries().GetPendingQueries()
		for _, q := range queries {
			if q.Type == "execute" && strings.HasPrefix(q.CorrelationID, "state_capture_") {
				result, _ := json.Marshal(map[string]any{
					"success": true,
					"result": map[string]any{
						// Legacy/malformed shape: array instead of object.
						"form_values": []any{
							map[string]any{"authToken": rawLegacySecret},
							rawLegacyToken,
							map[string]any{
								"nested": []any{
									map[string]any{"user_password": "deep-legacy-secret"},
								},
							},
						},
					},
				})
				env.capture.Queries().ApplyCommandResult(q.CorrelationID, "complete", result, "")
				return
			}
		}
	}()

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), ClientID: "test-client"}
	resp := env.handler.stateInteractHandler.HandleStateSave(req, json.RawMessage(`{"snapshot_name":"redaction_legacy_shape"}`))
	data := extractResponseData(t, resp)
	if data["status"] != "saved" {
		t.Fatalf("status = %v, want \"saved\"", data["status"])
	}

	savedRaw, err := env.handler.sessionStoreImpl.Load(act.StateNamespace, "redaction_legacy_shape")
	if err != nil {
		t.Fatalf("load persisted state: %v", err)
	}
	savedJSON := string(savedRaw)
	for _, leak := range []string{rawLegacySecret, rawLegacyToken, "deep-legacy-secret"} {
		if strings.Contains(savedJSON, leak) {
			t.Fatalf("persisted legacy shape leaked value %q: %s", leak, savedJSON)
		}
	}
	if !strings.Contains(savedJSON, "[REDACTED") {
		t.Fatalf("expected redaction markers in persisted legacy shape, got: %s", savedJSON)
	}
}

func TestSaveState_StateCapture_SkippedPilotDisabled(t *testing.T) {
	t.Parallel()
	env := newInteractHelpersTestEnv(t)
	requireSessionStore(t, env)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	resp := env.handler.stateInteractHandler.HandleStateSave(req, json.RawMessage(`{"snapshot_name":"no_pilot"}`))
	data := extractResponseData(t, resp)

	if data["state_capture"] != "skipped_pilot_disabled" {
		t.Errorf("state_capture = %v, want \"skipped_pilot_disabled\"", data["state_capture"])
	}
	if data["status"] != "saved" {
		t.Errorf("status = %v, want \"saved\"", data["status"])
	}
}

func TestSaveState_StateCapture_SkippedExtensionDisconnected(t *testing.T) {
	t.Parallel()
	env := newInteractHelpersTestEnv(t)
	env.enablePilot(t)
	requireSessionStore(t, env)
	// Force extension to appear disconnected despite test env default connection.
	env.capture.Extension().SimulateExtensionDisconnectForTest()

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	resp := env.handler.stateInteractHandler.HandleStateSave(req, json.RawMessage(`{"snapshot_name":"no_ext"}`))
	data := extractResponseData(t, resp)

	if data["state_capture"] != "skipped_extension_disconnected" {
		t.Errorf("state_capture = %v, want \"skipped_extension_disconnected\"", data["state_capture"])
	}
	if data["status"] != "saved" {
		t.Errorf("status = %v, want \"saved\"", data["status"])
	}
}

func TestSaveState_StateCapture_SkippedErrorOnExecuteFailure(t *testing.T) {
	t.Parallel()
	env := newInteractHelpersTestEnv(t)
	env.enablePilot(t)
	simulateExtensionConnection(t, env)
	requireSessionStore(t, env)

	go func() {
		time.Sleep(50 * time.Millisecond)
		queries := env.capture.Queries().GetPendingQueries()
		for _, q := range queries {
			if q.Type == "execute" && strings.HasPrefix(q.CorrelationID, "state_capture_") {
				result, _ := json.Marshal(map[string]any{
					"success": false,
					"error":   "execution_error",
					"message": "script failed",
				})
				env.capture.Queries().ApplyCommandResult(q.CorrelationID, "complete", result, "")
				return
			}
		}
	}()

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), ClientID: "test-client"}
	resp := env.handler.stateInteractHandler.HandleStateSave(req, json.RawMessage(`{"snapshot_name":"capture_failure"}`))
	data := extractResponseData(t, resp)

	if data["state_capture"] != "skipped_error" {
		t.Errorf("state_capture = %v, want \"skipped_error\"", data["state_capture"])
	}
}

// ============================================
// load_state — state_restore field always present
// ============================================

func TestLoadState_StateRestore_Queued(t *testing.T) {
	t.Parallel()
	env := newInteractHelpersTestEnv(t)
	env.enablePilot(t)
	simulateExtensionConnection(t, env)
	requireSessionStore(t, env)

	stateData := map[string]any{
		"url": "https://example.com/form", "title": "Form Page",
		"saved_at":        time.Now().Format(time.RFC3339),
		"form_values":     map[string]any{"email": "test@test.com"},
		"scroll_position": map[string]any{"x": 0.0, "y": 100.0},
	}
	raw, _ := json.Marshal(stateData)
	if err := env.handler.sessionStoreImpl.Save("saved_states", "with_forms", raw); err != nil {
		t.Fatalf("seed state save failed: %v", err)
	}

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	resp := env.handler.stateInteractHandler.HandleStateLoad(req, json.RawMessage(`{"snapshot_name":"with_forms"}`))
	data := extractResponseData(t, resp)

	if data["state_restore"] != "queued" {
		t.Errorf("state_restore = %v, want \"queued\"", data["state_restore"])
	}
	corrID, ok := data["restore_correlation_id"].(string)
	if !ok || corrID == "" {
		t.Error("restore_correlation_id should be present when state_restore=queued")
	}
	if !strings.HasPrefix(corrID, "state_restore_") {
		t.Errorf("restore_correlation_id = %q, want prefix \"state_restore_\"", corrID)
	}

	// Verify the queued execute command contains the form values
	pqs := env.capture.Queries().GetPendingQueries()
	found := false
	for _, q := range pqs {
		if q.Type == "execute" && q.CorrelationID == corrID {
			found = true
			var params map[string]any
			json.Unmarshal(q.Params, &params)
			script, _ := params["script"].(string)
			if !strings.Contains(script, "test@test.com") {
				t.Error("restore script should contain the form values")
			}
			break
		}
	}
	if !found {
		t.Error("expected execute query with restore_correlation_id")
	}
}

func TestLoadState_RestoresStorage(t *testing.T) {
	t.Parallel()
	env := newInteractHelpersTestEnv(t)
	env.enablePilot(t)
	simulateExtensionConnection(t, env)
	requireSessionStore(t, env)

	stateData := map[string]any{
		"url": "https://example.com/app", "title": "App",
		"saved_at":        time.Now().Format(time.RFC3339),
		"form_values":     map[string]any{"search": "query"},
		"local_storage":   map[string]any{"theme": "dark", "lang": "en"},
		"session_storage": map[string]any{"cart_id": "abc123"},
		"cookies":         map[string]any{"_ga": "GA1.2.123", "prefs": "compact"},
	}
	raw, _ := json.Marshal(stateData)
	if err := env.handler.sessionStoreImpl.Save("saved_states", "with_storage", raw); err != nil {
		t.Fatalf("seed state save failed: %v", err)
	}

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	resp := env.handler.stateInteractHandler.HandleStateLoad(req, json.RawMessage(`{"snapshot_name":"with_storage"}`))
	data := extractResponseData(t, resp)

	if data["state_restore"] != "queued" {
		t.Errorf("state_restore = %v, want \"queued\"", data["state_restore"])
	}

	corrID := data["restore_correlation_id"].(string)
	pqs := env.capture.Queries().GetPendingQueries()
	for _, q := range pqs {
		if q.Type == "execute" && q.CorrelationID == corrID {
			var params map[string]any
			json.Unmarshal(q.Params, &params)
			script, _ := params["script"].(string)

			// Verify localStorage restore
			if !strings.Contains(script, "localStorage.setItem") {
				t.Error("restore script should contain localStorage.setItem calls")
			}
			if !strings.Contains(script, "dark") {
				t.Error("restore script should contain localStorage value 'dark'")
			}

			// Verify sessionStorage restore
			if !strings.Contains(script, "sessionStorage.setItem") {
				t.Error("restore script should contain sessionStorage.setItem calls")
			}
			if !strings.Contains(script, "abc123") {
				t.Error("restore script should contain sessionStorage value 'abc123'")
			}

			// Verify cookie restore
			if !strings.Contains(script, "document.cookie") {
				t.Error("restore script should contain document.cookie assignments")
			}
			if !strings.Contains(script, "compact") {
				t.Error("restore script should contain cookie value 'compact'")
			}
			return
		}
	}
	t.Error("expected execute query with restore_correlation_id")
}

func TestLoadState_StateRestore_SkippedNoData(t *testing.T) {
	t.Parallel()
	env := newInteractHelpersTestEnv(t)
	env.enablePilot(t)
	requireSessionStore(t, env)

	stateData := map[string]any{
		"url": "https://example.com/plain", "title": "Plain Page",
		"saved_at": time.Now().Format(time.RFC3339),
	}
	raw, _ := json.Marshal(stateData)
	if err := env.handler.sessionStoreImpl.Save("saved_states", "no_data", raw); err != nil {
		t.Fatalf("seed state save failed: %v", err)
	}

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	resp := env.handler.stateInteractHandler.HandleStateLoad(req, json.RawMessage(`{"snapshot_name":"no_data"}`))
	data := extractResponseData(t, resp)

	if data["state_restore"] != "skipped_no_state_data" {
		t.Errorf("state_restore = %v, want \"skipped_no_state_data\"", data["state_restore"])
	}
	if _, ok := data["restore_correlation_id"]; ok {
		t.Error("restore_correlation_id should be absent when state_restore != queued")
	}
}

func TestLoadState_StateRestore_SkippedPilotDisabled(t *testing.T) {
	t.Parallel()
	env := newInteractHelpersTestEnv(t)
	requireSessionStore(t, env)

	stateData := map[string]any{
		"url": "https://example.com/form", "title": "Form Page",
		"saved_at":    time.Now().Format(time.RFC3339),
		"form_values": map[string]any{"email": "test@test.com"},
	}
	raw, _ := json.Marshal(stateData)
	if err := env.handler.sessionStoreImpl.Save("saved_states", "has_forms", raw); err != nil {
		t.Fatalf("seed state save failed: %v", err)
	}

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	resp := env.handler.stateInteractHandler.HandleStateLoad(req, json.RawMessage(`{"snapshot_name":"has_forms"}`))
	data := extractResponseData(t, resp)

	if data["state_restore"] != "skipped_pilot_disabled" {
		t.Errorf("state_restore = %v, want \"skipped_pilot_disabled\"", data["state_restore"])
	}
}

func TestLoadState_StateRestore_SkippedExtensionDisconnected(t *testing.T) {
	t.Parallel()
	env := newInteractHelpersTestEnv(t)
	env.enablePilot(t)
	requireSessionStore(t, env)
	// Force extension to appear disconnected despite test env default connection.
	env.capture.Extension().SimulateExtensionDisconnectForTest()

	stateData := map[string]any{
		"url":             "https://example.com/form",
		"title":           "Form Page",
		"saved_at":        time.Now().Format(time.RFC3339),
		"form_values":     map[string]any{"email": "test@test.com"},
		"scroll_position": map[string]any{"x": 0.0, "y": 20.0},
	}
	raw, _ := json.Marshal(stateData)
	if err := env.handler.sessionStoreImpl.Save("saved_states", "has_forms_disconnected", raw); err != nil {
		t.Fatalf("seed state save failed: %v", err)
	}

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	resp := env.handler.stateInteractHandler.HandleStateLoad(req, json.RawMessage(`{"snapshot_name":"has_forms_disconnected"}`))
	data := extractResponseData(t, resp)

	if data["state_restore"] != "skipped_extension_disconnected" {
		t.Errorf("state_restore = %v, want \"skipped_extension_disconnected\"", data["state_restore"])
	}
}

func TestLoadState_StorageOnly_QueuesRestore(t *testing.T) {
	t.Parallel()
	env := newInteractHelpersTestEnv(t)
	env.enablePilot(t)
	simulateExtensionConnection(t, env)
	requireSessionStore(t, env)

	// State with ONLY storage data (no form_values)
	stateData := map[string]any{
		"url": "https://example.com/app", "title": "App",
		"saved_at":      time.Now().Format(time.RFC3339),
		"local_storage": map[string]any{"theme": "light"},
	}
	raw, _ := json.Marshal(stateData)
	if err := env.handler.sessionStoreImpl.Save("saved_states", "storage_only", raw); err != nil {
		t.Fatalf("seed state save failed: %v", err)
	}

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	resp := env.handler.stateInteractHandler.HandleStateLoad(req, json.RawMessage(`{"snapshot_name":"storage_only"}`))
	data := extractResponseData(t, resp)

	if data["state_restore"] != "queued" {
		t.Errorf("state_restore = %v, want \"queued\" even with only storage data", data["state_restore"])
	}
}

func TestLoadState_IncludeURL_SkipsNavigationWhenExtensionDisconnected(t *testing.T) {
	t.Parallel()
	env := newInteractHelpersTestEnv(t)
	env.enablePilot(t)
	requireSessionStore(t, env)
	// Force extension to appear disconnected despite test env default connection.
	env.capture.Extension().SimulateExtensionDisconnectForTest()

	stateData := map[string]any{
		"url":      "https://example.com/restore-target",
		"title":    "Restore Target",
		"saved_at": time.Now().Format(time.RFC3339),
	}
	raw, _ := json.Marshal(stateData)
	if err := env.handler.sessionStoreImpl.Save("saved_states", "nav_disconnected", raw); err != nil {
		t.Fatalf("seed state save failed: %v", err)
	}

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	resp := env.handler.stateInteractHandler.HandleStateLoad(req, json.RawMessage(`{"snapshot_name":"nav_disconnected","include_url":true}`))
	data := extractResponseData(t, resp)

	state, _ := data["state"].(map[string]any)
	if _, ok := state["navigation_queued"]; ok {
		t.Error("navigation_queued should be absent when extension is disconnected")
	}
}

func TestLoadState_IncludeURL_QueuesNavigationWhenConnected(t *testing.T) {
	t.Parallel()
	env := newInteractHelpersTestEnv(t)
	env.enablePilot(t)
	simulateExtensionConnection(t, env)
	requireSessionStore(t, env)

	stateData := map[string]any{
		"url":      "https://example.com/restore-target",
		"title":    "Restore Target",
		"saved_at": time.Now().Format(time.RFC3339),
	}
	raw, _ := json.Marshal(stateData)
	if err := env.handler.sessionStoreImpl.Save("saved_states", "nav_connected", raw); err != nil {
		t.Fatalf("seed state save failed: %v", err)
	}

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	resp := env.handler.stateInteractHandler.HandleStateLoad(req, json.RawMessage(`{"snapshot_name":"nav_connected","include_url":true}`))
	data := extractResponseData(t, resp)

	state, _ := data["state"].(map[string]any)
	if state["navigation_queued"] != true {
		t.Error("navigation_queued should be true when include_url is requested and extension is connected")
	}
	corrID, ok := state["correlation_id"].(string)
	if !ok || !strings.HasPrefix(corrID, "nav_") {
		t.Errorf("correlation_id = %v, want nav_* string", state["correlation_id"])
	}
}

// Pure function tests (buildStateRestoreScript, stateCaptureScript, parseCapturedStatePayload)
// live in internal/tools/interact/state_test.go.
