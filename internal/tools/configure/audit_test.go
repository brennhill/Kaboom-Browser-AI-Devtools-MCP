// Purpose: Tests for configure audit log handler.
// Docs: docs/features/feature/enhanced-cli-config/index.md

// audit_test.go — Tests for audit log summarization.
package configure

import (
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/audit"
)

func TestSummarizeAuditEntries_Empty(t *testing.T) {
	t.Parallel()

	result := SummarizeAuditEntries(nil)
	if result["total_calls"] != 0 {
		t.Errorf("total_calls = %v, want 0", result["total_calls"])
	}
	if result["success_count"] != 0 {
		t.Errorf("success_count = %v, want 0", result["success_count"])
	}
	if result["failure_count"] != 0 {
		t.Errorf("failure_count = %v, want 0", result["failure_count"])
	}
	if result["audit_session_count"] != 0 {
		t.Errorf("audit_session_count = %v, want 0", result["audit_session_count"])
	}
	byTool, ok := result["calls_by_tool"].(map[string]int)
	if !ok {
		t.Fatal("calls_by_tool should be map[string]int")
	}
	if len(byTool) != 0 {
		t.Errorf("calls_by_tool should be empty, got %v", byTool)
	}
}

func TestSummarizeAuditEntries_MixedEntries(t *testing.T) {
	t.Parallel()

	entries := []audit.AuditEntry{
		{ToolName: "observe", AuditSessionID: "s1", Success: true},
		{ToolName: "observe", AuditSessionID: "s1", Success: true},
		{ToolName: "configure", AuditSessionID: "s2", Success: false},
		{ToolName: "generate", AuditSessionID: "s2", Success: true},
	}

	result := SummarizeAuditEntries(entries)
	if result["total_calls"] != 4 {
		t.Errorf("total_calls = %v, want 4", result["total_calls"])
	}
	if result["success_count"] != 3 {
		t.Errorf("success_count = %v, want 3", result["success_count"])
	}
	if result["failure_count"] != 1 {
		t.Errorf("failure_count = %v, want 1", result["failure_count"])
	}
	if result["audit_session_count"] != 2 {
		t.Errorf("audit_session_count = %v, want 2", result["audit_session_count"])
	}

	byTool := result["calls_by_tool"].(map[string]int)
	if byTool["observe"] != 2 {
		t.Errorf("calls_by_tool[observe] = %d, want 2", byTool["observe"])
	}
	if byTool["configure"] != 1 {
		t.Errorf("calls_by_tool[configure] = %d, want 1", byTool["configure"])
	}
	if byTool["generate"] != 1 {
		t.Errorf("calls_by_tool[generate] = %d, want 1", byTool["generate"])
	}
}

func TestSummarizeAuditEntries_AllFailures(t *testing.T) {
	t.Parallel()

	entries := []audit.AuditEntry{
		{ToolName: "interact", AuditSessionID: "s1", Success: false},
		{ToolName: "interact", AuditSessionID: "s1", Success: false},
	}

	result := SummarizeAuditEntries(entries)
	if result["success_count"] != 0 {
		t.Errorf("success_count = %v, want 0", result["success_count"])
	}
	if result["failure_count"] != 2 {
		t.Errorf("failure_count = %v, want 2", result["failure_count"])
	}
}
