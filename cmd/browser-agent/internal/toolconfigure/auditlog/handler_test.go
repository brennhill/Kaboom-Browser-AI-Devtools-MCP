// handler_test.go — Verifies configure audit-log parsing, filtering, analysis, and clearing.

package auditlog

import (
	"encoding/json"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/audit"
)

func populatedTrail() *audit.AuditTrail {
	trail := audit.NewAuditTrail(audit.AuditConfig{MaxEntries: 10, Enabled: true})
	trail.Record(audit.AuditEntry{AuditSessionID: "session-a", ToolName: "observe", Success: true})
	trail.Record(audit.AuditEntry{AuditSessionID: "session-b", ToolName: "analyze", Success: false})
	return trail
}

func TestExecuteReportsFilteredEntries(t *testing.T) {
	result, problem := New(populatedTrail()).Execute(json.RawMessage(`{"tool_name":"observe"}`))
	if problem != nil {
		t.Fatalf("execute report: %v", problem)
	}
	if result.Operation != "report" || result.Count != 1 {
		t.Fatalf("expected one reported entry, got %#v", result)
	}
	if len(result.Entries) != 1 || result.Entries[0].ToolName != "observe" {
		t.Fatalf("unexpected entries: %#v", result.Entries)
	}
}

func TestExecuteAnalyzesEntries(t *testing.T) {
	result, problem := New(populatedTrail()).Execute(json.RawMessage(`{"operation":"analyze"}`))
	if problem != nil {
		t.Fatalf("execute analysis: %v", problem)
	}
	if result.Operation != "analyze" {
		t.Fatalf("expected analyze operation, got %q", result.Operation)
	}
	if result.Summary["total_calls"] != 2 || result.Summary["failure_count"] != 1 {
		t.Fatalf("unexpected summary: %#v", result.Summary)
	}
}

func TestExecuteClearsTrail(t *testing.T) {
	trail := populatedTrail()
	result, problem := New(trail).Execute(json.RawMessage(`{"operation":"clear"}`))
	if problem != nil {
		t.Fatalf("execute clear: %v", problem)
	}
	if result.Operation != "clear" || result.Cleared != 2 {
		t.Fatalf("unexpected clear result: %#v", result)
	}
	if remaining := trail.Query(audit.AuditFilter{}); len(remaining) != 0 {
		t.Fatalf("expected empty trail, got %d entries", len(remaining))
	}
}

func TestExecuteRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name string
		args json.RawMessage
		kind ProblemKind
	}{
		{name: "malformed JSON", args: json.RawMessage(`{`), kind: InvalidJSON},
		{name: "unknown operation", args: json.RawMessage(`{"operation":"delete"}`), kind: InvalidOperation},
		{name: "invalid timestamp", args: json.RawMessage(`{"since":"yesterday"}`), kind: InvalidSince},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, problem := New(populatedTrail()).Execute(tt.args)
			if problem == nil || problem.Kind != tt.kind {
				t.Fatalf("expected %s problem, got %#v", tt.kind, problem)
			}
		})
	}
}

func TestExecuteRequiresTrail(t *testing.T) {
	_, problem := New(nil).Execute(nil)
	if problem == nil || problem.Kind != Unavailable {
		t.Fatalf("expected unavailable problem, got %#v", problem)
	}
}
