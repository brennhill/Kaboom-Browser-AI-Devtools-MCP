// draw_sessions_handler_test.go — Tests persisted draw-session loading.

package annotation

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

func TestLoadDrawSessionRejectsTraversal(t *testing.T) {
	t.Parallel()
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	response := LoadDrawSession(NewStore(time.Minute), req, json.RawMessage(`{"file":"../secret.json"}`), t.TempDir(), nil)
	var result mcp.MCPToolResult
	if err := json.Unmarshal(response.Result, &result); err != nil {
		t.Fatal(err)
	}
	if !result.IsError {
		t.Fatal("expected path traversal to fail")
	}
}

func TestDrawSessionHistoryFiltersSortsAndReportsDirectoryFailures(t *testing.T) {
	t.Parallel()
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	if result := decodeDrawResult(t, ListDrawHistory(req, "", errors.New("unavailable"))); !result.IsError {
		t.Fatal("directory resolution error was ignored")
	}
	if result := decodeDrawResult(t, ListDrawHistory(req, filepath.Join(t.TempDir(), "missing"), nil)); !result.IsError {
		t.Fatal("missing directory error was ignored")
	}
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "draw-session-dir.json"), 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"ignored.json", "draw-session-old.json", "draw-session-new.json"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(`{}`), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	old := time.Now().Add(-time.Hour)
	_ = os.Chtimes(filepath.Join(dir, "draw-session-old.json"), old, old)
	result := decodeDrawResult(t, ListDrawHistory(req, dir, nil))
	if result.IsError || !strings.Contains(result.Content[0].Text, `"count":2`) ||
		strings.Index(result.Content[0].Text, "draw-session-new") > strings.Index(result.Content[0].Text, "draw-session-old") {
		t.Fatalf("history = %+v", result)
	}
}

func TestLoadDrawSessionHydratesCanonicalStores(t *testing.T) {
	t.Parallel()
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	dir := t.TempDir()
	store := NewStore(time.Minute)
	t.Cleanup(store.Close)
	for _, args := range []json.RawMessage{nil, json.RawMessage(`not-json`), json.RawMessage(`{"file":"missing.json"}`)} {
		if result := decodeDrawResult(t, LoadDrawSession(store, req, args, dir, nil)); !result.IsError {
			t.Fatalf("LoadDrawSession(%s) accepted invalid input", args)
		}
	}
	if result := decodeDrawResult(t, LoadDrawSession(store, req, json.RawMessage(`{"file":"session.json"}`), dir, errors.New("blocked"))); !result.IsError {
		t.Fatal("directory error was ignored")
	}
	if err := os.WriteFile(filepath.Join(dir, "broken.json"), []byte(`not-json`), 0o600); err != nil {
		t.Fatal(err)
	}
	if result := decodeDrawResult(t, LoadDrawSession(store, req, json.RawMessage(`{"file":"broken.json"}`), dir, nil)); !result.IsError {
		t.Fatal("corrupt session was accepted")
	}
	payload := `{"tab_id":7,"page_url":"https://example.test","timestamp":1234,"annot_session_name":"review","annotations":[],"element_details":{"corr":{"tag_name":"button"}}}`
	if err := os.WriteFile(filepath.Join(dir, "draw-session-good.json"), []byte(payload), 0o600); err != nil {
		t.Fatal(err)
	}
	result := decodeDrawResult(t, LoadDrawSession(store, req, json.RawMessage(`{"file":"draw-session-good.json"}`), dir, nil))
	if result.IsError || !strings.Contains(result.Content[0].Text, `"annot_session":"review"`) {
		t.Fatalf("loaded session = %+v", result)
	}
	if session := store.GetSession(7); session == nil || session.Timestamp == 0 {
		t.Fatalf("hydrated tab session = %#v", session)
	}
	if _, ok := store.GetDetail("corr"); !ok {
		t.Fatal("element detail was not hydrated")
	}
	// Loading the same persisted page must not duplicate it in the named session.
	_ = LoadDrawSession(store, req, json.RawMessage(`{"file":"draw-session-good.json"}`), dir, nil)
	if named := store.GetNamedSession("review"); named == nil || len(named.Pages) != 1 {
		t.Fatalf("named session = %#v", named)
	}
}

func decodeDrawResult(t *testing.T, response mcp.JSONRPCResponse) mcp.MCPToolResult {
	t.Helper()
	var result mcp.MCPToolResult
	if err := json.Unmarshal(response.Result, &result); err != nil {
		t.Fatal(err)
	}
	return result
}
