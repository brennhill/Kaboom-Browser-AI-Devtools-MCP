// contract.go — Shared batch and saved-sequence replay primitives.
// Docs: docs/features/feature/batch-sequences/index.md

package replay

import (
	"encoding/json"
	"strings"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

const (
	MaxSteps           = 50
	DefaultStepTimeout = 10_000
)

// StepResult describes one replayed interact step.
type StepResult struct {
	StepIndex     int    `json:"step_index"`
	Action        string `json:"action"`
	CorrelationID string `json:"correlation_id,omitempty"`
	Status        string `json:"status"`
	Error         string `json:"error,omitempty"`
	DurationMs    int64  `json:"duration_ms"`
}

// ForceAsync prevents an individual replay step from blocking internally.
func ForceAsync(stepArgs json.RawMessage) json.RawMessage {
	var args map[string]any
	if err := json.Unmarshal(stepArgs, &args); err != nil {
		return stepArgs
	}
	args["sync"] = false
	args["wait"] = false
	updated, err := json.Marshal(args)
	if err != nil {
		return stepArgs
	}
	return updated
}

// CorrelationID extracts correlation_id from an MCP tool response.
func CorrelationID(resp mcp.JSONRPCResponse) string {
	if resp.Result == nil {
		return ""
	}
	var result mcp.MCPToolResult
	if json.Unmarshal(resp.Result, &result) != nil {
		return ""
	}
	for _, block := range result.Content {
		text := block.Text
		if idx := strings.Index(text, "\n{"); idx >= 0 {
			text = text[idx+1:]
		}
		var data map[string]any
		if block.Type == "text" && json.Unmarshal([]byte(text), &data) == nil {
			if id, ok := data["correlation_id"].(string); ok {
				return id
			}
		}
	}
	return ""
}

// ErrorMessage extracts a human-readable MCP tool error.
func ErrorMessage(resp mcp.JSONRPCResponse) string {
	if resp.Result == nil {
		return ""
	}
	var result mcp.MCPToolResult
	if json.Unmarshal(resp.Result, &result) != nil {
		return ""
	}
	for _, block := range result.Content {
		if block.Type != "text" || block.Text == "" {
			continue
		}
		var data map[string]any
		structured := block.Text
		if index := strings.IndexByte(structured, '{'); index >= 0 {
			structured = structured[index:]
		}
		if json.Unmarshal([]byte(structured), &data) == nil {
			if message, ok := data["message"].(string); ok {
				return message
			}
		}
		return block.Text
	}
	return ""
}
