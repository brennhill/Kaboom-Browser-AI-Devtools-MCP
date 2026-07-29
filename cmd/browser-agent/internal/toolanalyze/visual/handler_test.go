// handler_test.go — Tests visual-regression analyze handlers.

package visual

import (
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/persistence"
)

type fakeDeps struct {
	stored         persistence.SessionStoreArgs
	screenshotPath string
	hasStore       bool
	storeResult    json.RawMessage
	storeErr       error
}

func (f *fakeDeps) CaptureScreenshot(req mcp.JSONRPCRequest) mcp.JSONRPCResponse {
	path := f.screenshotPath
	if path == "" {
		path = "/tmp/current.png"
	}
	return mcp.Succeed(req, "Screenshot captured", map[string]any{"path": path})
}

func (f *fakeDeps) GetTrackingStatus() (bool, int, string) {
	return true, 7, "https://example.test"
}

func (f *fakeDeps) HasSessionStore() bool {
	if !f.hasStore && f.storeResult == nil && f.storeErr == nil {
		return true
	}
	return f.hasStore
}

func (f *fakeDeps) HandleSessionStore(args persistence.SessionStoreArgs) (json.RawMessage, error) {
	f.stored = args
	if f.storeErr != nil {
		return nil, f.storeErr
	}
	if f.storeResult != nil {
		return f.storeResult, nil
	}
	return json.RawMessage(`{}`), nil
}

func writeSolidPNG(t *testing.T, path string, fill color.RGBA) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	for y := 0; y < 2; y++ {
		for x := 0; x < 2; x++ {
			img.SetRGBA(x, y, fill)
		}
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(file, img); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestDiffBaselineComparesStoredImage(t *testing.T) {
	dir := t.TempDir()
	baselinePath := filepath.Join(dir, "baseline.png")
	currentPath := filepath.Join(dir, "current.png")
	writeSolidPNG(t, baselinePath, color.RGBA{R: 255, A: 255})
	writeSolidPNG(t, currentPath, color.RGBA{R: 255, A: 255})
	metadata, err := json.Marshal(map[string]any{
		"path":     baselinePath,
		"url":      "https://example.test",
		"saved_at": "2026-01-01T00:00:00Z",
		"name":     "home",
	})
	if err != nil {
		t.Fatal(err)
	}
	storeResult, err := json.Marshal(map[string]any{"data": json.RawMessage(metadata)})
	if err != nil {
		t.Fatal(err)
	}
	deps := &fakeDeps{
		hasStore:       true,
		screenshotPath: currentPath,
		storeResult:    storeResult,
	}
	resp := DiffBaseline(
		deps,
		mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`2`)},
		json.RawMessage(`{"baseline":"home","threshold":10}`),
	)
	if !strings.Contains(string(resp.Result), "Visual diff complete") ||
		!strings.Contains(string(resp.Result), `\"pixels_changed\":0`) {
		t.Fatalf("response = %s", resp.Result)
	}
	if deps.stored.Action != "load" || deps.stored.Key != "home" {
		t.Fatalf("load args = %#v", deps.stored)
	}
}

func TestVisualHandlersReportStoreFailures(t *testing.T) {
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`3`)}
	noStore := &fakeDeps{hasStore: false, storeErr: errors.New("disabled")}
	if resp := DiffBaseline(noStore, req, json.RawMessage(`{"baseline":"home"}`)); !strings.Contains(string(resp.Result), "not initialized") {
		t.Fatalf("diff response = %s", resp.Result)
	}
	if resp := ListBaselines(noStore, req, nil); !strings.Contains(string(resp.Result), "not initialized") {
		t.Fatalf("list response = %s", resp.Result)
	}

	failing := &fakeDeps{hasStore: true, storeErr: errors.New("disk unavailable")}
	if resp := ListBaselines(failing, req, nil); !strings.Contains(string(resp.Result), "disk unavailable") {
		t.Fatalf("list response = %s", resp.Result)
	}
}

func TestListBaselinesReturnsStoredPayload(t *testing.T) {
	deps := &fakeDeps{
		hasStore:    true,
		storeResult: json.RawMessage(`{"keys":["home","checkout"]}`),
	}
	resp := ListBaselines(
		deps,
		mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`4`)},
		nil,
	)
	if !strings.Contains(string(resp.Result), "checkout") {
		t.Fatalf("response = %s", resp.Result)
	}
}

func TestSaveBaselinePersistsScreenshotMetadata(t *testing.T) {
	t.Parallel()
	deps := &fakeDeps{}
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	response := SaveBaseline(deps, req, json.RawMessage(`{"name":"home"}`))
	var result mcp.MCPToolResult
	if err := json.Unmarshal(response.Result, &result); err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatal("expected baseline save to succeed")
	}
	if deps.stored.Namespace != "visual_baselines" || deps.stored.Key != "home" {
		t.Fatalf("stored args = %#v", deps.stored)
	}
}
