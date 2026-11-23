// Purpose: Normalizes accessibility summary counts to the canonical wire contract.
// Why: Keeps accessibility payloads semantically consistent across extension and server boundaries.
// Docs: docs/features/feature/enhanced-wcag-audit/index.md

package a11ysummary

import (
	"encoding/json"
	"math"
	"strconv"
)

// Counts captures accessibility summary totals.
type Counts struct {
	Violations   int
	Passes       int
	Incomplete   int
	Inapplicable int
}

// BuildSummary returns a summary containing only canonical count keys.
func BuildSummary(counts Counts) map[string]any {
	return map[string]any{
		"violations":   counts.Violations,
		"passes":       counts.Passes,
		"incomplete":   counts.Incomplete,
		"inapplicable": counts.Inapplicable,
	}
}

// countsFromAuditResult derives counts from top-level a11y arrays.
func countsFromAuditResult(auditResult map[string]any) Counts {
	return Counts{
		Violations:   arrayLen(auditResult["violations"]),
		Passes:       arrayLen(auditResult["passes"]),
		Incomplete:   arrayLen(auditResult["incomplete"]),
		Inapplicable: arrayLen(auditResult["inapplicable"]),
	}
}

// EnsureAuditSummary adds or normalizes the "summary" field in an a11y result payload.
// Existing canonical values and unrelated metadata are preserved.
func EnsureAuditSummary(auditResult map[string]any) {
	if auditResult == nil {
		return
	}
	fallback := countsFromAuditResult(auditResult)

	rawSummary, ok := auditResult["summary"]
	if !ok {
		auditResult["summary"] = BuildSummary(fallback)
		return
	}

	summaryMap, ok := rawSummary.(map[string]any)
	if !ok {
		auditResult["summary"] = BuildSummary(fallback)
		return
	}

	violations := countOrDefault(summaryMap["violations"], fallback.Violations)
	passes := countOrDefault(summaryMap["passes"], fallback.Passes)
	incomplete := countOrDefault(summaryMap["incomplete"], fallback.Incomplete)
	inapplicable := countOrDefault(summaryMap["inapplicable"], fallback.Inapplicable)

	normalized := make(map[string]any, len(summaryMap)+4)
	for k, v := range summaryMap {
		if isLegacyCountKey(k) {
			continue
		}
		normalized[k] = v
	}

	normalized["violations"] = violations
	normalized["passes"] = passes
	normalized["incomplete"] = incomplete
	normalized["inapplicable"] = inapplicable
	auditResult["summary"] = normalized
}

func isLegacyCountKey(key string) bool {
	switch key {
	case "violation_count", "pass_count", "incomplete_count", "inapplicable_count":
		return true
	default:
		return false
	}
}

func arrayLen(value any) int {
	items, ok := value.([]any)
	if !ok {
		return 0
	}
	return len(items)
}

func countOrDefault(value any, defaultValue int) int {
	if v, ok := parseCount(value); ok {
		return v
	}
	return defaultValue
}

func parseCount(value any) (int, bool) {
	switch v := value.(type) {
	case int:
		return v, true
	case int8:
		return int(v), true
	case int16:
		return int(v), true
	case int32:
		return int(v), true
	case int64:
		return intFromInt64(v)
	case uint:
		return intFromUint64(uint64(v))
	case uint8:
		return int(v), true
	case uint16:
		return int(v), true
	case uint32:
		return int(v), true
	case uint64:
		return intFromUint64(v)
	case float32:
		return int(v), true
	case float64:
		return int(v), true
	case json.Number:
		return parseCountString(v.String())
	case string:
		return parseCountString(v)
	}
	return 0, false
}

func parseCountString(value string) (int, bool) {
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, false
	}
	return parsed, true
}

func intFromInt64(value int64) (int, bool) {
	if value > int64(math.MaxInt) || value < int64(math.MinInt) {
		return 0, false
	}
	return int(value), true
}

func intFromUint64(value uint64) (int, bool) {
	if value > uint64(math.MaxInt) {
		return 0, false
	}
	return int(value), true // #nosec G115 -- value is explicitly bounded to math.MaxInt above.
}
