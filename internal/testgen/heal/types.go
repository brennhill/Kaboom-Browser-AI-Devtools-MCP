// Purpose: Declares request/result types, error codes, and batch limits for selector healing.
// Docs: docs/features/feature/test-generation/index.md

package heal

// TestHealRequest represents generate {type: "test_heal"} parameters.
type TestHealRequest struct {
	Action          string   `json:"action"`           // "analyze" | "repair" | "batch"
	TestFile        string   `json:"test_file"`        // For analyze/repair
	TestDir         string   `json:"test_dir"`         // For batch
	BrokenSelectors []string `json:"broken_selectors"` // For repair
	AutoApply       bool     `json:"auto_apply"`       // For repair
}

// HealedSelector represents a repaired selector.
type HealedSelector struct {
	OldSelector string  `json:"old_selector"`
	NewSelector string  `json:"new_selector"`
	Confidence  float64 `json:"confidence"`
	Strategy    string  `json:"strategy"`
	LineNumber  int     `json:"line_number"`
}

// HealResult represents selector healing output.
type HealResult struct {
	Healed         []HealedSelector `json:"healed"`
	Unhealed       []string         `json:"unhealed"`
	UpdatedContent string           `json:"updated_content,omitempty"`
	Summary        HealSummary      `json:"summary"`
}

// HealSummary provides statistics on healing results.
type HealSummary struct {
	TotalBroken  int `json:"total_broken"`
	HealedAuto   int `json:"healed_auto"`
	HealedManual int `json:"healed_manual"`
	Unhealed     int `json:"unhealed"`
}

// BatchHealResult represents results from healing a batch of test files.
type BatchHealResult struct {
	FilesProcessed int              `json:"files_processed"`
	FilesSkipped   int              `json:"files_skipped"`
	TotalSelectors int              `json:"total_selectors"`
	TotalHealed    int              `json:"total_healed"`
	TotalUnhealed  int              `json:"total_unhealed"`
	FileResults    []FileHealResult `json:"file_results"`
	Warnings       []string         `json:"warnings,omitempty"`
}

// FileHealResult represents healing results for a single file.
type FileHealResult struct {
	FilePath string `json:"file_path"`
	Healed   int    `json:"healed"`
	Unhealed int    `json:"unhealed"`
	Skipped  bool   `json:"skipped"`
	Reason   string `json:"reason,omitempty"`
}

// Error codes for selector healing.
const (
	ErrTestFileNotFound      = "test_file_not_found"
	ErrSelectorInjection     = "selector_injection_detected"
	ErrInvalidSelectorSyntax = "invalid_selector_syntax"
)

// Batch limits.
const (
	MaxFilesPerBatch    = 20
	MaxFileSizeBytes    = 500 * 1024      // 500KB
	MaxTotalBatchSize   = 5 * 1024 * 1024 // 5MB
	MaxSelectorsPerFile = 50
)
