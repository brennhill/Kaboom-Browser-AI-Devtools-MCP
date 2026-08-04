// Purpose: Tests for capture query command dispatch and response routing.
// Docs: docs/features/feature/backend-log-streaming/index.md

// query_commands_test.go — Tests for canonical query dispatch and Capture disconnect orchestration.
package capture

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
)

// ============================================
// Canonical Query Dispatcher Integration Tests
// ============================================

func TestNewCaptureDelegation_QueryDispatcher(t *testing.T) {
	t.Parallel()

	c := NewCapture()
	t.Cleanup(c.Close)

	id, _ := c.Queries().CreatePendingQuery(queries.PendingQuery{Type: "dom", Params: json.RawMessage(`{}`)})
	if id == "" {
		t.Fatal("CreatePendingQuery returned empty id")
	}

	pending := c.Queries().GetPendingQueries()
	if len(pending) != 1 {
		t.Fatalf("GetPendingQueries len = %d, want 1", len(pending))
	}

	c.Queries().SetQueryResult(id, json.RawMessage(`{"ok":true}`))
	result, found := c.Queries().TakeQueryResult(id)
	if !found {
		t.Fatal("TakeQueryResult returned false")
	}
	if string(result) != `{"ok":true}` {
		t.Errorf("result = %s, want {\"ok\":true}", string(result))
	}

	c.Queries().SetQueryTimeout(5 * time.Second)
	if got := c.Queries().GetQueryTimeout(); got != 5*time.Second {
		t.Errorf("GetQueryTimeout = %v, want 5s", got)
	}

	c.Queries().RegisterCommand("c-1", "q-1", 30*time.Second)
	c.Queries().ApplyCommandResult("c-1", "complete", json.RawMessage(`{"done":true}`), "")
	cmd, cmdFound := c.Queries().GetCommandResult("c-1")
	if !cmdFound {
		t.Fatal("GetCommandResult returned false")
	}
	if cmd.Status != "complete" {
		t.Errorf("cmd.Status = %q, want complete", cmd.Status)
	}

	c.Queries().RegisterCommand("c-2", "q-2", 30*time.Second)
	c.Queries().ExpireCommand("c-2")

	_ = c.Queries().GetPendingCommands()
	_ = c.Queries().GetCompletedCommands()
	failed := c.Queries().GetFailedCommands()
	if len(failed) == 0 {
		t.Error("GetFailedCommands should contain expired command")
	}
}

// ============================================
// GetPendingQueriesDisconnectAware Tests
// ============================================

func TestNewCapture_GetPendingQueriesDisconnectAware_NeverSynced(t *testing.T) {
	t.Parallel()

	c := NewCapture()
	t.Cleanup(c.Close)

	c.Queries().CreatePendingQuery(queries.PendingQuery{Type: "dom", Params: json.RawMessage(`{}`)})

	pending := NewSyncHandler(c).GetPendingQueriesDisconnectAware()
	if len(pending) != 1 {
		t.Fatalf("pending len = %d, want 1 (never synced = not disconnected)", len(pending))
	}
}

func TestNewCapture_GetPendingQueriesDisconnectAware_RecentSync(t *testing.T) {
	t.Parallel()

	c := NewCapture()
	t.Cleanup(c.Close)

	mutateExtensionStateForTest(c.Extension(), func(state *ExtensionState) {
		state.lastSyncSeen = time.Now()
		state.lastExtensionConnected = true
	})

	c.Queries().CreatePendingQuery(queries.PendingQuery{Type: "dom", Params: json.RawMessage(`{}`)})

	pending := NewSyncHandler(c).GetPendingQueriesDisconnectAware()
	if len(pending) != 1 {
		t.Fatalf("pending len = %d, want 1 (recently synced)", len(pending))
	}
}

func TestNewCapture_GetPendingQueriesDisconnectAware_Disconnected(t *testing.T) {
	t.Parallel()

	c := NewCapture()
	t.Cleanup(c.Close)

	mutateExtensionStateForTest(c.Extension(), func(state *ExtensionState) {
		state.lastSyncSeen = time.Now().Add(-20 * time.Second)
		state.lastExtensionConnected = true
	})

	c.Queries().CreatePendingQuery(queries.PendingQuery{
		Type:          "dom",
		Params:        json.RawMessage(`{}`),
		CorrelationID: "corr-disc",
	})

	pending := NewSyncHandler(c).GetPendingQueriesDisconnectAware()
	if len(pending) != 0 {
		t.Fatalf("pending len = %d, want 0 (disconnected)", len(pending))
	}
}
