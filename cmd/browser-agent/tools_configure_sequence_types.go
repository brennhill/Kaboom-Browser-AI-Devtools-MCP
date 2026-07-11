// Purpose: Type aliases and re-exports for backward compatibility after configurehandler extraction.

package main

import (
	"sync"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/configurehandler"
)

// Type aliases for backward compatibility.
type Sequence = configurehandler.Sequence
type SequenceSummary = configurehandler.SequenceSummary
type SequenceStepResult = configurehandler.SequenceStepResult
type RecordingSnapshot = configurehandler.RecordingSnapshot

// Re-exported constants.
const (
	sequenceNamespace  = configurehandler.SequenceNamespace
	maxSequenceSteps   = configurehandler.MaxSequenceSteps
	maxSequenceNameLen = configurehandler.MaxSequenceNameLen
	defaultStepTimeout = configurehandler.DefaultStepTimeout
)

// Re-exported variables.
var sequenceNamePattern = configurehandler.SequenceNamePattern

// replayMu prevents concurrent sequence replays and batch executions.
// Shared between replay_sequence (configure) and batch execution (interact).
var replayMu sync.Mutex
