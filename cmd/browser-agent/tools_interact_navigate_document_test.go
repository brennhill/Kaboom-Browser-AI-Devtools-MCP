// Purpose: Validate interact(what="navigate_and_document") workflow behavior and composable enrichment.
// Why: Prevents regressions in the single-call navigation + documentation workflow for multi-page apps.
// Docs: docs/features/feature/interact-explore/index.md

package main

import (
	"encoding/base64"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
)

func TestInteract_NavigateAndDocument_InWhatEnum(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	tools := h.ToolsList()
	var interactSchema map[string]any
	for _, tool := range tools {
		if tool.Name == "interact" {
			interactSchema = tool.InputSchema
			break
		}
	}
	if interactSchema == nil {
		t.Fatal("interact tool not found in ToolsList()")
	}

	props, ok := interactSchema["properties"].(map[string]any)
	if !ok {
		t.Fatal("interact schema missing properties")
	}
	whatProp, ok := props["what"].(map[string]any)
	if !ok {
		t.Fatal("interact schema missing 'what' property")
	}
	enumValues, ok := whatProp["enum"].([]string)
	if !ok {
		t.Fatal("'what' property missing enum")
	}

	found := false
	for _, v := range enumValues {
		if v == "navigate_and_document" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("'what' enum should include 'navigate_and_document', got: %v", enumValues)
	}
}

