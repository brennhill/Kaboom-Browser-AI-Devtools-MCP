// handlers.go — Generates annotation-derived artifacts — visual_test (Playwright), annotation_report (Markdown), and annotation_issues (JSON).
// Why: Converts draw-mode annotations into actionable test scripts and structured issue reports.
// Docs: docs/features/feature/annotated-screenshots/index.md

package annotations

import (
	"encoding/json"
	"fmt"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/annotation"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

// Deps is everything the annotation artifact handlers need from the host server.
// It is deliberately narrower than toolgenerate.Deps: these handlers read the
// annotation store and nothing else. *ToolHandler in cmd/browser-agent/ satisfies
// it, as does any toolgenerate.Deps value.
type Deps interface {
	// GetAnnotationStore returns the annotation store.
	GetAnnotationStore() *annotation.Store
}

// HandleVisualTest generates a Playwright test from annotation session data.
func HandleVisualTest(d Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		TestName     string `json:"test_name"`
		AnnotSession string `json:"annot_session"`
	}
	if len(args) > 0 {
		mcp.LenientUnmarshal(args, &params)
	}

	pages, noDataResp, noData := resolveAnnotationPages(d, req, params.AnnotSession)
	if noData {
		return noDataResp
	}

	testName := params.TestName
	if testName == "" {
		testName = "visual review annotations"
	}

	script := GeneratePlaywrightFromAnnotations(testName, pages, d.GetAnnotationStore())
	return mcp.SucceedText(req, script)
}

// HandleAnnotationReport generates a Markdown report from annotation session data.
func HandleAnnotationReport(d Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		AnnotSession string `json:"annot_session"`
	}
	if len(args) > 0 {
		mcp.LenientUnmarshal(args, &params)
	}

	pages, noDataResp, noData := resolveAnnotationPages(d, req, params.AnnotSession)
	if noData {
		return noDataResp
	}

	report := GenerateMarkdownReport(pages, d.GetAnnotationStore())
	return mcp.SucceedText(req, report)
}

// HandleAnnotationIssues generates a structured JSON issue list from annotations.
func HandleAnnotationIssues(d Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		AnnotSession string `json:"annot_session"`
	}
	if len(args) > 0 {
		mcp.LenientUnmarshal(args, &params)
	}

	pages, noDataResp, noData := resolveAnnotationPages(d, req, params.AnnotSession)
	if noData {
		return noDataResp
	}

	issues := BuildIssueList(pages, d.GetAnnotationStore())
	result := map[string]any{
		"issues":      issues,
		"total_count": len(issues),
		"page_count":  len(pages),
	}

	summary := fmt.Sprintf("Annotation issues (%d issues across %d pages)", len(issues), len(pages))
	return mcp.Succeed(req, summary, result)
}

func resolveAnnotationPages(d Deps, req mcp.JSONRPCRequest, sessionName string) ([]*annotation.Session, mcp.JSONRPCResponse, bool) {
	pages, err := collectAnnotationPages(d, sessionName)
	if err == "" {
		return pages, mcp.JSONRPCResponse{}, false
	}
	return nil, mcp.Succeed(req, "No annotations", map[string]any{
		"status":  "no_data",
		"message": err,
	}), true
}

// collectAnnotationPages gathers annotation pages from either a named or anonymous session.
// Returns (pages, errorMessage). errorMessage is empty on success.
func collectAnnotationPages(d Deps, sessionName string) ([]*annotation.Session, string) {
	store := d.GetAnnotationStore()
	if sessionName != "" {
		ns := store.GetNamedSession(sessionName)
		if ns == nil || len(ns.Pages) == 0 {
			return nil, "No annotations found in session '" + sessionName + "'. Use interact({action: 'draw_mode_start', annot_session: '" + sessionName + "'}) to create annotations."
		}
		return ns.Pages, ""
	}

	session := store.GetLatestSession()
	if session == nil || len(session.Annotations) == 0 {
		return nil, "No annotation session found. Use interact({action: 'draw_mode_start'}) to create annotations."
	}
	return []*annotation.Session{session}, ""
}
