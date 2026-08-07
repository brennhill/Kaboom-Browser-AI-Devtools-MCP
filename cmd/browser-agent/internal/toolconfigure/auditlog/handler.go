// handler.go — Owns configure audit-log parsing, validation, filtering, analysis, and clearing.

package auditlog

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/audit"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	cfg "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/configure"
)

type ProblemKind string

const (
	Unavailable      ProblemKind = "unavailable"
	InvalidJSON      ProblemKind = "invalid_json"
	InvalidOperation ProblemKind = "invalid_operation"
	InvalidSince     ProblemKind = "invalid_since"
)

type Problem struct {
	Kind    ProblemKind
	Message string
}

// Handle executes an audit-log operation and owns its complete MCP response boundary.
func Handle(trail *audit.AuditTrail, recorder *audit.Recorder, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	result, problem := newHandler(trail).execute(args)
	if problem != nil {
		switch problem.Kind {
		case Unavailable:
			return mcp.Fail(req, mcp.ErrNotInitialized, problem.Message, "Internal error — do not retry")
		case InvalidJSON:
			return mcp.Fail(req, mcp.ErrInvalidJSON, "Invalid JSON arguments: "+problem.Message, "Fix JSON syntax and call again")
		case InvalidOperation:
			return mcp.Fail(req, mcp.ErrInvalidParam, problem.Message, "Use operation: analyze, report, or clear", mcp.WithParam("operation"))
		default:
			return mcp.Fail(req, mcp.ErrInvalidParam, problem.Message, "Use RFC3339 format, for example 2026-02-17T15:04:05Z", mcp.WithParam("since"))
		}
	}

	switch result.Operation {
	case "clear":
		recorder.ResetSessions()
		return mcp.Succeed(req, "Audit log cleared", map[string]any{
			"status": "ok", "operation": result.Operation, "cleared": result.Cleared,
		})
	case "analyze":
		return mcp.Succeed(req, "Audit log analysis", map[string]any{
			"status": "ok", "operation": result.Operation, "summary": result.Summary,
		})
	default:
		return mcp.Succeed(req, "Audit log entries", map[string]any{
			"status": "ok", "operation": result.Operation, "entries": result.Entries, "count": result.Count,
		})
	}
}

func (p *Problem) Error() string {
	return p.Message
}

type Result struct {
	Operation string
	Entries   []audit.AuditEntry
	Count     int
	Summary   map[string]any
	Cleared   int
}

type Handler struct {
	trail *audit.AuditTrail
}

func newHandler(trail *audit.AuditTrail) *Handler {
	return &Handler{trail: trail}
}

func (h *Handler) execute(args json.RawMessage) (Result, *Problem) {
	if h.trail == nil {
		return Result{}, &Problem{Kind: Unavailable, Message: "Audit trail not initialized"}
	}

	var params struct {
		Operation      string `json:"operation"`
		AuditSessionID string `json:"audit_session_id"`
		ToolName       string `json:"tool_name"`
		Limit          int    `json:"limit"`
		Since          string `json:"since"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &params); err != nil {
			return Result{}, &Problem{Kind: InvalidJSON, Message: err.Error()}
		}
	}

	operation := strings.ToLower(strings.TrimSpace(params.Operation))
	if operation == "" {
		operation = "report"
	}
	if operation != "analyze" && operation != "report" && operation != "clear" {
		return Result{}, &Problem{
			Kind:    InvalidOperation,
			Message: "Invalid audit_log operation: " + params.Operation,
		}
	}
	if operation == "clear" {
		return Result{Operation: operation, Cleared: h.trail.Clear()}, nil
	}

	filter := audit.AuditFilter{
		AuditSessionID: params.AuditSessionID,
		ToolName:       params.ToolName,
		Limit:          params.Limit,
	}
	if params.Since != "" {
		since, err := time.Parse(time.RFC3339, params.Since)
		if err != nil {
			return Result{}, &Problem{
				Kind:    InvalidSince,
				Message: fmt.Sprintf("Invalid 'since' timestamp: %v", err),
			}
		}
		filter.Since = &since
	}

	entries := h.trail.Query(filter)
	result := Result{Operation: operation, Entries: entries, Count: len(entries)}
	if operation == "analyze" {
		result.Summary = cfg.SummarizeAuditEntries(entries)
		result.Entries = nil
	}
	return result, nil
}
