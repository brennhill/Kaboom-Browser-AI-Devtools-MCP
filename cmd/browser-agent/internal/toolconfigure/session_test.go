// session_test.go — Tests configure session persistence entry points.

package toolconfigure

import (
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/persistence"
)

func TestSessionHandlerLoadReportsPersistedContext(t *testing.T) {
	t.Parallel()
	store, err := persistence.NewSessionStore(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(store.Shutdown)
	handler := NewSessionHandler(SessionDeps{}, store, nil)
	result := parseRespJSON(t, handler.Load(newReq(), nil))
	if result["status"] != "ok" || result["project_id"] == nil || result["session_count"] == nil {
		t.Fatalf("loaded session context = %#v", result)
	}
}

func TestSessionHandlerLoadFailsClearlyWithoutStore(t *testing.T) {
	t.Parallel()
	handler := NewSessionHandler(SessionDeps{}, nil, nil)
	isError, text := parseResp(t, handler.Load(mcp.JSONRPCRequest{JSONRPC: mcp.JSONRPCVersion, ID: 1}, nil))
	if !isError || (!strings.Contains(text, mcp.ErrNotInitialized) && !strings.Contains(text, "not initialized")) {
		t.Fatalf("nil-store response = error:%t text:%q", isError, text)
	}
}
