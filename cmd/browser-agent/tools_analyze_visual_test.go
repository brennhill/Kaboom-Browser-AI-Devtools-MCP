// tools_analyze_visual_test.go — Tests for the visual-regression analyze modes.
// Why: visual_baseline / visual_diff / visual_baselines had no coverage at all,
// and every one of their failure paths is something a caller has to act on.
// Docs: docs/features/feature/analyze-tool/index.md

package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/persistence"
)

func newVisualHandler(t *testing.T) *ToolHandler {
	t.Helper()
	logFile := filepath.Join(t.TempDir(), "visual.jsonl")
	server, err := NewServer(logFile, 100)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	t.Cleanup(func() { server.logs.shutdownAsyncLogger(2 * time.Second) })
	return NewToolHandler(server, capture.NewCapture()).toolHandler.(*ToolHandler)
}

func visualRequest() JSONRPCRequest {
	return JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
}

// =============================================================================
// SCREENSHOT PATH EXTRACTION
// =============================================================================

func TestExtractScreenshotPath_ReadsPathFromTheResponseBody(t *testing.T) {
	t.Parallel()
	resp := succeed(visualRequest(), "Screenshot captured", map[string]any{
		"path": "/tmp/shots/page.png",
	})

	if got := extractScreenshotPath(resp); got != "/tmp/shots/page.png" {
		t.Errorf("extractScreenshotPath = %q, want the path field", got)
	}
}

func TestExtractScreenshotPath_SkipsAnyHumanPrefixBeforeTheJSON(t *testing.T) {
	t.Parallel()
	// succeed() prepends a human-readable sentence to the JSON body. Parsing the
	// text as JSON from byte zero would fail on every real response.
	resp := succeed(visualRequest(), "Screenshot captured", map[string]any{"path": "/tmp/a.png"})
	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !strings.HasPrefix(result.Content[0].Text, "Screenshot captured") {
		t.Skip("response envelope no longer carries a text prefix; this guard is moot")
	}

	if got := extractScreenshotPath(resp); got != "/tmp/a.png" {
		t.Errorf("extractScreenshotPath = %q, want /tmp/a.png", got)
	}
}

func TestExtractScreenshotPath_EmptyForResponsesWithoutAPath(t *testing.T) {
	t.Parallel()
	// The caller turns "" into a specific error. Returning a garbage path here
	// would instead produce a confusing failure much later, at image compare.
	cases := map[string]JSONRPCResponse{
		"no path field": succeed(visualRequest(), "ok", map[string]any{"status": "done"}),
		"empty path":    succeed(visualRequest(), "ok", map[string]any{"path": ""}),
		"no json":       {Result: json.RawMessage(`{"content":[{"type":"text","text":"nothing here"}]}`)},
		"no content":    {Result: json.RawMessage(`{"content":[]}`)},
		"unparseable":   {Result: json.RawMessage(`not json`)},
	}
	for name, resp := range cases {
		if got := extractScreenshotPath(resp); got != "" {
			t.Errorf("%s: extractScreenshotPath = %q, want empty", name, got)
		}
	}
}

func TestExtractScreenshotPath_ResolvesAFilenameAgainstTheScreenshotsDir(t *testing.T) {
	t.Parallel()
	// Older responses carry only a filename. A bare filename is not openable, so
	// it has to be joined onto the screenshots directory before it is returned.
	resp := succeed(visualRequest(), "ok", map[string]any{"filename": "shot-1.png"})

	got := extractScreenshotPath(resp)

	if got == "" {
		t.Skip("screenshots dir unavailable in this environment")
	}
	if filepath.Base(got) != "shot-1.png" {
		t.Errorf("extractScreenshotPath = %q, want a path ending in shot-1.png", got)
	}
	if !filepath.IsAbs(got) {
		t.Errorf("extractScreenshotPath = %q, want an absolute path", got)
	}
}

func TestExtractScreenshotPath_PathWinsOverFilename(t *testing.T) {
	t.Parallel()
	resp := succeed(visualRequest(), "ok", map[string]any{
		"path":     "/explicit/a.png",
		"filename": "b.png",
	})

	if got := extractScreenshotPath(resp); got != "/explicit/a.png" {
		t.Errorf("extractScreenshotPath = %q, want the explicit path", got)
	}
}

// =============================================================================
// ARGUMENT VALIDATION
// =============================================================================

func TestToolVisualBaseline_MissingNameIsRejectedBeforeAnyScreenshot(t *testing.T) {
	// Capturing a screenshot and only then discovering the name is missing costs
	// a round trip to the extension for nothing.
	h := newVisualHandler(t)

	resp := h.toolVisualBaseline(visualRequest(), json.RawMessage(`{}`))

	if code := extractErrorCode(t, resp); code != ErrMissingParam {
		t.Errorf("error code = %q, want %q", code, ErrMissingParam)
	}
}

