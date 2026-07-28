// Purpose: Unit tests for ExtensionLogBuffer store helpers (append/snapshot/clear).
// Why: Guards the extracted extension-log store behavior after Capture decomposition.
// Docs: docs/features/feature/backend-log-streaming/index.md

package capture

import (
	"fmt"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"testing"
	"time"
)

func TestExtensionLogBuffer_AppendAppliesAmortizedEviction(t *testing.T) {
	t.Parallel()

	buf := ExtensionLogBuffer{logs: make([]types.ExtensionLog, 0)}
	total := MaxExtensionLogs + MaxExtensionLogs/2 + 1
	for i := 0; i < total; i++ {
		buf.append(types.ExtensionLog{
			Level:     "info",
			Message:   fmt.Sprintf("log-%d", i),
			Timestamp: time.Unix(int64(i), 0),
		})
	}

	if got := len(buf.logs); got != MaxExtensionLogs {
		t.Fatalf("buffer length = %d, want %d", got, MaxExtensionLogs)
	}

	// After compaction we should retain the newest MaxExtensionLogs entries.
	expectedFirst := total - MaxExtensionLogs
	if got := buf.logs[0].Message; got != fmt.Sprintf("log-%d", expectedFirst) {
		t.Fatalf("first kept log = %q, want %q", got, fmt.Sprintf("log-%d", expectedFirst))
	}
	if got := buf.logs[len(buf.logs)-1].Message; got != fmt.Sprintf("log-%d", total-1) {
		t.Fatalf("last kept log = %q, want %q", got, fmt.Sprintf("log-%d", total-1))
	}
}

func TestExtensionLogBuffer_SnapshotReturnsDetachedCopy(t *testing.T) {
	t.Parallel()

	buf := ExtensionLogBuffer{
		logs: []types.ExtensionLog{{Level: "info", Message: "one"}, {Level: "warn", Message: "two"}},
	}

	snap := buf.snapshot()
	if len(snap) != 2 {
		t.Fatalf("snapshot len = %d, want 2", len(snap))
	}
	snap[0].Message = "mutated"

	if buf.logs[0].Message != "one" {
		t.Fatalf("buffer should remain unchanged, got %q", buf.logs[0].Message)
	}
}

func TestExtensionLogBuffer_ClearReturnsCountAndEmpties(t *testing.T) {
	t.Parallel()

	buf := ExtensionLogBuffer{
		logs: []types.ExtensionLog{{Level: "info", Message: "one"}, {Level: "warn", Message: "two"}},
	}

	count := buf.clear()
	if count != 2 {
		t.Fatalf("clear count = %d, want 2", count)
	}
	if len(buf.logs) != 0 {
		t.Fatalf("buffer len after clear = %d, want 0", len(buf.logs))
	}
}
