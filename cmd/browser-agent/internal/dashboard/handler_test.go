// Purpose: Tests for dashboard HTML rendering.
// Docs: docs/features/feature/mcp-persistent-server/index.md

// handler_test.go — Tests for dashboard helpers.
package dashboard

import (
	"encoding/json"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func testJSONResponse(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func TestRootNegotiatesHTMLAndJSON(t *testing.T) {
	handler := Root(RootOptions{Name: "kaboom", Version: "1.2.3", JSONResponse: testJSONResponse})

	htmlRequest := httptest.NewRequest(http.MethodGet, "/", nil)
	htmlResponse := httptest.NewRecorder()
	handler(htmlResponse, htmlRequest)
	if htmlResponse.Code != http.StatusOK || htmlResponse.Header().Get("Content-Type") != "text/html; charset=utf-8" {
		t.Fatalf("HTML response status=%d content-type=%q", htmlResponse.Code, htmlResponse.Header().Get("Content-Type"))
	}

	jsonRequest := httptest.NewRequest(http.MethodGet, "/", nil)
	jsonRequest.Header.Set("Accept", "application/json")
	jsonResponse := httptest.NewRecorder()
	handler(jsonResponse, jsonRequest)
	var discovery map[string]string
	if err := json.Unmarshal(jsonResponse.Body.Bytes(), &discovery); err != nil {
		t.Fatalf("JSON discovery decode failed: %v", err)
	}
	if discovery["name"] != "kaboom" || discovery["version"] != "1.2.3" {
		t.Fatalf("discovery = %v", discovery)
	}
}

func TestStatusUsesInjectedRuntimeFacts(t *testing.T) {
	handler := Status(StatusOptions{
		Version: "1.2.3", StartedAt: time.Now().Add(-time.Minute), JSONResponse: testJSONResponse,
		Logs:       func() (int, int) { return 3, 100 },
		Terminal:   func() (int, int, []string) { return 7891, 2, []string{"one", "two"} },
		ListenPort: func() int { return 7890 },
		Audit:      func() any { return map[string]any{"score": 1} },
	})
	response := httptest.NewRecorder()
	handler(response, httptest.NewRequest(http.MethodGet, "/api/status", nil))

	var body map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("status decode failed: %v", err)
	}
	if body["version"] != "1.2.3" || body["listen_port"] != float64(7890) {
		t.Fatalf("status response = %v", body)
	}
}

func TestParseMCPCommand_ToolCalls(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantTool   string
		wantParams string
	}{
		{
			name:       "observe errors",
			body:       `{"jsonrpc":"2.0","method":"tools/call","params":{"name":"observe","arguments":{"what":"errors"}}}`,
			wantTool:   "observe",
			wantParams: "what=errors",
		},
		{
			name:       "interact navigate",
			body:       `{"jsonrpc":"2.0","method":"tools/call","params":{"name":"interact","arguments":{"what":"navigate","url":"https://example.com"}}}`,
			wantTool:   "interact",
			wantParams: "what=navigate url=https://example.com",
		},
		{
			name:       "interact click",
			body:       `{"jsonrpc":"2.0","method":"tools/call","params":{"name":"interact","arguments":{"what":"click","selector":"#btn"}}}`,
			wantTool:   "interact",
			wantParams: "what=click selector=#btn",
		},
		{
			name:       "analyze accessibility",
			body:       `{"jsonrpc":"2.0","method":"tools/call","params":{"name":"analyze","arguments":{"what":"accessibility"}}}`,
			wantTool:   "analyze",
			wantParams: "what=accessibility",
		},
		{
			name:       "generate test",
			body:       `{"jsonrpc":"2.0","method":"tools/call","params":{"name":"generate","arguments":{"what":"test"}}}`,
			wantTool:   "generate",
			wantParams: "what=test",
		},
		{
			name:       "configure clear",
			body:       `{"jsonrpc":"2.0","method":"tools/call","params":{"name":"configure","arguments":{"what":"clear","buffer":"all"}}}`,
			wantTool:   "configure",
			wantParams: "what=clear buffer=all",
		},
		{
			name:       "configure noise rule",
			body:       `{"jsonrpc":"2.0","method":"tools/call","params":{"name":"configure","arguments":{"what":"noise_rule","noise_action":"auto_detect"}}}`,
			wantTool:   "configure",
			wantParams: "what=noise_rule noise_action=auto_detect",
		},
		{
			name:       "empty body",
			body:       "",
			wantTool:   "unknown",
			wantParams: "",
		},
		{
			name:       "invalid json",
			body:       "not json",
			wantTool:   "unknown",
			wantParams: "",
		},
		{
			name:       "non-tool method",
			body:       `{"jsonrpc":"2.0","method":"initialize","params":{}}`,
			wantTool:   "initialize",
			wantParams: "",
		},
		{
			name:       "observe with no arguments",
			body:       `{"jsonrpc":"2.0","method":"tools/call","params":{"name":"observe","arguments":{}}}`,
			wantTool:   "observe",
			wantParams: "",
		},
		{
			name:       "long url truncated",
			body:       `{"jsonrpc":"2.0","method":"tools/call","params":{"name":"interact","arguments":{"what":"navigate","url":"https://example.com/very/long/path/that/exceeds/the/forty/character/limit/and/should/be/truncated"}}}`,
			wantTool:   "interact",
			wantParams: "what=navigate url=https://example.com/very/long/path/th...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tool, params := parseMCPCommand(tt.body)
			if tool != tt.wantTool {
				t.Errorf("tool = %q, want %q", tool, tt.wantTool)
			}
			if params != tt.wantParams {
				t.Errorf("params = %q, want %q", params, tt.wantParams)
			}
		})
	}
}

func TestBuildRecentCommands_UsesToolAndParams(t *testing.T) {
	entries := []types.HTTPDebugEntry{
		{
			Timestamp:      time.Now(),
			Endpoint:       "/mcp",
			Method:         "POST",
			RequestBody:    `{"jsonrpc":"2.0","method":"tools/call","params":{"name":"observe","arguments":{"what":"logs"}}}`,
			ResponseStatus: 200,
			DurationMs:     5,
		},
		{
			Timestamp:      time.Now().Add(-time.Second),
			Endpoint:       "/mcp",
			Method:         "POST",
			RequestBody:    `{"jsonrpc":"2.0","method":"tools/call","params":{"name":"interact","arguments":{"what":"click","selector":"#btn"}}}`,
			ResponseStatus: 200,
			DurationMs:     120,
		},
	}

	cmds := buildRecentCommands(entries)
	if len(cmds) != 2 {
		t.Fatalf("expected 2 commands, got %d", len(cmds))
	}

	// Newest first
	if cmds[0].Tool != "observe" {
		t.Errorf("cmds[0].Tool = %q, want observe", cmds[0].Tool)
	}
	if cmds[0].Params != "what=logs" {
		t.Errorf("cmds[0].Params = %q, want what=logs", cmds[0].Params)
	}
	if cmds[1].Tool != "interact" {
		t.Errorf("cmds[1].Tool = %q, want interact", cmds[1].Tool)
	}
	if cmds[1].Params != "what=click selector=#btn" {
		t.Errorf("cmds[1].Params = %q, want what=click selector=#btn", cmds[1].Params)
	}
}
