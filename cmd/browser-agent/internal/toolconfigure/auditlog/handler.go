// handler.go — Owns configure audit-log parsing, validation, filtering, analysis, and clearing.

package auditlog

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/audit"
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

func New(trail *audit.AuditTrail) *Handler {
	return &Handler{trail: trail}
}

func (h *Handler) Execute(args json.RawMessage) (Result, *Problem) {
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
