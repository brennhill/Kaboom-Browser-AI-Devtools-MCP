// Purpose: Tests for observe IndexedDB data retrieval.
// Docs: docs/features/feature/mcp-persistent-server/index.md

// tools_observe_indexeddb_test.go — TDD coverage for IndexedDB observe flows.
package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

func TestObserveIndexedDB_MissingDatabase(t *testing.T) {
	t.Parallel()
	h, _, cap := makeToolHandler(t)
	cap.SetTrackingStatusForTest(11, "https://app.example.com")

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := h.observeDispatcher.Handle(req, json.RawMessage(`{"what":"indexeddb","store":"users"}`))
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("missing database should return error")
	}
	text := firstText(result)
	if !strings.Contains(text, "missing_param") || !strings.Contains(text, "database") {
		t.Fatalf("expected missing_param error for database, got: %s", text)
	}
}

func TestObserveIndexedDB_MissingStore(t *testing.T) {
	t.Parallel()
	h, _, cap := makeToolHandler(t)
	cap.SetTrackingStatusForTest(11, "https://app.example.com")

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := h.observeDispatcher.Handle(req, json.RawMessage(`{"what":"indexeddb","database":"app-cache"}`))
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("missing store should return error")
	}
	text := firstText(result)
	if !strings.Contains(text, "missing_param") || !strings.Contains(text, "store") {
		t.Fatalf("expected missing_param error for store, got: %s", text)
	}
}

func TestObserveIndexedDB_ReturnsEntries(t *testing.T) {
	t.Parallel()
	h, _, cap := makeToolHandler(t)
	cap.SetTrackingStatusForTest(12, "https://app.example.com")

	scriptCh := make(chan string, 1)
	go func() {
		deadline := time.Now().Add(1200 * time.Millisecond)
		for time.Now().Before(deadline) {
			for _, q := range cap.Queries().GetPendingQueries() {
				if q.Type != "execute" {
					continue
				}
				var params map[string]any
				_ = json.Unmarshal(q.Params, &params)
				script, _ := params["script"].(string)
				scriptCh <- script
				cap.Queries().SetQueryResult(q.ID, json.RawMessage(`{"success":true,"result":{"ok":true,"database":"app-cache","store":"users","entries":[{"key":"u1","value":{"id":"u1","name":"Alice"}}],"count":1,"limit":10}}`))
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
		scriptCh <- ""
	}()

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := h.observeDispatcher.Handle(req, json.RawMessage(`{"what":"indexeddb","database":"app-cache","store":"users","limit":10}`))
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("indexeddb should succeed, got: %s", firstText(result))
	}

	data := extractResultJSON(t, result)
	if data["database"] != "app-cache" {
		t.Fatalf("database = %v, want app-cache", data["database"])
	}
	if data["store"] != "users" {
		t.Fatalf("store = %v, want users", data["store"])
	}
	if count, _ := data["count"].(float64); count != 1 {
		t.Fatalf("count = %v, want 1", data["count"])
	}
	entries, _ := data["entries"].([]any)
	if len(entries) != 1 {
		t.Fatalf("entries length = %d, want 1", len(entries))
	}

	script := <-scriptCh
	if script == "" {
		t.Fatal("did not observe execute query for indexeddb")
	}
	if !strings.Contains(script, "indexedDB.open") {
		t.Fatalf("script should query indexedDB, got: %q", script)
	}
	if !strings.Contains(script, "app-cache") || !strings.Contains(script, "users") {
		t.Fatalf("script should include database/store names, got: %q", script)
	}
}

func TestObserveStorage_IncludesIndexedDBListing(t *testing.T) {
	t.Parallel()
	h, _, cap := makeToolHandler(t)
	cap.SetTrackingStatusForTest(13, "https://app.example.com")

	scriptCh := make(chan string, 1)
	go func() {
		deadline := time.Now().Add(1500 * time.Millisecond)
		handledState := false
		handledExec := false
		for time.Now().Before(deadline) {
			for _, q := range cap.Queries().GetPendingQueries() {
				switch q.Type {
				case "state_capture":
					if handledState {
						continue
					}
					cap.Queries().SetQueryResult(q.ID, json.RawMessage(`{"url":"https://app.example.com","localStorage":{"theme":"dark"},"sessionStorage":{"token":"abc"},"cookies":"debug=true"}`))
					handledState = true
				case "execute":
					if handledExec {
						continue
					}
					var params map[string]any
					_ = json.Unmarshal(q.Params, &params)
					script, _ := params["script"].(string)
					scriptCh <- script
					cap.Queries().SetQueryResult(q.ID, json.RawMessage(`{"success":true,"result":{"supported":true,"databases":[{"name":"app-cache","version":3,"object_stores":["users","settings"]}]}}`))
					handledExec = true
				}
			}

			if handledState && handledExec {
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
		scriptCh <- ""
	}()

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := h.observeDispatcher.Handle(req, json.RawMessage(`{"what":"storage"}`))
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("storage should succeed, got: %s", firstText(result))
	}

	data := extractResultJSON(t, result)
	indexeddb, ok := data["indexeddb"].(map[string]any)
	if !ok {
		t.Fatalf("indexeddb listing missing from storage response: %v", data)
	}
	if supported, _ := indexeddb["supported"].(bool); !supported {
		t.Fatalf("indexeddb.supported = %v, want true", indexeddb["supported"])
	}
	databases, _ := indexeddb["databases"].([]any)
	if len(databases) != 1 {
		t.Fatalf("indexeddb.databases length = %d, want 1", len(databases))
	}

	script := <-scriptCh
	if script == "" {
		t.Fatal("did not observe execute query for storage indexeddb listing")
	}
	if !strings.Contains(script, "indexedDB.databases") {
		t.Fatalf("storage indexeddb script should call indexedDB.databases, got: %q", script)
	}
}
