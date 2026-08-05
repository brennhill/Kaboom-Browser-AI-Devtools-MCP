// handlers_coverage_test.go — Unit tests for the analyze-local MCP handlers and builders.
// Covers navigation dispatch, security/third-party audits, link validation, summary
// builders and detail-hint construction without a host-object compatibility fake.

package toolanalyze

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

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
