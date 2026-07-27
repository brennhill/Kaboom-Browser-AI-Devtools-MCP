// Purpose: Handles analyze annotation modes for session reads and annotation detail.
// Why: Keeps annotation retrieval and detail formatting separate from draw-session file I/O.
// Docs: docs/features/feature/annotated-screenshots/index.md

package annotationanalysis

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolanalyze"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolresp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/annotation"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
)

// Handler owns annotation analysis over the shared annotation and command stores.
type Handler struct {
	annotationStore *annotation.Store
	capture         *capture.Store
	formatCommand   func(mcp.JSONRPCRequest, queries.CommandResult, string) mcp.JSONRPCResponse
	logEntries      func() []mcp.LogEntry
}

// New constructs annotation analysis with explicit runtime dependencies.
func New(
	annotationStore *annotation.Store,
	captureStore *capture.Store,
	formatCommand func(mcp.JSONRPCRequest, queries.CommandResult, string) mcp.JSONRPCResponse,
	logEntries func() []mcp.LogEntry,
) *Handler {
	return &Handler{
		annotationStore: annotationStore,
		capture:         captureStore,
		formatCommand:   formatCommand,
		logEntries:      logEntries,
	}
}

// annotationWaitCommandTTL is how long pending annotation commands remain active.
const annotationWaitCommandTTL = 10 * time.Minute

// annotationBlockingWaitDefault is the initial synchronous wait budget for annotations(background:false).
// If annotations arrive in this window, return them directly without requiring polling.
const annotationBlockingWaitDefault = 15 * time.Second

// annotationErrorCorrelationWindow is the time window around an annotation's timestamp
// in which console errors are considered correlated.
const annotationErrorCorrelationWindow = 5 * time.Second

// annotationBlockingWaitMax caps caller-provided timeout_ms for wait=true annotation calls.
const annotationBlockingWaitMax = 10 * time.Minute

// toolGetAnnotations returns latest annotation session or a named multi-page session.
func (h *Handler) GetAnnotations(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		Wait         *bool  `json:"wait"`
		Background   *bool  `json:"background"`
		AnnotSession string `json:"annot_session"`
		Operation    string `json:"operation"`
		Correlation  string `json:"correlation_id"`
		TimeoutMs    int    `json:"timeout_ms"`
		URL          string `json:"url"`
		URLPattern   string `json:"url_pattern"`
	}
	if len(args) > 0 {
		mcp.LenientUnmarshal(args, &params)
	}
	// Canonical param is "background" (false = block). "wait" is a quiet alias.
	// Default to wait=true when both background and wait are omitted.
	waitValue := true // default: blocking
	if params.Background != nil {
		waitValue = !*params.Background
	} else if params.Wait != nil {
		waitValue = *params.Wait
	}

	urlFilter, filterResp, hasFilterErr := resolveAnnotationURLFilter(req, params.URL, params.URLPattern)
	if hasFilterErr {
		return filterResp
	}

	operation := strings.ToLower(strings.TrimSpace(params.Operation))
	if operation != "" {
		if operation != "flush" {
			return mcp.Fail(req, mcp.ErrInvalidParam, "Invalid annotations operation: "+params.Operation, "Use operation='flush' for annotation waiter recovery.", mcp.WithParam("operation"), mcp.WithHint("flush"))
		}
		return h.toolFlushAnnotations(req, params.Correlation, urlFilter)
	}

	waitTimeout := annotationBlockingWaitDuration(params.TimeoutMs)
	if params.AnnotSession != "" {
		return h.getNamedAnnotations(req, params.AnnotSession, waitValue, waitTimeout, urlFilter)
	}
	return h.getAnonymousAnnotations(req, waitValue, waitTimeout, urlFilter)
}

func annotationBlockingWaitDuration(timeoutMs int) time.Duration {
	if timeoutMs <= 0 {
		return annotationBlockingWaitDefault
	}
	waitDuration := time.Duration(timeoutMs) * time.Millisecond
	if waitDuration > annotationBlockingWaitMax {
		return annotationBlockingWaitMax
	}
	return waitDuration
}

