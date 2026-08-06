// eval.go — Eval runner library for kaboom-hooks.
// Loads JSON test fixtures, runs hooks, and validates output against expectations.

package eval

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/hook"
)

// Fixture represents a single eval test case loaded from a JSON file.
type Fixture struct {
	Description  string        `json:"description"`
	Hook         string        `json:"hook"`
	ProjectRoot  string        `json:"project_root"`
	SessionState *SessionState `json:"session_state,omitempty"`
	Input        FixtureInput  `json:"input"`
	Expect       Expectation   `json:"expect"`

	// Set by the loader, not from JSON.
	FixturePath string `json:"-"`
}

// FixtureInput holds the hook input fields for a fixture.
type FixtureInput struct {
	ToolName     string          `json:"tool_name"`
	ToolInput    json.RawMessage `json:"tool_input"`
	ToolResponse json.RawMessage `json:"tool_response,omitempty"`
}

// SessionState describes pre-existing session state for a fixture.
type SessionState struct {
	Touches []hook.TouchEntry `json:"touches"`
}

// Expectation defines what to validate about the hook output.
type Expectation struct {
	HasOutput    bool     `json:"has_output"`
	Contains     []string `json:"contains,omitempty"`
	NotContains  []string `json:"not_contains,omitempty"`
	MaxTokens    int      `json:"max_tokens,omitempty"`
	MaxLatencyMs int      `json:"max_latency_ms,omitempty"`
}

// Result holds the outcome of running a single fixture.
type Result struct {
	Fixture   *Fixture
	Output    string
	LatencyMs int64
	Passed    bool
	Failures  []string
}

type evaluationMode bool

const (
	contractEvaluation    evaluationMode = false
	performanceEvaluation evaluationMode = true
)

type hookRunner func(string, hook.Input, string, string) string

type fixtureRuntime struct {
	run     hookRunner
	measure func(func() string) (string, time.Duration)
}

func wallClockFixtureRuntime() fixtureRuntime {
	return fixtureRuntime{
		run: runHook,
		measure: func(run func() string) (string, time.Duration) {
			start := time.Now()
			output := run()
			return output, time.Since(start)
		},
	}
}

// fixtureDirs are the subdirectories of testdata/ that contain eval fixtures.
// Hook infrastructure dirs test specific hook behaviors.
// Principle dirs (u01-u10) test the 10 universal principles across
// the discover → suggest → enforce → migrate cycle.
var fixtureDirs = []string{
	// Hook infrastructure
	"quality-gate",
	"compress-output",
	"session-track",
	"blast-radius",
	"decision-guard",
	// Universal principles
	"u01-errors-not-ignored",
	"u02-single-responsibility",
	"u03-separation-of-concerns",
	"u04-no-magic-globals",
	"u05-immutability",
	"u06-fail-fast",
	"u07-explicit-over-implicit",
	"u08-no-raw-resource-access",
	"u09-testing-structure",
	"u10-dead-code-deleted",
}

// LoadFixtures loads all JSON fixture files from the hook subdirectories.
func LoadFixtures(dir string) ([]*Fixture, error) {
	var fixtures []*Fixture

	for _, hookDir := range fixtureDirs {
		hookPath := filepath.Join(dir, hookDir)
		info, err := os.Stat(hookPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("stat dir %s: %w", hookPath, err)
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("read dir %s: not a directory", hookPath)
		}
		err = filepath.WalkDir(hookPath, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("read fixture %s: %w", path, err)
			}
			var fix Fixture
			if err := json.Unmarshal(data, &fix); err != nil {
				return fmt.Errorf("parse fixture %s: %w", path, err)
			}
			fix.FixturePath = path
			fixtures = append(fixtures, &fix)
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("walk fixtures %s: %w", hookPath, err)
		}
	}
	return fixtures, nil
}

// runContractFixture validates deterministic output contracts without treating
// scheduler contention as hook latency. Production SLOs belong to the serial
// performance evaluation below.
func runContractFixture(fix *Fixture, repoRoot string) *Result {
	return runFixture(fix, repoRoot, contractEvaluation, wallClockFixtureRuntime())
}

