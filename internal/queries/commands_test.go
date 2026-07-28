// Purpose: Tests for query command construction and validation.
// Docs: docs/features/feature/query-service/index.md

// commands_test.go — Tests for command lifecycle, expiration, and status normalization.
package queries

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// ============================================
// Command Lifecycle Tests
// ============================================

func TestNewQueryDispatcher_RegisterCommand(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	defer qd.Close()

	qd.RegisterCommand("corr-1", "q-1", 30*time.Second)

	cmd, found := qd.GetCommandResult("corr-1")
	if !found {
		t.Fatal("GetCommandResult returned false for registered command")
	}
	if cmd.CorrelationID != "corr-1" {
		t.Errorf("CorrelationID = %q, want corr-1", cmd.CorrelationID)
	}
	if cmd.Status != "pending" {
		t.Errorf("Status = %q, want pending", cmd.Status)
	}
	if cmd.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}
	if cmd.ExpiresAt.IsZero() {
		t.Error("ExpiresAt should be set")
	}
	if !cmd.ExpiresAt.After(cmd.CreatedAt) {
		t.Errorf("ExpiresAt (%v) should be after CreatedAt (%v)", cmd.ExpiresAt, cmd.CreatedAt)
	}
}

func TestNewQueryDispatcher_RegisterCommand_EmptyCorrelationID(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	defer qd.Close()

	qd.RegisterCommand("", "q-1", 30*time.Second)

	_, found := qd.GetCommandResult("")
	if found {
		t.Error("empty correlation ID should not register a command")
	}
}

func TestNewQueryDispatcher_ApplyCommandResultComplete(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	defer qd.Close()

	qd.RegisterCommand("corr-2", "q-2", 30*time.Second)

	resultData := json.RawMessage(`{"title":"Page Title"}`)
	qd.ApplyCommandResult("corr-2", "complete", resultData, "")

	cmd, found := qd.GetCommandResult("corr-2")
	if !found {
		t.Fatal("GetCommandResult returned false for completed command")
	}
	if cmd.Status != "complete" {
		t.Errorf("Status = %q, want complete", cmd.Status)
	}
	if string(cmd.Result) != `{"title":"Page Title"}` {
		t.Errorf("Result = %s, want {\"title\":\"Page Title\"}", string(cmd.Result))
	}
	if cmd.Error != "" {
		t.Errorf("Error = %q, want empty", cmd.Error)
	}
	if cmd.CompletedAt.IsZero() {
		t.Error("CompletedAt should be set")
	}
}

func TestNewQueryDispatcher_ApplyCommandResult_WithError(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	defer qd.Close()

	qd.RegisterCommand("corr-err", "q-err", 30*time.Second)
	qd.ApplyCommandResult("corr-err", "complete", nil, "element not found")

	cmd, found := qd.GetCommandResult("corr-err")
	if !found {
		t.Fatal("GetCommandResult returned false")
	}
	if cmd.Status != "error" {
		t.Errorf("Status = %q, want error", cmd.Status)
	}
	if cmd.Error != "element not found" {
		t.Errorf("Error = %q, want 'element not found'", cmd.Error)
	}
}

func TestNewQueryDispatcher_ApplyCommandResult_ErrorStatus(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	defer qd.Close()

	qd.RegisterCommand("corr-err-status", "q-err-status", 30*time.Second)
	qd.ApplyCommandResult("corr-err-status", "error", nil, "execution failed")

	completed := qd.GetCompletedCommands()
	for _, cmd := range completed {
		if cmd.CorrelationID == "corr-err-status" {
			t.Fatal("error command should not remain in completedResults")
		}
	}

	failed := qd.GetFailedCommands()
	if len(failed) == 0 {
		t.Fatal("expected failed command entry for error status")
	}
	var foundErr bool
	for _, cmd := range failed {
		if cmd.CorrelationID != "corr-err-status" {
			continue
		}
		foundErr = true
		if cmd.Status != "error" {
			t.Errorf("Status = %q, want error", cmd.Status)
		}
		if cmd.Error != "execution failed" {
			t.Errorf("Error = %q, want execution failed", cmd.Error)
		}
	}
	if !foundErr {
		t.Fatal("expected failed command with correlation_id corr-err-status")
	}
}

func TestNewQueryDispatcher_ApplyCommandResult_TimeoutStatus(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	defer qd.Close()

	qd.RegisterCommand("corr-timeout-status", "q-timeout-status", 30*time.Second)
	qd.ApplyCommandResult("corr-timeout-status", "timeout", nil, "action timed out")

	cmd, found := qd.GetCommandResult("corr-timeout-status")
	if !found {
		t.Fatal("GetCommandResult returned false for timeout status")
	}
	if cmd.Status != "timeout" {
		t.Errorf("Status = %q, want timeout", cmd.Status)
	}
}