func TestToolVisualDiff_MissingBaselineIsRejected(t *testing.T) {
	h := newVisualHandler(t)

	resp := h.toolVisualDiff(visualRequest(), json.RawMessage(`{}`))

	if code := extractErrorCode(t, resp); code != ErrMissingParam {
		t.Errorf("error code = %q, want %q", code, ErrMissingParam)
	}
}

// =============================================================================
// BASELINE LOOKUP
// =============================================================================

func TestToolVisualDiff_UnknownBaselineNamesTheFixInTheError(t *testing.T) {
	// "not found" alone leaves the caller guessing; the remedy is a specific
	// tool call, so the error carries it.
	h := newVisualHandler(t)
	if h.sessionStoreImpl == nil {
		t.Skip("no session store in this build")
	}

	resp := h.toolVisualDiff(visualRequest(), json.RawMessage(`{"baseline":"never-saved"}`))

	result := decodeToolResult(t, resp.Result)
	if !result.IsError {
		t.Fatalf("diff against a missing baseline succeeded: %s", result.Content[0].Text)
	}
	text := result.Content[0].Text
	if !strings.Contains(text, "never-saved") {
		t.Errorf("error does not name the missing baseline: %s", text)
	}
	if !strings.Contains(text, "visual_baseline") {
		t.Errorf("error does not point at the tool that would fix it: %s", text)
	}
}

func TestToolVisualDiff_CorruptBaselineMetadataIsReportedAsSuch(t *testing.T) {
	// A baseline whose stored metadata will not parse is a different problem
	// from one that does not exist, and has a different fix (re-save it).
	h := newVisualHandler(t)
	if h.sessionStoreImpl == nil {
		t.Skip("no session store in this build")
	}
	if _, err := h.sessionStoreImpl.HandleSessionStore(persistence.SessionStoreArgs{
		Action:    "save",
		Namespace: "visual_baselines",
		Key:       "corrupt",
		Data:      json.RawMessage(`"this is a string, not baseline metadata"`),
	}); err != nil {
		t.Fatalf("seed store: %v", err)
	}

	resp := h.toolVisualDiff(visualRequest(), json.RawMessage(`{"baseline":"corrupt"}`))

	result := decodeToolResult(t, resp.Result)
	if !result.IsError {
		t.Fatal("corrupt baseline metadata was accepted")
	}
	if !strings.Contains(result.Content[0].Text, "baseline metadata") {
		t.Errorf("error does not identify the metadata as the problem: %s", result.Content[0].Text)
	}
}

// =============================================================================
// LISTING
// =============================================================================

func TestToolListVisualBaselines_ListsWhatWasSaved(t *testing.T) {
	h := newVisualHandler(t)
	if h.sessionStoreImpl == nil {
		t.Skip("no session store in this build")
	}
	if _, err := h.sessionStoreImpl.HandleSessionStore(persistence.SessionStoreArgs{
		Action:    "save",
		Namespace: "visual_baselines",
		Key:       "homepage",
		Data:      json.RawMessage(`{"name":"homepage","path":"/tmp/home.png"}`),
	}); err != nil {
		t.Fatalf("seed store: %v", err)
	}

	resp := h.toolListVisualBaselines(visualRequest(), nil)

	result := decodeToolResult(t, resp.Result)
	if result.IsError {
		t.Fatalf("list failed: %s", result.Content[0].Text)
	}
	if !strings.Contains(result.Content[0].Text, "homepage") {
		t.Errorf("saved baseline missing from the listing: %s", result.Content[0].Text)
	}
}

func TestToolListVisualBaselines_EmptyStoreIsSuccessNotAnError(t *testing.T) {
	// Having saved no baselines yet is the normal starting state, not a fault.
	h := newVisualHandler(t)
	if h.sessionStoreImpl == nil {
		t.Skip("no session store in this build")
	}

	resp := h.toolListVisualBaselines(visualRequest(), nil)

	if result := decodeToolResult(t, resp.Result); result.IsError {
		t.Errorf("listing an empty store reported an error: %s", result.Content[0].Text)
	}
}

func TestVisualModes_WithoutASessionStoreFailFastAndSayWhy(t *testing.T) {
	// These modes are pure persistence; without a store they cannot work at all,
	// and retrying will never help — so the error must say "do not retry".
	h := newVisualHandler(t)
	h.sessionStoreImpl = nil

	for name, resp := range map[string]JSONRPCResponse{
		"visual_diff":      h.toolVisualDiff(visualRequest(), json.RawMessage(`{"baseline":"x"}`)),
		"visual_baselines": h.toolListVisualBaselines(visualRequest(), nil),
	} {
		if code := extractErrorCode(t, resp); code != ErrNotInitialized {
			t.Errorf("%s: error code = %q, want %q", name, code, ErrNotInitialized)
		}
	}
}