func (h *Handler) getAnonymousAnnotations(req mcp.JSONRPCRequest, wait bool, waitTimeout time.Duration, urlFilter string) mcp.JSONRPCResponse {
	if wait {
		if session := h.annotationStore.GetLatestSessionSinceDraw(); session != nil {
			return h.formatAnnotationSession(req, session, urlFilter)
		}

		if session, _ := h.annotationStore.WaitForSession(waitTimeout); session != nil {
			return h.formatAnnotationSession(req, session, urlFilter)
		}

		corrID := toolresp.NewCorrelationID("ann")
		h.capture.RegisterCommand(corrID, "", annotationWaitCommandTTL)
		h.annotationStore.RegisterWaiter(corrID, "", urlFilter)

		return mcp.Succeed(req, "Waiting for annotations", map[string]any{
			"status":         "waiting_for_user",
			"correlation_id": corrID,
			"annotations":    []any{},
			"count":          0,
			"filter_applied": annotation.FilterAppliedValue(urlFilter),
			"message":        "Draw mode is active. The user is drawing annotations. Poll with observe({what: 'command_result', correlation_id: '" + corrID + "'}) to check for results.",
		})
	}

	session := h.annotationStore.GetLatestSession()
	if session == nil {
		return mcp.Succeed(req, "No annotations", map[string]any{
			"annotations":    []any{},
			"count":          0,
			"filter_applied": annotation.FilterAppliedValue(urlFilter),
			"message":        "No annotation session found. Use interact({action: 'draw_mode_start'}) to activate draw mode, then the user draws annotations and presses ESC to finish.",
		})
	}
	return h.formatAnnotationSession(req, session, urlFilter)
}

func (h *Handler) formatAnnotationSession(req mcp.JSONRPCRequest, session *annotation.Session, urlFilter string) mcp.JSONRPCResponse {
	return mcp.Succeed(req, "Annotations retrieved", buildAnnotationSessionResult(session, urlFilter))
}

// #lizard forgives
func (h *Handler) getNamedAnnotations(req mcp.JSONRPCRequest, sessionName string, wait bool, waitTimeout time.Duration, urlFilter string) mcp.JSONRPCResponse {
	if wait {
		if ns := h.annotationStore.GetNamedSessionSinceDraw(sessionName); ns != nil {
			return h.formatNamedAnnotationSession(req, ns, urlFilter)
		}

		if ns, _ := h.annotationStore.WaitForNamedSession(sessionName, waitTimeout); ns != nil {
			return h.formatNamedAnnotationSession(req, ns, urlFilter)
		}

		corrID := toolresp.NewCorrelationID("ann")
		h.capture.RegisterCommand(corrID, "", annotationWaitCommandTTL)
		h.annotationStore.RegisterWaiter(corrID, sessionName, urlFilter)

		return mcp.Succeed(req, "Waiting for annotations", map[string]any{
			"status":             "waiting_for_user",
			"correlation_id":     corrID,
			"annot_session_name": sessionName,
			"pages":              []any{},
			"page_count":         0,
			"total_count":        0,
			"filter_applied":     annotation.FilterAppliedValue(urlFilter),
			"message":            "Draw mode is active. The user is drawing annotations. Poll with observe({what: 'command_result', correlation_id: '" + corrID + "'}) to check for results.",
		})
	}

	ns := h.annotationStore.GetNamedSession(sessionName)
	if ns == nil {
		return mcp.Succeed(req, "No annotations", map[string]any{
			"annot_session_name": sessionName,
			"pages":              []any{},
			"page_count":         0,
			"total_count":        0,
			"filter_applied":     annotation.FilterAppliedValue(urlFilter),
			"message":            "Named session '" + sessionName + "' not found. Use interact({action: 'draw_mode_start', annot_session: '" + sessionName + "'}) to start.",
		})
	}

	return h.formatNamedAnnotationSession(req, ns, urlFilter)
}

func (h *Handler) formatNamedAnnotationSession(req mcp.JSONRPCRequest, ns *annotation.NamedSession, urlFilter string) mcp.JSONRPCResponse {
	return mcp.Succeed(req, "Annotations retrieved", buildNamedAnnotationSessionResult(ns, urlFilter))
}

