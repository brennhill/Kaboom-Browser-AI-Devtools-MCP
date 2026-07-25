// Purpose: Tests for capture query command dispatch and response routing.
// Docs: docs/features/feature/backend-log-streaming/index.md

// query_commands_test.go — Tests for Capture delegation of command methods and disconnect detection.
package capture

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
)

// ============================================
// Capture Delegation Tests (query_dispatcher.go)
// ============================================

func TestNewCaptureDelegation_QueryDispatcher(t *testing.T) {
	t.Parallel()

	c := NewCapture()
	t.Cleanup(c.Close)

	id, _ := c.CreatePendingQuery(queries.PendingQuery{Type: "dom", Params: json.RawMessage(`{}`)})
	if id == "" {
		t.Fatal("CreatePendingQuery returned empty id")
	}

	pending := c.GetPendingQueries()
	if len(pending) != 1 {
		t.Fatalf("GetPendingQueries len = %d, want 1", len(pending))
	}

	c.SetQueryResult(id, json.RawMessage(`{"ok":true}`))
	result, found := c.TakeQueryResult(id)
	if !found {
		t.Fatal("TakeQueryResult returned false")
	}
	if string(result) != `{"ok":true}` {
		t.Errorf("result = %s, want {\"ok\":true}", string(result))
	}

	c.SetQueryTimeout(5 * time.Second)
	if got := c.GetQueryTimeout(); got != 5*time.Second {
		t.Errorf("GetQueryTimeout = %v, want 5s", got)
	}

	c.RegisterCommand("c-1", "q-1", 30*time.Second)
	c.CompleteCommand("c-1", json.RawMessage(`{"done":true}`), "")
	cmd, cmdFound := c.GetCommandResult("c-1")
	if !cmdFound {
		t.Fatal("GetCommandResult returned false")
	}
	if cmd.Status != "complete" {
		t.Errorf("cmd.Status = %q, want complete", cmd.Status)
	}

	c.RegisterCommand("c-2", "q-2", 30*time.Second)
	c.ExpireCommand("c-2")

	_ = c.GetPendingCommands()
	_ = c.GetCompletedCommands()
	failed := c.GetFailedCommands()
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

	c.CreatePendingQuery(queries.PendingQuery{Type: "dom", Params: json.RawMessage(`{}`)})

	pending := c.GetPendingQueriesDisconnectAware()
	if len(pending) != 1 {
		t.Fatalf("pending len = %d, want 1 (never synced = not disconnected)", len(pending))
	}
}

func TestNewCapture_GetPendingQueriesDisconnectAware_RecentSync(t *testing.T) {
	t.Parallel()

	c := NewCapture()
	t.Cleanup(c.Close)

	c.mu.Lock()
	c.extensionState.lastSyncSeen = time.Now()
	c.mu.Unlock()

	c.CreatePendingQuery(queries.PendingQuery{Type: "dom", Params: json.RawMessage(`{}`)})

	pending := c.GetPendingQueriesDisconnectAware()
	if len(pending) != 1 {
		t.Fatalf("pending len = %d, want 1 (recently synced)", len(pending))
	}
}

func TestNewCapture_GetPendingQueriesDisconnectAware_Disconnected(t *testing.T) {
	t.Parallel()

	c := NewCapture()
	t.Cleanup(c.Close)

	c.mu.Lock()
	c.extensionState.lastSyncSeen = time.Now().Add(-20 * time.Second)
	c.mu.Unlock()

	c.CreatePendingQuery(queries.PendingQuery{
		Type:          "dom",
		Params:        json.RawMessage(`{}`),
		CorrelationID: "corr-disc",
	})

	pending := c.GetPendingQueriesDisconnectAware()
	if len(pending) != 0 {
		t.Fatalf("pending len = %d, want 0 (disconnected)", len(pending))
	}
}
