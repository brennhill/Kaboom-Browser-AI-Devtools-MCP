// a11yaudit.go — Transport-independent accessibility audit: tracking gate, extension audit, summaries.
// Why: observe/analyze(accessibility) and the combined audit all ran the axe workflow through the MCP
// adapter and the whole Capture; this use case owns it behind a tracking check and an audit port so
// MCP parsing and response assembly stay in the adapters.
// Docs: docs/features/feature/observe/index.md

// Package a11yaudit runs the tracked tab's accessibility audit.
// It must not import internal/tools/mcp, internal/composition/capture/**, internal/tools/**,
// net/http or cmd/** (enforced by scripts/contracts/layering).
package a11yaudit

import (
	"encoding/json"
	"sort"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/analysis/a11ysummary"
)

// FailureKind classifies why an audit could not produce a result.
type FailureKind string

// Failure kinds; adapters map each to a protocol error.
const (
	KindNotTracking   FailureKind = "not_tracking"
	KindInvalidResult FailureKind = "invalid_result"
)

// Failure is a typed audit failure.
type Failure struct {
	Kind   FailureKind
	Detail string
}

func (f *Failure) Error() string { return string(f.Kind) + ": " + f.Detail }

// Deps wires the use case to its ports.
type Deps struct {
	TrackingEnabled func() bool
	// Execute runs axe in the tracked tab (scope, tags, frame, force refresh).
	Execute func(scope string, tags []string, frame any, forceRefresh bool) (json.RawMessage, error)
}

// Options are the caller-chosen audit settings.
type Options struct {
	Scope        string
	Tags         []string
	Frame        any
	ForceRefresh bool
	// Summary returns the compact pass/violation/top-issue projection.
	Summary bool
}

// Result is an audit outcome. PartialError is set when the extension audit
// failed (issue #276): Data then carries an empty, well-shaped partial result.
type Result struct {
	Data         map[string]any
	PartialError string
}

// Service runs the audit.
type Service struct{ deps Deps }

// New binds the use case to its ports.
func New(deps Deps) *Service { return &Service{deps: deps} }

// Audit runs the accessibility audit on the tracked tab.
func (s *Service) Audit(opts Options) (Result, error) {
	if s.deps.TrackingEnabled == nil || !s.deps.TrackingEnabled() || s.deps.Execute == nil {
		return Result{}, &Failure{Kind: KindNotTracking}
	}
	raw, err := s.deps.Execute(opts.Scope, opts.Tags, opts.Frame, opts.ForceRefresh)
	if err != nil {
		return Result{PartialError: err.Error(), Data: map[string]any{
			"violations":   []any{},
			"passes":       []any{},
			"incomplete":   []any{},
			"inapplicable": []any{},
			"error":        err.Error(),
			"partial":      true,
			"summary":      a11ysummary.BuildSummary(a11ysummary.Counts{}),
		}}, nil
	}
	var audit map[string]any
	if err := json.Unmarshal(raw, &audit); err != nil {
		return Result{}, &Failure{Kind: KindInvalidResult, Detail: err.Error()}
	}
	if opts.Summary {
		return Result{Data: buildSummary(audit)}, nil
	}
	a11ysummary.EnsureAuditSummary(audit)
	return Result{Data: audit}, nil
}

// buildSummary creates a compact representation of an a11y audit result.
func buildSummary(auditResult map[string]any) map[string]any {
	passes, _ := auditResult["passes"].([]any)
	violations, _ := auditResult["violations"].([]any)
	incomplete, _ := auditResult["incomplete"].([]any)

	type issueInfo struct {
		rule     string
		severity string
		count    int
	}
	issues := make([]issueInfo, 0, len(violations))
	for _, v := range violations {
		vMap, ok := v.(map[string]any)
		if !ok {
			continue
		}
		rule, _ := vMap["id"].(string)
		impact, _ := vMap["impact"].(string)
		nodes, _ := vMap["nodes"].([]any)
		issues = append(issues, issueInfo{rule: rule, severity: impact, count: len(nodes)})
	}
	sort.Slice(issues, func(i, j int) bool {
		return issues[i].count > issues[j].count
	})
	topN := 5
	if len(issues) < topN {
		topN = len(issues)
	}
	topIssues := make([]map[string]any, topN)
	for i := 0; i < topN; i++ {
		topIssues[i] = map[string]any{
			"rule":     issues[i].rule,
			"count":    issues[i].count,
			"severity": issues[i].severity,
		}
	}

	return map[string]any{
		"pass":       len(passes),
		"violations": len(violations),
		"incomplete": len(incomplete),
		"top_issues": topIssues,
	}
}
