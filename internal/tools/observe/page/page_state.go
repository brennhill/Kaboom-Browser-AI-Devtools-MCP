// Purpose: MCP adapters for the live-page modes — page, storage, indexeddb — plus analyze's accessibility audit.
// Why: The workflows live in internal/app/{pageinfo,pagestorage,a11yaudit}; this file parses arguments,
// maps typed failures to canonical MCP errors and adds response metadata.
// Docs: docs/features/feature/observe/index.md

package page

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/app/a11yaudit"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/app/pagestorage"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/syncruntime"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/observe/core"
)

// GetPageInfo adapts the pageinfo use case to observe(what="page").
func GetPageInfo(deps core.Deps, req mcp.JSONRPCRequest, _ json.RawMessage) mcp.JSONRPCResponse {
	info := deps.PageInfo.Read()
	result := map[string]any{
		"url":                     info.URL,
		"title":                   info.Title,
		"tracked":                 info.Tracked,
		"csp_restricted":          info.CSPRestricted,
		"csp_level":               info.CSPLevel,
		"tab_status":              info.TabStatus,
		"page_ready_for_commands": info.ReadyForCommands,
		"metadata":                core.BuildResponseMetadata(deps.Extension, time.Now()),
	}
	if info.TabID > 0 {
		result["tab_id"] = info.TabID
	}
	if info.TabActiveKnown {
		result["is_active"] = info.TabActive
	}

	// Include blocked_actions/blocked_reason when CSP restricts — omit entirely
	// when CSP is clear to avoid wasting tokens on normal pages. (#262)
	if info.CSPRestricted {
		if actions, reason := syncruntime.CSPBlockedActions(info.CSPLevel); actions != nil {
			result["blocked_actions"] = actions
			result["blocked_reason"] = reason
		}
	}

	return mcp.Succeed(req, "Page info", result)
}

// storageParams holds parsed parameters for storage queries.
type storageParams = pagestorage.StorageOptions

func parseStorageParams(args json.RawMessage) storageParams {
	p := storageParams{Summary: true}
	if len(args) == 0 {
		return p
	}
	var raw struct {
		Summary     *bool  `json:"summary"`
		StorageType string `json:"storage_type"`
		Key         string `json:"key"`
	}
	if json.Unmarshal(args, &raw) == nil {
		if raw.Summary != nil {
			p.Summary = *raw.Summary
		}
		p.StorageType = raw.StorageType
		p.Key = raw.Key
	}
	return p
}

// notTrackingFailure is the shared "no tab is being tracked" response for live-page modes.
func notTrackingFailure(deps core.Deps, req mcp.JSONRPCRequest, what string) mcp.JSONRPCResponse {
	return mcp.Fail(req,
		mcp.ErrNoData,
		"No tab is being tracked. Open the Kaboom extension popup and click 'Track This Tab'.",
		"Track a tab first, then call observe with what='"+what+"'.",
		mcp.WithHint(deps.DiagnosticHintString()),
	)
}

// GetStorage adapts the pagestorage use case to observe(what="storage").
func GetStorage(deps core.Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	fields, err := deps.Storage.Storage(context.Background(), parseStorageParams(args))
	if err != nil {
		return storageFailure(deps, req, err)
	}
	fields["metadata"] = core.BuildResponseMetadata(deps.Extension, time.Now())
	return mcp.Succeed(req, "Browser storage", fields)
}

func storageFailure(deps core.Deps, req mcp.JSONRPCRequest, err error) mcp.JSONRPCResponse {
	var failure *pagestorage.Failure
	if !errors.As(err, &failure) {
		return mcp.Fail(req, mcp.ErrExtError, "Storage capture failed: "+err.Error(), "Check that the tab is accessible.")
	}
	switch failure.Kind {
	case pagestorage.KindNotTracking:
		return notTrackingFailure(deps, req, "storage")
	case pagestorage.KindQueueFull:
		return queueFullResponse(req, failure.Detail)
	case pagestorage.KindTimeout:
		return mcp.Fail(req, mcp.ErrExtTimeout, "Storage capture timeout: "+failure.Detail,
			"Ensure the extension is connected and the page has loaded.", mcp.WithHint(deps.DiagnosticHintString()))
	case pagestorage.KindInvalidResult:
		return mcp.Fail(req, mcp.ErrInvalidJSON, "Failed to parse storage result: "+failure.Detail,
			"Check extension logs for errors")
	default:
		return mcp.Fail(req, mcp.ErrExtError, "Storage capture failed: "+failure.Detail,
			"Check that the tab is accessible.", mcp.WithHint(deps.DiagnosticHintString()))
	}
}