// runPerformanceFixture validates the real hook implementation and its
// wall-clock production latency budget.
func runPerformanceFixture(fix *Fixture, repoRoot string) *Result {
	return runFixture(fix, repoRoot, performanceEvaluation, wallClockFixtureRuntime())
}

func runFixture(fix *Fixture, repoRoot string, mode evaluationMode, runtime fixtureRuntime) *Result {
	result := &Result{Fixture: fix}

	// Resolve project root.
	projectRoot := ""
	if fix.ProjectRoot == "REPO_ROOT" {
		projectRoot = repoRoot
	} else if fix.ProjectRoot != "" {
		projectRoot = fix.ProjectRoot
	}

	// Resolve relative file_path in tool_input to absolute using projectRoot.
	toolInput := fix.Input.ToolInput
	if materialized, cleanup := materializeFixtureFile(toolInput, repoRoot); cleanup != nil {
		toolInput = materialized
		defer cleanup()
	}
	if projectRoot != "" {
		toolInput = resolveToolInputPaths(toolInput, projectRoot)
	}

	// Build hook input.
	input := hook.Input{
		ToolName:     fix.Input.ToolName,
		ToolInput:    toolInput,
		ToolResponse: fix.Input.ToolResponse,
	}

	// Setup session state if needed.
	sessionDir := ""
	if fix.SessionState != nil {
		dir, err := os.MkdirTemp("", "eval-session-*")
		if err == nil {
			sessionDir = dir
			defer os.RemoveAll(dir)
			for _, touch := range fix.SessionState.Touches {
				_ = hook.AppendTouch(dir, touch)
			}
		}
	}

	// Run the hook and measure latency.
	output, elapsed := runtime.measure(func() string {
		return runtime.run(fix.Hook, input, projectRoot, sessionDir)
	})

	result.Output = output
	result.LatencyMs = elapsed.Milliseconds()

	// Validate.
	result.Failures = validate(fix.Expect, output, elapsed, mode == performanceEvaluation)
	result.Passed = len(result.Failures) == 0

	return result
}

func materializeFixtureFile(raw json.RawMessage, repoRoot string) (json.RawMessage, func()) {
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil {
		return raw, nil
	}
	var filePath string
	if json.Unmarshal(fields["file_path"], &filePath) != nil || filePath != "EVAL_OVERSIZED_FILE" {
		return raw, nil
	}
	file, err := os.CreateTemp(repoRoot, ".kaboom-eval-oversized-*.go")
	if err != nil {
		return raw, nil
	}
	path := file.Name()
	if _, err = file.WriteString("package evalfixture\n\n" + strings.Repeat("// eval fixture line\n", 801)); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return raw, nil
	}
	_ = file.Close()
	fields["file_path"], _ = json.Marshal(path)
	materialized, _ := json.Marshal(fields)
	return materialized, func() { _ = os.Remove(path) }
}

// runHook dispatches to the correct hook function.
func runHook(hookName string, input hook.Input, projectRoot, sessionDir string) string {
	switch hookName {
	case "quality-gate":
		r := hook.RunQualityGate(input)
		if r == nil {
			return ""
		}
		return r.FormatContext()

	case "compress-output":
		r := hook.CompressOutput(input)
		if r == nil {
			return ""
		}
		return r.FormatContext()

	case "session-track":
		if sessionDir == "" {
			dir, err := os.MkdirTemp("", "eval-session-*")
			if err != nil {
				return ""
			}
			sessionDir = dir
			defer os.RemoveAll(dir)
		}
		r := hook.RunSessionTrack(input, sessionDir)
		if r == nil {
			return ""
		}
		return r.FormatContext()

	case "blast-radius":
		r := hook.RunBlastRadius(input, projectRoot, sessionDir)
		if r == nil {
			return ""
		}
		return r.FormatContext()

	case "decision-guard":
		r := hook.RunDecisionGuard(input, projectRoot)
		if r == nil {
			return ""
		}
		return r.FormatContext()

	default:
		return ""
	}
}