func buildAnnotationSessionResult(session *annotation.Session, urlFilter string) map[string]any {
	matched := annotation.URLMatches(urlFilter, session.PageURL)
	annotations := session.Annotations
	if !matched {
		annotations = []annotation.Annotation{}
	}

	result := map[string]any{
		"annotations":    annotations,
		"count":          len(annotations),
		"page_url":       session.PageURL,
		"filter_applied": annotation.FilterAppliedValue(urlFilter),
	}
	if session.ScreenshotPath != "" && matched {
		result["screenshot"] = session.ScreenshotPath
	}
	projects := toolanalyze.BuildProjectSummaries([]*annotation.Session{session})
	if len(projects) > 0 {
		result["projects"] = projects
	}
	if !matched && urlFilter != "" {
		result["message"] = "No annotations match the requested url filter."
	}
	if len(annotations) > 0 {
		result["hints"] = toolanalyze.BuildSessionHints(session.ScreenshotPath)
	}
	return result
}

func buildNamedAnnotationSessionResult(ns *annotation.NamedSession, urlFilter string) map[string]any {
	allProjects := toolanalyze.BuildProjectSummaries(ns.Pages)
	filteredPages := filterAnnotationPages(ns.Pages, urlFilter)

	totalCount := 0
	pages := make([]map[string]any, 0, len(filteredPages))
	for _, page := range filteredPages {
		totalCount += len(page.Annotations)
		p := map[string]any{
			"page_url":    page.PageURL,
			"annotations": page.Annotations,
			"count":       len(page.Annotations),
			"tab_id":      page.TabID,
		}
		if page.ScreenshotPath != "" {
			p["screenshot"] = page.ScreenshotPath
		}
		pages = append(pages, p)
	}

	// Find first screenshot for hints
	var screenshotPath string
	for _, page := range filteredPages {
		if page.ScreenshotPath != "" {
			screenshotPath = page.ScreenshotPath
			break
		}
	}

	result := map[string]any{
		"annot_session_name": ns.Name,
		"pages":              pages,
		"page_count":         len(filteredPages),
		"total_count":        totalCount,
		"filter_applied":     annotation.FilterAppliedValue(urlFilter),
	}
	if len(allProjects) > 0 {
		result["projects"] = allProjects
	}
	if len(allProjects) > 1 && urlFilter == "" {
		result["scope_ambiguous"] = true
		result["scope_warning"] = toolanalyze.BuildScopeWarning(allProjects)
	}
	if len(filteredPages) == 0 && urlFilter != "" {
		result["message"] = "No pages in this annotation session match the requested url filter."
	}
	if totalCount > 0 {
		result["hints"] = toolanalyze.BuildSessionHints(screenshotPath)
	}
	return result
}

func resolveAnnotationURLFilter(req mcp.JSONRPCRequest, urlValue, urlPatternValue string) (string, mcp.JSONRPCResponse, bool) {
	urlValue = strings.TrimSpace(urlValue)
	urlPatternValue = strings.TrimSpace(urlPatternValue)
	if urlValue != "" && urlPatternValue != "" && urlValue != urlPatternValue {
		return "", mcp.Fail(req, mcp.ErrInvalidParam,
			"Conflicting annotation scope filters: 'url' and 'url_pattern' differ",
			"Provide only one annotation scope filter, or set both to the same value.",
			mcp.WithParam("url"), mcp.WithParam("url_pattern"),
		), true
	}
	if urlPatternValue != "" {
		return urlPatternValue, mcp.JSONRPCResponse{}, false
	}
	return urlValue, mcp.JSONRPCResponse{}, false
}

func filterAnnotationPages(pages []*annotation.Session, urlFilter string) []*annotation.Session {
	if strings.TrimSpace(urlFilter) == "" {
		return pages
	}
	filtered := make([]*annotation.Session, 0, len(pages))
	for _, page := range pages {
		if annotation.URLMatches(urlFilter, page.PageURL) {
			filtered = append(filtered, page)
		}
	}
	return filtered
}

