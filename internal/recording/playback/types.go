// Purpose: Declares the replay engine's session/result types and its recording source seam.
// Docs: docs/features/feature/playback-engine/index.md

package playback

import (
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/recording"
)

// RecordingSource supplies recordings to the replay engine. *recording.RecordingManager
// satisfies it; tests can substitute an in-memory fake.
type RecordingSource interface {
	// LookupRecording returns the recording with the given ID, preferring the
	// in-memory copy over the on-disk one.
	LookupRecording(recordingID string) (*recording.Recording, error)
}

// Result is the outcome of replaying a single recorded action.
type Result struct {
	Status          string
	ActionIndex     int
	ActionType      string
	SelectorUsed    string
	ExecutedAt      time.Time
	DurationMs      int64
	Error           string
	Coordinates     *Coordinates
	SelectorFragile bool
}

// Coordinates is an X/Y position on the page.
type Coordinates struct {
	X int
	Y int
}

// Session accumulates the results of replaying one recording.
type Session struct {
	RecordingID      string
	StartedAt        time.Time
	Results          []Result
	ActionsExecuted  int
	ActionsFailed    int
	SelectorFailures map[string]int
}
