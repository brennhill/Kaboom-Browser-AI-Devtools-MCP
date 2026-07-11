// handlers_coverage_test.go — Unit tests for the analyze-local MCP handlers and builders.
// Covers navigation dispatch, security/third-party audits, link validation, summary
// builders and detail-hint construction via a fake Deps + fake security scanner.

package analyzehandler

import (
	"encoding/json"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/security"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

// ---------------------------------------------------------------------------
// Test fakes
// ---------------------------------------------------------------------------

type fakeScanner struct {
	result any
	err    error
}

func (s fakeScanner) HandleSecurityAudit(_ json.RawMessage, _ []capture.NetworkBody, _ []security.LogEntry, _ []string, _ []capture.NetworkWaterfallEntry) (any, error) {
	return s.result, s.err
}

type fakeAnalyzeDeps struct {
	trackingEnabled bool
	tabID           int
	tabURL          string
	networkBodies   []capture.NetworkBody
	waterfall       []capture.NetworkWaterfallEntry
	consoleSec      []security.LogEntry
	scanner         SecurityScannerInterface
	scannerSet      bool
	logEntries      []types.LogEntry
	a11yResult      json.RawMessage
	a11yErr         error

	enqueueBlocked bool

	enqueueCalled   bool
	maybeWaitCalled bool
}

func (f *fakeAnalyzeDeps) EnqueuePendingQuery(req mcp.JSONRPCRequest, _ queries.PendingQuery, _ interface{ String() string }) (mcp.JSONRPCResponse, bool) {
	// placeholder — real signature defined below
	return mcp.JSONRPCResponse{}, false
}

func (f *fakeAnalyzeDeps) GetTrackingStatus() (bool, int, string) {
	return f.trackingEnabled, f.tabID, f.tabURL
}
func (f *fakeAnalyzeDeps) NetworkBodies() []capture.NetworkBody { return f.networkBodies }
func (f *fakeAnalyzeDeps) NetworkWaterfallEntries() []capture.NetworkWaterfallEntry {
	return f.waterfall
}
func (f *fakeAnalyzeDeps) ConsoleSecurityEntries() []security.LogEntry { return f.consoleSec }
func (f *fakeAnalyzeDeps) SecurityScanner() SecurityScannerInterface {
	return f.scanner
}
func (f *fakeAnalyzeDeps) LogEntries() []types.LogEntry { return f.logEntries }
func (f *fakeAnalyzeDeps) ExecuteA11yQuery(_ string, _ []string, _ any, _ bool) (json.RawMessage, error) {
	return f.a11yResult, f.a11yErr
}

func az_newReq() mcp.JSONRPCRequest { return mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1} }

func az_parse(t *testing.T, resp mcp.JSONRPCResponse) (bool, string) {
	t.Helper()
	var r struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if err := json.Unmarshal(resp.Result, &r); err != nil {
		t.Fatalf("failed to unmarshal result: %v (raw=%s)", err, string(resp.Result))
	}
	text := ""
	if len(r.Content) > 0 {
		text = r.Content[0].Text
	}
	return r.IsError, text
}
