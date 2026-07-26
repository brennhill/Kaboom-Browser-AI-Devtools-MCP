// Purpose: Declares the diff result types and the recording source seam used by comparison.
// Docs: docs/features/feature/playback-engine/index.md

package logdiff

import "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/recording"

// RecordingSource supplies recordings to the diff engine. *recording.Manager
// satisfies it; tests can substitute an in-memory fake.
type RecordingSource interface {
	// GetRecording loads a recording by ID, validating the ID first.
	GetRecording(recordingID string) (*recording.Item, error)
}

// Result is the comparison of an original recording against a replay recording.
type Result struct {
	Status            string
	OriginalRecording string
	ReplayRecording   string
	Summary           string
	NewErrors         []LogEntry
	MissingEvents     []LogEntry
	ChangedValues     []ValueChange
	ActionStats       ActionComparison
}

// LogEntry is a single log line surfaced by a diff.
type LogEntry struct {
	Type       string
	Severity   string
	Level      string
	Message    string
	Timestamp  int64
	Selector   string
	ActionType string
}

// ValueChange is a typed-field value that differed between the two runs.
type ValueChange struct {
	Field     string
	FromValue string
	ToValue   string
	Timestamp int64
}

// ActionComparison is the per-action-type breakdown of both recordings.
type ActionComparison struct {
	OriginalCount     int
	ReplayCount       int
	ErrorsOriginal    int
	ErrorsReplay      int
	ClicksOriginal    int
	ClicksReplay      int
	TypesOriginal     int
	TypesReplay       int
	NavigatesOriginal int
	NavigatesReplay   int
}
