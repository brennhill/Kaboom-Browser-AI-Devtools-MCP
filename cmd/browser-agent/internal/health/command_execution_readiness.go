// command_execution_readiness.go — Assesses async command execution reliability by analyzing recent success/failure/timeout rates.
// Why: Surfaces command queue health in doctor and health endpoints to diagnose extension responsiveness.

package health

import (
	"fmt"
	"strings"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
)

const (
	commandExecutionWindow      = 5 * time.Minute
	commandFailureWarnThreshold = 1
	commandFailureFailThreshold = 3
	commandPendingStallWarnAge  = 45 * time.Second
	commandPendingStallFailAge  = 2 * time.Minute
)

func BuildCommandExecutionInfo(cap *capture.Capture) CommandExecutionInfo {
	return BuildCommandExecutionInfoAt(cap, time.Now())
}

func BuildCommandExecutionInfoAt(cap *capture.Capture, now time.Time) CommandExecutionInfo {
	info := CommandExecutionInfo{
		Ready:         true,
		Status:        "pass",
		WindowSeconds: int(commandExecutionWindow.Seconds()),
	}
	if cap == nil {
		info.Ready = false
		info.Status = "fail"
		info.Detail = "Capture not initialized"
		return info
	}

	pending := cap.Queries().GetPendingCommands()
	failed := cap.Queries().GetFailedCommands()
	completed := cap.Queries().GetCompletedCommands()

	info.QueueDepth = cap.Queries().QueueDepth()
	info.PendingCount = len(pending)

	var oldestPendingAge time.Duration
	for _, cmd := range pending {
		if cmd == nil || cmd.CreatedAt.IsZero() {
			continue
		}
		age := now.Sub(cmd.CreatedAt)
		if age < 0 {
			age = 0
		}
		if age > oldestPendingAge {
			oldestPendingAge = age
		}
	}
	if oldestPendingAge > 0 {
		info.OldestPendingAgeMs = oldestPendingAge.Milliseconds()
	}

	var lastSuccess time.Time
	for _, cmd := range completed {
		if cmd == nil || cmd.Status != "complete" {
			continue
		}
		eventTime := cmd.CompletedAt
		if eventTime.IsZero() {
			eventTime = cmd.CreatedAt
		}
		if eventTime.IsZero() {
			continue
		}
		if eventTime.After(lastSuccess) {
			lastSuccess = eventTime
		}
		age := now.Sub(eventTime)
		if age < 0 || age > commandExecutionWindow {
			continue
		}
		info.RecentSuccessCount++
	}
	if !lastSuccess.IsZero() {
		info.LastSuccessAt = lastSuccess.UTC().Format(time.RFC3339Nano)
		lastSuccessAge := now.Sub(lastSuccess)
		if lastSuccessAge < 0 {
			lastSuccessAge = 0
		}
		info.LastSuccessAgeMs = lastSuccessAge.Milliseconds()
	}

	for _, cmd := range failed {
		if cmd == nil {
			continue
		}
		eventTime := cmd.CompletedAt
		if eventTime.IsZero() {
			eventTime = cmd.CreatedAt
		}
		if eventTime.IsZero() {
			continue
		}
		age := now.Sub(eventTime)
		if age < 0 || age > commandExecutionWindow {
			continue
		}
		info.RecentFailedCount++
		switch cmd.Status {
		case "expired":
			info.RecentExpiredCount++
		case "timeout":
			info.RecentTimeoutCount++
		case "error":
			info.RecentErrorCount++
		case "cancelled":
			info.RecentCancelledCount++
		}
	}

	attempts := info.RecentSuccessCount + info.RecentFailedCount
	if attempts > 0 {
		info.RecentFailureRatePct = float64(info.RecentFailedCount) * 100 / float64(attempts)
	}

	detailParts := []string{
		fmt.Sprintf("window=%ds", info.WindowSeconds),
	}

	switch {
	case info.RecentFailedCount >= commandFailureFailThreshold:
		info.Status = "fail"
	case info.RecentFailedCount >= commandFailureWarnThreshold:
		info.Status = "warn"
	}

	if info.RecentFailedCount == 0 {
		detailParts = append(detailParts, "no recent command failures")
	} else {
		detailParts = append(detailParts, fmt.Sprintf(
			"recent failures=%d/%d (%.1f%%): expired=%d timeout=%d error=%d cancelled=%d",
			info.RecentFailedCount,
			attempts,
			info.RecentFailureRatePct,
			info.RecentExpiredCount,
			info.RecentTimeoutCount,
			info.RecentErrorCount,
			info.RecentCancelledCount,
		))
	}

	lastSuccessAge := time.Duration(info.LastSuccessAgeMs) * time.Millisecond
	lastSuccessStaleWarn := info.LastSuccessAt == "" || lastSuccessAge >= commandPendingStallWarnAge
	lastSuccessStaleFail := info.LastSuccessAt == "" || lastSuccessAge >= commandPendingStallFailAge
	pendingStallWarn := info.PendingCount > 0 &&
		time.Duration(info.OldestPendingAgeMs)*time.Millisecond >= commandPendingStallWarnAge &&
		lastSuccessStaleWarn
	pendingStallFail := info.PendingCount > 0 &&
		time.Duration(info.OldestPendingAgeMs)*time.Millisecond >= commandPendingStallFailAge &&
		lastSuccessStaleFail

	if pendingStallFail {
		info.Status = "fail"
	}
	if pendingStallWarn && info.Status == "pass" {
		info.Status = "warn"
	}
	if pendingStallWarn {
		lastSuccessHint := "none"
		if info.LastSuccessAt != "" {
			lastSuccessHint = fmt.Sprintf("%.1fs ago", float64(info.LastSuccessAgeMs)/1000.0)
		}
		detailParts = append(detailParts, fmt.Sprintf(
			"pending backlog: %d command(s), oldest=%.1fs, last_success=%s",
			info.PendingCount,
			float64(info.OldestPendingAgeMs)/1000.0,
			lastSuccessHint,
		))
	}

	info.Ready = info.Status == "pass"
	info.Detail = strings.Join(detailParts, "; ")
	return info
}
