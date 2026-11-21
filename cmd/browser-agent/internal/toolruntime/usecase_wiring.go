// usecase_wiring.go — Composes the internal/app use cases (screenshot, pageinfo, pagestorage, a11yaudit).
// Why: Every use case is built here from the narrow owners it needs (browser-command owner, tracking
// state, waterfall, logs) rather than the whole Capture; adapters receive the composed services.
// Docs: docs/features/feature/observe/index.md

package toolruntime

import (
	"context"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/app/a11yaudit"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/app/pageinfo"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/app/pagestorage"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/app/screenshot"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/healthreader"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/syncruntime"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/telemetrystore"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
	observepage "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/observe/page"
)

// trackingEnabled reports the tracked-tab gate; a handler without a capture
// (degraded composition) reports no tracked tab, so use cases fail with
// not_tracking before they would need the command owner.
func trackingEnabled(h *ToolHandler) func() bool {
	if h.capture == nil {
		return func() bool { return false }
	}
	return func() bool {
		enabled, _, _ := h.capture.Extension().GetTrackingStatus()
		return enabled
	}
}

// extensionOf returns the extension connection owner, or nil without a capture.
func extensionOf(h *ToolHandler) *syncruntime.Runtime {
	if h.capture == nil {
		return nil
	}
	return h.capture.Extension()
}

// healthOwners returns the health/doctor state owners, or the zero value
// (degraded composition) without a capture.
func healthOwners(h *ToolHandler) healthreader.Owners {
	if h.capture == nil {
		return healthreader.Owners{}
	}
	return h.capture.HealthOwners()
}

// telemetryOf returns the telemetry store, or nil without a capture.
func telemetryOf(h *ToolHandler) *telemetrystore.Store {
	if h.capture == nil {
		return nil
	}
	return h.capture.Telemetry()
}

// queryRunner returns the browser-command owner, or nil without a capture.
func queryRunner(h *ToolHandler) queries.QueryRunner {
	if h.capture == nil {
		return nil
	}
	return h.capture.Commands()
}

// newScreenshotService binds the screenshot use case to the browser-command owner.
func newScreenshotService(h *ToolHandler) *screenshot.Service {
	return screenshot.New(screenshot.Deps{Commands: queryRunner(h), TrackingEnabled: trackingEnabled(h)})
}

// newStorageService binds the storage/IndexedDB use case to the command owner.
func newStorageService(h *ToolHandler) *pagestorage.Service {
	return pagestorage.New(pagestorage.Deps{Commands: queryRunner(h), TrackingEnabled: trackingEnabled(h)})
}

// newA11yService binds the accessibility audit to the async a11y query.
func newA11yService(h *ToolHandler) *a11yaudit.Service {
	return a11yaudit.New(a11yaudit.Deps{TrackingEnabled: trackingEnabled(h), Execute: h.asyncCommands.ExecuteA11yQuery})
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
