// dispatcher_cleanup_test.go — Tests for TTL cleanup and client-isolated result waits.

package queries

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestCleanExpiredResultsPrunesStaleStateAndExpiresOrphans(t *testing.T) {
	t.Parallel()
	qd := NewQueryDispatcher()
	t.Cleanup(qd.Close)
	now := time.Now()
	grace := AsyncCommandTimeout + 10*time.Second

	qd.mu.Lock()
	qd.queryResults["stale"] = QueryResultEntry{Result: json.RawMessage(`1`), CreatedAt: now.Add(-2 * QueryResultTTL)}
	qd.queryResults["fresh"] = QueryResultEntry{Result: json.RawMessage(`2`), CreatedAt: now}
	qd.pendingQueries = append(qd.pendingQueries,
		PendingQueryEntry{Query: PendingQueryResponse{ID: "orphan", CorrelationID: "corr-orphan"}, Expires: now.Add(-grace - time.Minute)},
		PendingQueryEntry{Query: PendingQueryResponse{ID: "orphan-untracked"}, Expires: now.Add(-grace - time.Minute)},
		PendingQueryEntry{Query: PendingQueryResponse{ID: "live", CorrelationID: "corr-live"}, Expires: now.Add(time.Minute)},
	)
	qd.mu.Unlock()

	qd.resultsMu.Lock()
	qd.activeCommands["corr-orphan"] = &CommandResult{CorrelationID: "corr-orphan", Status: "pending", CreatedAt: now, ExpiresAt: now.Add(time.Hour)}
	qd.activeCommands["corr-ancient"] = &CommandResult{CorrelationID: "corr-ancient", Status: "pending", CreatedAt: now.Add(-2 * QueryResultTTL), ExpiresAt: now.Add(time.Hour)}
	qd.resultsMu.Unlock()

	qd.cleanExpiredResults()

	qd.mu.Lock()
	_, staleKept := qd.queryResults["stale"]
	_, freshKept := qd.queryResults["fresh"]
	pending := make([]string, 0, len(qd.pendingQueries))
	for _, entry := range qd.pendingQueries {
		pending = append(pending, entry.Query.ID)
	}
	qd.mu.Unlock()
	if staleKept || !freshKept {
		t.Fatalf("results after cleanup: stale kept=%v fresh kept=%v", staleKept, freshKept)
	}
	if strings.Join(pending, ",") != "live" {
		t.Fatalf("pending after cleanup = %v, want [live]", pending)
	}

	qd.resultsMu.RLock()
	orphan := qd.activeCommands["corr-orphan"]
	_, ancientKept := qd.activeCommands["corr-ancient"]
	qd.resultsMu.RUnlock()
	if ancientKept {
		t.Fatal("command older than the result TTL was not pruned")
	}
	if orphan != nil && orphan.Status != "expired" {
		t.Fatalf("orphaned command status = %q, want expired", orphan.Status)
	}
	if result, ok := qd.GetCommandResult("corr-orphan"); !ok || result.Status != "expired" {
		t.Fatalf("orphaned command = %#v, %v; want expired outcome preserved", result, ok)
	}
}

func TestWaitForResultWithClientHonorsIsolation(t *testing.T) {
	t.Parallel()
	qd := NewQueryDispatcher()
	t.Cleanup(qd.Close)

	qd.SetQueryResultWithClientNoCommandComplete("q-owned", json.RawMessage(`{"ok":true}`), "client-a")
	if _, err := qd.WaitForResultWithClient("q-owned", 10*time.Millisecond, "client-b"); err == nil || !strings.Contains(err.Error(), "timeout") {
		t.Fatalf("foreign client wait err = %v, want timeout", err)
	}
	got, err := qd.WaitForResultWithClient("q-owned", time.Second, "client-a")
	if err != nil || string(got) != `{"ok":true}` {
		t.Fatalf("owner wait = %s, %v", got, err)
	}
	if _, err := qd.WaitForResultWithClient("q-owned", 10*time.Millisecond, ""); err == nil {
		t.Fatal("result was replayed after being consumed")
	}
}
