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
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
)

type fakeDeps struct {
	stored         persistence.SessionStoreArgs
	screenshotPath string
	hasStore       bool
	storeResult    json.RawMessage
	storeErr       error
	screenshotResp *mcp.JSONRPCResponse
}

func (f *fakeDeps) CaptureScreenshot(req mcp.JSONRPCRequest) mcp.JSONRPCResponse {
	if f.screenshotResp != nil {
		return *f.screenshotResp
	}
	path := f.screenshotPath
	if path == "" {
		path = "/tmp/current.png"
	}
	return mcp.Succeed(req, "Screenshot captured", map[string]any{"path": path})
}

func TestVisualHandlersSurfaceCaptureAndArtifactFailures(t *testing.T) {
	req := mcp.JSONRPCRequest{JSONRPC: mcp.JSONRPCVersion, ID: 9}
	if resp := SaveBaseline(&fakeDeps{}, req, json.RawMessage(`{}`)); !strings.Contains(string(resp.Result), "missing_param") {
		t.Fatalf("missing-name response = %s", resp.Result)
	}
	captureFailure := mcp.Fail(req, mcp.ErrExtError, "capture unavailable", "retry")
	if resp := SaveBaseline(&fakeDeps{screenshotResp: &captureFailure}, req, json.RawMessage(`{"name":"home"}`)); !strings.Contains(string(resp.Result), "capture unavailable") {
		t.Fatalf("capture-failure response = %s", resp.Result)
	}
	missingPath := mcp.Succeed(req, "Screenshot captured", map[string]any{"status": "ok"})
	if resp := SaveBaseline(&fakeDeps{screenshotResp: &missingPath}, req, json.RawMessage(`{"name":"home"}`)); !strings.Contains(string(resp.Result), "path not available") {
		t.Fatalf("missing-path response = %s", resp.Result)
	}
	if resp := SaveBaseline(&fakeDeps{hasStore: true, storeErr: errors.New("disk full")}, req, json.RawMessage(`{"name":"home"}`)); !strings.Contains(string(resp.Result), "disk full") {
		t.Fatalf("store-failure response = %s", resp.Result)
	}

	if resp := DiffBaseline(&fakeDeps{}, req, json.RawMessage(`{}`)); !strings.Contains(string(resp.Result), "missing_param") {
		t.Fatalf("missing-baseline response = %s", resp.Result)
	}
	invalidMetadata := &fakeDeps{hasStore: true, storeResult: json.RawMessage(`{"data":"not-json"}`)}
	if resp := DiffBaseline(invalidMetadata, req, json.RawMessage(`{"baseline":"home"}`)); !strings.Contains(string(resp.Result), "parse baseline metadata") {
		t.Fatalf("invalid-metadata response = %s", resp.Result)
	}
	metadata, _ := json.Marshal(map[string]any{"path": filepath.Join(t.TempDir(), "missing.png")})
	storeResult, _ := json.Marshal(map[string]any{"data": json.RawMessage(metadata)})
	missingImage := &fakeDeps{hasStore: true, screenshotPath: filepath.Join(t.TempDir(), "current.png"), storeResult: storeResult}
	if resp := DiffBaseline(missingImage, req, json.RawMessage(`{"baseline":"home"}`)); !strings.Contains(string(resp.Result), "Image comparison failed") {
		t.Fatalf("missing-image response = %s", resp.Result)
	}

	invalidList := &fakeDeps{hasStore: true, storeResult: json.RawMessage(`not-json`)}
	if resp := ListBaselines(invalidList, req, nil); !strings.Contains(string(resp.Result), "not-json") {
		t.Fatalf("invalid-list fallback = %s", resp.Result)
	}
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

func TestDiffBaselineWritesChangedImageAndDimensionDelta(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(state.StateDirEnv, dir)
	baselinePath := filepath.Join(dir, "baseline.png")
	currentPath := filepath.Join(dir, "current.png")
	writeSolidPNG(t, baselinePath, color.RGBA{R: 255, A: 255})
	img := image.NewRGBA(image.Rect(0, 0, 3, 2))
	file, err := os.Create(currentPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(file, img); err != nil {
		t.Fatal(err)
	}
	_ = file.Close()
	metadata, _ := json.Marshal(map[string]any{"path": baselinePath, "url": "https://example.test"})
	storeResult, _ := json.Marshal(map[string]any{"data": json.RawMessage(metadata)})
	deps := &fakeDeps{hasStore: true, screenshotPath: currentPath, storeResult: storeResult}
	resp := DiffBaseline(deps, mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 5}, json.RawMessage(`{"baseline":"changed","threshold":0}`))
	if !strings.Contains(string(resp.Result), `\"dimensions_match\":false`) ||
		!strings.Contains(string(resp.Result), `\"dimension_delta\"`) || !strings.Contains(string(resp.Result), `\"diff_path\"`) {
		t.Fatalf("changed diff response = %s", resp.Result)
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
