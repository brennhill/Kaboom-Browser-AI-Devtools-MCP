// contract.go — Canonical saved-sequence constants and wire models.
// Docs: docs/features/feature/batch-sequences/index.md

package sequencehandler

import (
	"encoding/json"
	"regexp"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/replay"
)

const (
	SequenceNamespace  = "sequences"
	MaxSequenceSteps   = replay.MaxSteps
	MaxSequenceNameLen = 64
	DefaultStepTimeout = replay.DefaultStepTimeout
)

var SequenceNamePattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// Sequence represents a named, replayable list of interact actions.
type Sequence struct {
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	SavedAt     string            `json:"saved_at"`
	StepCount   int               `json:"step_count"`
	Steps       []json.RawMessage `json:"steps"`
}

// SequenceSummary is returned by list_sequences (omits step details).
type SequenceSummary struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	SavedAt     string   `json:"saved_at"`
	StepCount   int      `json:"step_count"`
}
