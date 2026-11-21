// screenshot_wiring.go — Composes the screenshot and pageinfo use cases; adapts screenshots for analyze visual handlers.
// Why: Every screenshot caller (observe, interact include_screenshot, analyze visual_*) shares one
// use-case instance shape built from the command owner and tracking state, not the whole Capture;
// observe(what="page") and interact page context share one pageinfo projection the same way.
// Docs: docs/features/feature/observe/index.md

package toolruntime

import (
	"context"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/app/pageinfo"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/app/screenshot"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	observepage "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/observe/page"
)

// newScreenshotService binds the screenshot use case to the browser-command owner.
// A handler without a capture (degraded composition) reports no tracked tab, so
// the use case fails with not_tracking before it would need Commands.
func newScreenshotService(h *ToolHandler) *screenshot.Service {
	deps := screenshot.Deps{TrackingEnabled: func() bool { return false }}
	if h.capture != nil {
		deps.Commands = h.capture.Commands()
		deps.TrackingEnabled = func() bool {
			enabled, _, _ := h.capture.Extension().GetTrackingStatus()
			return enabled
		}
	}
	return screenshot.New(deps)
}

// newPageInfoService binds the pageinfo use case to the extension state, the
// waterfall and the log buffer. A handler without a capture reports an
// untracked page.
func newPageInfoService(h *ToolHandler) *pageinfo.Service {
	deps := pageinfo.Deps{LogEntries: h.state.Logs.EntriesWithAddedAt}
	if h.capture != nil {
		deps.Extension = h.capture.Extension()
		deps.LastWaterfallPageURL = h.capture.Telemetry().NetworkWaterfall().LastPageURL
	}
	return pageinfo.New(deps)
}

func (d visualAnalyzeDeps) CaptureScreenshot(ctx context.Context) (screenshot.Result, error) {
	return newScreenshotService(d.h).Capture(ctx, screenshot.Options{})
}

func (d visualAnalyzeDeps) ScreenshotFailure(req mcp.JSONRPCRequest, err error) mcp.JSONRPCResponse {
	return observepage.ScreenshotFailure(req, err, d.h.Guards.DiagnosticHintString())
}
