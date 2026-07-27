// handler.go — Implements visual regression analyze modes.
// Why: Isolates screenshot-baseline and image diff behavior from other inspect analysis paths.
// Docs: docs/features/feature/analyze-tool/index.md

package visual

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/persistence"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
	az "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/analyze"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/analyze/imagediff"
)

type Deps interface {
	CaptureScreenshot(req mcp.JSONRPCRequest) mcp.JSONRPCResponse
	GetTrackingStatus() (bool, int, string)
	HasSessionStore() bool
	HandleSessionStore(args persistence.SessionStoreArgs) (json.RawMessage, error)
}

func SaveBaseline(d Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	parsed, err := az.ParseVisualBaselineArgs(args)
	if err != nil {
		return mcp.Fail(req, mcp.ErrMissingParam, err.Error(), "Add the 'name' parameter for the baseline", mcp.WithParam("name"))
	}

	screenshotResp := d.CaptureScreenshot(req)
	if responseIsError(screenshotResp) {
		return screenshotResp
	}

	screenshotPath := extractScreenshotPath(screenshotResp)
	if screenshotPath == "" {
		return mcp.Fail(req, mcp.ErrExtError, "Screenshot captured but path not available", "Try again or check extension connection")
	}

	now := time.Now()
	_, _, trackedURL := d.GetTrackingStatus()
	metadata := az.BaselineMetadata{
		Path:      screenshotPath,
		URL:       trackedURL,
		SavedAt:   now.Format(time.RFC3339),
		Name:      parsed.Name,
		Timestamp: now.UnixMilli(),
	}
	metadataJSON, _ := json.Marshal(metadata)

	if d.HasSessionStore() {
		storeArgs := persistence.SessionStoreArgs{
			Action:    "save",
			Namespace: "visual_baselines",
			Key:       parsed.Name,
			Data:      metadataJSON,
		}
		if _, err := d.HandleSessionStore(storeArgs); err != nil {
			return mcp.Fail(req, mcp.ErrInvalidParam, "Failed to store baseline: "+err.Error(), "Check session store configuration")
		}
	}

	return mcp.Succeed(req, "Visual baseline saved", map[string]any{
		"status":   "saved",
		"name":     parsed.Name,
		"path":     screenshotPath,
		"url":      trackedURL,
		"saved_at": metadata.SavedAt,
	})
}

