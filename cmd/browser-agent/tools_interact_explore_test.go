// Purpose: Validate interact(what="explore_page") compound action.
// Why: Prevents regressions in the single-call page exploration that combines
// screenshot, interactive elements, page metadata, readable text, and navigation links.
// Docs: docs/features/feature/interact-explore/index.md

// tools_interact_explore_test.go — Tests for interact(what="explore_page") action.
package main

import (
	"net/http/httptest"
	"strings"
	"testing"
)

// ============================================
// Dispatch Tests
// ============================================

func TestInteract_ExplorePage_DispatchesPendingQuery(t *testing.T) {
	t.Parallel()
	h, _, cap := makeToolHandler(t)
	cap.Extension().SetPilotEnabled(true)
	httpReq := httptest.NewRequest("POST", "/sync", strings.NewReader(`{"ext_session_id":"test"}`))
	httpReq.Header.Set("X-Kaboom-Client", "test-client")
	cap.HandleSync(httptest.NewRecorder(), httpReq)
	cap.Extension().SetTrackingStatusForTest(42, "https://example.com")

	resp := callInteractRaw(h, `{"what":"explore_page"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("explore_page should queue successfully, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	if data["status"] != "queued" {
		t.Errorf("status = %v, want 'queued'", data["status"])
	}
	corr, _ := data["correlation_id"].(string)
	if !strings.HasPrefix(corr, "explore_page_") {
		t.Errorf("correlation_id should start with 'explore_page_', got: %s", corr)
	}

	pq := cap.Queries().GetLastPendingQuery()
	if pq == nil {
		t.Fatal("expected pending query to be created")
	}
	if pq.Type != "explore_page" {
		t.Errorf("pending query type = %q, want 'explore_page'", pq.Type)
	}

	assertSnakeCaseFields(t, string(resp.Result))
}

func TestInteract_ExplorePage_NoURL_UsesCurrentTab(t *testing.T) {
	t.Parallel()
	h, _, cap := makeToolHandler(t)
	cap.Extension().SetPilotEnabled(true)
	httpReq := httptest.NewRequest("POST", "/sync", strings.NewReader(`{"ext_session_id":"test"}`))
	httpReq.Header.Set("X-Kaboom-Client", "test-client")
	cap.HandleSync(httptest.NewRecorder(), httpReq)
	cap.Extension().SetTrackingStatusForTest(42, "https://example.com")

	resp := callInteractRaw(h, `{"what":"explore_page"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("explore_page without URL should succeed, got: %s", result.Content[0].Text)
	}

	pq := cap.Queries().GetLastPendingQuery()
	if pq == nil {
		t.Fatal("expected pending query to be created")
	}

	// Params should NOT contain a url field (uses current tab)
	paramsStr := string(pq.Params)
	if strings.Contains(paramsStr, `"url"`) {
		t.Errorf("pending query params should not contain url when not provided, got: %s", paramsStr)
	}
}

func TestInteract_ExplorePage_WithURL_IncludesNavigate(t *testing.T) {
	t.Parallel()
	h, _, cap := makeToolHandler(t)
	cap.Extension().SetPilotEnabled(true)
	httpReq := httptest.NewRequest("POST", "/sync", strings.NewReader(`{"ext_session_id":"test"}`))
	httpReq.Header.Set("X-Kaboom-Client", "test-client")
	cap.HandleSync(httptest.NewRecorder(), httpReq)
	cap.Extension().SetTrackingStatusForTest(42, "https://example.com")

	resp := callInteractRaw(h, `{"what":"explore_page","url":"https://example.com"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("explore_page with URL should succeed, got: %s", result.Content[0].Text)
	}

	pq := cap.Queries().GetLastPendingQuery()
	if pq == nil {
		t.Fatal("expected pending query to be created")
	}

	// Params should contain the url field
	paramsStr := string(pq.Params)
	if !strings.Contains(paramsStr, `"url"`) {
		t.Errorf("pending query params should contain url when provided, got: %s", paramsStr)
	}
	if !strings.Contains(paramsStr, "https://example.com") {
		t.Errorf("pending query params should contain the actual URL, got: %s", paramsStr)
	}
}

func TestInteract_ExplorePage_ForwardsParams(t *testing.T) {
	t.Parallel()
	h, _, cap := makeToolHandler(t)
	cap.Extension().SetPilotEnabled(true)
	httpReq := httptest.NewRequest("POST", "/sync", strings.NewReader(`{"ext_session_id":"test"}`))
	httpReq.Header.Set("X-Kaboom-Client", "test-client")
	cap.HandleSync(httptest.NewRecorder(), httpReq)
	cap.Extension().SetTrackingStatusForTest(42, "https://example.com")

	resp := callInteractRaw(h, `{"what":"explore_page","visible_only":true,"limit":50}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("explore_page with params should succeed, got: %s", result.Content[0].Text)
	}

	pq := cap.Queries().GetLastPendingQuery()
	if pq == nil {
		t.Fatal("expected pending query to be created")
	}
	if pq.Type != "explore_page" {
		t.Errorf("pending query type = %q, want 'explore_page'", pq.Type)
	}
}

// ============================================
// Mode Spec Tests (via describe_capabilities)
// ============================================

func TestInteract_ExplorePage_ModeSpec_Present(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callConfigureRaw(h, `{"what":"describe_capabilities","tool":"interact","mode":"explore_page"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("describe_capabilities for interact/explore_page should succeed, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	if data == nil {
		t.Fatal("describe_capabilities should return non-nil data")
	}
}

// ============================================
// Schema Tests
// ============================================

func TestInteract_ExplorePage_InWhatEnum(t *testing.T) {
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
		if v == "explore_page" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("'what' enum should include 'explore_page', got: %v", enumValues)
	}
}

func TestInteract_ExplorePage_InValidActions(t *testing.T) {
	t.Parallel()
	_, _, _ = makeToolHandler(t)

	validActions := getValidInteractActions()
	if !strings.Contains(validActions, "explore_page") {
		t.Errorf("valid interact actions should include 'explore_page': %s", validActions)
	}
}

// ============================================
// URL Validation (Security)
// ============================================

func TestInteract_ExplorePage_JavascriptURL_Rejected(t *testing.T) {
	t.Parallel()
	h, _, cap := makeToolHandler(t)
	cap.Extension().SetPilotEnabled(true)
	httpReq := httptest.NewRequest("POST", "/sync", strings.NewReader(`{"ext_session_id":"test"}`))
	httpReq.Header.Set("X-Kaboom-Client", "test-client")
	cap.HandleSync(httptest.NewRecorder(), httpReq)
	cap.Extension().SetTrackingStatusForTest(42, "https://example.com")

	resp := callInteractRaw(h, `{"what":"explore_page","url":"javascript:alert(1)"}`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("javascript: URL should be rejected")
	}
	text := result.Content[0].Text
	if !strings.Contains(text, "http") && !strings.Contains(text, "https") {
		t.Errorf("error should mention http/https requirement, got: %s", text)
	}
}

func TestInteract_ExplorePage_DataURL_Rejected(t *testing.T) {
	t.Parallel()
	h, _, cap := makeToolHandler(t)
	cap.Extension().SetPilotEnabled(true)
	httpReq := httptest.NewRequest("POST", "/sync", strings.NewReader(`{"ext_session_id":"test"}`))
	httpReq.Header.Set("X-Kaboom-Client", "test-client")
	cap.HandleSync(httptest.NewRecorder(), httpReq)
	cap.Extension().SetTrackingStatusForTest(42, "https://example.com")

	resp := callInteractRaw(h, `{"what":"explore_page","url":"data:text/html,<script>alert(1)</script>"}`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("data: URL should be rejected")
	}
}

func TestInteract_ExplorePage_ChromeURL_Rejected(t *testing.T) {
	t.Parallel()
	h, _, cap := makeToolHandler(t)
	cap.Extension().SetPilotEnabled(true)
	httpReq := httptest.NewRequest("POST", "/sync", strings.NewReader(`{"ext_session_id":"test"}`))
	httpReq.Header.Set("X-Kaboom-Client", "test-client")
	cap.HandleSync(httptest.NewRecorder(), httpReq)
	cap.Extension().SetTrackingStatusForTest(42, "https://example.com")

	resp := callInteractRaw(h, `{"what":"explore_page","url":"chrome://settings"}`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("chrome: URL should be rejected")
	}
}

func TestInteract_ExplorePage_FileURL_Rejected(t *testing.T) {
	t.Parallel()
	h, _, cap := makeToolHandler(t)
	cap.Extension().SetPilotEnabled(true)
	httpReq := httptest.NewRequest("POST", "/sync", strings.NewReader(`{"ext_session_id":"test"}`))
	httpReq.Header.Set("X-Kaboom-Client", "test-client")
	cap.HandleSync(httptest.NewRecorder(), httpReq)
	cap.Extension().SetTrackingStatusForTest(42, "https://example.com")

	resp := callInteractRaw(h, `{"what":"explore_page","url":"file:///etc/passwd"}`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("file: URL should be rejected")
	}
}

func TestInteract_ExplorePage_HTTPURL_Accepted(t *testing.T) {
	t.Parallel()
	h, _, cap := makeToolHandler(t)
	cap.Extension().SetPilotEnabled(true)
	httpReq := httptest.NewRequest("POST", "/sync", strings.NewReader(`{"ext_session_id":"test"}`))
	httpReq.Header.Set("X-Kaboom-Client", "test-client")
	cap.HandleSync(httptest.NewRecorder(), httpReq)
	cap.Extension().SetTrackingStatusForTest(42, "https://example.com")

	resp := callInteractRaw(h, `{"what":"explore_page","url":"http://example.com"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("http: URL should be accepted, got error: %s", result.Content[0].Text)
	}
}

func TestInteract_ExplorePage_MalformedURL_Rejected(t *testing.T) {
	t.Parallel()
	h, _, cap := makeToolHandler(t)
	cap.Extension().SetPilotEnabled(true)
	httpReq := httptest.NewRequest("POST", "/sync", strings.NewReader(`{"ext_session_id":"test"}`))
	httpReq.Header.Set("X-Kaboom-Client", "test-client")
	cap.HandleSync(httptest.NewRecorder(), httpReq)
	cap.Extension().SetTrackingStatusForTest(42, "https://example.com")

	resp := callInteractRaw(h, `{"what":"explore_page","url":"://bad"}`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("malformed URL (://bad) should be rejected")
	}
}

func TestInteract_ExplorePage_BareDomain_Rejected(t *testing.T) {
	t.Parallel()
	h, _, cap := makeToolHandler(t)
	cap.Extension().SetPilotEnabled(true)
	httpReq := httptest.NewRequest("POST", "/sync", strings.NewReader(`{"ext_session_id":"test"}`))
	httpReq.Header.Set("X-Kaboom-Client", "test-client")
	cap.HandleSync(httptest.NewRecorder(), httpReq)
	cap.Extension().SetTrackingStatusForTest(42, "https://example.com")

	resp := callInteractRaw(h, `{"what":"explore_page","url":"example.com"}`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("bare domain (example.com) should be rejected — requires http/https scheme")
	}
}

func TestInteract_ExplorePage_VbscriptURL_Rejected(t *testing.T) {
	t.Parallel()
	h, _, cap := makeToolHandler(t)
	cap.Extension().SetPilotEnabled(true)
	httpReq := httptest.NewRequest("POST", "/sync", strings.NewReader(`{"ext_session_id":"test"}`))
	httpReq.Header.Set("X-Kaboom-Client", "test-client")
	cap.HandleSync(httptest.NewRecorder(), httpReq)
	cap.Extension().SetTrackingStatusForTest(42, "https://example.com")

	resp := callInteractRaw(h, `{"what":"explore_page","url":"vbscript:alert(1)"}`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("vbscript: URL should be rejected")
	}
}

func TestInteract_ExplorePage_BlobURL_Rejected(t *testing.T) {
	t.Parallel()
	h, _, cap := makeToolHandler(t)
	cap.Extension().SetPilotEnabled(true)
	httpReq := httptest.NewRequest("POST", "/sync", strings.NewReader(`{"ext_session_id":"test"}`))
	httpReq.Header.Set("X-Kaboom-Client", "test-client")
	cap.HandleSync(httptest.NewRecorder(), httpReq)
	cap.Extension().SetTrackingStatusForTest(42, "https://example.com")

	resp := callInteractRaw(h, `{"what":"explore_page","url":"blob:http://example.com/abc"}`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("blob: URL should be rejected")
	}
}

// ============================================
// Response Structure
// ============================================

func TestInteract_ExplorePage_ErrorResponseStructure(t *testing.T) {
	t.Parallel()
	// Without pilot/extension setup, this tests the error response format.
	h, _, _ := makeToolHandler(t)

	resp := callInteractRaw(h, `{"what":"explore_page"}`)
	if resp.Result == nil {
		t.Fatal("interact(explore_page) returned nil result")
	}

	result := parseToolResult(t, resp)
	if len(result.Content) == 0 {
		t.Error("explore_page error response should return at least one content block")
	}
	if result.Content[0].Type != "text" {
		t.Errorf("content type = %q, want 'text'", result.Content[0].Type)
	}
	if !result.IsError {
		t.Error("explore_page without pilot should be an error")
	}

	assertSnakeCaseFields(t, string(resp.Result))
}

func TestInteract_ExplorePage_SuccessResponseStructure(t *testing.T) {
	t.Parallel()
	h, _, cap := makeToolHandler(t)
	cap.Extension().SetPilotEnabled(true)
	httpReq := httptest.NewRequest("POST", "/sync", strings.NewReader(`{"ext_session_id":"test"}`))
	httpReq.Header.Set("X-Kaboom-Client", "test-client")
	cap.HandleSync(httptest.NewRecorder(), httpReq)
	cap.Extension().SetTrackingStatusForTest(42, "https://example.com")

	resp := callInteractRaw(h, `{"what":"explore_page"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("explore_page with pilot should succeed, got: %s", result.Content[0].Text)
	}
	if len(result.Content) == 0 {
		t.Error("explore_page success response should return at least one content block")
	}

	data := extractResultJSON(t, result)
	if data["status"] != "queued" {
		t.Errorf("status = %v, want 'queued'", data["status"])
	}
	if _, ok := data["correlation_id"].(string); !ok {
		t.Error("success response should contain correlation_id")
	}

	assertSnakeCaseFields(t, string(resp.Result))
}

func TestInteract_ExplorePage_TabIDForwarded(t *testing.T) {
	t.Parallel()
	h, _, cap := makeToolHandler(t)
	cap.Extension().SetPilotEnabled(true)
	httpReq := httptest.NewRequest("POST", "/sync", strings.NewReader(`{"ext_session_id":"test"}`))
	httpReq.Header.Set("X-Kaboom-Client", "test-client")
	cap.HandleSync(httptest.NewRecorder(), httpReq)
	cap.Extension().SetTrackingStatusForTest(42, "https://example.com")

	resp := callInteractRaw(h, `{"what":"explore_page","tab_id":99}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("explore_page with tab_id should succeed, got: %s", result.Content[0].Text)
	}

	pq := cap.Queries().GetLastPendingQuery()
	if pq == nil {
		t.Fatal("expected pending query to be created")
	}
	if pq.TabID != 99 {
		t.Errorf("pending query TabID = %d, want 99", pq.TabID)
	}
}
