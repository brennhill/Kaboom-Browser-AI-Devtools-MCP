// Purpose: Tests for health endpoint fast-path response.
// Docs: docs/features/feature/mcp-persistent-server/index.md

package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/logstore"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/bridge"
	statecfg "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
)

func TestHandleHealthIncludesBridgeFastPathCounters(t *testing.T) {
	t.Setenv(statecfg.StateDirEnv, t.TempDir())
	bridge.ResetFastPathResourceReadCounters()
	bridge.RecordFastPathResourceRead("kaboom://capabilities", true, 0)
	bridge.RecordFastPathResourceRead("kaboom://playbook/nonexistent/quick", false, -32002)

	s := &Server{
		logs: logstore.New(logstore.Config{
			MaxEntries: 100,
			LogFile:    filepath.Join(t.TempDir(), "kaboom.jsonl"),
			AddWarning: func(string) {},
		}),
	}

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	s.handleHealth(rr, req, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("handleHealth status = %d, want %d", rr.Code, http.StatusOK)
	}

	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal(health) error = %v", err)
	}
	fastPath, ok := body["bridge_fastpath"].(map[string]any)
	if !ok {
		t.Fatalf("bridge_fastpath missing or wrong type: %#v", body["bridge_fastpath"])
	}
	if got, _ := fastPath["resources_read_success"].(float64); int(got) != 1 {
		t.Fatalf("resources_read_success = %v, want 1", fastPath["resources_read_success"])
	}
	if got, _ := fastPath["resources_read_failure"].(float64); int(got) != 1 {
		t.Fatalf("resources_read_failure = %v, want 1", fastPath["resources_read_failure"])
	}
}
