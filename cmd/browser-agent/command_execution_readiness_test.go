// Purpose: Tests for command execution readiness gating.
// Docs: docs/features/feature/mcp-persistent-server/index.md

package main

import (
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/health"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
)

func addCommandResultForTest(cap *capture.Capture, correlationID string, status string) {
	cap.Queries().RegisterCommand(correlationID, "query-"+correlationID, time.Minute)
	errText := ""
	if status != "complete" {
		errText = "synthetic-" + status
	}
	cap.Queries().ApplyCommandResult(correlationID, status, nil, errText)
}

func findDoctorCheck(checks []health.DoctorCheck, name string) (health.DoctorCheck, bool) {
	for _, check := range checks {
		if check.Name == name {
			return check, true
		}
	}
	return health.DoctorCheck{}, false
}

func TestCommandExecutionInfo_NoFailuresPass(t *testing.T) {
	t.Parallel()

	cap := capture.NewCapture()
	addCommandResultForTest(cap, "ok-1", "complete")

	info := health.BuildCommandExecutionInfoAt(cap, time.Now())
	if info.Status != "pass" {
		t.Fatalf("status = %q, want pass", info.Status)
	}
	if !info.Ready {
		t.Fatal("ready = false, want true")
	}
	if info.RecentSuccessCount != 1 {
		t.Fatalf("recent_success_count = %d, want 1", info.RecentSuccessCount)
	}
	if info.RecentFailedCount != 0 {
		t.Fatalf("recent_failed_count = %d, want 0", info.RecentFailedCount)
	}
	if info.LastSuccessAt == "" {
		t.Fatal("last_success_at should be populated")
	}
}

func TestCommandExecutionInfo_SingleFailureWarn(t *testing.T) {
	t.Parallel()

	cap := capture.NewCapture()
	addCommandResultForTest(cap, "fail-1", "expired")

	info := health.BuildCommandExecutionInfoAt(cap, time.Now())
	if info.Status != "warn" {
		t.Fatalf("status = %q, want warn", info.Status)
	}
	if info.Ready {
		t.Fatal("ready = true, want false")
	}
	if info.RecentFailedCount != 1 {
		t.Fatalf("recent_failed_count = %d, want 1", info.RecentFailedCount)
	}
	if info.RecentExpiredCount != 1 {
		t.Fatalf("recent_expired_count = %d, want 1", info.RecentExpiredCount)
	}
	if info.RecentFailureRatePct < 99.0 {
		t.Fatalf("recent_failure_rate_pct = %.2f, want ~100", info.RecentFailureRatePct)
	}
}

func TestCommandExecutionInfo_ThreeFailuresFail(t *testing.T) {
	t.Parallel()

	cap := capture.NewCapture()
	addCommandResultForTest(cap, "fail-expired", "expired")
	addCommandResultForTest(cap, "fail-timeout", "timeout")
	addCommandResultForTest(cap, "fail-error", "error")

	info := health.BuildCommandExecutionInfoAt(cap, time.Now())
	if info.Status != "fail" {
		t.Fatalf("status = %q, want fail", info.Status)
	}
	if info.Ready {
		t.Fatal("ready = true, want false")
	}
	if info.RecentFailedCount != 3 {
		t.Fatalf("recent_failed_count = %d, want 3", info.RecentFailedCount)
	}
}

func TestRunDoctorChecks_IncludesCommandExecution(t *testing.T) {
	t.Parallel()

	cap := capture.NewCapture()
	addCommandResultForTest(cap, "warn-expired", "expired")

	checks := health.RunDoctorChecks(cap)
	check, ok := findDoctorCheck(checks, "command_execution")
	if !ok {
		t.Fatal("expected command_execution check in doctor output")
	}
	if check.Status != "warn" {
		t.Fatalf("command_execution status = %q, want warn", check.Status)
	}
	if check.Fix == "" {
		t.Fatal("command_execution warn should include fix guidance")
	}
}

func TestHealthResponse_IncludesCommandExecution(t *testing.T) {
	t.Parallel()

	hm := health.NewMetrics()
	cap := capture.NewCapture()
	addCommandResultForTest(cap, "warn-timeout", "timeout")

	resp := getHealthResponse(hm, cap, nil, nil, nil, "test")
	if resp.CommandExecution.Status != "warn" {
		t.Fatalf("command_execution.status = %q, want warn", resp.CommandExecution.Status)
	}
	if resp.CommandExecution.Ready {
		t.Fatal("command_execution.ready = true, want false")
	}
	if resp.CommandExecution.RecentTimeoutCount != 1 {
		t.Fatalf("command_execution.recent_timeout_count = %d, want 1", resp.CommandExecution.RecentTimeoutCount)
	}
}