func TestInteract_NavigateAndDocument_ModeSpec_Present(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callConfigureRaw(h, `{"what":"describe_capabilities","tool":"interact","mode":"navigate_and_document"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("describe_capabilities for interact/navigate_and_document should succeed, got: %s", result.Content[0].Text)
	}
}

func TestNavigateAndDocument_URLChangeTimeout(t *testing.T) {
	t.Parallel()
	env := newToolTestEnv(t)
	env.capture.SetPilotEnabled(true)
	env.capture.SimulateExtensionConnectForTest()
	env.capture.SetTrackingStatusForTest(42, "https://example.com/old")

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	args := json.RawMessage(`{"selector":"a.nav","wait_for_url_change":true,"wait_for_stable":false,"timeout_ms":50}`)

	var resp mcp.JSONRPCResponse
	done := make(chan struct{})
	go func() {
		resp = env.handler.interactAction().HandleNavigateAndDocument(req, args)
		close(done)
	}()

	clickQuery := waitForPendingQuery(t, env.capture, func(q queries.PendingQueryResponse) bool {
		if q.Type != "dom_action" {
			return false
		}
		var payload map[string]any
		_ = json.Unmarshal(q.Params, &payload)
		return payload["action"] == "click"
	})
	env.capture.ApplyCommandResult(clickQuery.CorrelationID, "complete", json.RawMessage(`{"success":true}`), "")

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("handleNavigateAndDocument timed out")
	}

	assertIsError(t, resp, "URL did not change")
	result := parseToolResult(t, resp)
	traceMeta, ok := result.Metadata["workflow_trace"].(map[string]any)
	if !ok {
		t.Fatalf("expected workflow_trace metadata on URL timeout response, got: %#v", result.Metadata)
	}
	if traceMeta["status"] != "failed" {
		t.Fatalf("workflow_trace.status = %v, want failed", traceMeta["status"])
	}
	stages, _ := traceMeta["stages"].([]any)
	if len(stages) < 2 {
		t.Fatalf("expected >=2 stages in workflow_trace, got: %#v", traceMeta["stages"])
	}
}

func TestNavigateAndDocument_AppendsPageContext(t *testing.T) {
	t.Parallel()
	env := newToolTestEnv(t)
	env.capture.SetPilotEnabled(true)
	env.capture.SimulateExtensionConnectForTest()
	env.capture.UpdateTrackedTab(42, "https://example.com/old", "Old")

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	args := json.RawMessage(`{"selector":"a.nav","wait_for_url_change":true,"wait_for_stable":false,"timeout_ms":500}`)

	var resp mcp.JSONRPCResponse
	done := make(chan struct{})
	go func() {
		resp = env.handler.interactAction().HandleNavigateAndDocument(req, args)
		close(done)
	}()

	clickQuery := waitForPendingQuery(t, env.capture, func(q queries.PendingQueryResponse) bool {
		if q.Type != "dom_action" {
			return false
		}
		var payload map[string]any
		_ = json.Unmarshal(q.Params, &payload)
		return payload["action"] == "click"
	})
	env.capture.ApplyCommandResult(clickQuery.CorrelationID, "complete", json.RawMessage(`{"success":true}`), "")
	env.capture.UpdateTrackedTab(42, "https://example.com/new", "New")

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("handleNavigateAndDocument timed out")
	}

	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("navigate_and_document should succeed, got: %s", firstText(result))
	}

	var hasPageContext bool
	for _, block := range result.Content {
		if block.Type != "text" {
			continue
		}
		if strings.Contains(block.Text, "--- Page Context ---") &&
			strings.Contains(block.Text, "https://example.com/new") &&
			strings.Contains(block.Text, `"title":"New"`) {
			hasPageContext = true
			break
		}
	}
	if !hasPageContext {
		t.Fatalf("expected page context block with new url/title, got blocks: %#v", result.Content)
	}
	pageCtx, ok := result.Metadata["page_context"].(map[string]any)
	if !ok {
		t.Fatalf("expected metadata.page_context map, got: %#v", result.Metadata["page_context"])
	}
	if got := pageCtx["url"]; got != "https://example.com/new" {
		t.Fatalf("metadata.page_context.url = %v, want https://example.com/new", got)
	}
	if got := pageCtx["title"]; got != "New" {
		t.Fatalf("metadata.page_context.title = %v, want New", got)
	}
	traceMeta, ok := result.Metadata["workflow_trace"].(map[string]any)
	if !ok {
		t.Fatalf("expected metadata.workflow_trace map, got: %#v", result.Metadata["workflow_trace"])
	}
	if _, ok := traceMeta["trace_id"].(string); !ok {
		t.Fatalf("workflow_trace.trace_id missing: %#v", traceMeta)
	}
	if traceMeta["status"] != "success" {
		t.Fatalf("workflow_trace.status = %v, want success", traceMeta["status"])
	}
	stages, _ := traceMeta["stages"].([]any)
	if len(stages) < 3 {
		t.Fatalf("expected >=3 stages in workflow trace, got: %#v", traceMeta["stages"])
	}
}

func TestNavigateAndDocument_TabIDMismatchReturnsError(t *testing.T) {
	t.Parallel()
	env := newToolTestEnv(t)
	env.capture.SetPilotEnabled(true)
	env.capture.SimulateExtensionConnectForTest()
	env.capture.SetTrackingStatusForTest(42, "https://example.com/old")

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	args := json.RawMessage(`{"selector":"a.nav","tab_id":99,"wait_for_url_change":true,"wait_for_stable":false}`)

	resp := env.handler.interactAction().HandleNavigateAndDocument(req, args)
	assertIsError(t, resp, "tracked tab_id")
	result := parseToolResult(t, resp)
	traceMeta, ok := result.Metadata["workflow_trace"].(map[string]any)
	if !ok {
		t.Fatalf("expected workflow_trace metadata on tab mismatch response, got: %#v", result.Metadata)
	}
	if traceMeta["status"] != "failed" {
		t.Fatalf("workflow_trace.status = %v, want failed", traceMeta["status"])
	}

	if len(env.capture.GetPendingQueries()) != 0 {
		t.Fatalf("tab mismatch should fail before dispatching click, pending=%d", len(env.capture.GetPendingQueries()))
	}
}

func TestNavigateAndDocument_TabIDRequiresTracking(t *testing.T) {
	t.Parallel()
	env := newToolTestEnv(t)
	env.capture.SetPilotEnabled(true)
	env.capture.SimulateExtensionConnectForTest()

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	args := json.RawMessage(`{"selector":"a.nav","tab_id":42,"wait_for_url_change":true,"wait_for_stable":false}`)

	resp := env.handler.interactAction().HandleNavigateAndDocument(req, args)
	assertIsError(t, resp, "requires an actively tracked tab")

	if len(env.capture.GetPendingQueries()) != 0 {
		t.Fatalf("missing tracked tab should fail before dispatching click, pending=%d", len(env.capture.GetPendingQueries()))
	}
}

func TestNavigateAndDocument_TimeoutBudgetExhaustedBeforeStable(t *testing.T) {
	t.Parallel()
	env := newToolTestEnv(t)
	env.capture.SetPilotEnabled(true)
	env.capture.SimulateExtensionConnectForTest()
	env.capture.SetTrackingStatusForTest(42, "https://example.com/old")

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	args := json.RawMessage(`{"selector":"a.nav","timeout_ms":40,"wait_for_url_change":false,"wait_for_stable":true}`)

	var resp mcp.JSONRPCResponse
	done := make(chan struct{})
	go func() {
		resp = env.handler.interactAction().HandleNavigateAndDocument(req, args)
		close(done)
	}()

	clickQuery := waitForPendingQuery(t, env.capture, func(q queries.PendingQueryResponse) bool {
		if q.Type != "dom_action" {
			return false
		}
		var payload map[string]any
		_ = json.Unmarshal(q.Params, &payload)
		return payload["action"] == "click"
	})

	// Consume the entire workflow budget before click completes.
	time.Sleep(90 * time.Millisecond)
	env.capture.ApplyCommandResult(clickQuery.CorrelationID, "complete", json.RawMessage(`{"success":true}`), "")

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("handleNavigateAndDocument timed out")
	}

	result := parseToolResult(t, resp)
	traceMeta, ok := result.Metadata["workflow_trace"].(map[string]any)
	if !ok {
		t.Fatalf("expected workflow_trace metadata on timeout/pending response, got: %#v", result.Metadata)
	}
	status, _ := traceMeta["status"].(string)
	if status != "pending" && status != "failed" {
		t.Fatalf("workflow_trace.status = %v, want pending|failed", traceMeta["status"])
	}
	if status == "failed" && !strings.Contains(firstText(result), "timeout_ms exhausted before wait_for_stable") {
		t.Fatalf("expected timeout error details when failed status, got: %s", firstText(result))
	}

	for _, q := range env.capture.GetPendingQueries() {
		if q.Type != "dom_action" {
			continue
		}
		var payload map[string]any
		_ = json.Unmarshal(q.Params, &payload)
		if payload["action"] == "wait_for_stable" {
			t.Fatal("wait_for_stable should not be queued when workflow timeout budget is exhausted")
		}
	}
}

func TestInteract_NavigateAndDocument_IncludeScreenshot(t *testing.T) {
	t.Parallel()
	env := newToolTestEnv(t)
	env.capture.SetPilotEnabled(true)
	httpReq := httptest.NewRequest("POST", "/sync", strings.NewReader(`{"ext_session_id":"test"}`))
	httpReq.Header.Set("X-Kaboom-Client", "test-client")
	env.capture.HandleSync(httptest.NewRecorder(), httpReq)
	env.capture.SetTrackingStatusForTest(42, "https://example.com")

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	args := json.RawMessage(`{"what":"navigate_and_document","selector":"button","wait_for_url_change":false,"wait_for_stable":false,"include_screenshot":true}`)

	var resp mcp.JSONRPCResponse
	done := make(chan struct{})
	go func() {
		resp = env.handler.toolInteract(req, args)
		close(done)
	}()

	clickQuery := waitForPendingQuery(t, env.capture, func(q queries.PendingQueryResponse) bool {
		if q.Type != "dom_action" {
			return false
		}
		var payload map[string]any
		_ = json.Unmarshal(q.Params, &payload)
		return payload["action"] == "click"
	})
	env.capture.ApplyCommandResult(clickQuery.CorrelationID, "complete", json.RawMessage(`{"success":true}`), "")

	screenshotQuery := waitForPendingQuery(t, env.capture, func(q queries.PendingQueryResponse) bool {
		return q.Type == "screenshot"
	})
	fakeImage := base64.StdEncoding.EncodeToString([]byte("navigate-and-document-shot"))
	screenshotPayload, _ := json.Marshal(map[string]any{
		"filename": "navigate-and-document.jpg",
		"path":     "/tmp/navigate-and-document.jpg",
		"data_url": "data:image/jpeg;base64," + fakeImage,
	})
	env.capture.SetQueryResult(screenshotQuery.ID, screenshotPayload)

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("toolInteract timed out for navigate_and_document")
	}

	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("navigate_and_document with include_screenshot should succeed, got: %s", firstText(result))
	}
	var hasImage bool
	for _, block := range result.Content {
		if block.Type == "image" && block.Data == fakeImage {
			hasImage = true
			break
		}
	}
	if !hasImage {
		t.Fatalf("expected image block in navigate_and_document response, got: %#v", result.Content)
	}
}

func waitForPendingQuery(t *testing.T, cap *capture.Store, match func(queries.PendingQueryResponse) bool) queries.PendingQueryResponse {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		for _, q := range cap.GetPendingQueries() {
			if match(q) {
				return q
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timed out waiting for pending query")
	return queries.PendingQueryResponse{}
}
