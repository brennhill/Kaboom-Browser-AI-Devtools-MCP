// handler_test.go — Verifies configure audit-log parsing, filtering, analysis, and clearing.

package auditlog

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/audit"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

func populatedTrail() *audit.AuditTrail {
	trail := audit.NewAuditTrail(audit.AuditConfig{MaxEntries: 10, Enabled: true})
	trail.Record(audit.AuditEntry{AuditSessionID: "session-a", ToolName: "observe", Success: true})
	trail.Record(audit.AuditEntry{AuditSessionID: "session-b", ToolName: "analyze", Success: false})
	return trail
}

func TestExecuteReportsFilteredEntries(t *testing.T) {
	result, problem := newHandler(populatedTrail()).execute(json.RawMessage(`{"tool_name":"observe"}`))
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
	result, problem := newHandler(populatedTrail()).execute(json.RawMessage(`{"operation":"analyze"}`))
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
	result, problem := newHandler(trail).execute(json.RawMessage(`{"operation":"clear"}`))
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
			_, problem := newHandler(populatedTrail()).execute(tt.args)
			if problem == nil || problem.Kind != tt.kind {
				t.Fatalf("expected %s problem, got %#v", tt.kind, problem)
			}
		})
	}
}

func TestExecuteRequiresTrail(t *testing.T) {
	_, problem := newHandler(nil).execute(nil)
	if problem == nil || problem.Kind != Unavailable {
		t.Fatalf("expected unavailable problem, got %#v", problem)
	}
}

func TestHandleOwnsResponsesAndResetsRecorderSessionsOnClear(t *testing.T) {
	t.Parallel()
	trail := populatedTrail()
	recorder := audit.NewRecorder(trail)
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), ClientID: "client"}
	recorder.Record(req, "observe", nil, mcp.Succeed(req, "ok", nil), time.Now())
	if recorder.SessionID("client") == "" {
		t.Fatal("test setup did not create audit session")
	}

	for operation, field := range map[string]string{"analyze": `\"summary\"`, "report": `\"entries\"`} {
		response := Handle(trail, recorder, req, json.RawMessage(`{"operation":"`+operation+`"}`))
		if !strings.Contains(string(response.Result), `operation\":\"`+operation) || !strings.Contains(string(response.Result), field) {
			t.Fatalf("%s response = %s", operation, response.Result)
		}
	}
	response := Handle(trail, recorder, req, json.RawMessage(`{"operation":"clear"}`))
	if !strings.Contains(string(response.Result), `operation\":\"clear`) || recorder.SessionID("client") != "" {
		t.Fatalf("clear response/session = %s / %q", response.Result, recorder.SessionID("client"))
	}
}
