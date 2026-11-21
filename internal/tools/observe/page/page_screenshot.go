// Purpose: MCP adapter for observe(what='screenshot'): parse arguments, run the screenshot use case, assemble JSON-RPC.
// Why: The workflow lives in internal/app/screenshot; this file keeps only MCP parsing, error-code mapping
// and the text + image content blocks, so other MCP callers can reuse the same mapping.
// Docs: docs/features/feature/observe/index.md

package page

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/app/screenshot"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/observe/core"
)

type screenshotArgs struct {
	Format        string `json:"format,omitempty"`
	Quality       int    `json:"quality,omitempty"`
	FullPage      bool   `json:"full_page,omitempty"`
	Selector      string `json:"selector,omitempty"`
	WaitForStable bool   `json:"wait_for_stable,omitempty"`
	SaveTo        string `json:"save_to,omitempty"`
}

// GetScreenshot captures a screenshot of the current page via the extension.
func GetScreenshot(deps core.Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var parsed screenshotArgs
	mcp.LenientUnmarshal(args, &parsed)
	result, err := deps.Screenshots.Capture(context.Background(), screenshot.Options(parsed))
	if err != nil {
		return ScreenshotFailure(req, err, deps.DiagnosticHintString())
	}
	return screenshotSuccess(req, result)
}

// screenshotSuccess renders a capture as a text block plus an inline image block.
// data_url is carried only in the image block, never duplicated in the text.
func screenshotSuccess(req mcp.JSONRPCRequest, result screenshot.Result) mcp.JSONRPCResponse {
	base64Data, mimeType := result.Image()
	return mcp.AppendImageToResponse(mcp.Succeed(req, "Screenshot captured", result.Fields), base64Data, mimeType)
}

// ScreenshotFailure maps a use-case failure to the canonical MCP error response.
func ScreenshotFailure(req mcp.JSONRPCRequest, err error, hint string) mcp.JSONRPCResponse {
	var failure *screenshot.Failure
	if !errors.As(err, &failure) {
		return mcp.Fail(req, mcp.ErrExtError, "Screenshot capture failed: "+err.Error(), "Retry the screenshot.", mcp.WithHint(hint))
	}
	switch failure.Kind {
	case screenshot.KindNotTracking:
		return mcp.Fail(req, mcp.ErrNoData, "No tab is being tracked. Open the Kaboom extension popup and click 'Track This Tab' on the page you want to monitor. Check observe with what='pilot' for extension status.", "", mcp.WithHint(hint))
	case screenshot.KindInvalidFormat:
		return mcp.Fail(req, mcp.ErrInvalidParam, "Invalid screenshot format: "+failure.Detail, "Use 'png' or 'jpeg'", mcp.WithParam("format"))
	case screenshot.KindInvalidQuality:
		return mcp.Fail(req, mcp.ErrInvalidParam, "Invalid quality: "+failure.Detail+" (must be 1-100)", "Use a value between 1 and 100", mcp.WithParam("quality"))
	case screenshot.KindQueueFull:
		return queueFullResponse(req, failure.Detail)
	case screenshot.KindTimeout:
		return mcp.Fail(req, mcp.ErrExtTimeout, "Screenshot capture timeout: "+failure.Detail, "Ensure the extension is connected and the page has loaded. Try refreshing the page, then retry.", mcp.WithHint(hint))
	case screenshot.KindMalformedResult:
		return mcp.Fail(req, mcp.ErrInvalidJSON, "Failed to parse screenshot result: "+failure.Detail, "Check extension logs for errors")
	default:
		return mcp.Fail(req, mcp.ErrExtError, "Screenshot capture failed: "+failure.Detail, "Check that the tab is visible and accessible. The extension reported an error.", mcp.WithHint(hint))
	}
}

// queueFullResponse is the canonical command-queue-full failure for live-page modes.
func queueFullResponse(req mcp.JSONRPCRequest, detail string) mcp.JSONRPCResponse {
	return mcp.Fail(req, mcp.ErrQueueFull, "Command queue full: "+detail,
		"Wait for in-flight commands to complete, then retry.")
}
