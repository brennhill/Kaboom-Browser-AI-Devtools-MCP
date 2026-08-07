// handler_test.go — Shared fixtures for annotation analysis owner tests.

package annotationanalysis

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/asynccommand"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/annotation"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

type annotationTestLogs struct {
	entries []types.LogEntry
}

func (l *annotationTestLogs) Entries() []types.LogEntry {
	return append([]types.LogEntry(nil), l.entries...)
}

func (l *annotationTestLogs) SeedEntries(entries []types.LogEntry, _ []time.Time) {
	l.entries = append(l.entries, entries...)
}

type annotationTestServer struct {
	logs *annotationTestLogs
}

type annotationTestHarness struct {
	annotationAnalysis *Handler
	annotationStore    *annotation.Store
	capture            *capture.Capture
	asyncCommands      *asynccommand.Handler
	server             *annotationTestServer
}

func createTestToolHandler(t *testing.T) *annotationTestHarness {
	t.Helper()
	store := annotation.NewStore(10 * time.Minute)
	t.Cleanup(store.Close)
	captured := capture.NewCapture()
	commands := asynccommand.New(asynccommand.Deps{
		Capture:            captured,
		AttachEvidence:     func(string, map[string]any) {},
		AttachRetryContext: func(string, map[string]any, string, string) {},
	})
	server := &annotationTestServer{logs: &annotationTestLogs{}}
	return &annotationTestHarness{
		annotationAnalysis: New(store, captured, commands.FormatCommandResult, server.logs.Entries),
		annotationStore:    store,
		capture:            captured,
		asyncCommands:      commands,
		server:             server,
	}
}

func replaceAnnotationStoreForTest(h *annotationTestHarness, store *annotation.Store) {
	h.annotationStore = store
	h.annotationAnalysis = New(store, h.capture, h.asyncCommands.FormatCommandResult, h.server.logs.Entries)
}

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

func extractJSONFromText(text string) string {
	for index, character := range text {
		if character == '{' || character == '[' {
			return text[index:]
		}
	}
	return ""
}