func TestNewQueryDispatcher_ApplyCommandResult_UnknownStatusDefaultsError(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	defer qd.Close()

	qd.RegisterCommand("corr-unknown-status", "q-unknown-status", 30*time.Second)
	qd.ApplyCommandResult("corr-unknown-status", "mystery_status", json.RawMessage(`{"ok":true}`), "")

	// Unknown status should be treated as error to surface protocol drift
	cmd, found := qd.GetCommandResult("corr-unknown-status")
	if !found {
		t.Fatal("GetCommandResult returned false")
	}
	if cmd.Status != "error" {
		t.Errorf("Status = %q, want error for unknown normalized status", cmd.Status)
	}

	// Should appear in failed commands
	failed := qd.GetFailedCommands()
	foundInFailed := false
	for _, f := range failed {
		if f.CorrelationID == "corr-unknown-status" {
			foundInFailed = true
		}
	}
	if !foundInFailed {
		t.Error("unknown status command should appear in GetFailedCommands()")
	}
}

func TestNewQueryDispatcher_ApplyCommandResult_UnknownStatusWithErrorField(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	defer qd.Close()

	qd.RegisterCommand("corr-unknown-err", "q-unknown-err", 30*time.Second)
	qd.ApplyCommandResult("corr-unknown-err", "mystery_status", nil, "something went wrong")

	cmd, found := qd.GetCommandResult("corr-unknown-err")
	if !found {
		t.Fatal("GetCommandResult returned false")
	}
	if cmd.Status != "error" {
		t.Errorf("Status = %q, want error", cmd.Status)
	}
	if cmd.Error != "something went wrong" {
		t.Errorf("Error = %q, want 'something went wrong'", cmd.Error)
	}
}

func TestNewQueryDispatcher_ApplyCommandResult_ErrorFieldForcesErrorStatus(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		status string
	}{
		{name: "empty status", status: ""},
		{name: "complete status", status: "complete"},
		{name: "ok status", status: "ok"},
		{name: "pending status", status: "pending"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			qd := NewQueryDispatcher()
			defer qd.Close()

			corrID := "corr-error-field-" + strings.ReplaceAll(tc.name, " ", "-")
			qd.RegisterCommand(corrID, "q-"+corrID, 30*time.Second)
			qd.ApplyCommandResult(corrID, tc.status, json.RawMessage(`{"ok":false}`), "forced failure")

			cmd, found := qd.GetCommandResult(corrID)
			if !found {
				t.Fatal("GetCommandResult returned false")
			}
			if cmd.Status != "error" {
				t.Errorf("Status = %q, want error (status=%q, error present)", cmd.Status, tc.status)
			}
			if cmd.Error != "forced failure" {
				t.Errorf("Error = %q, want forced failure", cmd.Error)
			}
		})
	}
}

func TestNewQueryDispatcher_ApplyCommandResult_EmptyCorrelationID(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	defer qd.Close()

	qd.ApplyCommandResult("", "complete", json.RawMessage(`{}`), "")
}

func TestNewQueryDispatcher_ApplyCommandResult_NotRegistered(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	defer qd.Close()

	qd.ApplyCommandResult("nonexistent", "complete", json.RawMessage(`{}`), "")
}

func TestNewQueryDispatcher_ExpireCommand(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	defer qd.Close()

	qd.RegisterCommand("corr-exp", "q-exp", 30*time.Second)
	qd.ExpireCommand("corr-exp")

	failed := qd.GetFailedCommands()
	found := false
	for _, cmd := range failed {
		if cmd.CorrelationID == "corr-exp" {
			found = true
			if cmd.Status != "expired" {
				t.Errorf("Status = %q, want expired", cmd.Status)
			}
			if !strings.Contains(cmd.Error, "expired") {
				t.Errorf("Error = %q, want 'expired' message", cmd.Error)
			}
		}
	}
	if !found {
		t.Error("expired command not found in GetFailedCommands()")
	}
}

func TestNewQueryDispatcher_ExpireCommand_EmptyID(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	defer qd.Close()

	qd.ExpireCommand("")
}

func TestNewQueryDispatcher_ExpireCommand_AlreadyCompleted(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	defer qd.Close()

	qd.RegisterCommand("corr-done", "q-done", 30*time.Second)
	qd.ApplyCommandResult("corr-done", "complete", json.RawMessage(`{"ok":true}`), "")

	qd.ExpireCommand("corr-done")

	cmd, found := qd.GetCommandResult("corr-done")
	if !found {
		t.Fatal("completed command should still be findable")
	}
	if cmd.Status != "complete" {
		t.Errorf("Status = %q, want complete (should not be overwritten to expired)", cmd.Status)
	}
}

func TestNewQueryDispatcher_ExpireCommandResultAfterDispatchTimeout(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	defer qd.Close()

	// Create async query with short timeout, then acknowledge delivery so it is
	// no longer in pendingQueries (simulates extension dequeued but never replied).
	queryID, _ := qd.CreatePendingQueryWithTimeout(PendingQuery{
		Type:          "browser_action",
		CorrelationID: "corr-dispatched-timeout",
	}, 30*time.Millisecond, "test-client")
	if queryID == "" {
		t.Fatal("CreatePendingQueryWithTimeout returned empty query ID")
	}

	qd.AcknowledgePendingQuery(queryID)
	time.Sleep(50 * time.Millisecond)

	cmd, found := qd.GetCommandResult("corr-dispatched-timeout")
	if !found {
		t.Fatal("expected expired command result to be present")
	}
	if cmd.Status != "expired" {
		t.Fatalf("status = %q, want expired", cmd.Status)
	}
	if !strings.Contains(cmd.Error, "waiting for extension result") {
		t.Fatalf("error = %q, want waiting-for-extension-result message", cmd.Error)
	}
}