// GetIndexedDB adapts the pagestorage use case to observe(what="indexeddb").
func GetIndexedDB(deps core.Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		Database string `json:"database"`
		Store    string `json:"store"`
		Limit    int    `json:"limit"`
	}
	mcp.LenientUnmarshal(args, &params)

	if params.Database == "" {
		return mcp.Fail(req,
			mcp.ErrMissingParam,
			"Required parameter 'database' is missing for observe(what='indexeddb')",
			"Add the 'database' parameter and call again.",
			mcp.WithParam("database"),
		)
	}
	if params.Store == "" {
		return mcp.Fail(req,
			mcp.ErrMissingParam,
			"Required parameter 'store' is missing for observe(what='indexeddb')",
			"Add the 'store' parameter and call again.",
			mcp.WithParam("store"),
		)
	}
	params.Limit = core.ClampLimit(params.Limit, 100)

	fields, err := deps.Storage.IndexedDB(params.Database, params.Store, params.Limit)
	if err != nil {
		var failure *pagestorage.Failure
		if errors.As(err, &failure) && failure.Kind == pagestorage.KindNotTracking {
			return notTrackingFailure(deps, req, "indexeddb")
		}
		detail := err.Error()
		if failure != nil {
			detail = failure.Detail
		}
		return mcp.Fail(req,
			mcp.ErrExtError,
			"IndexedDB inspection failed: "+detail,
			"Ensure the tab is accessible and the database/store names are correct.",
			mcp.WithHint(deps.DiagnosticHintString()),
		)
	}
	fields["database"] = params.Database
	fields["store"] = params.Store
	fields["limit"] = params.Limit
	fields["metadata"] = core.BuildResponseMetadata(deps.Extension, time.Now())
	return mcp.Succeed(req, "IndexedDB entries", fields)
}

// RunA11yAudit adapts the a11yaudit use case to analyze/observe accessibility.
func RunA11yAudit(deps core.Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		Selector     string   `json:"selector"`
		Scope        string   `json:"scope"`
		Tags         []string `json:"tags"`
		ForceRefresh bool     `json:"force_refresh"`
		Frame        any      `json:"frame"`
		Summary      bool     `json:"summary"`
	}
	mcp.LenientUnmarshal(args, &params)
	if params.Scope == "" && params.Selector != "" {
		params.Scope = params.Selector
	}

	result, err := deps.A11y.Audit(a11yaudit.Options{
		Scope: params.Scope, Tags: params.Tags, Frame: params.Frame,
		ForceRefresh: params.ForceRefresh, Summary: params.Summary,
	})
	if err != nil {
		return A11yFailure(req, err, deps.DiagnosticHintString())
	}
	if result.PartialError != "" {
		// Issue #276: partial results with an error field instead of a hard failure.
		return mcp.Succeed(req, "A11y audit (partial — "+result.PartialError+")", result.Data)
	}
	return mcp.Succeed(req, "A11y audit", result.Data)
}

// A11yFailure maps an a11yaudit failure to its canonical MCP error.
func A11yFailure(req mcp.JSONRPCRequest, err error, hint string) mcp.JSONRPCResponse {
	var failure *a11yaudit.Failure
	if errors.As(err, &failure) && failure.Kind == a11yaudit.KindNotTracking {
		return mcp.Fail(req, mcp.ErrNoData, "No tab is being tracked. Open the Kaboom extension popup and click 'Track This Tab' on the page you want to monitor. Check observe with what='pilot' for extension status.", "", mcp.WithHint(hint))
	}
	detail := err.Error()
	if failure != nil {
		detail = failure.Detail
	}
	return mcp.Fail(req, mcp.ErrInvalidJSON, "Failed to parse a11y result: "+detail, "Check extension logs for errors")
}