func DiffBaseline(d Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	parsed, err := az.ParseVisualDiffArgs(args)
	if err != nil {
		return mcp.Fail(req, mcp.ErrMissingParam, err.Error(), "Add the 'baseline' parameter with the baseline name", mcp.WithParam("baseline"))
	}

	if !d.HasSessionStore() {
		return mcp.Fail(req, mcp.ErrNotInitialized, "Session store not initialized", "Internal error — do not retry")
	}

	loadArgs := persistence.SessionStoreArgs{
		Action:    "load",
		Namespace: "visual_baselines",
		Key:       parsed.Baseline,
	}
	loadResult, err := d.HandleSessionStore(loadArgs)
	if err != nil {
		return mcp.Fail(req, mcp.ErrInvalidParam, "Baseline '"+parsed.Baseline+"' not found: "+err.Error(), "Save a baseline first with analyze(what='visual_baseline', name='"+parsed.Baseline+"')")
	}

	var storeResp struct {
		Data json.RawMessage `json:"data"`
	}
	json.Unmarshal(loadResult, &storeResp)

	var baseline az.BaselineMetadata
	if err := json.Unmarshal(storeResp.Data, &baseline); err != nil {
		return mcp.Fail(req, mcp.ErrInvalidJSON, "Failed to parse baseline metadata: "+err.Error(), "Re-save the baseline")
	}

	screenshotResp := d.CaptureScreenshot(req)
	if responseIsError(screenshotResp) {
		return screenshotResp
	}

	currentPath := extractScreenshotPath(screenshotResp)
	if currentPath == "" {
		return mcp.Fail(req, mcp.ErrExtError, "Current screenshot path not available", "Try again")
	}

	diffResult, err := imagediff.CompareImages(baseline.Path, currentPath, parsed.Threshold)
	if err != nil {
		return mcp.Fail(req, mcp.ErrExtError, "Image comparison failed: "+err.Error(), "Check that baseline image exists at: "+baseline.Path)
	}

	var diffPath string
	if diffResult.PixelsChanged > 0 {
		screenshotsDir, err := state.ScreenshotsDir()
		if err == nil {
			diffPath = filepath.Join(screenshotsDir, fmt.Sprintf("diff-%s-%d.png", parsed.Baseline, time.Now().UnixMilli()))
			baselineImg, err1 := imagediff.LoadImage(baseline.Path)
			currentImg, err2 := imagediff.LoadImage(currentPath)
			if err1 == nil && err2 == nil {
				changedGrid := imagediff.RebuildChangedGrid(baselineImg, currentImg, parsed.Threshold)
				imagediff.WriteDiffImage(baselineImg, currentImg, changedGrid, diffPath)
			}
		}
	}

	response := map[string]any{
		"diff_percentage":  diffResult.DiffPercentage,
		"pixels_changed":   diffResult.PixelsChanged,
		"pixels_total":     diffResult.PixelsTotal,
		"dimensions_match": diffResult.DimensionsMatch,
		"verdict":          diffResult.Verdict,
		"threshold":        diffResult.Threshold,
		"regions":          diffResult.Regions,
		"baseline": map[string]any{
			"path":     baseline.Path,
			"url":      baseline.URL,
			"saved_at": baseline.SavedAt,
		},
		"current_path": currentPath,
	}

	if diffPath != "" {
		response["diff_path"] = diffPath
	}
	if diffResult.DimensionDelta != nil {
		response["dimension_delta"] = map[string]int{
			"width":  diffResult.DimensionDelta[0],
			"height": diffResult.DimensionDelta[1],
		}
	}

	return mcp.Succeed(req, "Visual diff complete", response)
}

func ListBaselines(d Deps, req mcp.JSONRPCRequest, _ json.RawMessage) mcp.JSONRPCResponse {
	if !d.HasSessionStore() {
		return mcp.Fail(req, mcp.ErrNotInitialized, "Session store not initialized", "Internal error — do not retry")
	}

	listArgs := persistence.SessionStoreArgs{
		Action:    "list",
		Namespace: "visual_baselines",
	}
	listResult, err := d.HandleSessionStore(listArgs)
	if err != nil {
		return mcp.Fail(req, mcp.ErrInvalidParam, "Failed to list baselines: "+err.Error(), "Check session store")
	}

	var listData map[string]any
	if err := json.Unmarshal(listResult, &listData); err != nil {
		listData = map[string]any{"raw": string(listResult)}
	}
	return mcp.Succeed(req, "Visual baselines", listData)
}

// extractScreenshotPath extracts the file path from a screenshot response.
func extractScreenshotPath(resp mcp.JSONRPCResponse) string {
	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil || len(result.Content) == 0 {
		return ""
	}
	text := result.Content[0].Text

	idx := 0
	for i, ch := range text {
		if ch == '{' {
			idx = i
			break
		}
	}

	var data map[string]any
	if err := json.Unmarshal([]byte(text[idx:]), &data); err != nil {
		return ""
	}
	if p, ok := data["path"].(string); ok && p != "" {
		return p
	}
	if filename, ok := data["filename"].(string); ok && filename != "" {
		if dir, err := state.ScreenshotsDir(); err == nil {
			return filepath.Join(dir, filename)
		}
	}
	return ""
}

func responseIsError(resp mcp.JSONRPCResponse) bool {
	var result mcp.MCPToolResult
	return json.Unmarshal(resp.Result, &result) != nil || result.IsError
}
