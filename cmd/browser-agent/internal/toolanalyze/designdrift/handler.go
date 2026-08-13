// handler.go — The analyze/design_audit mode: captures the page once, runs the analyzers, returns one envelope.
//
// PURPOSE: answer "is this page internally consistent with its own design
// system?" in one call, covering GitHub #693, #694 and #695.
//
// CONTRACT: one mode taking categories, not three modes. The page is probed
// exactly once and every analyzer works from that single payload, so the three
// categories cannot disagree about what the page looked like.

package designdrift

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/styleprobe"
)

// maxProbeElements is the element cap requested from the page. Higher than the
// probe's own default because a design audit over a truncated set produces a
// majority and a rhythm computed from partial evidence.
const maxProbeElements = 200

// Deps is the capture surface the mode needs: one style probe, and the tracked
// page's URL for the response envelope.
type Deps struct {
	// ProbeStyles runs a computed-style query against the tracked tab and
	// returns the raw probe payload.
	ProbeStyles func(selector string, maxElements int, includeCustomProperties bool) (json.RawMessage, error)
	// TrackingStatus reports whether a tab is tracked and which URL it holds.
	TrackingStatus func() (enabled bool, tabURL string)
}

type auditParams struct {
	Selector   string      `json:"selector"`
	Categories []string    `json:"categories"`
	Spec       *designSpec `json:"spec,omitempty"`
}

// elementView is the analyzer-facing view of one probed element. The analyzers
// never see the wire type, so a wire change cannot silently alter their
// semantics without this translation being updated too.
type elementView struct {
	Selector      string
	Index         int
	Styles        map[string]string
	Box           styleprobe.WireStyleProbeBox
	InFlow        bool
	ParentDisplay string
	ParentGap     string
	Tokens        map[string]string
}

// Handle runs the design audit.
func Handle(d Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params auditParams
	if len(args) > 0 {
		if resp, stop := mcp.ParseArgs(req, args, &params); stop {
			return resp
		}
	}
	if strings.TrimSpace(params.Selector) == "" {
		return mcp.Fail(req, mcp.ErrMissingParam,
			"design_audit requires a 'selector' naming the group of elements to compare",
			"Pass the selector of the repeated component, for example selector:'.step-card'",
			mcp.WithParam("selector"))
	}

	categories, invalid := resolveCategories(params.Categories)
	if invalid != "" {
		// Answering with an empty result for a misspelled category would look
		// like a clean page. Naming the valid set is what makes the mistake
		// self-correcting.
		return mcp.Fail(req, mcp.ErrInvalidParam,
			"Unknown design_audit category: "+invalid,
			"Use valid categories: "+strings.Join(allCategories(), ", "),
			mcp.WithParam("categories"))
	}

	enabled, tabURL := d.TrackingStatus()
	if !enabled {
		return mcp.Fail(req, mcp.ErrNoData, "No tab is being tracked. Track a tab first.",
			"Open the extension popup and click 'Track This Tab'.",
			mcp.WithRecoveryToolCall(map[string]any{
				"tool":      "configure",
				"arguments": map[string]any{"what": "health"},
			}))
	}

	probe, err := captureProbe(d, params.Selector)
	if err != nil {
		return mcp.Fail(req, mcp.ErrInternal,
			"Could not read computed styles from the page: "+err.Error(),
			"Confirm the extension is connected and the selector matches elements on the tracked tab.")
	}

	result := runAudit(params, probe, categories)
	result.PageURL = tabURL
	return mcp.Succeed(req, summarize(result), result)
}

// runAudit is the pure core: probe payload in, envelope out. Keeping it free of
// transport lets every analyzer hazard be table-tested without a browser.
func runAudit(params auditParams, probe styleprobe.WireStyleProbeResult, categories map[string]bool) auditResult {
	elements := viewsFrom(probe)
	tokens := buildTokenTable(probe.RootTokens)

	byCategory := make(map[string][]finding, len(categories))
	skips := make([]skipped, 0)

	if len(elements) == 0 {
		for _, category := range allCategories() {
			if categories[category] {
				skips = append(skips, skipped{Category: category, Reason: reasonNoElements})
			}
		}
		return buildAuditResult(params.Selector, elements, probe.MatchCount, probe.Truncated, byCategory, skips)
	}

	for _, category := range allCategories() {
		if !categories[category] {
			continue
		}
		findings, skip := runCategory(category, elements, tokens, params.Spec)
		if skip != nil {
			skips = append(skips, *skip)
			continue
		}
		byCategory[category] = findings
	}

	return buildAuditResult(params.Selector, elements, probe.MatchCount, probe.Truncated, byCategory, skips)
}

