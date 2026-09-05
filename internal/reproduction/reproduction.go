// Purpose: Implements reproduction script generation from captured enhanced-action timelines.
// Why: Turns observed failures into repeatable scripts for debugging and regression validation.
// Docs: docs/features/feature/reproduction-scripts/index.md

package reproduction

import (
	"encoding/json"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"time"
)

// Params are the parsed arguments for generate({format: "reproduction"}).
type Params struct {
	Format             string `json:"format"`
	OutputFormat       string `json:"output_format"`
	LastN              int    `json:"last_n"`
	BaseURL            string `json:"base_url"`
	IncludeScreenshots bool   `json:"include_screenshots"`
	ErrorMessage       string `json:"error_message"`
}

// Result is the response payload.
type Result struct {
	Script      string `json:"script"`
	Format      string `json:"format"`
	ActionCount int    `json:"action_count"`
	DurationMs  int64  `json:"duration_ms"`
	StartURL    string `json:"start_url"`
	Metadata    Meta   `json:"metadata"`
}

// Meta provides traceability for the generated script.
type Meta struct {
	GeneratedAt      string   `json:"generated_at"`
	SelectorsUsed    []string `json:"selectors_used"`
	ActionsAvailable int      `json:"actions_available"`
	ActionsIncluded  int      `json:"actions_included"`
	// FallbackOrder and LocatorCoverage let a caller tell a session recorded with all
	// three locators from one that only ever had selectors, without reading the script.
	FallbackOrder   []string       `json:"fallback_order"`
	LocatorCoverage map[string]int `json:"locator_coverage"`
	// EnvironmentPinned says whether the artifact depends on a pinned environment.
	EnvironmentPinned     bool `json:"environment_pinned"`
	EnvironmentPinChanged bool `json:"environment_pin_changed,omitempty"`
}

const maxReproOutputBytes = 200 * 1024 // 200KB cap

// ParseParams unmarshals and defaults the reproduction parameters.
func ParseParams(args json.RawMessage) Params {
	var params Params
	if len(args) > 0 {
		_ = json.Unmarshal(args, &params)
	}
	if params.OutputFormat == "" {
		params.OutputFormat = "kaboom-agentic-browser"
	}
	return params
}

// ValidateOutputFormat returns an error message if format is invalid, empty string if OK.
func ValidateOutputFormat(format string) string {
	if format != "kaboom-agentic-browser" && format != "playwright" {
		return "Invalid output_format: " + format
	}
	return ""
}

// FilterLastN returns the last N actions, or all if lastN <= 0.
func FilterLastN(actions []types.EnhancedAction, lastN int) []types.EnhancedAction {
	if lastN > 0 && lastN < len(actions) {
		return actions[len(actions)-lastN:]
	}
	return actions
}

// GenerateScript dispatches to the correct format generator.
func GenerateScript(actions []types.EnhancedAction, params Params) string {
	switch params.OutputFormat {
	case "playwright":
		return GeneratePlaywrightScript(actions, params)
	default:
		return GenerateKaboomScript(actions, params)
	}
}

// BuildResult assembles the response payload from a generated script.
func BuildResult(script string, params Params, actions, allActions []types.EnhancedAction) Result {
	startURL := reproStartURL(actions)
	var durationMs int64
	if len(actions) > 1 {
		durationMs = actions[len(actions)-1].Timestamp - actions[0].Timestamp
	}
	pin, pinChanged := sessionPin(actions)
	return Result{
		Script:      script,
		Format:      params.OutputFormat,
		ActionCount: len(actions),
		DurationMs:  durationMs,
		StartURL:    startURL,
		Metadata: Meta{
			GeneratedAt:           time.Now().Format(time.RFC3339),
			SelectorsUsed:         collectSelectorTypes(actions),
			ActionsAvailable:      len(allActions),
			ActionsIncluded:       len(actions),
			FallbackOrder:         fallbackOrder(),
			LocatorCoverage:       locatorCoverage(actions),
			EnvironmentPinned:     pin != nil,
			EnvironmentPinChanged: pinChanged,
		},
	}
}

func reproStartURL(actions []types.EnhancedAction) string {
	if len(actions) == 0 {
		return ""
	}
	if actions[0].Type == "navigate" && actions[0].ToURL != "" {
		return actions[0].ToURL
	}
	return actions[0].URL
}