// toolFlushAnnotations forces completion of a pending annotation waiter.
// This is a recovery path for stuck waiters that would otherwise remain pending.
func (h *Handler) toolFlushAnnotations(req mcp.JSONRPCRequest, correlationID string, fallbackURLFilter string) mcp.JSONRPCResponse {
	correlationID = strings.TrimSpace(correlationID)
	if correlationID == "" {
		return mcp.Fail(req, mcp.ErrMissingParam,
			"Required parameter 'correlation_id' is missing for operation='flush'",
			"Pass the correlation_id returned by analyze({what:'annotations',wait:true}).",
			mcp.WithParam("correlation_id"))
	}
	if !strings.HasPrefix(correlationID, "ann_") {
		return mcp.Fail(req, mcp.ErrInvalidParam,
			"Invalid annotation correlation_id: "+correlationID,
			"Use an annotation correlation_id (prefix ann_) from analyze({what:'annotations',wait:true}).",
			mcp.WithParam("correlation_id"))
	}

	// Remove waiter first so later session writes cannot re-complete the same flush target.
	sessionName, waiterURLFilter, _ := h.annotationStore.TakeWaiter(correlationID)

	// Idempotent behavior: if the command is already terminal, return current state.
	if cmd, found := h.capture.GetCommandResult(correlationID); found && cmd != nil && cmd.Status != "pending" {
		return h.formatCommand(req, *cmd, correlationID)
	}

	effectiveURLFilter := waiterURLFilter
	if strings.TrimSpace(effectiveURLFilter) == "" {
		effectiveURLFilter = fallbackURLFilter
	}

	payload := h.buildFlushedAnnotationResult(sessionName, effectiveURLFilter)
	h.capture.ApplyCommandResult(correlationID, "complete", payload, "")

	// Normal path: return canonical command_result envelope.
	if cmd, found := h.capture.GetCommandResult(correlationID); found && cmd != nil {
		return h.formatCommand(req, *cmd, correlationID)
	}

	// Recovery fallback: command tracker no longer has this correlation_id.
	// Return flush payload directly so callers still receive deterministic output.
	var data map[string]any
	_ = json.Unmarshal(payload, &data)
	data["status"] = "complete"
	data["final"] = true
	data["correlation_id"] = correlationID
	data["lifecycle_status"] = "complete"
	return mcp.Succeed(req, "Annotation flush completed", data)
}

func (h *Handler) buildFlushedAnnotationResult(sessionName string, urlFilter string) json.RawMessage {
	if sessionName != "" {
		if ns := h.annotationStore.GetNamedSession(sessionName); ns != nil {
			data := buildNamedAnnotationSessionResult(ns, urlFilter)
			data["status"] = "complete"
			data["terminal_reason"] = "flushed"
			encoded, _ := json.Marshal(data)
			return encoded
		}

		encoded := mcp.SafeMarshal(map[string]any{
			"status":             "complete",
			"annot_session_name": sessionName,
			"pages":              []any{},
			"page_count":         0,
			"total_count":        0,
			"filter_applied":     annotation.FilterAppliedValue(urlFilter),
			"terminal_reason":    "abandoned",
			"message":            "Annotation waiter flushed with no named-session annotations available.",
		}, "{}")
		return encoded
	}

	if session := h.annotationStore.GetLatestSession(); session != nil {
		data := buildAnnotationSessionResult(session, urlFilter)
		data["status"] = "complete"
		data["terminal_reason"] = "flushed"
		encoded, _ := json.Marshal(data)
		return encoded
	}

	encoded := mcp.SafeMarshal(map[string]any{
		"status":          "complete",
		"annotations":     []any{},
		"count":           0,
		"filter_applied":  annotation.FilterAppliedValue(urlFilter),
		"terminal_reason": "abandoned",
		"message":         "Annotation waiter flushed with no captured annotations available.",
	}, "{}")
	return encoded
}

