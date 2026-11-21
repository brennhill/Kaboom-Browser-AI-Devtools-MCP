// Purpose: Tests for capture query command dispatch and response routing.
// Docs: docs/features/feature/backend-log-streaming/index.md

// query_commands_test.go — Tests for canonical query dispatch and Capture disconnect orchestration.
package capture

import (
	"encoding/json"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/browsercommand/commandtest"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/syncruntime"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
)

// ============================================
// Canonical Query Dispatcher Integration Tests
// ============================================

func TestNewCaptureDelegation_QueryDispatcher(t *testing.T) {
	t.Parallel()

	c := NewCapture()
	t.Cleanup(c.Close)

	id, _ := commandtest.Enqueue(c.Commands(), queries.PendingQuery{Type: "dom", Params: json.RawMessage(`{}`)})
	if id == "" {
		t.Fatal("CreatePendingQuery returned empty id")
	}

	pending := c.Commands().Pending()
	if len(pending) != 1 {
		t.Fatalf("GetPendingQueries len = %d, want 1", len(pending))
	}

	commandtest.Deliver(c.Commands(), id, json.RawMessage(`{"ok":true}`))
	result, found := commandtest.Take(c.Commands(), id)
	if !found {
		t.Fatal("TakeQueryResult returned false")
	}
	if string(result) != `{"ok":true}` {
		t.Errorf("result = %s, want {\"ok\":true}", string(result))
	}

	commandtest.Register(c.Commands(), "c-1", 30*time.Second)
	commandtest.Complete(c.Commands(), "c-1", "complete", json.RawMessage(`{"done":true}`), "")
	cmd, cmdFound := c.Commands().Command("c-1")
	if !cmdFound {
		t.Fatal("GetCommandResult returned false")
	}
	if cmd.Status != "complete" {
		t.Errorf("cmd.Status = %q, want complete", cmd.Status)
	}

	commandtest.Register(c.Commands(), "c-2", 30*time.Second)
	commandtest.Expire(c.Commands(), "c-2")

	_ = c.Commands().PendingCommands()
	_ = c.Commands().CompletedCommands()
	failed := c.Commands().FailedCommands()
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

	commandtest.Enqueue(c.Commands(), queries.PendingQuery{Type: "dom", Params: json.RawMessage(`{}`)})

	pending := newSyncHandlerForTest(c).GetPendingQueriesDisconnectAware()
	if len(pending) != 1 {
		t.Fatalf("pending len = %d, want 1 (never synced = not disconnected)", len(pending))
	}
}

func TestNewCapture_GetPendingQueriesDisconnectAware_RecentSync(t *testing.T) {
	t.Parallel()

	c := NewCapture()
	t.Cleanup(c.Close)

	runSyncRequest(t, c, syncruntime.SyncRequest{ExtSessionID: "recent-sync"})

	commandtest.Enqueue(c.Commands(), queries.PendingQuery{Type: "dom", Params: json.RawMessage(`{}`)})

	pending := newSyncHandlerForTest(c).GetPendingQueriesDisconnectAware()
	if len(pending) != 1 {
		t.Fatalf("pending len = %d, want 1 (recently synced)", len(pending))
	}
}

func TestNewCapture_GetPendingQueriesDisconnectAware_Disconnected(t *testing.T) {
	t.Parallel()

	c := NewCapture()
	t.Cleanup(c.Close)

	runSyncRequest(t, c, syncruntime.SyncRequest{ExtSessionID: "disconnect-sync"})
	c.Extension().MarkDisconnected()

	commandtest.Enqueue(c.Commands(), queries.PendingQuery{
		Type:          "dom",
		Params:        json.RawMessage(`{}`),
		CorrelationID: "corr-disc",
	})

	pending := newSyncHandlerForTest(c).GetPendingQueriesDisconnectAware()
	if len(pending) != 0 {
		t.Fatalf("pending len = %d, want 0 (disconnected)", len(pending))
	}
}
