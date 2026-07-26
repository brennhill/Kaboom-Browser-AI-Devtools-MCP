// Purpose: Implements visual regression analyze modes (visual_baseline, visual_diff, visual_baselines).
// Why: Isolates screenshot-baseline and image diff behavior from other inspect analysis paths.
// Docs: docs/features/feature/analyze-tool/index.md

package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/persistence"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
	az "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/analyze"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/analyze/imagediff"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/observe"
)

// ============================================
// Visual Regression (#82)
// ============================================

func (h *ToolHandler) toolVisualBaseline(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	parsed, err := az.ParseVisualBaselineArgs(args)
	if err != nil {
		return fail(req, ErrMissingParam, err.Error(), "Add the 'name' parameter for the baseline", withParam("name"))
	}

	screenshotResp := observe.GetScreenshot(h, req, json.RawMessage(`{}`))
	if isErrorResponse(screenshotResp) {
		return screenshotResp
	}

	screenshotPath := extractScreenshotPath(screenshotResp)
	if screenshotPath == "" {
		return fail(req, ErrExtError, "Screenshot captured but path not available", "Try again or check extension connection")
	}

	now := time.Now()
	_, _, trackedURL := h.capture.GetTrackingStatus()
	metadata := az.BaselineMetadata{
		Path:      screenshotPath,
		URL:       trackedURL,
		SavedAt:   now.Format(time.RFC3339),
		Name:      parsed.Name,
		Timestamp: now.UnixMilli(),
	}
	metadataJSON, _ := json.Marshal(metadata)

	if h.sessionStoreImpl != nil {
		storeArgs := persistence.SessionStoreArgs{
			Action:    "save",
			Namespace: "visual_baselines",
			Key:       parsed.Name,
			Data:      metadataJSON,
		}
		if _, err := h.sessionStoreImpl.HandleSessionStore(storeArgs); err != nil {
			return fail(req, ErrInvalidParam, "Failed to store baseline: "+err.Error(), "Check session store configuration")
		}
	}

	return succeed(req, "Visual baseline saved", map[string]any{
		"status":   "saved",
		"name":     parsed.Name,
		"path":     screenshotPath,
		"url":      trackedURL,
		"saved_at": metadata.SavedAt,
	})
}

func (h *ToolHandler) toolVisualDiff(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	parsed, err := az.ParseVisualDiffArgs(args)
	if err != nil {
		return fail(req, ErrMissingParam, err.Error(), "Add the 'baseline' parameter with the baseline name", withParam("baseline"))
	}

	if resp, blocked := h.requireSessionStore(req); blocked {
		return resp
	}

	loadArgs := persistence.SessionStoreArgs{
		Action:    "load",
		Namespace: "visual_baselines",
		Key:       parsed.Baseline,
	}
	loadResult, err := h.sessionStoreImpl.HandleSessionStore(loadArgs)
	if err != nil {
		return fail(req, ErrInvalidParam, "Baseline '"+parsed.Baseline+"' not found: "+err.Error(), "Save a baseline first with analyze(what='visual_baseline', name='"+parsed.Baseline+"')")
	}

	var storeResp struct {
		Data json.RawMessage `json:"data"`
	}
	json.Unmarshal(loadResult, &storeResp)

	var baseline az.BaselineMetadata
	if err := json.Unmarshal(storeResp.Data, &baseline); err != nil {
		return fail(req, ErrInvalidJSON, "Failed to parse baseline metadata: "+err.Error(), "Re-save the baseline")
	}

	screenshotResp := observe.GetScreenshot(h, req, json.RawMessage(`{}`))
	if isErrorResponse(screenshotResp) {
		return screenshotResp
	}

	currentPath := extractScreenshotPath(screenshotResp)
	if currentPath == "" {
		return fail(req, ErrExtError, "Current screenshot path not available", "Try again")
	}

	diffResult, err := imagediff.CompareImages(baseline.Path, currentPath, parsed.Threshold)
	if err != nil {
		return fail(req, ErrExtError, "Image comparison failed: "+err.Error(), "Check that baseline image exists at: "+baseline.Path)
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

	return succeed(req, "Visual diff complete", response)
}

func (h *ToolHandler) toolListVisualBaselines(req JSONRPCRequest, _ json.RawMessage) JSONRPCResponse {
	if resp, blocked := h.requireSessionStore(req); blocked {
		return resp
	}

	listArgs := persistence.SessionStoreArgs{
		Action:    "list",
		Namespace: "visual_baselines",
	}
	listResult, err := h.sessionStoreImpl.HandleSessionStore(listArgs)
	if err != nil {
		return fail(req, ErrInvalidParam, "Failed to list baselines: "+err.Error(), "Check session store")
	}

	var listData map[string]any
	if err := json.Unmarshal(listResult, &listData); err != nil {
		listData = map[string]any{"raw": string(listResult)}
	}
	return succeed(req, "Visual baselines", listData)
}

// extractScreenshotPath extracts the file path from a screenshot response.
func extractScreenshotPath(resp JSONRPCResponse) string {
	var result MCPToolResult
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
