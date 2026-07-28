// Purpose: Tests for analyze annotation processing.
// Docs: docs/features/feature/analyze-tool/index.md

// tools_analyze_annotations_test.go — Tests for analyze annotations handlers.
package main

import (
	"encoding/json"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolanalyze/annotationanalysis"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/annotation"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

func replaceAnnotationStoreForTest(h *ToolHandler, store *annotation.Store) {
	h.annotationStore = store
	h.annotationAnalysis = annotationanalysis.New(
		store,
		h.capture,
		h.asyncCommands.FormatCommandResult,
		h.server.logs.Entries,
	)
}

// unmarshalMCPText extracts the text from an MCP tool response.
func unmarshalMCPText(t *testing.T, raw json.RawMessage) string {
	t.Helper()
	var result mcp.MCPToolResult
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}
	if len(result.Content) == 0 {
		t.Fatal("expected at least one content block")
	}
	return result.Content[0].Text
}
