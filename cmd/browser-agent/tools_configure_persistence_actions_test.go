// tools_configure_persistence_actions_test.go — Tests noise, store, and load actions.
// Docs: docs/features/feature/config-profiles/index.md

package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

// ============================================
// configure(action:"noise_rule") — Response Fields
// ============================================

func TestToolsConfigureNoiseRule_ListAction(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callConfigureRaw(h, `{"what":"noise_rule","noise_action":"list"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("noise_rule list should succeed, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	if _, ok := data["rules"]; !ok {
		t.Error("response missing 'rules' field")
	}
	if _, ok := data["statistics"]; !ok {
		t.Error("response missing 'statistics' field")
	}

	// Verify statistics fields
	stats, _ := data["statistics"].(map[string]any)
	if stats == nil {
		t.Fatal("statistics should be a map")
	}
	for _, field := range []string{"total_filtered", "per_rule", "last_signal_at", "last_noise_at"} {
		if _, ok := stats[field]; !ok {
			t.Errorf("statistics missing field %q", field)
		}
	}

	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsConfigureNoiseRule_DefaultAction(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	// No noise_action should default to "list"
	resp := callConfigureRaw(h, `{"what":"noise_rule"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("noise_rule default should succeed, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	if _, ok := data["rules"]; !ok {
		t.Error("default action should return rules (list)")
	}

	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsConfigureNoiseRule_ResetAction(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callConfigureRaw(h, `{"what":"noise_rule","noise_action":"reset"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("noise_rule reset should succeed, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	if data["status"] != "ok" {
		t.Errorf("status = %v, want 'ok'", data["status"])
	}
	if _, ok := data["total_rules"]; !ok {
		t.Error("response missing 'total_rules' field")
	}

	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsConfigureNoiseRule_RemoveMissingRuleID(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callConfigureRaw(h, `{"what":"noise_rule","noise_action":"remove"}`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("noise_rule remove without rule_id should return isError:true")
	}
	if !strings.Contains(result.Content[0].Text, "rule_id") {
		t.Error("error should mention 'rule_id' parameter")
	}
	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsConfigureNoiseRule_UnknownSubAction(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callConfigureRaw(h, `{"what":"noise_rule","noise_action":"invalid_action"}`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("noise_rule with unknown sub-action should return isError:true")
	}
	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsConfigureNoiseRule_AutoDetect(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callConfigureRaw(h, `{"what":"noise_rule","noise_action":"auto_detect"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("noise_rule auto_detect should succeed, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	for _, field := range []string{"proposals", "total_rules", "proposals_count", "message"} {
		if _, ok := data[field]; !ok {
			t.Errorf("auto_detect response missing field %q", field)
		}
	}

	assertSnakeCaseFields(t, string(resp.Result))
}

// ============================================
// configure(action:"store") — Response Fields
// ============================================

func TestToolsConfigureStore_ListDefault(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	// store with no sub-action defaults to "list"; namespace is required for list
	resp := callConfigureRaw(h, `{"what":"store","namespace":"test_ns"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("store default should succeed, got: %s", result.Content[0].Text)
	}

	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsConfigureStore_InvalidJSON(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := h.configureSessions.Store(req, json.RawMessage(`{bad}`))
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("store invalid JSON should return isError:true")
	}
	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsConfigureStore_DefaultNamespaceForList(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callConfigureRaw(h, `{"what":"store"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("store list with default namespace should succeed, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	if data["namespace"] != "session" {
		t.Fatalf("namespace = %v, want session", data["namespace"])
	}
	if _, ok := data["keys"]; !ok {
		t.Fatalf("response should contain keys, got: %+v", data)
	}

	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsConfigureStore_CanonicalActionAndData(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	saveResp := callConfigureRaw(h, `{"what":"store","store_action":"save","key":"flat_key","data":"flat_value"}`)
	saveResult := parseToolResult(t, saveResp)
	if saveResult.IsError {
		t.Fatalf("store save via canonical fields should succeed, got: %s", saveResult.Content[0].Text)
	}
	saveData := extractResultJSON(t, saveResult)
	if saveData["status"] != "saved" {
		t.Fatalf("status = %v, want saved", saveData["status"])
	}
	if saveData["namespace"] != "session" {
		t.Fatalf("namespace = %v, want session", saveData["namespace"])
	}

	loadResp := callConfigureRaw(h, `{"what":"store","store_action":"load","key":"flat_key"}`)
	loadResult := parseToolResult(t, loadResp)
	if loadResult.IsError {
		t.Fatalf("store load with default namespace should succeed, got: %s", loadResult.Content[0].Text)
	}
	loadData := extractResultJSON(t, loadResult)
	if loadData["namespace"] != "session" {
		t.Fatalf("namespace = %v, want session", loadData["namespace"])
	}
	if loadData["key"] != "flat_key" {
		t.Fatalf("key = %v, want flat_key", loadData["key"])
	}
	if loadData["data"] != "flat_value" {
		t.Fatalf("data = %v, want flat_value", loadData["data"])
	}
}

// ============================================
// configure(action:"load") — Response Fields
// ============================================

func TestToolsConfigureLoad_ResponseFields(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callConfigureRaw(h, `{"what":"load"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("load should succeed, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	if data["status"] != "ok" {
		t.Errorf("status = %v, want 'ok'", data["status"])
	}

	assertSnakeCaseFields(t, string(resp.Result))
}

// ============================================
// configure(action:"test_boundary_start") — Response Fields
