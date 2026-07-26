// Purpose: Declares request/response types, constants, and error codes for test generation and failure classification.
// Docs: docs/features/feature/test-generation/index.md

package testgen

// ============================================
// Test Generation Types
// ============================================

// TestFromContextRequest represents generate {type: "test_from_context"} parameters.
type TestFromContextRequest struct {
	Context      string `json:"context"`       // "error", "interaction", "regression"
	ErrorID      string `json:"error_id"`      // Optional: specific error to reproduce
	Framework    string `json:"framework"`     // "playwright", "vitest", "jest"
	OutputFormat string `json:"output_format"` // "file", "inline"
	BaseURL      string `json:"base_url"`
	IncludeMocks bool   `json:"include_mocks"`
	TestName     string `json:"test_name"` // Optional: override filename base
}

// GeneratedTest represents the output of test generation.
type GeneratedTest struct {
	Framework  string          `json:"framework"`
	Filename   string          `json:"filename"`
	Content    string          `json:"content"`
	Selectors  []string        `json:"selectors"`
	Assertions int             `json:"assertions"`
	Coverage   TestCoverage    `json:"coverage"`
	Metadata   TestGenMetadata `json:"metadata"`
}

// TestCoverage describes what the generated test covers.
type TestCoverage struct {
	ErrorReproduced bool `json:"error_reproduced"`
	NetworkMocked   bool `json:"network_mocked"`
	StateCaptured   bool `json:"state_captured"`
}

// TestGenMetadata provides traceability.
type TestGenMetadata struct {
	SourceError string   `json:"source_error,omitempty"`
	GeneratedAt string   `json:"generated_at"`
	ContextUsed []string `json:"context_used"`
}

// ============================================
// Test Classify Types
// ============================================

// TestClassifyRequest represents generate {type: "test_classify"} parameters.
type TestClassifyRequest struct {
	Action     string        `json:"action"` // "failure", "batch"
	Failure    *TestFailure  `json:"failure"`
	Failures   []TestFailure `json:"failures"`
	TestOutput string        `json:"test_output"`
}

// TestFailure represents a single test failure to classify.
type TestFailure struct {
	TestName   string `json:"test_name"`
	Error      string `json:"error"`
	Screenshot string `json:"screenshot"` // base64, optional
	Trace      string `json:"trace"`      // stack trace
	DurationMs int64  `json:"duration_ms"`
}

// FailureClassification represents the result of classifying a test failure.
type FailureClassification struct {
	Category          string        `json:"category"`
	Confidence        float64       `json:"confidence"`
	Evidence          []string      `json:"evidence"`
	RecommendedAction string        `json:"recommended_action"`
	IsRealBug         bool          `json:"is_real_bug"`
	IsFlaky           bool          `json:"is_flaky"`
	IsEnvironment     bool          `json:"is_environment"`
	SuggestedFix      *SuggestedFix `json:"suggested_fix,omitempty"`
}

// SuggestedFix provides actionable fix suggestion.
type SuggestedFix struct {
	Type string `json:"type"` // "selector_update", "add_wait", "mock_network", etc.
	Old  string `json:"old,omitempty"`
	New  string `json:"new,omitempty"`
	Code string `json:"code,omitempty"`
}

// BatchClassifyResult represents the result of classifying multiple failures.
type BatchClassifyResult struct {
	TotalClassified int                     `json:"total_classified"`
	RealBugs        int                     `json:"real_bugs"`
	FlakyTests      int                     `json:"flaky_tests"`
	TestBugs        int                     `json:"test_bugs"`
	Uncertain       int                     `json:"uncertain"`
	Classifications []FailureClassification `json:"classifications"`
	Summary         map[string]int          `json:"summary"` // category -> count
}

// ============================================
// Constants
// ============================================

// Error codes for test generation and classification.
// Selector-healing error codes live in internal/testgen/heal.
const (
	ErrNoErrorContext          = "no_error_context"
	ErrNoActionsCaptured       = "no_actions_captured"
	ErrNoBaseline              = "no_baseline"
	ErrClassificationUncertain = "classification_uncertain"
	ErrBatchTooLarge           = "batch_too_large"
)

// Classification categories.
const (
	CategorySelectorBroken = "selector_broken"
	CategoryTimingFlaky    = "timing_flaky"
	CategoryNetworkFlaky   = "network_flaky"
	CategoryRealBug        = "real_bug"
	CategoryTestBug        = "test_bug"
	CategoryUnknown        = "unknown"
)

// MaxFailuresPerBatch is the limit for batch failure classification.
const MaxFailuresPerBatch = 20