// toolGetAnnotationDetail returns full DOM/style detail for a specific annotation.
func (h *Handler) GetAnnotationDetail(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		CorrelationID string `json:"correlation_id"`
	}
	if len(args) > 0 {
		if resp, stop := mcp.ParseArgs(req, args, &params); stop {
			return resp
		}
	}

	if resp, blocked := toolresp.RequireString(req, params.CorrelationID, "correlation_id", "Add the 'correlation_id' from the annotation you want detail for"); blocked {
		return resp
	}

	detail, found := h.annotationStore.GetDetail(params.CorrelationID)
	if !found {
		return mcp.Fail(req, mcp.ErrNoData, "Annotation detail not found or expired for correlation_id: "+params.CorrelationID, "Detail data expires after 10 minutes. Re-run draw mode to capture fresh data.")
	}

	result := map[string]any{
		"correlation_id":  detail.CorrelationID,
		"selector":        detail.Selector,
		"tag":             detail.Tag,
		"text_content":    detail.TextContent,
		"classes":         detail.Classes,
		"id":              detail.ID,
		"computed_styles": detail.ComputedStyles,
		"parent_selector": detail.ParentSelector,
		"bounding_rect":   detail.BoundingRect,
	}
	if len(detail.A11yFlags) > 0 {
		result["a11y_flags"] = detail.A11yFlags
	}
	if detail.OuterHTML != "" {
		result["outer_html"] = detail.OuterHTML
	}
	if len(detail.ShadowDOM) > 0 {
		result["shadow_dom"] = detail.ShadowDOM
	}
	if len(detail.AllElements) > 0 {
		result["all_elements"] = detail.AllElements
		result["element_count"] = detail.ElementCount
	}
	if len(detail.SelectorCandidates) > 0 {
		result["selector_candidates"] = detail.SelectorCandidates
	}
	if len(detail.IframeContent) > 0 {
		result["iframe_content"] = detail.IframeContent
	}
	if len(detail.ParentContext) > 0 {
		result["parent_context"] = detail.ParentContext
	}
	if len(detail.Siblings) > 0 {
		result["siblings"] = detail.Siblings
	}
	if detail.CSSFramework != "" {
		result["css_framework"] = detail.CSSFramework
	}
	if detail.JSFramework != "" {
		result["js_framework"] = detail.JSFramework
	}
	if len(detail.Component) > 0 {
		result["component"] = detail.Component
	}

	// Error correlation: find console errors near the annotation's timestamp
	hasCorrelatedErrors := false
	if annotTS := h.annotationStore.FindAnnotationTimestamp(params.CorrelationID); annotTS > 0 {
		correlatedErrors := h.findErrorsNearTimestamp(annotTS, annotationErrorCorrelationWindow)
		if len(correlatedErrors) > 0 {
			result["correlated_errors"] = correlatedErrors
			result["error_correlation_window_seconds"] = 5
			hasCorrelatedErrors = true
		}
	}

	// Detail-level LLM hints (context-aware)
	if detailHints := toolanalyze.BuildDetailHints(detail.CSSFramework, detail.JSFramework, detail.A11yFlags, hasCorrelatedErrors); detailHints != nil {
		result["hints"] = detailHints
	}

	return mcp.Succeed(req, "Annotation detail", result)
}

func (h *Handler) findErrorsNearTimestamp(timestampMillis int64, window time.Duration) []map[string]string {
	entries := h.logEntries()
	annotationTime := time.UnixMilli(timestampMillis)
	windowStart, windowEnd := annotationTime.Add(-window), annotationTime.Add(window)
	var matched []map[string]string
	for index := len(entries) - 1; index >= 0 && len(matched) < 5; index-- {
		entry := entries[index]
		level, _ := entry["level"].(string)
		timestamp, _ := entry["ts"].(string)
		if level != "error" || timestamp == "" {
			continue
		}
		entryTime, err := time.Parse(time.RFC3339, timestamp)
		if err != nil || entryTime.Before(windowStart) || entryTime.After(windowEnd) {
			continue
		}
		message, _ := entry["message"].(string)
		matched = append(matched, map[string]string{"message": message, "ts": timestamp})
	}
	return matched
}