func runCategory(category string, elements []elementView, tokens tokenTable, spec *designSpec) ([]finding, *skipped) {
	switch category {
	case categoryStyleConsistency:
		return analyzeConsistency(elements, spec)
	case categoryDesignTokens:
		return analyzeTokens(elements, tokens, spec)
	case categorySpacing:
		return analyzeSpacing(elements, spec)
	}
	return nil, nil
}

// probeFailure is how the extension reports a failure: in-band, as a JSON body
// carrying an error key, with no transport error and no isError flag.
//
// Decoding such a body straight into WireStyleProbeResult succeeds — Go ignores
// unknown fields — and yields a zero value, so a dead service worker, a query
// timeout and a selector that matched nothing all became "no elements matched".
// Rule 25: a real failure must not be masked as an expected state.
type probeFailure struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

// decodeProbeFailure recognises the extension's in-band error shape.
func decodeProbeFailure(raw json.RawMessage) (error, bool) {
	var failure probeFailure
	if json.Unmarshal(raw, &failure) != nil || failure.Error == "" {
		// EXPECTED_ABSENCE: a successful probe carries no error key, and a body
		// that will not decode at all is reported by the caller's own unmarshal.
		return nil, false
	}
	detail := failure.Error
	if failure.Message != "" {
		detail = failure.Error + ": " + failure.Message
	}
	return fmt.Errorf("the page could not be probed (%s)", detail), true
}

// captureProbe asks the page for the style payload the analyzers consume.
func captureProbe(d Deps, selector string) (styleprobe.WireStyleProbeResult, error) {
	raw, err := d.ProbeStyles(selector, maxProbeElements, true)
	if err != nil {
		return styleprobe.WireStyleProbeResult{}, err
	}
	if failure, reported := decodeProbeFailure(raw); reported {
		return styleprobe.WireStyleProbeResult{}, failure
	}
	var probe styleprobe.WireStyleProbeResult
	if err := json.Unmarshal(raw, &probe); err != nil {
		return styleprobe.WireStyleProbeResult{}, fmt.Errorf("probe payload was not the expected shape: %w", err)
	}
	return probe, nil
}

// viewsFrom translates wire elements into the analyzer view, merging each
// element's in-scope custom properties over the document table.
func viewsFrom(probe styleprobe.WireStyleProbeResult) []elementView {
	views := make([]elementView, 0, len(probe.Elements))
	for _, el := range probe.Elements {
		views = append(views, elementView{
			Selector:      el.Selector,
			Index:         el.Index,
			Styles:        el.ComputedStyles,
			Box:           el.BoxModel,
			InFlow:        el.InFlow,
			ParentDisplay: el.ParentDisplay,
			ParentGap:     el.ParentGap,
			Tokens:        el.CustomProperties,
		})
	}
	return views
}

// resolveCategories validates the requested set, defaulting to all three.
func resolveCategories(requested []string) (map[string]bool, string) {
	if len(requested) == 0 {
		all := make(map[string]bool, len(allCategories()))
		for _, category := range allCategories() {
			all[category] = true
		}
		return all, ""
	}
	selected := make(map[string]bool, len(requested))
	for _, candidate := range requested {
		normalized := strings.TrimSpace(candidate)
		if !isKnownCategory(normalized) {
			return nil, candidate
		}
		selected[normalized] = true
	}
	return selected, ""
}

func isKnownCategory(candidate string) bool {
	for _, category := range allCategories() {
		if candidate == category {
			return true
		}
	}
	return false
}

// summarize is the one-line headline. It leads with the error count because
// that is the batch a caller can act on without review.
func summarize(result auditResult) string {
	if result.TotalFindings == 0 {
		if len(result.ChecksCompleted) == 0 {
			return "Design audit ran no checks"
		}
		return fmt.Sprintf("Design audit found no drift across %d element(s)", result.ElementsAudited)
	}
	return fmt.Sprintf("Design audit found %d finding(s) across %d element(s): %d error(s), %d warning(s)",
		result.TotalFindings, result.ElementsAudited, result.BySeverity[severityError], result.BySeverity[severityWarning])
}
