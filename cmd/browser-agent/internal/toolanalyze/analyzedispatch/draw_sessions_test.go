// Purpose: Tests for analyze draw-mode annotation retrieval.
// Docs: docs/features/feature/analyze-tool/index.md

// tools_analyze_annotations_draw_test.go — Tests for enriched annotation detail fields
// and draw history/session handlers.
package analyzedispatch

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/mediaapi"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolgenerate/annotations"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/annotation"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
)

func newDrawTestDispatcher(t *testing.T) (*Dispatcher, *annotation.Store) {
	t.Helper()
	// Isolate the state directory. Without this these tests read the developer's
	// real ~/.kaboom/screenshots — 4051 sessions on the machine where this was
	// found — so "EmptyDir" was not empty, results depended on whose laptop ran
	// them, and every case walked thousands of files.
	t.Setenv(state.StateDirEnv, t.TempDir())
	store := annotation.NewStore(10 * time.Minute)
	t.Cleanup(store.Close)
	return NewDispatcher(Config{AnnotationStore: store}), store
}

func drawResponseText(t *testing.T, raw json.RawMessage) string {
	t.Helper()
	var result mcp.MCPToolResult
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if len(result.Content) == 0 {
		t.Fatal("expected response content")
	}
	return result.Content[0].Text
}

func drawResponseJSON(text string) string {
	for index, character := range text {
		if character == '{' || character == '[' {
			return text[index:]
		}
	}
	return ""
}

func TestToolListDrawHistory_EmptyDir(t *testing.T) {
	dispatcher, _ := newDrawTestDispatcher(t)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	args := json.RawMessage(`{"what": "draw_history"}`)

	resp := dispatcher.DrawHistory(req, args)
	text := drawResponseText(t, resp.Result)
	jsonText := drawResponseJSON(text)

	var data map[string]any
	if err := json.Unmarshal([]byte(jsonText), &data); err != nil {
		t.Fatalf("Failed to parse response JSON: %v\nraw text: %s", err, text)
	}

	// count should be present (may be 0 or more depending on existing files)
	if _, exists := data["count"]; !exists {
		t.Error("expected 'count' field in response")
	}

	sessions, ok := data["sessions"].([]any)
	if !ok {
		t.Fatal("expected 'sessions' to be an array")
	}

	// Verify count matches sessions length
	count := data["count"].(float64)
	if int(count) != len(sessions) {
		t.Errorf("expected count=%d to match sessions length=%d", int(count), len(sessions))
	}
}

func TestToolListDrawHistory_WithSessions(t *testing.T) {
	dispatcher, _ := newDrawTestDispatcher(t)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	args := json.RawMessage(`{"what": "draw_history"}`)

	resp := dispatcher.DrawHistory(req, args)
	text := drawResponseText(t, resp.Result)
	jsonText := drawResponseJSON(text)

	var data map[string]any
	if err := json.Unmarshal([]byte(jsonText), &data); err != nil {
		t.Fatalf("Failed to parse response JSON: %v\nraw text: %s", err, text)
	}

	// Verify response has the expected shape
	if _, exists := data["count"]; !exists {
		t.Error("expected 'count' field in response")
	}
	if _, exists := data["sessions"]; !exists {
		t.Error("expected 'sessions' field in response")
	}
	if _, exists := data["storage_dir"]; !exists {
		t.Error("expected 'storage_dir' field in response")
	}

	// Verify sessions is an array (may be empty or populated)
	if _, ok := data["sessions"].([]any); !ok {
		t.Error("expected 'sessions' to be an array")
	}
}

func TestToolGetDrawSession_MissingFile(t *testing.T) {
	dispatcher, _ := newDrawTestDispatcher(t)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	args := json.RawMessage(`{"file": "draw-session-nonexistent.json"}`)

	resp := dispatcher.DrawSession(req, args)
	text := drawResponseText(t, resp.Result)

	if !strings.Contains(text, "not found") {
		t.Errorf("expected 'not found' error, got %q", text)
	}
}

func TestToolGetDrawSession_PathTraversal(t *testing.T) {
	dispatcher, _ := newDrawTestDispatcher(t)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	args := json.RawMessage(`{"file": "../../../etc/passwd"}`)

	resp := dispatcher.DrawSession(req, args)
	text := drawResponseText(t, resp.Result)

	if !strings.Contains(text, "path traversal") {
		t.Errorf("expected 'path traversal' error, got %q", text)
	}
}

func TestToolGetDrawSession_MissingParam(t *testing.T) {
	dispatcher, _ := newDrawTestDispatcher(t)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	args := json.RawMessage(`{}`)

	resp := dispatcher.DrawSession(req, args)
	text := drawResponseText(t, resp.Result)

	if !strings.Contains(text, "Required parameter 'file'") {
		t.Errorf("expected missing 'file' parameter error, got %q", text)
	}
}

func TestToolGetDrawSession_HydratesStoreForGenerators(t *testing.T) {
	dispatcher, store := newDrawTestDispatcher(t)

	t.Setenv("KABOOM_STATE_DIR", t.TempDir())
	dir, err := mediaapi.ScreenshotsDir()
	if err != nil {
		t.Fatalf("screenshotsDir: %v", err)
	}

	fileName := "draw-session-77-1700000000000.json"
	filePath := filepath.Join(dir, fileName)
	payload := `{
		"annotations": [{
			"id": "ann-1",
			"rect": {"x": 10, "y": 20, "width": 160, "height": 40},
			"text": "Button contrast is low",
			"timestamp": 1700000000000,
			"page_url": "https://example.com/checkout",
			"element_summary": "Checkout button",
			"correlation_id": "corr-qa-1"
		}],
		"element_details": {
			"corr-qa-1": {
				"selector": "button.checkout",
				"tag": "button",
				"text_content": "Checkout",
				"classes": ["checkout"],
				"computed_styles": {"color": "rgb(120,120,120)"},
				"parent_selector": "body",
				"bounding_rect": {"x": 10, "y": 20, "width": 160, "height": 40}
			}
		},
		"page_url": "https://example.com/checkout",
		"tab_id": 77,
		"screenshot": "/tmp/test.png",
		"timestamp": 1700000000000,
		"annot_session_name": "qa-review"
	}`
	if err := os.WriteFile(filePath, []byte(payload), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	loadResp := dispatcher.DrawSession(req, json.RawMessage(`{"file":"`+fileName+`"}`))
	loadText := drawResponseText(t, loadResp.Result)
	if !strings.Contains(loadText, `"annot_session":"qa-review"`) {
		t.Fatalf("draw_session should expose annot_session alias, got: %s", loadText)
	}

	reportResp := annotations.HandleAnnotationReport(store, req, json.RawMessage(`{"annot_session":"qa-review"}`))
	reportText := drawResponseText(t, reportResp.Result)
	if !strings.Contains(reportText, "# Annotation Report") {
		t.Fatalf("annotation_report should render report, got: %s", reportText)
	}
	if !strings.Contains(reportText, "Button contrast is low") {
		t.Fatalf("annotation_report missing loaded annotation text, got: %s", reportText)
	}

	detail, found := store.GetDetail("corr-qa-1")
	if !found || detail.Selector != "button.checkout" {
		t.Fatalf("annotation detail should be hydrated from file, got: %#v", detail)
	}
}
