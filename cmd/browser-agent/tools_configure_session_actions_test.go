// tools_configure_session_actions_test.go — Tests boundary, audit, and session actions.
// Docs: docs/features/feature/mcp-persistent-server/index.md

package main

import (
	"strings"
	"testing"
)

// ============================================

func TestToolsConfigureTestBoundaryStart_ResponseFields(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callConfigureRaw(h, `{"what":"test_boundary_start","test_id":"test-123","label":"My Test"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("test_boundary_start should succeed, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	for _, field := range []string{"status", "test_id", "label", "message"} {
		if _, ok := data[field]; !ok {
			t.Errorf("test_boundary_start response missing field %q", field)
		}
	}
	if data["status"] != "ok" {
		t.Errorf("status = %v, want 'ok'", data["status"])
	}
	if data["test_id"] != "test-123" {
		t.Errorf("test_id = %v, want 'test-123'", data["test_id"])
	}
	if data["label"] != "My Test" {
		t.Errorf("label = %v, want 'My Test'", data["label"])
	}

	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsConfigureTestBoundaryStart_MissingTestID(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callConfigureRaw(h, `{"what":"test_boundary_start"}`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("test_boundary_start without test_id should return isError:true")
	}
	if !strings.Contains(result.Content[0].Text, "test_id") {
		t.Error("error should mention 'test_id' parameter")
	}
	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsConfigureTestBoundaryStart_DefaultLabel(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callConfigureRaw(h, `{"what":"test_boundary_start","test_id":"abc"}`)
	result := parseToolResult(t, resp)
	data := extractResultJSON(t, result)

	label, _ := data["label"].(string)
	if !strings.Contains(label, "abc") {
		t.Errorf("default label should contain test_id, got: %q", label)
	}
}

// ============================================
// configure(action:"test_boundary_end") — Response Fields
// ============================================

func TestToolsConfigureTestBoundaryEnd_ResponseFields(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	// Must start a boundary first so end succeeds.
	callConfigureRaw(h, `{"what":"test_boundary_start","test_id":"test-123"}`)

	resp := callConfigureRaw(h, `{"what":"test_boundary_end","test_id":"test-123"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("test_boundary_end should succeed, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	for _, field := range []string{"status", "test_id", "was_active", "message"} {
		if _, ok := data[field]; !ok {
			t.Errorf("test_boundary_end response missing field %q", field)
		}
	}
	if data["status"] != "ok" {
		t.Errorf("status = %v, want 'ok'", data["status"])
	}
	if data["test_id"] != "test-123" {
		t.Errorf("test_id = %v, want 'test-123'", data["test_id"])
	}

	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsConfigureTestBoundaryEnd_MissingTestID(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callConfigureRaw(h, `{"what":"test_boundary_end"}`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("test_boundary_end without test_id should return isError:true")
	}
	if !strings.Contains(result.Content[0].Text, "test_id") {
		t.Error("error should mention 'test_id' parameter")
	}
	assertSnakeCaseFields(t, string(resp.Result))
}

// ============================================
// configure(action:"audit_log") — Response Fields
// ============================================

func TestToolsConfigureAuditLog_ResponseFields(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callConfigureRaw(h, `{"what":"audit_log"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("audit_log should succeed, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	if data["status"] != "ok" {
		t.Errorf("status = %v, want 'ok'", data["status"])
	}
	if _, ok := data["entries"]; !ok {
		t.Error("response missing 'entries' field")
	}

	assertSnakeCaseFields(t, string(resp.Result))
}

// ============================================
// configure(action:"diff_sessions") — Response Fields
// ============================================

func TestToolsConfigureDiffSessions_ResponseFields(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callConfigureRaw(h, `{"what":"diff_sessions"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("diff_sessions should succeed, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	if data["status"] != "ok" {
		t.Errorf("status = %v, want 'ok'", data["status"])
	}

	assertSnakeCaseFields(t, string(resp.Result))
}

// ============================================
// All configure actions safety net
// ============================================
