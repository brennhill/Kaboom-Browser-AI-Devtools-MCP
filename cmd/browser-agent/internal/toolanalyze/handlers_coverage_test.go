// handlers_coverage_test.go — Unit tests for the analyze-local MCP handlers and builders.
// Covers navigation dispatch, security/third-party audits, link validation, summary
// builders and detail-hint construction without a host-object compatibility fake.

package toolanalyze

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
)

func TestHandleLinkHealthQueuesCanonicalCommandAndForwardsParameters(t *testing.T) {
	t.Parallel()
	var queued queries.PendingQuery
	var queueTimeout time.Duration
	deps := Deps{
		EnqueuePendingQuery: func(_ mcp.JSONRPCRequest, query queries.PendingQuery, timeout time.Duration) (mcp.JSONRPCResponse, bool) {
			queued, queueTimeout = query, timeout
			return mcp.JSONRPCResponse{}, false
		},
		MaybeWaitForCommand: func(req mcp.JSONRPCRequest, correlationID string, args json.RawMessage, summary string) mcp.JSONRPCResponse {
			return mcp.Succeed(req, summary, map[string]any{"status": "queued", "correlation_id": correlationID})
		},
	}
	args := json.RawMessage(`{"domain":"example.com","timeout_ms":15000,"max_workers":20}`)
	response := HandleLinkHealth(deps, az_newReq(), args)
	isError, text := az_parse(t, response)
	if isError || !strings.Contains(text, `"status":"queued"`) || !strings.Contains(text, `"correlation_id":"link_health_`) {
		t.Fatalf("link health response = %s", text)
	}
	if queued.Type != "link_health" || queued.CorrelationID == "" || string(queued.Params) != string(args) || queueTimeout != queries.AsyncCommandTimeout {
		t.Fatalf("queued link health command = %#v, timeout=%s", queued, queueTimeout)
	}
	var params map[string]any
	if err := json.Unmarshal(queued.Params, &params); err != nil || params["domain"] != "example.com" {
		t.Fatalf("forwarded params = %#v, error=%v", params, err)
	}
}

func TestHandleLinkHealthReturnsQueueRejection(t *testing.T) {
	t.Parallel()
	deps := Deps{EnqueuePendingQuery: func(req mcp.JSONRPCRequest, _ queries.PendingQuery, _ time.Duration) (mcp.JSONRPCResponse, bool) {
		return mcp.Fail(req, mcp.ErrQueueFull, "queue full", "retry"), true
	}}
	isError, text := az_parse(t, HandleLinkHealth(deps, az_newReq(), nil))
	if !isError || !strings.Contains(text, mcp.ErrQueueFull) {
		t.Fatalf("queue rejection = error:%t text:%q", isError, text)
	}
}

func TestToolAnalyzePackageRespectsTenFileBoundary(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	files := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			files++
		}
	}
	if files > 10 {
		t.Fatalf("toolanalyze package has %d files; want at most 10 change-coupled owners", files)
	}
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