// ============================================
// WaitForCommand Tests
// ============================================

func TestNewQueryDispatcher_WaitForCommand_Immediate(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	defer qd.Close()

	qd.RegisterCommand("corr-wait", "q-wait", 30*time.Second)
	qd.ApplyCommandResult("corr-wait", "complete", json.RawMessage(`{"done":true}`), "")

	cmd, found := qd.WaitForCommand("corr-wait", 1*time.Second)
	if !found {
		t.Fatal("WaitForCommand returned false")
	}
	if cmd.Status != "complete" {
		t.Errorf("Status = %q, want complete", cmd.Status)
	}
}

func TestNewQueryDispatcher_WaitForCommand_Async(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	defer qd.Close()

	qd.RegisterCommand("corr-async", "q-async", 30*time.Second)

	go func() {
		time.Sleep(20 * time.Millisecond)
		qd.ApplyCommandResult("corr-async", "complete", json.RawMessage(`{"async":true}`), "")
	}()

	cmd, found := qd.WaitForCommand("corr-async", 2*time.Second)
	if !found {
		t.Fatal("WaitForCommand returned false")
	}
	if cmd.Status != "complete" {
		t.Errorf("Status = %q, want complete", cmd.Status)
	}
}

func TestNewQueryDispatcher_WaitForCommand_NotFound(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	defer qd.Close()

	cmd, found := qd.WaitForCommand("nonexistent", 50*time.Millisecond)
	if found {
		t.Errorf("WaitForCommand found = true, want false for nonexistent; cmd = %+v", cmd)
	}
}

// ============================================
// GetPendingCommands / GetCompletedCommands / GetFailedCommands Tests
// ============================================

func TestNewQueryDispatcher_GetPendingCommands(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	defer qd.Close()

	qd.RegisterCommand("corr-p1", "q-p1", 30*time.Second)
	qd.RegisterCommand("corr-p2", "q-p2", 30*time.Second)

	pending := qd.GetPendingCommands()
	if len(pending) != 2 {
		t.Fatalf("GetPendingCommands len = %d, want 2", len(pending))
	}
}

func TestNewQueryDispatcher_GetCompletedCommands(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	defer qd.Close()

	qd.RegisterCommand("corr-c1", "q-c1", 30*time.Second)
	qd.ApplyCommandResult("corr-c1", "complete", json.RawMessage(`{}`), "")

	qd.RegisterCommand("corr-c2", "q-c2", 30*time.Second)

	completed := qd.GetCompletedCommands()
	if len(completed) != 1 {
		t.Fatalf("GetCompletedCommands len = %d, want 1", len(completed))
	}
	if completed[0].CorrelationID != "corr-c1" {
		t.Errorf("completed[0].CorrelationID = %q, want corr-c1", completed[0].CorrelationID)
	}
}

func TestNewQueryDispatcher_GetFailedCommands_Empty(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	defer qd.Close()

	failed := qd.GetFailedCommands()
	if len(failed) != 0 {
		t.Errorf("GetFailedCommands len = %d, want 0", len(failed))
	}
}

// ============================================
// ExpireAllPendingQueries Tests
// ============================================

func TestNewQueryDispatcher_ExpireAllPendingQueries(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	defer qd.Close()

	qd.CreatePendingQuery(PendingQuery{
		Type:          "dom",
		Params:        json.RawMessage(`{}`),
		CorrelationID: "corr-exp-1",
	})
	qd.CreatePendingQuery(PendingQuery{
		Type:          "a11y",
		Params:        json.RawMessage(`{}`),
		CorrelationID: "corr-exp-2",
	})

	qd.ExpireAllPendingQueries("extension_disconnected")

	pending := qd.GetPendingQueries()
	if len(pending) != 0 {
		t.Fatalf("pending after ExpireAll = %d, want 0", len(pending))
	}

	failed := qd.GetFailedCommands()
	correlationIDs := make(map[string]bool)
	for _, cmd := range failed {
		correlationIDs[cmd.CorrelationID] = true
		if cmd.Status != "expired" {
			t.Errorf("cmd %s status = %q, want expired", cmd.CorrelationID, cmd.Status)
		}
		if cmd.Error != "extension_disconnected" {
			t.Errorf("cmd %s error = %q, want extension_disconnected", cmd.CorrelationID, cmd.Error)
		}
	}
	if !correlationIDs["corr-exp-1"] {
		t.Error("corr-exp-1 missing from failed commands")
	}
	if !correlationIDs["corr-exp-2"] {
		t.Error("corr-exp-2 missing from failed commands")
	}
}
