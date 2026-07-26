// Purpose: Type aliases and re-exports for backward compatibility after toolconfigure extraction.

package main

import (
	"sync"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolconfigure"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolconfigure/netrecord"
)

// Type aliases for backward compatibility.
type Sequence = toolconfigure.Sequence
type SequenceSummary = toolconfigure.SequenceSummary
type SequenceStepResult = toolconfigure.SequenceStepResult
type RecordingSnapshot = netrecord.RecordingSnapshot

// Re-exported constants.
const (
	sequenceNamespace  = toolconfigure.SequenceNamespace
	maxSequenceSteps   = toolconfigure.MaxSequenceSteps
	maxSequenceNameLen = toolconfigure.MaxSequenceNameLen
	defaultStepTimeout = toolconfigure.DefaultStepTimeout
)

// Re-exported variables.
var sequenceNamePattern = toolconfigure.SequenceNamePattern

// replayMu prevents concurrent sequence replays and batch executions.
// Shared between replay_sequence (configure) and batch execution (interact).
var replayMu sync.Mutex