// validate checks expectations against actual output.
func validate(expect Expectation, output string, elapsed time.Duration, enforceLatency bool) []string {
	var failures []string

	if expect.HasOutput && output == "" {
		failures = append(failures, "expected output but got empty")
	}
	if !expect.HasOutput && output != "" {
		failures = append(failures, fmt.Sprintf("expected no output but got: %s", truncate(output, 200)))
	}

	for _, s := range expect.Contains {
		if !strings.Contains(output, s) {
			failures = append(failures, fmt.Sprintf("output missing %q", s))
		}
	}

	for _, s := range expect.NotContains {
		if strings.Contains(output, s) {
			failures = append(failures, fmt.Sprintf("output should not contain %q", s))
		}
	}

	if expect.MaxTokens > 0 {
		tokens := len(output) / 4
		if tokens > expect.MaxTokens {
			failures = append(failures, fmt.Sprintf("output ~%d tokens exceeds budget %d", tokens, expect.MaxTokens))
		}
	}

	// Latency budgets measure real performance; the race detector inflates wall time
	// several-fold and makes the measurement meaningless, so skip the budget under -race.
	if enforceLatency && !raceDetectorActive && expect.MaxLatencyMs > 0 && elapsed.Milliseconds() > int64(expect.MaxLatencyMs) {
		failures = append(failures, fmt.Sprintf("latency %dms exceeds budget %dms", elapsed.Milliseconds(), expect.MaxLatencyMs))
	}

	return failures
}

// resolveToolInputPaths resolves relative file_path values in tool_input JSON
// to absolute paths by joining with the project root.
func resolveToolInputPaths(raw json.RawMessage, projectRoot string) json.RawMessage {
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil {
		return raw
	}
	fpRaw, ok := fields["file_path"]
	if !ok {
		return raw
	}
	var fp string
	if json.Unmarshal(fpRaw, &fp) != nil {
		return raw
	}
	if fp == "" || filepath.IsAbs(fp) {
		return raw
	}
	abs := filepath.Join(projectRoot, fp)
	absJSON, _ := json.Marshal(abs)
	fields["file_path"] = absJSON
	out, _ := json.Marshal(fields)
	return out
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}

// Report holds aggregate eval results.
type Report struct {
	Total  int                    `json:"total"`
	Passed int                    `json:"passed"`
	Failed int                    `json:"failed"`
	ByHook map[string]*HookReport `json:"by_hook"`
}

// HookReport holds per-hook aggregate results.
type HookReport struct {
	Total    int     `json:"total"`
	Passed   int     `json:"passed"`
	AvgLatMs float64 `json:"avg_latency_ms"`
	MaxLatMs int64   `json:"max_latency_ms"`
}

// Aggregate builds a report from a list of results.
func Aggregate(results []*Result) *Report {
	report := &Report{
		ByHook: make(map[string]*HookReport),
	}

	for _, r := range results {
		report.Total++
		if r.Passed {
			report.Passed++
		} else {
			report.Failed++
		}

		hr, ok := report.ByHook[r.Fixture.Hook]
		if !ok {
			hr = &HookReport{}
			report.ByHook[r.Fixture.Hook] = hr
		}
		hr.Total++
		if r.Passed {
			hr.Passed++
		}
		hr.AvgLatMs = (hr.AvgLatMs*float64(hr.Total-1) + float64(r.LatencyMs)) / float64(hr.Total)
		if r.LatencyMs > hr.MaxLatMs {
			hr.MaxLatMs = r.LatencyMs
		}
	}

	return report
}

// FormatReport produces a human-readable eval summary.
func FormatReport(report *Report) string {
	var b strings.Builder
	for hookName, hr := range report.ByHook {
		fmt.Fprintf(&b, "  %-20s %d/%d passed (avg %.0fms, max %dms)\n",
			hookName+":", hr.Passed, hr.Total, hr.AvgLatMs, hr.MaxLatMs)
	}
	fmt.Fprintf(&b, "\nAll evals: %d/%d passed.\n", report.Passed, report.Total)
	return b.String()
}
